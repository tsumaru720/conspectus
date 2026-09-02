// Package views holds the analysis pages (overview, breakdown, analytics,
// projections) and the shared view-composition helpers they use.
// The pages fetch their data over the JSON API (loopback or remote) and
// render through the render engine; the analysis blocks they contribute to
// the core asset/class/manage pages arrive via the engine's vars providers.
package views

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"conspectus/internal/render"
)

/* ---- request helpers ---- */

// UIValues decodes the conspectus_ui cookie (the server-visible mirror of the
// localStorage UI state: range, horizon, window, search, per-page).
func UIValues(r *http.Request) url.Values {
	return render.UICookieValues(r)
}

// PathID extracts a positive {id} path value.
func PathID(r *http.Request) (int32, error) {
	v := r.PathValue("id")
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid id %q", v)
	}
	return int32(n), nil
}

// ScopeParam renders "asset_id=3"-style scope query fragments.
func ScopeParam(kind string, id int32) string {
	return kind + "_id=" + strconv.FormatInt(int64(id), 10)
}

// RangeQS encodes the range subset (from/to/granularity/closed) of UI values.
func RangeQS(ui url.Values) string {
	q := url.Values{}
	for _, k := range []string{"from", "to", "granularity", "closed"} {
		if v := ui.Get(k); v != "" {
			q.Set(k, v)
		}
	}
	return q.Encode()
}

// pathScope reads the asset/class scope segment of the request path
// (/asset/{id}, /class/{id}). kind is "" when the path carries none.
func pathScope(r *http.Request) (kind string, id int) {
	v := r.PathValue("id")
	if v == "" {
		return "", 0
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return "", 0
	}
	if strings.Contains(r.URL.Path, "/asset/") {
		return "asset", n
	}
	if strings.Contains(r.URL.Path, "/class/") {
		return "class", n
	}
	return "", 0
}

// rememberedScope reads the scope the browser last looked at, mirrored by
// app.js into the UI cookie (scope.asset_id / scope.class_id). kind is ""
// when nothing is remembered.
func rememberedScope(r *http.Request) (kind string, id int) {
	ui := UIValues(r)
	if v := ui.Get("scope.asset_id"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return "asset", n
		}
	}
	if v := ui.Get("scope.class_id"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return "class", n
		}
	}
	return "", 0
}

// RequestScope resolves the scope a browse-picker page renders with: the
// path wins; otherwise the remembered scope applies.
func RequestScope(r *http.Request) (kind string, id int) {
	if kind, id := pathScope(r); kind != "" {
		return kind, id
	}
	return rememberedScope(r)
}

// StatsQS builds the query string a page passes to the stats API: the range
// from the UI cookie plus the asset/class scope from the request path.
// Returns "" or "?params".
func StatsQS(r *http.Request) string {
	kind, id := pathScope(r)
	return statsQS(r, kind, id)
}

// ScopedStatsQS is StatsQS for the browse-picker pages: when the path
// carries no scope, the remembered scope applies.
func ScopedStatsQS(r *http.Request) string {
	kind, id := RequestScope(r)
	return statsQS(r, kind, id)
}

func statsQS(r *http.Request, kind string, id int) string {
	q := url.Values{}
	for _, k := range []string{"from", "to", "granularity", "closed"} {
		if v := UIValues(r).Get(k); v != "" {
			q.Set(k, v)
		}
	}
	if id > 0 {
		q.Set(kind+"_id", strconv.Itoa(id))
	}
	if enc := q.Encode(); enc != "" {
		return "?" + enc
	}
	return ""
}

