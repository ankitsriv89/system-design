// main is the entry point for the URL shortener server.
// It reads configuration from environment variables, wires up the PostgreSQL
// store and in-memory cache, registers routes, and runs a graceful-shutdown
// HTTP server.
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
	"go.uber.org/zap"

	"github.com/ankitsriv89/url-shortener/internal/api"
	"github.com/ankitsriv89/url-shortener/internal/link"
	"github.com/ankitsriv89/url-shortener/internal/store"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync() //nolint:errcheck

	// Configuration via environment variables with sensible local defaults.
	addr    := env("LISTEN_ADDR",   ":8081")
	baseURL := env("BASE_URL",      "http://localhost:8081")
	dsn     := env("DATABASE_URL",  "postgres://url:url@localhost:5432/urlshortener?sslmode=disable")

	// Connect to PostgreSQL (shared instance on the host VM).
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("db open failed", zap.Error(err))
	}
	defer db.Close()

	// Conservative pool settings — this is one of many small projects on the same instance.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(3)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx := context.Background()

	pg := store.NewPostgresStore(db)
	if err := pg.Migrate(ctx); err != nil {
		log.Fatal("migration failed", zap.Error(err))
	}
	if err := pg.SeedOwners(ctx); err != nil {
		log.Fatal("seed owners failed", zap.Error(err))
	}

	// In-memory cache — no external Redis required.
	// Cache resets on restart, which is acceptable for a URL shortener demo.
	cache := store.NewMemCache()

	service := api.NewService(pg, cache, link.RandomCodeGenerator{})
	h := api.New(service, pg, cache, baseURL, log)

	r := mux.NewRouter()
	r.Use(corsMiddleware)
	r.Use(loggingMiddleware(log))
	h.Routes(r)

	// Prometheus metrics endpoint.
	r.Handle("/metrics", promhttp.Handler())

	// Serve the frontend from the web/ directory.
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	}).Methods(http.MethodGet)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web"))))

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("server starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("listen failed", zap.Error(err))
		}
	}()

	// Block until SIGINT or SIGTERM, then gracefully drain in-flight requests.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}

// corsMiddleware adds permissive CORS headers required for the browser-based demo UI.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Owner-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs each request's method, path, and duration using structured JSON.
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

// env returns the value of the named environment variable, or fallback if unset.
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
