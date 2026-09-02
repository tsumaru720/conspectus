package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	gomysql "github.com/go-sql-driver/mysql"

	"conspectus/internal/config"
	"conspectus/internal/domain"
	"conspectus/internal/storage"
)

const (
	EnvHost     = "CONSPECTUS_MYSQL_HOST"
	EnvUsername = "CONSPECTUS_MYSQL_USERNAME"
	EnvPassword = "CONSPECTUS_MYSQL_PASSWORD"
	EnvDatabase = "CONSPECTUS_MYSQL_DATABASE"
)

const defaultPort = "3306"

func init() {
	storage.RegisterDriver(storage.DriverInfo{
		Name: "mysql",
		EnvVars: []storage.EnvVar{
			{Key: EnvHost, Required: true, Purpose: "MySQL host, optionally host:port (default port " + defaultPort + ")"},
			{Key: EnvUsername, Purpose: "database user"},
			{Key: EnvPassword, Purpose: "database password"},
			{Key: EnvDatabase, Required: true, Purpose: "database name"},
		},
	}, Open)
}

type core struct {
	db     *sql.DB
	tx     *sql.Tx
	ownsDB bool
	loc    *time.Location
}

type store struct{ *core }

func Open(cfg config.Config) (domain.Repos, error) {
	dsn, err := BuildDSN(os.Getenv)
	if err != nil {
		return nil, err
	}
	return openDSN(dsn, cfg.Timezone)
}

// BuildDSN pins the driver loc to UTC: the server session (and therefore
// every TIMESTAMP we read or write) is expected to run UTC - the default
// for the database container images. The app's calendar timezone (TZ) is
// a display/bucketing concern on top and never touches the driver.
func BuildDSN(get func(string) string) (string, error) {
	host := get(EnvHost)
	if host == "" {
		return "", fmt.Errorf("mysql: %s is required - the MySQL host, optionally host:port (default port %s)", EnvHost, defaultPort)
	}
	if !strings.Contains(host, ":") {
		host = host + ":" + defaultPort
	}
	name := get(EnvDatabase)
	if name == "" {
		return "", fmt.Errorf("mysql: %s is required - the database name", EnvDatabase)
	}

	drvCfg := gomysql.Config{
		User:                 get(EnvUsername),
		Passwd:               get(EnvPassword),
		Net:                  "tcp",
		Addr:                 host,
		DBName:               name,
		ParseTime:            true,
		Loc:                  time.UTC,
		Collation:            "utf8mb4_general_ci",
		AllowNativePasswords: true,
	}
	return drvCfg.FormatDSN(), nil
}

func openDSN(dsn string, tz *time.Location) (domain.Repos, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql: open: %w", err)
	}
	db.SetConnMaxLifetime(time.Hour)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("mysql: ping: %w", err)
	}
	return &store{&core{db: db, ownsDB: true, loc: tz}}, nil
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (c *core) ex() execer {
	if c.tx != nil {
		return c.tx
	}
	return c.db
}

func (s *store) Tx(ctx context.Context, fn func(domain.Repos) error) error {
	if s.tx != nil {
		return fn(s)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mysql: begin: %w", err)
	}
	if err := fn(&store{&core{db: s.db, tx: tx, loc: s.loc}}); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *store) Close() error {
	if s.ownsDB {
		return s.db.Close()
	}
	return nil
}

func (s *store) Assets() domain.AssetRepo      { return &assetRepo{s.core} }
func (s *store) Classes() domain.ClassRepo     { return &classRepo{s.core} }
func (s *store) Logs() domain.LogRepo          { return &logRepo{s.core} }
func (s *store) Payments() domain.PaymentRepo  { return &paymentRepo{s.core} }
func (s *store) Settings() domain.SettingsRepo { return &settingsRepo{s.core} }

func (s *store) RawDB() *sql.DB { return s.db }

func mapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func isDuplicateKey(err error) bool {
	if myErr, ok := errors.AsType[*gomysql.MySQLError](err); ok {
		return myErr.Number == 1062
	}
	return false
}
