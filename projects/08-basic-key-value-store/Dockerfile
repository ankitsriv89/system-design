FROM golang:1.22-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build -ldflags="-s -w" -o /kvstore .

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /kvstore /app/kvstore
COPY web/ web/

RUN mkdir -p /data/kvstore

ENV DATA_DIR=/data/kvstore
ENV LISTEN_ADDR=:8088

EXPOSE 8088
ENTRYPOINT ["/app/kvstore"]
