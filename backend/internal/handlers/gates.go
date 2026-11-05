package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"smart-visitors/internal/auth"
	"smart-visitors/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func GatesList(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := database.QueryContext(r.Context(), `SELECT id::text, name, active FROM gates ORDER BY name`)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load gates.")
			return
		}
		defer rows.Close()
		gates := make([]map[string]any, 0)
		for rows.Next() {
			var id, name string
			var active bool
			if err := rows.Scan(&id, &name, &active); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not load gates.")
				return
			}
			gates = append(gates, map[string]any{"id": id, "name": name, "active": active})
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load gates.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": gates, "next_cursor": nil})
	}
}

func GatesCreate(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		if input.Name == "" || len(input.Name) > 100 {
			writeErr(w, http.StatusUnprocessableEntity, "name must be between 1 and 100 characters.")
			return
		}
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		id := uuid.NewString()
		tx, err := database.BeginTx(r.Context(), nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create gate.")
			return
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO gates (id, name) VALUES ($1, $2)`, id, input.Name); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
				strings.Contains(strings.ToLower(err.Error()), "unique") {
				writeErr(w, http.StatusConflict, "A gate with that name already exists.")
			} else {
				writeErr(w, http.StatusInternalServerError, "Could not create gate.")
			}
			return
		}
		if err := db.AppendAuditEvent(r.Context(), tx, db.AuditEvent{
			ActorID: &actor.ID, Action: "gate.create", Target: id,
			Meta: map[string]any{"name": input.Name},
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record gate creation.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create gate.")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": id, "name": input.Name, "active": true})
	}
}

func GatesUpdate(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Name   *string `json:"name"`
			Active *bool   `json:"active"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		if input.Name == nil && input.Active == nil {
			writeErr(w, http.StatusUnprocessableEntity, "Provide name or active.")
			return
		}
		if input.Name != nil {
			trimmed := strings.TrimSpace(*input.Name)
			if trimmed == "" || len(trimmed) > 100 {
				writeErr(w, http.StatusUnprocessableEntity, "name must be between 1 and 100 characters.")
				return
			}
			input.Name = &trimmed
		}
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		id := chi.URLParam(r, "id")

		tx, err := database.BeginTx(r.Context(), nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not update gate.")
			return
		}
		defer tx.Rollback()
		result, err := tx.ExecContext(r.Context(), `
			UPDATE gates SET name = COALESCE($1, name), active = COALESCE($2, active)
			WHERE id = $3`, input.Name, input.Active, id)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") ||
				strings.Contains(strings.ToLower(err.Error()), "unique") {
				writeErr(w, http.StatusConflict, "A gate with that name already exists.")
			} else {
				writeErr(w, http.StatusInternalServerError, "Could not update gate.")
			}
			return
		}
		affected, err := result.RowsAffected()
		if err != nil || affected == 0 {
			writeErr(w, http.StatusNotFound, "Gate not found.")
			return
		}
		if err := db.AppendAuditEvent(r.Context(), tx, db.AuditEvent{
			ActorID: &actor.ID, Action: "gate.update", Target: id,
			Meta: map[string]any{"name": input.Name, "active": input.Active},
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record gate update.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not update gate.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
