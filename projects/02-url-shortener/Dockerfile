# ── Stage 1: Build ──────────────────────────────────────────────────────────
# Uses the official Go Alpine image. CGO is disabled so the binary runs on
# any Linux/musl target including Oracle Cloud ARM64 (aarch64).
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Download dependencies separately so Docker layer cache is reused on code-only changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a fully static binary (no libc dependency).
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w" \
      -o /url-shortener ./cmd/server

# ── Stage 2: Runtime ─────────────────────────────────────────────────────────
# Minimal Alpine image — only ca-certificates (for outbound HTTPS in future
# features) and tzdata (for correct time zone handling in click timestamps).
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

# Copy the static binary and the frontend assets.
COPY --from=builder /url-shortener /url-shortener
COPY --from=builder /app/web ./web

EXPOSE 8081

ENTRYPOINT ["/url-shortener"]
