package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"smart-visitors/internal/ai"
	"smart-visitors/internal/config"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func AITrain(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := database.QueryContext(r.Context(), `
			SELECT id::text, COALESCE(pass_id::text, ''), COALESCE(gate_id::text, ''),
			       scan_time, outcome
			FROM access_logs ORDER BY scan_time ASC LIMIT 100000`)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load scan history for model training.")
			return
		}
		defer rows.Close()
		logs := make([]ai.LogRow, 0)
		byPass := make(map[string][]ai.LogRow)
		for rows.Next() {
			var item ai.LogRow
			if err := rows.Scan(&item.ID, &item.PassID, &item.GateID, &item.ScanTime, &item.Status); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not load scan history for model training.")
				return
			}
			item.ScanTime = item.ScanTime.UTC()
			logs = append(logs, item)
			if item.PassID != "" {
				byPass[item.PassID] = append(byPass[item.PassID], item)
			}
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load scan history for model training.")
			return
		}
		gateFrequency := ai.GetGateFreq(logs)
		features := make([][]float64, 0, len(logs))
		for _, item := range logs {
			siblings := byPass[item.PassID]
			if len(siblings) == 0 {
				siblings = []ai.LogRow{item}
			}
			vector, _ := ai.ExtractFeatures(item, siblings, gateFrequency)
			features = append(features, vector)
		}
		response, status, err := callML(r.Context(), cfg, http.MethodPost, "/train", map[string]any{
			"features": features, "contamination": cfg.AnomalyContamination,
			"gate_frequency": gateFrequency,
		}, 120*time.Second)
		if err != nil {
			writeErr(w, status, "Model training service is unavailable.")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(response)
	}
}

func AIFlags(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, err := queryLimit(r, 20)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "limit must be between 1 and 100.")
			return
		}
		rows, err := database.QueryContext(r.Context(), `
			SELECT id::text, pass_id::text, gate_id::text, scan_time, direction, outcome,
			       anomaly_score, COALESCE(anomaly_reasons, '[]'::jsonb),
			       COALESCE(anomaly_suspicious, false)
			FROM access_logs WHERE anomaly_score IS NOT NULL
			ORDER BY anomaly_score ASC, seq DESC LIMIT $1`, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load anomaly flags.")
			return
		}
		defer rows.Close()
		flags := make([]map[string]any, 0)
		for rows.Next() {
			var id, direction, outcome string
			var passID, gateID sql.NullString
			var scanTime time.Time
			var score float64
			var rawReasons []byte
			var suspicious bool
			if err := rows.Scan(&id, &passID, &gateID, &scanTime, &direction, &outcome, &score, &rawReasons, &suspicious); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not load anomaly flags.")
				return
			}
			var reasons []string
			if err := json.Unmarshal(rawReasons, &reasons); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not load anomaly flags.")
				return
			}
			flags = append(flags, map[string]any{
				"log_id": id, "pass_id": nullString(passID), "gate_id": nullString(gateID),
				"scan_time": scanTime.UTC(), "direction": direction, "outcome": outcome,
				"anomaly_score": score, "is_suspicious": suspicious, "reasons": reasons,
			})
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load anomaly flags.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"count": len(flags), "flags": flags})
	}
}

func AIQuery(database *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Question string `json:"question"`
			Limit    int    `json:"limit"`
		}
		if err := decodeJSON(w, r, &input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		if len(input.Question) > 2000 {
			writeErr(w, http.StatusUnprocessableEntity, "question is too long.")
			return
		}
		var limit *int
		if input.Limit > 0 {
			limit = &input.Limit
		}
		query, params, columns, rows, answer, warnings, apiErr := ai.RunQuery(database, input.Question, limit, cfg.AIMaxQueryRows)
		if apiErr != nil {
			writeErr(w, apiErr.Code, apiErr.Detail)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"sql": query, "params": params, "columns": columns, "rows": rows,
			"answer": answer, "warnings": warnings,
		})
	}
}

func InternalScanHistory(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		passID := chi.URLParam(r, "pass_id")
		if _, err := uuid.Parse(passID); err != nil {
			writeErr(w, http.StatusBadRequest, "pass_id must be a UUID.")
			return
		}
		limit := 5000
		if value := r.URL.Query().Get("limit"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 || parsed > 5000 {
				writeErr(w, http.StatusBadRequest, "limit must be between 1 and 5000.")
				return
			}
			limit = parsed
		}
		rows, err := database.QueryContext(r.Context(), `
			SELECT id::text, pass_id::text, COALESCE(gate_id::text, ''), scan_time,
			       direction, outcome
			FROM access_logs WHERE pass_id = $1 ORDER BY scan_time ASC LIMIT $2`, passID, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load scan history.")
			return
		}
		defer rows.Close()
		history := make([]map[string]any, 0)
		for rows.Next() {
			var id, pass, gate, direction, outcome string
			var scanTime time.Time
			if err := rows.Scan(&id, &pass, &gate, &scanTime, &direction, &outcome); err != nil {
				writeErr(w, http.StatusInternalServerError, "Could not load scan history.")
				return
			}
			history = append(history, map[string]any{
				"log_id": id, "pass_id": pass, "gate_id": gate,
				"scan_time": scanTime.UTC(), "direction": direction, "outcome": outcome,
			})
		}
		if err := rows.Err(); err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not load scan history.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": history})
	}
}

