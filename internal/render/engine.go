package render

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"conspectus/internal/domain"
	"conspectus/internal/httpapi"
	"conspectus/internal/settings"
)

const (
	UICookie    = "conspectus_ui"
	FlashCookie = "conspectus_flash"
)

type Settings interface {
	Get(key string) string
}

type SettingsAll interface {
	All() map[string]string
}

func UICookieValues(r *http.Request) url.Values {
	out := url.Values{}
	if r == nil {
		return out
	}
	c, err := r.Cookie(UICookie)
	if err != nil || c.Value == "" {
		return out
	}
	v, err := url.QueryUnescape(c.Value)
	if err != nil {
		v = c.Value
	}
	q, err := url.ParseQuery(v)
	if err != nil {
		return out
	}
	// The cookie mirrors client-side UI state, so it can carry anything a
	// browser can store. A from/to that is not a real month would fail the
	// stats API on every page; dropping it renders the default window and
	// the next valid pick overwrites the cookie.
	for _, k := range []string{"from", "to"} {
		if s := q.Get(k); s != "" {
			if _, err := domain.ParseMonth(s); err != nil {
				q.Del(k)
			}
		}
	}
	return q
}

// NavEntry is one primary navigation link.
type NavEntry struct {
	Label string
	Path  string
	Page  string
	Order int
	// Scoped entries link to the asset/class the request renders with
	// (see Engine.ScopeOf); the three browse-picker pages set it.
	Scoped bool
}

// blockGroup is an ordered list of blocks for one page; groups render in
// (order, seq) so the analysis sections come before the ledger tables.
type blockGroup struct {
	page   string
	order  int
	seq    int
	blocks []string
}

type varProvider struct {
	page string
	fn   func(r *http.Request, vars map[string]any)
}

// Engine renders the HTML frontend from the templates and assets in Dir.
// Files are read from disk (no embed): templates and assets are shipped next
// to the binary and can be edited in place - changes apply on restart.
type Engine struct {
	Dir      string
	Log      *slog.Logger
	Settings Settings
	Version  string

	// ReadOnly mirrors CONSPECTUS_READ_ONLY and feeds the templates'
	// .Settings.read_only flag (buttons disabled across the UI).
	ReadOnly bool

	// BannerText comes from CONSPECTUS_BANNER_TEXT; empty means no banner.
	BannerText string

	// ScopeOf resolves the scope a request renders with (path first, then
	// the remembered one). When set, Scoped nav entries link straight to
	// the scoped path so the address bar shows the scope without any JS.
	ScopeOf func(r *http.Request) (kind string, id int)

	hostname  string
	startedAt time.Time

	mu        sync.RWMutex
	cache     *template.Template
	hashes    map[string]string
	funcs     template.FuncMap
	nav       []NavEntry
	blockSet  []blockGroup
	providers []varProvider
	blockSeq  int
}

func New(dir string, log *slog.Logger, settings Settings) *Engine {
	e := &Engine{
		Dir: dir, Log: log, Settings: settings,
		hashes: map[string]string{},
	}
	e.BannerText = strings.TrimSpace(os.Getenv("CONSPECTUS_BANNER_TEXT"))
	e.hostname, _ = os.Hostname()
	e.startedAt = time.Now()
	if fi, err := os.Stat(path.Join(dir, "templates")); err != nil || !fi.IsDir() {
		log.Warn("web templates directory not found; HTML pages will fail to render (set CONSPECTUS_WEB_DIR)", "dir", dir)
	}
	return e
}

// bannerPlaceholders matches {{version}}, {{hostname}} and {{date}},
// tolerating inner whitespace (e.g. {{ version }}) like most template engines.
var bannerPlaceholders = regexp.MustCompile(`\{\{\s*(version|hostname|date)\s*\}\}`)

// banner expands the {{version}}, {{hostname}} and {{date}} placeholders in the
// configured banner text. {{date}} is the process start time, so every render
// of one run carries the same stamp. Unknown {{...}} sequences are left
// untouched.
func (e *Engine) banner() string {
	if e.BannerText == "" {
		return ""
	}
	return bannerPlaceholders.ReplaceAllStringFunc(e.BannerText, func(m string) string {
		switch bannerPlaceholders.FindStringSubmatch(m)[1] {
		case "version":
			return e.Version
		case "hostname":
			return e.hostname
		default: // date
			return e.startedAt.Format("2006-01-02 15:04")
		}
	})
}

