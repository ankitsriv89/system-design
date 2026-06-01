// Package paste defines the core domain types and service logic for the pastebin.
package paste

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"
)

// Visibility controls who can read a paste.
type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityUnlisted Visibility = "unlisted"
	VisibilityPrivate  Visibility = "private"
)

// Paste is the central aggregate for a stored text snippet.
type Paste struct {
	ID         string
	OwnerID    string // empty for anonymous pastes
	Title      string
	Language   string
	Visibility Visibility
	SizeBytes  int64
	ObjectKey  string // key in object storage
	ExpiresAt  *time.Time
	CreatedAt  time.Time
	BurnAfterRead bool
}

// Expired reports whether the paste has passed its expiry time.
func (p *Paste) Expired() bool {
	return p.ExpiresAt != nil && time.Now().After(*p.ExpiresAt)
}

var (
	ErrNotFound      = errors.New("paste not found")
	ErrExpired       = errors.New("paste has expired")
	ErrForbidden     = errors.New("access denied")
	ErrTooLarge      = errors.New("paste exceeds maximum size")
	ErrInvalidInput  = errors.New("invalid input")
)

const (
	MaxSizeBytes = 10 * 1024 * 1024 // 10 MB
	IDLength     = 8                 // bytes → 11 base64url chars
)

// NewID generates a cryptographically random URL-safe ID.
func NewID() (string, error) {
	b := make([]byte, IDLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ObjectKey returns the object storage key for a paste.
func ObjectKey(id string) string {
	return "pastes/" + id
}

// Repository is the persistence interface for paste metadata.
// Implementations live in the store package.
type Repository interface {
	Save(ctx context.Context, p *Paste) error
	Get(ctx context.Context, id string) (*Paste, error)
	Delete(ctx context.Context, id string) error
	// ListExpired returns IDs of pastes whose expires_at has passed.
	ListExpired(ctx context.Context, limit int) ([]string, error)
}

// ObjectStore is the interface for paste content (blobs).
type ObjectStore interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
}

// Cache is the interface for hot-path metadata caching.
type Cache interface {
	SetPaste(ctx context.Context, p *Paste, ttl time.Duration) error
	GetPaste(ctx context.Context, id string) (*Paste, error)
	DeletePaste(ctx context.Context, id string) error
	// Rate limiting
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// CreateRequest carries validated input for creating a paste.
type CreateRequest struct {
	OwnerID       string
	Title         string
	Language      string
	Visibility    Visibility
	Content       []byte
	TTL           *time.Duration // nil = no expiry
	BurnAfterRead bool
}

// Service orchestrates paste creation, retrieval, and deletion.
type Service struct {
	repo   Repository
	blobs  ObjectStore
	cache  Cache
}

func NewService(repo Repository, blobs ObjectStore, cache Cache) *Service {
	return &Service{repo: repo, blobs: blobs, cache: cache}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*Paste, error) {
	if len(req.Content) == 0 {
		return nil, ErrInvalidInput
	}
	if int64(len(req.Content)) > MaxSizeBytes {
		return nil, ErrTooLarge
	}
	if req.Visibility == "" {
		req.Visibility = VisibilityPublic
	}

	id, err := NewID()
	if err != nil {
		return nil, err
	}

	p := &Paste{
		ID:            id,
		OwnerID:       req.OwnerID,
		Title:         req.Title,
		Language:      req.Language,
		Visibility:    req.Visibility,
		SizeBytes:     int64(len(req.Content)),
		ObjectKey:     ObjectKey(id),
		CreatedAt:     time.Now().UTC(),
		BurnAfterRead: req.BurnAfterRead,
	}
	if req.TTL != nil {
		t := p.CreatedAt.Add(*req.TTL)
		p.ExpiresAt = &t
	}

	if err := s.blobs.Put(ctx, p.ObjectKey, req.Content, "text/plain; charset=utf-8"); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, p); err != nil {
		// Best-effort cleanup of orphaned blob.
		_ = s.blobs.Delete(ctx, p.ObjectKey)
		return nil, err
	}

	return p, nil
}

func (s *Service) Get(ctx context.Context, id, requesterID string) (*Paste, []byte, error) {
	// Try cache for metadata (public/unlisted pastes only).
	p, err := s.cache.GetPaste(ctx, id)
	if err != nil || p == nil {
		p, err = s.repo.Get(ctx, id)
		if err != nil {
			return nil, nil, err
		}
	}

	if p.Expired() {
		go func() { _ = s.hardDelete(context.Background(), p) }()
		return nil, nil, ErrExpired
	}

	if p.Visibility == VisibilityPrivate && p.OwnerID != requesterID {
		return nil, nil, ErrForbidden
	}

	content, err := s.blobs.Get(ctx, p.ObjectKey)
	if err != nil {
		return nil, nil, err
	}

	if p.BurnAfterRead {
		go func() { _ = s.hardDelete(context.Background(), p) }()
		return p, content, nil
	}

	// Cache public/unlisted metadata after a successful read.
	if p.Visibility != VisibilityPrivate {
		ttl := 5 * time.Minute
		if p.ExpiresAt != nil {
			remaining := time.Until(*p.ExpiresAt)
			if remaining < ttl {
				ttl = remaining
			}
		}
		_ = s.cache.SetPaste(ctx, p, ttl)
	}

	return p, content, nil
}

func (s *Service) Delete(ctx context.Context, id, requesterID string) error {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if p.OwnerID != "" && p.OwnerID != requesterID {
		return ErrForbidden
	}
	return s.hardDelete(ctx, p)
}

// hardDelete removes the paste from all layers atomically enough for our needs.
func (s *Service) hardDelete(ctx context.Context, p *Paste) error {
	_ = s.cache.DeletePaste(ctx, p.ID)
	_ = s.blobs.Delete(ctx, p.ObjectKey)
	return s.repo.Delete(ctx, p.ID)
}

// SweepExpired deletes up to limit expired pastes and returns how many were removed.
func (s *Service) SweepExpired(ctx context.Context, limit int) (int, error) {
	ids, err := s.repo.ListExpired(ctx, limit)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, id := range ids {
		p, err := s.repo.Get(ctx, id)
		if err != nil {
			continue
		}
		if err := s.hardDelete(ctx, p); err == nil {
			removed++
		}
	}
	return removed, nil
}
