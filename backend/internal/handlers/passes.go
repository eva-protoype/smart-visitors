package handlers

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"smart-visitors/internal/auth"
	"smart-visitors/internal/config"
	"smart-visitors/internal/cryptoutil"
	"smart-visitors/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
)

type visitInput struct {
	HostID     string       `json:"host_id"`
	Visitor    visitorInput `json:"visitor"`
	Purpose    string       `json:"purpose"`
	GateID     *string      `json:"gate_id"`
	ValidFrom  time.Time    `json:"valid_from"`
	ValidUntil time.Time    `json:"valid_until"`
	Mode       string       `json:"mode"`
}

func PassExtend(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		var input struct {
			ValidUntil time.Time `json:"valid_until"`
			Reason     string    `json:"reason"`
		}
		if err := decodeJSON(w, r, &input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		input.Reason = strings.TrimSpace(input.Reason)
		if input.ValidUntil.IsZero() || input.Reason == "" {
			writeErr(w, http.StatusUnprocessableEntity, "valid_until and reason are required.")
			return
		}
		passID := chi.URLParam(r, "id")
		ctx := r.Context()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not extend pass.")
			return
		}
		defer tx.Rollback()
		var status string
		var validFrom, currentUntil time.Time
		if err := tx.QueryRowContext(ctx, `
			SELECT status, valid_from, valid_until FROM passes WHERE id = $1 FOR UPDATE`,
			passID).Scan(&status, &validFrom, &currentUntil); errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "Pass not found.")
			return
		} else if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not extend pass.")
			return
		}
		newUntil := input.ValidUntil.UTC()
		if status != "active" || !newUntil.After(currentUntil) ||
			newUntil.Sub(validFrom) > time.Duration(cfg.PassMaxHours)*time.Hour {
			writeErr(w, http.StatusUnprocessableEntity, "Only active passes can be extended, within PASS_MAX_HOURS.")
			return
		}
		if _, err := tx.ExecContext(ctx, `UPDATE passes SET valid_until = $1 WHERE id = $2`, newUntil, passID); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not extend pass.")
			return
		}
		if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
			ActorID: &actor.ID, Action: "pass.extend", Target: passID, Reason: &input.Reason,
			Meta: map[string]any{"valid_until": newUntil.UTC().Format(time.RFC3339)},
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record pass extension.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not extend pass.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"pass_id": passID, "valid_until": newUntil})
	}
}

type visitorInput struct {
	Name   string  `json:"name"`
	Phone  *string `json:"phone"`
	IDNote *string `json:"id_note"`
}

type passPayload struct {
	Visitor    visitorInput `json:"visitor"`
	Purpose    string       `json:"purpose"`
	HostID     string       `json:"host_id"`
	GateID     *string      `json:"gate_id,omitempty"`
	CreatedVia string       `json:"created_via"`
	IssuedAt   time.Time    `json:"issued_at"`
}

