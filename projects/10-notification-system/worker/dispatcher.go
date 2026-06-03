// Package worker implements the async dispatch pipeline.
package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/ankitsriv89/10-notification-system/metrics"
	"github.com/ankitsriv89/10-notification-system/notification"
	"github.com/ankitsriv89/10-notification-system/store"
)

const (
	maxRetries    = 3
	baseBackoffMs = 200
	queueSize     = 1024
	dlqSize       = 256
	numWorkers    = 4
)

// Job is the unit of work passed through the dispatch pipeline.
type Job struct {
	Notification *notification.Notification
	AttemptNum   int
}

// Dispatcher owns the channel-based queue, worker pool, and DLQ.
type Dispatcher struct {
	queue     chan Job
	dlq       chan Job
	providers map[notification.Channel]Provider
	store     *store.Store
	met       *metrics.Metrics
	log       *zap.Logger
	wg        sync.WaitGroup
	// FailureRates is exported so the API can adjust provider failure rates at runtime for demos.
	FailureRates map[string]*float64
}

// NewDispatcher constructs the dispatcher with provider mocks.
func NewDispatcher(st *store.Store, met *metrics.Metrics, log *zap.Logger) *Dispatcher {
	emailRate := 0.1
	smsRate := 0.15
	pushRate := 0.05

	emailProv := NewMockEmailProvider(emailRate, 50, log)
	smsProv := NewMockSMSProvider(smsRate, 30, log)
	pushProv := NewMockPushProvider(pushRate, 20, log)

	return &Dispatcher{
		queue: make(chan Job, queueSize),
		dlq:   make(chan Job, dlqSize),
		providers: map[notification.Channel]Provider{
			notification.ChannelEmail: emailProv,
			notification.ChannelSMS:   smsProv,
			notification.ChannelPush:  pushProv,
		},
		store: st,
		met:   met,
		log:   log,
		FailureRates: map[string]*float64{
			"email": &emailProv.FailureRate,
			"sms":   &smsProv.FailureRate,
			"push":  &pushProv.FailureRate,
		},
	}
}

// Start launches the worker pool goroutines. All goroutines stop when ctx is cancelled.
func (d *Dispatcher) Start(ctx context.Context) {
	for i := 0; i < numWorkers; i++ {
		d.wg.Add(1)
		go d.worker(ctx, i)
	}
	// DLQ drain goroutine — logs and persists DLQ entries.
	d.wg.Add(1)
	go d.drainDLQ(ctx)
}

// Stop waits for all workers to finish after the context is cancelled.
func (d *Dispatcher) Stop() { d.wg.Wait() }

// Enqueue places a notification on the dispatch queue.
func (d *Dispatcher) Enqueue(n *notification.Notification) error {
	select {
	case d.queue <- Job{Notification: n, AttemptNum: 1}:
		d.met.QueueDepth.Inc()
		return nil
	default:
		return fmt.Errorf("dispatcher: queue full, shedding notification %s", n.ID)
	}
}

// QueueLen returns the current queue depth for observability.
func (d *Dispatcher) QueueLen() int { return len(d.queue) }

// DLQLen returns the current DLQ depth.
func (d *Dispatcher) DLQLen() int { return len(d.dlq) }

// worker processes jobs from the queue.
func (d *Dispatcher) worker(ctx context.Context, id int) {
	defer d.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-d.queue:
			if !ok {
				return
			}
			d.met.QueueDepth.Dec()
			d.dispatch(ctx, job)
		}
	}
}

// dispatch attempts delivery, recording the attempt and applying retry/DLQ logic.
func (d *Dispatcher) dispatch(ctx context.Context, job Job) {
	n := job.Notification
	prov, ok := d.providers[n.Channel]
	if !ok {
		d.log.Error("no provider for channel",
			zap.String("channel", string(n.Channel)),
			zap.String("notification_id", n.ID),
		)
		return
	}

	start := time.Now()
	sendErr := prov.Send(ctx, n)
	latencyMs := time.Since(start).Milliseconds()

	attempt := &notification.DeliveryAttempt{
		NotificationID: n.ID,
		Provider:       prov.Name(),
		AttemptNumber:  job.AttemptNum,
		LatencyMs:      latencyMs,
	}

	if sendErr == nil {
		attempt.Status = notification.StatusDelivered
		_ = d.store.InsertAttempt(ctx, attempt)
		_ = d.store.UpdateStatus(ctx, n.ID, notification.StatusDelivered)
		n.Status = notification.StatusDelivered
		d.met.Delivered.WithLabelValues(string(n.Channel)).Inc()
		d.met.DeliveryLatency.WithLabelValues(string(n.Channel)).Observe(float64(latencyMs))
		d.log.Info("notification delivered",
			zap.String("notification_id", n.ID),
			zap.String("channel", string(n.Channel)),
			zap.Int("attempt", job.AttemptNum),
		)
		return
	}

	// Failure path
	attempt.Status = notification.StatusFailed
	attempt.ErrorMsg = sendErr.Error()
	_ = d.store.InsertAttempt(ctx, attempt)
	d.met.Failed.WithLabelValues(string(n.Channel)).Inc()

	d.log.Warn("delivery failed",
		zap.String("notification_id", n.ID),
		zap.String("channel", string(n.Channel)),
		zap.Int("attempt", job.AttemptNum),
		zap.Error(sendErr),
	)

	if job.AttemptNum < maxRetries {
		// Exponential backoff re-enqueue
		backoff := time.Duration(baseBackoffMs*(1<<uint(job.AttemptNum))) * time.Millisecond
		time.AfterFunc(backoff, func() {
			retryJob := Job{Notification: n, AttemptNum: job.AttemptNum + 1}
			select {
			case d.queue <- retryJob:
				d.met.QueueDepth.Inc()
				d.met.Retries.WithLabelValues(string(n.Channel)).Inc()
			default:
				d.log.Error("retry queue full, dropping to DLQ",
					zap.String("notification_id", n.ID))
				d.sendToDLQ(n)
			}
		})
		_ = d.store.UpdateStatus(ctx, n.ID, notification.StatusQueued)
	} else {
		d.sendToDLQ(n)
	}
}

func (d *Dispatcher) sendToDLQ(n *notification.Notification) {
	select {
	case d.dlq <- Job{Notification: n}:
		d.met.DLQ.WithLabelValues(string(n.Channel)).Inc()
	default:
		d.log.Error("DLQ full, notification lost",
			zap.String("notification_id", n.ID))
	}
}

// drainDLQ persists DLQ entries and logs them.
func (d *Dispatcher) drainDLQ(ctx context.Context) {
	defer d.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-d.dlq:
			if !ok {
				return
			}
			n := job.Notification
			_ = d.store.UpdateStatus(ctx, n.ID, notification.StatusDLQ)
			d.log.Error("notification moved to DLQ",
				zap.String("notification_id", n.ID),
				zap.String("channel", string(n.Channel)),
				zap.String("user_id", n.UserID),
			)
		}
	}
}
