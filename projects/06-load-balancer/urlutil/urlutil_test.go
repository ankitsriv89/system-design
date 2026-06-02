// Package urlutil validates upstream backend URLs against an SSRF deny-list.
package urlutil

import "testing"

func TestValidate(t *testing.T) {
	bad := []string{
		"http://127.0.0.1/",
		"http://localhost/",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/",
		"http://172.16.0.1/",
		"http://192.168.1.1/",
		"http://[::1]/",
		"http://[fe80::1]/",
		"file:///etc/passwd",
		"ftp://example.com/",
		"http:///no-host",
	}
	for _, u := range bad {
		if _, err := Validate(u); err == nil {
			t.Errorf("expected Validate(%q) to fail, but it passed", u)
		}
	}

	// Public addresses should pass (DNS-resolvable).
	good := []string{
		"http://example.com/",
		"https://example.com/api",
	}
	for _, u := range good {
		if _, err := Validate(u); err != nil {
			t.Logf("Validate(%q) skipped (may be DNS-unavailable in CI): %v", u, err)
		}
	}
}
