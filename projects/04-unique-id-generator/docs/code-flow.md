# Code Flow — Unique ID Generator

This document traces the function call graph from `main()` through every significant path. Each section covers one operation with a Mermaid flowchart and explanatory prose.

---

## 1. Startup

```mermaid
flowchart TD
    A["main()"] --> B["zap.NewProduction()\nstructured logger"]
    B --> C["configFromEnv()\nread LISTEN_ADDR, DATABASE_URL, REGION"]
    C --> D["lease.New(dsn, region, log)\nopen *sql.DB pool"]
    D --> E["waitForDB(log, lm)\nping loop up to 60s"]
    E --> F["lm.Acquire(ctx)\nclaim worker_id from PG"]
    F --> G{"acquired?"}
    G -- no --> H["log.Fatal — exit 1"]
    G -- yes --> I["generator.New(workerID, incidentHook)\nbuild Snowflake engine"]
    I --> J["lm.StartRenewer(ctx)\nlaunch background goroutine"]
    J --> K["api.New(gen, region, log)\nbuild HTTP handler"]
    K --> L["mux.NewRouter()\nattach metrics + requestLogger middleware"]
    L --> M["h.Register(r)\nwire all routes"]
    M --> N["http.Server{}.ListenAndServe()\nblocks in background goroutine"]
    N --> O["signal.Notify — wait for SIGINT/SIGTERM"]
    O --> P["Shutdown sequence"]
```

**Why each step matters:**
- `waitForDB` retries instead of failing fast because Docker Compose starts containers in parallel; PostgreSQL is often not ready when the app container boots.
- `lm.Acquire` happens before `generator.New` because the worker_id must be known before the generator can be constructed — there is no way to change a generator's worker_id after creation.
- `StartRenewer` is called after `Acquire` succeeds so the renewer never runs without a valid lease.

---

## 2. Generate a single ID — `POST /v1/ids/next`

```mermaid
flowchart TD
    A["HTTP POST /v1/ids/next"] --> B["metrics.Middleware\nstart timer, wrap ResponseWriter"]
    B --> C["requestLogger middleware\nrecord method + path"]
    C --> D["Handler.nextID(w, r)"]
    D --> E["time.Now() — start timer for generation_duration"]
    E --> F["gen.Next()"]

    subgraph generator.Next
        F --> G["g.mu.Lock()\nexclusive access to lastMs + sequence"]
        G --> H["nowMs = time.Now().UnixMilli()"]
        H --> I{"now < g.lastMs?"}
        I -- yes: clock rolled back --> J["fire onIncident hook\nsleep until caught up"]
        J --> H
        I -- no --> K{"now == g.lastMs?"}
        K -- yes: same ms --> L["g.sequence = (g.sequence+1) & 0xFFF"]
        L --> M{"sequence wrapped to 0?"}
        M -- yes: exhausted --> N["spin: now = nowMs() until now > lastMs"]
        N --> O["g.sequence = 0"]
        M -- no --> P["keep sequence"]
        K -- no: new ms --> Q["g.sequence = 0"]
        P --> R["g.lastMs = now"]
        Q --> R
        O --> R
        R --> S["id = ts<<22 | workerID<<12 | sequence"]
        S --> T["g.mu.Unlock()"]
        T --> U["return id"]
    end

    U --> V["metrics.GenerationDuration.Observe(elapsed)"]
    V --> W["metrics.IDsGenerated.Add(1)"]
    W --> X["writeJSON 200\n{id, id_string, worker_id, region}"]
```

**Key decisions:**
- The mutex wraps the entire `lastMs + sequence` read-modify-write as one atomic unit. Without it, two concurrent goroutines could read the same `lastMs`, both increment the same `sequence`, and produce the same ID.
- The sequence is masked with `& 0xFFF` (= `& maxSequence = 4095`) rather than using a modulo to avoid the division and keep the operation branch-free.
- `id_string` is always included alongside `id` because JSON numbers in JavaScript are 64-bit doubles (IEEE 754), which cannot represent all int64 values without loss of precision above 2^53.

---

## 3. Generate a batch — `POST /v1/ids/batch`

```mermaid
flowchart TD
    A["HTTP POST /v1/ids/batch"] --> B["json.NewDecoder.Decode → batchRequest{count}"]
    B --> C{"0 < count <= 1000?"}
    C -- no --> D["writeError 400"]
    C -- yes --> E["gen.Batch(count)"]
    E --> F["loop i=0..count-1\nids[i] = g.Next()"]
    F --> G["build id_strings slice\nstrconv.FormatInt each id"]
    G --> H["writeJSON 200\n{ids, id_strings, count, worker_id}"]
```

