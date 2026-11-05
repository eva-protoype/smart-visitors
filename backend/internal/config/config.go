package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL          string
	JWTSecret            []byte
	MasterKey            []byte
	BootstrapAdminUser   string
	BootstrapAdminPass   string
	PassMaxHours         int
	RetentionPassDays    int
	RetentionLogDays     int
	TokenTTLHours        int
	RateLimitScan        int
	RateLimitPublic      int
	RateLimitLogin       int
	MLURL                string
	MLServiceToken       string
	AppBaseURL           string
	OffHourStart         int
	OffHourEnd           int
	AnomalyContamination float64
	PhotosDir            string
	AIArtifactDir        string
	AIMaxQueryRows       int
	Port                 string
	LogLevel             string
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}

	return parsed
}

func envHexKey(key string, minimumBytes int) ([]byte, error) {
	value := os.Getenv(key)
	if value == "" {
		return nil, fmt.Errorf("%s is required", key)
	}

	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != minimumBytes {
		return nil, fmt.Errorf("%s must be exactly %d bytes encoded as hex", key, minimumBytes)
	}

	return decoded, nil
}

func Load() (Config, error) {
	jwtSecret, err := envHexKey("JWT_SECRET", 32)
	if err != nil {
		return Config{}, err
	}
	masterKey, err := envHexKey("MASTER_KEY", 32)
	if err != nil {
		return Config{}, err
	}

	contamination := 0.1
	if value := os.Getenv("ANOMALY_CONTAMINATION"); value != "" {
		parsed, parseErr := strconv.ParseFloat(value, 64)
		if parseErr != nil || parsed <= 0 || parsed > 0.5 {
			return Config{}, errors.New("ANOMALY_CONTAMINATION must be greater than 0 and at most 0.5")
		}
		contamination = parsed
	}

	return Config{
		DatabaseURL:          getenv("DATABASE_URL", ""),
		JWTSecret:            jwtSecret,
		MasterKey:            masterKey,
		BootstrapAdminUser:   getenv("BOOTSTRAP_ADMIN_USER", "dev"),
		BootstrapAdminPass:   getenv("BOOTSTRAP_ADMIN_PASSWORD", ""),
		PassMaxHours:         envInt("PASS_MAX_HOURS", 24),
		RetentionPassDays:    envInt("RETENTION_PASS_DAYS", 30),
		RetentionLogDays:     envInt("RETENTION_LOG_DAYS", 90),
		TokenTTLHours:        envInt("TOKEN_TTL_HOURS", 12),
		RateLimitScan:        envInt("RATE_LIMIT_SCAN", 30),
		RateLimitPublic:      envInt("RATE_LIMIT_PUBLIC", 10),
		RateLimitLogin:       envInt("RATE_LIMIT_LOGIN", 5),
		MLURL:                getenv("ML_URL", "http://ml:8100"),
		MLServiceToken:       os.Getenv("ML_SERVICE_TOKEN"),
		AppBaseURL:           getenv("APP_BASE_URL", "http://localhost"),
		OffHourStart:         envInt("OFF_HOUR_START", 22),
		OffHourEnd:           envInt("OFF_HOUR_END", 6),
		AnomalyContamination: contamination,
		PhotosDir:            getenv("PHOTOS_DIR", "/data/photos"),
		AIArtifactDir:        getenv("AI_ARTIFACT_DIR", "/data/artifacts"),
		AIMaxQueryRows:       envInt("AI_MAX_QUERY_ROWS", 50),
		Port:                 getenv("PORT", "8000"),
		LogLevel:             getenv("LOG_LEVEL", "info"),
	}, nil
}

func (c Config) ValidateForStartup() error {
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	return nil
}
