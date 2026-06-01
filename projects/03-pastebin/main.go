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

	"github.com/ankitsriv89/pastebin/api"
	"github.com/ankitsriv89/pastebin/metrics"
	"github.com/ankitsriv89/pastebin/paste"
	"github.com/ankitsriv89/pastebin/store"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg := configFromEnv()

	// --- storage layer ---
	db, err := store.NewDB(cfg.databaseURL)
	if err != nil {
		log.Fatal("postgres init", zap.Error(err))
	}

	cache := store.NewRedisCache(cfg.redisAddr)

	blobs, err := store.NewMinioStore(cfg.minioEndpoint, cfg.minioAccessKey, cfg.minioSecretKey, cfg.minioUseSSL)
	if err != nil {
		log.Fatal("minio init", zap.Error(err))
	}

	// Wait for dependencies to be ready.
	waitForDeps(log, db, cache)

	ctx := context.Background()
	if err := blobs.EnsureBucket(ctx); err != nil {
		log.Fatal("minio bucket init", zap.Error(err))
	}

	// --- service + handler ---
	svc := paste.NewService(db, blobs, cache)
	h := api.New(svc, cache, log)

	r := mux.NewRouter()
	r.Use(metrics.Middleware)
	r.Use(requestLogger(log))
	h.Register(r)

	srv := &http.Server{
		Addr:         cfg.listenAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// --- background expiry sweeper ---
	go runSweeper(ctx, svc, log, cfg.sweepInterval)

	// --- start + graceful shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server starting", zap.String("addr", cfg.listenAddr))
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	<-quit
	log.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
}

func runSweeper(ctx context.Context, svc *paste.Service, log *zap.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()
			n, err := svc.SweepExpired(ctx, 500)
			dur := time.Since(start)
			metrics.ExpirySweepDuration.Observe(dur.Seconds())
			if err != nil {
				log.Error("expiry sweep error", zap.Error(err))
			} else if n > 0 {
				metrics.ExpiredPastesRemoved.Add(float64(n))
				log.Info("expiry sweep", zap.Int("removed", n), zap.Duration("duration", dur))
			}
		}
	}
}

func waitForDeps(log *zap.Logger, db *store.DB, cache *store.RedisCache) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		dbErr := db.Ping(ctx)
		cacheErr := cache.Ping(ctx)
		if dbErr == nil && cacheErr == nil {
			return
		}
		log.Info("waiting for dependencies",
			zap.NamedError("postgres", dbErr),
			zap.NamedError("redis", cacheErr),
		)
		select {
		case <-ctx.Done():
			log.Fatal("dependencies not ready in time")
		case <-time.After(2 * time.Second):
		}
	}
}

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

type config struct {
	listenAddr    string
	databaseURL   string
	redisAddr     string
	minioEndpoint string
	minioAccessKey string
	minioSecretKey string
	minioUseSSL   bool
	sweepInterval time.Duration
}

func configFromEnv() config {
	return config{
		listenAddr:     getEnv("LISTEN_ADDR", ":8082"),
		databaseURL:    getEnv("DATABASE_URL", "postgres://paste:paste@postgres:5432/pastebin?sslmode=disable"),
		redisAddr:      getEnv("REDIS_ADDR", "redis:6379"),
		minioEndpoint:  getEnv("MINIO_ENDPOINT", "minio:9000"),
		minioAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		minioSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
		minioUseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",
		sweepInterval:  mustDuration(getEnv("SWEEP_INTERVAL", "1m")),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic("invalid duration: " + s)
	}
	return d
}
