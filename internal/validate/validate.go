package validate

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// IsPrivateHost returns true if the hostname is a private, loopback,
// or reserved IP address. Matches the logic in 402-announce's isPrivateHost.
func IsPrivateHost(host string) bool {
	// Strip brackets from IPv6
	h := strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")

	// Strip zone ID
	if idx := strings.Index(h, "%"); idx != -1 {
		h = h[:idx]
	}

	// Localhost variants
	lower := strings.ToLower(strings.TrimSuffix(h, "."))
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return true
	}

	ip := net.ParseIP(h)
	if ip == nil {
		return false // DNS name, not an IP — assume public
	}

	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		isReserved(ip)
}

// isReserved checks for CGNAT (100.64/10), benchmarking (198.18/15),
// TEST-NET, 6to4 relay, and future-use ranges.
func isReserved(ip net.IP) bool {
	v4 := ip.To4()
	if v4 != nil {
		// CGNAT 100.64.0.0/10
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return true
		}
		// Benchmarking 198.18.0.0/15
		if v4[0] == 198 && (v4[1] == 18 || v4[1] == 19) {
			return true
		}
		// TEST-NET-1 192.0.2.0/24
		if v4[0] == 192 && v4[1] == 0 && v4[2] == 2 {
			return true
		}
		// TEST-NET-2 198.51.100.0/24
		if v4[0] == 198 && v4[1] == 51 && v4[2] == 100 {
			return true
		}
		// TEST-NET-3 203.0.113.0/24
		if v4[0] == 203 && v4[1] == 0 && v4[2] == 113 {
			return true
		}
		// IETF protocol assignments 192.0.0.0/24
		if v4[0] == 192 && v4[1] == 0 && v4[2] == 0 {
			return true
		}
		// 6to4 relay 192.88.99.0/24
		if v4[0] == 192 && v4[1] == 88 && v4[2] == 99 {
			return true
		}
		// Future use 240.0.0.0/4
		if v4[0] >= 240 {
			return true
		}
	}
	return false
}

// ValidatePublicURL checks that a URL uses http(s) and does not point to a
// private or loopback address.
func ValidatePublicURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got %q", u.Scheme)
	}
	hostname := u.Hostname()
	if IsPrivateHost(hostname) {
		return fmt.Errorf("URL points to a private/loopback address: %s", rawURL)
	}
	return nil
}
