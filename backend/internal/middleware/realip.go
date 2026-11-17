package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ProxyConfig controls whether forwarding headers are trusted and which
// immediate peers may set them.
type ProxyConfig struct {
	TrustHeaders bool
	TrustedCIDRs []netip.Prefix
}

// RealIP replaces RemoteAddr with the real client address when the request
// arrives from a trusted proxy. Untrusted peers cannot influence their own IP.
func RealIP(config ProxyConfig) func(http.Handler) http.Handler {
	isTrusted := func(addr netip.Addr) bool {
		for _, prefix := range config.TrustedCIDRs {
			if prefix.Contains(addr) {
				return true
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if config.TrustHeaders {
				peer, ok := parseRemoteAddr(r.RemoteAddr)
				if ok && isTrusted(peer) {
					if client, ok := forwardedClientIP(r, peer, isTrusted); ok {
						r.RemoteAddr = net.JoinHostPort(client.String(), "0")
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func forwardedClientIP(r *http.Request, peer netip.Addr, isTrusted func(netip.Addr) bool) (netip.Addr, bool) {
	if header := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); header != "" {
		if comma := strings.IndexByte(header, ','); comma >= 0 {
			header = strings.TrimSpace(header[:comma])
		}
		if client, ok := parseIP(header); ok {
			return client, true
		}
	}

	header := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if header == "" {
		return netip.Addr{}, false
	}

	hops := strings.Split(header, ",")
	candidate := peer
	for index := len(hops) - 1; index >= 0; index-- {
		hop, ok := parseIP(hops[index])
		if !ok {
			return netip.Addr{}, false
		}
		if !isTrusted(hop) {
			return hop, true
		}
		candidate = hop
	}

	return candidate, true
}

func parseRemoteAddr(remoteAddr string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return parseIP(host)
}

func parseIP(value string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}