func VisitsCreate(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input visitInput
		if err := decodeJSON(w, r, &input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		input.Visitor.Name = strings.TrimSpace(input.Visitor.Name)
		input.Purpose = strings.TrimSpace(input.Purpose)
		if input.Visitor.Name == "" || input.Purpose == "" || input.ValidFrom.IsZero() || input.ValidUntil.IsZero() {
			writeErr(w, http.StatusUnprocessableEntity, "visitor name, purpose, valid_from and valid_until are required.")
			return
		}
		if input.Mode == "" {
			input.Mode = "one_time"
		}
		if input.Mode != "one_time" && input.Mode != "window" {
			writeErr(w, http.StatusUnprocessableEntity, "mode must be one_time or window.")
			return
		}
		input.ValidFrom = input.ValidFrom.UTC()
		input.ValidUntil = input.ValidUntil.UTC()
		if !input.ValidUntil.After(input.ValidFrom) ||
			input.ValidUntil.Sub(input.ValidFrom) > time.Duration(cfg.PassMaxHours)*time.Hour {
			writeErr(w, http.StatusUnprocessableEntity, "validity window must be positive and within PASS_MAX_HOURS.")
			return
		}

		hostID := actor.ID
		if actor.Role == "admin" {
			hostID = strings.TrimSpace(input.HostID)
			if hostID == "" {
				writeErr(w, http.StatusUnprocessableEntity, "host_id is required for administrator-created visits.")
				return
			}
		} else if input.HostID != "" && input.HostID != actor.ID {
			writeErr(w, http.StatusForbidden, "Hosts can only create visits for themselves.")
			return
		}
		var hostRole string
		if err := database.QueryRowContext(r.Context(), `SELECT role FROM users WHERE id = $1 AND disabled_at IS NULL`, hostID).Scan(&hostRole); err != nil || hostRole != "host" {
			writeErr(w, http.StatusUnprocessableEntity, "host_id must identify an active host account.")
			return
		}
		if input.GateID != nil {
			var active bool
			if err := database.QueryRowContext(r.Context(), `SELECT active FROM gates WHERE id = $1`, *input.GateID).Scan(&active); err != nil || !active {
				writeErr(w, http.StatusUnprocessableEntity, "gate_id must identify an active gate.")
				return
			}
		}
		var overlap bool
		var phoneHash any
		if input.Visitor.Phone != nil && strings.TrimSpace(*input.Visitor.Phone) != "" {
			phoneHash = cryptoutil.TokenHash(cfg.MasterKey, strings.TrimSpace(*input.Visitor.Phone))
		}
		nameHash := cryptoutil.TokenHash(cfg.MasterKey, strings.ToLower(input.Visitor.Name))
		if err := database.QueryRowContext(r.Context(), `
			SELECT EXISTS (
				SELECT 1 FROM passes p JOIN visitors v ON v.id = p.visitor_id
				WHERE p.status = 'active'
				  AND p.valid_from < $1 AND p.valid_until > $2
				  AND (($3::text IS NOT NULL AND v.phone_hash = $3) OR
				       ($3::text IS NULL AND p.host_id = $4 AND v.name_hash = $5))
			)`, input.ValidUntil, input.ValidFrom, phoneHash, hostID, nameHash).Scan(&overlap); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not check for overlapping visits.")
			return
		}
		if overlap {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error": map[string]string{
					"code":    "OVERLAP_WARN",
					"message": "This visitor already has an active pass during the requested window.",
				},
			})
			return
		}

		passID, visitorID := uuid.NewString(), uuid.NewString()
		encryptedVisitor, err := encryptVisitorInput(cfg.MasterKey, visitorID, input.Visitor)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not encrypt visitor details.")
			return
		}
		inviteBytes := make([]byte, 16)
		if _, err := rand.Read(inviteBytes); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create an invitation.")
			return
		}
		inviteToken := base64.RawURLEncoding.EncodeToString(inviteBytes)
		placeholder := make([]byte, 16)
		if _, err := rand.Read(placeholder); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create an invitation.")
			return
		}
		placeholderToken := base64.RawURLEncoding.EncodeToString(placeholder)

		tx, err := database.BeginTx(r.Context(), nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create visit.")
			return
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO visitors (id, name, phone, id_note, name_hash, phone_hash, created_by, created_via)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'regular')`,
			visitorID, encryptedVisitor.name, encryptedVisitor.phone, encryptedVisitor.idNote,
			encryptedVisitor.nameHash, encryptedVisitor.phoneHash, actor.ID); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create visitor record.")
			return
		}
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO passes (id, token_hash, status, mode, visitor_id, host_id, gate_id, purpose,
			                    valid_from, valid_until, created_via, created_by, invite_token_hash)
			VALUES ($1, $2, 'draft', $3, $4, $5, $6, $7, $8, $9, 'regular', $10, $11)`,
			passID, cryptoutil.TokenHash(cfg.MasterKey, placeholderToken), input.Mode, visitorID,
			hostID, input.GateID, input.Purpose, input.ValidFrom, input.ValidUntil, actor.ID,
			cryptoutil.TokenHash(cfg.MasterKey, inviteToken)); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create visit.")
			return
		}
		if err := db.AppendAuditEvent(r.Context(), tx, db.AuditEvent{
			ActorID: &actor.ID, Action: "visit.create", Target: passID,
			Meta: map[string]any{"visitor_id": visitorID, "host_id": hostID, "mode": input.Mode},
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record visit creation.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create visit.")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"pass_id": passID, "invite_url": strings.TrimRight(cfg.AppBaseURL, "/") + "/fill/" + inviteToken,
			"status": "draft",
		})
	}
}

