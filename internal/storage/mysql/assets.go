package mysql

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"conspectus/internal/domain"
)

type assetRepo struct{ *core }

var assetSorts = map[string]string{
	"class,description": "c.description ASC, a.description ASC, a.id ASC",
	"description":       "a.description ASC, a.id ASC",
	"-description":      "a.description DESC, a.id ASC",
	"id":                "a.id ASC",
	"-id":               "a.id DESC",
}

const assetCols = "a.id, a.asset_class, a.description, a.closed"

func scanAsset(row interface{ Scan(...any) error }) (domain.Asset, error) {
	var a domain.Asset
	err := row.Scan(&a.ID, &a.ClassID, &a.Description, &a.Closed)
	return a, err
}

func (r *assetRepo) List(ctx context.Context, f domain.AssetFilter) ([]domain.Asset, int, error) {
	where := []string{"1=1"}
	args := []any{}
	if f.ClassID != 0 {
		where = append(where, "a.asset_class = ?")
		args = append(args, f.ClassID)
	}
	if f.Closed != nil {
		where = append(where, "a.closed = ?")
		args = append(args, *f.Closed)
	}
	if f.Query != "" && f.Regex != "" {
		return nil, 0, fmt.Errorf("%w: q and regex are mutually exclusive", domain.ErrValidation)
	}
	if f.Query != "" {
		where = append(where, "a.description LIKE ?")
		args = append(args, "%"+escapeLike(f.Query)+"%")
	}
	order, ok := assetSorts[f.Sort]
	if !ok {
		if f.Sort == "" {
			order = assetSorts["class,description"]
		} else {
			return nil, 0, fmt.Errorf("%w: invalid sort %q for assets", domain.ErrValidation, f.Sort)
		}
	}

	query := fmt.Sprintf(
		"SELECT %s FROM asset_list a JOIN asset_classes c ON c.id = a.asset_class WHERE %s ORDER BY %s",
		assetCols, strings.Join(where, " AND "), order)

	rows, err := r.ex().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("mysql: list assets: %w", err)
	}
	defer rows.Close()

	var out []domain.Asset
	var re *regexp.Regexp
	if f.Regex != "" {
		re, err = compileSearchRegex(f.Regex)
		if err != nil {
			return nil, 0, err
		}
	}
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("mysql: scan asset: %w", err)
		}
		if re != nil && !re.MatchString(a.Description) {
			continue
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	total := len(out)
	page, perPage := domain.Pagination(f.Page, f.PerPage)
	start := (page - 1) * perPage
	if start >= total {
		return []domain.Asset{}, total, nil
	}
	end := min(start+perPage, total)
	return out[start:end], total, nil
}

func (r *assetRepo) Get(ctx context.Context, id int32) (domain.Asset, error) {
	a, err := scanAsset(r.ex().QueryRowContext(ctx,
		fmt.Sprintf("SELECT %s FROM asset_list a WHERE a.id = ?", assetCols), id))
	if err != nil {
		return domain.Asset{}, mapNotFound(err)
	}
	return a, nil
}

