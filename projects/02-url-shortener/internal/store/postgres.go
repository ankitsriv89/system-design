package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/ankitsriv89/url-shortener/internal/analytics"
	"github.com/ankitsriv89/url-shortener/internal/link"
	"github.com/ankitsriv89/url-shortener/internal/owner"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS owners (
	id TEXT PRIMARY KEY,
	quota INTEGER NOT NULL CHECK (quota >= 0),
	plan TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS links (
	code TEXT PRIMARY KEY,
	long_url TEXT NOT NULL,
	owner_id TEXT NOT NULL REFERENCES owners(id),
	expires_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	disabled_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_links_owner ON links(owner_id);
CREATE INDEX IF NOT EXISTS idx_links_owner_active ON links(owner_id) WHERE disabled_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_links_expires ON links(expires_at);

CREATE TABLE IF NOT EXISTS click_events (
	id BIGSERIAL PRIMARY KEY,
	code TEXT NOT NULL REFERENCES links(code),
	clicked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	referrer TEXT NOT NULL DEFAULT '',
	user_agent TEXT NOT NULL DEFAULT '',
	ip_hash TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_click_events_code_time ON click_events(code, clicked_at DESC);
CREATE INDEX IF NOT EXISTS idx_click_events_time ON click_events(clicked_at DESC);
`)
	return err
}

func (s *PostgresStore) SeedOwners(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO owners(id, quota, plan) VALUES
	('demo', 100, 'demo'),
	('marketing', 1000, 'team'),
	('creator', 250, 'creator')
ON CONFLICT (id) DO UPDATE SET quota = EXCLUDED.quota, plan = EXCLUDED.plan;
`)
	return err
}

func (s *PostgresStore) GetOwner(ctx context.Context, id string) (owner.Owner, error) {
	var o owner.Owner
	err := s.db.QueryRowContext(ctx, `SELECT id, quota, plan, created_at FROM owners WHERE id = $1`, id).
		Scan(&o.ID, &o.Quota, &o.Plan, &o.CreatedAt)
	return o, err
}

func (s *PostgresStore) ActiveLinkCount(ctx context.Context, ownerID string, now time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
SELECT count(*)
FROM links
WHERE owner_id = $1
	AND disabled_at IS NULL
	AND (expires_at IS NULL OR expires_at > $2)
`, ownerID, now).Scan(&count)
	return count, err
}

func (s *PostgresStore) CreateLink(ctx context.Context, l link.Link) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO links(code, long_url, owner_id, expires_at)
VALUES ($1, $2, $3, $4)
`, l.Code, l.LongURL, l.OwnerID, l.ExpiresAt)
	if isUniqueViolation(err) {
		return link.ErrCollision
	}
	return err
}

func (s *PostgresStore) GetLink(ctx context.Context, code string) (link.Link, error) {
	var l link.Link
	err := s.db.QueryRowContext(ctx, `
SELECT code, long_url, owner_id, expires_at, created_at, disabled_at
FROM links
WHERE code = $1
`, code).Scan(&l.Code, &l.LongURL, &l.OwnerID, &l.ExpiresAt, &l.CreatedAt, &l.DisabledAt)
	if errors.Is(err, sql.ErrNoRows) {
		return l, link.ErrNotFound
	}
	return l, err
}

func (s *PostgresStore) RecordClick(ctx context.Context, code, referrer, userAgent, remoteAddr string) error {
	ipHash := hashIP(remoteAddr)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO click_events(code, referrer, user_agent, ip_hash)
VALUES ($1, $2, $3, $4)
`, code, trim(referrer, 500), trim(userAgent, 500), ipHash)
	return err
}

func (s *PostgresStore) Stats(ctx context.Context, code string) (analytics.Stats, error) {
	l, err := s.GetLink(ctx, code)
	if err != nil {
		return analytics.Stats{}, err
	}

	stats := analytics.Stats{
		Code:      l.Code,
		LongURL:   l.LongURL,
		OwnerID:   l.OwnerID,
		CreatedAt: l.CreatedAt,
		ExpiresAt: l.ExpiresAt,
	}
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM click_events WHERE code = $1`, code).Scan(&stats.TotalClicks); err != nil {
		return analytics.Stats{}, err
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, code, clicked_at, referrer, user_agent, ip_hash
FROM click_events
WHERE code = $1
ORDER BY clicked_at DESC
LIMIT 25
`, code)
	if err != nil {
		return analytics.Stats{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var e analytics.ClickEvent
		if err := rows.Scan(&e.ID, &e.Code, &e.ClickedAt, &e.Referrer, &e.UserAgent, &e.IPHash); err != nil {
			return analytics.Stats{}, err
		}
		stats.RecentClicks = append(stats.RecentClicks, e)
	}
	return stats, rows.Err()
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

func hashIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	sum := sha256.Sum256([]byte(host))
	return hex.EncodeToString(sum[:])[:16]
}

func trim(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max]
}
