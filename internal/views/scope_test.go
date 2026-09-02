package views

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func uiCookie(pairs string) *http.Cookie {
	return &http.Cookie{Name: "conspectus_ui", Value: url.QueryEscape(pairs)}
}

func TestRequestScope(t *testing.T) {
	r := httptest.NewRequest("GET", "/breakdown", nil)
	if kind, id := RequestScope(r); kind != "" || id != 0 {
		t.Errorf("bare path: RequestScope = %q, %d, want empty", kind, id)
	}

	r = httptest.NewRequest("GET", "/breakdown", nil)
	r.AddCookie(uiCookie("scope.asset_id=47"))
	if kind, id := RequestScope(r); kind != "asset" || id != 47 {
		t.Errorf("cookie only: RequestScope = %q, %d, want asset 47", kind, id)
	}

	r = httptest.NewRequest("GET", "/analytics/class/3", nil)
	r.SetPathValue("id", "3")
	r.AddCookie(uiCookie("scope.asset_id=47"))
	if kind, id := RequestScope(r); kind != "class" || id != 3 {
		t.Errorf("both present: RequestScope = %q, %d, want path class 3 to win", kind, id)
	}
}

func TestStatsQSScope(t *testing.T) {
	r := httptest.NewRequest("GET", "/breakdown", nil)
	r.AddCookie(uiCookie("scope.asset_id=47&from=2024-01"))
	if got := StatsQS(r); strings.Contains(got, "asset_id") {
		t.Errorf("StatsQS on a bare path = %q, want no scope", got)
	}
	got := ScopedStatsQS(r)
	if !strings.Contains(got, "asset_id=47") || !strings.Contains(got, "from=2024-01") {
		t.Errorf("ScopedStatsQS = %q, want remembered scope plus range", got)
	}

	r = httptest.NewRequest("GET", "/projections/class/3", nil)
	r.SetPathValue("id", "3")
	r.AddCookie(uiCookie("scope.asset_id=47"))
	if got := ProjectionsQS(r); strings.Contains(got, "asset_id") || !strings.Contains(got, "class_id=3") {
		t.Errorf("ProjectionsQS = %q, want path scope only", got)
	}
}

func TestScopeVarsRemembered(t *testing.T) {
	api := API{}

	r := httptest.NewRequest("GET", "/breakdown", nil)
	r.AddCookie(uiCookie("scope.asset_id=47"))
	v := RememberedScopeVars(api, r)
	if v["AssetID"] != 47 {
		t.Errorf("AssetID = %v, want 47", v["AssetID"])
	}
	if v["BasePath"] != "/breakdown" {
		t.Errorf("BasePath = %v, want /breakdown", v["BasePath"])
	}

	r = httptest.NewRequest("GET", "/breakdown/asset/47", nil)
	r.SetPathValue("id", "47")
	v = RememberedScopeVars(api, r)
	if v["AssetID"] != 47 || v["BasePath"] != "/breakdown" {
		t.Errorf("path scope: AssetID = %v, BasePath = %v, want 47 and /breakdown", v["AssetID"], v["BasePath"])
	}

	// Plain ScopeVars ignores what the browser remembers.
	r = httptest.NewRequest("GET", "/breakdown", nil)
	r.AddCookie(uiCookie("scope.asset_id=47"))
	v = ScopeVars(api, r)
	if v["AssetID"] != 0 {
		t.Errorf("ScopeVars AssetID = %v, want 0", v["AssetID"])
	}
}

func TestOverviewRedirect(t *testing.T) {
	// An API whose asset/class lookups fail stands in for a stale memory.
	v := &views{api: API{}}
	r := httptest.NewRequest("GET", "/", nil)
	if got := v.overviewRedirect(r); got != "" {
		t.Errorf("no memory: overviewRedirect = %q, want empty", got)
	}
	r = httptest.NewRequest("GET", "/", nil)
	r.AddCookie(uiCookie("scope.asset_id=47"))
	if got := v.overviewRedirect(r); got != "" {
		t.Errorf("stale asset: overviewRedirect = %q, want empty", got)
	}

	// A live API that knows the asset/class lets the redirect through.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":1,"description":"x"}}`))
	}))
	defer srv.Close()
	v = &views{api: API{Client: srv.Client(), Base: srv.URL}}
	r = httptest.NewRequest("GET", "/", nil)
	r.AddCookie(uiCookie("scope.asset_id=47"))
	if got := v.overviewRedirect(r); got != "/asset/47" {
		t.Errorf("remembered asset: overviewRedirect = %q, want /asset/47", got)
	}
	r = httptest.NewRequest("GET", "/", nil)
	r.AddCookie(uiCookie("scope.class_id=24"))
	if got := v.overviewRedirect(r); got != "/class/24" {
		t.Errorf("remembered class: overviewRedirect = %q, want /class/24", got)
	}
}