// ProjectionsQS is StatsQS plus the horizon/window pickers. The scope
// falls back to the remembered one when the path carries none.
func ProjectionsQS(r *http.Request) string {
	ui := UIValues(r)
	q := url.Values{}
	for _, k := range []string{"from", "to", "granularity", "closed"} {
		if v := ui.Get(k); v != "" {
			q.Set(k, v)
		}
	}
	if hz := ui.Get("horizon"); hz != "" {
		if n, err := strconv.Atoi(hz); err == nil && n > 0 && n <= 600 {
			q.Set("horizon", strconv.Itoa(n))
		}
	}
	if win := ui.Get("window"); win != "" {
		if n, err := strconv.Atoi(win); err == nil && n > 0 {
			q.Set("window", strconv.Itoa(n))
		}
	}
	if kind, id := RequestScope(r); id > 0 {
		q.Set(kind+"_id", strconv.Itoa(id))
	}
	if enc := q.Encode(); enc != "" {
		return "?" + enc
	}
	return ""
}

/* ---- API composition over the loopback/remote client ---- */

// API is a tiny JSON-envelope client. The views package constructs it once
// at registration time from the app's API client (loopback or remote).
type API struct {
	Client *http.Client
	Base   string
}

func (a API) client() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return http.DefaultClient
}

func (a API) Get(ctx context.Context, path string) (any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.Base+path, nil)
	if err != nil {
		return nil, err
	}
	return a.do(req)
}

func (a API) do(req *http.Request) (any, error) {
	resp, err := a.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var env map[string]any
	if uerr := json.Unmarshal(body, &env); uerr != nil {
		return nil, fmt.Errorf("api %s: status %d (undecodable response)", req.URL.Path, resp.StatusCode)
	}
	if e, ok := env["error"].(map[string]any); ok {
		msg, _ := e["message"].(string)
		if msg == "" {
			msg = "request failed"
		}
		return nil, errors.New(msg)
	}
	return env["data"], nil
}

// GetData fetches a JSON object envelope; errors degrade to an empty map only
// where the caller asks for it (GetDataQuiet).
func (a API) GetData(ctx context.Context, path string) (map[string]any, error) {
	v, err := a.Get(ctx, path)
	if err != nil {
		return nil, err
	}
	return MapOf(v), nil
}

// GetDataQuiet is GetData with a nil-safe empty-map fallback for optional cards.
func (a API) GetDataQuiet(ctx context.Context, path string) map[string]any {
	m, err := a.GetData(ctx, path)
	if err != nil {
		return map[string]any{}
	}
	return m
}

// GetEnvelope fetches a list envelope, returning rows plus the meta total.
func (a API) GetEnvelope(ctx context.Context, path string) (list []any, total int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.Base+path, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := a.client().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	var env map[string]any
	if uerr := json.Unmarshal(body, &env); uerr != nil {
		return nil, 0, fmt.Errorf("api %s: status %d (undecodable response)", req.URL.Path, resp.StatusCode)
	}
	if e, ok := env["error"].(map[string]any); ok {
		msg, _ := e["message"].(string)
		if msg == "" {
			msg = "request failed"
		}
		return nil, 0, errors.New(msg)
	}
	list, _ = env["data"].([]any)
	if list == nil {
		list = []any{}
	}
	if meta, ok := env["meta"].(map[string]any); ok {
		total = int(NumOf(meta["total"]))
	}
	return list, total, nil
}

// Series fetches one metric from /api/v1/stats/series. qs may carry a leading
// '?' (it is normalised). Errors degrade to empty months/data so a single
// failing series cannot take the page down.
func (a API) Series(ctx context.Context, qs, metric string) map[string]any {
	qs = strings.TrimPrefix(qs, "?")
	sep := ""
	if qs != "" {
		sep = "&"
	}
	res := a.GetDataQuiet(ctx, "/api/v1/stats/series?metric="+url.QueryEscape(metric)+sep+qs)
	if res == nil {
		res = map[string]any{}
	}
	if _, ok := res["months"]; !ok {
		res["months"] = []any{}
	}
	if _, ok := res["data"]; !ok {
		res["data"] = []any{}
	}
	return res
}

/* ---- conversions (JSON maps arrive with float64/string/bool values) ---- */

