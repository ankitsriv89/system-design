// Package policy defines the Policy domain model and in-process cache.
package policy

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

type Algorithm string

const (
	AlgorithmTokenBucket    Algorithm = "token_bucket"
	AlgorithmSlidingWindow  Algorithm = "sliding_window"
)

type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
)

// Policy describes a rate-limit rule for a subject type.
type Policy struct {
	ID          string    `json:"id"`
	SubjectType string    `json:"subject_type"` // e.g. "user", "ip", "api_key"
	Algorithm   Algorithm `json:"algorithm"`
	Capacity    float64   `json:"capacity"`    // token bucket: max tokens; sliding window: max requests
	RefillRate  float64   `json:"refill_rate"` // token bucket: tokens/s; sliding window: ignored
	WindowMs    int64     `json:"window_ms"`   // sliding window: duration in ms
	Action      Action    `json:"action"`      // what to do on limit: deny
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Decision is one audit decision recorded after a rate-limit check.
type Decision struct {
	ID            int64     `json:"id"`
	Subject       string    `json:"subject"`
	PolicyID      string    `json:"policy_id"`
	Allowed       bool      `json:"allowed"`
	Reason        string    `json:"reason,omitempty"`
	RetryAfterMs  int64     `json:"retry_after_ms,omitempty"`
	DecidedAt     time.Time `json:"decided_at"`
}

// Store persists policies in PostgreSQL.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

const schema = `
CREATE TABLE IF NOT EXISTS policies (
  id           TEXT PRIMARY KEY,
  subject_type TEXT NOT NULL,
  algorithm    TEXT NOT NULL,
  capacity     DOUBLE PRECISION NOT NULL,
  refill_rate  DOUBLE PRECISION NOT NULL DEFAULT 0,
  window_ms    BIGINT NOT NULL DEFAULT 0,
  action       TEXT NOT NULL DEFAULT 'deny',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_decisions (
  id            BIGSERIAL PRIMARY KEY,
  subject       TEXT NOT NULL,
  policy_id     TEXT NOT NULL,
  allowed       BOOLEAN NOT NULL,
  reason        TEXT,
  retry_after_ms BIGINT,
  decided_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audit_subject_idx ON audit_decisions(subject, decided_at DESC);
`

func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *Store) Upsert(ctx context.Context, p *Policy) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO policies (id, subject_type, algorithm, capacity, refill_rate, window_ms, action, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,now())
		ON CONFLICT (id) DO UPDATE SET
		  subject_type=EXCLUDED.subject_type,
		  algorithm=EXCLUDED.algorithm,
		  capacity=EXCLUDED.capacity,
		  refill_rate=EXCLUDED.refill_rate,
		  window_ms=EXCLUDED.window_ms,
		  action=EXCLUDED.action,
		  updated_at=now()`,
		p.ID, p.SubjectType, p.Algorithm, p.Capacity, p.RefillRate, p.WindowMs, p.Action,
	)
	return err
}

func (s *Store) Get(ctx context.Context, id string) (*Policy, error) {
	p := &Policy{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, subject_type, algorithm, capacity, refill_rate, window_ms, action, created_at, updated_at
		FROM policies WHERE id=$1`, id).Scan(
		&p.ID, &p.SubjectType, &p.Algorithm, &p.Capacity, &p.RefillRate, &p.WindowMs, &p.Action, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("policy %s not found", id)
	}
	return p, err
}

func (s *Store) List(ctx context.Context) ([]*Policy, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, subject_type, algorithm, capacity, refill_rate, window_ms, action, created_at, updated_at
		FROM policies ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Policy
	for rows.Next() {
		p := &Policy{}
		if err := rows.Scan(&p.ID, &p.SubjectType, &p.Algorithm, &p.Capacity, &p.RefillRate, &p.WindowMs, &p.Action, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) RecordDecision(ctx context.Context, subject, policyID string, allowed bool, reason string, retryAfterMs int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_decisions (subject, policy_id, allowed, reason, retry_after_ms)
		VALUES ($1,$2,$3,$4,$5)`,
		subject, policyID, allowed, reason, retryAfterMs,
	)
	return err
}

func (s *Store) RecentDecisions(ctx context.Context, subject string, limit int) ([]Decision, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, subject, policy_id, allowed, COALESCE(reason, ''), COALESCE(retry_after_ms, 0), decided_at
		FROM audit_decisions
		WHERE subject=$1
		ORDER BY decided_at DESC
		LIMIT $2`, subject, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	decisions := make([]Decision, 0, limit)
	for rows.Next() {
		var d Decision
		if err := rows.Scan(&d.ID, &d.Subject, &d.PolicyID, &d.Allowed, &d.Reason, &d.RetryAfterMs, &d.DecidedAt); err != nil {
			return nil, err
		}
		decisions = append(decisions, d)
	}
	return decisions, rows.Err()
}

// Cache is a short-TTL in-process policy cache to avoid DB round-trips on every request.
// Policies are read on every /check call; caching is critical for latency.
// TTL is intentionally short (e.g. 10s) so policy updates propagate quickly after a PUT.
type Cache struct {
	mu      sync.RWMutex
	store   *Store
	ttl     time.Duration
	entries map[string]cacheEntry
}

type cacheEntry struct {
	policy    *Policy
	expiresAt time.Time
}

func NewCache(store *Store, ttl time.Duration) *Cache {
	return &Cache{
		store:   store,
		ttl:     ttl,
		entries: make(map[string]cacheEntry),
	}
}

// Get returns the cached policy if still fresh, otherwise fetches from the DB.
// Uses RLock for the hot path and promotes to Lock only on a miss to minimise contention.
func (c *Cache) Get(ctx context.Context, id string) (*Policy, error) {
	c.mu.RLock()
	e, ok := c.entries[id]
	c.mu.RUnlock()

	if ok && time.Now().Before(e.expiresAt) {
		return e.policy, nil
	}

	p, err := c.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.entries[id] = cacheEntry{policy: p, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()

	return p, nil
}

// Invalidate removes a policy from the cache (called after PUT /policies/:id).
func (c *Cache) Invalidate(id string) {
	c.mu.Lock()
	delete(c.entries, id)
	c.mu.Unlock()
}
