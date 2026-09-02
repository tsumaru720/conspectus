package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"conspectus/internal/render"
	"conspectus/internal/views"
)

/* The ledger and asset/class pages are core data management: viewing, adding
 * and editing logs and payments. The analysis blocks that also live on the
 * asset/class pages (KPIs, charts, income roll-ups) are contributed by
 * internal/views through the vars providers and page blocks. */

var perPageChoices = []string{"10", "20", "50", "100", "200", "all"}

type ledgerLog struct {
	ID      int32  `json:"id"`
	At      string `json:"at"`
	AssetID int32  `json:"asset_id"`
	Deposit string `json:"deposit"`
	Value   string `json:"value"`
}

type ledgerPayment struct {
	ID      int32  `json:"id"`
	At      string `json:"at"`
	AssetID int32  `json:"asset_id"`
	Amount  string `json:"amount"`
}

type apiClass struct {
	ID          int32  `json:"id"`
	Description string `json:"description"`
}

func uiCookieValues(r *http.Request) url.Values {
	return render.UICookieValues(r)
}

func perPageOf(v url.Values) int {
	switch v.Get("per_page") {
	case "10":
		return 10
	case "50":
		return 50
	case "100":
		return 100
	case "200":
		return 200
	case "all":
		return 1000000
	default:
		return 20
	}
}

func perPageOfPath(r *http.Request) int {
	if v := r.PathValue("per"); v != "" {
		return perPageOf(url.Values{"per_page": {v}})
	}
	return perPageOf(uiCookieValues(r))
}

func pathInt(r *http.Request, key string, def int) int {
	if v := r.PathValue(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func pathID32(r *http.Request) (int32, error) {
	v := r.PathValue("id")
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid id %q", v)
	}
	return int32(n), nil
}

func (p *Pages) wireDate(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return rfc3339
	}
	return t.In(p.Loc).Format("2006-01-02")
}

func (p *Pages) today() string {
	return p.now().In(p.Loc).Format("2006-01-02")
}

func listQS(path string, ui url.Values, page, perPage int) string {
	q := url.Values{}
	for _, k := range []string{"q", "from", "to"} {
		if v := ui.Get(k); v != "" {
			q.Set(k, v)
		}
	}
	q.Set("page", strconv.Itoa(page))
	if perPage > 0 {
		q.Set("per_page", strconv.Itoa(perPage))
	}
	q.Set("sort", "-epoch")
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + q.Encode()
}

func ledgerPath(logPage, payPage int) string {
	if logPage <= 1 && payPage <= 1 {
		return "/ledger"
	}
	if payPage <= 1 {
		return "/ledger/logs/" + strconv.Itoa(logPage)
	}
	if logPage <= 1 {
		return "/ledger/pay/" + strconv.Itoa(payPage)
	}
	return "/ledger/logs/" + strconv.Itoa(logPage) + "/pay/" + strconv.Itoa(payPage)
}

func pagerData(key string, page, totalPages, totalItems int,
	endpoint, target, kind, perPage, urlTpl string, pageURL func(int) string) map[string]any {
	start := max(page-2, 1)
	end := min(page+2, totalPages)
	win := make([]any, 0, end-start+1)
	for i := start; i <= end; i++ {
		win = append(win, i)
	}
	choices := make([]any, 0, len(perPageChoices))
	for _, c := range perPageChoices {
		choices = append(choices, c)
	}
	return map[string]any{
		"Key":        key,
		"Page":       page,
		"TotalPages": totalPages,
		"Total":      totalItems,
		"PageURL":    pageURL,
		"Window":     win,
		"ShowFirst":  start > 1,
		"ShowLast":   end < totalPages,
		"Endpoint":   endpoint,
		"Target":     target,
		"Kind":       kind,
		"URLTpl":     urlTpl,
		"PerPage":    perPage,
		"Choices":    choices,
	}
}

func (p *Pages) assetNames(ctx context.Context) map[string]string {
	out := map[string]string{}
	assets, err := p.apiListAllAssets(ctx)
	if err != nil {
		return out
	}
	for _, a := range assets {
		out[strconv.Itoa(int(a.ID))] = a.Description
	}
	return out
}

