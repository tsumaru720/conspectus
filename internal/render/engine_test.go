package render

import (
	"bytes"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

type fakeSettings struct{ m map[string]string }

func (f fakeSettings) Get(key string) string { return f.m[key] }

func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func newEngineOver(t *testing.T, dir string, settings Settings) *Engine {
	t.Helper()
	return New(dir, slog.New(slog.DiscardHandler), settings)
}

func TestRealTemplatesParse(t *testing.T) {
	e := newEngineOver(t, "../../web", fakeSettings{m: map[string]string{}})
	tmpl, err := e.set()
	if err != nil {
		t.Fatalf("shipped template set failed to parse: %v", err)
	}
	for _, name := range []string{
		"overview.html", "ledger.html", "import.html", "manage.html",
		"breakdown.html", "analytics.html", "projections.html",
		"asset.html", "class.html", "banner.html",
		"partials/header.html", "partials/footer.html",
	} {
		if tmpl.Lookup(name) == nil {
			t.Errorf("template %s missing from the shipped set", name)
		}
	}
}

func TestBannerSubstitution(t *testing.T) {
	e := newEngineOver(t, t.TempDir(), fakeSettings{})
	e.Version = "9.9.9"
	e.hostname = "testhost"
	e.BannerText = "viewer {{version}} on {{ hostname }} at {{date}} / {{  version  }}"
	b := e.banner()
	if want := "viewer 9.9.9 on testhost at " + e.startedAt.Format("2006-01-02 15:04") + " / 9.9.9"; b != want {
		t.Fatalf("banner = %q, want %q", b, want)
	}
	e.BannerText = "keep {{unknown}} and {version} literal"
	if b := e.banner(); b != e.BannerText {
		t.Fatalf("unknown placeholders must be left untouched, got %q", b)
	}
	e.BannerText = ""
	if e.banner() != "" {
		t.Fatal("empty banner text must render no banner")
	}
}

func TestBlocksMergeRegisteredGroups(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"templates/blocks/overview/returns.html":         "r",
		"templates/blocks/overview/kpis.html":            "k",
		"templates/blocks/overview/monthly-changes.html": "m",
	})
	e := newEngineOver(t, dir, fakeSettings{})
	e.RegisterBlocks("overview", 50, "returns", "kpis", "missing-block")
	e.RegisterBlocks("overview", 100, "monthly-changes")
	blocks := e.BlocksFor("overview")
	if len(blocks) != 3 || blocks[0] != "returns" || blocks[1] != "kpis" || blocks[2] != "monthly-changes" {
		t.Fatalf("blocks = %v, want [returns kpis monthly-changes] (missing-block dropped)", blocks)
	}
	if got := e.BlocksFor("analytics"); got != nil {
		t.Fatalf("no group registered for analytics, got %v", got)
	}
}