func MapOf(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}

func NumOf(v any) float64 {
	f, _ := v.(float64)
	return f
}

func IntOf(v any) int { return int(NumOf(v)) }

func StrOf(v any) string {
	s, _ := v.(string)
	return s
}

func BoolOf(v any) bool {
	b, _ := v.(bool)
	return b
}

func AsFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

func ToFloats(v any) []any {
	if list, ok := v.([]any); ok {
		return list
	}
	if f64, ok := v.([]float64); ok {
		out := make([]any, 0, len(f64))
		for _, x := range f64 {
			out = append(out, x)
		}
		return out
	}
	if ints, ok := v.([]int); ok {
		out := make([]any, 0, len(ints))
		for _, x := range ints {
			out = append(out, float64(x))
		}
		return out
	}
	return []any{}
}

func ToStrings(v any) []any {
	if list, ok := v.([]any); ok {
		return list
	}
	if ss, ok := v.([]string); ok {
		out := make([]any, 0, len(ss))
		for _, x := range ss {
			out = append(out, x)
		}
		return out
	}
	return []any{}
}

/* ---- chart payload builders ---- */

// Chart assembles a chart JSON payload (labels + datasets).
func Chart(labels any, datasets ...map[string]any) map[string]any {
	ds := make([]any, 0, len(datasets))
	for _, d := range datasets {
		ds = append(ds, d)
	}
	return map[string]any{"labels": labels, "datasets": ds}
}

// Dataset is the plain form; per-series styling keys (dashed, band, filled,
// colorIndex, stack, width) are added by the caller as extra map entries.
func Dataset(label string, data any) map[string]any {
	return map[string]any{"label": label, "data": data}
}

// ToSubOne turns growth-index series into return-from-zero series (1.19 → 0.19).
func ToSubOne(v any) []any {
	src := ToFloats(v)
	out := make([]any, 0, len(src))
	for _, x := range src {
		if f, ok := x.(float64); ok {
			out = append(out, f-1)
		} else {
			out = append(out, 0.0)
		}
	}
	return out
}

// Cumulative returns the running total of a series.
func Cumulative(v any) []any {
	src := ToFloats(v)
	out := make([]any, 0, len(src))
	total := 0.0
	for _, x := range src {
		if f, ok := x.(float64); ok {
			total += f
		}
		out = append(out, total)
	}
	return out
}

// AnyNonZero reports whether any point of the series is non-zero.
func AnyNonZero(v any) bool {
	for _, x := range ToFloats(v) {
		if f, ok := x.(float64); ok && f != 0 {
			return true
		}
	}
	return false
}

// Regroup buckets monthly series into quarter/year groups (sum=true adds the
// values, otherwise the last value of each bucket wins).
func Regroup(months []any, vals []any, g string, sum bool) ([]any, []any) {
	if g != "quarter" && g != "year" {
		return months, vals
	}
	var keys []string
	acc := map[string]any{}
	for i, mo := range months {
		ms, _ := mo.(string)
		if len(ms) < 7 {
			continue
		}
		y := ms[:4]
		mn := 0
		for _, c := range ms[5:7] {
			if c >= '0' && c <= '9' {
				mn = mn*10 + int(c-'0')
			}
		}
		key := y
		if g == "quarter" {
			key = y + "-Q" + strconv.Itoa((mn-1)/3+1)
		}
		if _, seen := acc[key]; !seen {
			keys = append(keys, key)
		}
		if sum {
			prev := 0.0
			if p, ok := acc[key].(float64); ok {
				prev = p
			}
			cur := 0.0
			if f, ok := vals[i].(float64); ok {
				cur = f
			}
			acc[key] = prev + cur
		} else {
			acc[key] = vals[i]
		}
	}
	outLabels := make([]any, 0, len(keys))
	outVals := make([]any, 0, len(keys))
	for _, k := range keys {
		outLabels = append(outLabels, k)
		outVals = append(outVals, acc[k])
	}
	return outLabels, outVals
}