func (p *Pages) ledgerPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ui := uiCookieValues(r)
	page := pathInt(r, "n", 1)
	payPage := pathInt(r, "m", 1)
	pp := perPageOf(ui)

	var logs []ledgerLog
	logTotal, err := p.API.GetList(ctx, listQS("/api/v1/logs", ui, page, pp), &logs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var payments []ledgerPayment
	payTotal, err := p.API.GetList(ctx, listQS("/api/v1/payments", ui, payPage, pp), &payments)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// One asset-list fetch feeds both the row name lookup and the quick-pay
	// dropdown (open assets, alphabetical).
	names := map[string]string{}
	payAssets := []map[string]any{}
	if all, err := p.apiListAllAssets(ctx); err == nil {
		for _, a := range all {
			names[strconv.Itoa(int(a.ID))] = a.Description
			if !a.Closed {
				payAssets = append(payAssets, map[string]any{"ID": a.ID, "Name": a.Description})
			}
		}
	}

	logRows := make([]map[string]any, 0, len(logs))
	for _, e := range logs {
		logRows = append(logRows, map[string]any{
			"ID":          e.ID,
			"Date":        p.wireDate(e.At),
			"Description": names[strconv.Itoa(int(e.AssetID))],
			"Deposit":     e.Deposit,
			"Value":       e.Value,
		})
	}
	payRows := make([]map[string]any, 0, len(payments))
	for _, pm := range payments {
		payRows = append(payRows, map[string]any{
			"ID":          pm.ID,
			"Date":        p.wireDate(pm.At),
			"Description": names[strconv.Itoa(int(pm.AssetID))],
			"Amount":      pm.Amount,
		})
	}

	returnPath := ledgerPath(page, payPage)

	ppStr := ui.Get("per_page")
	logTpl := "/ledger/logs/PAGE"
	if payPage > 1 {
		logTpl = "/ledger/logs/PAGE/pay/" + strconv.Itoa(payPage)
	}
	payTpl := "/ledger/pay/PAGE"
	if page > 1 {
		payTpl = "/ledger/logs/" + strconv.Itoa(page) + "/pay/PAGE"
	}
	vars := map[string]any{
		"Title":        "Ledger",
		"Logs":         logRows,
		"Payments":     payRows,
		"LogsMeta":     map[string]any{"total": logTotal},
		"PaymentsMeta": map[string]any{"total": payTotal},
		"LogPager": pagerData("page", page, (logTotal+pp-1)/pp, logTotal,
			"/api/ui/v1/logs", "logs-tbody", "log", ppStr, logTpl,
			func(n int) string { return ledgerPath(n, payPage) }),
		"PayPager": pagerData("paypage", payPage, (payTotal+pp-1)/pp, payTotal,
			"/api/ui/v1/payments", "payments-tbody", "payment", ppStr, payTpl,
			func(n int) string { return ledgerPath(page, n) }),
		"LogReturn": returnPath,
		"PayReturn": returnPath,
		"PayAssets": payAssets,
		"PayDate":   p.today(),
	}
	if err := p.Render.RenderPage(w, r, "ledger", vars); err != nil {
		p.Log.Error("ledger render failed", "error", err)
	}
}

func (p *Pages) scopeVars(r *http.Request) map[string]any {
	api := views.API{Client: p.API.StdClient(), Base: p.API.Base()}
	return views.ScopeVars(api, r)
}

func scopeQS(kind string, id int32, ui url.Values) string {
	q := url.Values{}
	q.Set(kind+"_id", strconv.FormatInt(int64(id), 10))
	for _, k := range []string{"from", "to"} {
		if v := ui.Get(k); v != "" {
			q.Set(k, v)
		}
	}
	return "?" + q.Encode()
}

func payPager(key string, page, total, pp int, base, endpoint, target, kind, perPage string) map[string]any {
	payURL := func(n int) string {
		if n <= 1 {
			return base
		}
		return base + "/pay/" + strconv.Itoa(n)
	}
	return pagerData(key, page, (total+pp-1)/pp, total, endpoint, target, kind, perPage, base+"/pay/PAGE", payURL)
}

func (p *Pages) classByID(ctx context.Context, id int32) (map[string]any, bool) {
	var cls apiClass
	if err := p.API.Get(ctx, "/api/v1/classes/"+strconv.FormatInt(int64(id), 10), &cls); err != nil || cls.ID == 0 {
		return nil, false
	}
	return map[string]any{"id": cls.ID, "description": cls.Description}, true
}

