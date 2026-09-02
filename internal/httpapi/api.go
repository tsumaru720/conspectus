package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"conspectus/internal/domain"
	"conspectus/internal/settings"
	"conspectus/internal/stats"
)

type SystemInfo struct {
	Version               string
	Commit                string
	ExpectedSchemaVersion int
}

type API struct {
	Router   *Router
	Settings *settings.Service
	Stats    *stats.Service
	Log      *slog.Logger
	Info     SystemInfo

	// ReadOnly mirrors CONSPECTUS_READ_ONLY; the database-level read-only
	// setting was retired and the env var is the only control.
	ReadOnly bool

	Repos func() (domain.Repos, error)

	Now func() time.Time
	Loc *time.Location
}

func (a *API) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func (a *API) loc() *time.Location {
	if a.Loc != nil {
		return a.Loc
	}
	return time.UTC
}

func (a *API) reposOr500(w http.ResponseWriter) domain.Repos {
	repos, err := a.Repos()
	if err != nil {
		a.Log.Error("storage unavailable", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "storage unavailable")
		return nil
	}
	return repos
}

// Date-only values are calendar dates, not instants: they parse as UTC
// midnight so the UTC DSN/session writes the literal date into the TIMESTAMP
// columns. Parsing them in the app zone sent BST dates in as 23:00 the
// previous day.
func (a *API) parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.ParseInLocation("2006-01-02", s, time.UTC); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.In(a.loc()), nil
	}
	return time.Time{}, fmt.Errorf("%w: date must be YYYY-MM-DD or RFC3339, got %q", domain.ErrValidation, s)
}

// Calendar-date comparison, not instants: a UTC-midnight date reads as
// "after" local-midnight today in zones east of UTC.
func (a *API) rejectFuture(t time.Time) error {
	today := a.now().In(a.loc()).Format("2006-01-02")
	if d := t.Format("2006-01-02"); d > today {
		return fmt.Errorf("%w: future dates are not allowed (date %s is after today %s)",
			domain.ErrValidation, d, today)
	}
	return nil
}

// stampNow swaps a parsed date's midnight for the current time-of-day so
// entries read as "recorded at" (v1-style) while the calendar date stays
// exactly the picked date. The clock is UTC on purpose - NOW()/session time
// is UTC in our deployments, and stamping with a London wall clock would
// push entries recorded 00:00–00:59 BST back a day on write.
func (a *API) stampNow(t time.Time) time.Time {
	n := a.now().UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), n.Hour(), n.Minute(), n.Second(), n.Nanosecond(), time.UTC)
}

func parseID(s string) (int32, error) {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%w: invalid id %q", domain.ErrValidation, s)
	}
	return int32(n), nil
}

func queryInt(r *http.Request, name string) (int32, bool) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil || n < 0 {
		return 0, false
	}
	return int32(n), true
}

func queryMonth(r *http.Request, name string) (*domain.Month, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return nil, nil
	}
	m, err := domain.ParseMonth(s)
	if err != nil {
		return nil, fmt.Errorf("%w: %s must be YYYY-MM, got %q", domain.ErrValidation, name, s)
	}
	return &m, nil
}

func (a *API) SetErrorLogger() {
	ErrorLog = func(err error) {
		a.Log.Error("internal error", "error", err)
	}
}

