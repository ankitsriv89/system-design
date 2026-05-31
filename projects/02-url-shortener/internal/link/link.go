package link

import (
	"errors"
	"net/url"
	"time"
)

var (
	ErrInvalidURL    = errors.New("long_url must be an absolute http or https URL")
	ErrOwnerRequired = errors.New("X-Owner-ID header is required")
	ErrQuotaExceeded = errors.New("owner quota exceeded")
	ErrCollision     = errors.New("could not allocate unique code")
	ErrNotFound      = errors.New("link not found")
	ErrExpired       = errors.New("link expired")
	ErrDisabled      = errors.New("link disabled")
)

type Link struct {
	Code       string
	LongURL    string
	OwnerID    string
	ExpiresAt  *time.Time
	CreatedAt  time.Time
	DisabledAt *time.Time
}

func ValidateLongURL(raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return ErrInvalidURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrInvalidURL
	}
	if u.Host == "" {
		return ErrInvalidURL
	}
	return nil
}

func (l Link) Active(now time.Time) error {
	if l.DisabledAt != nil {
		return ErrDisabled
	}
	if l.ExpiresAt != nil && !l.ExpiresAt.After(now) {
		return ErrExpired
	}
	return nil
}
