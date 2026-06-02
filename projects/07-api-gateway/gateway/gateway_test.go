// Package gateway tests core routing and evaluation logic.
package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- stub implementations -------------------------------------------------

type stubKeyStore struct {
	keys map[string]*APIKey // hashed key -> APIKey
}

func (s *stubKeyStore) CreateKey(_ context.Context, k *APIKey) error {
	s.keys[k.HashedKey] = k
	return nil
}
func (s *stubKeyStore) GetKeyByID(_ context.Context, id string) (*APIKey, error) {
	for _, k := range s.keys {
		if k.ID == id {
			return k, nil
		}
	}
	return nil, ErrNotFound
}
func (s *stubKeyStore) Authenticate(_ context.Context, raw string) (*APIKey, error) {
	// stub: raw key == HashedKey for simplicity
	if k, ok := s.keys[raw]; ok {
		return k, nil
	}
	return nil, ErrUnauthorized
}
func (s *stubKeyStore) ListKeys(_ context.Context) ([]*APIKey, error) {
	out := make([]*APIKey, 0, len(s.keys))
	for _, k := range s.keys {
		out = append(out, k)
	}
	return out, nil
}
func (s *stubKeyStore) RevokeKey(_ context.Context, id string) error { return nil }

type stubRouteStore struct{}

func (s *stubRouteStore) UpsertRoute(_ context.Context, _ *Route) error  { return nil }
func (s *stubRouteStore) GetRoute(_ context.Context, _ string) (*Route, error) {
	return nil, ErrNotFound
}
func (s *stubRouteStore) ListRoutes(_ context.Context) ([]*Route, error) { return nil, nil }
func (s *stubRouteStore) DeleteRoute(_ context.Context, _ string) error  { return nil }

type stubLimiter struct{ allow bool }

func (s *stubLimiter) Allow(_ context.Context, _ string, _ int) (bool, error) {
	return s.allow, nil
}
func (s *stubLimiter) Remaining(_ context.Context, _ string, _ int) (int, error) {
	if s.allow {
		return 10, nil
	}
	return 0, nil
}

type stubLog struct{}

func (s *stubLog) Record(_ context.Context, _ *Decision) error { return nil }

// --- helpers --------------------------------------------------------------

func newTestGateway(ks *stubKeyStore, limiter RateLimiter) *Gateway {
	rs := &stubRouteStore{}
	router := &Router{}
	return New(Config{}, ks, rs, limiter, &stubLog{}, router)
}

func addRoute(gw *Gateway, r *Route) {
	gw.router.Reload([]*Route{r})
}

// --- tests ----------------------------------------------------------------

func TestRouterLongestPrefixWins(t *testing.T) {
	rt := &Router{}
	rt.Reload([]*Route{
		{PathPrefix: "/api", Active: true, ID: "short"},
		{PathPrefix: "/api/users", Active: true, ID: "long"},
	})
	match, ok := rt.Match("/api/users/42")
	if !ok {
		t.Fatal("expected match")
	}
	if match.ID != "long" {
		t.Fatalf("expected 'long', got %q", match.ID)
	}
}

func TestRouterNoMatchReturnsNotFound(t *testing.T) {
	rt := &Router{}
	rt.Reload([]*Route{{PathPrefix: "/api", Active: true, ID: "a"}})
	_, ok := rt.Match("/other/path")
	if ok {
		t.Fatal("expected no match")
	}
}

func TestEvaluateAllowedNoAuth(t *testing.T) {
	gw := newTestGateway(&stubKeyStore{keys: map[string]*APIKey{}}, &stubLimiter{allow: true})
	addRoute(gw, &Route{ID: "r1", PathPrefix: "/pub", Upstream: "http://svc:80", Active: true})

	req := httptest.NewRequest(http.MethodGet, "/pub/resource", nil)
	res, err := gw.Evaluate(context.Background(), "req-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.UpstreamURL != "http://svc:80/pub/resource" {
		t.Fatalf("unexpected upstream URL: %s", res.UpstreamURL)
	}
}

