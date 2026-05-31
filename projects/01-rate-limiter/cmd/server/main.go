package main

import (
	"context"
	"crypto/rand"
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
	"github.com/ankitsriv89/rate-limiter/internal/odyssey"
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
	// Pool limits prevent connection exhaustion under load;
	// idle limit keeps connections from piling up during quiet periods.
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)

	pStore := policy.NewStore(db)
	if err := pStore.Migrate(ctx); err != nil {
		log.Fatal("migration failed", zap.Error(err))
	}

	// Session signing key — used to HMAC-sign the session cookie so clients
	// cannot forge or tamper with it to hijack another player's progress.
	sessionSecret := env("SESSION_SECRET", "")
	if len(sessionSecret) < 32 {
		log.Warn("SESSION_SECRET not set or too short — generating ephemeral key (sessions will not survive restarts)")
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			log.Fatal("generate session key", zap.Error(err))
		}
		sessionSecret = string(b)
	}
	odyssey.SetSessionKey(sessionSecret)

	// Only trust X-Forwarded-For from configured proxy CIDRs.
	// Without this, any client can spoof their IP to bypass per-IP rate limits.
	odyssey.ParseTrustedProxies(env("TRUSTED_PROXIES", "127.0.0.1,::1"))

	oStore := odyssey.NewStore(db)
	if err := oStore.Migrate(ctx); err != nil {
		log.Fatal("odyssey migration failed", zap.Error(err))
	}
	if err := oStore.SeedQuestions(ctx); err != nil {
		log.Warn("odyssey seed questions", zap.Error(err))
	}

	cache := policy.NewCache(pStore, 10*time.Second)
	limiter := store.NewRedisLimiter(rdb)
	h := api.New(cache, pStore, limiter, log)

	var groqClient *odyssey.GroqClient
	if key := env("GROQ_API_KEY", ""); key != "" {
		groqClient = odyssey.NewGroqClient(key)
		log.Info("Groq AI enabled for question generation")
	} else {
		log.Info("GROQ_API_KEY not set — using built-in question bank only")
	}
	oh := api.NewOdysseyHandler(oStore, limiter, cache, groqClient, log)

	r := mux.NewRouter()
	r.Use(corsMiddleware)
	r.Use(loggingMiddleware(log))
	h.Routes(r)
	oh.Routes(r)
	r.Handle("/metrics", promhttp.Handler())
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("web")))

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
	// 10-second grace period lets in-flight requests drain before the process exits.
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
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
