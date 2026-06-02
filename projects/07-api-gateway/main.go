// Package main wires together the API gateway's data plane (proxy) and
// control plane (admin) and starts both HTTP servers.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/ankitsriv89/07-api-gateway/api"
	"github.com/ankitsriv89/07-api-gateway/gateway"
	"github.com/ankitsriv89/07-api-gateway/metrics"
	"github.com/ankitsriv89/07-api-gateway/store"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync() //nolint:errcheck

	proxyAddr := envOr("ADDR", ":8088")
	adminAddr := envOr("ADMIN_ADDR", ":8089")
	dbURL := envOr("DATABASE_URL", "postgres://gw:gw@postgres:5432/apigateway?sslmode=disable")
	redisAddr := envOr("REDIS_ADDR", "redis:6379")
	redisPass := envOr("REDIS_PASSWORD", "")
	adminToken := envOr("ADMIN_TOKEN", "")

	// Storage layer
	pg, err := store.NewPG(dbURL)
	if err != nil {
		log.Fatal("connect postgres", zap.Error(err))
	}
	defer pg.Close() //nolint:errcheck

	rl, err := store.NewRedis(redisAddr, redisPass, 0)
	if err != nil {
		log.Fatal("connect redis", zap.Error(err))
	}
	defer rl.Close() //nolint:errcheck

	// Metrics
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	// In-process router (copy-on-write route table)
	router := &gateway.Router{}

	// Gateway domain engine
	gw := gateway.New(gateway.Config{}, pg, pg, rl, pg, router)

	// Load routes from DB at startup
	ctx := context.Background()
	if err := gw.ReloadRoutes(ctx); err != nil {
		log.Warn("initial route load failed — starting with empty table", zap.Error(err))
	}

	// Periodic route refresh (picks up changes made directly to the DB)
	go routeReloader(ctx, gw, log, 30*time.Second)

	// HTTP handler
	h := api.New(gw, pg, pg, rl, log, m, newRequestID)

	// ---- Admin (control plane) ----
	adminMux := mux.NewRouter()
	if adminToken != "" {
		adminMux.Use(tokenAuthMiddleware(adminToken))
	}
	h.RegisterAdmin(adminMux)
	adminMux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	// Serve the web UI on admin port so operators can reach it directly.
	adminMux.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("web"))))
	adminMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	})

	// ---- Proxy (data plane) ----
	proxyMux := mux.NewRouter()
	h.RegisterProxy(proxyMux)

	adminSrv := &http.Server{
		Addr:         adminAddr,
		Handler:      adminMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	proxySrv := &http.Server{
		Addr:         proxyAddr,
		Handler:      proxyMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	errc := make(chan error, 2)
	go func() {
		log.Info("admin server starting", zap.String("addr", adminAddr))
		errc <- adminSrv.ListenAndServe()
	}()
	go func() {
		log.Info("proxy server starting", zap.String("addr", proxyAddr))
		errc <- proxySrv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-quit:
		log.Info("shutting down", zap.String("signal", sig.String()))
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server error", zap.Error(err))
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = adminSrv.Shutdown(shutdownCtx)
	_ = proxySrv.Shutdown(shutdownCtx)
	log.Info("stopped")
}

// routeReloader periodically reloads the in-process route table from the DB.
// It exits when ctx is cancelled — owner is main, exit condition is context cancellation.
func routeReloader(ctx context.Context, gw *gateway.Gateway, log *zap.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := gw.ReloadRoutes(ctx); err != nil {
				log.Warn("periodic route reload failed", zap.Error(err))
			}
		}
	}
}

// tokenAuthMiddleware rejects requests that lack the correct Bearer token.
func tokenAuthMiddleware(token string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Always allow health checks without auth.
			if r.URL.Path == "/healthz" {
				next.ServeHTTP(w, r)
				return
			}
			// Allow serving the UI without auth.
			if r.URL.Path == "/" || len(r.URL.Path) > 1 && r.URL.Path[:8] == "/static/" {
				next.ServeHTTP(w, r)
				return
			}
			auth := r.Header.Get("Authorization")
			if auth != "Bearer "+token {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// newRequestID generates a cryptographically random 8-byte hex request ID.
func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
