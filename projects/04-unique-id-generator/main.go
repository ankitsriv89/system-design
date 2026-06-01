// main is the entry point for the unique-id-generator service.
//
// Startup sequence:
//  1. Connect to PostgreSQL and acquire a worker_id lease.
//  2. Construct a Snowflake Generator bound to that worker_id.
//  3. Start the background lease renewer.
//  4. Serve the HTTP API.
//
// Shutdown sequence (SIGINT / SIGTERM):
//  1. Stop accepting new requests (HTTP graceful shutdown).
//  2. Cancel the background renewer context.
//  3. Release the worker_id lease so another instance can claim it immediately.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/ankitsriv89/uniqueid/api"
	"github.com/ankitsriv89/uniqueid/generator"
	"github.com/ankitsriv89/uniqueid/lease"
	"github.com/ankitsriv89/uniqueid/metrics"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync() //nolint:errcheck

	cfg := configFromEnv()

	// --- lease manager: claims a worker_id from PostgreSQL ---
	lm, err := lease.New(cfg.databaseURL, cfg.region, log)
	if err != nil {
		log.Fatal("lease manager init", zap.Error(err))
	}
	defer lm.Close() //nolint:errcheck

	// Wait for PostgreSQL to be ready before trying to acquire a lease.
	waitForDB(log, lm)

	acquireCtx, acquireCancel := context.WithTimeout(context.Background(), 30*time.Second)
	workerID, err := lm.Acquire(acquireCtx)
	acquireCancel()
	if err != nil {
		log.Fatal("failed to acquire worker lease", zap.Error(err))
	}

	// renewCtx is cancelled on shutdown to stop the background renewer.
	renewCtx, renewCancel := context.WithCancel(context.Background())
	defer renewCancel()

	// --- generator: Snowflake ID engine ---
	gen, err := generator.New(workerID, func(wid, driftMs int64) {
		// Fire metrics and persist the incident for alerting.
		metrics.ClockRollbacks.Inc()
		metrics.ClockDriftMs.Observe(float64(driftMs))
		log.Warn("clock rollback detected",
			zap.Int64("worker_id", wid),
			zap.Int64("drift_ms", driftMs),
		)
		lm.RecordClockIncident(renewCtx, driftMs)
	})
	if err != nil {
		log.Fatal("generator init", zap.Error(err))
	}

	// Start renewing the lease in the background. If renewal fails repeatedly
	// the process will log errors but continue serving; the lease TTL (30s)
	// gives enough runway to drain in-flight requests before expiry.
	lm.StartRenewer(renewCtx)

	// --- HTTP server ---
	h := api.New(gen, cfg.region, log)
	r := mux.NewRouter()
	r.Use(metrics.Middleware)
	r.Use(requestLogger(log))
	h.Register(r)

	srv := &http.Server{
		Addr:         cfg.listenAddr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server starting",
			zap.String("addr", cfg.listenAddr),
			zap.Int64("worker_id", workerID),
			zap.String("region", cfg.region),
		)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	<-quit
	log.Info("shutting down")

	// Stop renewer before releasing so the renewer goroutine doesn't race
	// a Release with its own UPDATE.
	renewCancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("http shutdown error", zap.Error(err))
	}

	if err := lm.Release(shutCtx); err != nil {
		log.Error("lease release error", zap.Error(err))
	}

	log.Info("shutdown complete")
}

// waitForDB blocks until PostgreSQL responds or the timeout elapses.
func waitForDB(log *zap.Logger, lm *lease.Manager) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for {
		if err := lm.Ping(ctx); err == nil {
			return
		}
		log.Info("waiting for postgres...")
		select {
		case <-ctx.Done():
			log.Fatal("postgres not ready in time")
		case <-time.After(2 * time.Second):
		}
	}
}

// requestLogger returns a middleware that logs each request's method, path, and duration.
func requestLogger(log *zap.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.Info("request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Duration("duration", time.Since(start)),
				zap.String("remote", r.RemoteAddr),
			)
		})
	}
}

// config holds all runtime configuration read from environment variables.
type config struct {
	listenAddr  string
	databaseURL string
	region      string
}

// configFromEnv reads configuration from environment variables with sensible defaults.
func configFromEnv() config {
	return config{
		listenAddr:  getEnv("LISTEN_ADDR", ":8083"),
		databaseURL: getEnv("DATABASE_URL", "postgres://uniqueid:uniqueid@postgres:5432/uniqueid?sslmode=disable"),
		region:      getEnv("REGION", "local"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
