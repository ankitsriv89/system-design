// Package balancer implements the core load-balancing domain: backend pool
// management, routing algorithms, and active health checking.
package balancer

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Status represents the health state of a backend.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDraining  Status = "draining"
)

// Algorithm selects the routing strategy.
type Algorithm string

const (
	AlgoRoundRobin       Algorithm = "round_robin"
	AlgoLeastConnections Algorithm = "least_connections"
	AlgoWeightedRR       Algorithm = "weighted_round_robin"
	AlgoRandom           Algorithm = "random"
)

// Backend is one upstream server inside a service pool.
// Immutable fields are grouped first; mutable counters follow on a separate
// cache line to avoid false sharing when many goroutines update ActiveConns.
type Backend struct {
	// --- immutable after construction ---
	URL     string
	Service string
	Weight  int
	// pad to 64 bytes before mutable fields
	_pad [40]byte //nolint:unused

	// --- mutable ---
	ActiveConns int64  // atomic
	TotalConns  uint64 // atomic, monotonically increasing
	mu          sync.RWMutex
	status      Status
	latencyEWMA float64 // exponential moving average of response latency ms
}

// GetStatus returns the backend's health status safely.
func (b *Backend) GetStatus() Status {
	b.mu.RLock()
	s := b.status
	b.mu.RUnlock()
	return s
}

// SetStatus updates the backend health status.
func (b *Backend) SetStatus(s Status) {
	b.mu.Lock()
	b.status = s
	b.mu.Unlock()
}

// RecordLatency updates the EWMA latency estimate (α = 0.2).
func (b *Backend) RecordLatency(ms float64) {
	b.mu.Lock()
	const alpha = 0.2
	if b.latencyEWMA == 0 {
		b.latencyEWMA = ms
	} else {
		b.latencyEWMA = alpha*ms + (1-alpha)*b.latencyEWMA
	}
	b.mu.Unlock()
}

// LatencyEWMA returns the smoothed latency estimate in milliseconds.
func (b *Backend) LatencyEWMA() float64 {
	b.mu.RLock()
	v := b.latencyEWMA
	b.mu.RUnlock()
	return v
}

// HealthEvent is a single observation from a health check.
type HealthEvent struct {
	Service    string
	BackendURL string
	Status     Status
	LatencyMs  int
	Time       time.Time
}

// Pool manages all backends for a named service.
type Pool struct {
	mu        sync.RWMutex
	backends  []*Backend
	algorithm Algorithm
	rrCounter uint64 // round-robin position, accessed atomically
}

// NewPool creates an empty pool with the given algorithm.
func NewPool(algo Algorithm) *Pool {
	return &Pool{algorithm: algo}
}

// Add inserts a backend. No-op if URL already registered for this service.
func (p *Pool) Add(b *Backend) {
	p.mu.Lock()
	for _, existing := range p.backends {
		if existing.URL == b.URL {
			p.mu.Unlock()
			return
		}
	}
	p.backends = append(p.backends, b)
	p.mu.Unlock()
}

// Remove marks a backend for removal and eliminates it from the pool.
func (p *Pool) Remove(url string) {
	p.mu.Lock()
	out := p.backends[:0]
	for _, b := range p.backends {
		if b.URL != url {
			out = append(out, b)
		}
	}
	p.backends = out
	p.mu.Unlock()
}

// Backends returns a snapshot of all backends (any status).
func (p *Pool) Backends() []*Backend {
	p.mu.RLock()
	snap := make([]*Backend, len(p.backends))
	copy(snap, p.backends)
	p.mu.RUnlock()
	return snap
}

// healthy returns only live backends.
func (p *Pool) healthy() []*Backend {
	p.mu.RLock()
	out := make([]*Backend, 0, len(p.backends))
	for _, b := range p.backends {
		if b.GetStatus() == StatusHealthy {
			out = append(out, b)
		}
	}
	p.mu.RUnlock()
	return out
}

// Next picks the next backend according to the configured algorithm.
// Returns nil if no healthy backends are available.
func (p *Pool) Next() *Backend {
	switch p.algorithm {
	case AlgoLeastConnections:
		return p.leastConnections()
	case AlgoWeightedRR:
		return p.weightedRoundRobin()
	default:
		return p.roundRobin()
	}
}

func (p *Pool) roundRobin() *Backend {
	healthy := p.healthy()
	n := uint64(len(healthy))
	if n == 0 {
		return nil
	}
	idx := atomic.AddUint64(&p.rrCounter, 1) % n
	return healthy[idx]
}

