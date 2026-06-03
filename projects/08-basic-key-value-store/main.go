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

	"github.com/ankitsriv89/08-basic-key-value-store/api"
	"github.com/ankitsriv89/08-basic-key-value-store/metrics"
	"github.com/ankitsriv89/08-basic-key-value-store/store"
)

func main() {
	log, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	dataDir := envOr("DATA_DIR", "/data/kvstore")
	addr := envOr("LISTEN_ADDR", ":8088")

	engine, err := store.Open(dataDir, log)
	if err != nil {
		log.Fatal("open engine", zap.Error(err))
	}
	defer engine.Close()

	met := metrics.New()
	h := api.New(engine, log, met)

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
		log.Info("server starting", zap.String("addr", addr))
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