func (e *Engine) assetHash(rel string) string {
	e.mu.RLock()
	h, ok := e.hashes[rel]
	e.mu.RUnlock()
	if ok {
		return h
	}
	data, err := os.ReadFile(path.Join(e.Dir, "assets", rel))
	if err != nil {
		return "0"
	}
	sum := sha256.Sum256(data)
	h = hex.EncodeToString(sum[:])[:10]
	e.mu.Lock()
	e.hashes[rel] = h
	e.mu.Unlock()
	return h
}

func (e *Engine) AssetURL(rel string) string {
	return fmt.Sprintf("/assets/%s?v=%s", rel, e.assetHash(rel))
}

func (e *Engine) ServeAsset(w http.ResponseWriter, r *http.Request) {
	rel := r.PathValue("path")
	if rel == "" || strings.Contains(rel, "..") {
		http.NotFound(w, r)
		return
	}
	data, err := os.ReadFile(path.Join(e.Dir, "assets", rel))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ext := strings.ToLower(path.Ext(rel))
	mime := map[string]string{
		".css": "text/css; charset=utf-8", ".js": "text/javascript; charset=utf-8",
		".svg": "image/svg+xml", ".png": "image/png", ".jpg": "image/jpeg",
		".jpeg": "image/jpeg", ".gif": "image/gif", ".woff2": "font/woff2",
		".html": "text/html; charset=utf-8",
	}[ext]
	if mime == "" {
		mime = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(data)
}

func (e *Engine) build() (*template.Template, error) {
	tmpl := template.New("web")
	funcs := template.FuncMap{}
	maps.Copy(funcs, e.Funcs())
	funcs["include"] = func(name string, data any) template.HTML {
		t := tmpl.Lookup(name)
		if t == nil {
			return template.HTML("<!-- missing template: " + name + " -->")
		}
		var b strings.Builder
		if err := t.Execute(&b, data); err != nil {
			return template.HTML("<!-- include error: " + err.Error() + " -->")
		}
		return template.HTML(b.String())
	}
	tmpl.Funcs(funcs)

	root := os.DirFS(e.Dir)
	var files []string
	err := fs.WalkDir(root, "templates", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("render: walk templates: %w", err)
	}
	sort.Strings(files)
	for _, p := range files {
		data, err := fs.ReadFile(root, p)
		if err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(p, "templates/")
		if _, err := tmpl.New(name).Parse(string(data)); err != nil {
			return nil, fmt.Errorf("render: parse %s: %w", p, err)
		}
	}
	return tmpl, nil
}

func (e *Engine) set() (*template.Template, error) {
	e.mu.RLock()
	t := e.cache
	e.mu.RUnlock()
	if t != nil {
		return t, nil
	}
	t, err := e.build()
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	e.cache = t
	e.mu.Unlock()
	return t, nil
}

// AddNav registers a primary navigation entry (idempotent per path).
func (e *Engine) AddNav(label, path, page string, order int) {
	e.addNav(label, path, page, order, false)
}

// AddScopedNav registers a nav entry whose link carries the request's
// applied asset/class scope (Engine.ScopeOf).
func (e *Engine) AddScopedNav(label, path, page string, order int) {
	e.addNav(label, path, page, order, true)
}

func (e *Engine) addNav(label, path, page string, order int, scoped bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, n := range e.nav {
		if n.Path == path {
			return
		}
	}
	e.nav = append(e.nav, NavEntry{Label: label, Path: path, Page: page, Order: order, Scoped: scoped})
}

// Nav returns the registered navigation entries in display order.
func (e *Engine) Nav() []NavEntry {
	e.mu.RLock()
	out := append([]NavEntry{}, e.nav...)
	e.mu.RUnlock()
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// RegisterBlocks contributes an ordered block list for a page; groups render
// ordered by (order, seq).
func (e *Engine) RegisterBlocks(page string, order int, blocks ...string) {
	if page == "" || len(blocks) == 0 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.blockSeq++
	e.blockSet = append(e.blockSet, blockGroup{page: page, order: order, seq: e.blockSeq, blocks: blocks})
}

// ProvideVars registers a template-var contributor for a page. Providers run
// (in registration order) before each render of that page; a panicking
// provider is logged and skipped rather than taking the page down.
func (e *Engine) ProvideVars(page string, fn func(r *http.Request, vars map[string]any)) {
	if page == "" || fn == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.providers = append(e.providers, varProvider{page: page, fn: fn})
}

func (e *Engine) runProviders(page string, r *http.Request, vars map[string]any) {
	e.mu.RLock()
	matching := make([]func(r *http.Request, vars map[string]any), 0, 4)
	for _, p := range e.providers {
		if p.page == page {
			matching = append(matching, p.fn)
		}
	}
	e.mu.RUnlock()
	for _, fn := range matching {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					e.Log.Error("vars provider panicked", "page", page, "panic", rec)
				}
			}()
			fn(r, vars)
		}()
	}
}