`Batch` is a simple loop over `Next()`. Each call acquires and releases the mutex individually — this is intentional. Holding the mutex for the entire batch would block other goroutines for longer than necessary and provide no correctness benefit since each ID is independently valid.

---

## 4. Inspect an ID — `GET /v1/ids/{id}/inspect`

```mermaid
flowchart TD
    A["GET /v1/ids/{id}/inspect"] --> B["mux.Vars(r)['id'] — extract path param"]
    B --> C["strconv.ParseInt(raw, 10, 64)"]
    C --> D{"parse error or id <= 0?"}
    D -- yes --> E["writeError 400"]
    D -- no --> F["generator.Decompose(id)"]

    subgraph generator.Decompose
        F --> G["timestampMs = id >> 22 + Epoch"]
        G --> H["workerID = id >> 12 & 0x3FF"]
        H --> I["sequence = id & 0xFFF"]
    end

    I --> J["time.UnixMilli(ts).UTC().Format(RFC3339Nano)"]
    J --> K["writeJSON 200\n{id, timestamp_ms, time, worker_id, sequence}"]
```

`Decompose` is a pure function — it does not touch the generator's mutex or state. It simply reverses the bit shifts used during generation.

---

## 5. Lease renewal — background goroutine

```mermaid
flowchart TD
    A["lm.StartRenewer(ctx)"] --> B["go func()"]
    B --> C["time.NewTicker(10s)"]
    C --> D{"select"}
    D -- ctx.Done --> E["return — stop renewer"]
    D -- ticker.C --> F["lm.renew(ctx)"]
    F --> G["UPDATE worker_leases SET expires_at=NOW()+30s\nWHERE worker_id=N AND expires_at >= NOW()"]
    G --> H{"rows affected == 0?"}
    H -- yes: lease expired --> I["return error\nlog.Error — caller should restart"]
    H -- no --> J["log.Debug 'lease renewed'\nmetrics.LeaseRenewals.Inc()"]
    J --> D
    I --> K["metrics.LeaseFailures.Inc()"]
    K --> D
```

The `AND expires_at >= NOW()` guard in the UPDATE is critical: if the lease expired before this renewal fired (e.g. the process was paused by the OS), the UPDATE affects 0 rows and we learn about it immediately rather than silently extending a slot we no longer legitimately hold.

---

## 6. Clock rollback handling — inside `generator.Next()`

```mermaid
flowchart TD
    A["now = nowMs()"] --> B{"now < g.lastMs?"}
    B -- no --> C["normal path"]
    B -- yes --> D["drift = g.lastMs - now"]
    D --> E["fire onIncident(workerID, drift)"]
    E --> F["metrics.ClockRollbacks.Inc()"]
    F --> G["metrics.ClockDriftMs.Observe(drift)"]
    G --> H["log.Warn clock rollback"]
    H --> I["lm.RecordClockIncident(ctx, drift)\nINSERT INTO clock_incidents"]
    I --> J["sleep drift ms"]
    J --> K["now = nowMs()"]
    K --> B
```

The spin loop re-checks `now < g.lastMs` after sleeping to handle the case where the OS wakes the goroutine early. The loop exits only when wall time has fully caught up with `lastMs`.

---

## 7. Graceful shutdown

```mermaid
flowchart TD
    A["<-quit (SIGINT/SIGTERM)"] --> B["log.Info 'shutting down'"]
    B --> C["renewCancel()\nstops lease renewer goroutine"]
    C --> D["srv.Shutdown(ctx, 15s)\ndrains in-flight HTTP requests"]
    D --> E["lm.Release(ctx)\nUPDATE worker_leases SET holder='', expires_at=past"]
    E --> F["log.Info 'shutdown complete'"]
    F --> G["os.Exit(0) via deferred log.Sync"]
```

`renewCancel` is called before `srv.Shutdown` to prevent a race where the renewer fires an UPDATE after `Release` has already cleared the lease row.

---

## Call graph summary

```mermaid
graph LR
    main --> lease.New
    main --> lease.Acquire
    main --> generator.New
    main --> lease.StartRenewer
    main --> api.New
    main --> mux.Router

    api.Handler --> generator.Next
    api.Handler --> generator.Batch
    api.Handler --> generator.Decompose
    api.Handler --> metrics

    lease.StartRenewer --> lease.renew
    lease.renew --> sql.DB

    generator.Next --> onIncident
    onIncident --> metrics
    onIncident --> lease.RecordClockIncident
    lease.RecordClockIncident --> sql.DB
```
