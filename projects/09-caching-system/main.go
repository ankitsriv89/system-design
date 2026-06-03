// Package main wires dependencies, starts the HTTP server, and handles signals.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/ankitsriv89/09-caching-system/api"
	"github.com/ankitsriv89/09-caching-system/cache"
	"github.com/ankitsriv89/09-caching-system/metrics"
	"github.com/ankitsriv89/09-caching-system/store"
)

func main() {
	log, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	addr := envOr("LISTEN_ADDR", ":8090")
	aofPath := envOr("AOF_PATH", "/data/cache/cache.aof")
	policyStr := envOr("EVICTION_POLICY", "lru")
	maxMBStr := envOr("MAX_MEMORY_MB", "64")
	defaultTTLStr := envOr("DEFAULT_TTL_MS", "0")

	maxBytes := parseMB(maxMBStr)
	defaultTTL := parseDurationMs(defaultTTLStr)

	policy := cache.PolicyLRU
	if policyStr == "lfu" {
		policy = cache.PolicyLFU
	}

	// open AOF before constructing the cache so we can replay on warm start
	aof, err := store.Open(aofPath, log)
	if err != nil {
		log.Fatal("open aof", zap.Error(err))
	}
	defer aof.Close()

	met := metrics.New(nil)

	c := cache.New(cache.Config{
		Policy:      policy,
		MaxBytes:    maxBytes,
		DefaultTTL:  defaultTTL,
		SweepPeriod: 30 * time.Second,
		Log:         log,
		OnEvict: func(rec cache.EvictionRecord) {
			met.Evictions.WithLabelValues(rec.Reason).Inc()
		},
	})
	defer c.Close()

	// warm restart: replay AOF entries
	entries, err := aof.Replay()
	if err != nil {
		log.Warn("aof replay failed; starting cold", zap.Error(err))
	} else {
		for _, e := range entries {
			ttl := time.Duration(-1) // signal "no default TTL"
			if !e.ExpiresAt.IsZero() {
				remaining := time.Until(e.ExpiresAt)
				if remaining <= 0 {
					continue // already expired
				}
				ttl = remaining
			}
			c.Set(e.Key, e.Value, ttl)
		}
		log.Info("aof replay complete", zap.Int("keys", len(entries)))
	}

	h := api.New(c, aof, met, log)

	r := mux.NewRouter()
	h.Register(r)
	r.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("server starting",
			zap.String("addr", addr),
			zap.String("policy", string(policy)),
			zap.Int64("max_bytes", maxBytes),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseMB(s string) int64 {
	var n int64
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int64(ch-'0')
		}
	}
	return n * 1024 * 1024
}

func parseDurationMs(s string) time.Duration {
	var n int64
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int64(ch-'0')
		}
	}
	return time.Duration(n) * time.Millisecond
}
