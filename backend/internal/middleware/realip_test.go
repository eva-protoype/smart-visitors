package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func mustPrefixes(t *testing.T, values ...string) []netip.Prefix {
	t.Helper()
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			t.Fatalf("parse prefix %q: %v", value, err)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

func requestIPAfterRealIP(t *testing.T, config ProxyConfig, remoteAddr string, headers map[string]string) string {
	t.Helper()
	var captured string
	handler := RealIP(config)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r.RemoteAddr
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	request.RemoteAddr = remoteAddr
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	handler.ServeHTTP(httptest.NewRecorder(), request)

	host, _, err := net.SplitHostPort(captured)
	if err != nil {
		t.Fatalf("captured RemoteAddr %q is not host:port: %v", captured, err)
	}
	return host
}

func TestRealIPUsesCloudflareHeaderFromTrustedProxy(t *testing.T) {
	config := ProxyConfig{
		TrustHeaders: true,
		TrustedCIDRs: mustPrefixes(t, "172.16.0.0/12"),
	}
	headers := map[string]string{
		"CF-Connecting-IP": "203.0.113.9",
		"X-Forwarded-For":  "198.51.100.7, 172.18.0.3",
	}
	if got := requestIPAfterRealIP(t, config, "172.18.0.2:54321", headers); got != "203.0.113.9" {
		t.Fatalf("client IP = %q, want 203.0.113.9", got)
	}
}

func TestRealIPUsesRightmostUntrustedForwardedHop(t *testing.T) {
	config := ProxyConfig{
		TrustHeaders: true,
		TrustedCIDRs: mustPrefixes(t, "172.16.0.0/12"),
	}
	headers := map[string]string{
		"X-Forwarded-For": "203.0.113.9, 198.51.100.7, 172.18.0.3, 172.18.0.4",
	}
	if got := requestIPAfterRealIP(t, config, "172.18.0.2:54321", headers); got != "198.51.100.7" {
		t.Fatalf("client IP = %q, want 198.51.100.7", got)
	}
}

func TestRealIPFallsBackToLeftmostHopWhenAllForwardedHopsAreTrusted(t *testing.T) {
	config := ProxyConfig{
		TrustHeaders: true,
		TrustedCIDRs: mustPrefixes(t, "172.16.0.0/12"),
	}
	headers := map[string]string{
		"X-Forwarded-For": "172.18.0.3, 172.18.0.4",
	}
	if got := requestIPAfterRealIP(t, config, "172.18.0.2:54321", headers); got != "172.18.0.3" {
		t.Fatalf("client IP = %q, want 172.18.0.3", got)
	}
}

func TestRealIPIgnoresHeadersFromUntrustedPeer(t *testing.T) {
	config := ProxyConfig{
		TrustHeaders: true,
		TrustedCIDRs: mustPrefixes(t, "172.16.0.0/12"),
	}
	headers := map[string]string{
		"CF-Connecting-IP": "1.2.3.4",
		"X-Forwarded-For":  "5.6.7.8",
	}
	if got := requestIPAfterRealIP(t, config, "198.51.100.20:40000", headers); got != "198.51.100.20" {
		t.Fatalf("client IP = %q, want direct peer 198.51.100.20", got)
	}
}

func TestRealIPIgnoresHeadersWhenTrustDisabled(t *testing.T) {
	config := ProxyConfig{
		TrustHeaders: false,
		TrustedCIDRs: mustPrefixes(t, "172.16.0.0/12"),
	}
	headers := map[string]string{
		"CF-Connecting-IP": "203.0.113.9",
		"X-Forwarded-For":  "198.51.100.7",
	}
	if got := requestIPAfterRealIP(t, config, "172.18.0.2:54321", headers); got != "172.18.0.2" {
		t.Fatalf("client IP = %q, want direct peer 172.18.0.2", got)
	}
}

func TestRealIPFallsBackWhenForwardedHeaderIsMalformed(t *testing.T) {
	config := ProxyConfig{
		TrustHeaders: true,
		TrustedCIDRs: mustPrefixes(t, "172.16.0.0/12"),
	}
	headers := map[string]string{
		"X-Forwarded-For": "not-an-ip, 172.18.0.3",
	}
	if got := requestIPAfterRealIP(t, config, "172.18.0.2:54321", headers); got != "172.18.0.2" {
		t.Fatalf("client IP = %q, want trusted peer fallback 172.18.0.2", got)
	}
}

func TestRealIPUsesForwardedHeaderWhenCloudflareHeaderIsInvalid(t *testing.T) {
	config := ProxyConfig{
		TrustHeaders: true,
		TrustedCIDRs: mustPrefixes(t, "172.16.0.0/12"),
	}
	headers := map[string]string{
		"CF-Connecting-IP": "not-an-ip",
		"X-Forwarded-For":  "198.51.100.7, 172.18.0.3",
	}
	if got := requestIPAfterRealIP(t, config, "172.18.0.2:54321", headers); got != "198.51.100.7" {
		t.Fatalf("client IP = %q, want 198.51.100.7", got)
	}
}

func TestRealIPHandlesIPv6PeerAndTrimmedCloudflareHeader(t *testing.T) {
	config := ProxyConfig{
		TrustHeaders: true,
		TrustedCIDRs: mustPrefixes(t, "2001:db8::/32"),
	}
	headers := map[string]string{
		"CF-Connecting-IP": " 203.0.113.9 ",
	}
	if got := requestIPAfterRealIP(t, config, "[2001:db8::2]:443", headers); got != "203.0.113.9" {
		t.Fatalf("client IP = %q, want 203.0.113.9", got)
	}
}
