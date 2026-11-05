package handlers

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"smart-visitors/internal/auth"
	"smart-visitors/internal/config"
	"smart-visitors/internal/cryptoutil"
	"smart-visitors/internal/db"

	"github.com/google/uuid"
)

var loginAttempts = newRequestLimiter()
var dummyPasswordHash, _ = auth.HashPassword("not-a-real-bootstrap-password")

func Bootstrap(ctx context.Context, database *sql.DB, cfg config.Config) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var adminCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&adminCount); err != nil {
		return err
	}
	var adminID string
	if adminCount == 0 {
		if cfg.BootstrapAdminPass == "" {
			return fmt.Errorf("BOOTSTRAP_ADMIN_PASSWORD is required when no admin account exists")
		}
		if len(cfg.BootstrapAdminPass) < 12 {
			return fmt.Errorf("BOOTSTRAP_ADMIN_PASSWORD must be at least 12 characters")
		}
		adminID = uuid.NewString()
		passwordHash, err := auth.HashPassword(cfg.BootstrapAdminPass)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO users (id, role, username, password_hash, name, must_reset_password)
			VALUES ($1, 'admin', $2, $3, 'Administrator', true)`,
			adminID, cfg.BootstrapAdminUser, passwordHash); err != nil {
			return fmt.Errorf("create bootstrap admin: %w", err)
		}
		if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
			ActorID: &adminID, Action: "user.bootstrap", Target: adminID,
		}); err != nil {
			return err
		}
	}

	for _, gate := range []string{"Main Gate", "Back Gate", "East Wing", "Parking"} {
		id := uuid.NewString()
		result, err := tx.ExecContext(ctx, `
			INSERT INTO gates (id, name) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`, id, gate)
		if err != nil {
			return fmt.Errorf("seed gate: %w", err)
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if inserted > 0 {
			if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
				ActorID: nil, Action: "gate.create", Target: id,
				Meta: map[string]any{"name": gate, "source": "bootstrap"},
			}); err != nil {
				return err
			}
		}
	}

	var issuerExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM keys WHERE name = 'issuer-ed25519')`).Scan(&issuerExists); err != nil {
		return err
	}
	if !issuerExists {
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return fmt.Errorf("generate issuer signing key: %w", err)
		}
		wrapped, err := cryptoutil.Seal(cfg.MasterKey, privateKey, []byte("issuer-ed25519"))
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO keys (name, wrapped) VALUES ('issuer-ed25519', $1)`, wrapped); err != nil {
			return fmt.Errorf("store issuer signing key: %w", err)
		}
	}

	return tx.Commit()
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Login(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input loginRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		input.Username = strings.TrimSpace(input.Username)
		if input.Username == "" || input.Password == "" {
			writeErr(w, http.StatusUnprocessableEntity, "username and password are required.")
			return
		}
		if !loginAttempts.allow(loginKey(input.Username, r.RemoteAddr), cfg.RateLimitLogin, time.Hour) {
			w.Header().Set("Retry-After", "3600")
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error": map[string]string{"code": "RATE_LIMITED", "message": "Too many login attempts. Try again later."},
			})
			return
		}

		var user auth.User
		var passwordHash string
		var tempExpires, expiresAt, disabledAt sql.NullTime
		var email, phone sql.NullString
		err := database.QueryRowContext(r.Context(), `
			SELECT id::text, role, username, name, email, phone, password_hash,
			       must_reset_password, token_version, temp_expires_at, expires_at, disabled_at
			FROM users WHERE username = $1`, input.Username).Scan(
			&user.ID, &user.Role, &user.Username, &user.Name, &email, &phone,
			&passwordHash, &user.MustResetPassword, &user.TokenVersion, &tempExpires, &expiresAt, &disabledAt)
		found := err == nil
		if !found {
			passwordHash = dummyPasswordHash
		}
		passwordOK := auth.VerifyPassword(passwordHash, input.Password)
		if err != nil && err != sql.ErrNoRows {
			writeErr(w, http.StatusInternalServerError, "Could not authenticate.")
			return
		}

		if !found || !passwordOK {
			if err := appendStandaloneAudit(r.Context(), database, db.AuditEvent{
				Action: "login.failure", Target: "authentication",
			}); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not record authentication attempt.")
				return
			}
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"code": "INVALID_CREDENTIALS", "message": "Username or password is incorrect."},
			})
			return
		}
		if disabledAt.Valid || (expiresAt.Valid && time.Now().After(expiresAt.Time)) {
			code, message := "ACCOUNT_DISABLED", "This account is disabled."
			if disabledAt.Valid {
				code, message = "ACCOUNT_DISABLED", "This account is disabled."
			} else {
				code, message = "ACCOUNT_EXPIRED", "This account has expired."
			}
			if err := appendStandaloneAudit(r.Context(), database, db.AuditEvent{
				Action: "login.failure", Target: user.ID, Meta: map[string]any{"code": code},
			}); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not record authentication attempt.")
				return
			}
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error": map[string]string{"code": code, "message": message},
			})
			return
		}
		if tempExpires.Valid && time.Now().After(tempExpires.Time) {
			if err := appendStandaloneAudit(r.Context(), database, db.AuditEvent{
				Action: "login.failure", Target: user.ID, Meta: map[string]any{"code": "TEMP_PASSWORD_EXPIRED"},
			}); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not record authentication attempt.")
				return
			}
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"code": "TEMP_PASSWORD_EXPIRED", "message": "This temporary password expired. Ask an administrator to reset it."},
			})
			return
		}

		user.Email, user.Phone = nullStringPtr(email), nullStringPtr(phone)
		if expiresAt.Valid {
			user.ExpiresAt = &expiresAt.Time
		}
		token, expiry, err := auth.SignToken(user, cfg.JWTSecret, time.Duration(cfg.TokenTTLHours)*time.Hour, time.Now())
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create session.")
			return
		}
		if err := appendStandaloneAudit(r.Context(), database, db.AuditEvent{
			ActorID: &user.ID, Action: "login.success", Target: user.ID,
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record login.")
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name: "sv_session", Value: token, Path: "/", HttpOnly: true,
			Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
			SameSite: http.SameSiteLaxMode, Expires: expiry, MaxAge: int(time.Until(expiry).Seconds()),
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"token": token, "expires_at": expiry.UTC().Format(time.RFC3339), "user": user,
		})
	}
}

func Me(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "Authentication is required.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: "sv_session", Value: "", Path: "/", HttpOnly: true,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func ChangePassword(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		var input struct {
			OldPassword string `json:"old"`
			NewPassword string `json:"new"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		if len(input.NewPassword) < 12 {
			writeErr(w, http.StatusUnprocessableEntity, "new password must be at least 12 characters.")
			return
		}

		ctx := r.Context()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not update password.")
			return
		}
		defer tx.Rollback()

		var currentHash string
		if err := tx.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = $1 FOR UPDATE`, user.ID).Scan(&currentHash); err != nil {
			writeErr(w, http.StatusUnauthorized, "Account is no longer available.")
			return
		}
		if !auth.VerifyPassword(currentHash, input.OldPassword) {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"code": "INVALID_CREDENTIALS", "message": "Current password is incorrect."},
			})
			return
		}
		newHash, err := auth.HashPassword(input.NewPassword)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not update password.")
			return
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE users SET password_hash = $1, must_reset_password = false,
				temp_expires_at = NULL, token_version = token_version + 1
			WHERE id = $2`, newHash, user.ID); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not update password.")
			return
		}
		if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
			ActorID: &user.ID, Action: "me.password_change", Target: user.ID,
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record password change.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not update password.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func appendStandaloneAudit(ctx context.Context, database *sql.DB, event db.AuditEvent) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := db.AppendAuditEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func loginKey(username, remoteAddress string) string {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err == nil {
		remoteAddress = host
	}
	return strings.ToLower(username) + "|" + remoteAddress
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
