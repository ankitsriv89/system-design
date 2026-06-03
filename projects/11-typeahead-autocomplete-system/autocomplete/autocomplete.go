// Package autocomplete implements the core typeahead suggestion domain logic.
package autocomplete

import (
	"context"
	"strings"
	"time"
	"unicode"
)

// SuggestItem is the canonical corpus entry stored in PostgreSQL.
type SuggestItem struct {
	ID         int64     `json:"id"`
	Text       string    `json:"text"`
	Category   string    `json:"category"`
	Popularity float64   `json:"popularity"`
	Locale     string    `json:"locale"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Suggestion is a ranked result returned to the caller.
type Suggestion struct {
	Text       string  `json:"text"`
	Category   string  `json:"category"`
	Score      float64 `json:"score"`
	ItemID     int64   `json:"item_id"`
}

// QueryLog records each autocomplete query for click-through feedback.
type QueryLog struct {
	ID             int64     `json:"id"`
	Prefix         string    `json:"prefix"`
	SelectedItemID *int64    `json:"selected_item_id,omitempty"`
	LatencyMS      int64     `json:"latency_ms"`
	Locale         string    `json:"locale"`
	CreatedAt      time.Time `json:"created_at"`
}

// IndexStats summarises the current index state for the admin API.
type IndexStats struct {
	TotalItems      int64     `json:"total_items"`
	TotalPrefixes   int64     `json:"total_prefixes"`
	LastRebuildAt   time.Time `json:"last_rebuild_at"`
	RebuildDuration int64     `json:"rebuild_duration_ms"`
}

// Store is the interface the API and worker need from storage.
type Store interface {
	// Corpus mutations
	AddItem(ctx context.Context, item *SuggestItem) (int64, error)
	GetItem(ctx context.Context, id int64) (*SuggestItem, error)
	ListItems(ctx context.Context, locale string, limit, offset int) ([]*SuggestItem, error)
	DeleteItem(ctx context.Context, id int64) error
	IncrementPopularity(ctx context.Context, id int64, delta float64) error

	// Suggestion read path (served from Redis)
	Suggest(ctx context.Context, prefix, locale string, limit int) ([]*Suggestion, error)

	// Index management
	RebuildIndex(ctx context.Context) (*IndexStats, error)
	GetIndexStats(ctx context.Context) (*IndexStats, error)

	// Click feedback
	RecordQuery(ctx context.Context, log *QueryLog) error
}

// NormalizePrefix lowercases and strips leading/trailing whitespace from a prefix.
// Limiting to 64 characters prevents oversized Redis keys.
func NormalizePrefix(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	runes := []rune(s)
	// strip non-printable characters
	filtered := runes[:0]
	for _, r := range runes {
		if unicode.IsPrint(r) {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) > 64 {
		filtered = filtered[:64]
	}
	return string(filtered)
}

// GeneratePrefixes returns all prefixes of text from length 1 up to maxLen.
// These are the keys indexed into Redis sorted sets.
func GeneratePrefixes(text string, maxLen int) []string {
	normalized := NormalizePrefix(text)
	runes := []rune(normalized)
	if maxLen <= 0 || maxLen > len(runes) {
		maxLen = len(runes)
	}
	prefixes := make([]string, 0, maxLen)
	for i := 1; i <= maxLen; i++ {
		prefixes = append(prefixes, string(runes[:i]))
	}
	return prefixes
}

// ScoreItem computes the Redis sorted-set score for an item.
// Higher popularity → higher score → returned first by ZREVRANGEBYSCORE.
func ScoreItem(popularity float64) float64 {
	return popularity
}