func TestRenderPageSmoke(t *testing.T) {
	e := newEngineOver(t, "../../web", fakeSettings{m: map[string]string{}})
	e.Version = "1.2.3"
	e.BannerText = "<b>hello banner</b> v{{version}}"
	e.RegisterBlocks("overview", 50, "kpis", "value-vs-deposits", "returns", "monthly-changes")
	r, _ := http.NewRequest("GET", "/?from=2025-01&granularity=quarter", nil)

	var buf bytes.Buffer
	w := &respRecorder{&buf}
	vars := map[string]any{
		"Title": "Test",
		"O": map[string]any{
			"value": 2506.0, "deposits": 2500.0, "return": 6.0, "return_pct": 0.0024,
			"twrr": 0.0036, "twrr_adj": 0.0036, "cagr": 0.0109, "months": 4, "assets": 1,
		},
		"Charts": map[string]any{
			"ValueVsDeposits": map[string]any{
				"labels":   []string{"2026-01", "2026-02"},
				"datasets": []any{map[string]any{"label": "Value", "data": []float64{2000, 2004}}, map[string]any{"label": "Deposits", "data": []float64{2000, 2000}}},
			},
			"Return":          map[string]any{"labels": []string{"2026-01"}, "datasets": []any{map[string]any{"label": "Return", "data": []float64{0}}}},
			"TWRR":            map[string]any{"labels": []string{"2026-01"}, "datasets": []any{map[string]any{"label": "TWRR", "data": []float64{0}}}},
			"MonthlyReturn":   map[string]any{"labels": []string{"2026-01"}, "datasets": []any{map[string]any{"label": "Δ", "data": []float64{4}}}},
			"MonthlyDeposits": map[string]any{"labels": []string{"2026-01"}, "datasets": []any{map[string]any{"label": "Δ", "data": []float64{0}}}},
		},
		"Tables": map[string]any{
			"ValueVsDeposits": []map[string]any{{"Month": "2026-01", "Value": 2000.0, "Deposits": 2000.0}},
		},
		"RangePresets": map[string]string{"m3": "2026-05", "m6": "2026-02", "ytd": "2026-01", "y1": "2025-08", "y5": "2021-08"},
		"RangeActive":  "all",
		"Benchmark":    false,
	}
	if err := e.RenderPage(w, r, "overview", vars); err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{
		"£2,506.00",
		"hello banner",
		"v1.2.3",
		"chart-data",
		"Value vs deposits",
		"data-theme",
		`"datasets":`,
	} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Errorf("rendered page missing %q", want)
		}
	}
	if bytes.Contains(buf.Bytes(), []byte(`<b>hello`)) {
		t.Errorf("banner text must be HTML-escaped")
	}
	if bytes.Contains(buf.Bytes(), []byte(`\"datasets\"`)) {
		t.Errorf("chart payload is double-encoded inside <script>; funcJSON must return template.JS")
	}
}

func TestNoBannerByDefault(t *testing.T) {
	e := newEngineOver(t, "../../web", fakeSettings{m: map[string]string{}})
	e.RegisterBlocks("overview", 50, "kpis", "value-vs-deposits", "returns", "monthly-changes")
	r, _ := http.NewRequest("GET", "/", nil)
	var buf bytes.Buffer
	w := &respRecorder{&buf}
	vars := map[string]any{"Title": "T", "O": map[string]any{}, "Charts": map[string]any{}, "RangePresets": map[string]string{}, "RangeActive": "all", "Benchmark": false}
	if err := e.RenderPage(w, r, "overview", vars); err != nil {
		t.Fatalf("render: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("banner-banner")) {
		t.Errorf("banner must not appear without CONSPECTUS_BANNER_TEXT")
	}
}

type respRecorder struct{ b *bytes.Buffer }

func (r *respRecorder) Header() http.Header         { return http.Header{} }
func (r *respRecorder) Write(p []byte) (int, error) { return r.b.Write(p) }
func (r *respRecorder) WriteHeader(int)             {}

func TestUICookieValuesSanitisesMonths(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: UICookie, Value: "from=2026-01&to=abc&q=savings&per_page=50"})
	ui := UICookieValues(req)
	if ui.Get("from") != "2026-01" {
		t.Errorf("from = %q, want 2026-01 kept", ui.Get("from"))
	}
	if ui.Get("to") != "" {
		t.Errorf("to = %q, want a non-month dropped", ui.Get("to"))
	}
	if ui.Get("q") != "savings" || ui.Get("per_page") != "50" {
		t.Errorf("other keys must pass through untouched, got q=%q per_page=%q", ui.Get("q"), ui.Get("per_page"))
	}

	req, _ = http.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: UICookie, Value: "from=nope&to=2026-13"})
	ui = UICookieValues(req)
	if ui.Get("from") != "" || ui.Get("to") != "" {
		t.Errorf("from=%q to=%q, want both dropped", ui.Get("from"), ui.Get("to"))
	}

	req, _ = http.NewRequest("GET", "/", nil)
	if got := UICookieValues(req); len(got) != 0 {
		t.Errorf("no cookie = %v, want empty values", got)
	}
}