// MonthlyChangeCharts builds the Δ return and Δ deposits bar charts from the
// stats decomposition series. That series diffs each month against the real
// prior month and omits the opening month of an unbounded range, where the
// delta would be the initial deposit seed rather than a change.
func MonthlyChangeCharts(decomp map[string]any) (returnChart, depositsChart map[string]any) {
	months := ToStrings(decomp["months"])
	return Chart(months, Dataset("Δ return", ToFloats(decomp["growth"]))),
		Chart(months, Dataset("Δ deposits", ToFloats(decomp["new_money"])))
}

// YearEndBuckets takes the last value of each year from a monthly series.
func YearEndBuckets(months []any, vals []any) ([]any, []any) {
	var labels, out []any
	curYear := ""
	curVal := any(0.0)
	for i, mo := range months {
		ms, _ := mo.(string)
		if len(ms) < 4 {
			continue
		}
		y := ms[:4]
		if y != curYear {
			if curYear != "" {
				labels = append(labels, curYear)
				out = append(out, curVal)
			}
			curYear = y
		}
		if i < len(vals) {
			curVal = vals[i]
		}
	}
	if curYear != "" {
		labels = append(labels, curYear)
		out = append(out, curVal)
	}
	return labels, out
}

// YearEndActuals returns complete years (12 months) with their final value.
func YearEndActuals(months, values []any) (years []string, lasts []float64) {
	count := map[string]int{}
	last := map[string]float64{}
	var order []string
	for i, mo := range months {
		ms, _ := mo.(string)
		if len(ms) < 4 {
			continue
		}
		y := ms[:4]
		if _, seen := count[y]; !seen {
			order = append(order, y)
		}
		count[y]++
		if i < len(values) {
			if f, ok := values[i].(float64); ok {
				last[y] = f
			}
		}
	}
	for _, y := range order {
		if count[y] == 12 {
			years = append(years, y)
			lasts = append(lasts, last[y])
		}
	}
	return years, lasts
}

// SortedNames case-insensitively sorts map keys (deterministic chart series).
func SortedNames(m map[string]any) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	return names
}

// SparklineOf pulls the sparkline sub-map out of an overview payload.
func SparklineOf(overview map[string]any) map[string]any {
	if overview == nil {
		return map[string]any{}
	}
	if sp, ok := overview["sparkline"].(map[string]any); ok {
		return sp
	}
	return map[string]any{}
}

/* ---- range presets ---- */

// RangePresets computes the from-month for each quick-select window relative
// to the latest month in the data.
func RangePresets(lastMonth string) map[string]string {
	presets := map[string]string{"m3": "", "m6": "", "ytd": "", "y1": "", "y5": ""}
	if lastMonth == "" || len(lastMonth) != 7 {
		return presets
	}
	y, _ := strconv.Atoi(lastMonth[:4])
	m, _ := strconv.Atoi(lastMonth[5:7])
	addMonths := func(n int) string {
		t := y*12 + (m - 1) + n
		ny := t / 12
		nm := t%12 + 1
		return fmt.Sprintf("%04d-%02d", ny, nm)
	}
	presets["m3"] = addMonths(-2)
	presets["m6"] = addMonths(-5)
	presets["ytd"] = fmt.Sprintf("%04d-01", y)
	presets["y1"] = addMonths(-11)
	presets["y5"] = addMonths(-59)
	return presets
}

// RangeActive maps the current from value back to a preset key ("" = custom).
func RangeActive(v url.Values, presets map[string]string) string {
	from := v.Get("from")
	for key, val := range presets {
		if from != "" && from == val {
			switch key {
			case "m3":
				return "3m"
			case "m6":
				return "6m"
			case "ytd":
				return "ytd"
			case "y1":
				return "1y"
			case "y5":
				return "5y"
			}
		}
	}
	if from == "" {
		return "all"
	}
	return ""
}

