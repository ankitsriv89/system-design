// Package api implements the HTTP transport layer for the message queue service.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/ankitsriv89/13-message-queue/metrics"
	"github.com/ankitsriv89/13-message-queue/queue"
	"github.com/ankitsriv89/13-message-queue/store"
)

// Handler holds dependencies for all HTTP handlers.
type Handler struct {
	db    *store.DB
	cache *store.Cache
	log   *zap.Logger
}

// NewHandler constructs a Handler.
func NewHandler(db *store.DB, cache *store.Cache, log *zap.Logger) *Handler {
	return &Handler{db: db, cache: cache, log: log}
}

// Router wires all routes and returns the root http.Handler.
func (h *Handler) Router() http.Handler {
	r := mux.NewRouter()

	r.Handle("/metrics", promhttp.Handler())
	r.HandleFunc("/healthz", h.healthz).Methods(http.MethodGet)

	// Static web assets.
	r.PathPrefix("/static/").Handler(
		http.StripPrefix("/static/", http.FileServer(http.Dir("web"))))

	// Root UI.
	r.HandleFunc("/", h.serveUI).Methods(http.MethodGet)

	// Topic management.
	r.HandleFunc("/v1/topics", h.createTopic).Methods(http.MethodPost)
	r.HandleFunc("/v1/topics", h.listTopics).Methods(http.MethodGet)
	r.HandleFunc("/v1/topics/{topic}", h.getTopic).Methods(http.MethodGet)

	// Message operations.
	r.HandleFunc("/v1/topics/{topic}/messages", h.publish).Methods(http.MethodPost)
	r.HandleFunc("/v1/topics/{topic}/messages:poll", h.poll).Methods(http.MethodPost)
	r.HandleFunc("/v1/messages/{id}:ack", h.ack).Methods(http.MethodPost)

	// Admin / observability.
	r.HandleFunc("/v1/topics/{topic}/depth", h.getDepth).Methods(http.MethodGet)
	r.HandleFunc("/v1/topics/{topic}/dlq", h.listDLQ).Methods(http.MethodGet)
	r.HandleFunc("/v1/stats", h.getStats).Methods(http.MethodGet)

	return metricsMiddleware(r)
}

// --- Static + Health -------------------------------------------------------

var healthBody = []byte(`{"status":"ok"}`)

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(healthBody)
}

