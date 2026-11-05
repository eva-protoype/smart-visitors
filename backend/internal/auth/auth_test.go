package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPasswordHashVerification(t *testing.T) {
	password := "visitor-development-password"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("password hash is not Argon2id: %q", hash)
	}
	if !VerifyPassword(hash, password) {
		t.Fatal("correct password did not verify")
	}
	if VerifyPassword(hash, password+"-wrong") {
		t.Fatal("incorrect password verified")
	}
	if VerifyPassword("$argon2id$not-a-valid-hash", password) {
		t.Fatal("malformed password hash verified")
	}
}

func TestJWTValidation(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	user := User{ID: "user-1", Role: "host", Name: "Visitor Host", TokenVersion: 3}
	secret := []byte(strings.Repeat("a", 32))
	token, expires, err := SignToken(user, secret, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := VerifyToken(token, secret, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != user.ID || claims.Role != user.Role || claims.TokenVersion != user.TokenVersion {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if !expires.Equal(now.Add(time.Hour)) {
		t.Fatalf("unexpected expiration: %s", expires)
	}
	if _, err := VerifyToken(token, secret, now.Add(time.Hour)); err == nil {
		t.Fatal("expired token verified")
	}
	if _, err := VerifyToken(token, []byte(strings.Repeat("b", 32)), now); err == nil {
		t.Fatal("token verified with the wrong secret")
	}
}

func TestBearerTokenAcceptsSessionCookie(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/me", nil)
	request.AddCookie(&http.Cookie{Name: "sv_session", Value: "cookie-token"})
	token, ok := bearerToken(request)
	if !ok || token != "cookie-token" {
		t.Fatalf("expected session cookie token, got %q, ok=%t", token, ok)
	}

	request.Header.Set("Authorization", "Bearer header-token")
	token, ok = bearerToken(request)
	if !ok || token != "header-token" {
		t.Fatalf("expected bearer token precedence, got %q, ok=%t", token, ok)
	}

	request.Header.Set("Authorization", "")
	request.Header.Del("Cookie")
	if _, ok := bearerToken(request); ok {
		t.Fatal("accepted request without an authorization token")
	}
}