func InviteGet(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !publicLimits.allow(clientIP(r), cfg.RateLimitPublic, time.Hour) {
			w.Header().Set("Retry-After", "3600")
			writeErr(w, http.StatusTooManyRequests, "Too many public requests. Try again later.")
			return
		}
		token := chi.URLParam(r, "token")
		if !validOpaqueToken(token) {
			writeErr(w, http.StatusGone, "This invitation is invalid or has already been used.")
			return
		}
		var id, visitorID, name, purpose, hostName string
		var phone sql.NullString
		err := database.QueryRowContext(r.Context(), `
			SELECT p.id::text, v.id::text, v.name, v.phone, p.purpose, u.name
			FROM passes p JOIN visitors v ON v.id = p.visitor_id JOIN users u ON u.id = p.host_id
			WHERE p.invite_token_hash = $1 AND p.status = 'draft'`,
			cryptoutil.TokenHash(cfg.MasterKey, token)).Scan(&id, &visitorID, &name, &phone, &purpose, &hostName)
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusGone, "This invitation is invalid or has already been used.")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load invitation.")
			return
		}
		name, err = decryptVisitorName(cfg.MasterKey, visitorID, name)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not decrypt invitation details.")
			return
		}
		decryptedPhone, err := decryptOptionalVisitorValue(cfg.MasterKey, visitorID, "phone", phone)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not decrypt invitation details.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"pass_id": id, "visitor": map[string]any{"name": name, "phone": decryptedPhone},
			"purpose": purpose, "host_name": hostName,
		})
	}
}

func InviteFill(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !publicLimits.allow(clientIP(r), cfg.RateLimitPublic, time.Hour) {
			w.Header().Set("Retry-After", "3600")
			writeErr(w, http.StatusTooManyRequests, "Too many public requests. Try again later.")
			return
		}
		token := chi.URLParam(r, "token")
		if !validOpaqueToken(token) {
			writeErr(w, http.StatusGone, "This invitation is invalid or has already been used.")
			return
		}
		var input struct {
			Name    string  `json:"name"`
			Phone   *string `json:"phone"`
			IDNote  *string `json:"id_note"`
			Consent bool    `json:"consent"`
		}
		if err := decodeJSON(w, r, &input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		if input.Name == "" || len(input.Name) > 200 || !input.Consent {
			writeErr(w, http.StatusUnprocessableEntity, "A visitor name and consent are required.")
			return
		}

		ctx := r.Context()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not activate pass.")
			return
		}
		defer tx.Rollback()
		var passID, visitorID, hostID, mode, purpose string
		var gateID sql.NullString
		var createdVia string
		err = tx.QueryRowContext(ctx, `
			SELECT p.id::text, p.visitor_id::text, p.host_id::text, p.mode, p.purpose,
			       p.gate_id::text, p.created_via
			FROM passes p
			WHERE p.invite_token_hash = $1 AND p.status = 'draft'
			FOR UPDATE`, cryptoutil.TokenHash(cfg.MasterKey, token)).Scan(
			&passID, &visitorID, &hostID, &mode, &purpose, &gateID, &createdVia)
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusGone, "This invitation is invalid or has already been used.")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not activate pass.")
			return
		}
		var validFrom, validUntil time.Time
		if err := tx.QueryRowContext(ctx, `SELECT valid_from, valid_until FROM passes WHERE id = $1`, passID).Scan(&validFrom, &validUntil); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not activate pass.")
			return
		}
		if !validUntil.After(validFrom) || validUntil.Sub(validFrom) > time.Duration(cfg.PassMaxHours)*time.Hour {
			writeErr(w, http.StatusConflict, "Pass validity window is no longer valid.")
			return
		}

		payload := passPayload{
			Visitor: visitorInput{Name: input.Name, Phone: input.Phone, IDNote: input.IDNote},
			Purpose: purpose, HostID: hostID, CreatedVia: createdVia, IssuedAt: time.Now().UTC(),
		}
		if gateID.Valid {
			payload.GateID = &gateID.String
		}
		rawQRToken, err := randomToken()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not issue pass.")
			return
		}
		if err := storePassSecrets(ctx, tx, cfg, passID, payload, rawQRToken); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not store sealed visitor details.")
			return
		}
		encryptedVisitor, err := encryptVisitorInput(cfg.MasterKey, visitorID,
			visitorInput{Name: input.Name, Phone: input.Phone, IDNote: input.IDNote})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not encrypt visitor details.")
			return
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE visitors SET name = $1, phone = $2, id_note = $3, name_hash = $4,
				phone_hash = $5, consent_at = now()
			WHERE id = $6`, encryptedVisitor.name, encryptedVisitor.phone, encryptedVisitor.idNote,
			encryptedVisitor.nameHash, encryptedVisitor.phoneHash, visitorID); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not save visitor consent.")
			return
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE passes SET status = 'active', token_hash = $1, invite_token_hash = NULL
			WHERE id = $2 AND status = 'draft'`,
			cryptoutil.TokenHash(cfg.MasterKey, rawQRToken), passID); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not activate pass.")
			return
		}
		if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
			Action: "pass.activate", Target: passID,
			Meta: map[string]any{"visitor_id": visitorID, "created_via": createdVia},
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record pass activation.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not activate pass.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"pass_id": passID, "status": "active", "token": "sv1:" + rawQRToken,
			"visitor_url": strings.TrimRight(cfg.AppBaseURL, "/") + "/p/" + rawQRToken,
			"valid_from":  validFrom.UTC().Format(time.RFC3339),
			"valid_until": validUntil.UTC().Format(time.RFC3339),
		})
	}
}

