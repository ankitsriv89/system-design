// Package worker runs the periodic index rebuild background job.
package worker

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/ankitsriv89/11-typeahead-autocomplete-system/autocomplete"
)

// Rebuilder calls RebuildIndex on a fixed interval.
// It stops when ctx is cancelled.
type Rebuilder struct {
	store    autocomplete.Store
	interval time.Duration
	log      *zap.Logger
}

// New creates a Rebuilder. interval controls how often a full rebuild runs.
func New(store autocomplete.Store, interval time.Duration, log *zap.Logger) *Rebuilder {
	return &Rebuilder{store: store, interval: interval, log: log}
}

// Run blocks until ctx is cancelled, triggering a rebuild every interval.
// The first rebuild fires immediately so that fresh deployments have a warm index.
func (r *Rebuilder) Run(ctx context.Context) {
	r.log.Info("worker: index rebuilder started", zap.Duration("interval", r.interval))
	r.rebuild(ctx)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.rebuild(ctx)
		case <-ctx.Done():
			r.log.Info("worker: index rebuilder stopping")
			return
		}
	}
}

// TriggerRebuild performs a single on-demand rebuild (called by admin API).
func (r *Rebuilder) TriggerRebuild(ctx context.Context) (*autocomplete.IndexStats, error) {
	return r.store.RebuildIndex(ctx)
}

func (r *Rebuilder) rebuild(ctx context.Context) {
	start := time.Now()
	stats, err := r.store.RebuildIndex(ctx)
	if err != nil {
		r.log.Error("worker: rebuild failed", zap.Error(err))
		return
	}
	r.log.Info("worker: rebuild succeeded",
		zap.Int64("items", stats.TotalItems),
		zap.Int64("prefixes", stats.TotalPrefixes),
		zap.Duration("elapsed", time.Since(start)))
}
