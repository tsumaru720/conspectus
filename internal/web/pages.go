package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"conspectus/internal/apiclient"
	"conspectus/internal/domain"
	"conspectus/internal/httpapi"
	"conspectus/internal/importer"
	"conspectus/internal/render"
)

type Pages struct {
	Router   *httpapi.Router
	Render   *render.Engine
	Settings *SettingsViaAPI
	Log      *slog.Logger
	Importer *importer.Importer
	API      *apiclient.Client
	Loc      *time.Location
	Now      func() time.Time
	Version  string
}

func (p *Pages) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

// isFutureDate compares a parsed calendar date against today in the app zone.
func (p *Pages) isFutureDate(at time.Time) bool {
	today := p.now().In(p.Loc)
	return at.After(time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, p.Loc))
}

func (p *Pages) Register() error {
	reg := func(name, method, pattern string, h http.HandlerFunc) error {
		return p.Router.Register("core:"+name, method, pattern, h)
	}
	errs := []error{}
	must := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	must(reg("pages.assets", "GET", "/assets/{path...}", p.Render.ServeAsset))
	must(reg("pages.import", "GET", "/import", p.handleImport))
	must(reg("pages.import.quick", "POST", "/import/quick", p.handleImportQuick))
	must(reg("pages.import.csv.preview", "POST", "/import/csv/preview", p.handleCSVPreview))
	must(reg("pages.import.csv.commit", "POST", "/import/csv/commit", p.handleCSVCommit))
	must(reg("pages.manage", "GET", "/manage", p.handleManage))
	must(reg("pages.manage.asset.create", "POST", "/manage/asset/create", p.formAsset(p.createAsset)))
	must(reg("pages.manage.asset.update", "POST", "/manage/asset/update", p.formAsset(p.updateAsset)))
	must(reg("pages.manage.asset.close", "POST", "/manage/asset/close", p.simpleAsset("close")))
	must(reg("pages.manage.asset.reopen", "POST", "/manage/asset/reopen", p.simpleAsset("reopen")))
	must(reg("pages.manage.asset.delete", "POST", "/manage/asset/delete", p.simpleAsset("delete")))
	must(reg("pages.manage.class.create", "POST", "/manage/class/create", p.formClass("create")))
	must(reg("pages.manage.class.update", "POST", "/manage/class/update", p.formClass("update")))
	must(reg("pages.manage.class.delete", "POST", "/manage/class/delete", p.simpleClassDelete()))
	must(reg("pages.manage.log.delete", "POST", "/manage/log/delete", p.deleteRow("log")))
	must(reg("pages.manage.payment.delete", "POST", "/manage/payment/delete", p.deleteRow("payment")))
	must(reg("pages.manage.log.update", "POST", "/manage/log/update", p.updateRow("log")))
	must(reg("pages.manage.payment.update", "POST", "/manage/payment/update", p.updateRow("payment")))
	must(reg("pages.payment.quick", "POST", "/payments/quick", p.handlePaymentQuick))

	must(reg("pages.ledger", "GET", "/ledger", p.ledgerPage))
	must(reg("pages.ledger.logs", "GET", "/ledger/logs/{n}", p.ledgerPage))
	must(reg("pages.ledger.pay", "GET", "/ledger/pay/{n}", p.ledgerPage))
	must(reg("pages.ledger.both", "GET", "/ledger/logs/{n}/pay/{m}", p.ledgerPage))
	must(reg("pages.asset", "GET", "/asset/{id}", p.assetPage))
	must(reg("pages.asset.pay", "GET", "/asset/{id}/pay/{n}", p.assetPage))
	must(reg("pages.class", "GET", "/class/{id}", p.classPage))
	must(reg("pages.class.pay", "GET", "/class/{id}/pay/{n}", p.classPage))

	must(reg("ui.logs", "GET", "/api/ui/v1/logs/p/{n}", p.uiLogs))
	must(reg("ui.logs.per", "GET", "/api/ui/v1/logs/p/{n}/per/{per}", p.uiLogs))
	must(reg("ui.payments", "GET", "/api/ui/v1/payments/p/{n}", p.uiPayments))
	must(reg("ui.payments.per", "GET", "/api/ui/v1/payments/p/{n}/per/{per}", p.uiPayments))
	must(reg("ui.payments.asset", "GET", "/api/ui/v1/assets/{id}/payments/p/{n}", p.uiPayments))
	must(reg("ui.payments.asset.per", "GET", "/api/ui/v1/assets/{id}/payments/p/{n}/per/{per}", p.uiPayments))
	must(reg("ui.payments.class", "GET", "/api/ui/v1/classes/{id}/payments/p/{n}", p.uiPayments))
	must(reg("ui.payments.class.per", "GET", "/api/ui/v1/classes/{id}/payments/p/{n}/per/{per}", p.uiPayments))

	if len(errs) > 0 {
		return errs[0]
	}

	// Core navigation and core page blocks. The analysis pages (overview,
	// breakdown, analytics, projections) register their nav entries
	// and blocks from internal/views.
	p.Render.AddNav("Ledger", "/ledger", "ledger", 60)
	p.Render.AddNav("Import", "/import", "import", 70)
	p.Render.AddNav("Manage", "/manage", "manage", 80)
	// The payments ledger renders after the analysis blocks.
	p.Render.RegisterBlocks("asset", 100, "payments")
	p.Render.RegisterBlocks("class", 100, "payments")
	return nil
}