func InternalAnomalyWriteback(database *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logID := chi.URLParam(r, "log_id")
		if _, err := uuid.Parse(logID); err != nil {
			writeErr(w, http.StatusBadRequest, "log_id must be a UUID.")
			return
		}
		var input struct {
			Score        float64  `json:"score"`
			IsSuspicious bool     `json:"is_suspicious"`
			Reasons      []string `json:"reasons"`
		}
		if err := decodeJSON(w, r, &input); err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid JSON request.")
			return
		}
		if input.Score < -1 || input.Score > 1 || len(input.Reasons) > 20 {
			writeErr(w, http.StatusUnprocessableEntity, "Invalid anomaly result.")
			return
		}
		reasons, err := json.Marshal(input.Reasons)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "Invalid anomaly result.")
			return
		}
		result, err := database.ExecContext(r.Context(), `
			UPDATE access_logs SET anomaly_score = $1, anomaly_reasons = $2::jsonb,
			                       anomaly_suspicious = $3
			WHERE id = $4`, input.Score, string(reasons), input.IsSuspicious, logID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "Could not store anomaly result.")
			return
		}
		affected, err := result.RowsAffected()
		if err != nil || affected == 0 {
			writeErr(w, http.StatusNotFound, "Access log not found.")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func RequireMLSignature(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" || !validMLSignature(r, token) {
				writeErr(w, http.StatusUnauthorized, "Invalid service authentication.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ScoreScanAsync(cfg config.Config, logID, passID, gateID string, scanTime time.Time, direction, outcome string) {
	if cfg.MLServiceToken == "" || cfg.MLURL == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := callML(ctx, cfg, http.MethodPost, "/score", map[string]any{
		"log_id": logID, "pass_id": passID, "gate_id": gateID,
		"scan_time": scanTime.UTC(), "direction": direction, "outcome": outcome,
	}, 5*time.Second); err != nil {
		slog.Warn("anomaly scoring unavailable", "log_id", logID, "error", err)
	}
}

func callML(ctx context.Context, cfg config.Config, method, path string, value any, timeout time.Duration) ([]byte, int, error) {
	if cfg.MLServiceToken == "" {
		return nil, http.StatusServiceUnavailable, errors.New("ML_SERVICE_TOKEN is not configured")
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	baseURL, err := url.Parse(strings.TrimRight(cfg.MLURL, "/"))
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, http.StatusServiceUnavailable, errors.New("ML_URL is invalid")
	}
	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, baseURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	req.Header.Set("Content-Type", "application/json")
	signMLRequest(req, cfg.MLServiceToken, body)
	client := &http.Client{Timeout: timeout}
	response, err := client.Do(req)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, http.StatusBadGateway, fmt.Errorf("ML service returned status %d", response.StatusCode)
	}
	return responseBody, response.StatusCode, nil
}

func signMLRequest(request *http.Request, token string, body []byte) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	digest := sha256.Sum256(body)
	canonical := timestamp + "\n" + request.Method + "\n" + request.URL.RequestURI() + "\n" + hex.EncodeToString(digest[:])
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(canonical))
	request.Header.Set("X-ML-Timestamp", timestamp)
	request.Header.Set("X-ML-Signature", hex.EncodeToString(mac.Sum(nil)))
}

func validMLSignature(request *http.Request, token string) bool {
	timestamp := request.Header.Get("X-ML-Timestamp")
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(seconds, 0)) > 5*time.Minute || time.Until(time.Unix(seconds, 0)) > 5*time.Minute {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, (1<<20)+1))
	if err != nil {
		return false
	}
	if len(body) > 1<<20 {
		return false
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	digest := sha256.Sum256(body)
	canonical := timestamp + "\n" + request.Method + "\n" + request.URL.RequestURI() + "\n" + hex.EncodeToString(digest[:])
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(canonical))
	provided, err := hex.DecodeString(request.Header.Get("X-ML-Signature"))
	return err == nil && hmac.Equal(provided, mac.Sum(nil))
}
