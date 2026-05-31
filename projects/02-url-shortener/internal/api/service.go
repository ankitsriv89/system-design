package api

import (
	"context"
	"errors"
	"time"

	"github.com/ankitsriv89/url-shortener/internal/link"
	"github.com/ankitsriv89/url-shortener/internal/metrics"
	"github.com/ankitsriv89/url-shortener/internal/owner"
	"github.com/ankitsriv89/url-shortener/internal/store"
)

const (
	codeLength     = 7
	maxCodeRetries = 8
)

type Service struct {
	db        linkStore
	cache     store.Cache
	generator link.CodeGenerator
	now       func() time.Time
}

type linkStore interface {
	GetOwner(ctx context.Context, id string) (owner.Owner, error)
	ActiveLinkCount(ctx context.Context, ownerID string, now time.Time) (int, error)
	CreateLink(ctx context.Context, l link.Link) error
	GetLink(ctx context.Context, code string) (link.Link, error)
}

func NewService(db linkStore, cache store.Cache, generator link.CodeGenerator) *Service {
	return &Service{
		db:        db,
		cache:     cache,
		generator: generator,
		now:       time.Now,
	}
}

func (s *Service) CreateLink(ctx context.Context, ownerID, longURL string, expiresAt *time.Time) (link.Link, error) {
	if ownerID == "" {
		return link.Link{}, link.ErrOwnerRequired
	}
	if err := link.ValidateLongURL(longURL); err != nil {
		return link.Link{}, err
	}
	o, err := s.db.GetOwner(ctx, ownerID)
	if err != nil {
		return link.Link{}, err
	}
	count, err := s.db.ActiveLinkCount(ctx, ownerID, s.now())
	if err != nil {
		return link.Link{}, err
	}
	if count >= o.Quota {
		return link.Link{}, link.ErrQuotaExceeded
	}

	for range maxCodeRetries {
		code, err := s.generator.Generate(codeLength)
		if err != nil {
			return link.Link{}, err
		}
		l := link.Link{Code: code, LongURL: longURL, OwnerID: ownerID, ExpiresAt: expiresAt}
		err = s.db.CreateLink(ctx, l)
		if err == nil {
			metrics.LinksCreated.WithLabelValues(ownerID).Inc()
			return s.db.GetLink(ctx, code)
		}
		if errors.Is(err, link.ErrCollision) {
			continue
		}
		return link.Link{}, err
	}
	return link.Link{}, link.ErrCollision
}