func VisitsList(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		limit, err := queryLimit(r, 50)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "limit must be between 1 and 100.")
			return
		}
		scope := r.URL.Query().Get("scope")
		if scope != "" && scope != "upcoming" && scope != "active" && scope != "ended" {
			writeErr(w, http.StatusBadRequest, "scope must be upcoming, active or ended.")
			return
		}
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		var nameHash any
		if query != "" {
			nameHash = cryptoutil.TokenHash(cfg.MasterKey, strings.ToLower(query))
		}
		rows, err := database.QueryContext(r.Context(), `
			SELECT p.id::text, p.status, p.mode, p.visitor_id::text, p.host_id::text,
			       p.gate_id::text, p.purpose, p.valid_from, p.valid_until,
			       v.name, h.name
			FROM passes p JOIN visitors v ON v.id = p.visitor_id JOIN users h ON h.id = p.host_id
			WHERE ($1 = 'admin' OR p.host_id = $2)
			  AND ($3 = '' OR
			       ($3 = 'upcoming' AND (p.status = 'draft' OR (p.status = 'active' AND p.valid_from > now()))) OR
			       ($3 = 'active' AND p.status = 'active' AND p.valid_from <= now() AND p.valid_until >= now()) OR
			       ($3 = 'ended' AND p.status IN ('used','expired','revoked','shredded')))
			  AND ($4 = '' OR v.name_hash = $5 OR p.purpose ILIKE '%' || $4 || '%')
			ORDER BY p.valid_from DESC LIMIT $6`, actor.Role, actor.ID, scope, query, nameHash, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load visits.")
			return
		}
		defer rows.Close()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var id, status, mode, visitorID, hostID, purpose, visitorName, hostName string
			var gateID sql.NullString
			var validFrom, validUntil time.Time
			if err := rows.Scan(&id, &status, &mode, &visitorID, &hostID, &gateID, &purpose,
				&validFrom, &validUntil, &visitorName, &hostName); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not load visits.")
				return
			}
			visitorName, err = decryptVisitorName(cfg.MasterKey, visitorID, visitorName)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not decrypt visitor details.")
				return
			}
			items = append(items, map[string]any{
				"id": id, "status": status, "mode": mode, "visitor_id": visitorID,
				"host_id": hostID, "gate_id": nullString(gateID), "purpose": purpose,
				"valid_from": validFrom.UTC(), "valid_until": validUntil.UTC(),
				"visitor_name": visitorName, "host_name": hostName,
			})
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load visits.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nil})
	}
}

func PassDetails(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		passID := chi.URLParam(r, "id")
		var hostID string
		if err := database.QueryRowContext(r.Context(), `SELECT host_id::text FROM passes WHERE id = $1`, passID).Scan(&hostID); errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "Pass not found.")
			return
		} else if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load pass.")
			return
		}
		if actor.Role != "admin" && actor.ID != hostID {
			if err := appendStandaloneAudit(r.Context(), database, db.AuditEvent{
				ActorID: &actor.ID, Action: "forbidden_attempt", Target: passID,
				Meta: map[string]any{"action": "pass.decrypt"},
			}); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not record denied decryption attempt.")
				return
			}
			writeErr(w, http.StatusForbidden, "You cannot decrypt another host's pass.")
			return
		}
		ctx := r.Context()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not decrypt pass.")
			return
		}
		defer tx.Rollback()
		var blob, wrappedDEK, signature []byte
		if err := tx.QueryRowContext(ctx, `SELECT blob, wrap, payload_sig FROM pass_secrets WHERE pass_id = $1`, passID).Scan(&blob, &wrappedDEK, &signature); err != nil {
			writeErr(w, http.StatusGone, "Pass details are no longer available.")
			return
		}
		dek, err := cryptoutil.Open(cfg.MasterKey, wrappedDEK, []byte("dek:"+passID))
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not decrypt pass details.")
			return
		}
		plaintext, err := cryptoutil.Open(dek, blob, []byte("pass:"+passID))
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not decrypt pass details.")
			return
		}
		signingKey, err := issuerPrivateKey(ctx, tx, cfg.MasterKey)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not verify pass details.")
			return
		}
		payloadHash := sha256.Sum256(plaintext)
		if !ed25519.Verify(signingKey.Public().(ed25519.PublicKey), payloadHash[:], signature) {
			writeErr(w, http.StatusInternalServerError, "Pass details failed integrity verification.")
			return
		}
		if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
			ActorID: &actor.ID, Action: "pass.decrypt", Target: passID,
			Meta: map[string]any{"role": actor.Role},
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record pass decryption.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not decrypt pass.")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(plaintext)
	}
}

