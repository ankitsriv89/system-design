FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /rate-limiter ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /rate-limiter /rate-limiter
COPY --from=builder /app/web ./web
EXPOSE 8081
ENTRYPOINT ["/rate-limiter"]
