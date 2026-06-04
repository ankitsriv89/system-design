// Package worker runs background goroutines that maintain queue health:
// restoring expired leases and moving poison messages to the DLQ.
package worker

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/ankitsriv89/13-message-queue/metrics"
	"github.com/ankitsriv89/13-message-queue/store"
)

// SweepInterval controls how frequently the reaper runs.
const SweepInterval = 5 * time.Second

// Reaper periodically restores messages with expired visibility timeouts and
// moves messages that have exceeded MaxDeliveryAttempts to the DLQ.
type Reaper struct {
	db  *store.DB
	log *zap.Logger
}

// New constructs a Reaper.
func New(db *store.DB, log *zap.Logger) *Reaper {
	return &Reaper{db: db, log: log}
}

// Run starts the sweep loop; it exits when ctx is cancelled.
func (r *Reaper) Run(ctx context.Context) {
	r.log.Info("reaper started", zap.Duration("interval", SweepInterval))
	ticker := time.NewTicker(SweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			r.log.Info("reaper stopped")
			return
		case <-ticker.C:
			r.sweep(ctx)
		}
	}
}

func (r *Reaper) sweep(ctx context.Context) {
	// Move poison messages to DLQ first so they are not accidentally restored.
	dlqMoved, err := r.db.MoveExpiredToDeadLetter(ctx)
	if err != nil {
		r.log.Error("dlq sweep failed", zap.Error(err))
	} else if dlqMoved > 0 {
		r.log.Info("moved messages to DLQ", zap.Int64("count", dlqMoved))
		// We do not know the topic breakdown here; increment with "all" label for now.
		metrics.MessagesDeadLettered.WithLabelValues("all").Add(float64(dlqMoved))
	}

	// Restore messages whose visibility timeout expired but still have retries left.
	restored, err := r.db.RestoreExpiredMessages(ctx)
	if err != nil {
		r.log.Error("restore expired failed", zap.Error(err))
	} else if restored > 0 {
		r.log.Info("restored expired messages", zap.Int64("count", restored))
		metrics.MessagesRestored.WithLabelValues("all").Add(float64(restored))
	}
}

// DepthUpdater periodically refreshes queue-depth Prometheus gauges.
type DepthUpdater struct {
	db     *store.DB
	topics []string
	log    *zap.Logger
}

// NewDepthUpdater constructs a DepthUpdater for the given topic list.
func NewDepthUpdater(db *store.DB, topics []string, log *zap.Logger) *DepthUpdater {
	return &DepthUpdater{db: db, topics: topics, log: log}
}

// Run starts the depth-gauge update loop; exits when ctx is cancelled.
func (u *DepthUpdater) Run(ctx context.Context) {
	u.log.Info("depth updater started")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			u.update(ctx)
		}
	}
}

func (u *DepthUpdater) update(ctx context.Context) {
	for _, topic := range u.topics {
		depth, err := u.db.GetQueueDepth(ctx, topic)
		if err != nil {
			u.log.Warn("queue depth query failed", zap.String("topic", topic), zap.Error(err))
			continue
		}
		for partition, n := range depth {
			metrics.QueueDepth.WithLabelValues(topic, fmt.Sprintf("%d", partition)).Set(float64(n))
		}
		dlq, err := u.db.GetDLQDepth(ctx, topic)
		if err != nil {
			u.log.Warn("dlq depth query failed", zap.String("topic", topic), zap.Error(err))
			continue
		}
		metrics.DLQDepth.WithLabelValues(topic).Set(float64(dlq))
	}
}