func PassQR(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		passID := chi.URLParam(r, "id")
		var hostID, storedHash string
		var wrappedToken []byte
		err := database.QueryRowContext(r.Context(), `
			SELECT p.host_id::text, p.token_hash, s.qr_wrap
			FROM passes p JOIN pass_secrets s ON s.pass_id = p.id WHERE p.id = $1`, passID).Scan(&hostID, &storedHash, &wrappedToken)
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "Pass QR is not available.")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load pass QR.")
			return
		}
		if actor.Role != "admin" && actor.ID != hostID {
			writeErr(w, http.StatusForbidden, "You cannot view another host's pass QR.")
			return
		}
		token, err := cryptoutil.Open(cfg.MasterKey, wrappedToken, []byte("qr:"+passID))
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load pass QR.")
			return
		}
		tokenString := string(token)
		if cryptoutil.TokenHash(cfg.MasterKey, tokenString) != storedHash {
			writeErr(w, http.StatusInternalServerError, "Pass QR integrity check failed.")
			return
		}
		image, err := qrcode.Encode("sv1:"+tokenString, qrcode.Medium, 256)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not generate pass QR.")
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "private, no-store")
		_, _ = w.Write(image)
	}
}

func PassRevoke(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		var input struct {
			Reason string `json:"reason"`
		}
		if err := decodeJSON(w, r, &input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		input.Reason = strings.TrimSpace(input.Reason)
		if input.Reason == "" {
			writeErr(w, http.StatusUnprocessableEntity, "A reason is required.")
			return
		}
		passID := chi.URLParam(r, "id")
		tx, err := database.BeginTx(r.Context(), nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not revoke pass.")
			return
		}
		defer tx.Rollback()
		var hostID, status string
		if err := tx.QueryRowContext(r.Context(), `SELECT host_id::text, status FROM passes WHERE id = $1 FOR UPDATE`, passID).Scan(&hostID, &status); errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "Pass not found.")
			return
		} else if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not revoke pass.")
			return
		}
		if actor.Role != "admin" && actor.ID != hostID {
			writeErr(w, http.StatusForbidden, "You cannot revoke another host's pass.")
			return
		}
		if status == "shredded" || status == "revoked" {
			writeErr(w, http.StatusConflict, "Pass is already closed.")
			return
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE passes SET status = 'revoked' WHERE id = $1`, passID); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not revoke pass.")
			return
		}
		if err := db.AppendAuditEvent(r.Context(), tx, db.AuditEvent{
			ActorID: &actor.ID, Action: "pass.revoke", Target: passID, Reason: &input.Reason,
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record pass revocation.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not revoke pass.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func PublicPass(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !publicLimits.allow(clientIP(r), cfg.RateLimitPublic, time.Hour) {
			w.Header().Set("Retry-After", "3600")
			writeErr(w, http.StatusTooManyRequests, "Too many public requests. Try again later.")
			return
		}
		token := chi.URLParam(r, "token")
		if !validOpaqueToken(token) {
			writeErr(w, http.StatusNotFound, "Pass not found.")
			return
		}
		var visitorID, name, hostName, purpose, status, mode string
		var validFrom, validUntil time.Time
		var gateID sql.NullString
		err := database.QueryRowContext(r.Context(), `
			SELECT v.id::text, v.name, h.name, p.purpose, p.status, p.mode, p.valid_from, p.valid_until, p.gate_id::text
			FROM passes p JOIN visitors v ON v.id = p.visitor_id JOIN users h ON h.id = p.host_id
			WHERE p.token_hash = $1`,
			cryptoutil.TokenHash(cfg.MasterKey, token)).Scan(
			&visitorID, &name, &hostName, &purpose, &status, &mode, &validFrom, &validUntil, &gateID)
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "Pass not found.")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load pass.")
			return
		}
		name, err = decryptVisitorName(cfg.MasterKey, visitorID, name)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not decrypt pass details.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"name": name, "host_name": hostName, "purpose": purpose, "status": status,
			"mode": mode, "gate_id": nullString(gateID),
			"valid_from": validFrom.UTC(), "valid_until": validUntil.UTC(),
		})
	}
}

type scanInput struct {
	Token     string `json:"token"`
	GateID    string `json:"gate_id"`
	Direction string `json:"direction"`
}

func PassesScan(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		if !scanLimits.allow(clientIP(r), cfg.RateLimitScan, time.Minute) {
			w.Header().Set("Retry-After", "60")
			writeErr(w, http.StatusTooManyRequests, "Too many scans. Try again shortly.")
			return
		}
		var input scanInput
		if err := decodeJSON(w, r, &input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		input.Token = strings.TrimSpace(input.Token)
		input.GateID = strings.TrimSpace(input.GateID)
		if input.GateID == "" || (input.Direction != "entry" && input.Direction != "exit") {
			writeErr(w, http.StatusUnprocessableEntity, "gate_id and a valid direction are required.")
			return
		}
		token := strings.TrimPrefix(input.Token, "sv1:")
		if !validOpaqueToken(token) {
			respondWithScanLog(w, r, database, cfg, db.AccessLog{
				GateID: optionalUUID(input.GateID), GuardID: &actor.ID, Direction: input.Direction,
				Outcome: "denied", Reason: "BAD_TOKEN",
			}, scanResult{Reason: "BAD_TOKEN"})
			return
		}

		ctx := r.Context()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not process scan.")
			return
		}
		defer tx.Rollback()
		var gateName string
		var gateActive bool
		gateErr := tx.QueryRowContext(ctx, `SELECT name, active FROM gates WHERE id = $1`, input.GateID).Scan(&gateName, &gateActive)
		if gateErr != nil || !gateActive {
			result := scanResult{Reason: "BAD_GATE"}
			logID, err := appendScanLog(ctx, tx, db.AccessLog{
				GateID: optionalUUID(input.GateID), GuardID: &actor.ID, Direction: input.Direction,
				Outcome: "denied", Reason: result.Reason,
			})
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not record scan.")
				return
			}
			if err := tx.Commit(); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not process scan.")
				return
			}
			go ScoreScanAsync(cfg, logID, "", input.GateID, time.Now().UTC(), input.Direction, "denied")
			writeScanResult(w, result)
			return
		}

		var passID, status, mode, visitorID, hostID, purpose, createdVia string
		var passGate sql.NullString
		var validFrom, validUntil time.Time
		tokenHash := cryptoutil.TokenHash(cfg.MasterKey, token)
		err = tx.QueryRowContext(ctx, `
			SELECT p.id::text, p.status, p.mode, p.visitor_id::text, p.host_id::text, p.gate_id::text,
			       p.purpose, p.valid_from, p.valid_until, p.created_via
			FROM passes p
			WHERE p.token_hash = $1 OR p.id = (
				SELECT st.pass_id FROM stickers st WHERE st.token_hash = $1 AND st.status = 'assigned'
			)
			FOR UPDATE OF p`, tokenHash).Scan(
			&passID, &status, &mode, &visitorID, &hostID, &passGate, &purpose, &validFrom, &validUntil, &createdVia)
		if errors.Is(err, sql.ErrNoRows) {
			result := scanResult{Reason: "UNKNOWN_TOKEN"}
			var stickerStatus string
			if stickerErr := tx.QueryRowContext(ctx, `
				SELECT status FROM stickers WHERE token_hash = $1`, tokenHash).Scan(&stickerStatus); stickerErr == nil {
				if stickerStatus == "free" {
					result.Reason = "STICKER_UNBOUND"
				} else if stickerStatus == "retired" {
					result.Reason = "PASS_REVOKED"
				} else {
					result.Reason = "INTERNAL"
				}
			} else if stickerErr != sql.ErrNoRows {
				writeErr(w, http.StatusInternalServerError, "Could not process scan.")
				return
			}
			logID, err := appendScanLog(ctx, tx, db.AccessLog{
				GateID: optionalUUID(input.GateID), GuardID: &actor.ID, Direction: input.Direction,
				Outcome: "denied", Reason: result.Reason,
			})
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not record scan.")
				return
			}
			if err := tx.Commit(); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not process scan.")
				return
			}
			go ScoreScanAsync(cfg, logID, "", input.GateID, time.Now().UTC(), input.Direction, "denied")
			writeScanResult(w, result)
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not process scan.")
			return
		}

		result := scanResult{
			Reason: "OK", PassID: passID, GateName: gateName, Purpose: purpose, Mode: mode,
			ValidFrom: validFrom.UTC(), ValidUntil: validUntil.UTC(), Direction: input.Direction,
		}
		reason := "OK"
		switch {
		case status == "revoked" || status == "shredded":
			reason = "PASS_REVOKED"
		case status == "expired" || time.Now().Before(validFrom) || time.Now().After(validUntil):
			reason = "NOT_IN_WINDOW"
		case passGate.Valid && passGate.String != input.GateID:
			reason = "WRONG_GATE"
		case input.Direction == "entry" && status == "used":
			reason = "ALREADY_USED"
		case input.Direction == "entry" && status == "active":
			result.Approved = true
			if mode == "one_time" {
				if _, err := tx.ExecContext(ctx, `UPDATE passes SET status = 'used' WHERE id = $1`, passID); err != nil {
					writeErr(w, http.StatusInternalServerError, "Could not process scan.")
					return
				}
			}
		case input.Direction == "exit" && (status == "active" || status == "used"):
			result.Approved = true
		default:
			reason = "NOT_IN_WINDOW"
		}
		result.Reason = reason
		var visitorName, hostName string
		var photoRef sql.NullString
		if err := tx.QueryRowContext(ctx, `
			SELECT v.name, u.name, v.photo_ref
			FROM visitors v JOIN users u ON u.id = $2 WHERE v.id = $1`,
			visitorID, hostID).Scan(&visitorName, &hostName, &photoRef); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load scan card.")
			return
		}
		visitorName, err = decryptVisitorName(cfg.MasterKey, visitorID, visitorName)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not decrypt scan card.")
			return
		}
		result.Name, result.HostName = visitorName, hostName
		if result.Approved && photoRef.Valid {
			photo, err := readEncryptedVisitorPhoto(cfg.PhotosDir, cfg.MasterKey, visitorID, photoRef.String)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not load scan photo.")
				return
			}
			result.Photo = base64.StdEncoding.EncodeToString(photo)
		}
		outcome := "denied"
		if result.Approved {
			outcome = "approved"
		}
		result.Outcome = outcome
		logEntry := db.AccessLog{
			PassID: &passID, GateID: &input.GateID, GuardID: &actor.ID,
			ScanTime: time.Now().UTC(), Direction: input.Direction, Outcome: outcome, Reason: reason,
		}
		logID, err := appendScanLog(ctx, tx, logEntry)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record scan.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not process scan.")
			return
		}
		go ScoreScanAsync(cfg, logID, passID, input.GateID, logEntry.ScanTime, input.Direction, outcome)
		writeScanResult(w, result)
	}
}

type scanResult struct {
	Approved   bool      `json:"approved"`
	Outcome    string    `json:"outcome"`
	Reason     string    `json:"reason"`
	Message    string    `json:"message"`
	PassID     string    `json:"pass_id,omitempty"`
	Name       string    `json:"name,omitempty"`
	HostName   string    `json:"host_name,omitempty"`
	Purpose    string    `json:"purpose,omitempty"`
	GateName   string    `json:"gate_name,omitempty"`
	Mode       string    `json:"mode,omitempty"`
	ValidFrom  time.Time `json:"valid_from,omitempty"`
	ValidUntil time.Time `json:"valid_until,omitempty"`
	Direction  string    `json:"direction,omitempty"`
	Photo      string    `json:"photo,omitempty"`
}

func writeScanResult(w http.ResponseWriter, result scanResult) {
	if result.Approved {
		result.Outcome = "approved"
	} else {
		result.Outcome = "denied"
	}
	switch result.Reason {
	case "BAD_TOKEN":
		result.Message = "this code isn't recognised"
	case "UNKNOWN_TOKEN":
		result.Message = "unknown pass — check with admin"
	case "NOT_IN_WINDOW":
		result.Message = "pass not active yet/expired"
	case "WRONG_GATE":
		result.Message = "this pass is for another gate"
	case "ALREADY_USED":
		result.Message = "already entered — badge closed"
	case "PASS_REVOKED":
		result.Message = "cancelled by host/admin"
	case "STICKER_UNBOUND":
		result.Message = "sticker not yet issued"
	case "BAD_GATE":
		result.Message = "unknown or inactive gate"
	case "OK":
		result.Message = "Access granted"
	case "INTERNAL":
		result.Message = "system error — try again / call admin"
	}
	writeJSON(w, http.StatusOK, result)
}

func respondWithScanLog(w http.ResponseWriter, r *http.Request, database *sql.DB, cfg config.Config, entry db.AccessLog, result scanResult) {
	tx, err := database.BeginTx(r.Context(), nil)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Could not process scan.")
		return
	}
	defer tx.Rollback()
	logID, err := appendScanLog(r.Context(), tx, entry)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "Could not record scan.")
		return
	}
	if err := tx.Commit(); err != nil {
		writeErr(w, http.StatusInternalServerError, "Could not process scan.")
		return
	}
	gateID := ""
	if entry.GateID != nil {
		gateID = *entry.GateID
	}
	go ScoreScanAsync(cfg, logID, "", gateID, time.Now().UTC(), entry.Direction, entry.Outcome)
	writeScanResult(w, result)
}

func appendScanLog(ctx context.Context, tx *sql.Tx, entry db.AccessLog) (string, error) {
	if entry.GateID != nil && !validUUID(*entry.GateID) {
		entry.GateID = nil
	}
	return db.AppendAccessLog(ctx, tx, entry)
}

func issuerPrivateKey(ctx context.Context, tx *sql.Tx, masterKey []byte) (ed25519.PrivateKey, error) {
	var wrapped []byte
	if err := tx.QueryRowContext(ctx, `SELECT wrapped FROM keys WHERE name = 'issuer-ed25519'`).Scan(&wrapped); err != nil {
		return nil, fmt.Errorf("load issuer signing key: %w", err)
	}
	plaintext, err := cryptoutil.Open(masterKey, wrapped, []byte("issuer-ed25519"))
	if err != nil {
		return nil, fmt.Errorf("unwrap issuer signing key: %w", err)
	}
	if len(plaintext) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid issuer signing key")
	}
	return ed25519.PrivateKey(plaintext), nil
}

func storePassSecrets(ctx context.Context, tx *sql.Tx, cfg config.Config, passID string, payload passPayload, qrToken string) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode pass payload: %w", err)
	}
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return fmt.Errorf("generate pass data key: %w", err)
	}
	blob, err := cryptoutil.Seal(dek, payloadJSON, []byte("pass:"+passID))
	if err != nil {
		return fmt.Errorf("encrypt pass payload: %w", err)
	}
	wrappedDEK, err := cryptoutil.Seal(cfg.MasterKey, dek, []byte("dek:"+passID))
	if err != nil {
		return fmt.Errorf("wrap pass data key: %w", err)
	}
	signingKey, err := issuerPrivateKey(ctx, tx, cfg.MasterKey)
	if err != nil {
		return fmt.Errorf("load issuer signing key: %w", err)
	}
	payloadHash := sha256.Sum256(payloadJSON)
	signature := ed25519.Sign(signingKey, payloadHash[:])
	qrWrap, err := cryptoutil.Seal(cfg.MasterKey, []byte(qrToken), []byte("qr:"+passID))
	if err != nil {
		return fmt.Errorf("wrap QR token: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pass_secrets (pass_id, blob, wrap, payload_sig, qr_wrap)
		VALUES ($1, $2, $3, $4, $5)`, passID, blob, wrappedDEK, signature, qrWrap); err != nil {
		return fmt.Errorf("store encrypted pass payload: %w", err)
	}
	return nil
}

