// Package main wires dependencies and starts the message queue service.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/ankitsriv89/13-message-queue/api"
	"github.com/ankitsriv89/13-message-queue/store"
	"github.com/ankitsriv89/13-message-queue/worker"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	dsn := getEnv("DATABASE_URL", "postgres://mq:mq@localhost:5432/messagequeue?sslmode=disable")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPass := getEnv("REDIS_PASSWORD", "")
	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))
	listenAddr := getEnv("LISTEN_ADDR", ":8094")

	db, err := store.NewDB(dsn)
	if err != nil {
		log.Fatal("open db", zap.Error(err))
	}
	defer db.Close()

	cache := store.NewCache(redisAddr, redisPass, redisDB)
	defer cache.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cache.Ping(ctx); err != nil {
		log.Warn("redis ping failed — continuing without cache", zap.Error(err))
	}

	reaper := worker.New(db, log)
	go reaper.Run(ctx)

	h := api.NewHandler(db, cache, log)
	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      h.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("server starting", zap.String("addr", listenAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutCtx)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
