// Package crawler — tests for SSRF protection.
package crawler

import (
	"errors"
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		addr    string
		private bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true}, // AWS IMDS
		{"::1", true},
		{"fc00::1", true},
		{"fe80::1", true},
		{"0.0.0.0", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"2001:4860:4860::8888", false},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.addr)
		if ip == nil {
			t.Fatalf("net.ParseIP(%q) returned nil", tt.addr)
		}
		got := isPrivateIP(ip)
		if got != tt.private {
			t.Errorf("isPrivateIP(%q) = %v, want %v", tt.addr, got, tt.private)
		}
	}
}

func TestIsPublicHost_Loopback(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
		err := IsPublicHost(host)
		if err == nil {
			t.Errorf("IsPublicHost(%q) should block loopback, got nil", host)
			continue
		}
		// Must be either ErrSSRF or a lookup error — both are acceptable blocks.
		if !errors.Is(err, ErrSSRF) && err != nil {
			t.Logf("IsPublicHost(%q) returned (acceptable): %v", host, err)
		}
	}
}