func randomToken() (string, error) {
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(token), nil
}

func validOpaqueToken(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(raw) == 16 && base64.RawURLEncoding.EncodeToString(raw) == token
}

func validUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func optionalUUID(value string) *string {
	if !validUUID(value) {
		return nil
	}
	return &value
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("request must contain one JSON value")
	}
	return nil
}

type requestWindow struct {
	start time.Time
	count int
}

type requestLimiter struct {
	mu          sync.Mutex
	windows     map[string]requestWindow
	lastCleanup time.Time
}

var scanLimits = newRequestLimiter()
var publicLimits = newRequestLimiter()

func newRequestLimiter() *requestLimiter {
	return &requestLimiter{windows: make(map[string]requestWindow)}
}

func (l *requestLimiter) allow(key string, limit int, period time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.Sub(l.lastCleanup) >= 5*time.Minute {
		for candidate, window := range l.windows {
			if now.Sub(window.start) >= period {
				delete(l.windows, candidate)
			}
		}
		l.lastCleanup = now
	}
	window := l.windows[key]
	if window.start.IsZero() || now.Sub(window.start) >= period {
		window = requestWindow{start: now}
	}
	if window.count >= limit {
		l.windows[key] = window
		return false
	}
	window.count++
	l.windows[key] = window
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
