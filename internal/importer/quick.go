package importer

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"conspectus/internal/apiclient"
	"conspectus/internal/domain"
)

type QuickRow struct {
	ID          int32
	Description string
	Deposit     string
	Value       string
	Stale       bool
	MonthsSince int
}

type latestRow struct {
	AssetID     int32   `json:"asset_id"`
	Description string  `json:"description"`
	Closed      bool    `json:"closed"`
	At          *string `json:"at"`
	Deposit     string  `json:"deposit"`
	Value       string  `json:"value"`
}

func QuickRows(ctx context.Context, api *apiclient.Client, loc *time.Location, now time.Time) ([]QuickRow, error) {
	var latest []latestRow
	if _, err := api.GetList(ctx, "/api/v1/logs/latest", &latest); err != nil {
		return nil, err
	}
	nowMonth := domain.MonthOf(now, loc)
	var out []QuickRow
	for _, row := range latest {
		if row.AssetID == 0 || row.Closed {
			continue
		}
		qr := QuickRow{ID: row.AssetID, Description: row.Description}
		if row.At != nil {
			qr.Deposit = row.Deposit
			qr.Value = row.Value
			if at, err := time.Parse(time.RFC3339, *row.At); err == nil {
				since := monthDiff(domain.MonthOf(at, loc), nowMonth)
				qr.MonthsSince = since
				if since > 1 {
					qr.Stale = true
				}
			}
		} else {
			qr.Deposit = "0.00"
			qr.Value = "0.00"
			qr.Stale = true
		}
		out = append(out, qr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Description < out[j].Description })
	return out, nil
}

type QuickEntry struct {
	AssetID int32
	Deposit domain.Money
	Value   domain.Money
	Payment domain.Money
}

func ParseQuickSubmission(form map[string][]string) ([]QuickEntry, error) {
	ids := map[int32]bool{}
	for k := range form {
		if n, _ := parsePrefixed(k, "deposit_"); n > 0 {
			ids[int32(n)] = true
		}
		if n, _ := parsePrefixed(k, "value_"); n > 0 {
			ids[int32(n)] = true
		}
	}
	var out []QuickEntry
	for id := range ids {
		first := func(prefix string) string {
			if vs, ok := form[prefix+strconv.Itoa(int(id))]; ok && len(vs) > 0 {
				return vs[0]
			}
			return ""
		}
		if first("skip_") != "" {
			continue
		}
		deposit, err := domain.ParseMoney(orDefault(first("deposit_"), "0"))
		if err != nil {
			return nil, err
		}
		value, err := domain.ParseMoney(orDefault(first("value_"), "0"))
		if err != nil {
			return nil, err
		}
		var payment domain.Money
		if p := first("payment_"); p != "" {
			if payment, err = domain.ParseMoney(p); err != nil {
				return nil, err
			}
		}
		out = append(out, QuickEntry{AssetID: id, Deposit: deposit, Value: value, Payment: payment})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AssetID < out[j].AssetID })
	return out, nil
}

func parsePrefixed(key, prefix string) (int64, bool) {
	rest, ok := strings.CutPrefix(key, prefix)
	if !ok || rest == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func monthDiff(a, b domain.Month) int {
	return (b.Year-a.Year)*12 + int(b.Month) - int(a.Month)
}
