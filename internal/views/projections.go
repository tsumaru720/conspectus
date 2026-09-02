package views

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
)

func (v *views) projectionsPage(w http.ResponseWriter, r *http.Request) {
	vars, err := v.projectionsVars(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if vars == nil {
		vars = map[string]any{}
	}
	vars["Title"] = "Projections"
	if err := v.deps.Render.RenderPage(w, r, "projections", vars); err != nil {
		v.deps.Log.Error("page render failed", "page", "projections", "error", err)
	}
}

func (v *views) projectionsVars(r *http.Request) (map[string]any, error) {
	ctx := r.Context()
	p4, err := v.api.GetData(ctx, "/api/v1/stats/projections"+ProjectionsQS(r))
	if err != nil {
		return nil, err
	}
	p4["horizon_months"] = IntOf(p4["horizon_months"])

	assumptions, _ := p4["assumptions"].(map[string]any)
	valueNow := 0.0
	if assumptions != nil {
		valueNow = AsFloat(assumptions["value_now"])
	}

	var milestones []any
	if msAny, ok := p4["milestones"].([]any); ok {
		for _, mAny := range msAny {
			if m, ok := mAny.(map[string]any); ok {
				nm := map[string]any{}
				maps.Copy(nm, m)
				if t := AsFloat(nm["target"]); t > 0 {
					nm["progress"] = valueNow / t
				}
				milestones = append(milestones, nm)
			}
		}
	}
	p4["milestones"] = milestones

	projChart := Chart(
		ToStrings(p4["months"]),
		Dataset("Typical path (median of 500)", ToFloats(p4["expected"])),
		map[string]any{"label": "Unlucky (10th percentile)", "data": ToFloats(p4["lower"]), "dashed": true},
		map[string]any{"label": "Lucky (90th percentile)", "data": ToFloats(p4["upper"]), "dashed": true, "band": true},
		map[string]any{"label": "Average month, every month (CAGR)", "data": ToFloats(p4["cagr_path"]), "dashed": true, "width": 1},
	)

	var targetChart map[string]any
	targetValue, targetYear, rMonthly := 0.0, 0, 0.0
	if tc, ok := p4["target_curve"].(map[string]any); ok {
		targetValue = AsFloat(tc["value"])
		targetYear = IntOf(tc["year"])
		rMonthly = AsFloat(tc["required_monthly"])
		years, vals := YearEndBuckets(ToStrings(tc["curve_months"]), ToFloats(tc["curve_values"]))
		targetChart = Chart(years, Dataset("Required value (year-end)", vals))
	} else {
		targetChart = Chart([]any{}, Dataset("Required value (year-end)", []any{}))
	}

	valSeries := v.api.Series(ctx, ScopedStatsQS(r), "value")
	valMonths := ToStrings(valSeries["months"])
	valData := ToFloats(valSeries["data"])
	trackYears, trackActuals := YearEndActuals(valMonths, valData)
	currentYear := ""
	if n := len(valMonths); n > 0 {
		if ms, ok := valMonths[n-1].(string); ok && len(ms) >= 4 {
			currentYear = ms[:4]
		}
	}
	curYearFull := false
	for _, y := range trackYears {
		if y == currentYear {
			curYearFull = true
		}
	}
	latestValue := 0.0
	if n := len(valData); n > 0 {
		if f, ok := valData[n-1].(float64); ok {
			latestValue = f
		}
	}
	requiredFor := func(y string) float64 {
		return RequiredAtYear(targetValue, targetYear, rMonthly, y)
	}
	trackRows := []any{}
	actualVals := []any{}
	requiredVals := []any{}
	for i, y := range trackYears {
		actual := trackActuals[i]
		required := requiredFor(y)
		trackRows = append(trackRows, map[string]any{
			"Year": y, "Actual": actual, "Required": required,
			"Diff": actual - required, "HasTarget": targetValue > 0,
		})
		actualVals = append(actualVals, actual)
		requiredVals = append(requiredVals, required)
	}
	if currentYear != "" && !curYearFull {
		trackRows = append(trackRows, map[string]any{
			"Year": currentYear, "Actual": latestValue, "Required": requiredFor(currentYear),
			"Diff": latestValue - requiredFor(currentYear), "HasTarget": targetValue > 0,
			"Current": true,
		})
		actualVals = append(actualVals, latestValue)
		requiredVals = append(requiredVals, requiredFor(currentYear))
	}
	descRows := make([]any, 0, len(trackRows))
	for _, trackRow := range slices.Backward(trackRows) {
		descRows = append(descRows, trackRow)
	}
	trackRows = descRows
	chartYears := make([]any, 0, len(trackYears)+1)
	for _, y := range trackYears {
		chartYears = append(chartYears, y)
	}
	if currentYear != "" && !curYearFull {
		chartYears = append(chartYears, currentYear)
	}
	var trackChart map[string]any
	if targetValue > 0 {
		trackChart = Chart(chartYears, map[string]any{
			"label": "Actual", "data": actualVals, "colorIndex": 1,
		}, map[string]any{
			"label": "Target pace", "data": requiredVals, "colorIndex": 0,
		})
	} else {
		trackChart = Chart(chartYears, Dataset("Actual", actualVals))
	}

	return map[string]any{
		"P":     p4,
		"Scope": RememberedScopeVars(v.api, r),
		"Charts": map[string]any{
			"Projections": projChart,
			"TargetCurve": targetChart,
			"YearlyTrack": trackChart,
		},
		"Track": map[string]any{
			"Rows":      trackRows,
			"HasTarget": targetValue > 0,
			"ValueNow":  valueNow,
		},
	}, nil
}

// manageVars feeds the projection-targets card on the manage page: the
// stored targets as a sorted list for the per-target table.
func (v *views) manageVars(r *http.Request, vars map[string]any) {
	vars["ProjectionTargetList"] = mergeTargets(parseStoredTargets(v.deps.Settings.Get("projection_targets")))
}

func (v *views) saveTargets(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		v.redirectFlash(w, r, "err", "Bad form.")
		return
	}
	added, err := parseTargetsInput(r.PostFormValue("targets"))
	if err != nil {
		v.redirectFlash(w, r, "err", err.Error())
		return
	}
	if len(added) == 0 {
		v.redirectFlash(w, r, "err", "Enter at least one amount.")
		return
	}
	// The form adds to what is stored: merge, dedupe, sort, then write the
	// canonical JSON array back (same shape the stats service parses).
	val, err := marshalTargets(append(parseStoredTargets(v.deps.Settings.Get("projection_targets")), added...))
	if err != nil {
		v.redirectFlash(w, r, "err", err.Error())
		return
	}
	if err := v.deps.Settings.Set(r.Context(), "projection_targets", val); err != nil {
		v.redirectFlash(w, r, "err", err.Error())
		return
	}
	v.redirectFlash(w, r, "ok", fmt.Sprintf("Added %d projection target(s).", len(added)))
}

