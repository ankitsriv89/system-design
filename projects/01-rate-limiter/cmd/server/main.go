package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/ankitsriv89/rate-limiter/internal/api"
	"github.com/ankitsriv89/rate-limiter/internal/policy"
	"github.com/ankitsriv89/rate-limiter/internal/store"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	redisAddr := env("REDIS_ADDR", "localhost:6379")
	dsn := env("DATABASE_URL", "postgres://rl:rl@localhost:5432/ratelimiter?sslmode=disable")
	addr := env("LISTEN_ADDR", ":8080")

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("redis unreachable", zap.Error(err))
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("db open", zap.Error(err))
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)

	pStore := policy.NewStore(db)
	if err := pStore.Migrate(ctx); err != nil {
		log.Fatal("migration failed", zap.Error(err))
	}

	cache := policy.NewCache(pStore, 10*time.Second)
	limiter := store.NewRedisLimiter(rdb)
	h := api.New(cache, pStore, limiter, log)

	r := mux.NewRouter()
	r.Use(loggingMiddleware(log))
	h.Routes(r)
	r.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("server starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("listen", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}

func loggingMiddleware(log *zap.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.Info("request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Duration("dur", time.Since(start)),
			)
		})
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
