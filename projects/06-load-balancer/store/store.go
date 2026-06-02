// Package store persists backend configuration and health history to PostgreSQL.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// Backend is the DB row representation of a backend.
type Backend struct {
	ID        int
	Service   string
	URL       string
	Weight    int
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Store handles all database interactions.
type Store struct {
	db  *sql.DB
	log *zap.Logger
}

// New opens a connection pool. DSN should be a libpq-format connection string.
func New(dsn string, log *zap.Logger) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{db: db, log: log}, nil
}

// Close releases the connection pool.
func (s *Store) Close() error {
	return s.db.Close()
}

// UpsertBackend inserts or updates a backend record.
func (s *Store) UpsertBackend(ctx context.Context, service, url string, weight int, status string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO backends (service, url, weight, status, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (service, url)
		DO UPDATE SET weight = EXCLUDED.weight,
		              status = EXCLUDED.status,
		              updated_at = NOW()
	`, service, url, weight, status)
	if err != nil {
		return fmt.Errorf("store: upsert backend: %w", err)
	}
	return nil
}

// DeleteBackend removes a backend record.
func (s *Store) DeleteBackend(ctx context.Context, service, url string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM backends WHERE service = $1 AND url = $2`,
		service, url,
	)
	if err != nil {
		return fmt.Errorf("store: delete backend: %w", err)
	}
	return nil
}

// ListBackends returns all backends, optionally filtered by service.
func (s *Store) ListBackends(ctx context.Context, service string) ([]Backend, error) {
	var rows *sql.Rows
	var err error
	if service == "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, service, url, weight, status, created_at, updated_at FROM backends ORDER BY service, id`)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, service, url, weight, status, created_at, updated_at FROM backends WHERE service = $1 ORDER BY id`,
			service)
	}
	if err != nil {
		return nil, fmt.Errorf("store: list backends: %w", err)
	}
	defer rows.Close()

	var out []Backend
	for rows.Next() {
		var b Backend
		if err := rows.Scan(&b.ID, &b.Service, &b.URL, &b.Weight, &b.Status, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: scan backend: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateBackendStatus records a new status for a backend.
func (s *Store) UpdateBackendStatus(ctx context.Context, service, url, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE backends SET status = $1, updated_at = NOW() WHERE service = $2 AND url = $3`,
		status, service, url,
	)
	return err
}

// RecordHealthEvent inserts a health check observation.
func (s *Store) RecordHealthEvent(ctx context.Context, service, backendURL, status string, latencyMs int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO health_events (service, backend_url, status, latency_ms) VALUES ($1, $2, $3, $4)`,
		service, backendURL, status, latencyMs,
	)
	if err != nil {
		return fmt.Errorf("store: record health event: %w", err)
	}
	return nil
}

// HealthHistory returns the most recent health events for a service.
func (s *Store) HealthHistory(ctx context.Context, service string, limit int) ([]HealthRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT backend_url, status, latency_ms, recorded_at
		FROM health_events
		WHERE service = $1
		ORDER BY recorded_at DESC
		LIMIT $2
	`, service, limit)
	if err != nil {
		return nil, fmt.Errorf("store: health history: %w", err)
	}
	defer rows.Close()

	var out []HealthRow
	for rows.Next() {
		var h HealthRow
		if err := rows.Scan(&h.BackendURL, &h.Status, &h.LatencyMs, &h.RecordedAt); err != nil {
			return nil, fmt.Errorf("store: scan health row: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// HealthRow is one row from health_events.
type HealthRow struct {
	BackendURL string
	Status     string
	LatencyMs  int
	RecordedAt time.Time
}
