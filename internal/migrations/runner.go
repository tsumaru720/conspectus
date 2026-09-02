package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	gomysql "github.com/go-sql-driver/mysql"
)

const StateKey = "db_migration_state"

const VersionKey = "db_version"

const lockName = "conspectus_migrate"

type migrationState struct {
	File            string `json:"file"`
	StatementIndex  int    `json:"statement_index"`
	StatementsTotal int    `json:"statements_total"`
	StartedAt       string `json:"started_at"`
}

type Runner struct {
	DB       *sql.DB
	Logger   *slog.Logger
	Set      []Migration
	LockWait time.Duration
}

type Result struct {
	Applied  []int
	From, To int
}

func ExpectedVersion(set []Migration) int {
	if len(set) == 0 {
		return 0
	}
	return set[len(set)-1].Version
}

func (r *Runner) CurrentVersion(ctx context.Context) (int, error) {
	exists, err := r.settingsTableExists(ctx)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}
	var v string
	err = r.DB.QueryRowContext(ctx, "SELECT value FROM settings WHERE setting = ?", VersionKey).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("migrate: read db_version: %w", err)
	}
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &n); err != nil {
		return 0, nil
	}
	return n, nil
}

func (r *Runner) Plan(ctx context.Context) ([]Migration, int, error) {
	current, err := r.CurrentVersion(ctx)
	if err != nil {
		return nil, 0, err
	}
	return r.pending(current), current, nil
}

func (r *Runner) pending(current int) []Migration {
	var out []Migration
	for _, m := range r.Set {
		if m.Version > current {
			out = append(out, m)
		}
	}
	return out
}

func (r *Runner) releaseLockAndClose(conn *sql.Conn) {
	_, _ = conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", lockName)
	_ = conn.Close()
}

func (r *Runner) Run(ctx context.Context) (Result, error) {
	res := Result{}
	lockWait := r.LockWait
	if lockWait == 0 {
		lockWait = 30 * time.Second
	}

	conn, err := r.acquireLock(ctx, lockWait)
	if err != nil {
		return res, err
	}
	defer r.releaseLockAndClose(conn)

	current, err := r.CurrentVersion(ctx)
	if err != nil {
		return res, err
	}
	res.From = current
	res.To = current

	pending := r.pending(current)
	if err := r.checkGapless(current, pending); err != nil {
		return res, err
	}
	if current > ExpectedVersion(r.Set) {
		return res, fmt.Errorf("migrate: schema version %d is newer than this build applies (%d) - upgrade the application", current, ExpectedVersion(r.Set))
	}
	if len(pending) == 0 {
		return res, nil
	}

	for _, m := range pending {
		if err := r.applyOne(ctx, conn, m); err != nil {
			return res, fmt.Errorf("migration %04d (%s) failed: %w", m.Version, m.Name, err)
		}
		res.Applied = append(res.Applied, m.Version)
		res.To = m.Version
	}
	r.clearState(ctx)
	return res, nil
}

func (r *Runner) applyOne(ctx context.Context, conn *sql.Conn, m Migration) error {
	stmts := []string{m.Contents}
	if !m.Raw {
		stmts = Statements(m.Contents)
	}

	start := 0
	if st, ok := r.readState(ctx); ok && st.File == m.Name {
		if st.StatementIndex > 0 && st.StatementIndex < len(stmts) {
			start = st.StatementIndex
			r.warn("resuming %s at statement %d of %d (previous attempt incomplete)", m.Name, start+1, len(stmts))
		}
	}

	for k := start; k < len(stmts); k++ {
		if err := r.writeState(ctx, migrationState{
			File:            m.Name,
			StatementIndex:  k,
			StatementsTotal: len(stmts),
			StartedAt:       time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			return fmt.Errorf("persist state before statement %d: %w", k+1, err)
		}
		if _, err := conn.ExecContext(ctx, stmts[k]); err != nil {
			if isAlreadyApplied(err) {
				r.warn("statement %d/%d of %s reported already-applied (%v) - continuing", k+1, len(stmts), m.Name, err)
			} else {
				return fmt.Errorf("statement %d/%d failed: %v\nSQL: %s", k+1, len(stmts), err, firstLines(stmts[k], 3))
			}
		}
	}
	return r.setVersion(ctx, conn, m.Version)
}

func isAlreadyApplied(err error) bool {
	if myErr, ok := errors.AsType[*gomysql.MySQLError](err); ok {
		return myErr.Number == 1050 || myErr.Number == 1061
	}
	return false
}

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[:n]
		return strings.Join(lines, "\n") + "\n…"
	}
	return strings.Join(lines, "\n")
}