func (p *Pages) render(w http.ResponseWriter, r *http.Request, page string, vars map[string]any) {
	if vars == nil {
		vars = map[string]any{}
	}
	if err := p.Render.RenderPage(w, r, page, vars); err != nil {
		p.Log.Error("page render failed", "page", page, "error", err)
	}
}

func redirectFlash(w http.ResponseWriter, r *http.Request, to, kind, msg string) {
	http.SetCookie(w, &http.Cookie{
		Name:  render.FlashCookie,
		Value: url.QueryEscape(kind + ":" + msg),
		Path:  "/", MaxAge: 60,
	})
	http.Redirect(w, r, to, http.StatusSeeOther)
}

func (p *Pages) flashErr(w http.ResponseWriter, r *http.Request, to string, err error) {
	redirectFlash(w, r, to, "err", err.Error())
}

func (p *Pages) handleImport(w http.ResponseWriter, r *http.Request) {
	rows, err := importer.QuickRows(r.Context(), p.API, p.Loc, p.now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	today := p.now().In(p.Loc).Format("2006-01-02")
	p.render(w, r, "import", map[string]any{
		"Title":     "Import",
		"QuickRows": rows,
		"QuickDate": today,
		"CSVDate":   today,
	})
}

type quickUpdateEntry struct {
	AssetID int32        `json:"asset_id"`
	Deposit domain.Money `json:"deposit"`
	Value   domain.Money `json:"value"`
	Payment domain.Money `json:"payment,omitempty"`
}

type monthSnapshotResult struct {
	Written         int `json:"written"`
	Replaced        int `json:"replaced"`
	PaymentsWritten int `json:"payments_written"`
	Skipped         []struct {
		AssetID int32  `json:"asset_id"`
		Reason  string `json:"reason"`
	} `json:"skipped"`
	Duplicates []struct {
		AssetID int32  `json:"asset_id"`
		Action  string `json:"action"`
	} `json:"duplicates"`
}

func (p *Pages) handleImportQuick(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad form.", http.StatusBadRequest)
		return
	}
	date := r.PostFormValue("date")
	at, err := time.ParseInLocation("2006-01-02", date, p.Loc)
	if err != nil {
		redirectFlash(w, r, "/import", "err", "Invalid date.")
		return
	}
	if p.isFutureDate(at) {
		redirectFlash(w, r, "/import", "err", "Future dates are not allowed.")
		return
	}
	entries, err := importer.ParseQuickSubmission(r.PostForm)
	if err != nil {
		redirectFlash(w, r, "/import", "err", err.Error())
		return
	}
	onDup := "skip"
	if r.PostFormValue("on_duplicate") == "replace" {
		onDup = "replace"
	}
	rows := make([]quickUpdateEntry, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, quickUpdateEntry{AssetID: e.AssetID, Deposit: e.Deposit, Value: e.Value, Payment: e.Payment})
	}
	var res monthSnapshotResult
	if len(rows) > 0 {
		if err := p.API.Post(r.Context(), "/api/v1/logs", map[string]any{
			"date": date, "entries": rows, "on_duplicate": onDup,
		}, &res); err != nil {
			p.flashErr(w, r, "/import", err)
			return
		}
	}
	skipped := []map[string]any{}
	for _, s := range res.Skipped {
		skipped = append(skipped, map[string]any{"asset_id": s.AssetID, "reason": s.Reason})
	}
	dups := []map[string]any{}
	for _, d := range res.Duplicates {
		dups = append(dups, map[string]any{"asset_id": d.AssetID, "action": d.Action})
	}
	p.render(w, r, "import", map[string]any{
		"Title":     "Import",
		"QuickRows": nil,
		"QuickDate": date,
		"CSVDate":   p.now().In(p.Loc).Format("2006-01-02"),
		"QuickReport": map[string]any{
			"date": date, "written": res.Written, "replaced": res.Replaced,
			"payments_written": res.PaymentsWritten, "skipped": skipped, "duplicates": dups,
		},
	})
}

