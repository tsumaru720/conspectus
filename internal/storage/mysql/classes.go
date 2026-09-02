package mysql

import (
	"context"
	"fmt"
	"strings"

	"conspectus/internal/domain"
)

type classRepo struct{ *core }

func (r *classRepo) List(ctx context.Context) ([]domain.Class, error) {
	rows, err := r.ex().QueryContext(ctx, "SELECT id, description FROM asset_classes ORDER BY description, id")
	if err != nil {
		return nil, fmt.Errorf("mysql: list classes: %w", err)
	}
	defer rows.Close()
	var out []domain.Class
	for rows.Next() {
		var c domain.Class
		if err := rows.Scan(&c.ID, &c.Description); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *classRepo) Get(ctx context.Context, id int32) (domain.Class, error) {
	return r.getClass(ctx, id)
}

func (r *classRepo) Create(ctx context.Context, c domain.Class) (domain.Class, error) {
	if strings.TrimSpace(c.Description) == "" {
		return domain.Class{}, fmt.Errorf("%w: description is required", domain.ErrValidation)
	}
	var n int
	if err := r.ex().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM asset_classes WHERE description = ?", strings.TrimSpace(c.Description)).Scan(&n); err != nil {
		return domain.Class{}, err
	}
	if n > 0 {
		return domain.Class{}, fmt.Errorf("%w: a class with this description already exists", domain.ErrConflict)
	}
	res, err := r.ex().ExecContext(ctx, "INSERT INTO asset_classes (description) VALUES (?)", strings.TrimSpace(c.Description))
	if err != nil {
		return domain.Class{}, fmt.Errorf("mysql: create class: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Class{}, err
	}
	c.ID = int32(id)
	return c, nil
}

func (r *classRepo) Update(ctx context.Context, c domain.Class) (domain.Class, error) {
	if strings.TrimSpace(c.Description) == "" {
		return domain.Class{}, fmt.Errorf("%w: description is required", domain.ErrValidation)
	}
	if _, err := r.getClass(ctx, c.ID); err != nil {
		return domain.Class{}, err
	}
	var n int
	if err := r.ex().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM asset_classes WHERE description = ? AND id <> ?", strings.TrimSpace(c.Description), c.ID).Scan(&n); err != nil {
		return domain.Class{}, err
	}
	if n > 0 {
		return domain.Class{}, fmt.Errorf("%w: a class with this description already exists", domain.ErrConflict)
	}
	if _, err := r.ex().ExecContext(ctx, "UPDATE asset_classes SET description = ? WHERE id = ?",
		strings.TrimSpace(c.Description), c.ID); err != nil {
		return domain.Class{}, fmt.Errorf("mysql: update class: %w", err)
	}
	return r.getClass(ctx, c.ID)
}

func (r *classRepo) Delete(ctx context.Context, id int32) error {
	if _, err := r.getClass(ctx, id); err != nil {
		return err
	}
	var n int
	if err := r.ex().QueryRowContext(ctx, "SELECT COUNT(*) FROM asset_list WHERE asset_class = ?", id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("%w: %d assets still reference this class", domain.ErrConflict, n)
	}
	_, err := r.ex().ExecContext(ctx, "DELETE FROM asset_classes WHERE id = ?", id)
	return err
}
