package mysql

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"conspectus/internal/domain"
)

type paymentRepo struct{ *core }

var paymentSorts = map[string]string{
	"-epoch":  "p.epoch DESC, p.id DESC",
	"epoch":   "p.epoch ASC, p.id ASC",
	"asset":   "a.description ASC, p.epoch DESC",
	"-asset":  "a.description DESC, p.epoch DESC",
	"amount":  "p.amount ASC, p.epoch DESC",
	"-amount": "p.amount DESC, p.epoch DESC",
}

const paymentCols = "p.id, p.asset_id, p.epoch, p.amount"

func scanPayment(row interface{ Scan(...any) error }) (domain.Payment, error) {
	var p domain.Payment
	var amount string
	err := row.Scan(&p.ID, &p.AssetID, &p.At, &amount)
	if err != nil {
		return domain.Payment{}, err
	}
	if p.Amount, err = domain.ParseMoney(amount); err != nil {
		return domain.Payment{}, fmt.Errorf("mysql: scan payment amount %q: %w", amount, err)
	}
	return p, nil
}

func (r *paymentRepo) paymentWhere(f domain.PaymentFilter) (string, []any, error) {
	where := []string{"1=1"}
	args := []any{}
	if f.AssetID != 0 {
		where = append(where, "p.asset_id = ?")
		args = append(args, f.AssetID)
	}
	if f.ClassID != 0 {
		where = append(where, "a.asset_class = ?")
		args = append(args, f.ClassID)
	}
	if f.From != nil {
		where = append(where, "p.epoch >= ?")
		args = append(args, f.From.Start(r.loc))
	}
	if f.To != nil {
		where = append(where, "p.epoch < ?")
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

func (r *paymentRepo) List(ctx context.Context, f domain.PaymentFilter) ([]domain.Payment, int, error) {
	where, args, err := r.paymentWhere(f)
	if err != nil {
		return nil, 0, err
	}
	var re *regexp.Regexp
	if f.Regex != "" {
		if re, err = compileSearchRegex(f.Regex); err != nil {
			return nil, 0, err
		}
	}
	order, ok := paymentSorts[f.Sort]
	if !ok {
		if f.Sort == "" {
			order = paymentSorts["-epoch"]
		} else {
			return nil, 0, fmt.Errorf("%w: invalid sort %q for payments", domain.ErrValidation, f.Sort)
		}
	}
	query := fmt.Sprintf(
		"SELECT %s, a.description FROM payments p JOIN asset_list a ON a.id = p.asset_id WHERE %s ORDER BY %s",
		paymentCols, where, order)
	rows, err := r.ex().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("mysql: list payments: %w", err)
	}
	defer rows.Close()

	type keyed struct {
		p     domain.Payment
		descr string
	}
	var all []keyed
	for rows.Next() {
		var k keyed
		var amount string
		if err := rows.Scan(&k.p.ID, &k.p.AssetID, &k.p.At, &amount, &k.descr); err != nil {
			return nil, 0, err
		}
		if k.p.Amount, err = domain.ParseMoney(amount); err != nil {
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
		return []domain.Payment{}, total, nil
	}
	end := min(start+perPage, total)
	out := make([]domain.Payment, 0, end-start)
	for _, k := range all[start:end] {
		out = append(out, k.p)
	}
	return out, total, nil
}

func (r *paymentRepo) Get(ctx context.Context, id int32) (domain.Payment, error) {
	p, err := scanPayment(r.ex().QueryRowContext(ctx,
		fmt.Sprintf("SELECT %s FROM payments p WHERE p.id = ?", paymentCols), id))
	if err != nil {
		return domain.Payment{}, mapNotFound(err)
	}
	return p, nil
}

func (r *paymentRepo) Create(ctx context.Context, p domain.Payment) (domain.Payment, error) {
	if _, err := r.core.getAsset(ctx, p.AssetID); err != nil {
		return domain.Payment{}, err
	}
	res, err := r.ex().ExecContext(ctx,
		"INSERT INTO payments (asset_id, epoch, amount) VALUES (?, ?, ?)",
		p.AssetID, p.At, p.Amount.String())
	if err != nil {
		return domain.Payment{}, fmt.Errorf("mysql: create payment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Payment{}, err
	}
	p.ID = int32(id)
	return p, nil
}

func (r *paymentRepo) Update(ctx context.Context, p domain.Payment) (domain.Payment, error) {
	if _, err := r.Get(ctx, p.ID); err != nil {
		return domain.Payment{}, err
	}
	if _, err := r.core.getAsset(ctx, p.AssetID); err != nil {
		return domain.Payment{}, err
	}
	_, err := r.ex().ExecContext(ctx,
		"UPDATE payments SET asset_id = ?, epoch = ?, amount = ? WHERE id = ?",
		p.AssetID, p.At, p.Amount.String(), p.ID)
	if err != nil {
		return domain.Payment{}, fmt.Errorf("mysql: update payment: %w", err)
	}
	return r.Get(ctx, p.ID)
}

func (r *paymentRepo) Delete(ctx context.Context, id int32) error {
	if _, err := r.Get(ctx, id); err != nil {
		return err
	}
	_, err := r.ex().ExecContext(ctx, "DELETE FROM payments WHERE id = ?", id)
	return err
}
