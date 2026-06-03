// Package notification provides unit tests for domain logic.
package notification

import (
	"testing"
	"time"
)

func TestRenderTemplate(t *testing.T) {
	tmpl := &Template{
		Subject: "Hello {{.Name}}",
		Body:    "Dear {{.Name}}, your code is {{.Code}}.",
	}
	subj, body := RenderTemplate(tmpl, map[string]string{"Name": "Alice", "Code": "1234"})
	if subj != "Hello Alice" {
		t.Fatalf("subject: got %q", subj)
	}
	if body != "Dear Alice, your code is 1234." {
		t.Fatalf("body: got %q", body)
	}
}

func TestRenderTemplate_MissingParam(t *testing.T) {
	tmpl := &Template{Body: "Hi {{.Name}}"}
	_, body := RenderTemplate(tmpl, map[string]string{})
	// placeholder left intact when param absent
	if body != "Hi {{.Name}}" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestIsQuietHour_NormalWindow(t *testing.T) {
	pref := &Preference{QuietStart: 22, QuietEnd: 8}
	cases := []struct {
		hour int
		want bool
	}{
		{22, true}, {23, true}, {0, true}, {7, true},
		{8, false}, {9, false}, {12, false}, {21, false},
	}
	for _, c := range cases {
		now := time.Date(2024, 1, 1, c.hour, 0, 0, 0, time.UTC)
		if got := IsQuietHour(pref, now); got != c.want {
			t.Errorf("hour=%d: got %v want %v", c.hour, got, c.want)
		}
	}
}

func TestIsQuietHour_Disabled(t *testing.T) {
	pref := &Preference{QuietStart: -1, QuietEnd: -1}
	now := time.Date(2024, 1, 1, 3, 0, 0, 0, time.UTC)
	if IsQuietHour(pref, now) {
		t.Fatal("expected not quiet")
	}
}

func TestIsQuietHour_SameDay(t *testing.T) {
	pref := &Preference{QuietStart: 10, QuietEnd: 14}
	cases := []struct {
		hour int
		want bool
	}{
		{9, false}, {10, true}, {13, true}, {14, false}, {15, false},
	}
	for _, c := range cases {
		now := time.Date(2024, 1, 1, c.hour, 0, 0, 0, time.UTC)
		if got := IsQuietHour(pref, now); got != c.want {
			t.Errorf("hour=%d: got %v want %v", c.hour, got, c.want)
		}
	}
}