func (p *Pages) handlePaymentQuick(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		redirectFlash(w, r, "/ledger", "err", "Bad form.")
		return
	}
	to := r.PostFormValue("return")
	if to == "" || !strings.HasPrefix(to, "/") {
		to = "/ledger"
	}
	assetID, err := id32(r.PostFormValue("asset_id"))
	if err != nil {
		redirectFlash(w, r, to, "err", "Choose an asset.")
		return
	}
	amount, err := domain.ParseMoney(r.PostFormValue("amount"))
	if err != nil || amount <= 0 {
		redirectFlash(w, r, to, "err", "Enter a positive amount.")
		return
	}
	at, err := time.ParseInLocation("2006-01-02", r.PostFormValue("date"), p.Loc)
	if err != nil {
		redirectFlash(w, r, to, "err", "Invalid date.")
		return
	}
	if p.isFutureDate(at) {
		redirectFlash(w, r, to, "err", "Future dates are not allowed.")
		return
	}
	var asset struct {
		Description string `json:"description"`
		Closed      bool   `json:"closed"`
	}
	if err := p.API.Get(r.Context(), "/api/v1/assets/"+strconv.FormatInt(int64(assetID), 10), &asset); err != nil {
		p.flashErr(w, r, to, err)
		return
	}
	if asset.Closed {
		redirectFlash(w, r, to, "err", "Asset is closed.")
		return
	}
	if err := p.API.Post(r.Context(), "/api/v1/payments", map[string]any{
		"asset_id": assetID, "date": at.Format("2006-01-02"), "amount": amount.String(),
	}, nil); err != nil {
		p.flashErr(w, r, to, err)
		return
	}
	redirectFlash(w, r, to, "ok", fmt.Sprintf("Payment of %s%s recorded for %s.",
		render.CurrencySymbol("GBP"), amount.String(), asset.Description))
}

func (p *Pages) handleCSVPreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		redirectFlash(w, r, "/import", "err", "Upload failed: "+err.Error())
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		redirectFlash(w, r, "/import", "err", "Missing file.")
		return
	}
	defer file.Close()
	at, err := time.ParseInLocation("2006-01-02", r.FormValue("date"), p.Loc)
	if err != nil {
		redirectFlash(w, r, "/import", "err", "Invalid date.")
		return
	}
	if p.isFutureDate(at) {
		redirectFlash(w, r, "/import", "err", "Future dates are not allowed.")
		return
	}
	report, token, err := p.Importer.Preview(r.Context(), file, at)
	if err != nil {
		redirectFlash(w, r, "/import", "err", err.Error())
		return
	}
	p.csvPage(w, r, report, token, at)
}

