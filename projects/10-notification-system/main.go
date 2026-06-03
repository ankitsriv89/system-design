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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/ankitsriv89/10-notification-system/api"
	"github.com/ankitsriv89/10-notification-system/metrics"
	"github.com/ankitsriv89/10-notification-system/store"
	"github.com/ankitsriv89/10-notification-system/worker"
)

func main() {
	log, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer log.Sync() //nolint:errcheck

	addr := envOr("LISTEN_ADDR", ":8091")
	dsn := envOr("DATABASE_URL", "postgres://notif:notif@localhost:5432/notifications?sslmode=disable")

	st, err := store.New(dsn)
	if err != nil {
		log.Fatal("store init", zap.Error(err))
	}
	defer st.Close()

	reg := prometheus.NewRegistry()
	met := metrics.New(reg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	disp := worker.NewDispatcher(st, met, log)
	disp.Start(ctx)

	h := api.New(st, disp, met, log)
	r := mux.NewRouter()
	h.Register(r)
	r.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("notification system listening", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down")
	cancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("server shutdown", zap.Error(err))
	}

	disp.Stop()
	log.Info("shutdown complete")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
