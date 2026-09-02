package httpapi

import (
	"fmt"
	"net/http"
	"slices"
	"sort"
	"sync"
)

type RouteInfo struct {
	Name    string `json:"name"`
	Method  string `json:"method"`
	Pattern string `json:"pattern"`
	Owner   string `json:"owner"`
}

type route struct {
	name    string
	method  string
	pattern string
	owner   string
	handler http.Handler
}

type routerMW struct {
	name  string
	order int
	mw    func(http.Handler) http.Handler
}

type Router struct {
	mu        sync.RWMutex
	routes    map[string]*route
	order     []string
	mws       []routerMW
	builtMux  *http.ServeMux
	builtName string
}

func NewRouter() *Router {
	return &Router{routes: map[string]*route{}}
}

func (r *Router) Register(name, method, pattern string, h http.Handler) error {
	if name == "" || h == nil {
		return fmt.Errorf("router: route needs a name and a handler")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.routes[name]; exists {
		return fmt.Errorf("router: route name %q already registered", name)
	}
	method = normalizeMethod(method)
	for _, existing := range r.routes {
		if existing.method == method && existing.pattern == pattern {
			return fmt.Errorf("router: pattern %q already registered as %q", method+" "+pattern, existing.name)
		}
	}
	owner := name
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == ':' {
			owner = name[:i]
			break
		}
	}
	r.routes[name] = &route{name: name, method: method, pattern: pattern, owner: owner, handler: h}
	r.order = append(r.order, name)
	r.invalidate()
	return nil
}

func normalizeMethod(method string) string {
	if method == "" || method == "*" {
		return ""
	}
	return method
}

func (r *Router) AddMiddleware(name string, order int, mw func(http.Handler) http.Handler) error {
	if mw == nil {
		return fmt.Errorf("router: nil middleware")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.mws {
		if m.name == name {
			return fmt.Errorf("router: middleware %q already registered", name)
		}
	}
	r.mws = append(r.mws, routerMW{name: name, order: order, mw: mw})
	sort.Slice(r.mws, func(i, j int) bool { return r.mws[i].order < r.mws[j].order })
	r.invalidate()
	return nil
}

func (r *Router) Routes() []RouteInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]RouteInfo, 0, len(r.order))
	for _, name := range r.order {
		rt := r.routes[name]
		out = append(out, RouteInfo{Name: rt.name, Method: rt.method, Pattern: rt.pattern, Owner: rt.owner})
	}
	return out
}

func (r *Router) invalidate() {
	r.builtMux = nil
}

func (r *Router) buildLocked() http.Handler {
	mux := http.NewServeMux()
	for _, name := range r.order {
		rt := r.routes[name]
		h := rt.handler
		pattern := rt.pattern
		if rt.method != "" {
			pattern = rt.method + " " + rt.pattern
		}
		mux.Handle(pattern, h)
	}
	h := http.Handler(mux)
	for _, v := range slices.Backward(r.mws) {
		h = v.mw(h)
	}
	r.builtMux = mux
	return h
}

func (r *Router) Handler() http.Handler {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.builtMux == nil {
		return r.buildLocked()
	}
	return r.chainCached()
}

func (r *Router) chainCached() http.Handler {
	h := http.Handler(r.builtMux)
	for _, v := range slices.Backward(r.mws) {
		h = v.mw(h)
	}
	return h
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.Handler().ServeHTTP(w, req)
}

// Middleware priority: the logger/headers/recover wrappers stay outermost
// (401s must still be logged, carry security headers and survive panics);
// basic auth is the first GATE - unauthenticated traffic is stopped before
// read-only, CSRF or any handler logic can run.
const (
	OrderLogger    = -1000
	OrderHeaders   = -950
	OrderRecover   = -900
	OrderBasicAuth = -850
	OrderReadOnly  = -800
	OrderCSRF      = -700
)