/* ---- projections helpers ---- */

// RequiredAtYear discounts a target value back to what you'd need on track
// at the given year.
func RequiredAtYear(targetValue float64, targetYear int, rMonthly float64, y string) float64 {
	if targetValue <= 0 {
		return 0
	}
	yearNum, _ := strconv.Atoi(y)
	monthsBetween := (targetYear - yearNum) * 12
	if monthsBetween <= 0 {
		return targetValue
	}
	factor := 1.0
	for range monthsBetween {
		factor *= (1 + rMonthly)
	}
	return targetValue / factor
}

/* ---- browse picker (scope) vars ---- */

// ScopeVars builds the data for partials/browse-picker.html: the asset/class
// option lists plus the current selection derived from the request path.
func ScopeVars(api API, r *http.Request) map[string]any {
	return scopeVars(api, r, false)
}

// RememberedScopeVars is ScopeVars for the browse-picker pages: when the
// path carries no scope, the remembered one applies, so the dropdown, the
// clear link and the scope chip match what the page is scoped to.
func RememberedScopeVars(api API, r *http.Request) map[string]any {
	return scopeVars(api, r, true)
}

func scopeVars(api API, r *http.Request, remember bool) map[string]any {
	ctx := r.Context()
	assetID := 0
	classID := 0
	kind, id := pathScope(r)
	if kind == "" && remember {
		kind, id = rememberedScope(r)
	}
	if kind == "asset" {
		assetID = id
	} else if kind == "class" {
		classID = id
	}
	basePath := r.URL.Path
	if i := strings.LastIndex(basePath, "/asset/"); i >= 0 {
		basePath = basePath[:i]
	}
	if i := strings.LastIndex(basePath, "/class/"); i >= 0 {
		basePath = basePath[:i]
	}
	if basePath == "" {
		// Bare /asset/{id} pages have nothing left after stripping the scope
		// segment; clearing the selection there means going home.
		basePath = "/"
	}
	label := ""
	if assetID > 0 {
		if a, err := api.GetData(ctx, "/api/v1/assets/"+strconv.Itoa(assetID)); err == nil {
			label = StrOf(a["description"])
		}
	} else if classID > 0 {
		if c, err := api.GetData(ctx, "/api/v1/classes/"+strconv.Itoa(classID)); err == nil {
			label = StrOf(c["description"])
		}
	}
	assets, classes := scopeOptions(api, ctx)
	open := []any{}
	closed := []any{}
	for _, a := range assets {
		if m, ok := a.(map[string]any); ok {
			if isClosed, _ := m["closed"].(bool); isClosed {
				closed = append(closed, a)
			} else {
				open = append(open, a)
			}
		}
	}
	return map[string]any{
		"AssetID":      assetID,
		"ClassID":      classID,
		"Label":        label,
		"BasePath":     basePath,
		"Assets":       assets,
		"Classes":      classes,
		"AssetsOpen":   open,
		"AssetsClosed": closed,
	}
}

func scopeOptions(api API, ctx context.Context) (assets []any, classes []any) {
	assets = []any{}
	classes = []any{}
	// Keep this query identical to Pages.apiListAllAssets' first page so
	// class pages, which fetch both, share one round-trip via the request
	// cache.
	if list, _, err := api.GetEnvelope(ctx, "/api/v1/assets?page=1&per_page=500&sort=description"); err == nil {
		for _, aAny := range list {
			a := MapOf(aAny)
			assets = append(assets, map[string]any{
				"id": IntOf(a["id"]), "name": StrOf(a["description"]), "closed": BoolOf(a["closed"]),
			})
		}
	}
	if v, err := api.Get(ctx, "/api/v1/classes"); err == nil {
		list, _ := v.([]any)
		names := make([]string, 0, len(list))
		byName := map[string]int{}
		for _, cAny := range list {
			c := MapOf(cAny)
			name := StrOf(c["description"])
			names = append(names, name)
			byName[name] = IntOf(c["id"])
		}
		sort.Strings(names)
		for _, n := range names {
			classes = append(classes, map[string]any{"id": byName[n], "name": n})
		}
	}
	return assets, classes
}

