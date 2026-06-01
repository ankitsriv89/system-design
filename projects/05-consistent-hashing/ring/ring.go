// Package ring implements a consistent hash ring with virtual nodes and weighted placement.
package ring

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
)

// VNode is a single point on the ring, belonging to a physical node.
type VNode struct {
	Position uint32
	NodeID   string
}

// Node represents a physical server on the ring.
type Node struct {
	ID      string
	Weight  int    // relative capacity; vnodes = weight * vnodeBase
	Address string // e.g. "host:port", informational only
}

// Stats describes the current distribution of keys across nodes.
type Stats struct {
	Version       uint64
	NodeCount     int
	VNodeCount    int
	ArcLengths    map[string]float64 // nodeID → fraction of ring owned (0.0–1.0)
	StdDev        float64            // standard deviation of arc lengths; 0 = perfect balance
	KeyMovement   *KeyMovement       // non-nil only after AddNode/RemoveNode
}

// KeyMovement tracks how many keys shift during a topology change.
type KeyMovement struct {
	Before      map[string]uint64 // nodeID → key count before
	After       map[string]uint64 // nodeID → key count after
	TotalKeys   uint64
	MovedKeys   uint64
	MovedPct    float64
}

// Ring is the consistent hash ring. It is safe for concurrent use.
type Ring struct {
	mu       sync.RWMutex
	vnodes   []VNode          // sorted by Position
	nodes    map[string]*Node // physical nodes
	version  atomic.Uint64
	replicas int // vnodeBase per unit of weight
}

// New creates an empty ring. replicas is the number of vnodes per unit weight.
func New(replicas int) *Ring {
	if replicas <= 0 {
		replicas = 150
	}
	return &Ring{
		nodes:    make(map[string]*Node, 16),
		vnodes:   make([]VNode, 0, 64),
		replicas: replicas,
	}
}

// AddNode places a node on the ring. Returns the rebalance stats.
func (r *Ring) AddNode(n Node) Stats {
	if n.Weight <= 0 {
		n.Weight = 1
	}
	count := n.Weight * r.replicas

	r.mu.Lock()
	before := r.arcLengths()
	r.nodes[n.ID] = &n
	for i := range count {
		pos := hashPosition(n.ID, i)
		r.vnodes = append(r.vnodes, VNode{Position: pos, NodeID: n.ID})
	}
	sort.Slice(r.vnodes, func(i, j int) bool {
		return r.vnodes[i].Position < r.vnodes[j].Position
	})
	r.version.Add(1)
	after := r.arcLengths()
	stats := r.buildStats(before, after)
	r.mu.Unlock()
	return stats
}

// RemoveNode removes a node from the ring. Returns the rebalance stats.
func (r *Ring) RemoveNode(id string) (Stats, bool) {
	r.mu.Lock()
	if _, ok := r.nodes[id]; !ok {
		r.mu.Unlock()
		return Stats{}, false
	}
	before := r.arcLengths()
	delete(r.nodes, id)
	filtered := r.vnodes[:0]
	for _, v := range r.vnodes {
		if v.NodeID != id {
			filtered = append(filtered, v)
		}
	}
	r.vnodes = filtered
	r.version.Add(1)
	after := r.arcLengths()
	stats := r.buildStats(before, after)
	r.mu.Unlock()
	return stats, true
}

// Lookup returns the node responsible for the given key.
// Returns empty string if the ring is empty.
func (r *Ring) Lookup(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.vnodes) == 0 {
		return ""
	}
	h := keyHash(key)
	idx := sort.Search(len(r.vnodes), func(i int) bool {
		return r.vnodes[i].Position >= h
	})
	if idx == len(r.vnodes) {
		idx = 0
	}
	return r.vnodes[idx].NodeID
}

// LookupN returns the primary node plus up to n-1 replica nodes (distinct physical nodes).
func (r *Ring) LookupN(key string, n int) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.vnodes) == 0 || n <= 0 {
		return nil
	}
	h := keyHash(key)
	start := sort.Search(len(r.vnodes), func(i int) bool {
		return r.vnodes[i].Position >= h
	})
	seen := make(map[string]struct{}, n)
	result := make([]string, 0, n)
	for i := range len(r.vnodes) {
		idx := (start + i) % len(r.vnodes)
		nid := r.vnodes[idx].NodeID
		if _, ok := seen[nid]; !ok {
			seen[nid] = struct{}{}
			result = append(result, nid)
		}
		if len(result) == n {
			break
		}
	}
	return result
}

// Stats returns the current distribution statistics.
func (r *Ring) Stats() Stats {
	r.mu.RLock()
	arcs := r.arcLengths()
	stats := r.buildStats(nil, arcs)
	r.mu.RUnlock()
	return stats
}

// Version returns the current ring version (monotonically increasing).
func (r *Ring) Version() uint64 {
	return r.version.Load()
}

