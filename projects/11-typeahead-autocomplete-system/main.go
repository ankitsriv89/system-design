// Package main wires dependencies and starts the typeahead autocomplete service.
package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/ankitsriv89/11-typeahead-autocomplete-system/api"
	"github.com/ankitsriv89/11-typeahead-autocomplete-system/store"
	"github.com/ankitsriv89/11-typeahead-autocomplete-system/worker"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	listenAddr := env("LISTEN_ADDR", ":8092")
	dsn := env("DATABASE_URL", "postgres://typeahead:typeahead@localhost:5432/typeahead?sslmode=disable")
	redisAddr := env("REDIS_ADDR", "localhost:6379")
	rebuildInterval := envDuration("REBUILD_INTERVAL", 30*time.Minute)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("main: open postgres", zap.Error(err))
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxIdleTime(5 * time.Minute)
	defer db.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		PoolSize:     10,
		MinIdleConns: 5,
	})
	defer rdb.Close()

	st, err := store.New(db, rdb, log)
	if err != nil {
		log.Fatal("main: init store", zap.Error(err))
	}

	rebuilder := worker.New(st, rebuildInterval, log)

	router := api.New(st, rebuilder, log)

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go rebuilder.Run(ctx)

	go func() {
		log.Info("main: server starting", zap.String("addr", listenAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("main: server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("main: shutting down")
	cancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("main: graceful shutdown failed", zap.Error(err))
	}
	log.Info("main: stopped")
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