func (e *Engine) BlocksFor(page string) []string {
	e.mu.RLock()
	groups := make([]blockGroup, 0, 4)
	for _, g := range e.blockSet {
		if g.page == page {
			groups = append(groups, g)
		}
	}
	e.mu.RUnlock()
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].order != groups[j].order {
			return groups[i].order < groups[j].order
		}
		return groups[i].seq < groups[j].seq
	})
	var out []string
	seen := map[string]bool{}
	for _, g := range groups {
		for _, b := range g.blocks {
			if seen[b] {
				continue
			}
			seen[b] = true
			if _, err := os.Stat(path.Join(e.Dir, "templates", "blocks", page, b+".html")); err == nil {
				out = append(out, b)
			} else {
				e.Log.Warn("block template missing; skipped", "page", page, "block", b)
			}
		}
	}
	return out
}

func (e *Engine) RenderData(page string, vars map[string]any, r *http.Request) map[string]any {
	data := map[string]any{}
	maps.Copy(data, vars)
	data["Page"] = page
	data["Version"] = e.Version
	data["Blocks"] = e.BlocksFor(page)
	if b := e.banner(); b != "" {
		data["Banner"] = b
	}
	settingsMap := map[string]any{}
	if e.Settings != nil {
		if all, ok := e.Settings.(SettingsAll); ok {
			for k, v := range all.All() {
				if settings.IsSecretKey(k) {
					continue
				}
				settingsMap[k] = v
			}
		}
		settingsMap["read_only"] = e.ReadOnly
	}
	data["Settings"] = settingsMap
	data["currency_symbol"] = e.symbol()
	data["Query"] = UICookieValues(r)
	scopeKind, scopeID := "", 0
	if e.ScopeOf != nil && r != nil {
		scopeKind, scopeID = e.ScopeOf(r)
	}
	nav := []map[string]any{}
	for _, n := range e.Nav() {
		p := n.Path
		if n.Scoped && scopeKind != "" && scopeID > 0 {
			p = p + "/" + scopeKind + "/" + strconv.Itoa(scopeID)
		}
		nav = append(nav, map[string]any{"Path": p, "Label": n.Label, "Page": n.Page})
	}
	data["Nav"] = nav
	return data
}

func (e *Engine) RenderPage(w http.ResponseWriter, r *http.Request, page string, vars map[string]any) error {
	data := e.RenderData(page, vars, r)

	if r != nil {
		if c, err := r.Cookie(FlashCookie); err == nil {
			v, _ := url.QueryUnescape(c.Value)
			if after, ok := strings.CutPrefix(v, "ok:"); ok {
				data["FlashOK"] = after
			} else if after, ok := strings.CutPrefix(v, "err:"); ok {
				data["FlashErr"] = after
			}
			http.SetCookie(w, &http.Cookie{Name: FlashCookie, Value: "", Path: "/", MaxAge: -1})
		}
	}

	if r != nil {
		data["CSRF"] = httpapi.EnsureCSRF(w, r)
	}

	e.runProviders(page, r, data)

	tmpl, err := e.set()
	if err != nil {
		http.Error(w, "render error: "+err.Error(), http.StatusInternalServerError)
		return err
	}
	target := page + ".html"
	if tmpl.Lookup(target) == nil {
		http.Error(w, "no template for page "+page, http.StatusInternalServerError)
		return fmt.Errorf("render: no template for page %q", page)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, target, data); err != nil {
		e.Log.Error("render failed", "page", page, "error", err)
		return err
	}
	return nil
}