func (p *Pages) handleCSVCommit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		redirectFlash(w, r, "/import", "err", "Bad form.")
		return
	}
	token := r.PostFormValue("token")
	res, err := p.Importer.Commit(r.Context(), token)
	if err != nil {
		redirectFlash(w, r, "/import", "err", err.Error())
		return
	}
	msg := fmt.Sprintf("CSV committed: %d created, %d updated, %d skipped",
		res.Created, res.Updated, res.Skipped)
	if len(res.Warnings) > 0 {
		msg += " - warning: " + strings.Join(res.Warnings, "; ")
	}
	redirectFlash(w, r, "/import", "ok", msg)
}

func (p *Pages) csvPage(w http.ResponseWriter, r *http.Request, report *importer.Report, token string, at time.Time) {
	rows := make([]map[string]any, 0, len(report.Rows))
	for _, row := range report.Rows {
		status := row.Note
		if row.Error != "" {
			status = row.Error
		}
		rows = append(rows, map[string]any{
			"line": row.Line, "id": row.AssetID, "asset": row.AssetName,
			"deposit": row.Deposit.String(), "value": row.Value.String(),
			"action": row.Action, "status": status, "error": row.Error,
		})
	}
	p.render(w, r, "import", map[string]any{
		"Title":   "Import",
		"CSVDate": at.Format("2006-01-02"),
		"CSVReport": map[string]any{
			"creates": report.Creates, "updates": report.Updates,
			"skipped": report.Skipped, "errors": report.Errors,
			"warnings": report.Warnings(),
			"rows":     rows, "commitToken": token,
		},
	})
}

type apiAssetRow struct {
	ID          int32  `json:"id"`
	ClassID     int32  `json:"class_id"`
	Description string `json:"description"`
	Closed      bool   `json:"closed"`
}

type apiClassRow struct {
	ID          int32  `json:"id"`
	Description string `json:"description"`
}

func (p *Pages) apiListAllAssets(ctx context.Context) ([]apiAssetRow, error) {
	var out []apiAssetRow
	for page := 1; ; page++ {
		var chunk []apiAssetRow
		total, err := p.API.GetList(ctx, apiclient.Query("/api/v1/assets", "per_page", 500, "page", page, "sort", "description"), &chunk)
		if err != nil {
			return nil, err
		}
		out = append(out, chunk...)
		if len(out) >= total || len(chunk) == 0 {
			break
		}
	}
	return out, nil
}

func (p *Pages) handleManage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	assets, err := p.apiListAllAssets(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	classes := []apiClassRow{}
	if err := p.API.Get(ctx, "/api/v1/classes", &classes); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Slice(assets, func(i, j int) bool {
		return strings.ToLower(assets[i].Description) < strings.ToLower(assets[j].Description)
	})

	counts := map[int32]int{}
	for _, a := range assets {
		counts[a.ClassID]++
	}
	className := map[int32]string{}
	classRows := make([]map[string]any, 0, len(classes))
	for _, c := range classes {
		className[c.ID] = c.Description
		classRows = append(classRows, map[string]any{"ID": c.ID, "Description": c.Description, "Count": counts[c.ID]})
	}
	assetRows := make([]map[string]any, 0, len(assets))
	openCount := 0
	for _, a := range assets {
		if !a.Closed {
			openCount++
		}
		assetRows = append(assetRows, map[string]any{
			"ID": a.ID, "Description": a.Description, "ClassID": a.ClassID,
			"ClassName": className[a.ClassID], "Closed": a.Closed,
		})
	}
	system := map[string]any{
		"version": p.Version, "commit": "",
		"routes": len(p.Router.Routes()),
	}
	var sys map[string]any
	if err := p.API.Get(ctx, "/api/v1/system", &sys); err == nil {
		for _, k := range []string{"schema_version", "expected_schema_version"} {
			if v, ok := sys[k]; ok {
				system[k] = v
			}
		}
	}
	if _, ok := system["schema_version"]; !ok {
		system["schema_version"] = "0"
	}

	// Settings opted in to the manage page via their display flag; an empty
	// (or failed) fetch hides the section entirely.
	display := []map[string]string{}
	for _, s := range p.Settings.Rows() {
		if s.Display {
			display = append(display, map[string]string{"description": s.Description, "value": s.Value})
		}
	}

	p.render(w, r, "manage", map[string]any{
		"Title":          "Manage",
		"Assets":         assetRows,
		"OpenCount":      openCount,
		"ClassRows":      classRows,
		"System":         system,
		"DisplaySettings": display,
	})
}

