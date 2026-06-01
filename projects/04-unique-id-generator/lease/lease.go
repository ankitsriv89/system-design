// Package lease manages PostgreSQL-backed worker ID leases.
//
// Each running generator instance holds an exclusive lease on a worker_id
// (0–1023). The lease has a TTL; the holder renews it on a background ticker
// before it expires. If a process crashes or fails to renew, the lease expires
// naturally and another instance can claim that worker_id.
//
// Split-brain prevention: worker_id assignment uses SELECT ... FOR UPDATE SKIP LOCKED
// so two concurrent instances can never claim the same ID in a race.
//
// On startup the caller should call Acquire, then pass the returned worker_id
// to generator.New. On shutdown call Release to free the slot immediately rather
// than waiting for the TTL.
package lease

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// pq registers the "postgres" driver as a side-effect.
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

const (
	// TTL is how long a lease is valid without renewal.
	TTL = 30 * time.Second
	// RenewInterval is how often the background renewer fires; must be < TTL/2.
	RenewInterval = 10 * time.Second
)

// Manager acquires, renews, and releases worker ID leases backed by PostgreSQL.
type Manager struct {
	db       *sql.DB
	log      *zap.Logger
	workerID int64
	region   string
}

// New creates a Manager connected to the given PostgreSQL DSN.
func New(dsn, region string, log *zap.Logger) (*Manager, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("lease: open db: %w", err)
	}
	// The lease manager only ever needs two concurrent connections: one for
	// the renew ticker and one for ad-hoc calls (Acquire, Release, RecordClockIncident).
	// Idle == Open so the pool never destroys and recreates connections between
	// the infrequent 10-second renew ticks, avoiding reconnect overhead on a
	// shared host where many services compete for file descriptors.
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(10 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	return &Manager{db: db, log: log, region: region}, nil
}

// Ping checks that the database connection is alive.
func (m *Manager) Ping(ctx context.Context) error {
	return m.db.PingContext(ctx)
}

// Acquire claims any available worker_id and returns it.
// It blocks until a worker_id is available or the context is cancelled.
// The caller must call StartRenewer to keep the lease alive.
func (m *Manager) Acquire(ctx context.Context) (int64, error) {
	// Expire stale leases first so slots freed by crashed workers become available.
	if _, err := m.db.ExecContext(ctx,
		`UPDATE worker_leases SET holder = '', expires_at = NOW() - INTERVAL '1 second'
		 WHERE expires_at < NOW() AND holder != ''`); err != nil {
		m.log.Warn("lease expiry cleanup failed", zap.Error(err))
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("lease: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var workerID int64
	err = tx.QueryRowContext(ctx, `
		SELECT worker_id FROM worker_leases
		WHERE expires_at < NOW() OR holder = ''
		ORDER BY worker_id
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`).Scan(&workerID)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("lease: no available worker_id slots (all 1024 are active)")
	}
	if err != nil {
		return 0, fmt.Errorf("lease: select worker_id: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE worker_leases
		SET holder = $1, region = $2, expires_at = NOW() + $3::interval
		WHERE worker_id = $4
	`, fmt.Sprintf("pid-%d", workerID), m.region, TTL.String(), workerID)
	if err != nil {
		return 0, fmt.Errorf("lease: update worker_id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("lease: commit: %w", err)
	}

	m.workerID = workerID
	m.log.Info("worker lease acquired", zap.Int64("worker_id", workerID), zap.String("region", m.region))
	return workerID, nil
}

// StartRenewer launches a background goroutine that renews the lease every
// RenewInterval. It stops when ctx is cancelled. Call this after Acquire.
func (m *Manager) StartRenewer(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(RenewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := m.renew(ctx); err != nil {
					m.log.Error("lease renewal failed", zap.Error(err), zap.Int64("worker_id", m.workerID))
				}
			}
		}
	}()
}

// renew extends the lease TTL from now.
func (m *Manager) renew(ctx context.Context) error {
	res, err := m.db.ExecContext(ctx, `
		UPDATE worker_leases
		SET expires_at = NOW() + $1::interval
		WHERE worker_id = $2 AND expires_at >= NOW()
	`, TTL.String(), m.workerID)
	if err != nil {
		return fmt.Errorf("lease renew exec: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		// Our lease expired before we could renew — another instance may have
		// taken our worker_id. This is a critical fault; the caller should restart.
		return fmt.Errorf("lease for worker_id %d expired before renewal", m.workerID)
	}
	m.log.Debug("lease renewed", zap.Int64("worker_id", m.workerID))
	return nil
}

// Release frees the worker_id slot immediately instead of waiting for TTL expiry.
// Call this on graceful shutdown.
func (m *Manager) Release(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, `
		UPDATE worker_leases SET holder = '', expires_at = NOW() - INTERVAL '1 second'
		WHERE worker_id = $1
	`, m.workerID)
	if err != nil {
		return fmt.Errorf("lease release: %w", err)
	}
	m.log.Info("worker lease released", zap.Int64("worker_id", m.workerID))
	return nil
}

// RecordClockIncident persists a clock rollback event for audit and alerting.
func (m *Manager) RecordClockIncident(ctx context.Context, driftMs int64) {
	_, err := m.db.ExecContext(ctx, `
		INSERT INTO clock_incidents (worker_id, drift_ms, detected_at)
		VALUES ($1, $2, NOW())
	`, m.workerID, driftMs)
	if err != nil {
		m.log.Error("failed to record clock incident", zap.Error(err), zap.Int64("drift_ms", driftMs))
	}
}

// Close shuts down the underlying database connection pool.
func (m *Manager) Close() error {
	return m.db.Close()
}
