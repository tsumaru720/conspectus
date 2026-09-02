package apiclient

import (
	"context"
	"net/http"
	"testing"
)

func TestRequestCacheSharesGETs(t *testing.T) {
	hits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/thing", func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"n":42}}`))
	})
	client := NewLoopback(mux)
	ctx := WithRequestCache(context.Background())

	var first, second struct {
		N int `json:"n"`
	}
	if err := client.Get(ctx, "/api/v1/thing", &first); err != nil {
		t.Fatalf("first get: %v", err)
	}
	if err := client.Get(ctx, "/api/v1/thing", &second); err != nil {
		t.Fatalf("second get: %v", err)
	}
	if hits != 1 {
		t.Fatalf("backend hit %d times for two identical GETs, want 1", hits)
	}
	if first.N != 42 || second.N != 42 {
		t.Fatalf("cached decode = %d/%d, want 42/42", first.N, second.N)
	}
}

func TestRequestCacheScopes(t *testing.T) {
	hits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/thing", func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"data":{}}`))
	})
	client := NewLoopback(mux)
	ctx := WithRequestCache(context.Background())

	// A context without a cache always reaches the backend.
	if err := client.Get(context.Background(), "/api/v1/thing", nil); err != nil {
		t.Fatalf("uncached get: %v", err)
	}
	// Different URLs within one cache are separate entries.
	if err := client.Get(ctx, "/api/v1/thing?x=1", nil); err != nil {
		t.Fatalf("get x=1: %v", err)
	}
	if err := client.Get(ctx, "/api/v1/thing?x=2", nil); err != nil {
		t.Fatalf("get x=2: %v", err)
	}
	// Non-GET requests are never served from or put into the cache.
	if err := client.Post(ctx, "/api/v1/thing", map[string]any{"x": 1}, nil); err != nil {
		t.Fatalf("post: %v", err)
	}
	if err := client.Get(ctx, "/api/v1/thing", nil); err != nil {
		t.Fatalf("get bare: %v", err)
	}
	// The repeat of x=1 is served from the cache.
	if err := client.Get(ctx, "/api/v1/thing?x=1", nil); err != nil {
		t.Fatalf("repeat x=1: %v", err)
	}
	if hits != 5 {
		t.Fatalf("backend hit %d times, want 5", hits)
	}
}
