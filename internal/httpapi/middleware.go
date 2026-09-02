package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"conspectus/internal/settings"
)

type internalKey struct{}

// WithInternalRequest marks a request as originating from this process (the
// loopback API client): it skips CSRF and the basic-auth gate, and its
// accesses are logged at debug level.
func WithInternalRequest(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), internalKey{}, true))
}

func IsInternalRequest(r *http.Request) bool {
	v, _ := r.Context().Value(internalKey{}).(bool)
	return v
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

type MiddlewareDeps struct {
	Log      *slog.Logger
	Settings *settings.Service
	ReadOnly bool
}

func InstallCoreMiddleware(r *Router, d MiddlewareDeps) {
	_ = r.AddMiddleware("core:logger", OrderLogger, requestLogger(d))
	_ = r.AddMiddleware("core:headers", OrderHeaders, secureHeaders())
	_ = r.AddMiddleware("core:recover", OrderRecover, recoverMW(d.Log))
	_ = r.AddMiddleware("core:basicauth", OrderBasicAuth, basicAuthMW(d))
	_ = r.AddMiddleware("core:readonly", OrderReadOnly, readOnlyMW(d))
	_ = r.AddMiddleware("core:csrf", OrderCSRF, csrfMW(d))
}

func requestLogger(d MiddlewareDeps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)
			if sw.status == 0 {
				sw.status = http.StatusOK
			}
			level := slog.LevelInfo
			if IsInternalRequest(r) {
				level = slog.LevelDebug
			}
			d.Log.Log(context.Background(), level, "http",
				"method", r.Method,
				"status", sw.status,
				"path", r.URL.RequestURI(),
				"bytes", sw.bytes,
				"dur", time.Since(start).Round(time.Microsecond).String(),
			)
		})
	}
}

func secureHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "same-origin")
			h.Set("Content-Security-Policy",
				"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
			next.ServeHTTP(w, r)
		})
	}
}

func recoverMW(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if p := recover(); p != nil {
					log.Error("panic recovered", "path", r.URL.Path, "panic", p)
					WriteError(w, http.StatusInternalServerError, "internal", "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func readOnlyMW(d MiddlewareDeps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if d.ReadOnly {
				switch r.Method {
				case http.MethodGet, http.MethodHead, http.MethodOptions:
				default:
					WriteError(w, http.StatusForbidden, "readonly",
						"server is in read-only mode (CONSPECTUS_READ_ONLY); data changes are disabled")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

const csrfCookie = "conspectus_csrf"

func csrfMW(d MiddlewareDeps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if IsInternalRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			if isSafeMethod(r.Method) || isHealthPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			if r.Header.Get("Authorization") != "" {
				next.ServeHTTP(w, r)
				return
			}
			if crossOrigin(r) {
				WriteError(w, http.StatusForbidden, "validation_error", "cross-origin mutation rejected")
				return
			}
			cookie, err := r.Cookie(csrfCookie)
			if err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			token := r.Header.Get("X-CSRF-Token")
			if token == "" {
				token = formCSRFToken(r)
			}
			if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(cookie.Value)) != 1 {
				WriteError(w, http.StatusForbidden, "unauthorized", "missing or invalid CSRF token (send X-CSRF-Token matching the conspectus_csrf cookie)")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isSafeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

func isHealthPath(p string) bool {
	return p == "/healthz" || p == "/readyz"
}

func formCSRFToken(r *http.Request) string {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			return ""
		}
		return r.PostFormValue("csrf_token")
	}
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return ""
		}
		return r.FormValue("csrf_token")
	}
	return ""
}

func crossOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		if ref := r.Header.Get("Referer"); ref != "" {
			origin = ref
		} else {
			return false
		}
	}
	host := r.Host
	if o := extractHost(origin); o != "" && o != host {
		return true
	}
	return false
}

func extractHost(originOrReferer string) string {
	s := originOrReferer
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	} else if strings.HasPrefix(s, "//") {
		s = s[2:]
	}
	if i := strings.IndexAny(s, "/"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

func NewCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(errors.New("csrf: rand unavailable: " + err.Error()))
	}
	return hex.EncodeToString(b)
}

func EnsureCSRF(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(csrfCookie); err == nil && c.Value != "" {
		return c.Value
	}
	t := NewCSRFToken()
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookie,
		Value:    t,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})
	return t
}