func id32(s string) (int32, error) {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("Invalid id.")
	}
	return int32(n), nil
}

type assetForm struct {
	ID, ClassID int32
	Description string
}

func (p *Pages) formAsset(fn func(w http.ResponseWriter, r *http.Request, f assetForm)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			redirectFlash(w, r, "/manage", "err", "Bad form.")
			return
		}
		f := assetForm{Description: strings.TrimSpace(r.PostFormValue("description"))}
		if v, err := id32(r.PostFormValue("id")); err == nil {
			f.ID = v
		}
		if v, err := id32(r.PostFormValue("class_id")); err == nil {
			f.ClassID = v
		}
		fn(w, r, f)
	}
}

func (p *Pages) createAsset(w http.ResponseWriter, r *http.Request, f assetForm) {
	if err := p.API.Post(r.Context(), "/api/v1/assets", map[string]any{
		"class_id": f.ClassID, "description": f.Description,
	}, nil); err != nil {
		p.flashErr(w, r, "/manage", err)
		return
	}
	redirectFlash(w, r, "/manage", "ok", "Asset created.")
}

func (p *Pages) updateAsset(w http.ResponseWriter, r *http.Request, f assetForm) {
	patch := map[string]any{"description": f.Description}
	if f.ClassID != 0 {
		patch["class_id"] = f.ClassID
	}
	if err := p.API.Patch(r.Context(), "/api/v1/assets/"+strconv.FormatInt(int64(f.ID), 10), patch, nil); err != nil {
		p.flashErr(w, r, "/manage", err)
		return
	}
	redirectFlash(w, r, "/manage", "ok", "Asset saved.")
}

func (p *Pages) simpleAsset(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id, err := id32(r.PostFormValue("id"))
		if err != nil {
			redirectFlash(w, r, "/manage", "err", "Invalid id.")
			return
		}
		idPath := strconv.FormatInt(int64(id), 10)
		msg := map[string]string{
			"close": "Asset closed.", "reopen": "Asset reopened.", "delete": "Asset deleted.",
		}[action]
		switch action {
		case "close":
			err = p.API.Post(r.Context(), "/api/v1/assets/"+idPath+"/close", nil, nil)
		case "reopen":
			err = p.API.Post(r.Context(), "/api/v1/assets/"+idPath+"/reopen", nil, nil)
		case "delete":
			err = p.API.Delete(r.Context(), "/api/v1/assets/"+idPath, nil)
			if err != nil && strings.Contains(err.Error(), "force=true") {
				redirectFlash(w, r, "/manage", "err", "Asset still has log entries/payments - delete them first (API ?force=true cascades).")
				return
			}
		}
		if err != nil {
			p.flashErr(w, r, "/manage", err)
			return
		}
		redirectFlash(w, r, "/manage", "ok", msg)
	}
}