func (r *Runner) checkGapless(current int, pending []Migration) error {
	want := current + 1
	for _, m := range pending {
		if m.Version != want {
			return fmt.Errorf("migration sequence gap: expected %04d next, found %04d (%s) - the set must be strictly sequential and gapless", want, m.Version, m.Name)
		}
		want++
	}
	return nil
}

func (r *Runner) settingsTableExists(ctx context.Context) (bool, error) {
	var n int
	err := r.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'settings'").Scan(&n)
	if err != nil {
		return false, fmt.Errorf("migrate: probe settings table: %w", err)
	}
	return n > 0, nil
}

func (r *Runner) writeState(ctx context.Context, st migrationState) error {
	exists, err := r.settingsTableExists(ctx)
	if err != nil || !exists {
		return err
	}
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = r.DB.ExecContext(ctx,
		"INSERT INTO settings (setting, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = VALUES(value)",
		StateKey, string(data))
	if isDataTooLong(err) {
		r.warn("migration state does not fit the settings column yet; resume bookmarking skipped until the schema widens")
		return nil
	}
	return err
}

func isDataTooLong(err error) bool {
	if myErr, ok := errors.AsType[*gomysql.MySQLError](err); ok {
		return myErr.Number == 1406
	}
	return false
}

func (r *Runner) readState(ctx context.Context) (migrationState, bool) {
	var raw string
	err := r.DB.QueryRowContext(ctx, "SELECT value FROM settings WHERE setting = ?", StateKey).Scan(&raw)
	if err != nil {
		return migrationState{}, false
	}
	var st migrationState
	if json.Unmarshal([]byte(raw), &st) != nil {
		return migrationState{}, false
	}
	return st, true
}

func (r *Runner) clearState(ctx context.Context) {
	if _, err := r.DB.ExecContext(ctx, "DELETE FROM settings WHERE setting = ?", StateKey); err != nil {
		r.warn("could not clear %s: %v", StateKey, err)
	}
}

func (r *Runner) setVersion(ctx context.Context, conn *sql.Conn, v int) error {
	if _, err := conn.ExecContext(ctx,
		"INSERT INTO settings (setting, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = VALUES(value)",
		VersionKey, fmt.Sprintf("%d", v)); err != nil {
		return fmt.Errorf("stamp db_version=%d: %w", v, err)
	}
	return nil
}

func (r *Runner) acquireLock(ctx context.Context, wait time.Duration) (*sql.Conn, error) {
	conn, err := r.DB.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("migrate: acquire connection: %w", err)
	}
	deadline := time.Now().Add(wait)
	for {
		var got int
		if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 5)", lockName).Scan(&got); err != nil {
			// The lock is advisory, and some servers prohibit GET_LOCK outright
			// (Percona XtraDB Cluster and MariaDB Galera with strict mode enforced).
			// Degrade to unlocked migration rather than refusing to start; if the
			// connection itself is broken the queries below fail loudly anyway.
			r.warn("advisory migration lock unavailable (%v) - continuing without cross-instance serialization", err)
			return conn, nil
		}
		if got == 1 {
			return conn, nil
		}
		if time.Now().After(deadline) {
			conn.Close()
			return nil, fmt.Errorf("migrate: another instance holds the migration lock; timed out after %s - re-run once it finishes", wait)
		}
		r.warn("migration lock held by another instance; waiting...")
		select {
		case <-ctx.Done():
			conn.Close()
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (r *Runner) Repair(ctx context.Context) (Result, error) {
	res := Result{}
	conn, err := r.acquireLock(ctx, 30*time.Second)
	if err != nil {
		return res, err
	}
	defer r.releaseLockAndClose(conn)

	st, ok := r.readState(ctx)
	if !ok {
		return res, fmt.Errorf("migrate --repair: no in-progress migration state recorded - nothing to repair")
	}
	var target Migration
	for _, m := range r.Set {
		if m.Name == st.File {
			target = m
			break
		}
	}
	if target.Name == "" {
		return res, fmt.Errorf("migrate --repair: recorded file %q is not in the migration set", st.File)
	}
	current, err := r.CurrentVersion(ctx)
	if err != nil {
		return res, err
	}
	res.From, res.To = current, target.Version
	if err := r.applyOne(ctx, conn, target); err != nil {
		return res, fmt.Errorf("repair of %s failed: %w", st.File, err)
	}
	res.Applied = append(res.Applied, target.Version)
	r.clearState(ctx)
	return res, nil
}

func (r *Runner) warn(format string, args ...any) {
	if r.Logger != nil {
		r.Logger.Warn(fmt.Sprintf(format, args...))
	}
}