/* ---- shared page-var composition ---- */

// AssetPageVars computes the analysis variables for the core asset/class
// pages (KPIs, charts, income roll-up). kind is "asset" or "class"; id the
// scope id. The core page supplies the CRUD half (payments ledger, quick-pay
// form, scope picker).
func AssetPageVars(api API, r *http.Request, kind string, id int32) map[string]any {
	ctx := r.Context()
	qs := "?" + ScopeParam(kind, id)
	if extra := RangeQS(UIValues(r)); extra != "" {
		qs = qs + "&" + extra
	}
	o := api.GetDataQuiet(ctx, "/api/v1/stats/overview"+qs)
	sp := SparklineOf(o)
	months := ToStrings(sp["months"])
	values := ToFloats(sp["value"])
	deposits := ToFloats(sp["deposits"])

	twrr := api.Series(ctx, qs, "twrr")
	twrrAdj := api.Series(ctx, qs, "twrr_adj")
	paySeries := api.Series(ctx, qs, "payments")
	retSeries := api.Series(ctx, qs, "return")
	decomp := api.Series(ctx, qs, "decomposition")

	incomeYears, incomeTotal := IncomeByYear(api, ctx, qs)

	last := ""
	if n := len(months); n > 0 {
		last, _ = months[n-1].(string)
	}
	presets := RangePresets(last)
	returnChart, depositsChart := MonthlyChangeCharts(decomp)

	return map[string]any{
		"O":            o,
		"HasPayments":  AnyNonZero(paySeries["data"]),
		"IncomeYears":  incomeYears,
		"IncomeTotal":  incomeTotal,
		"RangePresets": presets,
		"RangeActive":  RangeActive(UIValues(r), presets),
		"Charts": map[string]any{
			"ValueVsDeposits": Chart(months, Dataset("Value", values), Dataset("Deposits", deposits)),
			"Return":          Chart(ToStrings(retSeries["months"]), Dataset("Return (value − deposits)", ToFloats(retSeries["data"]))),
			"TWRR": Chart(twrr["months"],
				Dataset("TWRR", ToSubOne(twrr["data"])),
				Dataset("TWRR (income-aware)", ToSubOne(twrrAdj["data"])),
			),
			"PaymentsBar":      Chart(ToStrings(paySeries["months"]), Dataset("Payments", ToFloats(paySeries["data"]))),
			"IncomeCumulative": Chart(ToStrings(paySeries["months"]), Dataset("Cumulative income", Cumulative(paySeries["data"]))),
			"MonthlyReturn":    returnChart,
			// asset pages label the block "MonthlyChanges", class pages
			// "MonthlyFlow" - same series either way.
			"MonthlyChanges": depositsChart,
			"MonthlyFlow":    depositsChart,
		},
	}
}

// IncomeByYear fetches the per-year income roll-up, newest year first.
func IncomeByYear(api API, ctx context.Context, qs string) ([]any, float64) {
	years := []any{}
	total := 0.0
	data := api.GetDataQuiet(ctx, "/api/v1/stats/income"+qs)
	list, _ := data["by_year"].([]any)
	type yearAmount struct {
		year   int
		amount float64
	}
	rows := make([]yearAmount, 0, len(list))
	for _, yAny := range list {
		y := MapOf(yAny)
		rows = append(rows, yearAmount{IntOf(y["year"]), NumOf(y["amount"])})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].year > rows[j].year })
	for _, r := range rows {
		years = append(years, map[string]any{"Year": r.year, "Amount": r.amount})
		total += r.amount
	}
	return years, total
}
