package handlers

import (
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"smart-visitors/internal/ai"
	"smart-visitors/internal/config"
)

func AITrain(db *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := ai.TrainModel(db, cfg.AIArtifactDir, cfg.AnomalyContamination)
		writeJSON(w, 200, res)
	}
}

func AIFlags(db *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 20
		if s := r.URL.Query().Get("limit"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				limit = v
			}
		}
		only := false
		if s := r.URL.Query().Get("only_suspicious"); s == "true" || s == "1" {
			only = true
		}
		res := ai.ScoreRecentWithForest(db, cfg.AIArtifactDir, limit, only)
		writeJSON(w, 200, res)
	}
}

func AIScore(db *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logID := chi.URLParam(r, "log_id")
		if logID == "" {
			// chi fallback
			writeErr(w, 404, "Access log not found")
			return
		}
		var id, passID, status string
		var gate, st sql.NullString
		err := db.QueryRow(`SELECT id, pass_id, gate_id, scan_time, status FROM access_logs WHERE id=?`, logID).Scan(&id, &passID, &gate, &st, &status)
		if err == sql.ErrNoRows {
			writeErr(w, 404, "Access log not found")
			return
		}
		if err != nil {
			writeErr(w, 500, err.Error())
			return
		}
		bundle := ai.LoadBundle(cfg.AIArtifactDir)
		// Build LogRow via shared fetch (reuse ScoreRecentWithForest path for one row is overkill; score inline)
		logs, _ := ai.FetchRecentLogs(db, 200)
		_ = logs
		// Fetch this log + siblings
		all, _ := ai.FetchAllLogs(db)
		var target *ai.LogRow
		for _, l := range all {
			if l.ID == logID {
				c := l
				target = &c
				break
			}
		}
		if target == nil {
			writeErr(w, 404, "Access log not found")
			return
		}
		byPass, _ := ai.FetchSiblings(db, []string{target.PassID})
		sibs := byPass[target.PassID]
		if len(sibs) == 0 {
			sibs = []ai.LogRow{*target}
		}
		gateFreq := map[string]float64{}
		if bundle != nil {
			gateFreq = bundle.GateFreq
		} else {
			gateFreq = ai.GetGateFreq(all)
		}
		features, reasons := ai.ExtractFeatures(*target, sibs, gateFreq)
		var score float64
		var susp bool
		var isForest bool
		if bundle != nil {
			if s, ok := ai.ScoreBatch(bundle, [][]float64{features}); ok && len(s) == 1 {
				score = s[0]
				susp = score < bundle.Threshold
				isForest = true
			}
		}
		if !isForest {
			score = ai.RuleScore(features)
			susp = score <= ai.RuleSuspiciousThreshold
		}
		if len(reasons) == 0 && susp {
			reasons = []string{"statistical outlier for this facility"}
		}
		_ = gate
		_ = st
		writeJSON(w, 200, map[string]any{
			"log_id": target.ID, "pass_id": target.PassID, "gate_id": target.GateID,
			"scan_time": target.ScanTime.UTC().Format("2006-01-02T15:04:05"),
			"status":      target.Status,
			"anomaly_score": round4local(score), "is_suspicious": susp, "reasons": reasons,
			"model": modelForOne(isForest),
		})
	}
}

func round4local(f float64) float64 {
	return math.Round(f*10000) / 10000
}

func modelForOne(isForest bool) string {
	if isForest {
		return "isolation_forest"
	}
	return "rules"
}

type NLQueryReq struct {
	Question string `json:"question"`
	Limit    int    `json:"limit"`
}

func AIQuery(db *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req NLQueryReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, 422, "Invalid JSON")
			return
		}
		lim := req.Limit
		if lim == 0 {
			lim = 20
		}
		s, params, columns, rows, answer, warnings, apiErr := ai.RunQuery(db, req.Question, &lim, cfg.AIMaxQueryRows)
		if apiErr != nil {
			writeErr(w, apiErr.Code, apiErr.Detail)
			return
		}
		if rows == nil {
			rows = [][]any{}
		}
		if warnings == nil {
			warnings = []string{}
		}
		if params == nil {
			params = map[string]any{}
		}
		writeJSON(w, 200, map[string]any{
			"sql": s, "params": params, "columns": columns,
			"rows": rows, "answer": answer, "warnings": warnings,
		})
	}
}
