package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestCORSPreflightBypassesAuthMiddleware(t *testing.T) {
	authCalled := false
	router := chi.NewRouter()
	router.Route("/api", func(api chi.Router) {
		api.Use(CORS([]string{"https://project-dawnguard.github.io"}))
		api.With(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				authCalled = true
				w.WriteHeader(http.StatusUnauthorized)
			})
		}).Get("/me", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	})

	request := httptest.NewRequest(http.MethodOptions, "/api/me", nil)
	request.Header.Set("Origin", "https://project-dawnguard.github.io")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if authCalled {
		t.Fatal("auth middleware ran before the CORS preflight response")
	}
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://project-dawnguard.github.io" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}
