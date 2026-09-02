package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"conspectus/internal/stats"
)

// registerStatsRoutes wires the /api/v1/stats/* endpoints. They serve from
// the local stats service, so they exist only when storage is wired
// (frontend-only deployments compute their stats against the remote API).
func (a *API) registerStatsRoutes() error {
	reg := func(name, method, pattern string, h http.HandlerFunc) error {
		return a.Router.Register("core:"+name, method, pattern, h)
	}
	statsJSON := func(fn func(r *http.Request) (any, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			data, err := fn(r)
			if err != nil {
				writeStatsError(w, err)
				return
			}
			WriteData(w, http.StatusOK, data, nil)
		}
	}
	req := statsRequestFromQuery
	errs := []error{}
	must := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	must(reg("stats.overview", "GET", "/api/v1/stats/overview", statsJSON(func(r *http.Request) (any, error) {
		return a.Stats.Overview(r.Context(), req(r))
	})))
	must(reg("stats.series", "GET", "/api/v1/stats/series", statsJSON(func(r *http.Request) (any, error) {
		return a.Stats.Series(r.Context(), req(r), r.URL.Query().Get("metric"))
	})))
	must(reg("stats.income", "GET", "/api/v1/stats/income", statsJSON(func(r *http.Request) (any, error) {
		return a.Stats.Income(r.Context(), req(r))
	})))
	must(reg("stats.quality", "GET", "/api/v1/stats/quality", statsJSON(func(r *http.Request) (any, error) {
		return a.Stats.Quality(r.Context())
	})))
	must(reg("stats.breakdown", "GET", "/api/v1/stats/breakdown", statsJSON(func(r *http.Request) (any, error) {
		return a.Stats.Breakdown(r.Context(), req(r))
	})))
	must(reg("stats.analytics", "GET", "/api/v1/stats/analytics", statsJSON(func(r *http.Request) (any, error) {
		return a.Stats.Analytics(r.Context(), req(r))
	})))
	must(reg("stats.projections", "GET", "/api/v1/stats/projections", statsJSON(func(r *http.Request) (any, error) {
		return a.Stats.Projections(r.Context(), req(r))
	})))
	must(reg("stats.asset", "GET", "/api/v1/stats/assets/{id}", statsJSON(func(r *http.Request) (any, error) {
		id, err := parseID(r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		sr := req(r)
		sr.AssetID = id
		return a.Stats.Overview(r.Context(), sr)
	})))

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// statsRequestFromQuery builds a StatsRequest from explicit query parameters.
func statsRequestFromQuery(r *http.Request) stats.StatsRequest {
	q := r.URL.Query()
	sr := stats.StatsRequest{
		From:        q.Get("from"),
		To:          q.Get("to"),
		Granularity: q.Get("granularity"),
		Closed:      q.Get("closed"),
	}
	if v := q.Get("asset_id"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sr.AssetID = int32(n)
		}
	}
	if v := q.Get("class_id"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sr.ClassID = int32(n)
		}
	}
	if v := q.Get("horizon"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 600 {
			sr.Horizon = n
		}
	}
	if v := q.Get("window"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1200 {
			sr.Window = n
		}
	}
	return sr
}

// writeStatsError maps stats-service errors onto the JSON error envelope:
// validation → 400, not-found → 404, rest → 500.
func writeStatsError(w http.ResponseWriter, err error) {
	msg := err.Error()
	code, status := "internal", http.StatusInternalServerError
	switch {
	case strings.Contains(msg, "validation"):
		code, status = "validation_error", http.StatusBadRequest
	case strings.Contains(msg, "not found"):
		code, status = "not_found", http.StatusNotFound
	}
	WriteError(w, status, code, msg)
}
