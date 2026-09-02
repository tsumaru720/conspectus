package mysql

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"conspectus/internal/domain"
)

type logRepo struct{ *core }

var logSorts = map[string]string{
	"-epoch": "l.epoch DESC, l.id DESC",
	"epoch":  "l.epoch ASC, l.id ASC",
	"asset":  "a.description ASC, l.epoch DESC",
	"-asset": "a.description DESC, l.epoch DESC",
	"value":  "l.asset_value DESC, l.epoch DESC",
	"-value": "l.asset_value ASC, l.epoch DESC",
}

const logCols = "l.id, l.asset_id, l.epoch, l.deposit_value, l.asset_value"

func scanLog(row interface{ Scan(...any) error }) (domain.LogEntry, error) {
	var e domain.LogEntry
	var deposit, value string
	err := row.Scan(&e.ID, &e.AssetID, &e.At, &deposit, &value)
	if err != nil {
		return domain.LogEntry{}, err
	}
	if e.Deposit, err = domain.ParseMoney(deposit); err != nil {
		return domain.LogEntry{}, fmt.Errorf("mysql: scan log deposit %q: %w", deposit, err)
	}
	if e.Value, err = domain.ParseMoney(value); err != nil {
		return domain.LogEntry{}, fmt.Errorf("mysql: scan log value %q: %w", value, err)
	}
	return e, nil
}

func (r *logRepo) logWhere(f domain.LogFilter) (string, []any, error) {
	where := []string{"1=1"}
	args := []any{}
	if f.AssetID != 0 {
		where = append(where, "l.asset_id = ?")
		args = append(args, f.AssetID)
	}
	if f.ClassID != 0 {
		where = append(where, "a.asset_class = ?")
		args = append(args, f.ClassID)
	}
	if f.From != nil {
		where = append(where, "l.epoch >= ?")
		args = append(args, f.From.Start(r.loc))
	}
	if f.To != nil {
		where = append(where, "l.epoch < ?")
		args = append(args, f.To.AddMonths(1).Start(r.loc))
	}
	if f.Query != "" && f.Regex != "" {
		return "", nil, fmt.Errorf("%w: q and regex are mutually exclusive", domain.ErrValidation)
	}
	if f.Query != "" {
		where = append(where, "a.description LIKE ?")
		args = append(args, "%"+escapeLike(f.Query)+"%")
	}
	return strings.Join(where, " AND "), args, nil
}

func (r *logRepo) List(ctx context.Context, f domain.LogFilter) ([]domain.LogEntry, int, error) {
	where, args, err := r.logWhere(f)
	if err != nil {
		return nil, 0, err
	}
	var re *regexp.Regexp
	if f.Regex != "" {
		if re, err = compileSearchRegex(f.Regex); err != nil {
			return nil, 0, err
		}
	}
	order, ok := logSorts[f.Sort]
	if !ok {
		if f.Sort == "" {
			order = logSorts["-epoch"]
		} else {
			return nil, 0, fmt.Errorf("%w: invalid sort %q for logs", domain.ErrValidation, f.Sort)
		}
	}
	query := fmt.Sprintf(
		"SELECT %s, a.description FROM asset_log l JOIN asset_list a ON a.id = l.asset_id WHERE %s ORDER BY %s",
		logCols, where, order)
	rows, err := r.ex().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("mysql: list logs: %w", err)
	}
	defer rows.Close()

	type keyed struct {
		e     domain.LogEntry
		descr string
	}
	var all []keyed
	for rows.Next() {
		var k keyed
		var deposit, value string
		if err := rows.Scan(&k.e.ID, &k.e.AssetID, &k.e.At, &deposit, &value, &k.descr); err != nil {
			return nil, 0, err
		}
		if k.e.Deposit, err = domain.ParseMoney(deposit); err != nil {
			return nil, 0, err
		}
		if k.e.Value, err = domain.ParseMoney(value); err != nil {
			return nil, 0, err
		}
		if re != nil && !re.MatchString(k.descr) {
			continue
		}
		all = append(all, k)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	total := len(all)
	page, perPage := domain.Pagination(f.Page, f.PerPage)
	start := (page - 1) * perPage
	if start >= total {
		return []domain.LogEntry{}, total, nil
	}
	end := min(start+perPage, total)
	out := make([]domain.LogEntry, 0, end-start)
	for _, k := range all[start:end] {
		out = append(out, k.e)
	}
	return out, total, nil
}

