# Wiring & Deploy

## `cmd/cube-server/main.go`

Mirrors `cmd/server/main.go`. Key differences: port `:8081`, mounts cube routes,
mounts policy CRUD routes (so `seed_cube.sh` only needs `:8081`).

```go
package main

import (
    "context"; "crypto/rand"; "database/sql"; "net/http"
    "os"; "os/signal"; "syscall"; "time"

    "github.com/gorilla/mux"
    _ "github.com/lib/pq"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "github.com/redis/go-redis/v9"
    "go.uber.org/zap"

    "github.com/ankitsriv89/rate-limiter/internal/api"
    "github.com/ankitsriv89/rate-limiter/internal/cube"
    "github.com/ankitsriv89/rate-limiter/internal/odyssey"
    "github.com/ankitsriv89/rate-limiter/internal/policy"
    "github.com/ankitsriv89/rate-limiter/internal/store"
)

func main() {
    log, _ := zap.NewProduction()
    defer log.Sync()

    redisAddr := env("REDIS_ADDR", "localhost:6379")
    dsn       := env("DATABASE_URL", "postgres://rl:rl@localhost:5432/ratelimiter?sslmode=disable")
    addr      := env("LISTEN_ADDR", ":8081")

    rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
    ctx := context.Background()
    if err := rdb.Ping(ctx).Err(); err != nil {
        log.Fatal("redis unreachable", zap.Error(err))
    }

    db, err := sql.Open("postgres", dsn)
    if err != nil { log.Fatal("db open", zap.Error(err)) }
    db.SetMaxOpenConns(20)
    db.SetMaxIdleConns(5)

    pStore := policy.NewStore(db)
    if err := pStore.Migrate(ctx); err != nil { log.Fatal("policy migration", zap.Error(err)) }

    cStore := cube.NewStore(db)
    if err := cStore.Migrate(ctx); err != nil { log.Fatal("cube migration", zap.Error(err)) }
    if err := cStore.SeedObjects(ctx); err != nil { log.Warn("cube seed", zap.Error(err)) }

    sessionSecret := env("SESSION_SECRET", "")
    if len(sessionSecret) < 32 {
        log.Warn("SESSION_SECRET not set — generating ephemeral key")
        b := make([]byte, 32)
        rand.Read(b)
        sessionSecret = string(b)
    }
    odyssey.SetSessionKey(sessionSecret)
    odyssey.ParseTrustedProxies(env("TRUSTED_PROXIES", "127.0.0.1,::1"))

    cache   := policy.NewCache(pStore, 10*time.Second)
    limiter := store.NewRedisLimiter(rdb)

    var groqClient *odyssey.GroqClient
    if key := env("GROQ_API_KEY", ""); key != "" {
        groqClient = odyssey.NewGroqClient(key)
        log.Info("Groq AI enabled")
    }

    r := mux.NewRouter()
    r.Use(corsMiddleware)
    r.Use(loggingMiddleware(log))

    // Policy CRUD (shared policies table, needed for seed_cube.sh)
    api.New(cache, pStore, limiter, log).Routes(r)

    // Cube game routes
    api.NewCubeHandler(cStore, limiter, cache, groqClient, log).Routes(r)

    r.Handle("/metrics", promhttp.Handler())
    r.PathPrefix("/").Handler(http.FileServer(http.Dir("web")))

    srv := &http.Server{
        Addr:         addr,
        Handler:      r,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
    }
    go func() {
        log.Info("cube-server starting", zap.String("addr", addr))
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatal("listen", zap.Error(err))
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    srv.Shutdown(shutCtx)
}

func env(key, fallback string) string {
    if v := os.Getenv(key); v != "" { return v }
    return fallback
}
// corsMiddleware and loggingMiddleware: copy the two small funcs from cmd/server/main.go
// (or refactor into internal/api/middleware.go — either is acceptable for a small copy)
```

---

## `scripts/seed_cube.sh`

```bash
#!/usr/bin/env bash
set -euo pipefail
BASE=${BASE_URL:-http://localhost:8081}

echo "==> Seeding cube-hops policy at $BASE"

# 30 turns per refuel window (3 hours)
# refill_rate = 30 / (3 * 3600) = 0.002778 tokens/second
curl -sf -X PUT "$BASE/v1/policies/cube-hops" \
  -H "Content-Type: application/json" \
  -d '{
    "subject_type": "ip",
    "algorithm":    "token_bucket",
    "capacity":     30,
    "refill_rate":  0.002778,
    "window_ms":    0,
    "action":       "deny"
  }' | jq .

echo "==> cube-hops seeded"
curl -sf "$BASE/v1/policies/cube-hops" | jq .
```

---

## `docker-compose.yml` additions

Add a `cube-server` service (same image, separate command, separate port):

```yaml
  cube-server:
    build: .
    command: ["/cube-server"]       # assumes Dockerfile builds /cube-server binary
    ports:
      - "8081:8081"
    environment:
      LISTEN_ADDR:  ":8081"
      REDIS_ADDR:   "redis:6379"
      DATABASE_URL: "postgres://rl:rl@postgres:5432/ratelimiter?sslmode=disable"
      SESSION_SECRET: "${SESSION_SECRET:-change-me-in-production-32chars+}"
      GROQ_API_KEY:   "${GROQ_API_KEY:-}"
      TRUSTED_PROXIES: "172.0.0.0/8,127.0.0.1"
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
    restart: unless-stopped
```

---

## `Dockerfile` additions

Extend the existing multi-stage build to produce a second binary:

```dockerfile
# Build stage (add after existing server build)
RUN go build -o /cube-server ./cmd/cube-server

# Final stage: copy both binaries
COPY --from=builder /server      /server
COPY --from=builder /cube-server /cube-server
```

The docker-compose `command` field selects which binary runs in each service.

---

## Prometheus

Cube-server exposes `/metrics` on `:8081`. Add to `deploy/prometheus/prometheus.yml`:

```yaml
  - job_name: cube-server
    static_configs:
      - targets: ['cube-server:8081']
```

---

## Local quick-start (without Docker)

```bash
# 1. Start deps
docker compose up -d redis postgres

# 2. Run migrations + cube-server
go run ./cmd/cube-server

# 3. Seed the cube-hops policy
bash scripts/seed_cube.sh

# 4. Open the game
open http://localhost:8081/cube.html
```