func TestEvaluateAuthRequired_MissingToken(t *testing.T) {
	gw := newTestGateway(&stubKeyStore{keys: map[string]*APIKey{}}, &stubLimiter{allow: true})
	addRoute(gw, &Route{ID: "r2", PathPrefix: "/secure", Upstream: "http://svc:80", Active: true, AuthRequired: true})

	req := httptest.NewRequest(http.MethodGet, "/secure/data", nil)
	_, err := gw.Evaluate(context.Background(), "req-2", req)
	if err != ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestEvaluateAuthRequired_ValidKey(t *testing.T) {
	ks := &stubKeyStore{keys: map[string]*APIKey{
		"secret-token": {ID: "k1", Owner: "alice", HashedKey: "secret-token", Scopes: []string{"read"}, Active: true, QuotaPerMin: 100},
	}}
	gw := newTestGateway(ks, &stubLimiter{allow: true})
	addRoute(gw, &Route{ID: "r3", PathPrefix: "/secure", Upstream: "http://svc:80", Active: true, AuthRequired: true, RequiredScope: "read"})

	req := httptest.NewRequest(http.MethodGet, "/secure/data", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	res, err := gw.Evaluate(context.Background(), "req-3", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Key == nil || res.Key.ID != "k1" {
		t.Fatal("expected key k1 in result")
	}
}

func TestEvaluateRateLimited(t *testing.T) {
	ks := &stubKeyStore{keys: map[string]*APIKey{
		"tok": {ID: "k2", Owner: "bob", HashedKey: "tok", Scopes: []string{"*"}, Active: true, QuotaPerMin: 10},
	}}
	gw := newTestGateway(ks, &stubLimiter{allow: false})
	addRoute(gw, &Route{ID: "r4", PathPrefix: "/svc", Upstream: "http://svc:80", Active: true, AuthRequired: true})

	req := httptest.NewRequest(http.MethodGet, "/svc/x", nil)
	req.Header.Set("X-API-Key", "tok")
	_, err := gw.Evaluate(context.Background(), "req-4", req)
	if err != ErrRateLimited {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
}

func TestEvaluateForbiddenScope(t *testing.T) {
	ks := &stubKeyStore{keys: map[string]*APIKey{
		"tok2": {ID: "k3", Owner: "carol", HashedKey: "tok2", Scopes: []string{"read"}, Active: true, QuotaPerMin: 0},
	}}
	gw := newTestGateway(ks, &stubLimiter{allow: true})
	addRoute(gw, &Route{ID: "r5", PathPrefix: "/admin", Upstream: "http://admin:80", Active: true, AuthRequired: true, RequiredScope: "admin"})

	req := httptest.NewRequest(http.MethodGet, "/admin/panel", nil)
	req.Header.Set("X-API-Key", "tok2")
	_, err := gw.Evaluate(context.Background(), "req-5", req)
	if err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestStripPrefix(t *testing.T) {
	rt := &Route{PathPrefix: "/svc", Upstream: "http://backend:9000", StripPrefix: true}
	req := httptest.NewRequest(http.MethodGet, "/svc/resource?q=1", nil)
	url := buildUpstreamURL(rt, req)
	if url != "http://backend:9000/resource?q=1" {
		t.Fatalf("unexpected URL after strip: %s", url)
	}
}

func BenchmarkRouterMatch(b *testing.B) {
	rt := &Router{}
	routes := make([]*Route, 20)
	for i := range routes {
		routes[i] = &Route{PathPrefix: "/api/service" + string(rune('a'+i)), Active: true, ID: string(rune('a' + i))}
	}
	rt.Reload(routes)
	b.ResetTimer()
	for b.Loop() {
		rt.Match("/api/servicet/resource/123")
	}
}