func (p *Pages) formClass(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			redirectFlash(w, r, "/manage", "err", "Bad form.")
			return
		}
		desc := strings.TrimSpace(r.PostFormValue("description"))
		switch action {
		case "create":
			if err := p.API.Post(r.Context(), "/api/v1/classes", map[string]any{"description": desc}, nil); err != nil {
				p.flashErr(w, r, "/manage", err)
				return
			}
			redirectFlash(w, r, "/manage", "ok", "Class created.")
		case "update":
			id, err := id32(r.PostFormValue("id"))
			if err != nil {
				redirectFlash(w, r, "/manage", "err", "Invalid id.")
				return
			}
			if err := p.API.Patch(r.Context(), "/api/v1/classes/"+strconv.FormatInt(int64(id), 10), map[string]any{"description": desc}, nil); err != nil {
				p.flashErr(w, r, "/manage", err)
				return
			}
			redirectFlash(w, r, "/manage", "ok", "Class updated.")
		}
	}
}

func (p *Pages) simpleClassDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id, err := id32(r.PostFormValue("id"))
		if err != nil {
			redirectFlash(w, r, "/manage", "err", "Invalid id.")
			return
		}
		if err := p.API.Delete(r.Context(), "/api/v1/classes/"+strconv.FormatInt(int64(id), 10), nil); err != nil {
			p.flashErr(w, r, "/manage", err)
			return
		}
		redirectFlash(w, r, "/manage", "ok", "Class deleted.")
	}
}

func returnTo(v string) string {
	if v == "" || !strings.HasPrefix(v, "/") {
		return "/ledger"
	}
	return v
}

func (p *Pages) deleteRow(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id, err := id32(r.PostFormValue("id"))
		if err != nil {
			redirectFlash(w, r, "/ledger", "err", "Invalid id.")
			return
		}
		idPath := strconv.FormatInt(int64(id), 10)
		if kind == "log" {
			err = p.API.Delete(r.Context(), "/api/v1/logs/"+idPath, nil)
		} else {
			err = p.API.Delete(r.Context(), "/api/v1/payments/"+idPath, nil)
		}
		if err != nil {
			p.flashErr(w, r, "/ledger", err)
			return
		}
		redirectFlash(w, r, returnTo(r.PostFormValue("return")), "ok", "Deleted.")
	}
}

func (p *Pages) updateRow(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		to := returnTo(r.PostFormValue("return"))
		id, err := id32(r.PostFormValue("id"))
		if err != nil {
			redirectFlash(w, r, to, "err", "Invalid id.")
			return
		}
		at, err := time.ParseInLocation("2006-01-02", r.PostFormValue("date"), p.Loc)
		if err != nil {
			redirectFlash(w, r, to, "err", "Invalid date.")
			return
		}
		if p.isFutureDate(at) {
			redirectFlash(w, r, to, "err", "Future dates are not allowed.")
			return
		}
		idPath := strconv.FormatInt(int64(id), 10)
		if kind == "log" {
			deposit, err := domain.ParseMoney(r.PostFormValue("deposit"))
			if err != nil {
				redirectFlash(w, r, to, "err", "invalid deposit: "+err.Error())
				return
			}
			value, err := domain.ParseMoney(r.PostFormValue("value"))
			if err != nil {
				redirectFlash(w, r, to, "err", "invalid value: "+err.Error())
				return
			}
			if err := p.API.Patch(r.Context(), "/api/v1/logs/"+idPath, map[string]any{
				"date": at.Format("2006-01-02"), "deposit": deposit.String(), "value": value.String(),
			}, nil); err != nil {
				p.flashErr(w, r, to, err)
				return
			}
			redirectFlash(w, r, to, "ok", "Entry saved.")
			return
		}
		amount, err := domain.ParseMoney(r.PostFormValue("amount"))
		if err != nil || amount <= 0 {
			redirectFlash(w, r, to, "err", "Enter a positive amount.")
			return
		}
		if err := p.API.Patch(r.Context(), "/api/v1/payments/"+idPath, map[string]any{
			"date": at.Format("2006-01-02"), "amount": amount.String(),
		}, nil); err != nil {
			p.flashErr(w, r, to, err)
			return
		}
		redirectFlash(w, r, to, "ok", "Payment saved.")
	}
}
