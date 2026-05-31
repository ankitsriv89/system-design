package analytics

import "time"

type ClickEvent struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	ClickedAt time.Time `json:"clicked_at"`
	Referrer  string    `json:"referrer"`
	UserAgent string    `json:"user_agent"`
	IPHash    string    `json:"ip_hash"`
}

type Stats struct {
	Code         string       `json:"code"`
	LongURL      string       `json:"long_url"`
	OwnerID      string       `json:"owner_id"`
	CreatedAt    time.Time    `json:"created_at"`
	ExpiresAt    *time.Time   `json:"expires_at,omitempty"`
	TotalClicks  int64        `json:"total_clicks"`
	RecentClicks []ClickEvent `json:"recent_clicks"`
}
