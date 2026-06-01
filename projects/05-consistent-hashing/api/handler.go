// Package api implements HTTP transport for the consistent-hashing service.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/ankitsriv89/consistent-hashing/metrics"
	"github.com/ankitsriv89/consistent-hashing/ring"
	"github.com/ankitsriv89/consistent-hashing/store"
)

var (
	bufPool = sync.Pool{New: func() any { b := make([]byte, 0, 512); return &b }}

	healthBody = []byte(`{"status":"ok"}`)
)

// Handler is the HTTP handler for the consistent-hashing service.
type Handler struct {
	store  *store.Store
	log    *zap.Logger
	router *mux.Router
}

// New creates a Handler and registers all routes.
func New(s *store.Store, log *zap.Logger) *Handler {
	h := &Handler{store: s, log: log}
	r := mux.NewRouter()

	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/healthz", h.healthz).Methods(http.MethodGet)

	v1 := r.PathPrefix("/v1").Subrouter()
	v1.HandleFunc("/rings", h.listRings).Methods(http.MethodGet)
	v1.HandleFunc("/rings", h.createRing).Methods(http.MethodPost)
	v1.HandleFunc("/rings/{ring}", h.deleteRing).Methods(http.MethodDelete)
	v1.HandleFunc("/rings/{ring}/nodes", h.addNode).Methods(http.MethodPost)
	v1.HandleFunc("/rings/{ring}/nodes/{node}", h.removeNode).Methods(http.MethodDelete)
	v1.HandleFunc("/rings/{ring}/keys/{key}/owner", h.lookupKey).Methods(http.MethodGet)
	v1.HandleFunc("/rings/{ring}/keys/{key}/replicas", h.lookupReplicas).Methods(http.MethodGet)
	v1.HandleFunc("/rings/{ring}/stats", h.ringStats).Methods(http.MethodGet)
	v1.HandleFunc("/rings/{ring}/simulate", h.simulate).Methods(http.MethodGet)
	v1.HandleFunc("/rings/{ring}/vnodes", h.listVnodes).Methods(http.MethodGet)

	h.router = r
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

// --- handlers ---

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(healthBody)
}

func (h *Handler) listRings(w http.ResponseWriter, r *http.Request) {
	rings := h.store.ListRings()
	type item struct {
		ID       string `json:"id"`
		HashFn   string `json:"hash_fn"`
		Replicas int    `json:"replicas"`
		Nodes    int    `json:"node_count"`
		VNodes   int    `json:"vnode_count"`
		Version  uint64 `json:"version"`
	}
	out := make([]item, 0, len(rings))
	for _, m := range rings {
		s := m.Ring.Stats()
		out = append(out, item{
			ID:       m.ID,
			HashFn:   m.HashFn,
			Replicas: m.Replicas,
			Nodes:    s.NodeCount,
			VNodes:   s.VNodeCount,
			Version:  s.Version,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createRing(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string `json:"id"`
		HashFn   string `json:"hash_fn"`
		Replicas int    `json:"replicas"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if req.Replicas <= 0 {
		req.Replicas = 150
	}
	if req.HashFn == "" {
		req.HashFn = "sha256"
	}
	m, err := h.store.CreateRing(req.ID, req.HashFn, req.Replicas)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	metrics.RingOpsTotal.WithLabelValues(req.ID, "create_ring").Inc()
	h.log.Info("ring created", zap.String("ring_id", m.ID), zap.Int("replicas", m.Replicas))
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":       m.ID,
		"hash_fn":  m.HashFn,
		"replicas": m.Replicas,
	})
}

func (h *Handler) deleteRing(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["ring"]
	if !h.store.DeleteRing(id) {
		writeError(w, http.StatusNotFound, "ring not found")
		return
	}
	h.log.Info("ring deleted", zap.String("ring_id", id))
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) addNode(w http.ResponseWriter, r *http.Request) {
	ringID := mux.Vars(r)["ring"]
	m, err := h.store.GetRing(ringID)
	if err != nil {
		writeError(w, http.StatusNotFound, "ring not found")
		return
	}
	var req struct {
		ID      string `json:"id"`
		Weight  int    `json:"weight"`
		Address string `json:"address"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "node id is required")
		return
	}
	if req.Weight <= 0 {
		req.Weight = 1
	}
	stats := m.Ring.AddNode(ring.Node{ID: req.ID, Weight: req.Weight, Address: req.Address})
	metrics.RingOpsTotal.WithLabelValues(ringID, "add_node").Inc()
	metrics.NodeCount.WithLabelValues(ringID).Set(float64(stats.NodeCount))
	metrics.VNodeCount.WithLabelValues(ringID).Set(float64(stats.VNodeCount))
	metrics.RingStdDev.WithLabelValues(ringID).Set(stats.StdDev)
	if stats.KeyMovement != nil {
		metrics.KeyMovementPct.WithLabelValues(ringID).Set(stats.KeyMovement.MovedPct)
	}
	h.log.Info("node added", zap.String("ring_id", ringID), zap.String("node_id", req.ID),
		zap.Int("weight", req.Weight), zap.Float64("stddev", stats.StdDev))
	writeJSON(w, http.StatusCreated, stats)
}

func (h *Handler) removeNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ringID, nodeID := vars["ring"], vars["node"]
	m, err := h.store.GetRing(ringID)
	if err != nil {
		writeError(w, http.StatusNotFound, "ring not found")
		return
	}
	stats, ok := m.Ring.RemoveNode(nodeID)
	if !ok {
		writeError(w, http.StatusNotFound, "node not found")
		return
	}
	metrics.RingOpsTotal.WithLabelValues(ringID, "remove_node").Inc()
	metrics.NodeCount.WithLabelValues(ringID).Set(float64(stats.NodeCount))
	metrics.VNodeCount.WithLabelValues(ringID).Set(float64(stats.VNodeCount))
	metrics.RingStdDev.WithLabelValues(ringID).Set(stats.StdDev)
	if stats.KeyMovement != nil {
		metrics.KeyMovementPct.WithLabelValues(ringID).Set(stats.KeyMovement.MovedPct)
	}
	h.log.Info("node removed", zap.String("ring_id", ringID), zap.String("node_id", nodeID),
		zap.Float64("stddev", stats.StdDev))
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) lookupKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ringID, key := vars["ring"], vars["key"]
	m, err := h.store.GetRing(ringID)
	if err != nil {
		writeError(w, http.StatusNotFound, "ring not found")
		return
	}
	t0 := time.Now()
	owner := m.Ring.Lookup(key)
	metrics.LookupDuration.WithLabelValues(ringID).Observe(time.Since(t0).Seconds())
	if owner == "" {
		writeError(w, http.StatusServiceUnavailable, "ring is empty")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"key":     key,
		"ring_id": ringID,
		"owner":   owner,
		"version": strconv.FormatUint(m.Ring.Version(), 10),
	})
}