func (e *Engine) Funcs() template.FuncMap {
	if e.funcs != nil {
		return e.funcs
	}
	e.funcs = template.FuncMap{
		"money": func(v any) string { return formatMoneyWith(v, e.symbol()) },
		"pct":   funcPct,
		"month": funcMonth,
		"asset": e.AssetURL,
		"qs":    funcQS,
		"qp":    funcQP,
		"json":  funcJSON,
		"raw":   funcRaw,
		"add":   funcAdd,
		"sub":   funcSub,
		"seq":   funcSeq,
		"pos":   funcPos,
		"neg":   funcNeg,
	}
	return e.funcs
}

func funcPos(v any) bool { return toFloat(v) >= 0 }
func funcNeg(v any) bool { return toFloat(v) < 0 }

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case nil:
		return 0
	}
	return 0
}

func funcQP(base url.Values, key string) string {
	return base.Get(key)
}

func CurrencySymbol(code string) string {
	switch strings.ToUpper(code) {
	case "", "GBP":
		return "£"
	case "USD", "AUD", "CAD", "NZD", "HKD", "SGD":
		return "$"
	case "EUR":
		return "€"
	case "JPY":
		return "¥"
	case "CHF":
		return "CHF "
	case "SEK", "NOK", "DKK", "ISK":
		return "kr "
	}
	return strings.ToUpper(code) + " "
}

// displayCurrency is fixed: the reporting-currency setting was removed and
// GBP is baked in (CurrencySymbol maps it to £).
const displayCurrency = "GBP"

func (e *Engine) symbol() string {
	return CurrencySymbol(displayCurrency)
}

func formatMoneyWith(v any, symbol string) string {
	var f float64
	switch x := v.(type) {
	case float64:
		f = x
	case int:
		f = float64(x)
	case int64:
		f = float64(x)
	case string:
		fmt.Sscanf(x, "%g", &f)
	}
	neg := f < 0
	if neg {
		f = -f
	}
	whole := int64(f)
	cents := int64((f-float64(whole))*100 + 0.5)
	if cents >= 100 {
		whole++
		cents -= 100
	}
	ws := fmt.Sprintf("%d", whole)
	var parts []string
	for len(ws) > 3 {
		parts = append([]string{ws[len(ws)-3:]}, parts...)
		ws = ws[:len(ws)-3]
	}
	parts = append([]string{ws}, parts...)
	sign := ""
	if neg {
		sign = "-"
	}
	return fmt.Sprintf("%s%s%s.%02d", sign, symbol, strings.Join(parts, ","), cents)
}

func funcPct(v any) string {
	var f float64
	switch x := v.(type) {
	case float64:
		f = x
	case int:
		f = float64(x)
	case nil:
		return "—"
	}
	return fmt.Sprintf("%.2f%%", f*100)
}

func funcMonth(m string) string {
	parts := strings.Split(m, "-")
	if len(parts) != 2 {
		return m
	}
	names := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	var mm int
	if _, err := fmt.Sscanf(parts[1], "%d", &mm); err != nil || mm < 1 || mm > 12 {
		return m
	}
	return fmt.Sprintf("%s %s", names[mm-1], parts[0])
}

func funcQS(base url.Values, kv ...any) template.URL {
	out := url.Values{}
	for k, vs := range base {
		for _, v := range vs {
			out.Add(k, v)
		}
	}
	for i := 0; i+1 < len(kv); i += 2 {
		key := fmt.Sprint(kv[i])
		val := fmt.Sprint(kv[i+1])
		if val == "" || val == "<nil>" {
			out.Del(key)
		} else {
			out.Set(key, val)
		}
	}
	return template.URL(out.Encode())
}

func funcJSON(v any) template.JS {
	b, err := json.Marshal(v)
	if err != nil {
		return template.JS("null")
	}
	s := strings.ReplaceAll(string(b), "</script>", `<\/script>`)
	return template.JS(s)
}

func funcRaw(v any) template.HTML {
	switch x := v.(type) {
	case string:
		return template.HTML(x)
	case template.HTML:
		return x
	}
	return ""
}

func funcAdd(a, b int) int { return a + b }
func funcSub(a, b int) int { return a - b }

func funcSeq(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i + 1
	}
	return out
}
