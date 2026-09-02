package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
)

// basicAuthMW gates the whole app behind HTTP basic auth (or a bearer
// token). It is inert until users/tokens are configured: credentials come
// from CONSPECTUS_BASICAUTH_USERS / CONSPECTUS_BASICAUTH_TOKENS (user:sha256
// pairs and token sha256 hashes), falling back to the same-named settings
// keys. It runs FIRST among the gates (before read-only and CSRF) and
// challenges every method except HEAD and OPTIONS; /healthz and /readyz
// stay open so container healthchecks need no credentials (/healthz is a
// constant "ok", /readyz only reveals internal db/schema state). Loopback
// requests from this process pass through - they never leave it.
func basicAuthMW(d MiddlewareDeps) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if IsInternalRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			switch r.Method {
			case http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}
			if isHealthPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			users, tokens := basicAuthCredentials(d)
			if len(users) == 0 && len(tokens) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			auth := r.Header.Get("Authorization")
			if auth == "" {
				basicAuthChallenge(w)
				return
			}
			if strings.HasPrefix(auth, "Basic ") {
				user, pass, ok := r.BasicAuth()
				if !ok {
					basicAuthChallenge(w)
					return
				}
				want, known := users[strings.ToLower(user)]
				if !known || !hashEqual(pass, want) {
					d.Log.Warn("basic auth failed", "remote", r.RemoteAddr)
					basicAuthChallenge(w)
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
				token := strings.TrimSpace(after)
				for _, want := range tokens {
					if hashEqual(token, want) {
						next.ServeHTTP(w, r)
						return
					}
				}
				d.Log.Warn("bearer token rejected", "remote", r.RemoteAddr)
			}
			basicAuthChallenge(w)
		})
	}
}

func basicAuthChallenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="conspectus", charset="UTF-8"`)
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

func basicAuthCredentials(d MiddlewareDeps) (map[string]string, []string) {
	users := map[string]string{}
	tokens := []string{}

	parseUsers := func(spec string) {
		for pair := range strings.SplitSeq(spec, ",") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			i := strings.Index(pair, ":")
			if i <= 0 {
				continue
			}
			users[strings.ToLower(pair[:i])] = pair[i+1:]
		}
	}
	parseTokens := func(spec string) {
		for t := range strings.SplitSeq(spec, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tokens = append(tokens, t)
			}
		}
	}

	if v := os.Getenv("CONSPECTUS_BASICAUTH_USERS"); v != "" {
		parseUsers(v)
	} else if d.Settings != nil {
		if v := d.Settings.Get("basicauth_users"); v != "" {
			parseUsers(v)
		}
	}
	if v := os.Getenv("CONSPECTUS_BASICAUTH_TOKENS"); v != "" {
		parseTokens(v)
	} else if d.Settings != nil {
		if v := d.Settings.Get("basicauth_tokens"); v != "" {
			parseTokens(v)
		}
	}
	return users, tokens
}

func hashEqual(secret, wantHex string) bool {
	sum := sha256.Sum256([]byte(secret))
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(strings.ToLower(wantHex))) == 1
}