func (h *Handler) lookupReplicas(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ringID, key := vars["ring"], vars["key"]
	m, err := h.store.GetRing(ringID)
	if err != nil {
		writeError(w, http.StatusNotFound, "ring not found")
		return
	}
	n := 3
	if q := r.URL.Query().Get("n"); q != "" {
		if parsed, parseErr := strconv.Atoi(q); parseErr == nil && parsed > 0 {
			n = parsed
		}
	}
	replicas := m.Ring.LookupN(key, n)
	writeJSON(w, http.StatusOK, map[string]any{
		"key":      key,
		"ring_id":  ringID,
		"replicas": replicas,
		"version":  m.Ring.Version(),
	})
}

func (h *Handler) ringStats(w http.ResponseWriter, r *http.Request) {
	ringID := mux.Vars(r)["ring"]
	m, err := h.store.GetRing(ringID)
	if err != nil {
		writeError(w, http.StatusNotFound, "ring not found")
		return
	}
	writeJSON(w, http.StatusOK, m.Ring.Stats())
}

func (h *Handler) simulate(w http.ResponseWriter, r *http.Request) {
	ringID := mux.Vars(r)["ring"]
	m, err := h.store.GetRing(ringID)
	if err != nil {
		writeError(w, http.StatusNotFound, "ring not found")
		return
	}
	n := 10_000
	if q := r.URL.Query().Get("keys"); q != "" {
		if parsed, parseErr := strconv.Atoi(q); parseErr == nil && parsed > 0 && parsed <= 1_000_000 {
			n = parsed
		}
	}
	dist := m.Ring.SimulateKeys(n)
	writeJSON(w, http.StatusOK, map[string]any{
		"ring_id":    ringID,
		"total_keys": n,
		"version":    m.Ring.Version(),
		"distribution": dist,
	})
}

func (h *Handler) listVnodes(w http.ResponseWriter, r *http.Request) {
	ringID := mux.Vars(r)["ring"]
	m, err := h.store.GetRing(ringID)
	if err != nil {
		writeError(w, http.StatusNotFound, "ring not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ring_id": ringID,
		"version": m.Ring.Version(),
		"vnodes":  m.Ring.VNodes(),
	})
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, code int, v any) {
	buf := bufPool.Get().(*[]byte)
	*buf = (*buf)[:0]
	defer bufPool.Put(buf)
	enc, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	w.Write(enc)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return false
	}
	return true
}
