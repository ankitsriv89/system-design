// Package gateway contains the core domain logic for the API gateway:
// route matching, API key authentication, and rate-limit policy decisions.
// No HTTP or database imports live here — only pure domain types and interfaces.
package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// --- Domain errors --------------------------------------------------------

var (
	ErrNotFound         = errors.New("gateway: not found")
	ErrUnauthorized     = errors.New("gateway: unauthorized")
	ErrForbidden        = errors.New("gateway: forbidden: scope not allowed")
	ErrRateLimited      = errors.New("gateway: rate limited")
	ErrUpstreamTimeout  = errors.New("gateway: upstream timeout")
	ErrPayloadTooLarge  = errors.New("gateway: payload too large")
	ErrBadRoute         = errors.New("gateway: bad route configuration")
)

// --- Data model -----------------------------------------------------------

// APIKey represents a credential issued to a client.
type APIKey struct {
	ID          string    `json:"id"`
	Owner       string    `json:"owner"`
	HashedKey   string    `json:"-"`
	Scopes      []string  `json:"scopes"`
	QuotaPerMin int       `json:"quota_per_min"` // 0 = unlimited
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

// Route defines one proxy rule: how to match an incoming path and where to forward it.
type Route struct {
	ID           string    `json:"id"`
	PathPrefix   string    `json:"path_prefix"`    // e.g. "/api/users"
	Upstream     string    `json:"upstream"`       // e.g. "http://user-svc:8080"
	StripPrefix  bool      `json:"strip_prefix"`   // strip PathPrefix before forwarding
	AuthRequired bool      `json:"auth_required"`
	RequiredScope string   `json:"required_scope"` // empty = any scope
	MaxBodyBytes int64     `json:"max_body_bytes"` // 0 = use gateway default
	TimeoutSecs  int       `json:"timeout_secs"`   // 0 = use gateway default
	Active       bool      `json:"active"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Decision records the outcome of one gateway evaluation for observability.
type Decision struct {
	RequestID  string
	RouteID    string
	KeyID      string
	Outcome    string // "allowed", "blocked_auth", "blocked_rate", "blocked_scope"
	StatusCode int
	LatencyMs  int64
}

// --- Interfaces (defined here, implemented by store) ----------------------

// KeyStore persists and retrieves API keys.
type KeyStore interface {
	CreateKey(ctx context.Context, key *APIKey) error
	GetKeyByID(ctx context.Context, id string) (*APIKey, error)
	// Authenticate validates a raw key value and returns the matching APIKey.
	Authenticate(ctx context.Context, rawKey string) (*APIKey, error)
	ListKeys(ctx context.Context) ([]*APIKey, error)
	RevokeKey(ctx context.Context, id string) error
}

// RouteStore persists and retrieves routing rules.
type RouteStore interface {
	UpsertRoute(ctx context.Context, route *Route) error
	GetRoute(ctx context.Context, id string) (*Route, error)
	ListRoutes(ctx context.Context) ([]*Route, error)
	DeleteRoute(ctx context.Context, id string) error
}

// RateLimiter checks and records quota usage for an API key.
type RateLimiter interface {
	// Allow returns true if the request is within quota, false if rate-limited.
	Allow(ctx context.Context, keyID string, limitPerMin int) (bool, error)
	// Remaining returns how many requests remain in the current window.
	Remaining(ctx context.Context, keyID string, limitPerMin int) (int, error)
}

// DecisionLog records gateway decisions for analytics.
type DecisionLog interface {
	Record(ctx context.Context, d *Decision) error
}

// --- Router: in-process route table with copy-on-write -------------------

// Router holds a snapshot of routes and matches incoming request paths.
// It is rebuilt atomically from the RouteStore on demand.
type Router struct {
	mu     sync.RWMutex
	routes []*Route
}

// Reload replaces the route table with a fresh snapshot.
func (rt *Router) Reload(routes []*Route) {
	active := make([]*Route, 0, len(routes))
	for _, r := range routes {
		if r.Active {
			active = append(active, r)
		}
	}
	rt.mu.Lock()
	rt.routes = active
	rt.mu.Unlock()
}

// Match finds the longest prefix match among active routes.
func (rt *Router) Match(path string) (*Route, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	var best *Route
	for _, r := range rt.routes {
		if strings.HasPrefix(path, r.PathPrefix) {
			if best == nil || len(r.PathPrefix) > len(best.PathPrefix) {
				best = r
			}
		}
	}
	return best, best != nil
}

// --- Gateway: orchestrates auth + routing + rate-limit -------------------

const (
	defaultMaxBodyBytes  = 4 << 20  // 4 MiB
	defaultTimeoutSecs   = 30
)

// Config holds gateway-wide defaults.
type Config struct {
	DefaultMaxBodyBytes int64
	DefaultTimeoutSecs  int
}

// Gateway is the central decision engine.
type Gateway struct {
	cfg     Config
	keys    KeyStore
	routes  RouteStore
	limiter RateLimiter
	log     DecisionLog
	router  *Router
}

// New constructs a Gateway. router must be pre-loaded via router.Reload.
func New(cfg Config, keys KeyStore, routes RouteStore, limiter RateLimiter, log DecisionLog, router *Router) *Gateway {
	if cfg.DefaultMaxBodyBytes == 0 {
		cfg.DefaultMaxBodyBytes = defaultMaxBodyBytes
	}
	if cfg.DefaultTimeoutSecs == 0 {
		cfg.DefaultTimeoutSecs = defaultTimeoutSecs
	}
	return &Gateway{cfg: cfg, keys: keys, routes: routes, limiter: limiter, log: log, router: router}
}

// EvalResult carries the outcome of a gateway evaluation.
type EvalResult struct {
	Route     *Route
	Key       *APIKey // nil when route does not require auth
	UpstreamURL string  // fully resolved target URL for the upstream
	Decision  *Decision
}

// Evaluate checks auth, scopes, and rate limits for an incoming request.
// It returns EvalResult on success or an error from the ErrXxx sentinel set.
func (g *Gateway) Evaluate(ctx context.Context, requestID string, r *http.Request) (*EvalResult, error) {
	start := time.Now()

	route, ok := g.router.Match(r.URL.Path)
	if !ok {
		return nil, ErrNotFound
	}

	maxBody := route.MaxBodyBytes
	if maxBody == 0 {
		maxBody = g.cfg.DefaultMaxBodyBytes
	}
	if r.ContentLength > maxBody {
		return nil, fmt.Errorf("%w: limit %d bytes", ErrPayloadTooLarge, maxBody)
	}

	var key *APIKey

	if route.AuthRequired {
		rawKey := extractBearerToken(r)
		if rawKey == "" {
			return nil, ErrUnauthorized
		}
		var err error
		key, err = g.keys.Authenticate(ctx, rawKey)
		if err != nil || !key.Active {
			return nil, ErrUnauthorized
		}

		if route.RequiredScope != "" && !hasScope(key.Scopes, route.RequiredScope) {
			d := &Decision{
				RequestID: requestID, RouteID: route.ID, KeyID: key.ID,
				Outcome: "blocked_scope", StatusCode: http.StatusForbidden,
				LatencyMs: time.Since(start).Milliseconds(),
			}
			_ = g.log.Record(ctx, d)
			return nil, ErrForbidden
		}

		if key.QuotaPerMin > 0 {
			allowed, err := g.limiter.Allow(ctx, key.ID, key.QuotaPerMin)
			if err != nil {
				// Fail open on limiter errors to avoid availability outage.
				allowed = true
			}
			if !allowed {
				d := &Decision{
					RequestID: requestID, RouteID: route.ID, KeyID: key.ID,
					Outcome: "blocked_rate", StatusCode: http.StatusTooManyRequests,
					LatencyMs: time.Since(start).Milliseconds(),
				}
				_ = g.log.Record(ctx, d)
				return nil, ErrRateLimited
			}
		}
	}

	upstreamURL := buildUpstreamURL(route, r)

	keyID := ""
	if key != nil {
		keyID = key.ID
	}
	d := &Decision{
		RequestID: requestID, RouteID: route.ID, KeyID: keyID,
		Outcome: "allowed", StatusCode: 0, // filled by proxy after upstream responds
		LatencyMs: time.Since(start).Milliseconds(),
	}
	_ = g.log.Record(ctx, d)

	return &EvalResult{Route: route, Key: key, UpstreamURL: upstreamURL, Decision: d}, nil
}

// ReloadRoutes refreshes the in-process router from the store.
func (g *Gateway) ReloadRoutes(ctx context.Context) error {
	routes, err := g.routes.ListRoutes(ctx)
	if err != nil {
		return fmt.Errorf("gateway: reload routes: %w", err)
	}
	g.router.Reload(routes)
	return nil
}

// --- helpers --------------------------------------------------------------

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	// Also accept X-API-Key header for convenience.
	return r.Header.Get("X-API-Key")
}

func hasScope(scopes []string, required string) bool {
	for _, s := range scopes {
		if s == required || s == "*" {
			return true
		}
	}
	return false
}

func buildUpstreamURL(route *Route, r *http.Request) string {
	path := r.URL.Path
	if route.StripPrefix {
		path = strings.TrimPrefix(path, route.PathPrefix)
		if path == "" {
			path = "/"
		}
	}
	upstream := strings.TrimRight(route.Upstream, "/")
	q := ""
	if r.URL.RawQuery != "" {
		q = "?" + r.URL.RawQuery
	}
	return upstream + path + q
}
