FROM golang:1.26.3-alpine AS builder
WORKDIR /app
# Install build deps for cgo-free minio client.
RUN apk add --no-cache git
COPY go.mod ./
# Download and verify modules, generate go.sum.
RUN go mod download || true
COPY . .
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o pastebin .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=builder /app/pastebin .
EXPOSE 8082
ENTRYPOINT ["./pastebin"]
