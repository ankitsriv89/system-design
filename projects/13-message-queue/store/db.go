// Package store implements PostgreSQL persistence for topics, messages,
// consumer offsets, and the dead-letter queue.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/ankitsriv89/13-message-queue/queue"
)

// DB wraps a *sql.DB and exposes message-queue persistence operations.
type DB struct {
	db *sql.DB
}

// NewDB opens a PostgreSQL connection pool and verifies connectivity.
func NewDB(dsn string) (*DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &DB{db: db}, nil
}

// Close releases all DB connections.
func (s *DB) Close() error { return s.db.Close() }

// CreateTopic inserts a new topic; returns ErrTopicExists if duplicate.
func (s *DB) CreateTopic(ctx context.Context, t *queue.Topic) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO topics (name, partitions, retention_seconds, created_at)
		 VALUES ($1, $2, $3, $4)`,
		t.Name, t.Partitions, int64(t.RetentionPeriod.Seconds()), t.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return queue.ErrTopicExists
		}
		return fmt.Errorf("store: create topic: %w", err)
	}
	return nil
}

// GetTopic fetches a topic by name.
func (s *DB) GetTopic(ctx context.Context, name string) (*queue.Topic, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT name, partitions, retention_seconds, created_at FROM topics WHERE name = $1`, name)
	var t queue.Topic
	var retSecs int64
	if err := row.Scan(&t.Name, &t.Partitions, &retSecs, &t.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, queue.ErrTopicNotFound
		}
		return nil, fmt.Errorf("store: get topic: %w", err)
	}
	t.RetentionPeriod = time.Duration(retSecs) * time.Second
	return &t, nil
}

// ListTopics returns all topics.
func (s *DB) ListTopics(ctx context.Context) ([]*queue.Topic, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, partitions, retention_seconds, created_at FROM topics ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("store: list topics: %w", err)
	}
	defer rows.Close()

	topics := make([]*queue.Topic, 0, 16)
	for rows.Next() {
		var t queue.Topic
		var retSecs int64
		if err := rows.Scan(&t.Name, &t.Partitions, &retSecs, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: list topics scan: %w", err)
		}
		t.RetentionPeriod = time.Duration(retSecs) * time.Second
		topics = append(topics, &t)
	}
	return topics, rows.Err()
}

// PublishMessage inserts a message at the next offset for its partition.
// Returns the assigned offset.
func (s *DB) PublishMessage(ctx context.Context, m *queue.Message) (int64, error) {
	var offset int64
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO messages
		   (id, topic, partition, key, payload, published_at, visible_at, delivery_attempts, dead_lettered)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,0,false)
		 RETURNING offset`,
		m.ID, m.Topic, m.Partition, m.Key, m.Payload, m.PublishedAt, m.VisibleAt,
	).Scan(&offset)
	if err != nil {
		return 0, fmt.Errorf("store: publish message: %w", err)
	}
	return offset, nil
}

// PollMessages fetches up to maxN invisible messages for a consumer group,
// marks them invisible for visibilityTimeout, and returns them.
// Ordering is by offset within each partition (per-partition FIFO).
func (s *DB) PollMessages(ctx context.Context, req *queue.PollRequest) ([]*queue.Message, error) {
	now := time.Now().UTC()
	newVisibleAt := now.Add(req.VisibilityTimeout)

	var rows *sql.Rows
	var err error

	if req.Partition >= 0 {
		rows, err = s.db.QueryContext(ctx,
			`WITH candidates AS (
			   SELECT id FROM messages
			   WHERE topic = $1
			     AND partition = $2
			     AND dead_lettered = false
			     AND acked_at IS NULL
			     AND visible_at <= $3
			     AND (consumer_group IS NULL OR consumer_group = $4)
			   ORDER BY partition, offset
			   LIMIT $5
			   FOR UPDATE SKIP LOCKED
			 )
			 UPDATE messages SET visible_at = $6, consumer_group = $4,
			        delivery_attempts = delivery_attempts + 1
			 FROM candidates
			 WHERE messages.id = candidates.id
			 RETURNING messages.id, messages.topic, messages.partition, messages.offset,
			           messages.key, messages.payload, messages.published_at,
			           messages.visible_at, messages.delivery_attempts, messages.consumer_group`,
			req.Topic, req.Partition, now, req.ConsumerGroup, req.MaxMessages, newVisibleAt,
		)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`WITH candidates AS (
			   SELECT id FROM messages
			   WHERE topic = $1
			     AND dead_lettered = false
			     AND acked_at IS NULL
			     AND visible_at <= $2
			     AND (consumer_group IS NULL OR consumer_group = $3)
			   ORDER BY partition, offset
			   LIMIT $4
			   FOR UPDATE SKIP LOCKED
			 )
			 UPDATE messages SET visible_at = $5, consumer_group = $3,
			        delivery_attempts = delivery_attempts + 1
			 FROM candidates
			 WHERE messages.id = candidates.id
			 RETURNING messages.id, messages.topic, messages.partition, messages.offset,
			           messages.key, messages.payload, messages.published_at,
			           messages.visible_at, messages.delivery_attempts, messages.consumer_group`,
			req.Topic, now, req.ConsumerGroup, req.MaxMessages, newVisibleAt,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("store: poll: %w", err)
	}
	defer rows.Close()

	msgs := make([]*queue.Message, 0, req.MaxMessages)
	for rows.Next() {
		m := &queue.Message{}
		var cg sql.NullString
		if err := rows.Scan(&m.ID, &m.Topic, &m.Partition, &m.Offset,
			&m.Key, &m.Payload, &m.PublishedAt, &m.VisibleAt, &m.DeliveryAttempts, &cg); err != nil {
			return nil, fmt.Errorf("store: poll scan: %w", err)
		}
		if cg.Valid {
			m.ConsumerGroup = cg.String
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// AckMessage marks a message as acknowledged, preventing re-delivery.
func (s *DB) AckMessage(ctx context.Context, req *queue.AckRequest) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE messages SET acked_at = $1
		 WHERE id = $2 AND consumer_group = $3 AND acked_at IS NULL`,
		now, req.MessageID, req.ConsumerGroup,
	)
	if err != nil {
		return fmt.Errorf("store: ack: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return queue.ErrMessageNotFound
	}
	return nil
}

