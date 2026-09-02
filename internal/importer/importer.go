package importer

import (
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"conspectus/internal/apiclient"
	"conspectus/internal/domain"
)

type PreviewRow struct {
	Line      int
	AssetID   int32
	AssetName string
	Deposit   domain.Money
	Value     domain.Money
	Action    string
	Note      string
	Error     string
}

type Report struct {
	Rows    []PreviewRow
	Creates int
	Updates int
	Skipped int
	Errors  int
	Closed  int
	Unknown int
}

func (r *Report) Warnings() []string {
	var out []string
	if r.Closed > 0 {
		out = append(out, fmt.Sprintf("%d row(s) target closed assets and will not be applied", r.Closed))
	}
	if r.Unknown > 0 {
		out = append(out, fmt.Sprintf("%d row(s) have an id that matches no asset", r.Unknown))
	}
	return out
}

type CommitResult struct {
	Created  int
	Updated  int
	Skipped  int
	Warnings []string
}

type previewToken struct {
	report  *Report
	at      time.Time
	expires time.Time
}

type Importer struct {
	API *apiclient.Client
	Now func() time.Time
	Loc *time.Location

	mu     sync.Mutex
	tokens map[string]*previewToken
}

func New(api *apiclient.Client, loc *time.Location) *Importer {
	return &Importer{
		API: api, Loc: loc,
		tokens: map[string]*previewToken{},
		Now:    time.Now,
	}
}

func (im *Importer) now() time.Time {
	if im.Now != nil {
		return im.Now()
	}
	return time.Now()
}

const maxRows = 5000

type apiAsset struct {
	ID          int32  `json:"id"`
	ClassID     int32  `json:"class_id"`
	Description string `json:"description"`
	Closed      bool   `json:"closed"`
}

func (im *Importer) listAllAssets(ctx context.Context) ([]apiAsset, error) {
	var out []apiAsset
	page := 1
	for {
		var chunk []apiAsset
		total, err := im.API.GetList(ctx,
			apiclient.Query("/api/v1/assets", "per_page", 500, "page", page, "sort", "id"), &chunk)
		if err != nil {
			return nil, err
		}
		out = append(out, chunk...)
		if len(out) >= total || len(chunk) == 0 {
			break
		}
		page++
	}
	return out, nil
}

func (im *Importer) monthEntries(ctx context.Context, month domain.Month) (map[int32]int32, error) {
	out := map[int32]int32{}
	page := 1
	for {
		var chunk []struct {
			ID      int32 `json:"id"`
			AssetID int32 `json:"asset_id"`
		}
		total, err := im.API.GetList(ctx,
			apiclient.Query("/api/v1/logs",
				"from", month.Key(), "to", month.Key(),
				"per_page", 500, "page", page, "sort", "-epoch"), &chunk)
		if err != nil {
			return nil, err
		}
		for _, e := range chunk {
			if _, seen := out[e.AssetID]; !seen {
				out[e.AssetID] = e.ID
			}
		}
		if len(chunk) == 0 || len(out) >= total {
			break
		}
		page++
	}
	return out, nil
}

