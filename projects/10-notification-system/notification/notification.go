// Package notification defines domain types and the fanout pipeline for the notification system.
package notification

import (
	"strings"
	"time"
)

// Channel represents a delivery channel.
type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelSMS   Channel = "sms"
	ChannelPush  Channel = "push"
)

// Status represents the lifecycle state of a notification.
type Status string

const (
	StatusPending    Status = "pending"
	StatusQueued     Status = "queued"
	StatusDelivered  Status = "delivered"
	StatusFailed     Status = "failed"
	StatusDLQ        Status = "dlq"
	StatusSkipped    Status = "skipped" // user preference opt-out or quiet hours
)

// Priority controls dispatch ordering.
type Priority int

const (
	PriorityLow    Priority = 0
	PriorityNormal Priority = 1
	PriorityHigh   Priority = 2
)

// Notification is the core domain record.
type Notification struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Channel    Channel   `json:"channel"`
	TemplateID string    `json:"template_id"`
	Params     map[string]string `json:"params"`
	Subject    string    `json:"subject"`
	Body       string    `json:"body"`
	Priority   Priority  `json:"priority"`
	Status     Status    `json:"status"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Preference stores a user's channel opt-in and quiet-hours configuration.
type Preference struct {
	UserID     string    `json:"user_id"`
	Channel    Channel   `json:"channel"`
	Enabled    bool      `json:"enabled"`
	QuietStart int       `json:"quiet_start"` // hour 0-23, -1 = disabled
	QuietEnd   int       `json:"quiet_end"`   // hour 0-23, -1 = disabled
	UpdatedAt  time.Time `json:"updated_at"`
}

// Template holds a named message template.
type Template struct {
	ID        string    `json:"id"`
	Channel   Channel   `json:"channel"`
	Subject   string    `json:"subject"` // email only
	Body      string    `json:"body"`    // supports {{.Key}} placeholders
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DeliveryAttempt records each send attempt.
type DeliveryAttempt struct {
	ID             string    `json:"id"`
	NotificationID string    `json:"notification_id"`
	Provider       string    `json:"provider"`
	AttemptNumber  int       `json:"attempt_number"`
	Status         Status    `json:"status"`
	ErrorMsg       string    `json:"error_msg,omitempty"`
	LatencyMs      int64     `json:"latency_ms"`
	AttemptedAt    time.Time `json:"attempted_at"`
}

// RenderTemplate performs simple {{.Key}} substitution on a template body/subject.
func RenderTemplate(tmpl *Template, params map[string]string) (subject, body string) {
	subject = tmpl.Subject
	body = tmpl.Body
	for k, v := range params {
		placeholder := "{{." + k + "}}"
		subject = strings.ReplaceAll(subject, placeholder, v)
		body = strings.ReplaceAll(body, placeholder, v)
	}
	return subject, body
}

// IsQuietHour returns true if the current local hour falls within the preference's quiet window.
func IsQuietHour(pref *Preference, now time.Time) bool {
	if pref.QuietStart < 0 || pref.QuietEnd < 0 {
		return false
	}
	h := now.Hour()
	if pref.QuietStart <= pref.QuietEnd {
		return h >= pref.QuietStart && h < pref.QuietEnd
	}
	// wraps midnight
	return h >= pref.QuietStart || h < pref.QuietEnd
}