func (p *Pool) leastConnections() *Backend {
	healthy := p.healthy()
	if len(healthy) == 0 {
		return nil
	}
	best := healthy[0]
	bestScore := float64(atomic.LoadInt64(&best.ActiveConns)) + best.LatencyEWMA()/1000
	for _, b := range healthy[1:] {
		score := float64(atomic.LoadInt64(&b.ActiveConns)) + b.LatencyEWMA()/1000
		if score < bestScore {
			best = b
			bestScore = score
		}
	}
	return best
}

func (p *Pool) weightedRoundRobin() *Backend {
	healthy := p.healthy()
	if len(healthy) == 0 {
		return nil
	}
	total := 0
	for _, b := range healthy {
		total += b.Weight
	}
	if total == 0 {
		return p.roundRobin()
	}
	pos := int(atomic.AddUint64(&p.rrCounter, 1)) % total
	running := 0
	for _, b := range healthy {
		running += b.Weight
		if pos < running {
			return b
		}
	}
	return healthy[len(healthy)-1]
}

// SetAlgorithm changes the routing algorithm on the pool.
func (p *Pool) SetAlgorithm(a Algorithm) {
	p.mu.Lock()
	p.algorithm = a
	p.mu.Unlock()
}

// Algorithm returns the current routing algorithm.
func (p *Pool) Algorithm() Algorithm {
	p.mu.RLock()
	a := p.algorithm
	p.mu.RUnlock()
	return a
}

// HealthChecker runs periodic active health checks against backends.
type HealthChecker struct {
	client   *http.Client
	interval time.Duration
	timeout  time.Duration
	log      *zap.Logger
	events   chan<- HealthEvent
}

// NewHealthChecker constructs a checker. events is the channel where health
// observations are published; caller must drain it.
func NewHealthChecker(
	interval, timeout time.Duration,
	events chan<- HealthEvent,
	log *zap.Logger,
) *HealthChecker {
	return &HealthChecker{
		client:   &http.Client{Timeout: timeout},
		interval: interval,
		timeout:  timeout,
		log:      log,
		events:   events,
	}
}

// Run starts health-check loops for all pools. Stops when ctx is cancelled.
// One goroutine per pool; each goroutine owns its own ticker.
func (hc *HealthChecker) Run(ctx context.Context, pools map[string]*Pool) {
	for service, pool := range pools {
		go hc.checkLoop(ctx, service, pool)
	}
}

// WatchPool starts a health-check loop for a single pool. Call after
// dynamically adding a new service.
func (hc *HealthChecker) WatchPool(ctx context.Context, service string, pool *Pool) {
	go hc.checkLoop(ctx, service, pool)
}

func (hc *HealthChecker) checkLoop(ctx context.Context, service string, pool *Pool) {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, b := range pool.Backends() {
				go hc.probe(service, b)
			}
		}
	}
}

func (hc *HealthChecker) probe(service string, b *Backend) {
	url := b.URL + "/healthz"
	start := time.Now()
	resp, err := hc.client.Get(url) //nolint:noctx
	latencyMs := int(time.Since(start).Milliseconds())

	var newStatus Status
	if err != nil || resp.StatusCode >= 500 {
		newStatus = StatusUnhealthy
		hc.log.Warn("backend unhealthy",
			zap.String("service", service),
			zap.String("url", b.URL),
			zap.Error(err),
		)
	} else {
		newStatus = StatusHealthy
	}
	if resp != nil {
		resp.Body.Close()
	}

	prev := b.GetStatus()
	b.SetStatus(newStatus)
	b.RecordLatency(float64(latencyMs))

	if prev != newStatus {
		hc.log.Info("backend status changed",
			zap.String("service", service),
			zap.String("url", b.URL),
			zap.String("prev", string(prev)),
			zap.String("new", string(newStatus)),
		)
	}

	select {
	case hc.events <- HealthEvent{
		Service:    service,
		BackendURL: b.URL,
		Status:     newStatus,
		LatencyMs:  latencyMs,
		Time:       time.Now(),
	}:
	default:
		// drop if consumer is slow — health events are best-effort
	}
}

// LoadBalancer is the top-level domain object. It owns all service pools and
// the health checker.
type LoadBalancer struct {
	mu      sync.RWMutex
	pools   map[string]*Pool
	checker *HealthChecker
	log     *zap.Logger
	Events  chan HealthEvent
}

