// Package main wires together the consistent-hashing service.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/ankitsriv89/consistent-hashing/api"
	_ "github.com/ankitsriv89/consistent-hashing/metrics"
	"github.com/ankitsriv89/consistent-hashing/store"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8084"
	}

	s := store.New()
	h := api.New(s, log)

	srv := &http.Server{
		Addr:         addr,
		Handler:      h,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("consistent-hashing service starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
	}
	log.Info("server stopped")
}
