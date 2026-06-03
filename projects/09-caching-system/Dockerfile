FROM golang:1.22-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build -ldflags="-s -w" -o /caching-system .

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /caching-system /app/caching-system
COPY web/ web/

RUN mkdir -p /data/cache

ENV AOF_PATH=/data/cache/cache.aof
ENV LISTEN_ADDR=:8090
ENV EVICTION_POLICY=lru
ENV MAX_MEMORY_MB=64
ENV DEFAULT_TTL_MS=0

EXPOSE 8090
ENTRYPOINT ["/app/caching-system"]