func (a *API) RegisterCoreRoutes() error {
	reg := func(name, method, pattern string, h http.HandlerFunc) error {
		return a.Router.Register("core:"+name, method, pattern, h)
	}
	errs := []error{}
	must := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	must(reg("healthz", "GET", "/healthz", a.handleHealthz))
	must(reg("readyz", "GET", "/readyz", a.handleReadyz))
	must(reg("system", "GET", "/api/v1/system", a.handleSystem))
	// Fallback for /api/* paths no route owns. Without this the "/" page
	// route would serve HTML with a 200 and hide the 404 from API clients.
	// (Per-method: a method-less "/api/" pattern conflicts with the
	// method-specific "GET /" page route in Go's ServeMux.)
	apiFallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, `{"error":{"code":"not_found","message":"no such endpoint"}}`)
	})
	for _, method := range []string{"GET", "POST", "PATCH", "PUT", "DELETE"} {
		must(a.Router.Register("core:apifallback."+method, method, "/api/", apiFallback))
	}

	must(reg("assets.list", "GET", "/api/v1/assets", a.handleAssetsList))
	must(reg("assets.create", "POST", "/api/v1/assets", a.handleAssetsCreate))
	must(reg("assets.get", "GET", "/api/v1/assets/{id}", a.handleAssetsGet))
	must(reg("assets.update", "PATCH", "/api/v1/assets/{id}", a.handleAssetsUpdate))
	must(reg("assets.delete", "DELETE", "/api/v1/assets/{id}", a.handleAssetsDelete))
	must(reg("assets.close", "POST", "/api/v1/assets/{id}/close", a.handleAssetsClose))
	must(reg("assets.reopen", "POST", "/api/v1/assets/{id}/reopen", a.handleAssetsReopen))

	must(reg("classes.list", "GET", "/api/v1/classes", a.handleClassesList))
	must(reg("classes.create", "POST", "/api/v1/classes", a.handleClassesCreate))
	must(reg("classes.get", "GET", "/api/v1/classes/{id}", a.handleClassesGet))
	must(reg("classes.update", "PATCH", "/api/v1/classes/{id}", a.handleClassesUpdate))
	must(reg("classes.delete", "DELETE", "/api/v1/classes/{id}", a.handleClassesDelete))

	must(reg("logs.latest", "GET", "/api/v1/logs/latest", a.handleLogsLatest))
	must(reg("logs.list", "GET", "/api/v1/logs", a.handleLogsList))
	must(reg("logs.create", "POST", "/api/v1/logs", a.handleLogsCreate))
	must(reg("logs.get", "GET", "/api/v1/logs/{id}", a.handleLogsGet))
	must(reg("logs.update", "PATCH", "/api/v1/logs/{id}", a.handleLogsUpdate))
	must(reg("logs.delete", "DELETE", "/api/v1/logs/{id}", a.handleLogsDelete))

	must(reg("payments.list", "GET", "/api/v1/payments", a.handlePaymentsList))
	must(reg("payments.create", "POST", "/api/v1/payments", a.handlePaymentsCreate))
	must(reg("payments.get", "GET", "/api/v1/payments/{id}", a.handlePaymentsGet))
	must(reg("payments.update", "PATCH", "/api/v1/payments/{id}", a.handlePaymentsUpdate))
	must(reg("payments.delete", "DELETE", "/api/v1/payments/{id}", a.handlePaymentsDelete))

	must(reg("settings.get", "GET", "/api/v1/settings", a.handleSettingsGet))
	must(reg("settings.create", "POST", "/api/v1/settings/{key}", a.handleSettingCreate))
	must(reg("settings.update", "PATCH", "/api/v1/settings/{key}", a.handleSettingPatch))
	must(reg("settings.key.delete", "DELETE", "/api/v1/settings/{key}", a.handleSettingKeyDelete))

	if a.Stats != nil {
		if err := a.registerStatsRoutes(); err != nil {
			return err
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (a *API) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (a *API) handleReadyz(w http.ResponseWriter, r *http.Request) {
	checks := map[string]any{}
	ready := true

	repos, err := a.Repos()
	if err != nil {
		checks["db"] = map[string]any{"ok": false, "error": err.Error()}
		ready = false
	} else if err := repos.Ping(r.Context()); err != nil {
		checks["db"] = map[string]any{"ok": false, "error": err.Error()}
		ready = false
	} else {
		checks["db"] = map[string]any{"ok": true}
		if v, err := repos.Settings().Get(r.Context(), "db_version"); err == nil {
			if fmt.Sprint(a.Info.ExpectedSchemaVersion) != v {
				checks["migrations"] = map[string]any{"ok": false, "current": v, "expected": a.Info.ExpectedSchemaVersion}
				ready = false
			} else {
				checks["migrations"] = map[string]any{"ok": true, "current": v}
			}
		} else {
			checks["migrations"] = map[string]any{"ok": false, "current": "0", "expected": a.Info.ExpectedSchemaVersion}
			ready = false
		}
	}

	status := "ok"
	code := http.StatusOK
	if !ready {
		status = "unready"
		code = http.StatusServiceUnavailable
	}
	WriteJSON(w, code, map[string]any{"status": status, "checks": checks})
}

func (a *API) handleSystem(w http.ResponseWriter, r *http.Request) {
	sys := map[string]any{
		"version":                 a.Info.Version,
		"commit":                  a.Info.Commit,
		"expected_schema_version": a.Info.ExpectedSchemaVersion,
		"read_only":               a.ReadOnly,
		"routes":                  len(a.Router.Routes()),
	}
	if repos, err := a.Repos(); err == nil {
		if v, err := repos.Settings().Get(r.Context(), "db_version"); err == nil {
			sys["schema_version"] = v
		}
	}
	WriteData(w, http.StatusOK, sys, nil)
}
