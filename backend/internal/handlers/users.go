package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"smart-visitors/internal/auth"
	"smart-visitors/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var validAccountRoles = map[string]bool{"admin": true, "host": true, "guard": true}

func LookupUser(ctx context.Context, database *sql.DB, id string) (auth.User, error) {
	var user auth.User
	var email, phone sql.NullString
	var expiresAt, disabledAt sql.NullTime
	err := database.QueryRowContext(ctx, `
		SELECT id::text, role, username, name, email, phone, must_reset_password,
		       token_version, expires_at, disabled_at
		FROM users WHERE id = $1`, id).Scan(
		&user.ID, &user.Role, &user.Username, &user.Name, &email, &phone,
		&user.MustResetPassword, &user.TokenVersion, &expiresAt, &disabledAt)
	if err != nil {
		return auth.User{}, err
	}
	if email.Valid {
		user.Email = &email.String
	}
	if phone.Valid {
		user.Phone = &phone.String
	}
	if expiresAt.Valid {
		user.ExpiresAt = &expiresAt.Time
	}
	if disabledAt.Valid {
		user.DisabledAt = &disabledAt.Time
	}
	return user, nil
}

type UserCreate struct {
	Role     string     `json:"role"`
	Name     string     `json:"name"`
	Username string     `json:"username"`
	Email    *string    `json:"email"`
	Phone    *string    `json:"phone"`
	Expires  *time.Time `json:"expires_at"`
}

