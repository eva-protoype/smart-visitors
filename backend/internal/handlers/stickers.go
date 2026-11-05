package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"smart-visitors/internal/auth"
	"smart-visitors/internal/config"
	"smart-visitors/internal/cryptoutil"
	"smart-visitors/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func StickerBatchCreate(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Count int `json:"count"`
		}
		if err := decodeJSON(w, r, &input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		if input.Count < 1 || input.Count > 200 {
			writeErr(w, http.StatusUnprocessableEntity, "count must be between 1 and 200.")
			return
		}
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		batchID := uuid.NewString()
		ctx := r.Context()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create sticker batch.")
			return
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO sticker_batches (id, count, created_by) VALUES ($1, $2, $3)`,
			batchID, input.Count, actor.ID); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create sticker batch.")
			return
		}
		items := make([]map[string]any, 0, input.Count)
		for sequence := 1; sequence <= input.Count; sequence++ {
			token, err := randomToken()
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not generate sticker tokens.")
				return
			}
			id := uuid.NewString()
			humanCode := fmt.Sprintf("B-%s-%03d-%s", strings.ToUpper(batchID[:4]), sequence, strings.ToUpper(id[:3]))
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO stickers (id, batch_id, token_hash) VALUES ($1, $2, $3)`,
				id, batchID, cryptoutil.TokenHash(cfg.MasterKey, token)); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not store sticker batch.")
				return
			}
			items = append(items, map[string]any{
				"id": id, "human_code": humanCode, "token": "sv1:" + token,
				"status": "free", "printed": false,
			})
		}
		if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
			ActorID: &actor.ID, Action: "sticker.batch.generate", Target: batchID,
			Meta: map[string]any{"count": input.Count},
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record sticker batch.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create sticker batch.")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"batch_id": batchID, "items": items})
	}
}

func StickersList(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		if status != "" && status != "free" && status != "assigned" && status != "retired" {
			writeErr(w, http.StatusBadRequest, "status must be free, assigned or retired.")
			return
		}
		limit, err := queryLimit(r, 50)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "limit must be between 1 and 100.")
			return
		}
		rows, err := database.QueryContext(r.Context(), `
			SELECT id::text, batch_id::text, status, pass_id::text, claimed_by::text,
			       claimed_at, printed, retired_at
			FROM stickers WHERE ($1 = '' OR status = $1)
			ORDER BY id LIMIT $2`, status, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load stickers.")
			return
		}
		defer rows.Close()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var id, state string
			var batchID, passID, claimedBy sql.NullString
			var claimedAt, retiredAt sql.NullTime
			var printed bool
			if err := rows.Scan(&id, &batchID, &state, &passID, &claimedBy, &claimedAt, &printed, &retiredAt); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not load stickers.")
				return
			}
			items = append(items, map[string]any{
				"id": id, "batch_id": nullString(batchID), "status": state,
				"pass_id": nullString(passID), "claimed_by": nullString(claimedBy),
				"claimed_at": nullTime(claimedAt), "printed": printed, "retired_at": nullTime(retiredAt),
			})
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load stickers.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nil})
	}
}

