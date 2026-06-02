// Package main wires together all components and starts the load balancer server.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/ankitsriv89/06-load-balancer/api"
	"github.com/ankitsriv89/06-load-balancer/balancer"
	"github.com/ankitsriv89/06-load-balancer/store"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync() //nolint:errcheck

	addr := env("ADDR", ":8086")
	dsn := env("DATABASE_URL", "")

	lb := balancer.New(10*time.Second, 3*time.Second, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Persist backends to Postgres when DATABASE_URL is set.
	var st *store.Store
	if dsn != "" {
		var err error
		st, err = store.New(dsn, log)
		if err != nil {
			log.Warn("postgres unavailable, running without persistence", zap.Error(err))
		} else {
			defer st.Close()
			// Reload backends persisted from a prior run.
			reloadBackends(ctx, lb, st, log)
		}
	}

	// Drain health events into the store (if available).
	go drainEvents(ctx, lb, st, log)

	lb.Start(ctx)

	r := mux.NewRouter()
	api.New(ctx, lb, st, log, r)

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Info("load balancer listening", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
	}
}

func reloadBackends(ctx context.Context, lb *balancer.LoadBalancer, st *store.Store, log *zap.Logger) {
	backends, err := st.ListBackends(ctx, "")
	if err != nil {
		log.Error("reload backends from DB", zap.Error(err))
		return
	}
	for _, b := range backends {
		lb.AddBackend(ctx, b.Service, b.URL, b.Weight)
	}
	log.Info("reloaded backends from DB", zap.Int("count", len(backends)))
}

func drainEvents(ctx context.Context, lb *balancer.LoadBalancer, st *store.Store, log *zap.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-lb.Events:
			if st != nil {
				if err := st.RecordHealthEvent(ctx, ev.Service, ev.BackendURL, string(ev.Status), ev.LatencyMs); err != nil {
					log.Warn("record health event", zap.Error(err))
				}
				if err := st.UpdateBackendStatus(ctx, ev.Service, ev.BackendURL, string(ev.Status)); err != nil {
					log.Warn("update backend status", zap.Error(err))
				}
			}
		}
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
