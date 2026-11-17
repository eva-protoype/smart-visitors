package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	nextCalled := false
	handler := CORS([]string{"https://project-dawnguard.github.io"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	request.Header.Set("Origin", "https://project-dawnguard.github.io")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if !nextCalled {
		t.Fatal("next handler was not called")
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://project-dawnguard.github.io" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want configured origin", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want empty (bearer tokens only)", got)
	}
	if got := response.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Fatalf("Vary = %q, want Origin", got)
	}
}

func TestCORSDoesNotAllowUnknownOrigin(t *testing.T) {
	nextCalled := false
	handler := CORS([]string{"https://project-dawnguard.github.io"})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if !nextCalled {
		t.Fatal("next handler should still run for a normal request; the browser enforces CORS")
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want empty", got)
	}
}

func TestCORSAnswersAllowedPreflightBeforeNextHandler(t *testing.T) {
	nextCalled := false
	handler := CORS([]string{"https://project-dawnguard.github.io"})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))

	request := httptest.NewRequest(http.MethodOptions, "/api/auth/login", nil)
	request.Header.Set("Origin", "https://project-dawnguard.github.io")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if nextCalled {
		t.Fatal("preflight reached the auth/routing chain")
	}
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://project-dawnguard.github.io" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want configured origin", got)
	}
	for _, method := range []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"} {
		if !strings.Contains(response.Header().Get("Access-Control-Allow-Methods"), method) {
			t.Fatalf("Access-Control-Allow-Methods = %q, missing %s", response.Header().Get("Access-Control-Allow-Methods"), method)
		}
	}
	for _, header := range []string{"Authorization", "Content-Type"} {
		if !strings.Contains(response.Header().Get("Access-Control-Allow-Headers"), header) {
			t.Fatalf("Access-Control-Allow-Headers = %q, missing %s", response.Header().Get("Access-Control-Allow-Headers"), header)
		}
	}
	if got := response.Header().Get("Access-Control-Max-Age"); got != "600" {
		t.Fatalf("Access-Control-Max-Age = %q, want 600", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want empty", got)
	}
}

func TestCORSRejectsUnknownPreflight(t *testing.T) {
	nextCalled := false
	handler := CORS([]string{"https://project-dawnguard.github.io"})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))

	request := httptest.NewRequest(http.MethodOptions, "/api/me", nil)
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if nextCalled {
		t.Fatal("unknown preflight reached the next handler")
	}
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestCORSPassesThroughRequestWithoutOrigin(t *testing.T) {
	nextCalled := false
	handler := CORS(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodOptions, "/api/me", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if !nextCalled {
		t.Fatal("non-CORS request should reach the next handler")
	}
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
}
