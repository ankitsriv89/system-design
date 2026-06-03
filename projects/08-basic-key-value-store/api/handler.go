// Package api implements the HTTP transport layer for the key-value store.
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/ankitsriv89/08-basic-key-value-store/metrics"
	"github.com/ankitsriv89/08-basic-key-value-store/store"
)

// maxValueSize caps incoming value payloads to prevent memory exhaustion.
const maxValueSize = 1 * 1024 * 1024 // 1 MiB

var (
	notFoundBody    = []byte(`{"error":"key not found"}`)
	badRequestBody  = []byte(`{"error":"bad request"}`)
	internalErrBody = []byte(`{"error":"internal server error"}`)
	okBody          = []byte(`{"ok":true}`)
)

// Handler holds dependencies for all HTTP handlers.
type Handler struct {
	engine *store.Engine
	log    *zap.Logger
	met    *metrics.Metrics
}

// New returns a fully wired *Handler.
func New(engine *store.Engine, log *zap.Logger, met *metrics.Metrics) *Handler {
	return &Handler{engine: engine, log: log, met: met}
}

// Register mounts all routes on r.
func (h *Handler) Register(r *mux.Router) {
	r.Handle("/", http.FileServer(http.Dir("web"))).Methods(http.MethodGet)
	r.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("web"))),
	)

	r.HandleFunc("/v1/kv/{key}", h.handleGet).Methods(http.MethodGet)
	r.HandleFunc("/v1/kv/{key}", h.handleSet).Methods(http.MethodPut)
	r.HandleFunc("/v1/kv/{key}", h.handleDelete).Methods(http.MethodDelete)
	r.HandleFunc("/v1/admin/compact", h.handleCompact).Methods(http.MethodPost)
	r.HandleFunc("/v1/admin/stats", h.handleStats).Methods(http.MethodGet)
	r.HandleFunc("/healthz", h.handleHealth).Methods(http.MethodGet)
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	key := mux.Vars(r)["key"]
	if key == "" {
		writeJSON(w, http.StatusBadRequest, badRequestBody)
		return
	}

	val, ok, err := h.engine.Get(key)
	dur := time.Since(start).Seconds()
	if err != nil {
		h.log.Error("get failed", zap.String("key", key), zap.Error(err))
		h.met.RecordOp("get", "error", dur)
		writeJSON(w, http.StatusInternalServerError, internalErrBody)
		return
	}
	if !ok {
		h.met.RecordOp("get", "miss", dur)
		writeJSON(w, http.StatusNotFound, notFoundBody)
		return
	}

	h.met.RecordOp("get", "hit", dur)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	w.Write(val)
}

func (h *Handler) handleSet(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	key := mux.Vars(r)["key"]
	if key == "" || strings.ContainsAny(key, "\x00") {
		writeJSON(w, http.StatusBadRequest, badRequestBody)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxValueSize+1))
	if err != nil {
		h.met.RecordOp("set", "error", time.Since(start).Seconds())
		writeJSON(w, http.StatusBadRequest, badRequestBody)
		return
	}
	if int64(len(body)) > maxValueSize {
		writeJSON(w, http.StatusRequestEntityTooLarge, []byte(`{"error":"value exceeds 1 MiB limit"}`))
		return
	}

	if err := h.engine.Set(key, body); err != nil {
		h.log.Error("set failed", zap.String("key", key), zap.Error(err))
		h.met.RecordOp("set", "error", time.Since(start).Seconds())
		writeJSON(w, http.StatusInternalServerError, internalErrBody)
		return
	}

	h.met.RecordOp("set", "ok", time.Since(start).Seconds())
	writeJSON(w, http.StatusOK, okBody)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	key := mux.Vars(r)["key"]
	if key == "" {
		writeJSON(w, http.StatusBadRequest, badRequestBody)
		return
	}

	if err := h.engine.Delete(key); err != nil {
		h.log.Error("delete failed", zap.String("key", key), zap.Error(err))
		h.met.RecordOp("delete", "error", time.Since(start).Seconds())
		writeJSON(w, http.StatusInternalServerError, internalErrBody)
		return
	}

	h.met.RecordOp("delete", "ok", time.Since(start).Seconds())
	writeJSON(w, http.StatusOK, okBody)
}

func (h *Handler) handleCompact(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if err := h.engine.Compact(); err != nil {
		h.log.Error("compact failed", zap.Error(err))
		h.met.RecordOp("compact", "error", time.Since(start).Seconds())
		writeJSON(w, http.StatusInternalServerError, internalErrBody)
		return
	}
	h.met.RecordOp("compact", "ok", time.Since(start).Seconds())
	writeJSON(w, http.StatusOK, okBody)
}

type statsResponse struct {
	Writes       int64          `json:"writes"`
	Reads        int64          `json:"reads"`
	Deletes      int64          `json:"deletes"`
	Flushes      int64          `json:"flushes"`
	Compactions  int64          `json:"compactions"`
	MemtableSize int64          `json:"memtable_bytes"`
	MemtableKeys int            `json:"memtable_keys"`
	SSTCount     int64          `json:"sst_count"`
	WALEntries   int64          `json:"wal_entries"`
	SSTables     []sstableInfo  `json:"sstables"`
}

type sstableInfo struct {
	SeqNum uint64 `json:"seq"`
	Level  int    `json:"level"`
	MinKey string `json:"min_key"`
	MaxKey string `json:"max_key"`
	Count  int    `json:"count"`
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	s := &h.engine.Stats
	ssts := h.engine.SSTables()
	infos := make([]sstableInfo, 0, len(ssts))
	for _, m := range ssts {
		infos = append(infos, sstableInfo{
			SeqNum: m.SeqNum,
			Level:  m.Level,
			MinKey: m.MinKey,
			MaxKey: m.MaxKey,
			Count:  m.Count,
		})
	}
	resp := statsResponse{
		Writes:       s.Writes.Load(),
		Reads:        s.Reads.Load(),
		Deletes:      s.Deletes.Load(),
		Flushes:      s.Flushes.Load(),
		Compactions:  s.Compactions.Load(),
		MemtableSize: s.MemtableSize.Load(),
		MemtableKeys: h.engine.MemtableLen(),
		SSTCount:     s.SSTCount.Load(),
		WALEntries:   s.WALEntries.Load(),
		SSTables:     infos,
	}
	b, _ := json.Marshal(resp)
	writeJSON(w, http.StatusOK, b)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []byte(`{"status":"ok"}`))
}

func writeJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}
