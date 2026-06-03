// Package store implements PostgreSQL-backed persistence for the notification system.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/ankitsriv89/10-notification-system/notification"
)

// Store wraps a *sql.DB and provides all persistence operations.
type Store struct {
	db *sql.DB
}

// New opens and pings the database.
func New(dsn string) (*Store, error) {
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
	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// ── Notifications ─────────────────────────────────────────────────────────────

// CreateNotification inserts a new notification and returns it with its generated ID.
func (s *Store) CreateNotification(ctx context.Context, n *notification.Notification) error {
	params, err := json.Marshal(n.Params)
	if err != nil {
		return fmt.Errorf("store: marshal params: %w", err)
	}
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO notifications
			(user_id, channel, template_id, params, subject, body, priority, status, idempotency_key, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW(),NOW())
		ON CONFLICT (idempotency_key) DO UPDATE SET updated_at = NOW()
		RETURNING id, created_at, updated_at`,
		n.UserID, n.Channel, n.TemplateID, params, n.Subject, n.Body,
		int(n.Priority), string(n.Status), nullableString(n.IdempotencyKey),
	)
	return row.Scan(&n.ID, &n.CreatedAt, &n.UpdatedAt)
}

// GetNotification retrieves a single notification by ID.
func (s *Store) GetNotification(ctx context.Context, id string) (*notification.Notification, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, channel, template_id, params, subject, body, priority, status,
		       COALESCE(idempotency_key,''), created_at, updated_at
		FROM notifications WHERE id = $1`, id)
	return scanNotification(row)
}

// UpdateStatus changes a notification's status.
func (s *Store) UpdateStatus(ctx context.Context, id string, status notification.Status) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE notifications SET status=$1, updated_at=NOW() WHERE id=$2`,
		string(status), id)
	if err != nil {
		return fmt.Errorf("store: update status: %w", err)
	}
	return nil
}

// ListNotifications returns notifications with optional filters, newest first.
func (s *Store) ListNotifications(ctx context.Context, userID string, limit, offset int) ([]*notification.Notification, error) {
	query := `
		SELECT id, user_id, channel, template_id, params, subject, body, priority, status,
		       COALESCE(idempotency_key,''), created_at, updated_at
		FROM notifications`
	args := []interface{}{}
	if userID != "" {
		query += " WHERE user_id = $1"
		args = append(args, userID)
		query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	} else {
		query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	}
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list notifications: %w", err)
	}
	defer rows.Close()
	return scanNotifications(rows)
}

// CountNotificationsByStatus returns counts grouped by status.
func (s *Store) CountNotificationsByStatus(ctx context.Context) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM notifications GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("store: count by status: %w", err)
	}
	defer rows.Close()
	result := make(map[string]int64, 6)
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		result[status] = count
	}
	return result, rows.Err()
}

// ── Preferences ───────────────────────────────────────────────────────────────

// UpsertPreference inserts or replaces a user preference.
func (s *Store) UpsertPreference(ctx context.Context, p *notification.Preference) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO preferences (user_id, channel, enabled, quiet_start, quiet_end, updated_at)
		VALUES ($1,$2,$3,$4,$5,NOW())
		ON CONFLICT (user_id, channel) DO UPDATE
		SET enabled=$3, quiet_start=$4, quiet_end=$5, updated_at=NOW()`,
		p.UserID, string(p.Channel), p.Enabled, p.QuietStart, p.QuietEnd)
	if err != nil {
		return fmt.Errorf("store: upsert preference: %w", err)
	}
	return nil
}

// GetPreference retrieves a preference; returns nil if not found.
func (s *Store) GetPreference(ctx context.Context, userID string, ch notification.Channel) (*notification.Preference, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT user_id, channel, enabled, quiet_start, quiet_end, updated_at
		FROM preferences WHERE user_id=$1 AND channel=$2`, userID, string(ch))
	p := &notification.Preference{}
	err := row.Scan(&p.UserID, &p.Channel, &p.Enabled, &p.QuietStart, &p.QuietEnd, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get preference: %w", err)
	}
	return p, nil
}

// ListPreferences returns all preferences for a user.
func (s *Store) ListPreferences(ctx context.Context, userID string) ([]*notification.Preference, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, channel, enabled, quiet_start, quiet_end, updated_at
		FROM preferences WHERE user_id=$1`, userID)
	if err != nil {
		return nil, fmt.Errorf("store: list preferences: %w", err)
	}
	defer rows.Close()
	var prefs []*notification.Preference
	for rows.Next() {
		p := &notification.Preference{}
		if err := rows.Scan(&p.UserID, &p.Channel, &p.Enabled, &p.QuietStart, &p.QuietEnd, &p.UpdatedAt); err != nil {
			return nil, err
		}
		prefs = append(prefs, p)
	}
	return prefs, rows.Err()
}

// ── Templates ─────────────────────────────────────────────────────────────────

