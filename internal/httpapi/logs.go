package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"conspectus/internal/domain"
)

type logIn struct {
	AssetID int32         `json:"asset_id"`
	Date    string        `json:"date"`
	Deposit *domain.Money `json:"deposit"`
	Value   *domain.Money `json:"value"`
}

func entryFilterFromQuery(r *http.Request) (domain.EntryFilter, error) {
	f := domain.EntryFilter{
		Query: r.URL.Query().Get("q"),
		Regex: r.URL.Query().Get("regex"),
		Sort:  r.URL.Query().Get("sort"),
	}
	if f.Query != "" && f.Regex != "" {
		return f, fmt.Errorf("%w: q and regex are mutually exclusive", domain.ErrValidation)
	}
	if v, ok := queryInt(r, "asset_id"); ok {
		f.AssetID = v
	}
	if v, ok := queryInt(r, "class_id"); ok {
		f.ClassID = v
	}
	var err error
	if f.From, err = queryMonth(r, "from"); err != nil {
		return f, err
	}
	if f.To, err = queryMonth(r, "to"); err != nil {
		return f, err
	}
	if s := r.URL.Query().Get("page"); s != "" {
		if n, e := strconv.Atoi(s); e == nil {
			f.Page = n
		}
	}
	if s := r.URL.Query().Get("per_page"); s != "" {
		if n, e := strconv.Atoi(s); e == nil {
			f.PerPage = n
		}
	}
	return f, nil
}

func (a *API) handleLogsList(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	f, err := entryFilterFromQuery(r)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	logs, total, err := repos.Logs().List(r.Context(), f)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	page, perPage := domain.Pagination(f.Page, f.PerPage)
	WriteData(w, http.StatusOK, logs, &Meta{Page: page, PerPage: perPage, Total: total})
}

type logsCreateIn struct {
	Date        string       `json:"date"`
	Entries     []logEntryIn `json:"entries"`
	OnDuplicate string       `json:"on_duplicate"`
}

type logEntryIn struct {
	AssetID int32         `json:"asset_id"`
	Deposit domain.Money  `json:"deposit"`
	Value   domain.Money  `json:"value"`
	Payment *domain.Money `json:"payment"`
}

type skippedEntry struct {
	AssetID int32  `json:"asset_id"`
	Reason  string `json:"reason"`
}

type duplicateEntry struct {
	AssetID     int32  `json:"asset_id"`
	ExistingID  int32  `json:"existing_id"`
	ActionTaken string `json:"action"`
}

