// Package crawler — SSRF protection helpers.
//
// Two defence layers:
//  1. IsPublicHost — pre-flight DNS check at URL-submission time.
//  2. NewSafeTransport — re-checks the resolved IP inside DialContext, called
//     after every DNS resolution (including redirects), defeating DNS rebinding.
package crawler

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// ErrSSRF is returned when a target resolves to a private or reserved IP.
var ErrSSRF = errors.New("ssrf: target resolves to a private or reserved address")

// isPrivateIP returns true for addresses that must never be dialled by the crawler:
// loopback, RFC 1918 / RFC 4193 private, link-local unicast/multicast,
// unspecified (0.0.0.0 / ::), and multicast ranges.
func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

// IsPublicHost resolves host and returns ErrSSRF if any resolved address is in
// a private range. Called at enqueue time (user-supplied input boundary).
func IsPublicHost(host string) error {
	addrs, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("ssrf: host lookup failed for %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("ssrf: no addresses for %q", host)
	}
	for _, ip := range addrs {
		if isPrivateIP(ip) {
			return fmt.Errorf("%w: %s resolves to %s", ErrSSRF, host, ip)
		}
	}
	return nil
}

// NewSafeTransport returns an *http.Transport that blocks connections to any
// private IP at dial time. This is the second layer: it defeats DNS rebinding
// (where a host passes IsPublicHost but resolves to a private IP on the actual
// connect attempt due to a crafted short-TTL DNS response).
func NewSafeTransport() *http.Transport {
	baseDialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("ssrf: bad address %q: %w", addr, err)
			}
			// Resolve IPs ourselves so we can inspect them before connecting.
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("ssrf: resolve %q: %w", host, err)
			}
			for _, ip := range ips {
				if isPrivateIP(ip) {
					return nil, fmt.Errorf("%w: %s → %s", ErrSSRF, host, ip)
				}
			}
			// Dial the first IP directly (bypasses a second resolver hop).
			return baseDialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
		MaxIdleConns:    100,
		IdleConnTimeout: 90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
}

// CheckRedirectSSRF validates each redirect destination before following it.
// Use as http.Client.CheckRedirect to block redirect-based SSRF.
func CheckRedirectSSRF(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return http.ErrUseLastResponse
	}
	if err := IsPublicHost(req.URL.Hostname()); err != nil {
		return fmt.Errorf("redirect blocked: %w", err)
	}
	return nil
}