func (p *Pages) assetPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := pathID32(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var asset apiAssetRow
	if err := p.API.Get(ctx, "/api/v1/assets/"+strconv.FormatInt(int64(id), 10), &asset); err != nil {
		http.Error(w, "asset not found", http.StatusNotFound)
		return
	}
	className := ""
	classID := asset.ClassID
	if classID > 0 {
		if cls, ok := p.classByID(ctx, classID); ok {
			className, _ = cls["description"].(string)
		}
	}

	ui := uiCookieValues(r)
	payPage := pathInt(r, "n", 1)
	pp := perPageOf(ui)
	var payments []ledgerPayment
	payTotal, perr := p.API.GetList(ctx, listQS("/api/v1/payments"+scopeQS("asset", id, url.Values{}), url.Values{}, payPage, pp), &payments)
	payRows := []map[string]any{}
	if perr == nil {
		for _, pm := range payments {
			payRows = append(payRows, map[string]any{"Date": p.wireDate(pm.At), "Amount": pm.Amount})
		}
	}

	vars := map[string]any{
		"Title":     asset.Description,
		"Asset":     map[string]any{"id": asset.ID, "description": asset.Description, "closed": asset.Closed},
		"ClassName": className,
		"ClassID":   classID,
		"Scope":     p.scopeVars(r),
		"Payments":  payRows,
		"PayPager": payPager("paypage", payPage, payTotal, pp,
			"/asset/"+strconv.FormatInt(int64(id), 10),
			"/api/ui/v1/assets/"+strconv.FormatInt(int64(id), 10)+"/payments",
			"asset-payments-tbody", "pay-asset", ui.Get("per_page")),
		"Today": p.today(),
	}
	if err := p.Render.RenderPage(w, r, "asset", vars); err != nil {
		p.Log.Error("asset render failed", "error", err)
	}
}

func (p *Pages) classPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := pathID32(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cls, ok := p.classByID(ctx, id)
	if !ok {
		http.Error(w, "class not found", http.StatusNotFound)
		return
	}

	ui := uiCookieValues(r)
	payPage := pathInt(r, "n", 1)
	pp := perPageOf(ui)
	var payments []ledgerPayment
	payTotal, perr := p.API.GetList(ctx, listQS("/api/v1/payments"+scopeQS("class", id, url.Values{}), url.Values{}, payPage, pp), &payments)
	payRows := []map[string]any{}
	names := p.assetNames(ctx)
	if perr == nil {
		for _, pm := range payments {
			payRows = append(payRows, map[string]any{
				"Date":   p.wireDate(pm.At),
				"Asset":  names[strconv.Itoa(int(pm.AssetID))],
				"Amount": pm.Amount,
			})
		}
	}

	vars := map[string]any{
		"Title":    cls["description"],
		"Class":    cls,
		"Scope":    p.scopeVars(r),
		"Payments": payRows,
		"PayPager": payPager("paypage", payPage, payTotal, pp,
			"/class/"+strconv.FormatInt(int64(id), 10),
			"/api/ui/v1/classes/"+strconv.FormatInt(int64(id), 10)+"/payments",
			"class-payments-tbody", "pay-class", ui.Get("per_page")),
	}
	if err := p.Render.RenderPage(w, r, "class", vars); err != nil {
		p.Log.Error("class render failed", "error", err)
	}
}

/* ---- JSON table feeds for the fetch-based pagination (app.js) ---- */

func (p *Pages) uiLogs(w http.ResponseWriter, r *http.Request) {
	ui := uiCookieValues(r)
	page := pathInt(r, "n", 1)
	perPage := perPageOfPath(r)
	var logs []ledgerLog
	total, err := p.API.GetList(r.Context(), listQS("/api/v1/logs", ui, page, perPage), &logs)
	if err != nil {
		writeFeedError(w, err)
		return
	}
	names := p.assetNames(r.Context())
	rows := make([]map[string]any, 0, len(logs))
	for _, e := range logs {
		rows = append(rows, map[string]any{
			"id": e.ID, "date": p.wireDate(e.At),
			"asset":   names[strconv.Itoa(int(e.AssetID))],
			"deposit": e.Deposit, "value": e.Value,
		})
	}
	writeFeed(w, rows, page, (total+perPage-1)/perPage, total)
}

func (p *Pages) uiPayments(w http.ResponseWriter, r *http.Request) {
	ui := uiCookieValues(r)
	page := pathInt(r, "n", 1)
	perPage := perPageOfPath(r)
	path := "/api/v1/payments"
	if v := r.PathValue("id"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			kind := "class"
			if strings.Contains(r.URL.Path, "/assets/") {
				kind = "asset"
			}
			path = path + "?" + kind + "_id=" + strconv.Itoa(n)
		}
	}
	var payments []ledgerPayment
	total, err := p.API.GetList(r.Context(), listQS(path, ui, page, perPage), &payments)
	if err != nil {
		writeFeedError(w, err)
		return
	}
	names := p.assetNames(r.Context())
	rows := make([]map[string]any, 0, len(payments))
	for _, pm := range payments {
		rows = append(rows, map[string]any{
			"id": pm.ID, "date": p.wireDate(pm.At),
			"asset":  names[strconv.Itoa(int(pm.AssetID))],
			"amount": pm.Amount,
		})
	}
	writeFeed(w, rows, page, (total+perPage-1)/perPage, total)
}

func writeFeed(w http.ResponseWriter, rows []map[string]any, page, pages, total int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{"rows": rows, "page": page, "pages": pages, "total": total},
	})
}

func writeFeedError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"code": "internal", "message": err.Error()},
	})
}