func (r *assetRepo) Create(ctx context.Context, a domain.Asset) (domain.Asset, error) {
	if strings.TrimSpace(a.Description) == "" {
		return domain.Asset{}, fmt.Errorf("%w: description is required", domain.ErrValidation)
	}
	if a.ClassID == 0 {
		return domain.Asset{}, fmt.Errorf("%w: class_id is required", domain.ErrValidation)
	}
	if _, err := r.core.getClass(ctx, a.ClassID); err != nil {
		return domain.Asset{}, err
	}
	dup, err := r.assetExists(ctx, a.ClassID, a.Description, 0)
	if err != nil {
		return domain.Asset{}, err
	}
	if dup {
		return domain.Asset{}, fmt.Errorf("%w: an asset with this description already exists in the class", domain.ErrConflict)
	}
	res, err := r.ex().ExecContext(ctx,
		"INSERT INTO asset_list (asset_class, description, closed) VALUES (?, ?, ?)",
		a.ClassID, strings.TrimSpace(a.Description), boolToInt(a.Closed))
	if err != nil {
		return domain.Asset{}, fmt.Errorf("mysql: create asset: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Asset{}, err
	}
	a.ID = int32(id)
	return a, nil
}

func (r *assetRepo) Update(ctx context.Context, a domain.Asset) (domain.Asset, error) {
	if strings.TrimSpace(a.Description) == "" {
		return domain.Asset{}, fmt.Errorf("%w: description is required", domain.ErrValidation)
	}
	if a.ClassID == 0 {
		return domain.Asset{}, fmt.Errorf("%w: class_id is required", domain.ErrValidation)
	}
	if _, err := r.Get(ctx, a.ID); err != nil {
		return domain.Asset{}, err
	}
	if _, err := r.core.getClass(ctx, a.ClassID); err != nil {
		return domain.Asset{}, err
	}
	dup, err := r.assetExists(ctx, a.ClassID, a.Description, a.ID)
	if err != nil {
		return domain.Asset{}, err
	}
	if dup {
		return domain.Asset{}, fmt.Errorf("%w: an asset with this description already exists in the class", domain.ErrConflict)
	}
	_, err = r.ex().ExecContext(ctx,
		"UPDATE asset_list SET asset_class = ?, description = ?, closed = ? WHERE id = ?",
		a.ClassID, strings.TrimSpace(a.Description), boolToInt(a.Closed), a.ID)
	if err != nil {
		return domain.Asset{}, fmt.Errorf("mysql: update asset: %w", err)
	}
	return r.Get(ctx, a.ID)
}

func (r *assetRepo) SetClosed(ctx context.Context, id int32, closed bool) error {
	if _, err := r.Get(ctx, id); err != nil {
		return err
	}
	_, err := r.ex().ExecContext(ctx, "UPDATE asset_list SET closed = ? WHERE id = ?", boolToInt(closed), id)
	return err
}

func (r *assetRepo) Delete(ctx context.Context, id int32, force bool) (int64, int64, error) {
	if _, err := r.Get(ctx, id); err != nil {
		return 0, 0, err
	}
	var logs, payments int64
	row := r.ex().QueryRowContext(ctx,
		"SELECT (SELECT COUNT(*) FROM asset_log WHERE asset_id = ?), (SELECT COUNT(*) FROM payments WHERE asset_id = ?)", id, id)
	if err := row.Scan(&logs, &payments); err != nil {
		return 0, 0, err
	}
	if (logs > 0 || payments > 0) && !force {
		return logs, payments, fmt.Errorf("%w: asset has %d log entries and %d payments; retry with force=true to cascade-delete them",
			domain.ErrConflict, logs, payments)
	}
	if logs > 0 {
		if _, err := r.ex().ExecContext(ctx, "DELETE FROM asset_log WHERE asset_id = ?", id); err != nil {
			return 0, 0, err
		}
	}
	if payments > 0 {
		if _, err := r.ex().ExecContext(ctx, "DELETE FROM payments WHERE asset_id = ?", id); err != nil {
			return 0, 0, err
		}
	}
	if _, err := r.ex().ExecContext(ctx, "DELETE FROM asset_list WHERE id = ?", id); err != nil {
		return 0, 0, err
	}
	return logs, payments, nil
}

func (r *assetRepo) assetExists(ctx context.Context, classID int32, description string, excludeID int32) (bool, error) {
	row := r.ex().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM asset_list WHERE asset_class = ? AND description = ? AND id <> ?",
		classID, strings.TrimSpace(description), excludeID)
	var n int
	if err := row.Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func compileSearchRegex(pattern string) (*regexp.Regexp, error) {
	if len(pattern) > 512 {
		return nil, fmt.Errorf("%w: regex pattern too long (max 512 bytes)", domain.ErrValidation)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid regex: %v", domain.ErrValidation, err)
	}
	return re, nil
}

func (c *core) getClass(ctx context.Context, id int32) (domain.Class, error) {
	var cl domain.Class
	err := c.ex().QueryRowContext(ctx, "SELECT id, description FROM asset_classes WHERE id = ?", id).
		Scan(&cl.ID, &cl.Description)
	if err != nil {
		return domain.Class{}, mapNotFound(err)
	}
	return cl, nil
}
