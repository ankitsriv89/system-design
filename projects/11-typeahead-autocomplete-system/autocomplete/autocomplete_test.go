// Package autocomplete tests core domain helpers.
package autocomplete

import (
	"testing"
)

func TestNormalizePrefix(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"  Hello World  ", "hello world"},
		{"GO", "go"},
		{"", ""},
		{"a", "a"},
	}
	for _, c := range cases {
		got := NormalizePrefix(c.in)
		if got != c.want {
			t.Errorf("NormalizePrefix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGeneratePrefixes(t *testing.T) {
	prefixes := GeneratePrefixes("Go", 5)
	if len(prefixes) != 2 {
		t.Fatalf("expected 2 prefixes, got %d", len(prefixes))
	}
	if prefixes[0] != "g" || prefixes[1] != "go" {
		t.Errorf("unexpected prefixes: %v", prefixes)
	}
}

func TestGeneratePrefixesMaxLen(t *testing.T) {
	prefixes := GeneratePrefixes("hello", 3)
	if len(prefixes) != 3 {
		t.Fatalf("expected 3 prefixes, got %d", len(prefixes))
	}
}

func TestScoreItem(t *testing.T) {
	if ScoreItem(42.0) != 42.0 {
		t.Error("ScoreItem should return popularity unchanged")
	}
}

func BenchmarkNormalizePrefix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NormalizePrefix("  The Quick Brown Fox  ")
	}
}

func BenchmarkGeneratePrefixes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GeneratePrefixes("elasticsearch", 20)
	}
}
