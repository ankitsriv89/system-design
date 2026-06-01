# Dockerfile for the unique-id-generator service.
# Multi-stage build: builder compiles a statically linked binary; final image is minimal alpine.

FROM golang:1.23-alpine AS builder
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o uniqueid .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=builder /app/uniqueid .
EXPOSE 8083
ENTRYPOINT ["./uniqueid"]