// New creates a LoadBalancer.
func New(checkInterval, checkTimeout time.Duration, log *zap.Logger) *LoadBalancer {
	events := make(chan HealthEvent, 512)
	return &LoadBalancer{
		pools:   make(map[string]*Pool),
		checker: NewHealthChecker(checkInterval, checkTimeout, events, log),
		log:     log,
		Events:  events,
	}
}

// Start launches health-check goroutines. Must be called once.
func (lb *LoadBalancer) Start(ctx context.Context) {
	lb.mu.RLock()
	pools := make(map[string]*Pool, len(lb.pools))
	for k, v := range lb.pools {
		pools[k] = v
	}
	lb.mu.RUnlock()
	lb.checker.Run(ctx, pools)
}

// AddBackend registers a backend in a service pool. Creates the pool if new.
func (lb *LoadBalancer) AddBackend(ctx context.Context, service, url string, weight int) {
	lb.mu.Lock()
	pool, exists := lb.pools[service]
	if !exists {
		pool = NewPool(AlgoRoundRobin)
		lb.pools[service] = pool
	}
	lb.mu.Unlock()

	b := &Backend{
		URL:     url,
		Service: service,
		Weight:  weight,
		status:  StatusHealthy,
	}
	pool.Add(b)

	if !exists {
		lb.checker.WatchPool(ctx, service, pool)
	}
}

// RemoveBackend removes a backend from a service pool.
func (lb *LoadBalancer) RemoveBackend(service, url string) error {
	lb.mu.RLock()
	pool, ok := lb.pools[service]
	lb.mu.RUnlock()
	if !ok {
		return fmt.Errorf("balancer: service %q not found", service)
	}
	pool.Remove(url)
	return nil
}

// SetAlgorithm changes the routing algorithm for a service pool.
func (lb *LoadBalancer) SetAlgorithm(service string, algo Algorithm) error {
	lb.mu.RLock()
	pool, ok := lb.pools[service]
	lb.mu.RUnlock()
	if !ok {
		return fmt.Errorf("balancer: service %q not found", service)
	}
	pool.SetAlgorithm(algo)
	return nil
}

// Next picks the next backend for a service.
func (lb *LoadBalancer) Next(service string) (*Backend, error) {
	lb.mu.RLock()
	pool, ok := lb.pools[service]
	lb.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("balancer: service %q not found", service)
	}
	b := pool.Next()
	if b == nil {
		return nil, fmt.Errorf("balancer: no healthy backends for %q", service)
	}
	return b, nil
}

// Services returns a snapshot of all registered service names.
func (lb *LoadBalancer) Services() []string {
	lb.mu.RLock()
	out := make([]string, 0, len(lb.pools))
	for k := range lb.pools {
		out = append(out, k)
	}
	lb.mu.RUnlock()
	return out
}

// ServicePool returns the pool for a service (nil if not found).
func (lb *LoadBalancer) ServicePool(service string) *Pool {
	lb.mu.RLock()
	p := lb.pools[service]
	lb.mu.RUnlock()
	return p
}

// BackendStats bundles observable state for the API / UI.
type BackendStats struct {
	URL         string  `json:"url"`
	Service     string  `json:"service"`
	Status      Status  `json:"status"`
	Weight      int     `json:"weight"`
	ActiveConns int64   `json:"active_conns"`
	TotalConns  uint64  `json:"total_conns"`
	LatencyEWMA float64 `json:"latency_ewma_ms"`
}

// Stats returns a flat snapshot of all backends across all services.
func (lb *LoadBalancer) Stats() []BackendStats {
	lb.mu.RLock()
	pools := make(map[string]*Pool, len(lb.pools))
	for k, v := range lb.pools {
		pools[k] = v
	}
	lb.mu.RUnlock()

	var out []BackendStats
	for _, pool := range pools {
		for _, b := range pool.Backends() {
			out = append(out, BackendStats{
				URL:         b.URL,
				Service:     b.Service,
				Status:      b.GetStatus(),
				Weight:      b.Weight,
				ActiveConns: atomic.LoadInt64(&b.ActiveConns),
				TotalConns:  atomic.LoadUint64(&b.TotalConns),
				LatencyEWMA: math.Round(b.LatencyEWMA()*10) / 10,
			})
		}
	}
	return out
}
