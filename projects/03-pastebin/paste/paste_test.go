package paste_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ankitsriv89/pastebin/paste"
)

// --- in-memory fakes ---

type fakeRepo struct {
	data map[string]*paste.Paste
}

func newFakeRepo() *fakeRepo { return &fakeRepo{data: map[string]*paste.Paste{}} }

func (r *fakeRepo) Save(_ context.Context, p *paste.Paste) error {
	r.data[p.ID] = p
	return nil
}
func (r *fakeRepo) Get(_ context.Context, id string) (*paste.Paste, error) {
	p, ok := r.data[id]
	if !ok {
		return nil, paste.ErrNotFound
	}
	return p, nil
}
func (r *fakeRepo) Delete(_ context.Context, id string) error {
	if _, ok := r.data[id]; !ok {
		return paste.ErrNotFound
	}
	delete(r.data, id)
	return nil
}
func (r *fakeRepo) ListExpired(_ context.Context, limit int) ([]string, error) {
	var ids []string
	for id, p := range r.data {
		if p.ExpiresAt != nil && time.Now().After(*p.ExpiresAt) {
			ids = append(ids, id)
			if len(ids) >= limit {
				break
			}
		}
	}
	return ids, nil
}

type fakeBlobs struct {
	data map[string][]byte
}

func newFakeBlobs() *fakeBlobs { return &fakeBlobs{data: map[string][]byte{}} }

