package config

import (
	"net/netip"
	"strings"
	"testing"
)

func setMinimalEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", strings.Repeat("a", 64))
	t.Setenv("MASTER_KEY", strings.Repeat("b", 64))
	t.Setenv("DATABASE_URL", "postgres://smart_visitors:test@db:5432/smart_visitors?sslmode=disable")
	t.Setenv("APP_BASE_URL", "https://tunnel.example")
	t.Setenv("VISITOR_BASE_URL", "")
	t.Setenv("VISITOR_HASH_ROUTES", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("TRUST_PROXY_HEADERS", "")
	t.Setenv("TRUSTED_PROXY_CIDRS", "")
}

func TestLoadParsesMultiOriginSettings(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", " https://project-dawnguard.github.io , https://localhost:5173/ ")
	t.Setenv("TRUST_PROXY_HEADERS", "true")
	t.Setenv("TRUSTED_PROXY_CIDRS", "172.16.0.0/12, 2001:db8::/32")
	t.Setenv("VISITOR_BASE_URL", "https://project-dawnguard.github.io/visitor/")
	t.Setenv("VISITOR_HASH_ROUTES", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	wantOrigins := []string{"https://project-dawnguard.github.io", "https://localhost:5173"}
	if len(cfg.CORSAllowedOrigins) != len(wantOrigins) {
		t.Fatalf("CORSAllowedOrigins = %#v, want %#v", cfg.CORSAllowedOrigins, wantOrigins)
	}
	for index, want := range wantOrigins {
		if cfg.CORSAllowedOrigins[index] != want {
			t.Errorf("CORSAllowedOrigins[%d] = %q, want %q", index, cfg.CORSAllowedOrigins[index], want)
		}
	}
	if !cfg.TrustProxyHeaders {
		t.Error("TrustProxyHeaders = false, want true")
	}
	if len(cfg.TrustedProxyCIDRs) != 2 {
		t.Fatalf("TrustedProxyCIDRs = %#v, want two prefixes", cfg.TrustedProxyCIDRs)
	}
	if cfg.TrustedProxyCIDRs[0] != netip.MustParsePrefix("172.16.0.0/12") {
		t.Errorf("TrustedProxyCIDRs[0] = %v, want 172.16.0.0/12", cfg.TrustedProxyCIDRs[0])
	}
	if cfg.VisitorBaseURL != "https://project-dawnguard.github.io/visitor/" {
		t.Errorf("VisitorBaseURL = %q, want explicit value", cfg.VisitorBaseURL)
	}
	if !cfg.VisitorHashRoutes {
		t.Error("VisitorHashRoutes = false, want true")
	}
}

func TestLoadDefaultsVisitorBaseURLToAppBaseURL(t *testing.T) {
	setMinimalEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.VisitorBaseURL != "https://tunnel.example" {
		t.Fatalf("VisitorBaseURL = %q, want APP_BASE_URL", cfg.VisitorBaseURL)
	}
	if cfg.VisitorHashRoutes {
		t.Fatal("VisitorHashRoutes = true, want false by default")
	}
}

func TestLoadRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("TRUSTED_PROXY_CIDRS", "172.16.0.0/12, not-a-cidr")

	if _, err := Load(); err == nil {
		t.Fatal("Load() succeeded with an invalid CIDR")
	}
}