// Preview validates the CSV against the database for the month of at and
// returns a commit token; Commit later applies the rows to that same date.
func (im *Importer) Preview(ctx context.Context, r io.Reader, at time.Time) (*Report, string, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	assets, err := im.listAllAssets(ctx)
	if err != nil {
		return nil, "", err
	}
	byID := map[int32]apiAsset{}
	for _, a := range assets {
		byID[a.ID] = a
	}

	month := domain.MonthOf(at, im.loc())
	existing, err := im.monthEntries(ctx, month)
	if err != nil {
		return nil, "", err
	}

	report := &Report{}
	line := 0
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("%w: CSV parse error at line %d: %v", domain.ErrValidation, line+1, err)
		}
		line++
		if allEmpty(rec) {
			continue
		}
		row := PreviewRow{Line: line}
		// The name column is never imported, but skipped rows still show it
		// so the preview line is recognisable.
		if len(rec) > 1 {
			row.AssetName = strings.TrimSpace(rec[1])
		}

		rawID := strings.TrimSpace(rec[0])
		id64, err := strconv.ParseInt(rawID, 10, 32)
		if err != nil || id64 <= 0 {
			row.Note = "no id - skipped"
			report.Skipped++
			report.Rows = append(report.Rows, row)
			continue
		}

		a, ok := byID[int32(id64)]
		if !ok {
			row.AssetID = int32(id64)
			row.Note = fmt.Sprintf("unknown asset id %d - skipped", id64)
			report.Unknown++
			report.Rows = append(report.Rows, row)
			continue
		}
		row.AssetID = a.ID
		if row.AssetName == "" {
			row.AssetName = a.Description
		}

		if len(rec) < 4 {
			row.Error = "expected <id>,<name>,<deposit>,<value>"
			report.Errors++
			report.Rows = append(report.Rows, row)
			continue
		}
		if row.Deposit, err = domain.ParseMoney(rec[2]); err != nil {
			row.Error = "invalid deposit " + strings.TrimSpace(rec[2])
			report.Errors++
			report.Rows = append(report.Rows, row)
			continue
		}
		if row.Value, err = domain.ParseMoney(rec[3]); err != nil {
			row.Error = "invalid value " + strings.TrimSpace(rec[3])
			report.Errors++
			report.Rows = append(report.Rows, row)
			continue
		}

		if a.Closed {
			row.Note = "asset is closed - not updated"
			report.Closed++
			report.Rows = append(report.Rows, row)
			continue
		}

		if _, dup := existing[a.ID]; dup {
			row.Action = "update"
			row.Note = "replaces the existing entry for that month"
			report.Updates++
		} else {
			row.Action = "create"
			row.Note = "new entry for that month"
			report.Creates++
		}
		report.Rows = append(report.Rows, row)

		if len(report.Rows) >= maxRows {
			return nil, "", fmt.Errorf("%w: CSV exceeds the %d-row cap", domain.ErrValidation, maxRows)
		}
	}

	if len(report.Rows) == 0 {
		return nil, "", fmt.Errorf("%w: no data rows found - expected <id>,<name>,<deposit>,<value>", domain.ErrValidation)
	}

	token := newToken()
	im.mu.Lock()
	im.tokens[token] = &previewToken{report: report, at: at, expires: im.now().Add(15 * time.Minute)}
	im.mu.Unlock()
	return report, token, nil
}

type snapshotResult struct {
	Date            string `json:"date"`
	Month           string `json:"month"`
	Written         int    `json:"written"`
	Replaced        int    `json:"replaced"`
	PaymentsWritten int    `json:"payments_written"`
}

func (im *Importer) Commit(ctx context.Context, token string) (*CommitResult, error) {
	im.mu.Lock()
	pt, ok := im.tokens[token]
	if ok && im.now().After(pt.expires) {
		delete(im.tokens, token)
		ok = false
	}
	im.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("%w: preview token unknown or expired - preview again", domain.ErrValidation)
	}

	type entryRow struct {
		AssetID int32        `json:"asset_id"`
		Deposit domain.Money `json:"deposit"`
		Value   domain.Money `json:"value"`
	}
	entries := make([]entryRow, 0, len(pt.report.Rows))
	skipped := 0
	for _, row := range pt.report.Rows {
		if row.Action == "" {
			if row.Error == "" && row.Note != "" {
				skipped++
			}
			continue
		}
		entries = append(entries, entryRow{AssetID: row.AssetID, Deposit: row.Deposit, Value: row.Value})
	}
	var res snapshotResult
	if len(entries) > 0 {
		if err := im.API.Post(ctx, "/api/v1/logs", map[string]any{
			"date":         pt.at.Format("2006-01-02"),
			"entries":      entries,
			"on_duplicate": "replace",
		}, &res); err != nil {
			return nil, err
		}
	}
	out := &CommitResult{
		Created: res.Written,
		Updated: res.Replaced,
		Skipped: skipped,
	}
	if out.Created+out.Updated == 0 {
		out.Warnings = append(out.Warnings, "no assets were updated this month - check the file")
	}
	im.mu.Lock()
	delete(im.tokens, token)
	im.mu.Unlock()

	return out, nil
}

func (im *Importer) loc() *time.Location {
	if im.Loc != nil {
		return im.Loc
	}
	return time.UTC
}

func allEmpty(rec []string) bool {
	for _, f := range rec {
		if strings.TrimSpace(f) != "" {
			return false
		}
	}
	return true
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
