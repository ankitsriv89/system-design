FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o consistent-hashing .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/consistent-hashing /consistent-hashing
EXPOSE 8084
ENTRYPOINT ["/consistent-hashing"]
