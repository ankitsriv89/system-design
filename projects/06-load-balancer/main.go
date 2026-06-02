// Package main wires together all components and starts the load balancer server.
//
// Two listeners are started:
//   - Public  (ADDR, default :8086)  — /proxy/*, /healthz, /metrics, /static/*, /
//   - Admin   (ADMIN_ADDR, default 127.0.0.1:8087) — /v1/* control plane
//
// The admin listener is bound to loopback by default and requires a bearer token
// (ADMIN_TOKEN env var).  Set ADMIN_TOKEN to a long random secret in production.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/ankitsriv89/06-load-balancer/api"
	"github.com/ankitsriv89/06-load-balancer/balancer"
	"github.com/ankitsriv89/06-load-balancer/store"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync() //nolint:errcheck

	addr      := env("ADDR",       ":8086")
	adminAddr := env("ADMIN_ADDR", "127.0.0.1:8087")
	adminToken := env("ADMIN_TOKEN", "")
	dsn       := env("DATABASE_URL", "")

	if adminToken == "" {
		log.Warn("ADMIN_TOKEN not set — admin API is unauthenticated. Set ADMIN_TOKEN in production.")
	}

	lb := balancer.New(10*time.Second, 3*time.Second, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var st *store.Store
	if dsn != "" {
		var err error
		st, err = store.New(dsn, log)
		if err != nil {
			log.Warn("postgres unavailable, running without persistence", zap.Error(err))
		} else {
			defer st.Close()
			reloadBackends(ctx, lb, st, log)
		}
	}

	go drainEvents(ctx, lb, st, log)
	lb.Start(ctx)

	publicRouter := mux.NewRouter()
	adminRouter  := mux.NewRouter()
	api.New(ctx, lb, st, log, publicRouter, adminRouter, adminToken)

	publicSrv := &http.Server{
		Addr:         addr,
		Handler:      publicRouter,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	adminSrv := &http.Server{
		Addr:         adminAddr,
		Handler:      adminRouter,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("public listener", zap.String("addr", addr))
		if err := publicSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("public server error", zap.Error(err))
		}
	}()
	go func() {
		log.Info("admin listener", zap.String("addr", adminAddr))
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("admin server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down")

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	if err := publicSrv.Shutdown(shutCtx); err != nil {
		log.Error("public shutdown failed", zap.Error(err))
	}
	if err := adminSrv.Shutdown(shutCtx); err != nil {
		log.Error("admin shutdown failed", zap.Error(err))
	}
}

func reloadBackends(ctx context.Context, lb *balancer.LoadBalancer, st *store.Store, log *zap.Logger) {
	backends, err := st.ListBackends(ctx, "")
	if err != nil {
		log.Error("reload backends from DB", zap.Error(err))
		return
	}
	ok := 0
	for _, b := range backends {
		if err := lb.AddBackend(ctx, b.Service, b.URL, b.Weight); err != nil {
			log.Warn("reload backend rejected (SSRF check)", zap.String("url", b.URL), zap.Error(err))
			continue
		}
		ok++
	}
	log.Info("reloaded backends from DB", zap.Int("loaded", ok), zap.Int("total", len(backends)))
}

func drainEvents(ctx context.Context, lb *balancer.LoadBalancer, st *store.Store, log *zap.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-lb.Events:
			if st != nil {
				if err := st.RecordHealthEvent(ctx, ev.Service, ev.BackendURL, string(ev.Status), ev.LatencyMs); err != nil {
					log.Warn("record health event", zap.Error(err))
				}
				if err := st.UpdateBackendStatus(ctx, ev.Service, ev.BackendURL, string(ev.Status)); err != nil {
					log.Warn("update backend status", zap.Error(err))
				}
			}
		}
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
