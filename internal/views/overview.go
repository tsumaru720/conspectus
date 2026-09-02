package views

import (
	"maps"
	"net/http"
	"strconv"
)

func (v *views) overviewPage(w http.ResponseWriter, r *http.Request) {
	// A remembered scope takes the visitor straight to that asset/class
	// page - the dedicated page IS the scoped overview; clear the scope to
	// see the whole portfolio again.
	if target := v.overviewRedirect(r); target != "" {
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	vars, err := v.overviewVars(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if vars == nil {
		vars = map[string]any{}
	}
	vars["Title"] = "Overview"
	if err := v.deps.Render.RenderPage(w, r, "overview", vars); err != nil {
		v.deps.Log.Error("page render failed", "page", "overview", "error", err)
	}
}

// overviewRedirect returns the /asset/{id} or /class/{id} page to show
// instead of the overview when the browser remembers a scope that still
// exists; a stale memory (deleted asset/class) falls back to the overview.
func (v *views) overviewRedirect(r *http.Request) string {
	kind, id := rememberedScope(r)
	if id <= 0 {
		return ""
	}
	path := "/api/v1/assets/" + strconv.Itoa(id)
	if kind == "class" {
		path = "/api/v1/classes/" + strconv.Itoa(id)
	}
	if _, err := v.api.GetData(r.Context(), path); err != nil {
		return ""
	}
	return "/" + kind + "/" + strconv.Itoa(id)
}

func (v *views) overviewVars(r *http.Request) (map[string]any, error) {
	ctx := r.Context()
	qs := StatsQS(r)
	o, err := v.api.GetData(ctx, "/api/v1/stats/overview"+qs)
	if err != nil {
		return nil, err
	}
	sp := SparklineOf(o)
	months := ToStrings(sp["months"])
	values := ToFloats(sp["value"])
	deposits := ToFloats(sp["deposits"])

	if granularity := UIValues(r).Get("granularity"); granularity != "" {
		months, values = Regroup(months, values, granularity, false)
		months, deposits = Regroup(months, deposits, granularity, false)
	}

	retSeries := v.api.Series(ctx, qs, "return")
	twrr := v.api.Series(ctx, qs, "twrr")
	twrrAdj := v.api.Series(ctx, qs, "twrr_adj")
	paySeries := v.api.Series(ctx, qs, "payments")
	decomp := v.api.Series(ctx, qs, "decomposition")
	returnChart, depositsChart := MonthlyChangeCharts(decomp)
	twrrDatasets := []map[string]any{
		Dataset("TWRR", ToSubOne(twrr["data"])),
		Dataset("TWRR (income-aware)", ToSubOne(twrrAdj["data"])),
	}

	last := ""
	if len(months) > 0 {
		last, _ = months[len(months)-1].(string)
	}
	presets := RangePresets(last)

	return map[string]any{
		"O":     o,
		"Scope": ScopeVars(v.api, r),
		"Charts": map[string]any{
			"ValueVsDeposits":  Chart(months, Dataset("Value", values), Dataset("Deposits", deposits)),
			"Return":           Chart(retSeries["months"], Dataset("Return (value − deposits)", retSeries["data"])),
			"TWRR":             Chart(twrr["months"], twrrDatasets...),
			"IncomeCumulative": Chart(ToStrings(paySeries["months"]), Dataset("Cumulative income", Cumulative(paySeries["data"]))),
			"MonthlyReturn":    returnChart,
			"MonthlyDeposits":  depositsChart,
		},
		"RangePresets": presets,
		"RangeActive":  RangeActive(UIValues(r), presets),
	}, nil
}

func (v *views) provideAsset(r *http.Request, vars map[string]any) {
	id, err := PathID(r)
	if err != nil {
		return
	}
	mergeVars(vars, AssetPageVars(v.api, r, "asset", id))
}

func (v *views) provideOverviewClass(r *http.Request, vars map[string]any) {
	id, err := PathID(r)
	if err != nil {
		return
	}
	mergeVars(vars, AssetPageVars(v.api, r, "class", id))
}

func mergeVars(dst map[string]any, src map[string]any) {
	maps.Copy(dst, src)
}