func (h *Handler) serveUI(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

// --- Topics ----------------------------------------------------------------

type createTopicReq struct {
	Name            string `json:"name"`
	Partitions      int    `json:"partitions"`
	RetentionHours  int    `json:"retention_hours"`
}

func (h *Handler) createTopic(w http.ResponseWriter, r *http.Request) {
	var req createTopicReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Partitions <= 0 {
		req.Partitions = 1
	}
	if req.RetentionHours <= 0 {
		req.RetentionHours = 24
	}

	t := &queue.Topic{
		Name:            req.Name,
		Partitions:      req.Partitions,
		RetentionPeriod: time.Duration(req.RetentionHours) * time.Hour,
		CreatedAt:       time.Now().UTC(),
	}
	if err := h.db.CreateTopic(r.Context(), t); err != nil {
		if err == queue.ErrTopicExists {
			writeError(w, http.StatusConflict, "topic already exists")
			return
		}
		h.log.Error("create topic", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	// Cache partition count for fast publish routing.
	_ = h.cache.SetTopicPartitions(r.Context(), req.Name, req.Partitions)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, t)
}

func (h *Handler) listTopics(w http.ResponseWriter, r *http.Request) {
	topics, err := h.db.ListTopics(r.Context())
	if err != nil {
		h.log.Error("list topics", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, map[string]interface{}{"topics": topics})
}

func (h *Handler) getTopic(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["topic"]
	t, err := h.db.GetTopic(r.Context(), name)
	if err != nil {
		if err == queue.ErrTopicNotFound {
			writeError(w, http.StatusNotFound, "topic not found")
			return
		}
		h.log.Error("get topic", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, t)
}

// --- Messages --------------------------------------------------------------

type publishReq struct {
	Key     string `json:"key"`
	Payload string `json:"payload"`
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) {
	topicName := mux.Vars(r)["topic"]

	var req publishReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Payload == "" {
		writeError(w, http.StatusBadRequest, "payload is required")
		return
	}

	timer := metrics.PublishLatency.WithLabelValues(topicName)
	start := time.Now()
	defer func() { timer.Observe(time.Since(start).Seconds()) }()

	// Look up partition count from cache; fall back to DB.
	partitions, err := h.cache.GetTopicPartitions(r.Context(), topicName)
	if err != nil || partitions == 0 {
		t, err := h.db.GetTopic(r.Context(), topicName)
		if err != nil {
			if err == queue.ErrTopicNotFound {
				writeError(w, http.StatusNotFound, "topic not found")
				return
			}
			h.log.Error("get topic for publish", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		partitions = t.Partitions
		_ = h.cache.SetTopicPartitions(r.Context(), topicName, partitions)
	}

	// Determine target partition.
	var counter int64
	if req.Key == "" {
		counter, _ = h.cache.IncrPublishCounter(r.Context(), topicName)
	}
	partition := queue.PartitionFor(req.Key, partitions, counter)

	now := time.Now().UTC()
	m := &queue.Message{
		ID:          newID(),
		Topic:       topicName,
		Partition:   partition,
		Key:         req.Key,
		Payload:     []byte(req.Payload),
		PublishedAt: now,
		VisibleAt:   now,
	}

	offset, err := h.db.PublishMessage(r.Context(), m)
	if err != nil {
		h.log.Error("publish message", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	m.Offset = offset

	metrics.MessagesPublished.WithLabelValues(topicName, fmt.Sprintf("%d", partition)).Inc()
	h.log.Info("message published",
		zap.String("id", m.ID),
		zap.String("topic", topicName),
		zap.Int("partition", partition),
		zap.Int64("offset", offset),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]interface{}{
		"id":        m.ID,
		"topic":     m.Topic,
		"partition": m.Partition,
		"offset":    m.Offset,
	})
}

type pollReq struct {
	ConsumerGroup     string `json:"consumer_group"`
	Partition         int    `json:"partition"`
	MaxMessages       int    `json:"max_messages"`
	VisibilityTimeout int    `json:"visibility_timeout_seconds"`
}

func (h *Handler) poll(w http.ResponseWriter, r *http.Request) {
	topicName := mux.Vars(r)["topic"]

	var req pollReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.ConsumerGroup == "" {
		writeError(w, http.StatusBadRequest, "consumer_group is required")
		return
	}
	if req.MaxMessages <= 0 {
		req.MaxMessages = 10
	}
	if req.MaxMessages > 100 {
		req.MaxMessages = 100
	}
	if req.Partition == 0 && r.URL.Query().Get("partition") == "" {
		req.Partition = -1 // any partition
	}
	vt := queue.DefaultVisibilityTimeout
	if req.VisibilityTimeout > 0 {
		vt = time.Duration(req.VisibilityTimeout) * time.Second
	}

	timer := metrics.PollLatency.WithLabelValues(topicName)
	start := time.Now()
	defer func() { timer.Observe(time.Since(start).Seconds()) }()

	msgs, err := h.db.PollMessages(r.Context(), &queue.PollRequest{
		Topic:             topicName,
		ConsumerGroup:     req.ConsumerGroup,
		Partition:         req.Partition,
		MaxMessages:       req.MaxMessages,
		VisibilityTimeout: vt,
	})
	if err != nil {
		h.log.Error("poll messages", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	metrics.MessagesPolled.WithLabelValues(topicName, req.ConsumerGroup).Add(float64(len(msgs)))

	type msgResp struct {
		ID               string    `json:"id"`
		Topic            string    `json:"topic"`
		Partition        int       `json:"partition"`
		Offset           int64     `json:"offset"`
		Key              string    `json:"key"`
		Payload          string    `json:"payload"`
		PublishedAt      time.Time `json:"published_at"`
		VisibleAt        time.Time `json:"visible_at"`
		DeliveryAttempts int       `json:"delivery_attempts"`
	}
	out := make([]msgResp, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, msgResp{
			ID:               m.ID,
			Topic:            m.Topic,
			Partition:        m.Partition,
			Offset:           m.Offset,
			Key:              m.Key,
			Payload:          string(m.Payload),
			PublishedAt:      m.PublishedAt,
			VisibleAt:        m.VisibleAt,
			DeliveryAttempts: m.DeliveryAttempts,
		})
	}
	writeJSON(w, map[string]interface{}{"messages": out, "count": len(out)})
}

type ackReq struct {
	ConsumerGroup string `json:"consumer_group"`
}

func (h *Handler) ack(w http.ResponseWriter, r *http.Request) {
	msgID := mux.Vars(r)["id"]

	var req ackReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.ConsumerGroup == "" {
		writeError(w, http.StatusBadRequest, "consumer_group is required")
		return
	}

	start := time.Now()
	if err := h.db.AckMessage(r.Context(), &queue.AckRequest{
		MessageID:     msgID,
		ConsumerGroup: req.ConsumerGroup,
	}); err != nil {
		if err == queue.ErrMessageNotFound {
			writeError(w, http.StatusNotFound, "message not found or already acked")
			return
		}
		h.log.Error("ack message", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	metrics.AckLatency.WithLabelValues("").Observe(time.Since(start).Seconds())
	metrics.MessagesAcked.WithLabelValues("", req.ConsumerGroup).Inc()

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]string{"status": "acked", "id": msgID})
}

// --- Admin -----------------------------------------------------------------

func (h *Handler) getDepth(w http.ResponseWriter, r *http.Request) {
	topicName := mux.Vars(r)["topic"]
	depth, err := h.db.GetQueueDepth(r.Context(), topicName)
	if err != nil {
		h.log.Error("get depth", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	dlq, err := h.db.GetDLQDepth(r.Context(), topicName)
	if err != nil {
		h.log.Error("get dlq depth", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, map[string]interface{}{"partitions": depth, "dlq": dlq})
}

func (h *Handler) listDLQ(w http.ResponseWriter, r *http.Request) {
	topicName := mux.Vars(r)["topic"]
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	msgs, err := h.db.ListDLQMessages(r.Context(), topicName, limit)
	if err != nil {
		h.log.Error("list dlq", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, map[string]interface{}{"messages": msgs, "count": len(msgs)})
}

func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetStats(r.Context())
	if err != nil {
		h.log.Error("get stats", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, stats)
}

// --- Helpers ---------------------------------------------------------------

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// newID generates a time-sortable message ID using a nanosecond timestamp
// plus a 4-character random hex suffix to avoid collisions at high publish rates.
func newID() string {
	ns := time.Now().UnixNano()
	// Encode as hex + 4 pseudo-random digits from the lower bits.
	return fmt.Sprintf("%016x%04x", ns, ns&0xffff)
}

// metricsMiddleware records HTTP request count and latency.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWriter{ResponseWriter: w, code: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rw, r)
		dur := time.Since(start).Seconds()
		path := r.URL.Path
		metrics.HTTPRequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(rw.code)).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(dur)
	})
}

type responseWriter struct {
	http.ResponseWriter
	code int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.code = code
	rw.ResponseWriter.WriteHeader(code)
}
