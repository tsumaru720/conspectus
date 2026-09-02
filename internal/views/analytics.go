package views

import (
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
)

func (v *views) analyticsPage(w http.ResponseWriter, r *http.Request) {
	vars, err := v.analyticsVars(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if vars == nil {
		vars = map[string]any{}
	}
	vars["Title"] = "Analytics"
	if err := v.deps.Render.RenderPage(w, r, "analytics", vars); err != nil {
		v.deps.Log.Error("page render failed", "page", "analytics", "error", err)
	}
}

func (v *views) analyticsVars(r *http.Request) (map[string]any, error) {
	a, err := v.api.GetData(r.Context(), "/api/v1/stats/analytics"+ScopedStatsQS(r))
	if err != nil {
		return nil, err
	}

	hmRows := []any{}
	if hm, ok := a["heatmap"].(map[string]any); ok {
		var years []int
		if yearsAny, ok := hm["years"].([]any); ok {
			for _, yAny := range yearsAny {
				years = append(years, IntOf(yAny))
			}
		}
		cells, _ := hm["cells"].(map[string]any)
		for _, y := range years {
			row := map[string]any{"Year": y, "Cells": []any{}}
			cells12 := []any{}
			for mi := 1; mi <= 12; mi++ {
				cell := map[string]any{"Ret": "", "V": 0.0, "Neg": false}
				if rowAny, ok := cells[fmt.Sprintf("%d", y)].([]any); ok && mi-1 < len(rowAny) {
					if val := rowAny[mi-1]; val != nil {
						if f, ok := val.(float64); ok {
							cell["Ret"] = fmt.Sprintf("%+.2f%%", f*100)
							intensity := math.Abs(f) / 0.08
							if intensity > 1 {
								intensity = 1
							}
							cell["V"] = math.Round(intensity*1000) / 1000
							cell["Neg"] = f < 0
						}
					}
				}
				cells12 = append(cells12, cell)
			}
			row["Cells"] = cells12
			hmRows = append(hmRows, row)
		}
	}

	var hist map[string]any
	if h, ok := a["histogram"].(map[string]any); ok {
		hist = h
	} else {
		hist = map[string]any{}
	}
	decomp := v.api.Series(r.Context(), ScopedStatsQS(r), "decomposition")
	decompChart := Chart(
		ToStrings(decomp["months"]),
		map[string]any{"label": "New money", "data": ToFloats(decomp["new_money"]), "stack": "d"},
		map[string]any{"label": "Growth", "data": ToFloats(decomp["growth"]), "stack": "d"},
	)

	months := ToStrings(decomp["months"])
	last := ""
	if n := len(months); n > 0 {
		last, _ = months[n-1].(string)
	}
	presets := RangePresets(last)

	// yearlyTable emits newest-first for the table; charts want oldest-left.
	yearLabels := []any{}
	twrrAdjVals := []any{}
	changeVals := []any{}
	if yearlyRows, ok := a["yearly"].([]any); ok {
		for _, yearlyRow := range slices.Backward(yearlyRows) {
			row := MapOf(yearlyRow)
			yearLabels = append(yearLabels, strconv.Itoa(IntOf(row["year"])))
			twrrAdjVals = append(twrrAdjVals, AsFloat(row["twrr_adj"]))
			if row["change_pct"] == nil {
				changeVals = append(changeVals, nil)
			} else {
				changeVals = append(changeVals, AsFloat(row["change_pct"]))
			}
		}
	}

	return map[string]any{
		"A":           a,
		"Scope":       RememberedScopeVars(v.api, r),
		"HeatmapRows": hmRows,
		"Charts": map[string]any{
			"Histogram":     Chart(ToStrings(hist["bins"]), Dataset("Months", ToFloats(hist["counts"]))),
			"Decomposition": decompChart,
			"YearlyTWRR":    Chart(yearLabels, Dataset("Income-adjusted TWRR", twrrAdjVals)),
			"YearlyChange":  Chart(yearLabels, Dataset("Value change", changeVals)),
		},
		"RangePresets": presets,
		"RangeActive":  RangeActive(UIValues(r), presets),
	}, nil
}