// MoveExpiredToDeadLetter moves messages that have exceeded MaxDeliveryAttempts
// and whose visibility timeout has expired into the DLQ. Returns count moved.
func (s *DB) MoveExpiredToDeadLetter(ctx context.Context) (int64, error) {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE messages
		 SET dead_lettered = true
		 WHERE dead_lettered = false
		   AND acked_at IS NULL
		   AND visible_at <= $1
		   AND delivery_attempts >= $2`,
		now, queue.MaxDeliveryAttempts,
	)
	if err != nil {
		return 0, fmt.Errorf("store: dlq sweep: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// RestoreExpiredMessages makes messages whose visibility timeout has passed
// (but which have not exceeded MaxDeliveryAttempts) visible again.
// Returns count restored.
func (s *DB) RestoreExpiredMessages(ctx context.Context) (int64, error) {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE messages
		 SET visible_at = $1, consumer_group = NULL
		 WHERE dead_lettered = false
		   AND acked_at IS NULL
		   AND visible_at <= $1
		   AND delivery_attempts < $2`,
		now, queue.MaxDeliveryAttempts,
	)
	if err != nil {
		return 0, fmt.Errorf("store: restore expired: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// GetQueueDepth returns the number of unacked, non-DLQ messages per partition.
func (s *DB) GetQueueDepth(ctx context.Context, topic string) (map[int]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT partition, COUNT(*) FROM messages
		 WHERE topic = $1 AND acked_at IS NULL AND dead_lettered = false
		 GROUP BY partition`,
		topic,
	)
	if err != nil {
		return nil, fmt.Errorf("store: queue depth: %w", err)
	}
	defer rows.Close()

	depth := make(map[int]int64, 8)
	for rows.Next() {
		var p int
		var n int64
		if err := rows.Scan(&p, &n); err != nil {
			return nil, fmt.Errorf("store: queue depth scan: %w", err)
		}
		depth[p] = n
	}
	return depth, rows.Err()
}

// GetDLQDepth returns the number of dead-lettered messages for a topic.
func (s *DB) GetDLQDepth(ctx context.Context, topic string) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM messages WHERE topic = $1 AND dead_lettered = true`, topic,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("store: dlq depth: %w", err)
	}
	return n, nil
}

// ListDLQMessages returns up to limit dead-lettered messages for a topic.
func (s *DB) ListDLQMessages(ctx context.Context, topic string, limit int) ([]*queue.Message, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, topic, partition, offset, key, payload, published_at, delivery_attempts
		 FROM messages
		 WHERE topic = $1 AND dead_lettered = true
		 ORDER BY offset DESC
		 LIMIT $2`,
		topic, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("store: list dlq: %w", err)
	}
	defer rows.Close()

	msgs := make([]*queue.Message, 0, limit)
	for rows.Next() {
		m := &queue.Message{DeadLettered: true}
		if err := rows.Scan(&m.ID, &m.Topic, &m.Partition, &m.Offset,
			&m.Key, &m.Payload, &m.PublishedAt, &m.DeliveryAttempts); err != nil {
			return nil, fmt.Errorf("store: list dlq scan: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// GetStats returns aggregate stats across all topics.
func (s *DB) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{}, 8)

	var total, acked, dlq, inflight int64
	err := s.db.QueryRowContext(ctx,
		`SELECT
		   COUNT(*) FILTER (WHERE true) AS total,
		   COUNT(*) FILTER (WHERE acked_at IS NOT NULL) AS acked,
		   COUNT(*) FILTER (WHERE dead_lettered = true) AS dlq,
		   COUNT(*) FILTER (WHERE acked_at IS NULL AND dead_lettered = false AND visible_at > NOW()) AS inflight
		 FROM messages`,
	).Scan(&total, &acked, &dlq, &inflight)
	if err != nil {
		return nil, fmt.Errorf("store: stats: %w", err)
	}
	stats["total_messages"] = total
	stats["acked_messages"] = acked
	stats["dlq_messages"] = dlq
	stats["inflight_messages"] = inflight
	stats["pending_messages"] = total - acked - dlq - inflight
	return stats, nil
}

// isUniqueViolation detects PostgreSQL unique-constraint errors (code 23505).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// lib/pq exposes the code in the error string; check via type assertion.
	type pgErr interface{ Error() string }
	return len(err.Error()) > 0 &&
		containsCode(err.Error(), "23505")
}

func containsCode(s, code string) bool {
	for i := 0; i+len(code) <= len(s); i++ {
		if s[i:i+len(code)] == code {
			return true
		}
	}
	return false
}
