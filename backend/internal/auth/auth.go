package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

type User struct {
	ID                string     `json:"id"`
	Role              string     `json:"role"`
	Username          string     `json:"username"`
	Name              string     `json:"name"`
	Email             *string    `json:"email,omitempty"`
	Phone             *string    `json:"phone,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	MustResetPassword bool       `json:"must_reset_password"`
	TokenVersion      int        `json:"-"`
	DisabledAt        *time.Time `json:"-"`
}

type Claims struct {
	Subject      string `json:"sub"`
	Role         string `json:"role"`
	Name         string `json:"name"`
	TokenVersion int    `json:"token_version"`
	IssuedAt     int64  `json:"iat"`
	ExpiresAt    int64  `json:"exp"`
}

type contextKey struct{}
type metadataKey struct{}

type RequestMetadata struct {
	Role string
}

func WithRequestMetadata(ctx context.Context) context.Context {
	return context.WithValue(ctx, metadataKey{}, &RequestMetadata{Role: "public"})
}

func SetRequestRole(ctx context.Context, role string) {
	if metadata, ok := ctx.Value(metadataKey{}).(*RequestMetadata); ok {
		metadata.Role = role
	}
}

func RequestRole(ctx context.Context) string {
	if metadata, ok := ctx.Value(metadataKey{}).(*RequestMetadata); ok {
		return metadata.Role
	}
	return "public"
}

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, contextKey{}, user)
}

func UserFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(contextKey{}).(User)
	return user, ok
}

func HashPassword(password string) (string, error) {
	if len(password) < 12 {
		return "", errors.New("password must be at least 12 characters")
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, 2, 64*1024, 1, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=2,p=1$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || memory < 8*1024 || memory > 256*1024 ||
		iterations == 0 || iterations > 10 || parallelism == 0 || parallelism > 16 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) < 16 || len(expected) > 64 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return hmac.Equal(actual, expected)
}

func GenerateTemporaryPassword() (string, error) {
	var bytes [24]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("generate temporary password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:]), nil
}

func SignToken(user User, secret []byte, ttl time.Duration, now time.Time) (string, time.Time, error) {
	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", time.Time{}, err
	}

	expiry := now.Add(ttl)
	claims, err := json.Marshal(Claims{
		Subject:      user.ID,
		Role:         user.Role,
		Name:         user.Name,
		TokenVersion: user.TokenVersion,
		IssuedAt:     now.Unix(),
		ExpiresAt:    expiry.Unix(),
	})
	if err != nil {
		return "", time.Time{}, err
	}

	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	token := unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return token, expiry, nil
}

func VerifyToken(token string, secret []byte, now time.Time) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("invalid token")
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, errors.New("invalid token")
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil || header.Algorithm != "HS256" || header.Type != "JWT" {
		return Claims{}, errors.New("invalid token")
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, errors.New("invalid token")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return Claims{}, errors.New("invalid token")
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, errors.New("invalid token")
	}
	var claims Claims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil ||
		claims.Subject == "" || claims.ExpiresAt <= now.Unix() ||
		claims.IssuedAt > now.Add(time.Minute).Unix() || claims.TokenVersion < 0 {
		return Claims{}, errors.New("invalid or expired token")
	}
	return claims, nil
}

func Middleware(secret []byte, lookup func(context.Context, string) (User, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				writeUnauthorized(w)
				return
			}
			claims, err := VerifyToken(token, secret, time.Now())
			if err != nil {
				writeUnauthorized(w)
				return
			}
			user, err := lookup(r.Context(), claims.Subject)
			if err != nil || user.DisabledAt != nil ||
				(user.ExpiresAt != nil && time.Now().After(*user.ExpiresAt)) ||
				user.TokenVersion != claims.TokenVersion ||
				user.Role != claims.Role {
				writeUnauthorized(w)
				return
			}
			path := strings.TrimPrefix(r.URL.Path, "/api")
			if user.MustResetPassword && path != "/me/password" && path != "/auth/logout" && path != "/me" {
				writeAuthError(w, http.StatusForbidden, "PASSWORD_RESET_REQUIRED", "Change your temporary password before continuing.")
				return
			}
			SetRequestRole(r.Context(), user.Role)
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer "), true
	}
	cookie, err := r.Cookie("sv_session")
	if err != nil || cookie.Value == "" {
		return "", false
	}
	return cookie.Value, true
}

func RequireRoles(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				writeUnauthorized(w)
				return
			}
			if !allowed[user.Role] {
				writeAuthError(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to perform this action.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	writeAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required.")
}

func writeAuthError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
