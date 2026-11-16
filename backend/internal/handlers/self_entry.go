package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"smart-visitors/internal/config"
	"smart-visitors/internal/cryptoutil"
	"smart-visitors/internal/db"

	"github.com/google/uuid"
)

func SelfEntryMeta(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !publicLimits.allow(clientIP(r), cfg.RateLimitPublic, time.Hour) {
			w.Header().Set("Retry-After", "3600")
			writeErr(w, http.StatusTooManyRequests, "Too many public requests. Try again later.")
			return
		}
		hosts, err := database.QueryContext(r.Context(), `
			SELECT id::text, username, name
			FROM users
			WHERE role = 'host' AND disabled_at IS NULL
			ORDER BY name`)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load hosts.")
			return
		}
		defer hosts.Close()
		hostItems := make([]map[string]string, 0)
		for hosts.Next() {
			var id, username, name string
			if err := hosts.Scan(&id, &username, &name); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not load hosts.")
				return
			}
			hostItems = append(hostItems, map[string]string{"id": id, "username": username, "name": name})
		}
		if err := hosts.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load hosts.")
			return
		}

		gates, err := database.QueryContext(r.Context(), `
			SELECT id::text, name FROM gates WHERE active = true ORDER BY name`)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load gates.")
			return
		}
		defer gates.Close()
		gateItems := make([]map[string]string, 0)
		for gates.Next() {
			var id, name string
			if err := gates.Scan(&id, &name); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not load gates.")
				return
			}
			gateItems = append(gateItems, map[string]string{"id": id, "name": name})
		}
		writeJSON(w, http.StatusOK, map[string]any{"hosts": hostItems, "gates": gateItems})
	}
}

func SelfEntryCreate(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !publicLimits.allow(clientIP(r), cfg.RateLimitPublic, time.Hour) {
			w.Header().Set("Retry-After", "3600")
			writeErr(w, http.StatusTooManyRequests, "Too many public requests. Try again later.")
			return
		}
		var input struct {
			Visitor    visitorInput `json:"visitor"`
			HostID     string       `json:"host_id"`
			Purpose    string       `json:"purpose"`
			GateID     *string      `json:"gate_id"`
			ValidFrom  time.Time    `json:"valid_from"`
			ValidUntil time.Time    `json:"valid_until"`
			Consent    bool         `json:"consent"`
		}
		if err := decodeJSON(w, r, &input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		input.HostID = strings.TrimSpace(input.HostID)
		input.Purpose = strings.TrimSpace(input.Purpose)
		input.Visitor.Name = strings.TrimSpace(input.Visitor.Name)
		if input.HostID == "" || input.Visitor.Name == "" || input.Purpose == "" || !input.Consent {
			writeErr(w, http.StatusUnprocessableEntity, "host, visitor name, purpose and consent are required.")
			return
		}
		now := time.Now().UTC()
		if input.ValidFrom.IsZero() {
			input.ValidFrom = now
		}
		if input.ValidUntil.IsZero() {
			input.ValidUntil = input.ValidFrom.Add(time.Duration(cfg.PassMaxHours) * time.Hour)
		}
		input.ValidFrom = input.ValidFrom.UTC()
		input.ValidUntil = input.ValidUntil.UTC()
		if !input.ValidUntil.After(input.ValidFrom) ||
			input.ValidUntil.Sub(input.ValidFrom) > time.Duration(cfg.PassMaxHours)*time.Hour {
			writeErr(w, http.StatusUnprocessableEntity, "validity window must be positive and within PASS_MAX_HOURS.")
			return
		}

		ctx := r.Context()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create visitor pass.")
			return
		}
		defer tx.Rollback()

		var hostName string
		if err := tx.QueryRowContext(ctx, `
			SELECT name FROM users
			WHERE id = $1 AND role = 'host' AND disabled_at IS NULL`, input.HostID).Scan(&hostName); errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusUnprocessableEntity, "Select an active host.")
			return
		} else if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not validate host.")
			return
		}
		if input.GateID != nil {
			var active bool
			if err := tx.QueryRowContext(ctx, `SELECT active FROM gates WHERE id = $1`, *input.GateID).Scan(&active); err != nil || !active {
				writeErr(w, http.StatusUnprocessableEntity, "Select an active gate.")
				return
			}
		}
		var createdBy string
		if err := tx.QueryRowContext(ctx, `
			SELECT id::text FROM users
			WHERE role = 'admin' AND disabled_at IS NULL
			ORDER BY created_at LIMIT 1`).Scan(&createdBy); err != nil {
			writeErr(w, http.StatusInternalServerError, "No administrator is available to issue passes.")
			return
		}

		visitorID, passID := uuid.NewString(), uuid.NewString()
		encryptedVisitor, err := encryptVisitorInput(cfg.MasterKey, visitorID, input.Visitor)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not encrypt visitor details.")
			return
		}
		token, err := randomToken()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not issue visitor pass.")
			return
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO visitors
				(id, name, phone, id_note, name_hash, phone_hash, consent_at, created_by, created_via)
			VALUES ($1, $2, $3, $4, $5, $6, now(), $7, 'regular')`,
			visitorID, encryptedVisitor.name, encryptedVisitor.phone, encryptedVisitor.idNote,
			encryptedVisitor.nameHash, encryptedVisitor.phoneHash, createdBy); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not save visitor details.")
			return
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO passes
				(id, token_hash, status, mode, visitor_id, host_id, gate_id, purpose,
				 valid_from, valid_until, created_via, created_by)
			VALUES ($1, $2, 'active', 'window', $3, $4, $5, $6, $7, $8, 'self_service', $9)`,
			passID, cryptoutil.TokenHash(cfg.MasterKey, token), visitorID, input.HostID,
			input.GateID, input.Purpose, input.ValidFrom, input.ValidUntil, createdBy); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create visitor pass.")
			return
		}
		payload := passPayload{
			Visitor: input.Visitor, Purpose: input.Purpose, HostID: input.HostID,
			GateID: input.GateID, CreatedVia: "self_service", IssuedAt: now,
		}
		if err := storePassSecrets(ctx, tx, cfg, passID, payload, token); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not secure visitor pass.")
			return
		}
		if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
			Action: "pass.self_service", Target: passID,
			Meta: map[string]any{"visitor_id": visitorID, "host_id": input.HostID},
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record visitor pass.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create visitor pass.")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"pass_id":     passID,
			"status":      "active",
			"visitor_url": strings.TrimRight(cfg.AppBaseURL, "/") + "/p/" + token,
			"token":       "sv1:" + token,
			"host_name":   hostName,
			"valid_from":  input.ValidFrom,
			"valid_until": input.ValidUntil,
		})
	}
}
