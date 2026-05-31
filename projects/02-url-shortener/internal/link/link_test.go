package link

import (
	"strings"
	"testing"
	"time"
)

func TestRandomCodeGeneratorUsesBase62Alphabet(t *testing.T) {
	code, err := (RandomCodeGenerator{}).Generate(32)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 32 {
		t.Fatalf("len = %d, want 32", len(code))
	}
	for _, ch := range code {
		if !strings.ContainsRune(Alphabet, ch) {
			t.Fatalf("unexpected character %q in %q", ch, code)
		}
	}
}

func TestValidateLongURL(t *testing.T) {
	valid := []string{"https://example.com/a?b=c", "http://localhost:8080/demo"}
	for _, raw := range valid {
		if err := ValidateLongURL(raw); err != nil {
			t.Fatalf("ValidateLongURL(%q) unexpected error: %v", raw, err)
		}
	}

	invalid := []string{"ftp://example.com", "example.com", "/relative", "https://"}
	for _, raw := range invalid {
		if err := ValidateLongURL(raw); err == nil {
			t.Fatalf("ValidateLongURL(%q) expected error", raw)
		}
	}
}

func TestLinkActiveState(t *testing.T) {
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	if err := (Link{ExpiresAt: &future}).Active(now); err != nil {
		t.Fatalf("future expiry should be active: %v", err)
	}
	if err := (Link{ExpiresAt: &past}).Active(now); err != ErrExpired {
		t.Fatalf("past expiry err = %v, want ErrExpired", err)
	}
	if err := (Link{DisabledAt: &past}).Active(now); err != ErrDisabled {
		t.Fatalf("disabled err = %v, want ErrDisabled", err)
	}
}
