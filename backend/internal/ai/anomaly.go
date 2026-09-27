package ai

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"time"
)

// LogRow mirrors AccessLogModel row with parsed time.
type LogRow struct {
	ID       string
	PassID   string
	GateID   string
	ScanTime time.Time
	Status   string
}

func parseTime(s string) time.Time {
	// SQLite stores "2026-09-13 02:14:00" or RFC3339; try several layouts.
	layouts := []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC()
		}
	}
	return time.Now().UTC()
}

func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05")
}

// Flag is the API item shape (task.md §3.2).
type Flag struct {
	LogID        string   `json:"log_id"`
	PassID       string   `json:"pass_id"`
	GateID       string   `json:"gate_id"`
	ScanTime     string   `json:"scan_time"`
	Status       string   `json:"status"`
	AnomalyScore float64  `json:"anomaly_score"`
	IsSuspicious bool     `json:"is_suspicious"`
	Reasons      []string `json:"reasons"`
}

func FetchAllLogs(db *sql.DB) ([]LogRow, error) {
	rows, err := db.Query(`SELECT id, pass_id, gate_id, scan_time, status FROM access_logs ORDER BY scan_time ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LogRow
	for rows.Next() {
		var id, passID, status string
		var gate sql.NullString
		var st sql.NullString
		if err := rows.Scan(&id, &passID, &gate, &st, &status); err != nil {
			return nil, err
		}
		g := "Main Gate"
		if gate.Valid && gate.String != "" {
			g = gate.String
		}
		t := time.Now().UTC()
		if st.Valid && st.String != "" {
			t = parseTime(st.String)
		}
		out = append(out, LogRow{ID: id, PassID: passID, GateID: g, ScanTime: t, Status: status})
	}
	return out, rows.Err()
}

func FetchRecentLogs(db *sql.DB, limit int) ([]LogRow, error) {
	rows, err := db.Query(`SELECT id, pass_id, gate_id, scan_time, status FROM access_logs ORDER BY scan_time DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LogRow
	for rows.Next() {
		var id, passID, status string
		var gate sql.NullString
		var st sql.NullString
		if err := rows.Scan(&id, &passID, &gate, &st, &status); err != nil {
			return nil, err
		}
		g := "Main Gate"
		if gate.Valid && gate.String != "" {
			g = gate.String
		}
		t := time.Now().UTC()
		if st.Valid && st.String != "" {
			t = parseTime(st.String)
		}
		out = append(out, LogRow{ID: id, PassID: passID, GateID: g, ScanTime: t, Status: status})
	}
	return out, rows.Err()
}

func FetchSiblings(db *sql.DB, passIDs []string) (map[string][]LogRow, error) {
	grouped := map[string][]LogRow{}
	if len(passIDs) == 0 {
		return grouped, nil
	}
	uniq := map[string]bool{}
	var ids []string
	for _, p := range passIDs {
		if !uniq[p] {
			uniq[p] = true
			ids = append(ids, p)
		}
	}
	// Build placeholders
	ph := ""
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		if i > 0 {
			ph += ","
		}
		ph += "?"
		args = append(args, id)
	}
	rows, err := db.Query(`SELECT id, pass_id, gate_id, scan_time, status FROM access_logs WHERE pass_id IN (`+ph+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, passID, status string
		var gate sql.NullString
		var st sql.NullString
		if err := rows.Scan(&id, &passID, &gate, &st, &status); err != nil {
			return nil, err
		}
		g := "Main Gate"
		if gate.Valid && gate.String != "" {
			g = gate.String
		}
		t := time.Now().UTC()
		if st.Valid && st.String != "" {
			t = parseTime(st.String)
		}
		r := LogRow{ID: id, PassID: passID, GateID: g, ScanTime: t, Status: status}
		grouped[passID] = append(grouped[passID], r)
	}
	return grouped, rows.Err()
}

func GetGateFreq(logs []LogRow) map[string]float64 {
	total := len(logs)
	if total == 0 {
		return map[string]float64{}
	}
	counts := map[string]int{}
	for _, l := range logs {
		g := l.GateID
		if g == "" {
			g = "Main Gate"
		}
		counts[g]++
	}
	out := map[string]float64{}
	for g, c := range counts {
		out[g] = float64(c) / float64(total)
	}
	return out
}

