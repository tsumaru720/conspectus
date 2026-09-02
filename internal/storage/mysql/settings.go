package mysql

import (
	"context"
	"fmt"

	"conspectus/internal/domain"
)

type settingsRepo struct{ *core }

func (r *settingsRepo) Get(ctx context.Context, key string) (string, error) {
	var v string
	err := r.ex().QueryRowContext(ctx, "SELECT value FROM settings WHERE setting = ?", key).Scan(&v)
	if err != nil {
		return "", mapNotFound(err)
	}
	return v, nil
}

func (r *settingsRepo) Set(ctx context.Context, key, value string) error {
	if key == "" {
		return fmt.Errorf("%w: empty settings key", domain.ErrValidation)
	}
	if len(key) > 20 {
		return fmt.Errorf("%w: settings key longer than 20 characters: %q", domain.ErrValidation, key)
	}
	if len(value) > 2048 {
		return fmt.Errorf("%w: settings value longer than 2048 characters", domain.ErrValidation)
	}
	_, err := r.ex().ExecContext(ctx,
		"INSERT INTO settings (setting, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = VALUES(value)",
		key, value)
	if err != nil {
		return fmt.Errorf("mysql: set setting %q: %w", key, err)
	}
	return nil
}

// SetDescription upserts a setting's description without touching its
// value: an absent row is created with an empty value, an existing row
// keeps its current one.
func (r *settingsRepo) SetDescription(ctx context.Context, key, description string) error {
	if key == "" {
		return fmt.Errorf("%w: empty settings key", domain.ErrValidation)
	}
	if len(key) > 20 {
		return fmt.Errorf("%w: settings key longer than 20 characters: %q", domain.ErrValidation, key)
	}
	if len(description) > 120 {
		return fmt.Errorf("%w: settings description longer than 120 characters", domain.ErrValidation)
	}
	_, err := r.ex().ExecContext(ctx,
		"INSERT INTO settings (setting, value, description) VALUES (?, '', ?) "+
			"ON DUPLICATE KEY UPDATE description = VALUES(description)",
		key, description)
	if err != nil {
		return fmt.Errorf("mysql: set description for setting %q: %w", key, err)
	}
	return nil
}

// Create inserts a new setting row with its value and display metadata.
// The key must be fresh: the primary key turns an INSERT for an existing
// key into a duplicate-key error, reported as ErrConflict.
func (r *settingsRepo) Create(ctx context.Context, key, value, description string, display bool) error {
	if key == "" {
		return fmt.Errorf("%w: empty settings key", domain.ErrValidation)
	}
	if len(key) > 20 {
		return fmt.Errorf("%w: settings key longer than 20 characters: %q", domain.ErrValidation, key)
	}
	if len(value) > 2048 {
		return fmt.Errorf("%w: settings value longer than 2048 characters", domain.ErrValidation)
	}
	if len(description) > 120 {
		return fmt.Errorf("%w: settings description longer than 120 characters", domain.ErrValidation)
	}
	_, err := r.ex().ExecContext(ctx,
		"INSERT INTO settings (setting, value, description, display) VALUES (?, ?, ?, ?)",
		key, value, description, display)
	if err != nil {
		if isDuplicateKey(err) {
			return fmt.Errorf("%w: setting %q already exists", domain.ErrConflict, key)
		}
		return fmt.Errorf("mysql: create setting %q: %w", key, err)
	}
	return nil
}

// SetDisplay opts a setting in or out of the manage page. Opting in
// upserts: an absent row is created with an empty value. Opting out only
// clears the flag - an absent row is already undisplayed, so the UPDATE
// matching nothing is fine.
func (r *settingsRepo) SetDisplay(ctx context.Context, key string, display bool) error {
	if key == "" {
		return fmt.Errorf("%w: empty settings key", domain.ErrValidation)
	}
	if len(key) > 20 {
		return fmt.Errorf("%w: settings key longer than 20 characters: %q", domain.ErrValidation, key)
	}
	if display {
		_, err := r.ex().ExecContext(ctx,
			"INSERT INTO settings (setting, value, display) VALUES (?, '', 1) "+
				"ON DUPLICATE KEY UPDATE display = 1",
			key)
		if err != nil {
			return fmt.Errorf("mysql: set display for setting %q: %w", key, err)
		}
		return nil
	}
	if _, err := r.ex().ExecContext(ctx,
		"UPDATE settings SET display = 0 WHERE setting = ?", key); err != nil {
		return fmt.Errorf("mysql: clear display for setting %q: %w", key, err)
	}
	return nil
}

func (r *settingsRepo) Delete(ctx context.Context, key string) error {
	_, err := r.ex().ExecContext(ctx, "DELETE FROM settings WHERE setting = ?", key)
	return err
}

func (r *settingsRepo) All(ctx context.Context) (map[string]string, error) {
	rows, err := r.ex().QueryContext(ctx, "SELECT setting, value FROM settings")
	if err != nil {
		return nil, fmt.Errorf("mysql: list settings: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func (r *settingsRepo) List(ctx context.Context) ([]domain.Setting, error) {
	rows, err := r.ex().QueryContext(ctx,
		"SELECT setting, value, description, display FROM settings ORDER BY setting")
	if err != nil {
		return nil, fmt.Errorf("mysql: list settings: %w", err)
	}
	defer rows.Close()
	out := []domain.Setting{}
	for rows.Next() {
		var s domain.Setting
		if err := rows.Scan(&s.Key, &s.Value, &s.Description, &s.Display); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
