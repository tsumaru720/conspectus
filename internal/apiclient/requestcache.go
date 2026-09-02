package apiclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// A page render fetches its data through several independent helpers, some of
// which ask the API for the same URL more than once (settings, the scope
// picker's asset/class lookups). The request cache lets all fetches made
// while rendering one page share a single round-trip per URL: the inbound
// page request carries a cache in its context, and every loopback/remote GET
// made with that context (or a child of it) reads and fills it.

type reqCacheKey struct{}

// RequestCache memoizes GET responses for one inbound page request. It is
// safe for concurrent use; entries live exactly as long as the request.
type RequestCache struct {
	mu      sync.Mutex
	entries map[string]*cachedResponse
}

// WithRequestCache returns a context carrying a fresh, empty cache.
func WithRequestCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, reqCacheKey{}, &RequestCache{entries: map[string]*cachedResponse{}})
}

func cacheFrom(ctx context.Context) *RequestCache {
	c, _ := ctx.Value(reqCacheKey{}).(*RequestCache)
	return c
}

func (c *RequestCache) get(key string) (*cachedResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	hit, ok := c.entries[key]
	return hit, ok
}

func (c *RequestCache) put(key string, cr *cachedResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cr
}

type cachedResponse struct {
	status int
	header http.Header
	body   []byte
}

func (cr *cachedResponse) response() *http.Response {
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", cr.status, http.StatusText(cr.status)),
		StatusCode:    cr.status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        cr.header.Clone(),
		Body:          io.NopCloser(bytes.NewReader(cr.body)),
		ContentLength: int64(len(cr.body)),
		Request:       &http.Request{Method: http.MethodGet},
	}
}

// cachingRT serves repeated GETs from the context's RequestCache. Only 200
// responses are cached; other methods and statuses, and requests whose
// context carries no cache, pass through untouched.
type cachingRT struct {
	base http.RoundTripper
}

func (rt *cachingRT) RoundTrip(req *http.Request) (*http.Response, error) {
	cache := cacheFrom(req.Context())
	if req.Method != http.MethodGet || cache == nil {
		return rt.base.RoundTrip(req)
	}
	key := req.URL.RequestURI()
	if hit, ok := cache.get(key); ok {
		return hit.response(), nil
	}
	resp, err := rt.base.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return resp, err
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	hit := &cachedResponse{status: resp.StatusCode, header: resp.Header.Clone(), body: body}
	cache.put(key, hit)
	return hit.response(), nil
}
