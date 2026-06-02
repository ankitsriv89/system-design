// Package store implements the gateway's KeyStore, RouteStore, and DecisionLog
// using PostgreSQL and Redis.
package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/ankitsriv89/07-api-gateway/gateway"
)

// PGStore implements KeyStore, RouteStore, and DecisionLog backed by PostgreSQL.
type PGStore struct {
	db *sql.DB
}

// NewPG opens and pings a PostgreSQL connection pool.
func NewPG(dsn string) (*PGStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open pg: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("store: ping pg: %w", err)
	}
	return &PGStore{db: db}, nil
}

// Close releases the connection pool.
func (s *PGStore) Close() error { return s.db.Close() }

// hashKey returns the SHA-256 hex of a raw API key value.
func hashKey(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// ---- KeyStore ------------------------------------------------------------

func (s *PGStore) CreateKey(ctx context.Context, k *gateway.APIKey) error {
	k.HashedKey = hashKey(k.HashedKey) // caller sets HashedKey to raw; we hash it
	scopes := strings.Join(k.Scopes, ",")
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO api_keys (id, owner, hashed_key, scopes, quota_per_min, active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		k.ID, k.Owner, k.HashedKey, scopes, k.QuotaPerMin, k.Active, k.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("store: create key: %w", err)
	}
	return nil
}

func (s *PGStore) GetKeyByID(ctx context.Context, id string) (*gateway.APIKey, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, owner, hashed_key, scopes, quota_per_min, active, created_at
		FROM api_keys WHERE id = $1`, id)
	return scanKey(row)
}

func (s *PGStore) Authenticate(ctx context.Context, raw string) (*gateway.APIKey, error) {
	hashed := hashKey(raw)
	row := s.db.QueryRowContext(ctx, `
		SELECT id, owner, hashed_key, scopes, quota_per_min, active, created_at
		FROM api_keys WHERE hashed_key = $1`, hashed)
	return scanKey(row)
}

func (s *PGStore) ListKeys(ctx context.Context) ([]*gateway.APIKey, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, owner, hashed_key, scopes, quota_per_min, active, created_at
		FROM api_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: list keys: %w", err)
	}
	defer rows.Close()
	return scanKeys(rows)
}

func (s *PGStore) RevokeKey(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE api_keys SET active = false WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("store: revoke key: %w", err)
	}
	return nil
}

func scanKey(row *sql.Row) (*gateway.APIKey, error) {
	var k gateway.APIKey
	var scopes string
	err := row.Scan(&k.ID, &k.Owner, &k.HashedKey, &scopes, &k.QuotaPerMin, &k.Active, &k.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, gateway.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan key: %w", err)
	}
	if scopes != "" {
		k.Scopes = strings.Split(scopes, ",")
	}
	return &k, nil
}

func scanKeys(rows *sql.Rows) ([]*gateway.APIKey, error) {
	var out []*gateway.APIKey
	for rows.Next() {
		var k gateway.APIKey
		var scopes string
		if err := rows.Scan(&k.ID, &k.Owner, &k.HashedKey, &scopes, &k.QuotaPerMin, &k.Active, &k.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: scan keys: %w", err)
		}
		if scopes != "" {
			k.Scopes = strings.Split(scopes, ",")
		}
		out = append(out, &k)
	}
	return out, rows.Err()
}

// ---- RouteStore ----------------------------------------------------------

func (s *PGStore) UpsertRoute(ctx context.Context, r *gateway.Route) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO routes (id, path_prefix, upstream, strip_prefix, auth_required, required_scope, max_body_bytes, timeout_secs, active, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (id) DO UPDATE SET
			path_prefix    = EXCLUDED.path_prefix,
			upstream       = EXCLUDED.upstream,
			strip_prefix   = EXCLUDED.strip_prefix,
			auth_required  = EXCLUDED.auth_required,
			required_scope = EXCLUDED.required_scope,
			max_body_bytes = EXCLUDED.max_body_bytes,
			timeout_secs   = EXCLUDED.timeout_secs,
			active         = EXCLUDED.active,
			updated_at     = EXCLUDED.updated_at`,
		r.ID, r.PathPrefix, r.Upstream, r.StripPrefix, r.AuthRequired,
		r.RequiredScope, r.MaxBodyBytes, r.TimeoutSecs, r.Active, r.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("store: upsert route: %w", err)
	}
	return nil
}

func (s *PGStore) GetRoute(ctx context.Context, id string) (*gateway.Route, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, path_prefix, upstream, strip_prefix, auth_required, required_scope, max_body_bytes, timeout_secs, active, updated_at
		FROM routes WHERE id = $1`, id)
	return scanRoute(row)
}

func (s *PGStore) ListRoutes(ctx context.Context) ([]*gateway.Route, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, path_prefix, upstream, strip_prefix, auth_required, required_scope, max_body_bytes, timeout_secs, active, updated_at
		FROM routes ORDER BY path_prefix`)
	if err != nil {
		return nil, fmt.Errorf("store: list routes: %w", err)
	}
	defer rows.Close()
	var out []*gateway.Route
	for rows.Next() {
		r, err := scanRouteRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *PGStore) DeleteRoute(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM routes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("store: delete route: %w", err)
	}
	return nil
}

func scanRoute(row *sql.Row) (*gateway.Route, error) {
	var r gateway.Route
	err := row.Scan(&r.ID, &r.PathPrefix, &r.Upstream, &r.StripPrefix, &r.AuthRequired,
		&r.RequiredScope, &r.MaxBodyBytes, &r.TimeoutSecs, &r.Active, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, gateway.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan route: %w", err)
	}
	return &r, nil
}

func scanRouteRow(rows *sql.Rows) (*gateway.Route, error) {
	var r gateway.Route
	err := rows.Scan(&r.ID, &r.PathPrefix, &r.Upstream, &r.StripPrefix, &r.AuthRequired,
		&r.RequiredScope, &r.MaxBodyBytes, &r.TimeoutSecs, &r.Active, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("store: scan route row: %w", err)
	}
	return &r, nil
}

// ---- DecisionLog ---------------------------------------------------------

func (s *PGStore) Record(ctx context.Context, d *gateway.Decision) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO gateway_decisions (request_id, route_id, key_id, outcome, status_code, latency_ms, recorded_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		d.RequestID, d.RouteID, d.KeyID, d.Outcome, d.StatusCode, d.LatencyMs, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("store: record decision: %w", err)
	}
	return nil
}