func (v *views) deleteTarget(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		v.redirectFlash(w, r, "err", "Bad form.")
		return
	}
	n, err := strconv.ParseFloat(strings.TrimSpace(r.PostFormValue("value")), 64)
	if err != nil || n <= 0 {
		v.redirectFlash(w, r, "err", errInvalidTargets.Error())
		return
	}
	removed := false
	var kept []float64
	for _, m := range parseStoredTargets(v.deps.Settings.Get("projection_targets")) {
		if !removed && m == n {
			removed = true
			continue
		}
		kept = append(kept, m)
	}
	if !removed {
		v.redirectFlash(w, r, "err", "Target not found.")
		return
	}
	val, err := marshalTargets(kept)
	if err != nil {
		v.redirectFlash(w, r, "err", err.Error())
		return
	}
	if err := v.deps.Settings.Set(r.Context(), "projection_targets", val); err != nil {
		v.redirectFlash(w, r, "err", err.Error())
		return
	}
	v.redirectFlash(w, r, "ok", "Projection target removed.")
}

// parseStoredTargets decodes the projection_targets setting - a JSON array
// of amounts, exactly what the stats service parses defensively itself.
func parseStoredTargets(raw string) []float64 {
	var out []float64
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// parseTargetsInput accepts a plain comma list - "1000, 2000" with the
// spaces ignored - and tolerates surrounding brackets from a pasted array.
func parseTargetsInput(s string) ([]float64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	var out []float64
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.ParseFloat(part, 64)
		if err != nil || n <= 0 {
			return nil, errInvalidTargets
		}
		out = append(out, n)
	}
	return out, nil
}

// mergeTargets unions the lists, drops non-positive amounts and duplicates,
// and sorts ascending.
func mergeTargets(lists ...[]float64) []float64 {
	seen := map[float64]bool{}
	var out []float64
	for _, list := range lists {
		for _, n := range list {
			if n > 0 && !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	sort.Float64s(out)
	return out
}

// marshalTargets renders the canonical stored form; an empty list stores as
// "" (no targets), matching the unset case everywhere else.
func marshalTargets(list []float64) (string, error) {
	merged := mergeTargets(list)
	if len(merged) == 0 {
		return "", nil
	}
	b, err := json.Marshal(merged)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

var errInvalidTargets = errStr("targets must be positive amounts separated by commas, e.g. 100000, 250000")

type errStr string

func (e errStr) Error() string { return string(e) }

func (v *views) redirectFlash(w http.ResponseWriter, r *http.Request, kind, msg string) {
	http.SetCookie(w, &http.Cookie{
		Name:  "conspectus_flash",
		Value: url.QueryEscape(kind + ":" + msg),
		Path:  "/", MaxAge: 60,
	})
	http.Redirect(w, r, "/manage", http.StatusSeeOther)
}