func UsersCreate(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input UserCreate
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		input.Username = strings.TrimSpace(input.Username)
		if !validAccountRoles[input.Role] || input.Role == "admin" {
			writeErr(w, http.StatusUnprocessableEntity, "role must be host or guard.")
			return
		}
		if input.Name == "" || input.Username == "" {
			writeErr(w, http.StatusUnprocessableEntity, "name and username are required.")
			return
		}
		if input.Expires != nil && !input.Expires.After(time.Now()) {
			writeErr(w, http.StatusUnprocessableEntity, "expires_at must be in the future.")
			return
		}

		temporaryPassword, err := auth.GenerateTemporaryPassword()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create temporary credentials.")
			return
		}
		passwordHash, err := auth.HashPassword(temporaryPassword)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create temporary credentials.")
			return
		}
		tempExpiresAt := time.Now().Add(24 * time.Hour)

		user, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		id := uuid.NewString()
		ctx := r.Context()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create account.")
			return
		}
		defer tx.Rollback()

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO users (id, role, username, password_hash, name, email, phone,
			                   must_reset_password, temp_expires_at, expires_at, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, true, $8, $9, $10)`,
			id, input.Role, input.Username, passwordHash, input.Name, input.Email,
			input.Phone, tempExpiresAt, input.Expires, user.ID); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
				strings.Contains(strings.ToLower(err.Error()), "unique") {
				writeErr(w, http.StatusConflict, "A user with that username or email already exists.")
			} else {
				writeErr(w, http.StatusInternalServerError, "Could not create account.")
			}
			return
		}
		if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
			ActorID: &user.ID,
			Action:  "user.create",
			Target:  id,
			Meta:    map[string]any{"role": input.Role},
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record account creation.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create account.")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"user":                     map[string]any{"id": id, "role": input.Role, "username": input.Username, "name": input.Name, "email": input.Email, "phone": input.Phone, "must_reset_password": true},
			"temp_password":            temporaryPassword,
			"temp_password_expires_at": tempExpiresAt.UTC().Format(time.RFC3339),
		})
	}
}

func UsersList(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if value := r.URL.Query().Get("limit"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 {
				writeErr(w, http.StatusBadRequest, "limit must be a positive integer.")
				return
			}
			limit = parsed
			if limit > 100 {
				limit = 100
			}
		}
		role, query := r.URL.Query().Get("role"), strings.TrimSpace(r.URL.Query().Get("q"))
		if role != "" && !validAccountRoles[role] {
			writeErr(w, http.StatusBadRequest, "Unknown account role.")
			return
		}

		rows, err := database.QueryContext(r.Context(), `
			SELECT id::text, role, username, name, email, phone,
			       must_reset_password, temp_expires_at, expires_at, disabled_at, created_at
			FROM users
			WHERE ($1 = '' OR role = $1)
			  AND ($2 = '' OR username ILIKE '%' || $2 || '%' OR name ILIKE '%' || $2 || '%')
			ORDER BY created_at DESC LIMIT $3`, role, query, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load accounts.")
			return
		}
		defer rows.Close()

		users := make([]map[string]any, 0)
		for rows.Next() {
			var id, rowRole, username, name string
			var email, phone sql.NullString
			var mustReset bool
			var tempExpires, expiresAt, disabledAt sql.NullTime
			var createdAt time.Time
			if err := rows.Scan(&id, &rowRole, &username, &name, &email, &phone,
				&mustReset, &tempExpires, &expiresAt, &disabledAt, &createdAt); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not load accounts.")
				return
			}
			users = append(users, map[string]any{
				"id": id, "role": rowRole, "username": username, "name": name,
				"email": nullString(email), "phone": nullString(phone),
				"must_reset_password": mustReset, "temp_expires_at": nullTime(tempExpires),
				"expires_at": nullTime(expiresAt), "disabled_at": nullTime(disabledAt), "created_at": createdAt,
			})
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load accounts.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": users, "next_cursor": nil})
	}
}

func HostsList(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, err := queryLimit(r, 50)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "limit must be between 1 and 100.")
			return
		}
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		rows, err := database.QueryContext(r.Context(), `
			SELECT id::text, username, name FROM users
			WHERE role = 'host' AND disabled_at IS NULL
			  AND ($1 = '' OR username ILIKE '%' || $1 || '%' OR name ILIKE '%' || $1 || '%')
			ORDER BY name LIMIT $2`, query, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load hosts.")
			return
		}
		defer rows.Close()
		hosts := make([]map[string]string, 0)
		for rows.Next() {
			var id, username, name string
			if err := rows.Scan(&id, &username, &name); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not load hosts.")
				return
			}
			hosts = append(hosts, map[string]string{"id": id, "username": username, "name": name})
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load hosts.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": hosts, "next_cursor": nil})
	}
}

func UserResetPassword(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		targetID := chi.URLParam(r, "id")
		temporaryPassword, err := auth.GenerateTemporaryPassword()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not reset credentials.")
			return
		}
		passwordHash, err := auth.HashPassword(temporaryPassword)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not reset credentials.")
			return
		}
		tempExpiresAt := time.Now().Add(24 * time.Hour)

		ctx := r.Context()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not reset credentials.")
			return
		}
		defer tx.Rollback()
		result, err := tx.ExecContext(ctx, `
			UPDATE users SET password_hash = $1, must_reset_password = true,
				temp_expires_at = $2, token_version = token_version + 1
				WHERE id = $3 AND disabled_at IS NULL`, passwordHash, tempExpiresAt, targetID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not reset credentials.")
			return
		}
		affected, err := result.RowsAffected()
		if err != nil || affected == 0 {
			writeErr(w, http.StatusNotFound, "Account not found.")
			return
		}
		if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{ActorID: &user.ID, Action: "user.reset", Target: targetID}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record password reset.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not reset credentials.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"temp_password":            temporaryPassword,
			"temp_password_expires_at": tempExpiresAt.UTC().Format(time.RFC3339),
		})
	}
}

func UserDisable(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		targetID := chi.URLParam(r, "id")
		if targetID == actor.ID {
			writeErr(w, http.StatusBadRequest, "You cannot disable your own account.")
			return
		}

		ctx := r.Context()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not disable account.")
			return
		}
		defer tx.Rollback()
		result, err := tx.ExecContext(ctx, `UPDATE users SET disabled_at = now(), token_version = token_version + 1 WHERE id = $1 AND disabled_at IS NULL`, targetID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not disable account.")
			return
		}
		affected, err := result.RowsAffected()
		if err != nil || affected == 0 {
			writeErr(w, http.StatusNotFound, "Account not found.")
			return
		}
		if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{ActorID: &actor.ID, Action: "user.disable", Target: targetID}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record account disable.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not disable account.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func nullString(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func nullTime(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time.UTC()
}
