FROM golang:1.22-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build -ldflags="-s -w" -o /notification-system .

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /notification-system /app/notification-system
COPY web/ web/

ENV LISTEN_ADDR=:8091
ENV DATABASE_URL=postgres://notif:notif@postgres:5432/notifications?sslmode=disable

EXPOSE 8091
ENTRYPOINT ["/app/notification-system"]