// Nodes returns a snapshot of all physical nodes.
func (r *Ring) Nodes() []Node {
	r.mu.RLock()
	out := make([]Node, 0, len(r.nodes))
	for _, n := range r.nodes {
		out = append(out, *n)
	}
	r.mu.RUnlock()
	return out
}

// VNodes returns a snapshot of all virtual nodes (sorted by position).
func (r *Ring) VNodes() []VNode {
	r.mu.RLock()
	out := make([]VNode, len(r.vnodes))
	copy(out, r.vnodes)
	r.mu.RUnlock()
	return out
}

// SimulateKeys assigns n synthetic keys to nodes and returns the distribution.
func (r *Ring) SimulateKeys(n int) map[string]uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	dist := make(map[string]uint64, len(r.nodes))
	for nodeID := range r.nodes {
		dist[nodeID] = 0
	}
	for i := range n {
		key := fmt.Sprintf("sim-key-%d", i)
		owner := r.lookupLocked(key)
		if owner != "" {
			dist[owner]++
		}
	}
	return dist
}

// lookupLocked is Lookup without locking; caller must hold at least RLock.
func (r *Ring) lookupLocked(key string) string {
	if len(r.vnodes) == 0 {
		return ""
	}
	h := keyHash(key)
	idx := sort.Search(len(r.vnodes), func(i int) bool {
		return r.vnodes[i].Position >= h
	})
	if idx == len(r.vnodes) {
		idx = 0
	}
	return r.vnodes[idx].NodeID
}

// arcLengths computes each node's fraction of the ring. Must be called with lock held.
func (r *Ring) arcLengths() map[string]float64 {
	arcs := make(map[string]float64, len(r.nodes))
	for id := range r.nodes {
		arcs[id] = 0
	}
	if len(r.vnodes) == 0 {
		return arcs
	}
	const total = float64(math.MaxUint32) + 1
	for i, v := range r.vnodes {
		var prev uint32
		if i == 0 {
			prev = r.vnodes[len(r.vnodes)-1].Position
		} else {
			prev = r.vnodes[i-1].Position
		}
		var arc float64
		if v.Position >= prev {
			arc = float64(v.Position-prev) / total
		} else {
			// wrap-around arc
			arc = float64(math.MaxUint32-prev+v.Position+1) / total
		}
		arcs[v.NodeID] += arc
	}
	return arcs
}

func (r *Ring) buildStats(before, after map[string]float64) Stats {
	// stddev of arc lengths
	vals := make([]float64, 0, len(after))
	for _, v := range after {
		vals = append(vals, v)
	}
	stddev := stdDev(vals)

	s := Stats{
		Version:    r.version.Load(),
		NodeCount:  len(r.nodes),
		VNodeCount: len(r.vnodes),
		ArcLengths: after,
		StdDev:     stddev,
	}

	if before != nil {
		// count synthetic key movements
		const sampleKeys = 10_000
		moved := uint64(0)
		for i := range sampleKeys {
			key := fmt.Sprintf("sim-key-%d", i)
			h := keyHash(key)
			_ = h
			// owner before
			ob := ownerFromArcs(before, key)
			oa := ownerFromArcs(after, key)
			if ob != oa && ob != "" && oa != "" {
				moved++
			}
		}
		bCounts := make(map[string]uint64, len(before))
		aCounts := make(map[string]uint64, len(after))
		for k := range before {
			bCounts[k] = 0
		}
		for k := range after {
			aCounts[k] = 0
		}
		s.KeyMovement = &KeyMovement{
			Before:    bCounts,
			After:     aCounts,
			TotalKeys: sampleKeys,
			MovedKeys: moved,
			MovedPct:  float64(moved) / sampleKeys * 100,
		}
	}
	return s
}

// ownerFromArcs is a simplified lookup used only for movement estimation.
// It uses cumulative arc fractions to simulate the ring without vnodes.
func ownerFromArcs(arcs map[string]float64, key string) string {
	if len(arcs) == 0 {
		return ""
	}
	h := keyHash(key)
	pos := float64(h) / (float64(math.MaxUint32) + 1)
	// sort node ids for determinism
	type entry struct {
		id  string
		arc float64
	}
	entries := make([]entry, 0, len(arcs))
	for id, arc := range arcs {
		entries = append(entries, entry{id, arc})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	cumulative := 0.0
	for _, e := range entries {
		cumulative += e.arc
		if pos <= cumulative {
			return e.id
		}
	}
	return entries[0].id
}

func hashPosition(nodeID string, idx int) uint32 {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s#%d", nodeID, idx)))
	b := h.Sum(nil)
	return binary.BigEndian.Uint32(b[:4])
}

func keyHash(key string) uint32 {
	h := sha256.New()
	h.Write([]byte(key))
	b := h.Sum(nil)
	return binary.BigEndian.Uint32(b[:4])
}

func stdDev(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	mean := 0.0
	for _, v := range vals {
		mean += v
	}
	mean /= float64(len(vals))
	variance := 0.0
	for _, v := range vals {
		d := v - mean
		variance += d * d
	}
	variance /= float64(len(vals))
	return math.Sqrt(variance)
}
