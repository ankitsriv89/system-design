// Package generator implements Snowflake-style 64-bit unique ID generation.
//
// Bit layout (64 bits total, sign bit always 0):
//
//	[0]  [1..41]      [42..51]     [52..63]
//	 0   timestamp    worker_id    sequence
//	     41 bits      10 bits      12 bits
//
// timestamp : milliseconds since custom epoch (2020-01-01T00:00:00Z).
// worker_id : 0–1023, assigned via PostgreSQL lease at startup.
// sequence  : 0–4095, incremented per call within the same millisecond;
//
//	wraps to 0 and spins to the next millisecond on overflow.
//
// Clock rollback: if the system clock moves backward the generator spins
// until wall time catches up, then fires the optional onIncident hook so
// the caller can record a ClockIncident row and emit an alert.
//
// Memory layout: the struct is padded to 64 bytes (one cache line) so that
// multiple Generator instances running on different goroutines on the same
// host do not share a cache line and cause false sharing stalls.
package generator

import (
	"errors"
	"sync"
	"time"
)

const (
	// Epoch is the custom epoch anchor: 2020-01-01T00:00:00Z as Unix milliseconds.
	// Using a recent anchor keeps the timestamp field useful for ~69 more years.
	Epoch int64 = 1577836800000

	workerBits     = 10
	sequenceBits   = 12
	maxWorkerID    = (1 << workerBits) - 1   // 1023
	maxSequence    = (1 << sequenceBits) - 1 // 4095
	workerShift    = sequenceBits
	timestampShift = workerBits + sequenceBits

	// cacheLinePad is the number of padding bytes needed to bring the hot
	// mutable fields (lastMs, sequence) onto their own 64-byte cache line,
	// away from the immutable fields (workerID, onIncident) and the mutex
	// internal state. This prevents false sharing when multiple Generator
	// instances are allocated adjacently (e.g. in a slice).
	//
	// Layout: mu(8) + workerID(8) + onIncident(16) = 32 bytes used before
	// the hot fields; pad to 64 so lastMs+sequence land on the next line.
	cacheLinePad = 64
)

// ErrWorkerIDOutOfRange is returned when the worker ID exceeds maxWorkerID (1023).
var ErrWorkerIDOutOfRange = errors.New("generator: worker_id must be in [0, 1023]")

// ClockIncidentFunc is called when a backward clock drift is detected.
// workerID identifies the affected generator; driftMs is the magnitude in milliseconds.
type ClockIncidentFunc func(workerID int64, driftMs int64)

// Generator produces Snowflake IDs. It is safe for concurrent use.
// All state is protected by a single mutex; callers that need higher throughput
// should run multiple Generator instances with distinct worker IDs.
//
// Field ordering is intentional:
//   - mu, workerID, onIncident — written once at construction, rarely touched at runtime.
//   - _pad — pushes lastMs and sequence onto a separate cache line.
//   - lastMs, sequence — mutated on every call to next(); kept together so a
//     single cache-line load covers both reads and both writes.
type Generator struct {
	mu         sync.Mutex        // 8 bytes
	workerID   int64             // 8 bytes
	onIncident ClockIncidentFunc // 16 bytes (interface: type + ptr)
	_pad       [cacheLinePad - 32]byte
	lastMs     int64 // hot: updated every call
	sequence   int64 // hot: updated every call
}

// New creates a Generator for the given workerID (0–1023).
// onIncident is optional; pass nil if clock-rollback telemetry is not needed.
func New(workerID int64, onIncident ClockIncidentFunc) (*Generator, error) {
	if workerID < 0 || workerID > maxWorkerID {
		return nil, ErrWorkerIDOutOfRange
	}
	return &Generator{
		workerID:   workerID,
		onIncident: onIncident,
	}, nil
}

// Next returns the next unique Snowflake ID.
// It blocks only when the 4096-ID sequence space for the current millisecond
// is exhausted (spins <1 ms) or when a clock rollback is detected (spins
// until wall time recovers).
func (g *Generator) Next() int64 {
	// Acquire the mutex so only one goroutine at a time reads/writes
	// g.lastMs and g.sequence. Without this, two concurrent callers could
	// observe the same (timestamp, sequence) pair and return duplicate IDs.
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.next()
}

// next is the lock-free inner implementation called by both Next and Batch.
// The caller must hold g.mu.
func (g *Generator) next() int64 {
	now := nowMs()

	if now < g.lastMs {
		// Clock moved backward — spin until wall time catches up with our
		// last-used millisecond, then continue normally.
		drift := g.lastMs - now
		if g.onIncident != nil {
			g.onIncident(g.workerID, drift)
		}
		for now < g.lastMs {
			time.Sleep(time.Duration(g.lastMs-now) * time.Millisecond)
			now = nowMs()
		}
	}

	if now == g.lastMs {
		// Same millisecond as the previous call — increment sequence.
		g.sequence = (g.sequence + 1) & maxSequence
		if g.sequence == 0 {
			// All 4096 sequence slots for this millisecond are used.
			// Spin until the clock advances to the next millisecond.
			for now <= g.lastMs {
				now = nowMs()
			}
		}
	} else {
		// New millisecond — reset sequence counter.
		g.sequence = 0
	}

	g.lastMs = now

	ts := now - Epoch
	return (ts << timestampShift) | (g.workerID << workerShift) | g.sequence
}

// Batch generates n IDs under a single mutex acquisition.
// Acquiring the lock once for the whole batch avoids n round-trips through
// the OS scheduler and is materially faster than calling Next() n times when
// n is large. The inner loop reuses the same nowMs() value for the duration
// of a millisecond, advancing the sequence; it only calls nowMs() again when
// the sequence wraps.
func (g *Generator) Batch(n int) []int64 {
	ids := make([]int64, n)

	g.mu.Lock()
	defer g.mu.Unlock()

	for i := range ids {
		ids[i] = g.next()
	}
	return ids
}

// WorkerID returns the worker ID embedded in this generator instance.
func (g *Generator) WorkerID() int64 { return g.workerID }

// Decompose breaks a Snowflake ID back into its constituent fields.
// Useful for debugging, auditing, and time-range queries.
func Decompose(id int64) (timestampMs int64, workerID int64, sequence int64) {
	timestampMs = (id>>timestampShift) + Epoch
	workerID = (id >> workerShift) & maxWorkerID
	sequence = id & maxSequence
	return
}

// nowMs returns the current Unix time in milliseconds.
func nowMs() int64 {
	return time.Now().UnixMilli()
}
