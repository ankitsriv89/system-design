# Code Flow — Notification System (Project 10)

## Send Notification (`POST /v1/notifications`)

```mermaid
flowchart TD
    A[HTTP POST /v1/notifications] --> B[api.Handler.createNotification]
    B --> C{JSON decode & validate}
    C -->|invalid| ERR1[400 Bad Request]
    C -->|valid| D{template_id set?}
    D -->|yes| E[store.GetTemplate]
    E -->|not found| ERR2[404 Not Found]
    E -->|found| F[notification.RenderTemplate\nstring substitution on subject + body]
    D -->|no| G[use subject/body from request]
    F --> G
    G --> H[store.GetPreference user_id + channel]
    H --> I{pref.Enabled == false?}
    I -->|yes| SKIP1[status = skipped\nstore.CreateNotification\n201 Created]
    I -->|no| J{notification.IsQuietHour?}
    J -->|yes| SKIP2[status = skipped\nstore.CreateNotification\n201 Created]
    J -->|no| K[status = queued\nstore.CreateNotification]
    K --> L[dispatcher.Enqueue]
    L -->|queue full| ERR3[503 Service Unavailable]
    L -->|ok| M[met.Enqueued.Inc\n201 Created with notification JSON]
```

## Async Dispatch (worker goroutine)

```mermaid
flowchart TD
    Q[queue chan Job] --> W[worker goroutine selects job]
    W --> A[providers map lookup by channel]
    A -->|not found| LOG1[log error, discard]
    A -->|found| B[Provider.Send ctx + notification\nrecords wall-clock latency]
    B -->|nil error| C[InsertAttempt status=delivered\nUpdateStatus delivered\nmet.Delivered.Inc]
    B -->|error| D[InsertAttempt status=failed\nmet.Failed.Inc]
    D --> E{attempt < maxRetries 3}
    E -->|yes| F[time.AfterFunc backoff\nbase 200ms × 2^attempt\nre-enqueue with attempt+1\nmet.Retries.Inc]
    F -->|queue full| G[sendToDLQ]
    E -->|no| G
    G --> H[dlq channel]
    H --> I[drainDLQ goroutine\nUpdateStatus dlq\nlog Error\nmet.DLQ.Inc]
```

## Template Rendering

```mermaid
flowchart TD
    A[Template.Body with placeholders\ne.g. 'Hi {{.Name}} code {{.Code}}'] --> B[notification.RenderTemplate]
    B --> C[strings.ReplaceAll for each key in params map]
    C --> D[rendered subject + body]
```

Substitution is O(k × len(body)) where k = number of params. For typical templates (< 1 KB body, < 10 params) this is negligible.

## Quiet Hours Check

```mermaid
flowchart TD
    A[notification.IsQuietHour pref + now] --> B{QuietStart or QuietEnd == -1?}
    B -->|yes| RET1[return false — quiet hours disabled]
    B -->|no| C{QuietStart <= QuietEnd?}
    C -->|yes normal window e.g. 10–14| D[return h >= QuietStart AND h < QuietEnd]
    C -->|no wraps midnight e.g. 22–8| E[return h >= QuietStart OR h < QuietEnd]
```

## Call Graph Summary

```mermaid
graph LR
    main --> api.New
    main --> worker.NewDispatcher
    main --> store.New
    main --> metrics.New
    main --> mux.Router
    api.Handler --> store.GetTemplate
    api.Handler --> notification.RenderTemplate
    api.Handler --> store.GetPreference
    api.Handler --> notification.IsQuietHour
    api.Handler --> store.CreateNotification
    api.Handler --> worker.Dispatcher.Enqueue
    worker.Dispatcher --> worker.Provider.Send
    worker.Dispatcher --> store.InsertAttempt
    worker.Dispatcher --> store.UpdateStatus
    worker.drainDLQ --> store.UpdateStatus
```

## Key Design Decisions

**Why `time.AfterFunc` for retry backoff instead of a separate retry queue?**
`time.AfterFunc` fires a goroutine after the delay and re-enqueues the job. This keeps the retry logic inside the Dispatcher without a second data structure. The tradeoff: if the process restarts, in-flight retry timers are lost. For production, retries should be persisted (e.g. a `retry_after` column on notifications and a polling sweeper).

**Why buffered channels instead of Kafka?**
All other projects in this series use in-process concurrency primitives. A Kafka dependency would require a separate broker container, increasing resource usage on the shared Oracle Cloud instance. The channel model demonstrates the same fanout, backpressure, and DLQ semantics without the operational overhead. The production design section in `architecture.md` calls out Kafka as the natural evolution path.