func StickerRetire(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		id := chi.URLParam(r, "id")
		ctx := r.Context()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not retire sticker.")
			return
		}
		defer tx.Rollback()
		var status string
		var passID sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT status, pass_id::text FROM stickers WHERE id = $1 FOR UPDATE`, id).Scan(&status, &passID); errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "Sticker not found.")
			return
		} else if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not retire sticker.")
			return
		}
		if status == "retired" {
			writeErr(w, http.StatusConflict, "Sticker is already retired.")
			return
		}
		if _, err := tx.ExecContext(ctx, `UPDATE stickers SET status = 'retired', retired_at = now() WHERE id = $1`, id); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not retire sticker.")
			return
		}
		if passID.Valid {
			if _, err := tx.ExecContext(ctx, `UPDATE passes SET status = 'revoked' WHERE id = $1 AND status IN ('draft','active','used')`, passID.String); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not close the sticker pass.")
				return
			}
			if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
				ActorID: &actor.ID, Action: "pass.revoke", Target: passID.String, Reason: &input.Reason,
			}); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not record pass revocation.")
				return
			}
		}
		if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
			ActorID: &actor.ID, Action: "sticker.retire", Target: id, Reason: &input.Reason,
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record sticker retirement.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not retire sticker.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type walkInInput struct {
	HostUsername string       `json:"host_username"`
	Visitor      visitorInput `json:"visitor"`
	StickerToken string       `json:"sticker_token,omitempty"`
	Purpose      string       `json:"purpose"`
	GateID       *string      `json:"gate_id"`
	ValidFrom    time.Time    `json:"valid_from"`
	ValidUntil   time.Time    `json:"valid_until"`
	Mode         string       `json:"mode"`
	ConsentNote  string       `json:"consent_note"`
}

func WalkinCreate(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input walkInInput
		if err := decodeJSON(w, r, &input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		token, err := randomToken()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not issue walk-in pass.")
			return
		}
		tx, err := database.BeginTx(r.Context(), nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create walk-in.")
			return
		}
		defer tx.Rollback()
		passID, err := createWalkIn(r.Context(), tx, cfg, actor.ID, input, token, nil)
		if err != nil {
			writeWalkInError(w, err)
			return
		}
		if err := db.AppendAuditEvent(r.Context(), tx, db.AuditEvent{
			ActorID: &actor.ID, Action: "walkin.create", Target: passID,
			Meta: map[string]any{"host_username": input.HostUsername},
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record walk-in.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not create walk-in.")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"pass_id": passID, "status": "active", "token": "sv1:" + token,
		})
	}
}

func StickerClaim(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input walkInInput
		if err := decodeJSON(w, r, &input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		token := strings.TrimPrefix(strings.TrimSpace(input.StickerToken), "sv1:")
		if !validOpaqueToken(token) {
			writeErr(w, http.StatusUnprocessableEntity, "A valid sticker token is required.")
			return
		}
		actor, ok := auth.UserFromContext(r.Context())
		if !ok {
			writeErr(w, http.StatusUnauthorized, "Authentication is required.")
			return
		}
		tx, err := database.BeginTx(r.Context(), nil)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not claim sticker.")
			return
		}
		defer tx.Rollback()
		tokenHash := cryptoutil.TokenHash(cfg.MasterKey, token)
		var stickerID, status string
		if err := tx.QueryRowContext(r.Context(), `
			SELECT id::text, status FROM stickers WHERE token_hash = $1 FOR UPDATE`, tokenHash).Scan(&stickerID, &status); errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "Sticker not found.")
			return
		} else if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not claim sticker.")
			return
		}
		if status != "free" {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error": map[string]string{"code": "STICKER_ASSIGNED", "message": "Sticker is not available."},
			})
			return
		}
		passID, err := createWalkIn(r.Context(), tx, cfg, actor.ID, input, token, &stickerID)
		if err != nil {
			writeWalkInError(w, err)
			return
		}
		if _, err := tx.ExecContext(r.Context(), `
			UPDATE stickers SET status = 'assigned', pass_id = $1, claimed_by = $2, claimed_at = now()
			WHERE id = $3 AND status = 'free'`, passID, actor.ID, stickerID); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not bind sticker to walk-in.")
			return
		}
		reason := strings.TrimSpace(input.ConsentNote)
		if err := db.AppendAuditEvent(r.Context(), tx, db.AuditEvent{
			ActorID: &actor.ID, Action: "walkin.create", Target: passID,
			Meta: map[string]any{"host_username": input.HostUsername, "sticker_id": stickerID},
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record walk-in.")
			return
		}
		if err := db.AppendAuditEvent(r.Context(), tx, db.AuditEvent{
			ActorID: &actor.ID, Action: "sticker.claim", Target: stickerID,
			Reason: &reason, Meta: map[string]any{"pass_id": passID},
		}); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not record sticker claim.")
			return
		}
		if err := tx.Commit(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not claim sticker.")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"pass_id": passID, "status": "active", "token": "sv1:" + token,
			"sticker_id": stickerID, "confirmation_card": map[string]any{
				"name": input.Visitor.Name, "host": input.HostUsername, "purpose": input.Purpose,
			},
		})
	}
}

type walkInError struct {
	status  int
	message string
}

func (e walkInError) Error() string { return e.message }

func createWalkIn(ctx context.Context, tx *sql.Tx, cfg config.Config, guardID string, input walkInInput, token string, stickerID *string) (string, error) {
	input.Visitor.Name = strings.TrimSpace(input.Visitor.Name)
	input.Purpose = strings.TrimSpace(input.Purpose)
	input.HostUsername = strings.TrimSpace(input.HostUsername)
	if input.Visitor.Name == "" || input.Purpose == "" || input.HostUsername == "" || strings.TrimSpace(input.ConsentNote) == "" {
		return "", walkInError{http.StatusUnprocessableEntity, "name, host, purpose and consent_note are required."}
	}
	mode := input.Mode
	if mode == "" {
		mode = "one_time"
	}
	if mode != "one_time" && mode != "window" {
		return "", walkInError{http.StatusUnprocessableEntity, "mode must be one_time or window."}
	}
	var hostID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text FROM users WHERE username = $1 AND role = 'host' AND disabled_at IS NULL`,
		input.HostUsername).Scan(&hostID); errors.Is(err, sql.ErrNoRows) {
		return "", walkInError{http.StatusUnprocessableEntity, "host_username must identify an active host account."}
	} else if err != nil {
		return "", fmt.Errorf("lookup host: %w", err)
	}
	now := time.Now().UTC()
	validFrom := input.ValidFrom
	if validFrom.IsZero() {
		validFrom = now
	}
	validFrom = validFrom.UTC()
	validUntil := input.ValidUntil
	if validUntil.IsZero() {
		validUntil = validFrom.Add(time.Duration(cfg.PassMaxHours) * time.Hour)
	}
	validUntil = validUntil.UTC()
	if !validUntil.After(validFrom) || validUntil.Sub(validFrom) > time.Duration(cfg.PassMaxHours)*time.Hour {
		return "", walkInError{http.StatusUnprocessableEntity, "validity window must be positive and within PASS_MAX_HOURS."}
	}
	if input.GateID != nil {
		var active bool
		if err := tx.QueryRowContext(ctx, `SELECT active FROM gates WHERE id = $1`, *input.GateID).Scan(&active); err != nil || !active {
			return "", walkInError{http.StatusUnprocessableEntity, "gate_id must identify an active gate."}
		}
	}
	visitorID, passID := uuid.NewString(), uuid.NewString()
	encryptedVisitor, err := encryptVisitorInput(cfg.MasterKey, visitorID, input.Visitor)
	if err != nil {
		return "", fmt.Errorf("encrypt walk-in visitor: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO visitors (id, name, phone, id_note, name_hash, phone_hash, consent_at, created_by, created_via)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'walk_in')`,
		visitorID, encryptedVisitor.name, encryptedVisitor.phone, encryptedVisitor.idNote,
		encryptedVisitor.nameHash, encryptedVisitor.phoneHash, now, guardID); err != nil {
		return "", fmt.Errorf("create walk-in visitor: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO passes (id, token_hash, status, mode, visitor_id, host_id, gate_id, purpose,
		                    valid_from, valid_until, created_via, created_by, sticker_id)
		VALUES ($1, $2, 'active', $3, $4, $5, $6, $7, $8, $9, 'walk_in', $10, $11)`,
		passID, cryptoutil.TokenHash(cfg.MasterKey, token), mode, visitorID, hostID,
		input.GateID, input.Purpose, validFrom, validUntil, guardID, stickerID); err != nil {
		return "", fmt.Errorf("create walk-in pass: %w", err)
	}
	payload := passPayload{
		Visitor: input.Visitor, Purpose: input.Purpose, HostID: hostID,
		GateID: input.GateID, CreatedVia: "walk_in", IssuedAt: now,
	}
	if err := storePassSecrets(ctx, tx, cfg, passID, payload, token); err != nil {
		return "", fmt.Errorf("seal walk-in pass: %w", err)
	}
	if err := db.AppendAuditEvent(ctx, tx, db.AuditEvent{
		ActorID: &guardID, Action: "pass.activate", Target: passID,
		Meta: map[string]any{"visitor_id": visitorID, "created_via": "walk_in"},
	}); err != nil {
		return "", fmt.Errorf("audit walk-in pass activation: %w", err)
	}
	return passID, nil
}

func writeWalkInError(w http.ResponseWriter, err error) {
	var inputError walkInError
	if errors.As(err, &inputError) {
		writeErr(w, inputError.status, inputError.message)
		return
	}
	writeErr(w, http.StatusInternalServerError, "Could not create walk-in.")
}
