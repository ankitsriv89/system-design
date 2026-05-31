// Package api_test exercises the service-layer business logic in isolation.
// The fakeStore and sequenceGenerator stubs replace real storage so tests run without a database.
package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ankitsriv89/url-shortener/internal/link"
	"github.com/ankitsriv89/url-shortener/internal/owner"
)

// fakeStore is an in-memory linkStore used by service unit tests.
type fakeStore struct {
	owners map[string]owner.Owner
	count  int
	links  map[string]link.Link
}

func (f *fakeStore) GetOwner(_ context.Context, id string) (owner.Owner, error) {
	o, ok := f.owners[id]
	if !ok {
		return owner.Owner{}, errors.New("missing owner")
	}
	return o, nil
}

func (f *fakeStore) ActiveLinkCount(context.Context, string, time.Time) (int, error) {
	return f.count, nil
}

func (f *fakeStore) CreateLink(_ context.Context, l link.Link) error {
	if _, exists := f.links[l.Code]; exists {
		return link.ErrCollision
	}
	l.CreatedAt = time.Now()
	f.links[l.Code] = l
	return nil
}

func (f *fakeStore) GetLink(_ context.Context, code string) (link.Link, error) {
	l, ok := f.links[code]
	if !ok {
		return link.Link{}, link.ErrNotFound
	}
	return l, nil
}

// sequenceGenerator produces codes from a fixed list, enabling deterministic collision tests.
type sequenceGenerator struct {
	values []string
	i      int
}

func (g *sequenceGenerator) Generate(int) (string, error) {
	v := g.values[g.i]
	g.i++
	return v, nil
}

// TestCreateLinkEnforcesQuota verifies that creating a link when the owner is at quota returns ErrQuotaExceeded.
func TestCreateLinkEnforcesQuota(t *testing.T) {
	st := &fakeStore{
		owners: map[string]owner.Owner{"demo": {ID: "demo", Quota: 1}},
		count:  1, // already at quota
		links:  map[string]link.Link{},
	}
	// Service does not use the cache; passing nil is safe for these unit tests.
	svc := NewService(st, nil, &sequenceGenerator{values: []string{"abc123Z"}})
	_, err := svc.CreateLink(context.Background(), "demo", "https://example.com", nil)
	if !errors.Is(err, link.ErrQuotaExceeded) {
		t.Fatalf("err = %v, want ErrQuotaExceeded", err)
	}
}

// TestCreateLinkRetriesCollision verifies that the service automatically retries on a code collision
// and succeeds with the second generated code.
func TestCreateLinkRetriesCollision(t *testing.T) {
	st := &fakeStore{
		owners: map[string]owner.Owner{"demo": {ID: "demo", Quota: 10}},
		links: map[string]link.Link{
			// Pre-populate the first code to force a collision on the first attempt.
			"abc123Z": {Code: "abc123Z", LongURL: "https://old.example", OwnerID: "demo"},
		},
	}
	svc := NewService(st, nil, &sequenceGenerator{values: []string{"abc123Z", "xyz789A"}})
	got, err := svc.CreateLink(context.Background(), "demo", "https://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "xyz789A" {
		t.Fatalf("code = %q, want xyz789A (the retry code)", got.Code)
	}
}
