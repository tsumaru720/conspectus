package views

import (
	"log/slog"
	"net/http"

	"conspectus/internal/apiclient"
	"conspectus/internal/httpapi"
	"conspectus/internal/render"
	"conspectus/internal/settings"
)

// Deps wires the analysis pages into the app. API is the frontend's JSON
// client (loopback in bundled mode, remote in frontend-only mode); Settings
// is the DB-backed settings service (projection targets).
type Deps struct {
	Router   *httpapi.Router
	Render   *render.Engine
	API      *apiclient.Client
	Settings *settings.Service
	Log      *slog.Logger
}

type views struct {
	deps Deps
	api  API
}

// Register wires the analysis pages, their nav entries, their blocks and
// the vars they contribute to the core pages.
func Register(d Deps) error {
	v := &views{
		deps: d,
		api:  API{Client: d.API.StdClient(), Base: d.API.Base()},
	}
	reg := func(name, method, pattern string, h http.HandlerFunc) error {
		return d.Router.Register("views:"+name, method, pattern, h)
	}
	errs := []error{}
	must := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	// "/{$}" anchors the overview pattern to the exact root: without it the
	// route is a catch-all and would swallow unknown URLs with a 200.
	must(reg("page.overview", "GET", "/{$}", v.overviewPage))
	must(reg("page.breakdown", "GET", "/breakdown", v.breakdownPage))
	must(reg("page.breakdown.asset", "GET", "/breakdown/asset/{id}", v.breakdownPage))
	must(reg("page.breakdown.class", "GET", "/breakdown/class/{id}", v.breakdownPage))
	must(reg("page.analytics", "GET", "/analytics", v.analyticsPage))
	must(reg("page.analytics.asset", "GET", "/analytics/asset/{id}", v.analyticsPage))
	must(reg("page.analytics.class", "GET", "/analytics/class/{id}", v.analyticsPage))
	must(reg("page.projections", "GET", "/projections", v.projectionsPage))
	must(reg("page.projections.asset", "GET", "/projections/asset/{id}", v.projectionsPage))
	must(reg("page.projections.class", "GET", "/projections/class/{id}", v.projectionsPage))
	must(reg("manage.targets", "POST", "/projections/settings", v.saveTargets))
	must(reg("manage.targets.delete", "POST", "/projections/targets/delete", v.deleteTarget))
	if len(errs) > 0 {
		return errs[0]
	}

	d.Render.AddNav("Overview", "/", "overview", 10)
	d.Render.AddScopedNav("Breakdown", "/breakdown", "breakdown", 20)
	d.Render.AddScopedNav("Analytics", "/analytics", "analytics", 30)
	d.Render.AddScopedNav("Projections", "/projections", "projections", 40)
	// Nav links carry the applied scope so the URL always shows what the
	// page is scoped to; RequestScope is path first, remembered fallback.
	d.Render.ScopeOf = RequestScope

	d.Render.RegisterBlocks("overview", 50, "kpis", "value-vs-deposits", "returns", "monthly-changes")
	d.Render.RegisterBlocks("asset", 50, "kpis", "value-vs-deposits", "return-over-time", "returns", "monthly-changes", "income")
	d.Render.RegisterBlocks("class", 50, "kpis", "value-vs-deposits", "return-over-time", "returns", "monthly-changes", "income")
	d.Render.RegisterBlocks("breakdown", 55, "allocation-now", "allocation-over-time", "allocation-assets-over-time", "class-table", "closed-summary")
	d.Render.RegisterBlocks("class", 55, "assets")
	d.Render.RegisterBlocks("analytics", 60, "yearly-table", "yearly-charts", "records", "histogram", "heatmap", "decomposition")
	d.Render.RegisterBlocks("projections", 65, "scenarios", "milestones", "target-curve", "yearly-track")
	d.Render.RegisterBlocks("manage", 65, "targets")

	// Analysis vars for the core asset/class pages; the breakdown table on
	// class pages; the projection-targets card on the manage page.
	d.Render.ProvideVars("asset", v.provideAsset)
	d.Render.ProvideVars("class", v.provideOverviewClass)
	d.Render.ProvideVars("class", v.provideBreakdownClass)
	d.Render.ProvideVars("manage", v.manageVars)
	return nil
}