func (a *API) handleLogsCreate(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	var in logsCreateIn
	if err := DecodeJSON(r, &in); err != nil {
		WriteDomainError(w, err)
		return
	}
	if in.Date == "" {
		WriteError(w, http.StatusBadRequest, "validation_error", "date is required (YYYY-MM-DD)")
		return
	}
	if len(in.Entries) == 0 {
		WriteError(w, http.StatusBadRequest, "validation_error", "entries must not be empty")
		return
	}
	if len(in.Entries) > 500 {
		WriteError(w, http.StatusBadRequest, "validation_error", "entries capped at 500 per request")
		return
	}
	switch in.OnDuplicate {
	case "", "skip", "replace":
	default:
		WriteError(w, http.StatusBadRequest, "validation_error", "on_duplicate must be \"skip\" or \"replace\"")
		return
	}
	at, err := a.parseDate(in.Date)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	if err := a.rejectFuture(at); err != nil {
		WriteDomainError(w, err)
		return
	}
	at = a.stampNow(at)
	month := domain.MonthOf(at, a.loc())

	seen := map[int32]bool{}
	for _, row := range in.Entries {
		if row.AssetID == 0 {
			WriteError(w, http.StatusBadRequest, "validation_error", "every entry needs an asset_id")
			return
		}
		if seen[row.AssetID] {
			WriteError(w, http.StatusBadRequest, "validation_error",
				"duplicate asset_id in entries", strconv.Itoa(int(row.AssetID)))
			return
		}
		seen[row.AssetID] = true
	}

	var written, replaced, paymentsWritten int
	skipped := []skippedEntry{}
	duplicates := []duplicateEntry{}

	err = repos.Tx(r.Context(), func(tx domain.Repos) error {
		for _, row := range in.Entries {
			asset, err := tx.Assets().Get(r.Context(), row.AssetID)
			if err != nil {
				return err
			}
			if asset.Closed {
				skipped = append(skipped, skippedEntry{AssetID: row.AssetID, Reason: "asset is closed"})
				continue
			}
			existing, err := tx.Logs().FindByAssetMonth(r.Context(), row.AssetID, month)
			switch {
			case err == nil:
				if in.OnDuplicate == "replace" {
					existing.At = at
					existing.Deposit = row.Deposit
					existing.Value = row.Value
					if _, err := tx.Logs().Update(r.Context(), existing); err != nil {
						return err
					}
					replaced++
					duplicates = append(duplicates, duplicateEntry{AssetID: row.AssetID, ExistingID: existing.ID, ActionTaken: "replaced"})
				} else {
					duplicates = append(duplicates, duplicateEntry{AssetID: row.AssetID, ExistingID: existing.ID, ActionTaken: "skipped"})
				}
			case errors.Is(err, domain.ErrNotFound):
				if _, err := tx.Logs().Create(r.Context(), domain.LogEntry{
					AssetID: row.AssetID, At: at, Deposit: row.Deposit, Value: row.Value,
				}); err != nil {
					return err
				}
				written++
			default:
				return err
			}
			if row.Payment != nil && *row.Payment != 0 {
				if _, err := tx.Payments().Create(r.Context(), domain.Payment{
					AssetID: row.AssetID, At: at, Amount: *row.Payment,
				}); err != nil {
					return err
				}
				paymentsWritten++
			}
		}
		return nil
	})
	if err != nil {
		WriteDomainError(w, err)
		return
	}

	WriteData(w, http.StatusOK, map[string]any{
		"date":             in.Date,
		"month":            month.Key(),
		"written":          written,
		"replaced":         replaced,
		"payments_written": paymentsWritten,
		"skipped":          skipped,
		"duplicates":       duplicates,
	}, nil)
}

func (a *API) handleLogsGet(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	entry, err := repos.Logs().Get(r.Context(), id)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, entry, nil)
}

func (a *API) handleLogsUpdate(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	var in logIn
	if err := DecodeJSON(r, &in); err != nil {
		WriteDomainError(w, err)
		return
	}
	current, err := repos.Logs().Get(r.Context(), id)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	if in.AssetID != 0 {
		current.AssetID = in.AssetID
	}
	if in.Date != "" {
		at, err := a.parseDate(in.Date)
		if err != nil {
			WriteDomainError(w, err)
			return
		}
		if err := a.rejectFuture(at); err != nil {
			WriteDomainError(w, err)
			return
		}
		current.At = a.stampNow(at)
	}
	if in.Deposit != nil {
		current.Deposit = *in.Deposit
	}
	if in.Value != nil {
		current.Value = *in.Value
	}
	updated, err := repos.Logs().Update(r.Context(), current)
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, updated, nil)
}

func (a *API) handleLogsDelete(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	if err := repos.Logs().Delete(r.Context(), id); err != nil {
		WriteDomainError(w, err)
		return
	}
	WriteData(w, http.StatusOK, map[string]any{"deleted": true, "id": id}, nil)
}

func (a *API) handleLogsLatest(w http.ResponseWriter, r *http.Request) {
	repos := a.reposOr500(w)
	if repos == nil {
		return
	}
	assets, _, err := repos.Assets().List(r.Context(), domain.AssetFilter{PerPage: 500, Sort: "id"})
	if err != nil {
		WriteDomainError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(assets))
	for _, as := range assets {
		logs, _, err := repos.Logs().List(r.Context(), domain.LogFilter{AssetID: as.ID, PerPage: 1})
		if err != nil {
			WriteDomainError(w, err)
			return
		}
		row := map[string]any{
			"asset_id":    as.ID,
			"description": as.Description,
			"closed":      as.Closed,
		}
		if len(logs) > 0 {
			e := logs[0]
			row["id"] = e.ID
			row["at"] = e.At
			row["deposit"] = e.Deposit
			row["value"] = e.Value
		}
		out = append(out, row)
	}
	WriteData(w, http.StatusOK, out, nil)
}