// UpsertTemplate inserts or replaces a template.
func (s *Store) UpsertTemplate(ctx context.Context, t *notification.Template) error {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO templates (id, channel, subject, body, created_at, updated_at)
		VALUES ($1,$2,$3,$4,NOW(),NOW())
		ON CONFLICT (id) DO UPDATE
		SET channel=$2, subject=$3, body=$4, updated_at=NOW()
		RETURNING created_at, updated_at`,
		t.ID, string(t.Channel), t.Subject, t.Body)
	return row.Scan(&t.CreatedAt, &t.UpdatedAt)
}

// GetTemplate retrieves a template by ID.
func (s *Store) GetTemplate(ctx context.Context, id string) (*notification.Template, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, channel, subject, body, created_at, updated_at
		FROM templates WHERE id=$1`, id)
	t := &notification.Template{}
	err := row.Scan(&t.ID, &t.Channel, &t.Subject, &t.Body, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: get template: %w", err)
	}
	return t, nil
}

// ListTemplates returns all templates.
func (s *Store) ListTemplates(ctx context.Context) ([]*notification.Template, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, channel, subject, body, created_at, updated_at
		FROM templates ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: list templates: %w", err)
	}
	defer rows.Close()
	var templates []*notification.Template
	for rows.Next() {
		t := &notification.Template{}
		if err := rows.Scan(&t.ID, &t.Channel, &t.Subject, &t.Body, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

// ── Delivery Attempts ─────────────────────────────────────────────────────────

// InsertAttempt records a delivery attempt.
func (s *Store) InsertAttempt(ctx context.Context, a *notification.DeliveryAttempt) error {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO delivery_attempts
			(notification_id, provider, attempt_number, status, error_msg, latency_ms, attempted_at)
		VALUES ($1,$2,$3,$4,$5,$6,NOW())
		RETURNING id, attempted_at`,
		a.NotificationID, a.Provider, a.AttemptNumber,
		string(a.Status), a.ErrorMsg, a.LatencyMs)
	return row.Scan(&a.ID, &a.AttemptedAt)
}

// ListAttempts returns all delivery attempts for a notification.
func (s *Store) ListAttempts(ctx context.Context, notificationID string) ([]*notification.DeliveryAttempt, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, notification_id, provider, attempt_number, status, COALESCE(error_msg,''), latency_ms, attempted_at
		FROM delivery_attempts WHERE notification_id=$1 ORDER BY attempt_number`, notificationID)
	if err != nil {
		return nil, fmt.Errorf("store: list attempts: %w", err)
	}
	defer rows.Close()
	var attempts []*notification.DeliveryAttempt
	for rows.Next() {
		a := &notification.DeliveryAttempt{}
		if err := rows.Scan(&a.ID, &a.NotificationID, &a.Provider,
			&a.AttemptNumber, &a.Status, &a.ErrorMsg, &a.LatencyMs, &a.AttemptedAt); err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}

// CountRecentAttempts returns the number of attempts in the last N minutes — used for rate-limit checks.
func (s *Store) CountRecentAttempts(ctx context.Context, provider string, windowMinutes int) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM delivery_attempts
		WHERE provider=$1 AND attempted_at > NOW() - ($2 || ' minutes')::interval`,
		provider, windowMinutes).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("store: count recent attempts: %w", err)
	}
	return count, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanNotification(row rowScanner) (*notification.Notification, error) {
	n := &notification.Notification{}
	var paramsJSON []byte
	var ch, status string
	err := row.Scan(&n.ID, &n.UserID, &ch, &n.TemplateID, &paramsJSON,
		&n.Subject, &n.Body, &n.Priority, &status, &n.IdempotencyKey,
		&n.CreatedAt, &n.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: scan notification: %w", err)
	}
	n.Channel = notification.Channel(ch)
	n.Status = notification.Status(status)
	if len(paramsJSON) > 0 {
		if err := json.Unmarshal(paramsJSON, &n.Params); err != nil {
			return nil, fmt.Errorf("store: unmarshal params: %w", err)
		}
	}
	return n, nil
}

func scanNotifications(rows *sql.Rows) ([]*notification.Notification, error) {
	var results []*notification.Notification
	for rows.Next() {
		n := &notification.Notification{}
		var paramsJSON []byte
		var ch, status string
		err := rows.Scan(&n.ID, &n.UserID, &ch, &n.TemplateID, &paramsJSON,
			&n.Subject, &n.Body, &n.Priority, &status, &n.IdempotencyKey,
			&n.CreatedAt, &n.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("store: scan notifications row: %w", err)
		}
		n.Channel = notification.Channel(ch)
		n.Status = notification.Status(status)
		if len(paramsJSON) > 0 {
			if err := json.Unmarshal(paramsJSON, &n.Params); err != nil {
				return nil, err
			}
		}
		results = append(results, n)
	}
	return results, rows.Err()
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
