# Architecture — Web Crawler (Project 12)

## System Diagram

```mermaid
graph TD
    Browser["Browser / Demo UI"] -->|POST /v1/crawl-jobs| API["HTTP API\n(gorilla/mux)"]
    API -->|INSERT url_frontier| PG[("PostgreSQL\ncrawl_jobs\nurl_frontier\npage_fetches")]
    API -->|reads stats| PG
    API -->|reads seen count| Redis[("Redis\nseen-set\nrobots cache")]

    subgraph Workers["Worker Pool (×3 goroutines)"]
        W1["Worker 0"]
        W2["Worker 1"]
        W3["Worker 2"]
    end

    W1 & W2 & W3 -->|SELECT FOR UPDATE SKIP LOCKED| PG
    W1 & W2 & W3 -->|SISMEMBER / SADD| Redis
    W1 & W2 & W3 -->|GET robots.txt cache| Redis
    W1 & W2 & W3 -->|HTTP GET page| Internet["Internet\n(target hosts)"]
    W1 & W2 & W3 -->|UPSERT page_fetches| PG
    W1 & W2 & W3 -->|INSERT url_frontier| PG

    API -->|/metrics| Prometheus["Prometheus"]
    Prometheus --> Grafana["Grafana"]
```

## Sequence Diagram — Happy Path Crawl

```mermaid
sequenceDiagram
    participant B as Browser
    participant A as API
    participant DB as PostgreSQL
    participant R as Redis
    participant W as Worker
    participant H as Target Host

    B->>A: POST /v1/crawl-jobs {seed_url, max_depth}
    A->>DB: INSERT crawl_jobs
    A->>DB: INSERT url_frontier (seed URL, priority=10)
    A-->>B: 201 {id, status:"running"}

    loop Crawl loop
        W->>DB: SELECT FOR UPDATE SKIP LOCKED (5 URLs)
        DB-->>W: URLEntry[]
        W->>R: SISMEMBER crawl:seen urlHash
        alt already seen
            W->>DB: UPDATE status=skipped
        else unseen
            W->>R: GET robots:{host}
            alt cache miss
                W->>H: GET /robots.txt
                W->>R: SET robots:{host} 24h
            end
            alt path disallowed
                W->>DB: UPDATE status=skipped
            else allowed
                W->>W: sleep crawl-delay (≥1s)
                W->>H: GET {url} (15s timeout, 2MB cap)
                H-->>W: HTTP response
                W->>DB: UPSERT page_fetches
                W->>R: SADD crawl:seen urlHash
                W->>DB: UPDATE url_frontier status=done
                W->>W: ExtractLinks from HTML body
                loop each new link
                    W->>DB: INSERT url_frontier ON CONFLICT DO NOTHING
                end
            end
        end
    end
```

## Components

### HTTP API (`api/`)
Handles synchronous user and operator workflows. All handlers share a DB + Redis client injected at construction time. Static assets served from `web/`. No business logic — delegates all state mutations to the `store` package.

### Crawler Domain (`crawler/`)
Pure functions with no I/O: URL normalisation, hashing, link extraction, robots.txt parsing. The `HTTPFetcher` wraps a single `*http.Client` shared across all calls. All domain rules live here — isolated, unit-testable without Docker.

### Store (`store/`)
Two adapters:
- **`DB`** — PostgreSQL via `database/sql` + `lib/pq`. Uses `SELECT FOR UPDATE SKIP LOCKED` for lock-free worker fan-out on the frontier.
- **`Cache`** — Redis via `go-redis/v9`. Holds a Redis SET (`crawl:seen`) for O(1) deduplication and per-host JSON blobs for robots.txt caching.

### Worker (`worker/`)
Each `Worker` is a goroutine running a claim-fetch-enqueue loop. Workers exit cleanly when the context is cancelled (SIGTERM). The number of workers is configurable via `NUM_WORKERS` (default 3).

### Metrics (`metrics/`)
Prometheus counters and histograms registered at package init. Exposed at `/metrics` via `promhttp`.

## Data Flows

1. **Submission**: Browser → API → `crawl_jobs` + `url_frontier` (seed, priority 10)
2. **Crawl tick**: Worker → `url_frontier` (SKIP LOCKED) → Redis dedupe → robots check → HTTP fetch → `page_fetches` → extract links → re-enqueue to `url_frontier`
3. **Read path**: Browser polls `/v1/frontier/stats` (aggregated counts) and `/v1/pages` (recent fetches) every 3–6s

## Capacity Table

| Metric | Value |
|---|---|
| Workers (MVP) | 3 goroutines |
| Target fetch throughput | ~3 req/s (1s politeness delay per host) |
| Max body size per page | 2 MB |
| URL dedup structure | Redis SET (O(1) SISMEMBER) |
| robots.txt cache TTL | 24 h |
| Recrawl delay (done URLs) | 24 h |
| Frontier index | `(status, next_fetch_at, priority DESC)` partial index on `pending` |
| Page storage | PostgreSQL row ~200 B + content hash; no raw HTML stored in DB |
| Redis seen-set growth | ~64 B per URL hash × N URLs |
