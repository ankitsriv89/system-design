package store

import (
	"context"
	"time"
)

// Cache is the interface both MemCache and any future Redis-backed cache must satisfy.
type Cache interface {
	GetURL(ctx context.Context, code string) (url string, missing bool, found bool, err error)
	SetURL(ctx context.Context, code, longURL string, ttl time.Duration) error
	SetMissing(ctx context.Context, code string, ttl time.Duration) error
}
