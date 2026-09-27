package config

import "os"
import "strconv"

type Config struct {
	DBConnection        string
	AnomalyContamination float64
	AIArtifactDir       string
	AIMaxQueryRows      int
	LLMProvider         string
	Port                string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() Config {
	contam := 0.1
	if v := os.Getenv("ANOMALY_CONTAMINATION"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			contam = f
		}
	}
	maxRows := 50
	if v := os.Getenv("AI_MAX_QUERY_ROWS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxRows = n
		}
	}
	return Config{
		DBConnection:         getenv("DB_CONNECTION", "sqlite:///./smart_visitors.db"),
		AnomalyContamination: contam,
		AIArtifactDir:        getenv("AI_ARTIFACT_DIR", "./artifacts"),
		AIMaxQueryRows:       maxRows,
		LLMProvider:          getenv("LLM_PROVIDER", "none"),
		Port:                 getenv("PORT", "8000"),
	}
}
