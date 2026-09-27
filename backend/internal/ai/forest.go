package ai

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// Bundle mirrors the pickle metadata; model itself lives in Python pickle.
type Bundle struct {
	GateFreq      map[string]float64 `json:"gate_freq"`
	Threshold     float64            `json:"threshold"`
	NSamples      int                `json:"n_samples"`
	Contamination float64            `json:"contamination"`
	TrainedAt     string             `json:"trained_at"`
	HasModel      bool               `json:"-"`
	ArtifactPath  string             `json:"-"`
}

func ArtifactPath(artifactDir string) string {
	return filepath.Join(artifactDir, ArtifactName)
}

func MetaPath(artifactDir string) string {
	return filepath.Join(artifactDir, "isolation_forest.json")
}

// LoadBundle reads JSON sidecar; missing file => nil (rules mode).
func LoadBundle(artifactDir string) *Bundle {
	data, err := os.ReadFile(MetaPath(artifactDir))
	if err != nil {
		return nil
	}
	var b Bundle
	if err := json.Unmarshal(data, &b); err != nil {
		return nil
	}
	b.HasModel = true
	b.ArtifactPath = ArtifactPath(artifactDir)
	// Guard: pickle must exist too, else stale meta.
	if _, err := os.Stat(b.ArtifactPath); err != nil {
		return nil
	}
	return &b
}

func DropStaleArtifact(artifactDir string) {
	_ = os.Remove(ArtifactPath(artifactDir))
	_ = os.Remove(MetaPath(artifactDir))
}

// Score delegates to Python helper for a batch; falls back to rules on error.
func (b *Bundle) Score(features []float64) (float64, bool, bool) {
	s, ok := ScoreBatch(b, [][]float64{features})
	if !ok || len(s) == 0 {
		return 0, false, false
	}
	return s[0], s[0] < b.Threshold, true
}

func helperPath() string {
	// ml/isolation.py relative to backend working dir.
	for _, c := range []string{"ml/isolation.py", "backend/ml/isolation.py"} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "ml/isolation.py"
}

func pythonBin() string {
	if p := os.Getenv("PYTHON_BIN"); p != "" {
		return p
	}
	return "python3"
}

// ScoreBatch calls helper once for all vectors.
func ScoreBatch(b *Bundle, features [][]float64) ([]float64, bool) {
	if b == nil || !b.HasModel {
		return nil, false
	}
	payload, _ := json.Marshal(map[string]any{
		"features": features,
		"artifact": b.ArtifactPath,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, pythonBin(), helperPath(), "score",
		"--artifact", b.ArtifactPath)
	cmd.Stdin = bytes.NewReader(payload)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, false
	}
	var resp struct {
		Scores []float64 `json:"scores"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		return nil, false
	}
	return resp.Scores, true
}

// TrainModel fits forest via helper; falls back to rules when thin or helper fails.
func TrainModel(db *sql.DB, artifactDir string, contamination float64) map[string]any {
	logs, _ := FetchAllLogs(db)
	n := len(logs)
	gateFreq := GetGateFreq(logs)
	if n < MinTrainSamples {
		DropStaleArtifact(artifactDir)
		return map[string]any{
			"model": "rules", "n_samples": n, "n_features": NFeatures,
			"contamination": contamination, "artifact": nil,
		}
	}
	byPass := map[string][]LogRow{}
	for _, l := range logs {
		byPass[l.PassID] = append(byPass[l.PassID], l)
	}
	features := make([][]float64, 0, n)
	for _, l := range logs {
		f, _ := ExtractFeatures(l, byPass[l.PassID], gateFreq)
		features = append(features, f)
	}
	threshold, ok := trainViaHelper(features, artifactDir, contamination)
	if !ok {
		DropStaleArtifact(artifactDir)
		return map[string]any{
			"model": "rules", "n_samples": n, "n_features": NFeatures,
			"contamination": contamination, "artifact": nil,
		}
	}
	_ = os.MkdirAll(artifactDir, 0o755)
	meta := Bundle{
		GateFreq: gateFreq, Threshold: threshold, NSamples: n,
		Contamination: contamination, TrainedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.Marshal(meta)
	_ = os.WriteFile(MetaPath(artifactDir), data, 0o644)
	return map[string]any{
		"model": "isolation_forest", "n_samples": n, "n_features": NFeatures,
		"contamination": contamination, "artifact": ArtifactPath(artifactDir),
	}
}

func trainViaHelper(features [][]float64, artifactDir string, contamination float64) (float64, bool) {
	_ = os.MkdirAll(artifactDir, 0o755)
	payload, _ := json.Marshal(map[string]any{"features": features, "contamination": contamination})
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, pythonBin(), helperPath(), "train",
		"--artifact", ArtifactPath(artifactDir),
		"--contamination", floatStr(contamination))
	cmd.Stdin = bytes.NewReader(payload)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Run(); err != nil {
		return 0, false
	}
	var resp struct {
		Threshold float64 `json:"threshold"`
		NSamples  int     `json:"n_samples"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		return 0, false
	}
	return resp.Threshold, true
}

func floatStr(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func ScoreRecentWithForest(db *sql.DB, artifactDir string, limit int, onlySuspicious bool) map[string]any {
	if limit < 1 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	bundle := LoadBundle(artifactDir)
	// Fast path: no model => pure rules (anomaly.go ScoreRecent handles it).
	if bundle == nil {
		flags, model := ScoreRecent(db, nil, limit, onlySuspicious)
		return map[string]any{"model": model, "count": len(flags), "flags": flags}
	}
	logs, err := FetchRecentLogs(db, limit)
	if err != nil || len(logs) == 0 {
		return map[string]any{"model": "isolation_forest", "count": 0, "flags": []Flag{}}
	}
	passIDs := make([]string, 0, len(logs))
	for _, l := range logs {
		passIDs = append(passIDs, l.PassID)
	}
	byPass, _ := FetchSiblings(db, passIDs)
	features := make([][]float64, 0, len(logs))
	reasonsList := make([][]string, 0, len(logs))
	for _, l := range logs {
		sibs := byPass[l.PassID]
		if len(sibs) == 0 {
			sibs = []LogRow{l}
		}
		f, r := ExtractFeatures(l, sibs, bundle.GateFreq)
		features = append(features, f)
		reasonsList = append(reasonsList, r)
	}
	scores, ok := ScoreBatch(bundle, features)
	var flags []Flag
	for i, l := range logs {
		var score float64
		var susp bool
		var usedForest = true
		if ok && i < len(scores) {
			score = scores[i]
			susp = score < bundle.Threshold
		} else {
			score = RuleScore(features[i])
			susp = score <= RuleSuspiciousThreshold
			usedForest = false
		}
		_ = usedForest
		reasons := reasonsList[i]
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
	// Most suspicious first
	for i := 0; i < len(flags); i++ {
		for j := i + 1; j < len(flags); j++ {
			if flags[j].AnomalyScore < flags[i].AnomalyScore {
				flags[i], flags[j] = flags[j], flags[i]
			}
		}
	}
	if flags == nil {
		flags = []Flag{}
	}
	return map[string]any{"model": "isolation_forest", "count": len(flags), "flags": flags}
}
