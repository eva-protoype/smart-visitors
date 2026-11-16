package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"smart-visitors/internal/auth"
	"smart-visitors/internal/config"
	"smart-visitors/internal/cryptoutil"
)

func HostPassScan(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		var input struct {
			Token string `json:"token"`
		}
		if err := decodeJSON(w, r, &input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		token := strings.TrimPrefix(strings.TrimSpace(input.Token), "sv1:")
		if !validOpaqueToken(token) {
			writeErr(w, http.StatusBadRequest, "Enter a valid visitor QR token.")
			return
		}

		var passID, visitorID, name, hostName, purpose, status, mode string
		var validFrom, validUntil time.Time
		var gateID sql.NullString
		err := database.QueryRowContext(r.Context(), `
			SELECT p.id::text, v.id::text, v.name, h.name, p.purpose, p.status, p.mode,
			       p.valid_from, p.valid_until, p.gate_id::text
			FROM passes p
			JOIN visitors v ON v.id = p.visitor_id
			JOIN users h ON h.id = p.host_id
			WHERE p.token_hash = $1 AND p.host_id = $2`,
			cryptoutil.TokenHash(cfg.MasterKey, token), actor.ID).Scan(
			&passID, &visitorID, &name, &hostName, &purpose, &status,
			&mode, &validFrom, &validUntil, &gateID)
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "This QR pass is not assigned to your host account.")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not scan visitor pass.")
			return
		}
		name, err = decryptVisitorName(cfg.MasterKey, visitorID, name)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not decrypt visitor pass.")
			return
		}
		now := time.Now().UTC()
		result := "active"
		if status == "revoked" || status == "shredded" {
			result = "closed"
		} else if status == "expired" || now.Before(validFrom) || now.After(validUntil) {
			result = "outside_window"
		} else if status == "used" {
			result = "used"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"pass_id":     passID,
			"status":      result,
			"visitor":     map[string]string{"name": name},
			"host_name":   hostName,
			"purpose":     purpose,
			"mode":        mode,
			"gate_id":     nullString(gateID),
			"valid_from":  validFrom.UTC(),
			"valid_until": validUntil.UTC(),
		})
	}
}
