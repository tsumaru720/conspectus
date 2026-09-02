package views

import (
	"fmt"
	"net/http"
	"sort"
)

func (v *views) breakdownPage(w http.ResponseWriter, r *http.Request) {
	vars, err := v.breakdownVars(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if vars == nil {
		vars = map[string]any{}
	}
	vars["Title"] = "Breakdown"
	if err := v.deps.Render.RenderPage(w, r, "breakdown", vars); err != nil {
		v.deps.Log.Error("page render failed", "page", "breakdown", "error", err)
	}
}

func (v *views) breakdownVars(r *http.Request) (map[string]any, error) {
	b, err := v.api.GetData(r.Context(), "/api/v1/stats/breakdown"+ScopedStatsQS(r))
	if err != nil {
		return nil, err
	}

	className := map[string]string{}
	if classesAny, ok := b["classes"].([]any); ok {
		for _, cAny := range classesAny {
			if c, ok := cAny.(map[string]any); ok {
				className[fmt.Sprintf("%d", IntOf(c["class_id"]))] = StrOf(c["description"])
			}
		}
	}
	if assetsAny, ok := b["assets"].([]any); ok {
		for _, aAny := range assetsAny {
			if a, ok := aAny.(map[string]any); ok {
				a["class_name"] = className[fmt.Sprintf("%d", IntOf(a["class_id"]))]
			}
		}
	}

	labels := []any{}
	values := []any{}
	if classesAny, ok := b["classes"].([]any); ok {
		for _, cAny := range classesAny {
			if c, ok := cAny.(map[string]any); ok {
				desc, _ := c["description"].(string)
				val, _ := c["value"].(float64)
				labels = append(labels, desc)
				values = append(values, val)
			}
		}
	}

	// By-asset pie: sorted by value, largest first.
	assetLabels := []any{}
	assetValues := []any{}
	if assetsAny, ok := b["assets"].([]any); ok {
		type row struct {
			desc string
			val  float64
		}
		var rows []row
		for _, aAny := range assetsAny {
			if a, ok := aAny.(map[string]any); ok {
				desc, _ := a["description"].(string)
				val, _ := a["value"].(float64)
				rows = append(rows, row{desc, val})
			}
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].val > rows[j].val })
		for _, r := range rows {
			assetLabels = append(assetLabels, r.desc)
			assetValues = append(assetValues, r.val)
		}
	}

	var overTime map[string]any
	if ot, ok := b["allocation_over_time"].(map[string]any); ok {
		overTime = ot
	} else {
		overTime = map[string]any{}
	}
	otMonths := ToStrings(overTime["months"])
	otDatasets := []map[string]any{}
	if series, ok := overTime["series"].(map[string]any); ok {
		for _, name := range SortedNames(series) {
			otDatasets = append(otDatasets, map[string]any{
				"label": name, "data": ToFloats(series[name]), "filled": true,
			})
		}
	}

	var assetsOT map[string]any
	if aot, ok := b["assets_over_time"].(map[string]any); ok {
		assetsOT = aot
	} else {
		assetsOT = map[string]any{}
	}
	aotMonths := ToStrings(assetsOT["months"])
	idOf := map[string]int{}
	if assetsAny, ok := b["assets"].([]any); ok {
		for _, aAny := range assetsAny {
			if a, ok := aAny.(map[string]any); ok {
				idOf[StrOf(a["description"])] = IntOf(a["asset_id"])
			}
		}
	}
	aotSeries, _ := assetsOT["series"].(map[string]any)
	aotNames := SortedNames(aotSeries)
	sort.Slice(aotNames, func(i, j int) bool { return idOf[aotNames[i]] < idOf[aotNames[j]] })
	aotDatasets := []map[string]any{}
	for _, name := range aotNames {
		aotDatasets = append(aotDatasets, map[string]any{
			"label": name, "data": ToFloats(aotSeries[name]), "filled": true,
		})
	}

	last := ""
	if n := len(otMonths); n > 0 {
		last, _ = otMonths[n-1].(string)
	}
	presets := RangePresets(last)

	return map[string]any{
		"B":     b,
		"Scope": RememberedScopeVars(v.api, r),
		"Charts": map[string]any{
			"AllocationNow":      Chart(labels, Dataset("Value", values)),
			"AssetAllocationNow": Chart(assetLabels, Dataset("Value", assetValues)),
			"AllocationOverTime": Chart(otMonths, otDatasets...),
			"AssetsOverTime":     Chart(aotMonths, aotDatasets...),
		},
		"RangePresets": presets,
		"RangeActive":  RangeActive(UIValues(r), presets),
	}, nil
}

// provideBreakdownClass feeds the "Assets in this class" table (values,
// returns per asset) on the core class pages - that data is breakdown data.
func (v *views) provideBreakdownClass(r *http.Request, vars map[string]any) {
	if _, err := PathID(r); err != nil {
		return
	}
	b := v.api.GetDataQuiet(r.Context(), "/api/v1/stats/breakdown"+StatsQS(r))
	assets, _ := b["assets"].([]any)
	if assets == nil {
		assets = []any{}
	}
	vars["Assets"] = assets
}
