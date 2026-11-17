package middleware

import (
	"net/http"
	"strings"
)

const (
	corsAllowMethods = "GET, POST, PATCH, PUT, DELETE, OPTIONS"
	corsAllowHeaders = "Authorization, Content-Type"
	corsMaxAge       = "600"
)

// CORS allows exact origins only. Bearer tokens are used, so credentials are
// deliberately not enabled and a wildcard is never returned.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		normalized := strings.ToLower(strings.TrimRight(strings.TrimSpace(origin), "/"))
		if normalized != "" {
			allowed[normalized] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Origin")

			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			normalized := strings.ToLower(strings.TrimRight(origin, "/"))
			if _, ok := allowed[normalized]; !ok {
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", corsAllowMethods)
				w.Header().Set("Access-Control-Allow-Headers", corsAllowHeaders)
				w.Header().Set("Access-Control-Max-Age", corsMaxAge)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
