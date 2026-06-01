// Package api implements the HTTP transport layer for the unique-id-generator service.
//
// Routes:
//
//	POST /v1/ids/next         — generate one ID
//	POST /v1/ids/batch        — generate a batch of IDs (up to maxBatchSize)
//	GET  /v1/ids/{id}/inspect — decompose a Snowflake ID into its fields
//	GET  /v1/workers/health   — current worker ID and lease status
//	GET  /healthz             — liveness probe
//	GET  /metrics             — Prometheus exposition
//
// Memory optimisations (relevant when many services share one host):
//
//   - bufPool: a sync.Pool of *bytes.Buffer reused across JSON serialisations.
//     Each request borrows one buffer, encodes into it, writes it to the
//     ResponseWriter, then returns it to the pool.  This eliminates the
//     per-request heap allocation that json.NewEncoder(w) would cause.
//
//   - healthzBody: a package-level []byte written once at init.  The healthz
//     handler is called by Docker every 10 s × N projects; avoiding a
//     map[string]string allocation on each probe matters at scale.
//
//   - Handler.workerIDStr: the worker ID rendered to a string once at
//     construction rather than on every /next response.
//
//   - batchIDs reuses the []int64 slice returned by gen.Batch directly
//     instead of copying it, and builds the []string slice in one pass.
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/ankitsriv89/uniqueid/generator"
	"github.com/ankitsriv89/uniqueid/metrics"
)

const maxBatchSize = 1000

// bufPool holds *bytes.Buffer instances reused across requests to avoid
// per-request heap allocation for JSON serialisation.
var bufPool = sync.Pool{
	New: func() any { return bytes.NewBuffer(make([]byte, 0, 512)) },
}

// healthzBody is the fixed response for GET /healthz, built once at init.
var healthzBody = []byte(`{"status":"ok"}`)

// idGenerator is the subset of generator.Generator used by the handler.
// Defined as an interface so tests can inject a stub.
type idGenerator interface {
	Next() int64
	Batch(n int) []int64
	WorkerID() int64
}

// Handler holds all HTTP handler methods.
type Handler struct {
	gen          idGenerator
	log          *zap.Logger
	region       string
	workerIDStr  string // pre-rendered; avoids FormatInt on every /next call
}

// New creates a Handler backed by the given generator.
func New(gen idGenerator, region string, log *zap.Logger) *Handler {
	return &Handler{
		gen:         gen,
		log:         log,
		region:      region,
		workerIDStr: strconv.FormatInt(gen.WorkerID(), 10),
	}
}

var indexBody = []byte(`{"service":"unique-id-generator","version":"0.1.0","endpoints":[{"method":"POST","path":"/v1/ids/next"},{"method":"POST","path":"/v1/ids/batch"},{"method":"GET","path":"/v1/ids/{id}/inspect"},{"method":"GET","path":"/v1/workers/health"}]}`)

// Register wires all routes onto the router.
func (h *Handler) Register(r *mux.Router) {
	r.Handle("/metrics", metrics.Handler()).Methods(http.MethodGet)
	r.HandleFunc("/healthz", h.healthz).Methods(http.MethodGet)
	r.HandleFunc("/", h.index).Methods(http.MethodGet)

	v1 := r.PathPrefix("/v1").Subrouter()
	v1.HandleFunc("/ids/next", h.nextID).Methods(http.MethodPost)
	v1.HandleFunc("/ids/batch", h.batchIDs).Methods(http.MethodPost)
	v1.HandleFunc("/ids/{id}/inspect", h.inspectID).Methods(http.MethodGet)
	v1.HandleFunc("/workers/health", h.workerHealth).Methods(http.MethodGet)
}

// --- handlers ---

func (h *Handler) index(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(indexBody)
}

// healthz writes the static liveness response without any allocations.
func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(healthzBody)
}

type nextResponse struct {
	ID       int64  `json:"id"`
	IDString string `json:"id_string"` // string copy for JS clients that lose precision on int64
	WorkerID int64  `json:"worker_id"`
	Region   string `json:"region"`
}

// nextID generates a single Snowflake ID.
func (h *Handler) nextID(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	id := h.gen.Next()
	metrics.GenerationDuration.Observe(time.Since(start).Seconds())
	metrics.IDsGenerated.Add(1)

	h.writeJSON(w, http.StatusOK, nextResponse{
		ID:       id,
		IDString: strconv.FormatInt(id, 10),
		WorkerID: h.gen.WorkerID(),
		Region:   h.region,
	})
}

type batchRequest struct {
	Count int `json:"count"`
}

type batchResponse struct {
	IDs       []int64  `json:"ids"`
	IDStrings []string `json:"id_strings"` // string copies for JS clients
	Count     int      `json:"count"`
	WorkerID  int64    `json:"worker_id"`
}

// batchIDs generates up to maxBatchSize IDs in one call.
func (h *Handler) batchIDs(w http.ResponseWriter, r *http.Request) {
	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Count <= 0 || req.Count > maxBatchSize {
		h.writeError(w, http.StatusBadRequest, "count must be between 1 and 1000")
		return
	}

	start := time.Now()
	ids := h.gen.Batch(req.Count)
	metrics.GenerationDuration.Observe(time.Since(start).Seconds())
	metrics.IDsGenerated.Add(float64(len(ids)))

	// Build the string slice in one pass over the already-allocated ids slice.
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = strconv.FormatInt(id, 10)
	}

	h.writeJSON(w, http.StatusOK, batchResponse{
		IDs:       ids,
		IDStrings: strs,
		Count:     len(ids),
		WorkerID:  h.gen.WorkerID(),
	})
}

type inspectResponse struct {
	ID          string `json:"id"`
	TimestampMs int64  `json:"timestamp_ms"`
	Time        string `json:"time"`
	WorkerID    int64  `json:"worker_id"`
	Sequence    int64  `json:"sequence"`
}

// inspectID decomposes a Snowflake ID into its constituent fields.
func (h *Handler) inspectID(w http.ResponseWriter, r *http.Request) {
	raw := mux.Vars(r)["id"]
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		h.writeError(w, http.StatusBadRequest, "id must be a positive integer")
		return
	}

	ts, wid, seq := generator.Decompose(id)
	t := time.UnixMilli(ts).UTC()

	h.writeJSON(w, http.StatusOK, inspectResponse{
		ID:          raw,
		TimestampMs: ts,
		Time:        t.Format(time.RFC3339Nano),
		WorkerID:    wid,
		Sequence:    seq,
	})
}

type workerHealthResponse struct {
	WorkerID int64  `json:"worker_id"`
	Region   string `json:"region"`
	Status   string `json:"status"`
}

// workerHealth reports the current worker's identity and readiness.
func (h *Handler) workerHealth(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, http.StatusOK, workerHealthResponse{
		WorkerID: h.gen.WorkerID(),
		Region:   h.region,
		Status:   "ok",
	})
}

// --- helpers ---

// writeJSON serialises v into a pooled buffer, writes it to w, then returns
// the buffer to the pool. This avoids allocating a new io.Writer wrapper and
// internal buffer on every response.
func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()

	if err := json.NewEncoder(buf).Encode(v); err != nil {
		bufPool.Put(buf)
		h.log.Error("json encode error", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
	bufPool.Put(buf)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, map[string]string{"error": msg})
}
