// Package urlutil validates upstream backend URLs against an SSRF deny-list.
package urlutil

import (
	"fmt"
	"net"
	"net/url"
)

// denied is the set of networks that must never be reached as upstream backends.
// Covers: loopback, link-local (incl. AWS IMDS 169.254.169.254), RFC1918,
// IPv6 loopback/link-local/ULA, unspecified, and multicast.
var denied = func() []*net.IPNet {
	cidrs := []string{
		"127.0.0.0/8",          // IPv4 loopback
		"::1/128",              // IPv6 loopback
		"169.254.0.0/16",       // IPv4 link-local + AWS/GCP IMDS
		"fe80::/10",            // IPv6 link-local
		"10.0.0.0/8",           // RFC1918
		"172.16.0.0/12",        // RFC1918
		"192.168.0.0/16",       // RFC1918
		"fc00::/7",             // IPv6 ULA (fc00::/7 covers fc00:: and fd00::)
		"0.0.0.0/8",            // unspecified
		"100.64.0.0/10",        // CGNAT / shared address space
		"192.0.0.0/24",         // IETF protocol assignments
		"198.18.0.0/15",        // benchmark testing
		"198.51.100.0/24",      // TEST-NET-2 (documentation)
		"203.0.113.0/24",       // TEST-NET-3 (documentation)
		"240.0.0.0/4",          // reserved
		"224.0.0.0/4",          // multicast
		"ff00::/8",             // IPv6 multicast
	}
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic("urlutil: bad CIDR " + c)
		}
		out = append(out, n)
	}
	return out
}()

// Validate parses rawURL and verifies:
//   - scheme is http or https
//   - hostname resolves to at least one IP
//   - none of those IPs fall in the deny-list
//
// Returns the parsed *url.URL on success.
func Validate(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("urlutil: parse: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("urlutil: scheme must be http or https, got %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("urlutil: empty hostname")
	}

	addrs, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("urlutil: resolve %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("urlutil: %q resolved to no addresses", host)
	}

	for _, ip := range addrs {
		if err := checkIP(ip); err != nil {
			return nil, fmt.Errorf("urlutil: %q resolves to restricted IP %s: %w", host, ip, err)
		}
	}
	return u, nil
}

// IsDenied returns true if ip falls in any deny-list network.
// Exported so the DialContext hook in the HTTP transport can recheck at connect
// time (mitigates DNS rebinding between Validate and actual dial).
func IsDenied(ip net.IP) bool {
	for _, n := range denied {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func checkIP(ip net.IP) error {
	for _, n := range denied {
		if n.Contains(ip) {
			return fmt.Errorf("address in restricted range %s", n)
		}
	}
	return nil
}
