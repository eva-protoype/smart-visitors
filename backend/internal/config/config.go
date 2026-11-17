package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
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
	VisitorBaseURL       string
	VisitorHashRoutes    bool
	CORSAllowedOrigins   []string
	TrustProxyHeaders    bool
	TrustedProxyCIDRs    []netip.Prefix
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

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envOrigins(key string) []string {
	value := os.Getenv(key)
	if strings.TrimSpace(value) == "" {
		return nil
	}

	origins := make([]string, 0)
	for _, candidate := range strings.Split(value, ",") {
		origin := strings.TrimRight(strings.TrimSpace(candidate), "/")
		if origin != "" {
			origins = append(origins, origin)
		}
	}

	return origins
}

func envPrefixes(key string) ([]netip.Prefix, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil, nil
	}

	prefixes := make([]netip.Prefix, 0)
	for _, candidate := range strings.Split(value, ",") {
		raw := strings.TrimSpace(candidate)
		if raw == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, fmt.Errorf("%s contains invalid CIDR %q", key, raw)
		}
		prefixes = append(prefixes, prefix)
	}

	return prefixes, nil
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

	trustedProxyCIDRs, err := envPrefixes("TRUSTED_PROXY_CIDRS")
	if err != nil {
		return Config{}, err
	}

	appBaseURL := getenv("APP_BASE_URL", "http://localhost")

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
		AppBaseURL:           appBaseURL,
		VisitorBaseURL:       getenv("VISITOR_BASE_URL", appBaseURL),
		VisitorHashRoutes:    envBool("VISITOR_HASH_ROUTES", false),
		CORSAllowedOrigins:   envOrigins("CORS_ALLOWED_ORIGINS"),
		TrustProxyHeaders:    envBool("TRUST_PROXY_HEADERS", false),
		TrustedProxyCIDRs:    trustedProxyCIDRs,
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
