package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testLog() *slog.Logger { return slog.New(slog.DiscardHandler) }

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func newAuthServer(t *testing.T, usersEnv string) *Router {
	t.Helper()
	t.Setenv("CONSPECTUS_BASICAUTH_USERS", usersEnv)
	t.Setenv("CONSPECTUS_BASICAUTH_TOKENS", "")
	router := NewRouter()
	InstallCoreMiddleware(router, MiddlewareDeps{Log: testLog()})
	if err := router.Register("test:ok", "GET", "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})); err != nil {
		t.Fatal(err)
	}
	return router
}

func TestBasicAuthGate(t *testing.T) {
	// simon:sha256("letmein")
	router := newAuthServer(t, "simon:"+sha256Hex("letmein"))

	unauth := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, unauth)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no credentials: status = %d, want 401", rec.Code)
	}

	bad := httptest.NewRequest("GET", "/", nil)
	bad.SetBasicAuth("simon", "wrong")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, bad)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: status = %d, want 401", rec.Code)
	}

	good := httptest.NewRequest("GET", "/", nil)
	good.SetBasicAuth("simon", "letmein")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, good)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("correct credentials: status = %d body = %q", rec.Code, rec.Body.String())
	}
}

func TestBasicAuthInertWithoutCredentials(t *testing.T) {
	router := newAuthServer(t, "")
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("no configured credentials must not gate: status = %d", rec.Code)
	}
}

func TestBasicAuthExemptions(t *testing.T) {
	router := newAuthServer(t, "simon:"+sha256Hex("letmein"))
	if err := router.Register("test:health", "GET", "/healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})); err != nil {
		t.Fatal(err)
	}

	// HEAD and OPTIONS pass without credentials.
	for _, m := range []string{http.MethodHead, http.MethodOptions} {
		req := httptest.NewRequest(m, "/", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized {
			t.Fatalf("%s must not be gated", m)
		}
	}

	// Health endpoints stay open so container healthchecks need no creds.
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz gated: status = %d", rec.Code)
	}
}

func TestBasicAuthBeforeReadOnly(t *testing.T) {
	t.Setenv("CONSPECTUS_BASICAUTH_USERS", "simon:"+sha256Hex("letmein"))
	t.Setenv("CONSPECTUS_BASICAUTH_TOKENS", "")
	router := NewRouter()
	InstallCoreMiddleware(router, MiddlewareDeps{Log: testLog(), ReadOnly: true})
	if err := router.Register("test:post", "POST", "/api/v1/thing", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler reached without credentials")
	})); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/thing", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated POST = %d, want 401 (auth must gate before read-only)", rec.Code)
	}
}
