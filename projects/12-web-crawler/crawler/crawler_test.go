// Package crawler tests for URL normalization, hashing, link extraction, robots parsing.
package crawler

import (
	"net/url"
	"testing"
	"time"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		raw      string
		base     string
		expected string
	}{
		{"https://Example.COM/path?q=1#frag", "", "https://example.com/path?q=1"},
		{"/relative", "https://example.com", "https://example.com/relative"},
		{"ftp://example.com", "", ""},
		{"HTTPS://EXAMPLE.COM/", "", "https://example.com/"},
	}
	for _, tt := range tests {
		var base *url.URL
		if tt.base != "" {
			base, _ = url.Parse(tt.base)
		}
		got, err := NormalizeURL(tt.raw, base)
		if err != nil {
			t.Fatalf("NormalizeURL(%q) unexpected error: %v", tt.raw, err)
		}
		if got != tt.expected {
			t.Errorf("NormalizeURL(%q) = %q, want %q", tt.raw, got, tt.expected)
		}
	}
}

func TestURLHash(t *testing.T) {
	h1 := URLHash("https://example.com/")
	h2 := URLHash("https://example.com/")
	if h1 != h2 {
		t.Error("same URL should produce same hash")
	}
	h3 := URLHash("https://other.com/")
	if h1 == h3 {
		t.Error("different URLs should produce different hashes")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex hash, got len=%d", len(h1))
	}
}

func TestContentHash(t *testing.T) {
	h := ContentHash([]byte("hello world"))
	if len(h) != 64 {
		t.Errorf("expected 64-char hex hash, got len=%d", len(h))
	}
}

func TestExtractLinks(t *testing.T) {
	body := []byte(`<html><body>
		<a href="https://example.com/page1">link1</a>
		<a href="/relative">rel</a>
		<a href="ftp://bad.com">bad</a>
	</body></html>`)
	base, _ := url.Parse("https://example.com/")
	links := ExtractLinks(body, base)
	found := make(map[string]bool)
	for _, l := range links {
		found[l] = true
	}
	if !found["https://example.com/page1"] {
		t.Error("expected absolute link")
	}
	if !found["https://example.com/relative"] {
		t.Error("expected resolved relative link")
	}
	for l := range found {
		if l == "ftp://bad.com" {
			t.Error("ftp link should be filtered")
		}
	}
}

func TestParseRobotsTxt(t *testing.T) {
	body := []byte(`
User-agent: *
Disallow: /private/
Disallow: /admin
Crawl-delay: 2

User-agent: googlebot
Disallow: /nogoogle
`)
	disallowed, delay := ParseRobotsTxt(body, "mycrawler")
	if delay != 2*time.Second {
		t.Errorf("expected 2s crawl delay, got %v", delay)
	}
	found := false
	for _, p := range disallowed {
		if p == "/private/" {
			found = true
		}
	}
	if !found {
		t.Error("expected /private/ in disallowed")
	}
}

func TestIsAllowed(t *testing.T) {
	rule := &RobotsRule{
		Disallowed: []string{"/private/", "/admin"},
	}
	if !IsAllowed("/public/page", rule) {
		t.Error("/public/page should be allowed")
	}
	if IsAllowed("/private/secret", rule) {
		t.Error("/private/secret should be disallowed")
	}
	if IsAllowed("/admin", rule) {
		t.Error("/admin should be disallowed")
	}
}

func BenchmarkURLHash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		URLHash("https://example.com/some/long/path?query=value&other=123")
	}
}

func BenchmarkContentHash(b *testing.B) {
	body := make([]byte, 4096)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ContentHash(body)
	}
}