func (r *logRepo) Get(ctx context.Context, id int32) (domain.LogEntry, error) {
	e, err := scanLog(r.ex().QueryRowContext(ctx,
		fmt.Sprintf("SELECT %s FROM asset_log l WHERE l.id = ?", logCols), id))
	if err != nil {
		return domain.LogEntry{}, mapNotFound(err)
	}
	return e, nil
}

func (r *logRepo) Create(ctx context.Context, e domain.LogEntry) (domain.LogEntry, error) {
	if _, err := r.core.getAsset(ctx, e.AssetID); err != nil {
		return domain.LogEntry{}, err
	}
	if existing, err := r.FindByAssetMonth(ctx, e.AssetID, domain.MonthOf(e.At, r.loc)); err == nil {
		return domain.LogEntry{}, &domain.ConflictError{
			Message:    "an entry for this asset already exists in that month",
			ExistingID: existing.ID,
		}
	} else if !isNotFound(err) {
		return domain.LogEntry{}, err
	}
	res, err := r.ex().ExecContext(ctx,
		"INSERT INTO asset_log (asset_id, epoch, deposit_value, asset_value) VALUES (?, ?, ?, ?)",
		e.AssetID, e.At, e.Deposit.String(), e.Value.String())
	if err != nil {
		return domain.LogEntry{}, fmt.Errorf("mysql: create log: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.LogEntry{}, err
	}
	e.ID = int32(id)
	return e, nil
}

func (r *logRepo) Update(ctx context.Context, e domain.LogEntry) (domain.LogEntry, error) {
	if _, err := r.Get(ctx, e.ID); err != nil {
		return domain.LogEntry{}, err
	}
	if _, err := r.core.getAsset(ctx, e.AssetID); err != nil {
		return domain.LogEntry{}, err
	}
	if existing, err := r.FindByAssetMonth(ctx, e.AssetID, domain.MonthOf(e.At, r.loc)); err == nil {
		if existing.ID != e.ID {
			return domain.LogEntry{}, &domain.ConflictError{
				Message:    "another entry for this asset already exists in that month",
				ExistingID: existing.ID,
			}
		}
	} else if !isNotFound(err) {
		return domain.LogEntry{}, err
	}
	_, err := r.ex().ExecContext(ctx,
		"UPDATE asset_log SET asset_id = ?, epoch = ?, deposit_value = ?, asset_value = ? WHERE id = ?",
		e.AssetID, e.At, e.Deposit.String(), e.Value.String(), e.ID)
	if err != nil {
		return domain.LogEntry{}, fmt.Errorf("mysql: update log: %w", err)
	}
	return r.Get(ctx, e.ID)
}

func (r *logRepo) Delete(ctx context.Context, id int32) error {
	if _, err := r.Get(ctx, id); err != nil {
		return err
	}
	_, err := r.ex().ExecContext(ctx, "DELETE FROM asset_log WHERE id = ?", id)
	return err
}

func (r *logRepo) FindByAssetMonth(ctx context.Context, assetID int32, m domain.Month) (domain.LogEntry, error) {
	e, err := scanLog(r.ex().QueryRowContext(ctx,
		fmt.Sprintf("SELECT %s FROM asset_log l WHERE l.asset_id = ? AND l.epoch >= ? AND l.epoch < ? ORDER BY l.epoch DESC, l.id DESC LIMIT 1", logCols),
		assetID, m.Start(r.loc), m.AddMonths(1).Start(r.loc)))
	if err != nil {
		return domain.LogEntry{}, mapNotFound(err)
	}
	return e, nil
}

func (r *logRepo) SeriesRows(ctx context.Context, q domain.SeriesQuery) ([]domain.SeriesRow, error) {
	where := []string{"1=1"}
	args := []any{}
	if len(q.AssetIDs) > 0 {
		ph := make([]string, len(q.AssetIDs))
		for i, id := range q.AssetIDs {
			ph[i] = "?"
			args = append(args, id)
		}
		where = append(where, "l.asset_id IN ("+strings.Join(ph, ",")+")")
	}
	if q.ClassID != 0 {
		where = append(where, "a.asset_class = ?")
		args = append(args, q.ClassID)
	}
	logQuery := fmt.Sprintf(
		"SELECT l.asset_id, l.epoch, l.deposit_value, l.asset_value FROM asset_log l JOIN asset_list a ON a.id = l.asset_id WHERE %s ORDER BY l.asset_id, l.epoch, l.id",
		strings.Join(where, " AND "))

	rows, err := r.ex().QueryContext(ctx, logQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql: series rows: %w", err)
	}
	defer rows.Close()

	type cell struct {
		assetID int32
		month   domain.Month
		epoch   time.Time
		deposit domain.Money
		value   domain.Money
		entries int
	}
	cells := map[domain.Month]map[int32]*cell{}

	for rows.Next() {
		var assetID int32
		var epoch time.Time
		var deposit, value string
		if err := rows.Scan(&assetID, &epoch, &deposit, &value); err != nil {
			return nil, err
		}
		d, err := domain.ParseMoney(deposit)
		if err != nil {
			return nil, err
		}
		v, err := domain.ParseMoney(value)
		if err != nil {
			return nil, err
		}
		m := domain.MonthOf(epoch, r.loc)
		if q.From != nil && m.Before(*q.From) {
			continue
		}
		if q.To != nil && m.After(*q.To) {
			continue
		}
		byAsset, ok := cells[m]
		if !ok {
			byAsset = map[int32]*cell{}
			cells[m] = byAsset
		}
		c, ok := byAsset[assetID]
		if !ok {
			c = &cell{assetID: assetID, month: m, epoch: epoch, deposit: d, value: v, entries: 1}
			byAsset[assetID] = c
		} else {
			c.entries++
			if !epoch.Before(c.epoch) {
				c.epoch, c.deposit, c.value = epoch, d, v
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	payArgs := append([]any{}, args...)
	payQuery := fmt.Sprintf(
		"SELECT p.asset_id, p.epoch, p.amount, a.closed FROM payments p JOIN asset_list a ON a.id = p.asset_id WHERE %s",
		strings.ReplaceAll(strings.Join(where, " AND "), "l.asset_id", "p.asset_id"))
	prows, err := r.ex().QueryContext(ctx, payQuery, payArgs...)
	if err != nil {
		return nil, fmt.Errorf("mysql: series payments: %w", err)
	}
	defer prows.Close()
	pays := map[domain.Month]map[int32]domain.Money{}
	closed := map[int32]bool{}
	for prows.Next() {
		var assetID int32
		var epoch time.Time
		var amount string
		var assetClosed bool
		if err := prows.Scan(&assetID, &epoch, &amount, &assetClosed); err != nil {
			return nil, err
		}
		if assetClosed {
			closed[assetID] = true
		}
		amt, err := domain.ParseMoney(amount)
		if err != nil {
			return nil, err
		}
		m := domain.MonthOf(epoch, r.loc)
		if q.From != nil && m.Before(*q.From) {
			continue
		}
		if q.To != nil && m.After(*q.To) {
			continue
		}
		if pays[m] == nil {
			pays[m] = map[int32]domain.Money{}
		}
		pays[m][assetID] += amt
	}
	if err := prows.Err(); err != nil {
		return nil, err
	}

	var out []domain.SeriesRow
	for m, byAsset := range cells {
		for assetID, c := range byAsset {
			out = append(out, domain.SeriesRow{
				AssetID:  assetID,
				Month:    m,
				Deposit:  c.deposit,
				Value:    c.value,
				Payments: pays[m][assetID],
				Entries:  c.entries,
			})
		}
	}
	for m, byAsset := range pays {
		for assetID, amount := range byAsset {
			if _, ok := cells[m][assetID]; ok || amount == 0 {
				continue
			}
			var bestMonth domain.Month
			var best cell
			found := false
			for mm, byA := range cells {
				if mm.After(m) {
					continue
				}
				if c, ok := byA[assetID]; ok && (!found || mm.After(bestMonth)) {
					best = *c
					bestMonth = mm
					found = true
				}
			}
			if !found {
				continue
			}
			if closed[assetID] {
				best.deposit, best.value = 0, 0
			}
			out = append(out, domain.SeriesRow{
				AssetID:  assetID,
				Month:    m,
				Deposit:  best.deposit,
				Value:    best.value,
				Payments: amount,
				Entries:  0,
			})
		}
	}
	return out, nil
}

func (c *core) getAsset(ctx context.Context, id int32) (domain.Asset, error) {
	a, err := scanAsset(c.ex().QueryRowContext(ctx,
		"SELECT id, asset_class, description, closed FROM asset_list WHERE id = ?", id))
	if err != nil {
		return domain.Asset{}, mapNotFound(err)
	}
	return a, nil
}

func isNotFound(err error) bool {
	return errors.Is(err, domain.ErrNotFound)
}