func (b *fakeBlobs) Put(_ context.Context, key string, data []byte, _ string) error {
	b.data[key] = data
	return nil
}
func (b *fakeBlobs) Get(_ context.Context, key string) ([]byte, error) {
	d, ok := b.data[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return d, nil
}
func (b *fakeBlobs) Delete(_ context.Context, key string) error {
	delete(b.data, key)
	return nil
}

type fakeCache struct {
	data map[string]*paste.Paste
}

func newFakeCache() *fakeCache { return &fakeCache{data: map[string]*paste.Paste{}} }

func (c *fakeCache) SetPaste(_ context.Context, p *paste.Paste, _ time.Duration) error {
	c.data[p.ID] = p
	return nil
}
func (c *fakeCache) GetPaste(_ context.Context, id string) (*paste.Paste, error) {
	return c.data[id], nil
}
func (c *fakeCache) DeletePaste(_ context.Context, id string) error {
	delete(c.data, id)
	return nil
}
func (c *fakeCache) Allow(_ context.Context, _ string, _ int, _ time.Duration) (bool, error) {
	return true, nil
}

// --- tests ---

func newSvc() (*paste.Service, *fakeRepo, *fakeBlobs, *fakeCache) {
	repo := newFakeRepo()
	blobs := newFakeBlobs()
	cache := newFakeCache()
	return paste.NewService(repo, blobs, cache), repo, blobs, cache
}

func TestCreate_Public(t *testing.T) {
	svc, repo, blobs, _ := newSvc()
	ctx := context.Background()

	p, err := svc.Create(ctx, paste.CreateRequest{
		Content:    []byte("hello world"),
		Visibility: paste.VisibilityPublic,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if _, ok := repo.data[p.ID]; !ok {
		t.Error("paste not saved to repo")
	}
	if _, ok := blobs.data[p.ObjectKey]; !ok {
		t.Error("content not saved to blob store")
	}
}

func TestCreate_EmptyContent(t *testing.T) {
	svc, _, _, _ := newSvc()
	_, err := svc.Create(context.Background(), paste.CreateRequest{Content: []byte("")})
	if !errors.Is(err, paste.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreate_TooLarge(t *testing.T) {
	svc, _, _, _ := newSvc()
	big := make([]byte, paste.MaxSizeBytes+1)
	_, err := svc.Create(context.Background(), paste.CreateRequest{Content: big})
	if !errors.Is(err, paste.ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
}

func TestGet_NotFound(t *testing.T) {
	svc, _, _, _ := newSvc()
	_, _, err := svc.Get(context.Background(), "doesnotexist", "")
	if !errors.Is(err, paste.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGet_Expired(t *testing.T) {
	svc, _, _, _ := newSvc()
	ctx := context.Background()
	ttl := -1 * time.Second // already expired
	p, err := svc.Create(ctx, paste.CreateRequest{
		Content:    []byte("expiring"),
		Visibility: paste.VisibilityPublic,
		TTL:        &ttl,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.Get(ctx, p.ID, "")
	if !errors.Is(err, paste.ErrExpired) {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
}

func TestGet_PrivateForbidden(t *testing.T) {
	svc, _, _, _ := newSvc()
	ctx := context.Background()
	p, err := svc.Create(ctx, paste.CreateRequest{
		Content:    []byte("secret"),
		Visibility: paste.VisibilityPrivate,
		OwnerID:    "owner-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.Get(ctx, p.ID, "")
	if !errors.Is(err, paste.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestGet_PrivateOwnerAllowed(t *testing.T) {
	svc, _, _, _ := newSvc()
	ctx := context.Background()
	p, err := svc.Create(ctx, paste.CreateRequest{
		Content:    []byte("secret"),
		Visibility: paste.VisibilityPrivate,
		OwnerID:    "owner-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, content, err := svc.Get(ctx, p.ID, "owner-1")
	if err != nil {
		t.Fatalf("owner should be able to read: %v", err)
	}
	if string(content) != "secret" {
		t.Errorf("expected 'secret', got %q", string(content))
	}
}

func TestGet_BurnAfterRead(t *testing.T) {
	svc, repo, _, _ := newSvc()
	ctx := context.Background()
	p, err := svc.Create(ctx, paste.CreateRequest{
		Content:       []byte("burn me"),
		Visibility:    paste.VisibilityPublic,
		BurnAfterRead: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = svc.Get(ctx, p.ID, "")
	if err != nil {
		t.Fatalf("first read should succeed: %v", err)
	}

	// Give the async delete goroutine a moment to run.
	time.Sleep(50 * time.Millisecond)
	if _, ok := repo.data[p.ID]; ok {
		t.Error("burn-after-read paste should be deleted after first read")
	}
}

func TestDelete_OwnerOnly(t *testing.T) {
	svc, _, _, _ := newSvc()
	ctx := context.Background()
	p, err := svc.Create(ctx, paste.CreateRequest{
		Content:    []byte("owned"),
		Visibility: paste.VisibilityPublic,
		OwnerID:    "owner-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, p.ID, "other-user"); !errors.Is(err, paste.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	if err := svc.Delete(ctx, p.ID, "owner-1"); err != nil {
		t.Fatalf("owner delete should succeed: %v", err)
	}
}

func TestDelete_Anonymous(t *testing.T) {
	svc, _, _, _ := newSvc()
	ctx := context.Background()
	p, err := svc.Create(ctx, paste.CreateRequest{
		Content:    []byte("anon"),
		Visibility: paste.VisibilityPublic,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Anonymous pastes (no owner) can be deleted by anyone passing "".
	if err := svc.Delete(ctx, p.ID, ""); err != nil {
		t.Fatalf("anonymous delete should succeed: %v", err)
	}
}

func TestSweepExpired(t *testing.T) {
	svc, repo, _, _ := newSvc()
	ctx := context.Background()
	ttl := -1 * time.Second
	for i := 0; i < 3; i++ {
		if _, err := svc.Create(ctx, paste.CreateRequest{
			Content:    []byte("expiring"),
			Visibility: paste.VisibilityPublic,
			TTL:        &ttl,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// One live paste that should survive.
	live, _ := svc.Create(ctx, paste.CreateRequest{
		Content:    []byte("live"),
		Visibility: paste.VisibilityPublic,
	})

	n, err := svc.SweepExpired(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("expected 3 swept, got %d", n)
	}
	if _, ok := repo.data[live.ID]; !ok {
		t.Error("live paste should not be swept")
	}
}

func TestNewID_Unique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id, err := paste.NewID()
		if err != nil {
			t.Fatal(err)
		}
		if seen[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}