// ExtractFeatures builds 7-dim vector + reasons (task.md §4 verbatim).
func ExtractFeatures(log LogRow, siblings []LogRow, gateFreq map[string]float64) ([]float64, []string) {
	hour := float64(log.ScanTime.Hour())
	off := IsOffHours(log.ScanTime)
	weekend := log.ScanTime.Weekday() == time.Saturday || log.ScanTime.Weekday() == time.Sunday
	gate := log.GateID
	if gate == "" {
		gate = "Main Gate"
	}
	rarity := 1.0 - gateFreq[gate] // unseen => 1.0

	velocity := 0
	hoppingSet := map[string]bool{gate: true}
	denied24 := 0
	if siblings == nil {
		siblings = []LogRow{log}
	}
	for _, sib := range siblings {
		if HoursBetween(sib.ScanTime, log.ScanTime) <= 1.0 {
			velocity++
		}
		if HoursBetween(sib.ScanTime, log.ScanTime) <= 24.0 {
			g := sib.GateID
			if g == "" {
				g = "Main Gate"
			}
			hoppingSet[g] = true
		}
		if sib.Status == "denied" && HoursBetween(sib.ScanTime, log.ScanTime) <= 24.0 {
			denied24++
		}
	}
	if velocity > 10 {
		velocity = 10
	}
	hopping := len(hoppingSet)
	if hopping > 10 {
		hopping = 10
	}
	isDenied := 0.0
	if log.Status == "denied" {
		isDenied = 1.0
	}
	features := []float64{
		hour / 23.0,
		boolToFloat(off),
		boolToFloat(weekend),
		rarity,
		float64(velocity) / 10.0,
		float64(hopping) / 10.0,
		isDenied,
	}
	var reasons []string
	if off {
		reasons = append(reasons, fmt.Sprintf("off-hours entry (%02d:00)", log.ScanTime.Hour()))
	}
	if weekend {
		reasons = append(reasons, "weekend entry")
	}
	if velocity >= 3 {
		reasons = append(reasons, fmt.Sprintf("rapid repeat scans: %d in 1h", velocity))
	}
	if hopping >= 3 {
		reasons = append(reasons, fmt.Sprintf("gate hopping: %d gates in 24h", hopping))
	} else if hopping == 2 {
		reasons = append(reasons, "multi-gate use in 24h")
	}
	if isDenied == 1.0 {
		if denied24 <= 1 {
			reasons = append(reasons, "denied entry")
		} else {
			reasons = append(reasons, fmt.Sprintf("%d denied in 24h", denied24))
		}
	}
	if rarity >= 0.9 {
		reasons = append(reasons, fmt.Sprintf("rare gate: %s", gate))
	}
	if reasons == nil {
		reasons = []string{}
	}
	return features, reasons
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// RuleScore mirrors anomaly.py rule_score in [-1,0].
func RuleScore(features []float64) float64 {
	v := features[4] * 10.0
	h := features[5]*10.0 - 1.0
	if h < 0 {
		h = 0
	}
	s := -(0.45*features[1] + 0.25*math.Min(v, 3.0)/3.0 + 0.20*math.Min(h, 3.0)/3.0 + 0.10*features[6])
	if features[2] == 1.0 {
		s -= 0.05
	}
	if s < -1.0 {
		s = -1.0
	}
	return s
}

func round4(f float64) float64 {
	return math.Round(f*10000) / 10000
}

// ScoreRecent scores newest limit rows using bundle if present else rules.
func ScoreRecent(db *sql.DB, bundle *Bundle, limit int, onlySuspicious bool) ([]Flag, string) {
	if limit < 1 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	logs, err := FetchRecentLogs(db, limit)
	if err != nil {
		return []Flag{}, "rules"
	}
	if len(logs) == 0 {
		return []Flag{}, modelName(bundle, false)
	}
	passIDs := make([]string, 0, len(logs))
	for _, l := range logs {
		passIDs = append(passIDs, l.PassID)
	}
	byPass, _ := FetchSiblings(db, passIDs)
	gateFreq := map[string]float64{}
	if bundle != nil && bundle.GateFreq != nil {
		gateFreq = bundle.GateFreq
	} else {
		all, _ := FetchAllLogs(db)
		gateFreq = GetGateFreq(all)
	}
	usedForest := false
	var flags []Flag
	for _, l := range logs {
		sibs := byPass[l.PassID]
		if len(sibs) == 0 {
			sibs = []LogRow{l}
		}
		features, reasons := ExtractFeatures(l, sibs, gateFreq)
		score, susp, isForest := scoreWithBundle(features, bundle)
		if isForest {
			usedForest = true
		}
		if len(reasons) == 0 && susp {
			reasons = []string{"statistical outlier for this facility"}
		}
		flags = append(flags, Flag{
			LogID: l.ID, PassID: l.PassID, GateID: l.GateID,
			ScanTime: formatTime(l.ScanTime), Status: l.Status,
			AnomalyScore: round4(score), IsSuspicious: susp, Reasons: reasons,
		})
	}
	if onlySuspicious {
		var kept []Flag
		for _, f := range flags {
			if f.IsSuspicious {
				kept = append(kept, f)
			}
		}
		flags = kept
	}
	sort.Slice(flags, func(i, j int) bool { return flags[i].AnomalyScore < flags[j].AnomalyScore })
	if flags == nil {
		flags = []Flag{}
	}
	return flags, modelName(bundle, usedForest)
}

func modelName(bundle *Bundle, usedForest bool) string {
	if bundle != nil && bundle.HasModel && usedForest {
		return "isolation_forest"
	}
	// If bundle exists but no flag used forest (e.g. empty), report forest when bundle valid?
	if bundle != nil && bundle.HasModel && !usedForest {
		// score_recent in Python reports forest if any flag used forest, else rules.
		// With zero flags, fall back to bundle presence.
		return "isolation_forest"
	}
	return "rules"
}

func scoreWithBundle(features []float64, bundle *Bundle) (float64, bool, bool) {
	if bundle != nil && bundle.HasModel {
		s, susp, ok := bundle.Score(features)
		if ok {
			return s, susp, true
		}
	}
	s := RuleScore(features)
	return s, s <= RuleSuspiciousThreshold, false
}
