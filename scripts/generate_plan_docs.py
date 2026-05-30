from pathlib import Path
import re
import textwrap


ROOT = Path(__file__).resolve().parents[1]


PHASES = {
    1: {
        "name": "Core Building Blocks",
        "range": "01-13",
        "outcome": "Build the primitives that appear inside larger distributed systems: traffic control, storage, caching, queues, crawling, routing, and gateway behavior.",
    },
    2: {
        "name": "Real-Time and Product Systems",
        "range": "14-24",
        "outcome": "Compose the primitives into user-facing systems with realtime delivery, fanout, ranking, transactional flows, and product-facing APIs.",
    },
    3: {
        "name": "Distributed Infrastructure",
        "range": "25-35",
        "outcome": "Design infrastructure-scale systems around partitioning, replication, media pipelines, search, routing, collaborative state, and distributed storage.",
    },
    4: {
        "name": "Advanced Data and Platform Systems",
        "range": "36-46",
        "outcome": "Work through streaming analytics, marketplaces, fraud, exchanges, schedulers, CQRS, multi-tenancy, gaming, and ML serving.",
    },
    5: {
        "name": "Senior/Staff-Level Scale",
        "range": "47-50",
        "outcome": "Practice global architecture decisions: multi-region latency, consensus, capital-market latency, and planet-scale availability.",
    },
}


PROJECTS = [
    {
        "id": 1,
        "title": "Rate Limiter",
        "phase": 1,
        "stack": "Go, Redis, PostgreSQL, Docker, REST/gRPC, Prometheus",
        "problem": "Protect APIs from abusive or accidental traffic spikes while preserving fair access for legitimate clients.",
        "users": "API platform teams, gateway owners, and product teams exposing public or partner APIs.",
        "objectives": ["token bucket and sliding-window algorithms", "hot-key mitigation", "distributed counters", "policy evaluation at the edge"],
        "apis": ["POST /v1/limits/check", "PUT /v1/policies/{policy_id}", "GET /v1/usage/{subject}"],
        "models": ["Policy(id, subject_type, algorithm, capacity, refill_rate, window, action)", "UsageCounter(subject, policy_id, bucket, count, expires_at)", "AuditDecision(subject, allowed, reason, retry_after_ms)"],
        "events": ["rate_limit.exceeded", "policy.updated", "counter.compacted"],
        "storage": "Redis for low-latency counters and PostgreSQL for durable policy configuration and audit summaries.",
        "consistency": "Favor low-latency approximate enforcement with bounded drift; use Lua scripts or Redis transactions per key.",
        "caching": "Cache active policies in process with short TTL and version invalidation.",
        "failures": ["Redis unavailable", "policy cache stale", "clock skew between nodes", "single tenant hot key"],
        "milestones": ["Single-node token bucket library", "Redis-backed distributed limiter", "policy API and admin CLI", "load-test report with fairness metrics"],
        "interview": ["Explain fixed window vs sliding window vs token bucket.", "Defend fail-open vs fail-closed behavior.", "Discuss sharding counters and handling celebrity tenants."],
    },
    {
        "id": 2,
        "title": "URL Shortener",
        "phase": 1,
        "stack": "Go, PostgreSQL, Redis, Docker, REST, React admin dashboard",
        "problem": "Create short, memorable links that redirect reliably, track analytics, and handle high read traffic.",
        "users": "Marketing teams, creators, internal tooling teams, and public anonymous users.",
        "objectives": ["base62 encoding", "read-heavy API design", "cache-aside redirects", "analytics ingestion"],
        "apis": ["POST /v1/links", "GET /{code}", "GET /v1/links/{code}/stats"],
        "models": ["Link(code, long_url, owner_id, expires_at, created_at)", "ClickEvent(code, ts, referrer, country, device)", "Owner(id, quota, plan)"],
        "events": ["link.created", "link.clicked", "link.expired"],
        "storage": "PostgreSQL for canonical links, Redis for redirect cache, append-only click events for analytics.",
        "consistency": "Short-code creation must be unique; click analytics can be eventually consistent.",
        "caching": "Cache hot code to URL mappings with negative-cache entries for missing links.",
        "failures": ["cache stampede on viral link", "malicious destination URL", "code collision", "analytics backpressure"],
        "milestones": ["Short-code generator and redirect path", "owner quotas and expiration", "analytics event pipeline", "dashboard for link stats"],
        "interview": ["Estimate storage and QPS for redirects.", "Compare random IDs vs sequence-based IDs.", "Explain how to keep redirects fast while analytics lags."],
    },
    {
        "id": 3,
        "title": "Pastebin",
        "phase": 1,
        "stack": "Go, PostgreSQL, S3-compatible object storage, Redis, REST",
        "problem": "Store text snippets with optional expiration, privacy controls, syntax metadata, and high-read sharing links.",
        "users": "Developers, support engineers, and teams sharing logs or code snippets.",
        "objectives": ["blob storage boundaries", "metadata indexing", "expiration jobs", "abuse controls"],
        "apis": ["POST /v1/pastes", "GET /v1/pastes/{id}", "DELETE /v1/pastes/{id}"],
        "models": ["Paste(id, owner_id, visibility, language, size_bytes, object_key, expires_at)", "PasteVersion(paste_id, version, object_key)", "AbuseReport(paste_id, reason, status)"],
        "events": ["paste.created", "paste.expired", "paste.reported"],
        "storage": "Metadata in PostgreSQL, content in object storage, Redis for hot metadata and rate limits.",
        "consistency": "Metadata and object writes need a compensating cleanup path when one succeeds and the other fails.",
        "caching": "Cache public paste metadata and content for short TTL, bypass for private or recently deleted items.",
        "failures": ["object write succeeds but metadata write fails", "large paste abuse", "expired paste still cached", "read surge on shared paste"],
        "milestones": ["CRUD for public pastes", "private and expiring pastes", "object storage integration", "moderation and abuse workflow"],
        "interview": ["Explain why content should not live directly in SQL for large pastes.", "Discuss retention and deletion semantics.", "Handle read-after-delete and cache invalidation."],
    },
    {
        "id": 4,
        "title": "Unique ID Generator",
        "phase": 1,
        "stack": "Go, gRPC, PostgreSQL, Docker, Prometheus",
        "problem": "Generate sortable, globally unique IDs at high throughput without a central database bottleneck.",
        "users": "Internal services needing primary keys for orders, posts, messages, and events.",
        "objectives": ["Snowflake-style IDs", "worker coordination", "clock rollback handling", "ID semantics"],
        "apis": ["POST /v1/ids/next", "POST /v1/ids/batch", "GET /v1/workers/{worker_id}/health"],
        "models": ["WorkerLease(worker_id, region, expires_at, sequence_range)", "IdAllocation(ts_ms, worker_id, sequence, id)", "ClockIncident(worker_id, drift_ms, action)"],
        "events": ["worker.lease_acquired", "id.batch_allocated", "clock.rollback_detected"],
        "storage": "PostgreSQL for worker leases and audit records; in-memory counters for fast allocation.",
        "consistency": "Leases must prevent duplicate worker IDs; generated IDs remain unique even if clocks drift.",
        "caching": "Keep lease state in memory and renew periodically with defensive expiry checks.",
        "failures": ["clock moves backward", "worker lease split brain", "sequence overflow in a millisecond", "region outage"],
        "milestones": ["Local Snowflake generator", "lease service", "batch allocation API", "fault-injection tests for clocks and leases"],
        "interview": ["Break down timestamp, worker, and sequence bits.", "Compare UUID, ULID, database sequence, and Snowflake.", "Discuss ordering guarantees and clock risk."],
    },
    {
        "id": 5,
        "title": "Consistent Hashing",
        "phase": 1,
        "stack": "Go, CLI visualizer, Docker, optional React ring explorer",
        "problem": "Distribute keys across changing node sets while minimizing key movement during scale events.",
        "users": "Infrastructure engineers designing caches, storage rings, and shard routers.",
        "objectives": ["hash rings", "virtual nodes", "weighted placement", "rebalance analysis"],
        "apis": ["POST /v1/rings", "POST /v1/rings/{ring}/nodes", "GET /v1/rings/{ring}/keys/{key}/owner"],
        "models": ["Ring(id, hash_fn, replicas)", "Node(id, weight, status)", "Assignment(key_hash, primary_node, replica_nodes)"],
        "events": ["node.added", "node.removed", "ring.rebalanced"],
        "storage": "In-memory ring for MVP; persisted ring snapshots in PostgreSQL or local files for replay.",
        "consistency": "Ring versioning ensures clients route with a coherent snapshot during changes.",
        "caching": "Clients cache ring snapshots with version checks and graceful rollover.",
        "failures": ["uneven key distribution", "node churn", "ring version mismatch", "hot shard despite balanced hashing"],
        "milestones": ["Hash-ring library", "virtual node weighting", "rebalance simulator", "visual report of key movement"],
        "interview": ["Explain why modulo hashing causes massive movement.", "Discuss virtual nodes and weighted capacity.", "Relate the ring to distributed caches and databases."],
    },
    {
        "id": 6,
        "title": "Load Balancer",
        "phase": 1,
        "stack": "Go, HTTP reverse proxy, Docker Compose, Prometheus, React traffic console",
        "problem": "Distribute client requests across healthy backend instances with routing, retries, and observability.",
        "users": "Platform teams running internal services and edge routing layers.",
        "objectives": ["round-robin and least-connections", "health checks", "connection draining", "retry budgets"],
        "apis": ["PUT /v1/backends/{service}", "GET /v1/backends/{service}/health", "POST /proxy/{service}/..."],
        "models": ["Backend(service, url, weight, status)", "HealthCheck(service, backend, status, latency_ms)", "RouteRule(host, path, service)"],
        "events": ["backend.healthy", "backend.unhealthy", "request.retried"],
        "storage": "In-memory routing table for data plane; PostgreSQL or file config for control plane.",
        "consistency": "Route updates should be atomic per version; data-plane requests finish on previous version while new traffic uses latest.",
        "caching": "Cache DNS resolution and backend health state with short intervals.",
        "failures": ["backend flapping", "retry storm", "slow backend saturation", "misconfigured route"],
        "milestones": ["Reverse proxy with round robin", "active/passive health checks", "weighted least-connections", "dashboard and load-test comparison"],
        "interview": ["Compare L4 and L7 load balancing.", "Explain retries, timeouts, and circuit breakers together.", "Discuss how to drain instances during deploys."],
    },
    {
        "id": 7,
        "title": "API Gateway",
        "phase": 1,
        "stack": "Go, Redis, PostgreSQL, OpenTelemetry, REST/gRPC, Docker",
        "problem": "Provide a central entry point for authentication, routing, rate limits, request shaping, and service observability.",
        "users": "Microservice teams and external API consumers.",
        "objectives": ["auth middleware", "routing rules", "request transformation", "gateway-level SLOs"],
        "apis": ["PUT /v1/routes/{route_id}", "POST /v1/api-keys", "GET /gateway/{service}/..."],
        "models": ["ApiKey(id, owner, scopes, quota_policy)", "Route(id, match, upstream, auth_required)", "GatewayDecision(request_id, route, auth, rate_limit)"],
        "events": ["route.updated", "api_key.created", "gateway.request_blocked"],
        "storage": "PostgreSQL for tenants, keys, and routes; Redis for key lookups, rate counters, and policy cache.",
        "consistency": "Route and policy changes propagate by version; active requests use a consistent config snapshot.",
        "caching": "Cache key metadata, route table, and service discovery records in process.",
        "failures": ["auth service latency", "bad route config", "upstream timeout", "oversized request payload"],
        "milestones": ["Routing proxy", "API key auth and scopes", "rate limiting integration", "tracing and dashboard"],
        "interview": ["Explain what belongs in a gateway vs service code.", "Discuss gateway as a bottleneck and blast radius.", "Cover auth, limits, retries, and observability at the edge."],
    },
    {
        "id": 8,
        "title": "Basic Key-Value Store",
        "phase": 1,
        "stack": "Go, gRPC, write-ahead log, SSTable-style files, Docker",
        "problem": "Build a durable key-value database with predictable reads/writes and a path toward compaction and replication.",
        "users": "Infrastructure learners and backend teams needing embedded storage concepts.",
        "objectives": ["WAL durability", "memtable and segments", "compaction", "simple replication vocabulary"],
        "apis": ["PUT /v1/kv/{key}", "GET /v1/kv/{key}", "DELETE /v1/kv/{key}", "POST /v1/admin/compact"],
        "models": ["Record(key, value, version, tombstone, ts)", "Segment(file, min_key, max_key, level)", "WalEntry(op, key, value, version)"],
        "events": ["kv.written", "segment.flushed", "compaction.completed"],
        "storage": "Write-ahead log plus immutable sorted segments; metadata manifest tracks active files.",
        "consistency": "Single-node linearizable writes for MVP; later add leader-follower replication.",
        "caching": "Memtable for latest values, optional block cache and Bloom filters for segment reads.",
        "failures": ["process crash during write", "corrupted segment", "compaction crash", "large value pressure"],
        "milestones": ["In-memory KV API", "WAL recovery", "SSTable flush and reads", "compaction and benchmark report"],
        "interview": ["Explain LSM-tree basics.", "Compare B-tree and LSM tradeoffs.", "Discuss tombstones, compaction, and read amplification."],
    },
    {
        "id": 9,
        "title": "Caching System",
        "phase": 1,
        "stack": "Go, Redis-compatible protocol subset, Docker, Prometheus",
        "problem": "Implement a cache service with eviction, TTL, metrics, and predictable behavior under memory pressure.",
        "users": "Backend services needing low-latency reads and reduced database load.",
        "objectives": ["LRU and LFU eviction", "TTL expiration", "cache-aside patterns", "stampede protection"],
        "apis": ["PUT /v1/cache/{key}", "GET /v1/cache/{key}", "DELETE /v1/cache/{key}", "GET /v1/cache/stats"],
        "models": ["CacheEntry(key, value, size_bytes, ttl, last_accessed)", "EvictionRecord(key, reason, size_bytes)", "CacheStats(hit_rate, evictions, memory_used)"],
        "events": ["cache.hit", "cache.miss", "cache.evicted"],
        "storage": "In-memory indexed map plus eviction list; optional AOF snapshot for warm restart.",
        "consistency": "Cache is explicitly non-authoritative; correctness comes from TTL, invalidation, and source-of-truth fallback.",
        "caching": "This is the cache; include singleflight request coalescing for missed hot keys.",
        "failures": ["cache stampede", "memory exhaustion", "stale data after source update", "large object evicts many small hot keys"],
        "milestones": ["LRU cache library", "HTTP cache service", "TTL and eviction metrics", "stampede demo with load test"],
        "interview": ["Explain cache-aside vs write-through vs write-back.", "Discuss TTL and invalidation tradeoffs.", "Estimate hit-rate impact and memory sizing."],
    },
    {
        "id": 10,
        "title": "Notification System",
        "phase": 1,
        "stack": "Java/Spring Boot, Kafka, PostgreSQL, Redis, provider mocks, Docker",
        "problem": "Send email, SMS, and push notifications reliably while respecting preferences, retries, and provider limits.",
        "users": "Product teams needing transactional and engagement notifications.",
        "objectives": ["fanout pipeline", "template rendering", "retry and dead-letter queues", "user preferences"],
        "apis": ["POST /v1/notifications", "PUT /v1/preferences/{user_id}", "GET /v1/notifications/{id}"],
        "models": ["Notification(id, user_id, channel, template, status)", "Preference(user_id, channel, enabled, quiet_hours)", "DeliveryAttempt(notification_id, provider, status, error)"],
        "events": ["notification.requested", "notification.delivered", "notification.failed"],
        "storage": "PostgreSQL for templates, preferences, and delivery state; Kafka for asynchronous delivery; Redis for provider rate limits.",
        "consistency": "Accept request durably before delivery; delivery is at-least-once with idempotency keys per provider.",
        "caching": "Cache templates and preferences with invalidation on updates.",
        "failures": ["provider outage", "duplicate delivery", "poison message", "preference changed during queued send"],
        "milestones": ["Notification request API", "Kafka worker and provider mock", "preference and template support", "retry, DLQ, and dashboard"],
        "interview": ["Explain at-least-once delivery and idempotency.", "Discuss provider failover and rate limits.", "Cover user preference correctness."],
    },
    {
        "id": 11,
        "title": "Typeahead Autocomplete System",
        "phase": 1,
        "stack": "Go, Redis, PostgreSQL, OpenSearch optional, React demo",
        "problem": "Return relevant suggestions within milliseconds as users type partial queries.",
        "users": "Search teams, marketplace apps, social apps, and developer tools.",
        "objectives": ["trie/prefix index", "ranking features", "hot prefix caching", "index rebuilds"],
        "apis": ["GET /v1/suggest?q={prefix}", "POST /v1/corpus/items", "POST /v1/admin/rebuild-index"],
        "models": ["SuggestItem(id, text, category, popularity, locale)", "PrefixEntry(prefix, item_ids, score)", "QueryLog(prefix, selected_item, latency_ms)"],
        "events": ["suggestion.indexed", "suggestion.clicked", "index.rebuilt"],
        "storage": "PostgreSQL for corpus, Redis sorted sets for hot prefix suggestions, optional OpenSearch for fuzzy matching.",
        "consistency": "Suggestion freshness can lag writes; rebuilds publish immutable index versions.",
        "caching": "Cache top suggestions for hot prefixes and common locales.",
        "failures": ["large prefix fanout", "stale ranking data", "index rebuild failure", "locale-specific relevance bug"],
        "milestones": ["Prefix trie or Redis sorted-set MVP", "ranking and click feedback", "index rebuild job", "latency benchmark with hot prefixes"],
        "interview": ["Explain trie vs search-engine index.", "Discuss ranking and personalization.", "Design for p99 latency under typing traffic."],
    },
    {
        "id": 12,
        "title": "Web Crawler",
        "phase": 1,
        "stack": "Go, Kafka or NATS, PostgreSQL, Redis, object storage, Docker",
        "problem": "Crawl web pages politely, deduplicate URLs, extract content, and schedule recrawls.",
        "users": "Search, SEO, archive, and data ingestion teams.",
        "objectives": ["frontier scheduling", "robots.txt politeness", "deduplication", "content extraction"],
        "apis": ["POST /v1/crawl-jobs", "GET /v1/pages/{url_hash}", "GET /v1/frontier/stats"],
        "models": ["UrlFrontier(url, host, priority, next_fetch_at)", "PageFetch(url_hash, status, content_hash, fetched_at)", "RobotsRule(host, allowed, crawl_delay)"],
        "events": ["url.discovered", "page.fetched", "crawl.failed"],
        "storage": "PostgreSQL for crawl metadata, Redis for per-host queues and dedupe filters, object storage for page snapshots.",
        "consistency": "At-least-once crawling is acceptable; dedupe by normalized URL and content hash.",
        "caching": "Cache robots.txt and DNS lookups with host-level TTLs.",
        "failures": ["crawler trap", "host overload", "duplicate URL explosion", "malformed content"],
        "milestones": ["Single worker fetcher", "frontier and robots support", "distributed workers", "content extraction and recrawl policy"],
        "interview": ["Explain crawl frontier design.", "Discuss politeness and backpressure.", "Handle dedupe at URL and content levels."],
    },
    {
        "id": 13,
        "title": "Message Queue",
        "phase": 1,
        "stack": "Go, gRPC/HTTP, PostgreSQL or log segments, Docker, Prometheus",
        "problem": "Provide durable asynchronous messaging with acknowledgements, retries, ordering options, and consumer groups.",
        "users": "Backend teams decoupling services and smoothing load spikes.",
        "objectives": ["append-only logs", "consumer offsets", "visibility timeouts", "dead-letter queues"],
        "apis": ["POST /v1/topics/{topic}/messages", "GET /v1/topics/{topic}/messages:poll", "POST /v1/messages/{id}:ack"],
        "models": ["Topic(name, partitions, retention)", "Message(id, topic, partition, offset, key, payload)", "ConsumerOffset(group, topic, partition, offset)"],
        "events": ["message.published", "message.acked", "message.dead_lettered"],
        "storage": "Append-only partition logs for messages; metadata store for topics, leases, and offsets.",
        "consistency": "Guarantee at-least-once delivery; ordering is per partition.",
        "caching": "Cache topic metadata and producer partition choices.",
        "failures": ["consumer crash after processing before ack", "partition leader unavailable", "poison message", "producer retry duplicates"],
        "milestones": ["Single-topic queue", "acks and visibility timeout", "partitioned topics and consumer groups", "DLQ and benchmark"],
        "interview": ["Compare queues and streams.", "Explain ordering and partitioning.", "Discuss delivery semantics and idempotent consumers."],
    },
]


PHASE2_PLUS = [
    (14, "1:1 Chat System", 2, "Java/Spring Boot, WebSockets, Kafka, PostgreSQL, Redis", "Deliver realtime private messages with online presence, history, receipts, and offline push.", "Direct messages in social, support, or collaboration apps.", ["WebSocket session management", "message ordering", "presence", "read receipts"], ["POST /v1/conversations", "WS /v1/chat", "GET /v1/conversations/{id}/messages"], ["Conversation(id, user_a, user_b)", "Message(id, conversation_id, sender_id, body, seq, status)", "Presence(user_id, state, last_seen)"], ["message.sent", "message.delivered", "presence.changed"], "PostgreSQL for durable history, Kafka for fanout, Redis for sessions and presence.", "Ordering per conversation using sequence assignment.", "Cache recent conversations and online presence.", ["WebSocket reconnect", "duplicate send", "out-of-order delivery", "offline user"], ["REST conversation API", "WebSocket send/receive", "delivery/read receipts", "offline push integration"], ["Explain message ordering per conversation.", "Discuss WebSocket scaling and sticky sessions.", "Handle idempotent sends."]),
    (15, "Group Chat System", 2, "Java/Spring Boot, WebSockets, Kafka, PostgreSQL, Redis", "Support multi-user rooms with membership, fanout, moderation, history, and delivery state.", "Team collaboration, communities, and multiplayer social rooms.", ["fanout strategies", "membership authorization", "room sequencing", "moderation controls"], ["POST /v1/rooms", "POST /v1/rooms/{id}/messages", "WS /v1/rooms/{id}"], ["Room(id, owner_id, type)", "Membership(room_id, user_id, role)", "GroupMessage(id, room_id, sender_id, seq, body)"], ["room.message_sent", "room.member_joined", "room.moderation_action"], "PostgreSQL for room metadata/history, Kafka for fanout, Redis for active subscriptions.", "Order messages per room; membership checks must be strongly consistent enough to prevent unauthorized reads.", "Cache room membership and recent history.", ["large-room fanout", "membership cache stale", "moderation race", "notification storm"], ["Room CRUD", "group messaging over WebSocket", "roles and moderation", "large-room fanout benchmark"], ["Compare write fanout and read fanout.", "Discuss large group scaling.", "Explain membership correctness."]),
    (16, "News Feed System", 2, "Java/Spring Boot, Kafka, PostgreSQL, Redis, OpenSearch optional", "Generate personalized feeds from follows, posts, ranking signals, and fanout pipelines.", "Social apps, professional networks, and content platforms.", ["fanout-on-write/read", "ranking", "timeline materialization", "backfill"], ["POST /v1/posts", "GET /v1/feed", "POST /v1/follows"], ["Post(id, author_id, body, created_at)", "Follow(follower_id, followee_id)", "FeedItem(user_id, post_id, score, created_at)"], ["post.created", "follow.created", "feed_item.materialized"], "PostgreSQL for graph and posts, Redis for home timelines, Kafka for fanout.", "Eventual feed consistency; post creation remains durable before fanout.", "Cache home feed pages and author profile timelines.", ["celebrity fanout", "stale feed", "ranking job failure", "deleted post remains in feed"], ["Post/follow APIs", "fanout worker", "ranked home feed", "celebrity fallback path"], ["Explain fanout tradeoffs.", "Discuss feed ranking vs recency.", "Handle deletes and privacy changes."]),
    (17, "Proximity Service", 2, "Java/Spring Boot, Redis GEO, PostgreSQL/PostGIS, Kafka", "Find nearby entities such as friends, drivers, restaurants, or devices with low-latency geo queries.", "Maps, social, logistics, dating, and marketplace products.", ["geohash", "spatial indexing", "location freshness", "privacy controls"], ["POST /v1/locations", "GET /v1/nearby?lat=&lng=", "DELETE /v1/locations/{user_id}"], ["Location(user_id, lat, lng, geohash, updated_at)", "Visibility(user_id, mode)", "NearbyQuery(user_id, radius_m, result_count)"], ["location.updated", "nearby.queried", "location.expired"], "Redis GEO for hot current locations, PostGIS for durable/history queries.", "Locations are ephemeral and eventually consistent; privacy changes should take priority.", "Cache geohash cells briefly for popular areas.", ["GPS noise", "stale location", "privacy violation", "dense urban hot cell"], ["Location ingest", "nearby query", "privacy filters", "load test dense cells"], ["Explain geohash precision.", "Discuss freshness vs battery/network cost.", "Handle privacy and deletion."]),
    (18, "Instagram Photo/Video Sharing and Feed", 2, "Java/Spring Boot, Kafka, PostgreSQL, Redis, S3-compatible storage, CDN", "Upload media, process variants, store social graph, and serve ranked feeds.", "Consumer social media product with media-heavy workloads.", ["media upload pipeline", "fanout feed", "CDN integration", "async processing"], ["POST /v1/media/uploads", "POST /v1/posts", "GET /v1/feed", "POST /v1/posts/{id}/likes"], ["Media(id, owner_id, object_key, variants)", "Post(id, user_id, media_id, caption)", "Engagement(post_id, user_id, type)"], ["media.uploaded", "media.processed", "post.created", "post.liked"], "PostgreSQL for metadata, object storage for media, Redis for feed/cache, Kafka for processing.", "Post metadata durable before async media variants finish; feed is eventually consistent.", "CDN for media, Redis for feeds and counters.", ["failed transcoding", "viral post", "copyright/abuse report", "deleted media cached"], ["Upload and metadata", "thumbnail/variant worker", "feed and engagement", "CDN/cache invalidation plan"], ["Discuss media pipeline stages.", "Explain fanout for followers.", "Cover counters, ranking, and cache invalidation."]),
    (19, "Twitter/X Timeline and Posts", 2, "Java/Spring Boot, Kafka, PostgreSQL, Redis, OpenSearch", "Support short posts, follows, timelines, search, trends, and high-fanout accounts.", "Public social network and microblogging platform.", ["timeline fanout", "celebrity accounts", "search indexing", "trend aggregation"], ["POST /v1/posts", "GET /v1/home", "GET /v1/users/{id}/timeline", "GET /v1/search"], ["Tweet(id, author_id, text, created_at)", "Follow(follower_id, followee_id)", "TimelineItem(user_id, tweet_id, score)"], ["tweet.created", "tweet.deleted", "timeline.fanout_completed"], "PostgreSQL for tweets/graph, Redis for timelines, Kafka for fanout/search/trends.", "Durable tweet creation; timelines and search are eventually consistent.", "Cache home timelines, user timelines, and engagement counters.", ["celebrity fanout", "abusive content", "search lag", "counter inconsistency"], ["Tweet/follow API", "home timeline", "search indexer", "celebrity hybrid fanout"], ["Compare Twitter timeline strategies.", "Discuss hot accounts and ranking.", "Explain eventual consistency for search and trends."]),
    (20, "WhatsApp Real-Time Messaging", 2, "Java/Spring Boot, Netty/WebSockets, Kafka, PostgreSQL/Cassandra, Redis", "Deliver secure realtime messages with device sessions, groups, receipts, and offline sync.", "Mobile messaging app with high reliability expectations.", ["mobile connection lifecycle", "multi-device sync", "delivery receipts", "end-to-end encryption boundaries"], ["WS /v1/session", "POST /v1/messages", "GET /v1/sync"], ["Device(id, user_id, public_key)", "Message(id, chat_id, sender, recipient, ciphertext, status)", "Receipt(message_id, device_id, state)"], ["message.enqueued", "message.delivered", "device.connected"], "Cassandra/PostgreSQL for message history, Redis for sessions, Kafka for routing and push.", "Ordering per chat/device; server stores ciphertext and delivery metadata only.", "Cache device routes and active sessions.", ["mobile reconnect", "duplicate delivery", "device key rotation", "offline backlog"], ["Device/session model", "encrypted message routing", "receipts and sync", "group messaging path"], ["Discuss E2EE impact on server features.", "Explain delivery guarantees.", "Handle offline and multi-device sync."]),
    (21, "Dropbox File Storage and Sync", 2, "Java/Spring Boot, PostgreSQL, S3-compatible storage, Kafka, Redis, desktop sync mock", "Store files, version metadata, sync changes across devices, and support conflict resolution.", "Cloud file storage and collaboration products.", ["chunked uploads", "delta sync", "metadata tree", "conflict handling"], ["POST /v1/files/upload-session", "GET /v1/files/{id}", "GET /v1/sync?cursor="], ["FileNode(id, parent_id, owner_id, name, type)", "FileVersion(file_id, version, object_key, checksum)", "SyncCursor(device_id, last_event_id)"], ["file.created", "file.version_added", "file.deleted"], "PostgreSQL for metadata tree, object storage for chunks/files, Kafka as change log.", "Metadata operations need transactional semantics; sync delivery can be at-least-once.", "Cache folder listings and sync cursors.", ["conflicting edits", "partial upload", "deleted file cached", "large folder listing"], ["Metadata tree API", "chunked upload", "sync cursor feed", "conflict resolution demo"], ["Explain metadata vs blob storage.", "Discuss sync cursors and idempotency.", "Handle conflicts and large files."]),
    (22, "Ticket Booking System", 2, "Java/Spring Boot, PostgreSQL, Redis, Kafka, payment mock", "Sell limited inventory without overselling while supporting holds, payment windows, and seat maps.", "Events, cinema, travel, and reservation platforms.", ["inventory locking", "payment saga", "seat map reads", "idempotent checkout"], ["GET /v1/events/{id}/seats", "POST /v1/holds", "POST /v1/bookings"], ["Seat(event_id, seat_id, status)", "Hold(id, seat_id, user_id, expires_at)", "Booking(id, hold_id, payment_status)"], ["hold.created", "hold.expired", "booking.confirmed", "payment.failed"], "PostgreSQL for inventory and bookings, Redis for short-lived holds, Kafka for expiration/payment workflow.", "Prevent oversell with transactional seat state changes or atomic Redis reservation backed by reconciliation.", "Cache seat maps but bypass or refresh for selected seats.", ["payment timeout", "hold expiry race", "oversell", "flash-sale traffic"], ["Event and seat APIs", "hold and expiry worker", "booking/payment saga", "flash-sale load test"], ["Explain locking choices.", "Discuss sagas and compensation.", "Handle high concurrency on popular events."]),
    (23, "E-commerce Platform", 2, "Java/Spring Boot, Kafka, PostgreSQL, Redis, OpenSearch, payment mock", "Build catalog, cart, checkout, inventory, order management, and event-driven fulfillment.", "Marketplace or retail backend with transactional and search-heavy flows.", ["catalog search", "cart state", "inventory reservation", "order saga"], ["GET /v1/products", "POST /v1/cart/items", "POST /v1/orders", "GET /v1/orders/{id}"], ["Product(id, sku, price, stock)", "Cart(user_id, items)", "Order(id, user_id, status, total)", "InventoryReservation(order_id, sku, qty)"], ["order.created", "inventory.reserved", "payment.authorized", "order.shipped"], "PostgreSQL for orders/inventory, OpenSearch for catalog, Redis for cart/session, Kafka for workflows.", "Checkout uses saga with idempotency; catalog/search is eventually consistent.", "Cache product detail, inventory hints, and carts.", ["inventory race", "payment failure", "search index lag", "cart merge conflict"], ["Catalog/search", "cart and checkout", "order saga", "admin operations and metrics"], ["Explain transactional boundaries.", "Discuss inventory reservation.", "Cover idempotency in checkout."]),
    (24, "Recommendation System", 2, "Python/FastAPI, Kafka, PostgreSQL, Redis, feature store, vector DB optional", "Generate personalized recommendations using behavior events, ranking features, and online serving.", "Media, commerce, feed, and learning platforms.", ["candidate generation", "feature pipelines", "online ranking", "feedback loops"], ["POST /v1/events", "GET /v1/recommendations/{user_id}", "POST /v1/models/promote"], ["UserEvent(user_id, item_id, action, ts)", "FeatureVector(entity_id, values, version)", "Recommendation(user_id, item_id, score, reason)"], ["user.event_logged", "features.updated", "model.promoted"], "PostgreSQL for metadata, Kafka for events, Redis/feature store for online features, object storage for model artifacts.", "Recommendations can be eventually consistent; model promotion must be controlled and reversible.", "Cache recommendation lists with short TTL and diversity constraints.", ["cold start", "feedback loop", "model regression", "feature skew"], ["Event ingestion", "baseline popularity recommender", "feature pipeline", "online ranking service"], ["Explain candidate generation vs ranking.", "Discuss offline/online feature skew.", "Handle cold start and evaluation."]),
    (25, "Distributed Cache", 3, "Go, Redis protocol subset, consistent hashing, gossip, Docker", "Build a multi-node cache with sharding, replication, failover, and client routing.", "Platform teams needing low-latency shared cache infrastructure.", ["consistent hashing", "replication", "node membership", "client routing"], ["PUT /v1/cache/{key}", "GET /v1/cache/{key}", "POST /v1/cluster/nodes"], ["CacheNode(id, address, status)", "Shard(id, primary, replicas)", "ReplicationEntry(key, version, value)"], ["node.joined", "shard.moved", "replica.synced"], "In-memory nodes with optional AOF; cluster metadata persisted in a coordinator or gossip state.", "Cache is eventually consistent across replicas; primary handles writes for a shard.", "Local node memory plus client-side routing cache.", ["node failure", "rebalance data loss", "split brain", "hot shard"], ["Single-node cache", "consistent-hash cluster", "replication and failover", "rebalance benchmark"], ["Compare Redis Cluster concepts.", "Explain replication lag.", "Discuss client vs proxy routing."]),
    (26, "Uber Ride-Sharing and Matching", 3, "Java/Spring Boot, Kafka, Redis GEO, PostgreSQL/PostGIS, WebSockets", "Match riders to nearby drivers, track trips, price rides, and stream status updates.", "Ride-sharing, logistics, and dispatch platforms.", ["geo indexing", "matching algorithms", "state machines", "surge pricing"], ["POST /v1/rides", "POST /v1/drivers/location", "POST /v1/rides/{id}/accept", "WS /v1/rides/{id}"], ["Ride(id, rider_id, status, pickup, dropoff)", "Driver(id, status, current_location)", "Offer(ride_id, driver_id, expires_at)"], ["ride.requested", "driver.location_updated", "ride.matched", "ride.completed"], "PostGIS for durable geo/trips, Redis GEO for active drivers, Kafka for dispatch events.", "Ride state transitions must be guarded; location and offers are eventually consistent.", "Cache active driver locations and route estimates briefly.", ["driver accepts expired offer", "GPS drift", "surge hotspot", "dispatch worker crash"], ["Rider/driver APIs", "matching loop", "trip state machine", "simulation dashboard"], ["Explain matching under latency constraints.", "Discuss state transitions and idempotency.", "Handle hotspots and driver fairness."]),
    (27, "Netflix Video Streaming Platform", 3, "Java/Spring Boot, S3-compatible storage, CDN, Kafka, PostgreSQL, FFmpeg workers", "Ingest videos, transcode variants, manage catalog metadata, and stream through CDN-friendly URLs.", "Subscription media streaming platform.", ["transcoding pipeline", "adaptive bitrate", "CDN caching", "playback sessions"], ["POST /v1/videos/uploads", "GET /v1/catalog", "POST /v1/playback-sessions"], ["Video(id, title, status)", "Rendition(video_id, bitrate, object_key)", "PlaybackSession(id, user_id, video_id, license)"], ["video.uploaded", "video.transcoded", "playback.started"], "PostgreSQL for catalog, object storage for originals/renditions, Kafka for pipeline orchestration.", "Catalog updates are durable; renditions appear after async processing.", "CDN caches segments and manifests; Redis caches catalog metadata.", ["transcode failure", "regional CDN miss", "license check latency", "popular release surge"], ["Upload/catalog", "transcode worker", "HLS manifest generation", "playback analytics"], ["Explain adaptive bitrate streaming.", "Discuss CDN cache strategy.", "Separate control plane and data plane."]),
    (28, "YouTube Video Upload and Streaming", 3, "Java/Spring Boot, Kafka, S3-compatible storage, CDN, OpenSearch, PostgreSQL", "Support creator uploads, video processing, search, comments, subscriptions, and streaming.", "Creator video platform with upload and discovery workflows.", ["resumable uploads", "processing workflow", "search indexing", "creator analytics"], ["POST /v1/upload-sessions", "POST /v1/videos/{id}/publish", "GET /v1/watch/{id}", "GET /v1/search"], ["Video(id, channel_id, status, metadata)", "UploadPart(session_id, part_no, checksum)", "Comment(id, video_id, user_id, body)"], ["upload.completed", "video.processed", "video.published", "comment.created"], "PostgreSQL for metadata, object storage/CDN for media, OpenSearch for discovery, Kafka for pipeline events.", "Publishing waits for required processing; search and recommendations lag publication.", "CDN for video segments, Redis for watch metadata and counters.", ["upload interruption", "copyright flag", "comment spam", "processing backlog"], ["Resumable upload", "processing pipeline", "watch page API", "search and comments"], ["Discuss upload chunking.", "Explain processing fanout.", "Handle counters and viral videos."]),
    (29, "TikTok Short-Video Platform", 3, "Java/Spring Boot, Kafka, Redis, S3-compatible storage, CDN, Python ranking service", "Serve infinite short-video feeds with fast upload, engagement tracking, and ranking feedback.", "Short-form video and recommendation-heavy social product.", ["low-latency feed serving", "ranking feedback loop", "media CDN", "creator graph"], ["POST /v1/videos", "GET /v1/feed/for-you", "POST /v1/events/view", "POST /v1/follows"], ["ShortVideo(id, creator_id, object_key)", "WatchEvent(user_id, video_id, duration_ms)", "FeedCandidate(user_id, video_id, score)"], ["video.uploaded", "watch.logged", "candidate.generated"], "Object storage/CDN for video, Kafka for events, Redis for feed cache, feature store for ranking features.", "Engagement and recommendations are eventually consistent; upload metadata is durable.", "Cache per-user feed pages and hot video metadata.", ["ranking filter bubble", "viral video load", "event ingestion lag", "unsafe content"], ["Upload and playback", "event tracking", "baseline For You feed", "ranking feature pipeline"], ["Explain candidate generation/ranking loop.", "Discuss video CDN and prefetch.", "Handle safety and freshness."]),
    (30, "Facebook-Like Social Network News Feed", 3, "Java/Spring Boot, Kafka, PostgreSQL, Redis, graph store optional", "Design a rich social feed with friendships, groups, pages, ranking, privacy, and reactions.", "Large social network feed system.", ["privacy-aware feed generation", "graph relationships", "ranking signals", "multi-object feed items"], ["POST /v1/stories", "GET /v1/feed", "POST /v1/friendships", "POST /v1/reactions"], ["Story(id, actor_id, target_type, privacy)", "Edge(user_a, user_b, type)", "FeedStory(user_id, story_id, score)"], ["story.created", "privacy.changed", "feed.ranked"], "PostgreSQL for entities, Redis for feeds/counters, Kafka for fanout and ranking, graph store optional.", "Privacy changes must remove unauthorized visibility; feed freshness can be eventual.", "Cache feed pages with viewer-specific privacy checks.", ["privacy regression", "celebrity/page fanout", "ranking drift", "reaction counter lag"], ["Social graph", "story publish", "privacy-aware feed", "ranking and reactions"], ["Explain privacy filtering order.", "Compare graph models.", "Discuss fanout and ranking at scale."]),
    (31, "Google Docs Real-Time Collaborative Editing", 3, "TypeScript/Node or Java, WebSockets, PostgreSQL, Redis, CRDT library, React editor", "Allow multiple users to edit the same document concurrently with presence, comments, and version history.", "Collaborative productivity and knowledge tools.", ["CRDT/OT concepts", "presence", "conflict-free merges", "snapshotting"], ["POST /v1/docs", "WS /v1/docs/{id}/sync", "GET /v1/docs/{id}/versions"], ["Document(id, owner_id, title)", "Operation(doc_id, seq, client_id, payload)", "Snapshot(doc_id, version, object_key)"], ["doc.operation_submitted", "doc.snapshot_created", "presence.changed"], "PostgreSQL for metadata/operations, Redis pub-sub for sessions, object storage for snapshots.", "Use CRDT operations to converge across clients; persist ordered operation log.", "Cache active document state and presence in memory/Redis.", ["client offline edit", "operation replay bug", "large document snapshot", "presence leak"], ["Basic editor", "WebSocket sync", "CRDT persistence", "version history and comments"], ["Compare OT and CRDT.", "Discuss operation log compaction.", "Handle offline clients and convergence."]),
    (32, "Content Delivery Network CDN", 3, "Go, reverse proxy, object storage origin, Redis, Prometheus, Terraform notes", "Cache static assets at edge nodes, route users to nearby PoPs, and purge content safely.", "Media, web, and API acceleration platforms.", ["edge caching", "cache keys", "purge propagation", "origin shielding"], ["GET /edge/{path}", "POST /v1/purge", "PUT /v1/origins/{id}", "GET /v1/cache/stats"], ["Origin(id, base_url)", "CacheObject(key, etag, ttl, status)", "PurgeRequest(id, pattern, status)"], ["object.cached", "object.purged", "origin.fetch_failed"], "Edge disk/memory cache, Redis for purge coordination, object storage or HTTP origin.", "Content freshness follows TTL and purge version; purge should be idempotent and observable.", "Multi-layer memory/disk/origin-shield cache.", ["cache poisoning", "purge storm", "origin outage", "large object eviction"], ["Single edge proxy", "TTL and cache keys", "purge API", "multi-edge simulation"], ["Explain CDN request flow.", "Discuss purge vs TTL.", "Handle origin shielding and cache poisoning."]),
    (33, "Search Engine", 3, "Java/Python, crawler, Kafka, OpenSearch/Lucene, PostgreSQL", "Crawl, parse, index, rank, and query documents with relevance and freshness tradeoffs.", "Web, marketplace, docs, or internal search products.", ["inverted index", "ranking", "index refresh", "query serving"], ["POST /v1/documents", "GET /v1/search?q=", "POST /v1/admin/reindex"], ["Document(id, url, title, body, checksum)", "Posting(term, doc_id, tf)", "SearchClick(query, doc_id, rank)"], ["document.crawled", "document.indexed", "query.clicked"], "OpenSearch/Lucene for index, PostgreSQL for metadata, Kafka for indexing pipeline.", "Search index is eventually consistent; canonical documents are durable.", "Cache hot queries and term dictionaries.", ["index lag", "spam documents", "query hotspot", "bad ranking update"], ["Document ingestion", "inverted index search", "ranking signals", "crawler plus reindex pipeline"], ["Explain inverted indexes.", "Discuss freshness vs query latency.", "Handle ranking and spam."]),
    (34, "Google Maps Routing and Location Services", 3, "Java/Go, PostgreSQL/PostGIS, graph engine, Redis, Kafka", "Model places, roads, traffic, routing, geocoding, and location-aware queries.", "Maps, logistics, delivery, and mobility apps.", ["road graph modeling", "shortest path", "traffic updates", "geospatial indexing"], ["GET /v1/geocode", "GET /v1/routes?from=&to=", "POST /v1/traffic-events"], ["Place(id, name, lat, lng)", "RoadEdge(from, to, distance, speed)", "TrafficEvent(edge_id, speed, expires_at)"], ["traffic.updated", "route.requested", "place.indexed"], "PostGIS for places, graph storage for road network, Redis for route and tile caches.", "Traffic is eventually consistent; route requests use a timestamped graph snapshot.", "Cache popular routes, geocoding results, and map tiles.", ["traffic stale", "road closure", "route cache invalid", "large graph memory pressure"], ["Place search", "road graph import", "shortest path API", "traffic-aware routing"], ["Explain Dijkstra/A* tradeoffs.", "Discuss map tiling and geospatial indexes.", "Handle live traffic updates."]),
    (35, "Distributed Database", 3, "Go or Java, Raft library, gRPC, LSM storage, Docker Compose", "Build a small distributed database with replication, partitioning, leader election, and consistency modes.", "Infrastructure portfolio project for distributed storage interviews.", ["Raft consensus", "partitioning", "replication", "read/write consistency"], ["PUT /v1/tables/{table}/keys/{key}", "GET /v1/tables/{table}/keys/{key}", "POST /v1/admin/rebalance"], ["Table(name, partitions)", "Partition(id, leader, replicas)", "LogEntry(term, index, command)"], ["leader.elected", "partition.split", "replica.caught_up"], "Per-node LSM/WAL storage plus replicated consensus log per partition.", "Linearizable writes through partition leader; optional follower reads with staleness bounds.", "Block cache and routing metadata cache.", ["leader failure", "network partition", "slow replica", "split-brain prevention"], ["Single partition Raft KV", "multi-partition routing", "rebalancing", "consistency test suite"], ["Explain consensus vs eventual replication.", "Discuss partitioning and rebalancing.", "Handle linearizable reads."]),
    (36, "Real-Time Analytics System", 4, "Python/FastAPI, Kafka, Flink-style workers, ClickHouse, Redis, Grafana", "Ingest events and compute near-real-time metrics, funnels, and dashboards.", "Product analytics, observability, and business intelligence teams.", ["stream processing", "windowed aggregations", "late events", "OLAP serving"], ["POST /v1/events", "GET /v1/metrics/{name}", "GET /v1/funnels"], ["Event(id, user_id, type, ts, properties)", "Metric(name, window, value)", "Dashboard(id, queries)"], ["event.ingested", "metric.updated", "late_event.detected"], "Kafka for event log, ClickHouse for OLAP, Redis for hot aggregates.", "At-least-once ingestion with dedupe by event ID; aggregates tolerate bounded correction.", "Cache dashboard queries and hot metrics.", ["late events", "duplicate events", "consumer lag", "high-cardinality dimensions"], ["Event collector", "stream aggregation", "ClickHouse queries", "dashboard and alerts"], ["Explain event-time vs processing-time.", "Discuss OLAP schema design.", "Handle dedupe and late arrivals."]),
    (37, "Ad Serving and Tracking System", 4, "Java/Spring Boot, Kafka, Redis, PostgreSQL, ClickHouse", "Select ads under targeting, pacing, budget, and latency constraints while tracking impressions and clicks.", "Ad tech platforms and marketplace monetization systems.", ["auction/ranking", "budget pacing", "tracking attribution", "low-latency serving"], ["POST /v1/ad-request", "POST /v1/track/impression", "POST /v1/track/click"], ["Campaign(id, budget, bid, targeting)", "AdRequest(user_context, placement)", "Impression(id, campaign_id, price)"], ["ad.served", "impression.logged", "click.logged", "budget.updated"], "Redis for hot campaign/budget state, PostgreSQL for campaign config, ClickHouse for event analytics.", "Budget deductions need strong-enough atomicity; tracking is at-least-once with dedupe.", "Cache eligible campaigns by segment and placement.", ["overspend", "tracking fraud", "latency SLA breach", "campaign config lag"], ["Campaign API", "ad selection", "tracking pipeline", "pacing and analytics"], ["Explain serving latency budget.", "Discuss pacing and budget correctness.", "Handle click fraud and attribution."]),
    (38, "Fraud Detection System", 4, "Python/FastAPI, Kafka, Redis, feature store, PostgreSQL, model service", "Score transactions or actions for fraud using rules, features, and ML models with human review.", "Payments, marketplaces, fintech, and trust/safety teams.", ["streaming features", "rules engine", "model inference", "case management"], ["POST /v1/score", "POST /v1/events", "GET /v1/cases/{id}"], ["RiskEvent(id, actor_id, type, amount)", "Feature(entity_id, name, value)", "Case(id, score, status, reviewer)"], ["risk.event_received", "score.generated", "case.opened"], "Kafka for events, Redis/feature store for online features, PostgreSQL for cases and decisions.", "Scoring must be synchronous for high-risk actions; feature updates are eventually consistent.", "Cache hot entity features with TTL and version metadata.", ["feature missing", "model outage", "false positive spike", "fraud pattern shift"], ["Rule-based scorer", "feature pipeline", "ML model endpoint", "review queue and feedback"], ["Explain real-time vs batch fraud detection.", "Discuss precision/recall tradeoffs.", "Handle model fallback and auditability."]),
    (39, "Stock Trading Exchange System", 4, "Java or Rust, Kafka for market data, PostgreSQL audit, in-memory matching engine", "Match buy/sell orders deterministically while publishing market data and preserving auditability.", "Financial exchange or trading venue simulation.", ["order books", "price-time priority", "low-latency matching", "audit logs"], ["POST /v1/orders", "DELETE /v1/orders/{id}", "GET /v1/books/{symbol}"], ["Order(id, account, symbol, side, price, qty, ts)", "Trade(id, buy_order, sell_order, price, qty)", "BookLevel(symbol, side, price, qty)"], ["order.accepted", "trade.executed", "market_data.published"], "In-memory order book with append-only journal and PostgreSQL for audit/read models.", "Matching engine is single-writer deterministic per symbol partition.", "Cache market data snapshots for readers.", ["engine crash", "duplicate order", "market data lag", "clock/timestamp issues"], ["Single-symbol book", "order matching", "journal recovery", "market data feed"], ["Explain price-time priority.", "Discuss deterministic replay.", "Separate matching path from query path."]),
    (40, "Distributed Job Scheduler", 4, "Go, PostgreSQL, Redis, gRPC, Docker, Kubernetes notes", "Schedule one-off and recurring jobs across workers with leases, retries, priorities, and dependencies.", "Data pipelines, background task platforms, and internal automation.", ["leases", "cron scheduling", "dependency DAGs", "worker heartbeats"], ["POST /v1/jobs", "POST /v1/schedules", "POST /v1/jobs/{id}:cancel", "GET /v1/jobs/{id}"], ["Job(id, type, payload, status, run_at)", "Attempt(job_id, worker_id, status)", "Schedule(id, cron, next_run_at)"], ["job.scheduled", "job.started", "job.completed", "job.failed"], "PostgreSQL for durable jobs and schedules, Redis for worker heartbeats and fast queues.", "A job attempt is protected by a lease; workers must be idempotent.", "Cache runnable queues by priority while preserving DB as source of truth.", ["worker dies mid-job", "duplicate execution", "cron drift", "dependency deadlock"], ["Basic job queue", "workers and leases", "cron scheduler", "DAG and retry policies"], ["Explain lease-based scheduling.", "Discuss exactly-once myth.", "Handle retries and idempotency."]),
    (41, "Event Sourcing and CQRS Architecture", 4, "Java/Spring Boot, Kafka, PostgreSQL event store, Redis, React admin", "Model state changes as immutable events and maintain separate read models for query needs.", "Order, banking, logistics, and audit-heavy domains.", ["event modeling", "projections", "snapshots", "replay/backfill"], ["POST /v1/commands", "GET /v1/read-models/{type}/{id}", "POST /v1/admin/replay"], ["Event(stream_id, version, type, payload)", "Snapshot(stream_id, version, state)", "Projection(name, offset, status)"], ["domain.event_appended", "projection.updated", "replay.started"], "PostgreSQL as event store, Kafka for projection fanout, Redis for hot read models.", "Optimistic concurrency per stream; projections are eventually consistent.", "Cache read models and snapshot aggregate state.", ["projection lag", "bad event schema", "replay overload", "duplicate command"], ["Event store", "command handler", "read projection", "schema evolution and replay"], ["Explain CQRS tradeoffs.", "Discuss event versioning.", "Handle replay and projection consistency."]),
    (42, "Multi-Tenant SaaS Platform", 4, "Java/Spring Boot, PostgreSQL, Redis, Kafka, React admin, Kubernetes notes", "Serve multiple tenants with isolation, billing hooks, roles, quotas, and operational controls.", "B2B SaaS platform foundation.", ["tenant isolation", "RBAC", "quotas", "billing and audit"], ["POST /v1/tenants", "POST /v1/users/invite", "GET /v1/audit-log", "PUT /v1/quotas"], ["Tenant(id, plan, status)", "Membership(user_id, tenant_id, role)", "AuditLog(tenant_id, actor, action)"], ["tenant.created", "quota.exceeded", "audit.logged"], "PostgreSQL with tenant-aware schema strategy, Redis for quotas/sessions, Kafka for audit and billing events.", "Tenant isolation is mandatory; background events carry tenant ID and authorization context.", "Cache tenant config and permission checks carefully by tenant.", ["cross-tenant data leak", "noisy neighbor", "quota bypass", "tenant migration failure"], ["Tenant model", "RBAC and audit", "quotas and billing events", "tenant isolation tests"], ["Compare shared DB vs schema vs database per tenant.", "Discuss noisy-neighbor controls.", "Explain tenant-aware observability."]),
    (43, "Live Video Streaming at Scale", 4, "Go/Java, WebRTC or HLS, Kafka, Redis, CDN, FFmpeg workers", "Ingest live streams, transcode, distribute to viewers, and track stream health.", "Live events, gaming, education, and social streaming.", ["low-latency ingest", "transcoding ladders", "viewer fanout", "stream health"], ["POST /v1/streams", "POST /v1/streams/{id}/start", "GET /v1/streams/{id}/playback", "POST /v1/streams/{id}/chat"], ["LiveStream(id, creator_id, status)", "Segment(stream_id, seq, object_key)", "ViewerSession(id, stream_id, region)"], ["stream.started", "segment.created", "viewer.joined", "stream.ended"], "Redis for live state, object storage/CDN for HLS segments, Kafka for health and chat events.", "Live metadata is strongly guarded by state machine; segment availability is eventually consistent.", "CDN caches segments briefly; Redis caches stream manifests and viewer counts.", ["ingest dropout", "transcode lag", "chat flood", "regional viewer surge"], ["Stream lifecycle", "segment generation", "playback manifest", "health dashboard and chat"], ["Compare WebRTC and HLS.", "Discuss latency vs scale.", "Handle stream failures and viewer counts."]),
    (44, "Highly Scalable NoSQL Database", 4, "Go/Java, LSM storage, gossip, consistent hashing, Raft optional", "Design a Dynamo/Cassandra-style database with partitioning, replication, quorum reads/writes, and repair.", "Distributed database infrastructure practice.", ["quorum consistency", "gossip membership", "hinted handoff", "anti-entropy repair"], ["PUT /v1/tables/{table}/{key}", "GET /v1/tables/{table}/{key}", "POST /v1/admin/repair"], ["Table(name, replication_factor)", "Replica(key_range, node_id)", "VersionedValue(key, vector_clock, value)"], ["replica.write", "hint.stored", "repair.completed"], "LSM storage per node, consistent-hash ring metadata, gossip for membership.", "Tunable consistency with R/W quorums; conflict resolution by vector clocks or timestamps.", "Bloom filters, block cache, and routing cache.", ["conflicting writes", "node outage", "repair backlog", "hot partition"], ["Single-node LSM", "ring and replication", "quorum reads/writes", "repair and hinted handoff"], ["Explain Dynamo-style tradeoffs.", "Discuss quorum math.", "Handle conflict resolution and repairs."]),
    (45, "Real-Time Multiplayer Game Backend", 4, "Go, WebSockets/UDP, Redis, PostgreSQL, Kafka, Kubernetes notes", "Run authoritative game sessions with matchmaking, realtime state sync, leaderboards, and anti-cheat hooks.", "Competitive or cooperative online games.", ["authoritative server", "tick loop", "matchmaking", "state reconciliation"], ["POST /v1/matchmaking/join", "WS /v1/matches/{id}", "GET /v1/leaderboards"], ["Player(id, rating)", "Match(id, players, region, status)", "GameTick(match_id, seq, state_delta)"], ["match.created", "player.input_received", "match.completed"], "Redis for matchmaking/session state, PostgreSQL for accounts/results, Kafka for analytics.", "Server is authoritative; clients send inputs and receive state snapshots/deltas.", "Cache leaderboards and active match placement.", ["player disconnect", "lag compensation", "cheat attempt", "match server crash"], ["Matchmaking", "realtime session server", "leaderboards", "lag/chaos testing"], ["Explain authoritative game servers.", "Discuss tick rate and network tradeoffs.", "Handle matchmaking fairness and failures."]),
    (46, "Machine Learning Model Serving Infrastructure", 4, "Python/FastAPI, model registry, Redis, Kafka, Kubernetes, Prometheus", "Serve ML predictions with versioning, canaries, feature lookup, batching, and monitoring.", "ML platform and applied AI production teams.", ["model registry", "online inference", "batching", "model monitoring"], ["POST /v1/predict/{model}", "POST /v1/models/{model}/versions", "POST /v1/models/{model}/promote"], ["ModelVersion(model, version, artifact_uri, status)", "PredictionRequest(id, features)", "PredictionLog(model, version, latency, output)"], ["model.promoted", "prediction.logged", "drift.detected"], "Object storage for artifacts, PostgreSQL for registry, Redis/feature store for online features, Kafka for logs.", "Prediction path must pin a model version per request; promotion is controlled and auditable.", "Cache loaded models and hot features; batch compatible requests.", ["model load failure", "feature skew", "latency spike", "bad model promotion"], ["Registry API", "prediction service", "canary routing", "monitoring and rollback"], ["Explain model versioning.", "Discuss online/offline feature consistency.", "Handle canaries and rollback."]),
    (47, "Geo-Distributed Low-Latency System", 5, "Go/Java, Kubernetes, Terraform, Redis, PostgreSQL/CockroachDB notes, service mesh", "Serve users from multiple regions while minimizing latency and surviving regional failure.", "Global SaaS, social, commerce, and collaboration platforms.", ["regional routing", "data locality", "active-active tradeoffs", "failover"], ["GET /v1/edge/profile", "POST /v1/write", "GET /v1/health/regions"], ["Region(id, status, latency)", "UserHomeRegion(user_id, region)", "ReplicationLag(source, target, ms)"], ["region.degraded", "write.replicated", "traffic.shifted"], "Regional caches and services, globally replicated metadata store, asynchronous replication for non-critical data.", "Classify data by consistency need: local reads, home-region writes, or global consensus for critical records.", "Edge and regional caches with locality-aware invalidation.", ["region outage", "replication lag", "misrouted write", "data residency conflict"], ["Single-region app", "multi-region read routing", "home-region writes", "failover drill plan"], ["Explain active-active vs active-passive.", "Discuss data locality and residency.", "Handle failover without data loss."]),
    (48, "Strongly Consistent Global Database", 5, "Go/Java, Raft/Paxos concepts, TrueTime-style notes, Kubernetes, Jepsen-style tests", "Design a globally replicated database that offers strong consistency across regions.", "Critical financial, identity, and metadata systems.", ["consensus across regions", "transaction timestamps", "serializability", "latency tradeoffs"], ["BEGIN /v1/tx", "PUT /v1/tx/{id}/keys/{key}", "POST /v1/tx/{id}/commit", "GET /v1/keys/{key}"], ["Transaction(id, status, read_ts, commit_ts)", "ReplicaGroup(id, regions, leader)", "KeyVersion(key, value, ts)"], ["tx.committed", "leader.changed", "replica.quorum_lost"], "Replicated log per shard plus MVCC storage; metadata service maps keys to replica groups.", "Serializable transactions through consensus and timestamp ordering; latency is bounded by quorum distance.", "Read-only transactions may use safe timestamps and regional replicas.", ["leader region outage", "clock uncertainty", "cross-shard transaction", "quorum loss"], ["Single-shard consensus KV", "MVCC reads", "transaction coordinator", "partition/failure consistency tests"], ["Explain why global strong consistency costs latency.", "Discuss Spanner-like timestamp ideas.", "Handle cross-shard transactions."]),
    (49, "High-Frequency Trading Platform", 5, "Rust or Java, low-latency networking, in-memory matching, append-only journal, market data feed", "Design an ultra-low-latency trading platform with deterministic matching, risk checks, and market data.", "Capital markets infrastructure and latency-sensitive systems.", ["low-latency path", "deterministic matching", "pre-trade risk", "market data fanout"], ["binary submit order protocol", "GET /v1/audit/orders/{id}", "market data stream"], ["Order(id, account, symbol, side, price, qty)", "RiskLimit(account, max_position)", "Trade(id, symbol, price, qty)"], ["order.accepted", "risk.rejected", "trade.executed", "market_data.tick"], "In-memory hot path with append-only journal; separate read/audit stores fed asynchronously.", "Single-threaded deterministic matching per symbol; every accepted command is journaled before effects are published.", "Keep hot books and risk limits in memory; avoid request-path distributed cache calls.", ["GC pause or tail latency spike", "journal disk stall", "bad risk limit", "market data sequencing gap"], ["Order protocol simulation", "risk checks", "matching engine", "latency benchmark and replay"], ["Explain mechanical sympathy and hot path design.", "Discuss deterministic replay.", "Separate HFT from general stock exchange design."]),
    (50, "Planet-Scale Distributed System", 5, "Go/Java/Rust design, Kubernetes, Terraform, multi-region data stores, service mesh, observability stack", "Design a system serving billions of users with multi-region high availability, graceful degradation, and operational excellence.", "Staff-level architecture practice for internet-scale products.", ["global control/data planes", "multi-region HA", "blast-radius control", "operability"], ["GET /v1/global/feed", "POST /v1/global/action", "GET /v1/status/global", "POST /v1/admin/traffic-shift"], ["Cell(id, region, capacity, status)", "GlobalUser(user_id, home_cell, replicas)", "SLO(service, region, objective, burn_rate)"], ["cell.degraded", "traffic.shifted", "global_action_committed", "slo_burn_alert"], "Cell-based regional architecture with local stores, global metadata, async replication, and consensus only for critical control records.",
     "Use data classification: local eventual data, home-cell transactional data, and globally consistent control-plane data.", "Edge caches, regional caches, and service-owned caches with explicit invalidation and degradation modes.", ["global dependency outage", "cascading failure", "bad deploy", "regional isolation"], ["Reference single-cell service", "cell routing and isolation", "multi-region replication", "game-day and incident review docs"], ["Explain cell architecture.", "Discuss graceful degradation.", "Show how SLOs, rollouts, and incident response shape the design."]),
]


for item in PHASE2_PLUS:
    (
        pid,
        title,
        phase,
        stack,
        problem,
        users,
        objectives,
        apis,
        models,
        events,
        storage,
        consistency,
        caching,
        failures,
        milestones,
        interview,
    ) = item
    PROJECTS.append(
        {
            "id": pid,
            "title": title,
            "phase": phase,
            "stack": stack,
            "problem": problem,
            "users": users,
            "objectives": objectives,
            "apis": apis,
            "models": models,
            "events": events,
            "storage": storage,
            "consistency": consistency,
            "caching": caching,
            "failures": failures,
            "milestones": milestones,
            "interview": interview,
        }
    )


def slugify(title):
    title = title.lower()
    title = title.replace("1:1", "one-to-one")
    title = title.replace("twitter/x", "twitter-x")
    title = title.replace("google docs", "google-docs")
    title = title.replace("facebook-like", "facebook-like")
    title = title.replace("machine learning", "ml")
    title = re.sub(r"[^a-z0-9]+", "-", title)
    return title.strip("-")


def wrap_list(items):
    return "\n".join(f"- {item}" for item in items)


def numbered_milestones(items):
    return "\n".join(f"{idx}. {item}: acceptance is a demo, test, or metric proving the behavior works." for idx, item in enumerate(items, 1))


def project_dir(project):
    return ROOT / "projects" / f"{project['id']:02d}-{slugify(project['title'])}"


TECH_RATIONALES = {
    1: [
        "Go keeps the request-path limiter small, fast, and easy to deploy beside gateways or services.",
        "Redis is the right fit for atomic counters, TTL-backed buckets, and sub-millisecond policy checks.",
        "PostgreSQL stores durable policies and audit summaries where relational constraints matter more than raw speed.",
    ],
    2: [
        "Go handles the very hot redirect path with low overhead and simple horizontal scaling.",
        "Redis absorbs read-heavy short-code lookups while PostgreSQL remains the canonical store for ownership and uniqueness.",
        "A small React dashboard makes analytics and link administration visible without bloating the core service.",
    ],
    3: [
        "Object storage is best for paste bodies because content size varies and blobs should not pressure relational tables.",
        "PostgreSQL fits metadata, visibility, expiration, and abuse workflows that need indexes and transactions.",
        "Go gives a compact API and background cleanup service with straightforward streaming upload/download support.",
    ],
    4: [
        "Go and gRPC suit a tiny high-throughput infrastructure service with low serialization overhead.",
        "PostgreSQL is only used for worker leases and audit state, keeping ID generation off the database hot path.",
        "Prometheus is essential because clock rollback, sequence exhaustion, and lease churn must be visible immediately.",
    ],
    5: [
        "Go is ideal for a reusable hashing library and CLI simulator because the core algorithm is small and performance-sensitive.",
        "A visualizer helps show key movement, virtual nodes, and weighting, which are the main learning goals.",
        "Persistence is intentionally light because the project is about placement behavior, not durable storage.",
    ],
    6: [
        "Go has strong standard-library support for HTTP reverse proxies and connection handling.",
        "Docker Compose makes it easy to simulate many backend instances, failures, and deploy draining locally.",
        "Prometheus and a traffic console expose the balancing decisions, retries, and backend health transitions.",
    ],
    7: [
        "Go keeps gateway latency low while supporting middleware-style routing, auth, and transformations.",
        "Redis is well suited for API key metadata, quota counters, and fast policy decisions.",
        "PostgreSQL keeps route, tenant, and key configuration durable and queryable for operators.",
    ],
    8: [
        "Go is a strong fit for implementing WAL, memtable, and SSTable primitives without runtime complexity.",
        "gRPC gives a clear service boundary while keeping the storage engine reusable behind another API later.",
        "Local files make durability mechanics visible, which is the point of building this database from first principles.",
    ],
    9: [
        "Go works well for an in-memory server that needs predictable latency and explicit memory accounting.",
        "A Redis-compatible subset makes the project practical to test with familiar clients and benchmarks.",
        "Prometheus metrics are central because hit rate, eviction rate, and memory pressure define cache quality.",
    ],
    10: [
        "Java/Spring Boot fits workflow-heavy enterprise services with validation, persistence, and operational conventions.",
        "Kafka is the natural backbone for asynchronous delivery, retries, dead-letter queues, and provider isolation.",
        "Redis handles provider rate limits and fast preference/template cache lookups.",
    ],
    11: [
        "Go keeps suggestion serving fast and memory-efficient for prefix-heavy read traffic.",
        "Redis sorted sets map naturally to ranked prefix suggestions and hot-prefix caching.",
        "PostgreSQL preserves the canonical corpus while OpenSearch can be added for fuzzy matching once prefix search works.",
    ],
    12: [
        "Go is well suited for many concurrent fetchers with explicit timeouts and backpressure.",
        "Kafka or NATS decouples URL discovery, fetching, parsing, and recrawl scheduling.",
        "Redis helps with dedupe and per-host politeness state while object storage keeps raw page snapshots cheap.",
    ],
    13: [
        "Go is appropriate for a log-oriented queue because concurrency, networking, and binary storage are first-class concerns.",
        "Append-only segments make offsets, replay, retention, and partition ordering concrete.",
        "Prometheus is necessary because queue lag, ack rate, retries, and DLQ growth are the system's health signals.",
    ],
    14: [
        "Java/Spring Boot is a strong fit for user, conversation, and delivery workflows around the realtime path.",
        "WebSockets model bidirectional chat sessions directly, while Kafka decouples message routing and offline delivery.",
        "Redis is suited for presence, active sessions, and quick reconnect state.",
    ],
    15: [
        "Java/Spring Boot handles membership, roles, moderation, and durable message APIs cleanly.",
        "Kafka supports room fanout and notification workflows without blocking message acceptance.",
        "Redis tracks active room subscribers and cached membership for the low-latency WebSocket path.",
    ],
    16: [
        "Java/Spring Boot suits social graph APIs and feed workflow orchestration.",
        "Kafka is the right fit for fanout pipelines, ranking jobs, and backfills.",
        "Redis serves materialized timelines quickly while PostgreSQL remains the durable source for posts and follows.",
    ],
    17: [
        "Redis GEO is purpose-built for fast nearby lookups over hot, frequently changing locations.",
        "PostGIS gives durable spatial querying and indexing for history, analytics, and correctness checks.",
        "Kafka allows location updates, expiry, and downstream notifications to be processed asynchronously.",
    ],
    18: [
        "Object storage and CDN are the natural choices for media bytes because the read path is bandwidth-heavy.",
        "Java/Spring Boot fits metadata, social graph, and engagement workflows.",
        "Kafka separates upload acceptance from thumbnailing, transcoding, feed fanout, and moderation.",
    ],
    19: [
        "Java/Spring Boot works well for timeline, follow, post, and moderation APIs.",
        "Kafka is essential for tweet fanout, search indexing, trends, and counter updates.",
        "Redis serves hot timelines and counters, while OpenSearch supports public post discovery.",
    ],
    20: [
        "Java with Netty/WebSockets is a strong pairing for many long-lived mobile connections.",
        "Cassandra or partitioned PostgreSQL fits high-volume message history by chat or user key.",
        "Redis tracks active devices and sessions, while Kafka decouples routing, receipts, and push delivery.",
    ],
    21: [
        "Object storage is best for file contents and chunks; PostgreSQL is best for the metadata tree and versions.",
        "Kafka provides an ordered change stream for sync cursors and device updates.",
        "Redis accelerates folder listings, cursors, and active sync state without becoming the source of truth.",
    ],
    22: [
        "Java/Spring Boot fits transactional booking workflows, validations, and payment integration patterns.",
        "PostgreSQL provides the strongest local fit for seat inventory constraints and booking records.",
        "Redis handles short-lived holds efficiently, while Kafka drives expiry and payment saga events.",
    ],
    23: [
        "Java/Spring Boot matches the enterprise shape of catalog, cart, checkout, order, and inventory services.",
        "OpenSearch is suited for catalog discovery, while PostgreSQL protects orders and inventory transitions.",
        "Kafka coordinates checkout, payment, fulfillment, and search-index updates without tight coupling.",
    ],
    24: [
        "Python/FastAPI fits recommendation work because model code, feature jobs, and experimentation usually live in Python.",
        "Kafka captures behavior events and supports streaming feature updates.",
        "Redis or a feature store keeps online ranking fast, while PostgreSQL stores item/user metadata.",
    ],
    25: [
        "Go is a natural fit for a multi-node cache where networking, membership, and memory behavior are central.",
        "A Redis protocol subset makes the system familiar and benchmarkable with existing client patterns.",
        "Consistent hashing and gossip directly teach the cluster behavior the project is meant to showcase.",
    ],
    26: [
        "Redis GEO gives low-latency active-driver lookup, which is the core matching bottleneck.",
        "PostGIS keeps durable trip geography and allows richer spatial queries outside the dispatch hot path.",
        "Kafka and WebSockets separate matching state changes from realtime rider/driver updates.",
    ],
    27: [
        "Object storage plus CDN matches the real streaming data plane: store once, deliver many times globally.",
        "Java/Spring Boot fits catalog, entitlement, and playback-session control-plane APIs.",
        "Kafka and FFmpeg workers model the asynchronous transcoding pipeline cleanly.",
    ],
    28: [
        "Object storage handles resumable uploads and processed video assets without overloading application servers.",
        "Kafka is ideal for upload-completed, transcode, copyright, publish, and search-index workflows.",
        "OpenSearch fits video discovery while PostgreSQL manages creator, metadata, and publication state.",
    ],
    29: [
        "CDN and object storage are mandatory for efficient short-video playback at scale.",
        "Python is well suited for ranking and feedback-loop experimentation.",
        "Kafka captures watch events and engagement signals, while Redis keeps per-user feed serving fast.",
    ],
    30: [
        "Java/Spring Boot fits complex social product rules: privacy, graph actions, reactions, and moderation.",
        "Kafka supports privacy-aware fanout, ranking, backfills, and counter workflows.",
        "Redis keeps viewer-specific feed pages and counters fast, while PostgreSQL protects canonical entities.",
    ],
    31: [
        "TypeScript/Node pairs naturally with a React editor and CRDT libraries for collaborative document state.",
        "WebSockets are the right transport for low-latency operations and presence updates.",
        "PostgreSQL stores operation logs and metadata, while Redis pub-sub keeps active collaborators synchronized.",
    ],
    32: [
        "Go works well for edge proxying because HTTP, streaming, and concurrency are central.",
        "Object storage or HTTP origins model realistic CDN origin behavior without needing a huge backend.",
        "Redis is useful for purge propagation and shared edge metadata in a multi-node simulation.",
    ],
    33: [
        "OpenSearch or Lucene is the right tool for inverted indexes, relevance scoring, and query serving.",
        "Kafka decouples crawling, parsing, indexing, and reindexing so freshness can be tuned independently.",
        "Java or Python lets you combine mature search tooling with crawler and ranking experiments.",
    ],
    34: [
        "PostGIS is the right fit for places, geocoding, and spatial indexes.",
        "A graph engine in Java or Go makes routing algorithms and memory/performance tradeoffs explicit.",
        "Redis caches hot routes, geocoding responses, and traffic-adjusted results where milliseconds matter.",
    ],
    35: [
        "Go or Java is appropriate because consensus, RPC, storage, and concurrency are the primary learning goals.",
        "Raft gives a concrete, understandable path to leader election and replicated logs.",
        "An LSM storage layer connects the project to real distributed databases without hiding durability mechanics.",
    ],
    36: [
        "Python/FastAPI fits event ingestion demos and analytics APIs while keeping transformation code approachable.",
        "Kafka is the durable event backbone for stream processing and replay.",
        "ClickHouse is purpose-built for high-cardinality analytical queries and dashboard workloads.",
    ],
    37: [
        "Java/Spring Boot works well for campaign management and low-latency serving APIs with mature operational patterns.",
        "Redis keeps targeting and budget state hot enough for ad-decision latency constraints.",
        "ClickHouse is suited for impression/click analytics, while Kafka absorbs tracking volume.",
    ],
    38: [
        "Python/FastAPI fits rules, model inference, and feature engineering in the same ecosystem.",
        "Kafka captures risk events for both realtime scoring and offline model improvement.",
        "Redis or a feature store keeps online fraud features low-latency, while PostgreSQL preserves audit and review cases.",
    ],
    39: [
        "Java or Rust fits deterministic, low-latency matching better than a framework-heavy web stack.",
        "An in-memory order book is required because matching cannot wait on external storage calls.",
        "Kafka is appropriate for market data fanout after execution, while PostgreSQL stores audit/read models off the hot path.",
    ],
    40: [
        "Go suits schedulers because worker leases, heartbeats, concurrency, and RPC are core concerns.",
        "PostgreSQL is the durable source for jobs, schedules, and attempts.",
        "Redis accelerates runnable queues and worker liveness while Kubernetes notes connect the design to real deployment.",
    ],
    41: [
        "Java/Spring Boot fits command handling, domain modeling, and projection services.",
        "PostgreSQL can act as a clear event store for learning optimistic concurrency and append-only writes.",
        "Kafka distributes events to projections and integrations, while Redis serves hot read models.",
    ],
    42: [
        "Java/Spring Boot fits B2B SaaS concerns such as RBAC, audit logs, quotas, and tenant lifecycle workflows.",
        "PostgreSQL supports multiple tenant isolation strategies and strong relational constraints.",
        "Redis and Kafka handle quotas, sessions, audit, billing, and integration events without slowing core requests.",
    ],
    43: [
        "Go or Java can handle ingest/session control while FFmpeg workers perform the CPU-heavy media work.",
        "HLS/CDN is best for massive viewer scale; WebRTC is included when low latency is the stronger requirement.",
        "Redis tracks live stream state and viewer counts, while Kafka carries health, chat, and analytics events.",
    ],
    44: [
        "Go or Java is appropriate because storage layout, gossip, replication, and quorum behavior are the project itself.",
        "LSM storage matches write-heavy NoSQL systems and makes compaction/repair tradeoffs concrete.",
        "Consistent hashing and gossip model the decentralized membership style used by Dynamo/Cassandra-like systems.",
    ],
    45: [
        "Go fits realtime session servers because it handles many connections and tick-loop concurrency predictably.",
        "WebSockets or UDP map directly to gameplay state sync depending on reliability and latency needs.",
        "Redis supports matchmaking and active match state, while Kafka records analytics after the gameplay hot path.",
    ],
    46: [
        "Python/FastAPI fits ML serving because model artifacts, preprocessing, and inference libraries are Python-native.",
        "Kubernetes is suited for model version deployments, autoscaling, and canary rollouts.",
        "Redis or a feature store keeps feature lookup fast, while Kafka captures prediction logs and drift signals.",
    ],
    47: [
        "Go or Java services make the architecture realistic while keeping the focus on regional routing and resilience.",
        "Kubernetes, Terraform, and service mesh concepts are essential because deployment topology is part of the system design.",
        "CockroachDB/PostgreSQL notes help compare global SQL, home-region writes, and async replication tradeoffs.",
    ],
    48: [
        "Go or Java is appropriate for implementing or simulating consensus, MVCC, and transaction coordination.",
        "Raft/Paxos concepts are the core of strong global consistency, so the stack should expose those mechanics.",
        "Jepsen-style tests are included because correctness under partitions matters more than happy-path throughput.",
    ],
    49: [
        "Rust or carefully tuned Java is suited for latency-sensitive matching where allocation, pauses, and predictability matter.",
        "An in-memory matching engine and append-only journal are the realistic architecture for deterministic replay.",
        "A binary protocol and separate market-data stream keep the hot path lean and measurable.",
    ],
    50: [
        "Go, Java, or Rust are realistic service languages for a planet-scale architecture, but the main artifact is the system design.",
        "Kubernetes, Terraform, service mesh, and observability tooling are required because operations and rollout safety dominate this scale.",
        "Multi-region data stores and cell-based design force the key senior-level tradeoff: isolate blast radius while preserving user experience.",
    ],
}


def project_plan(project):
    phase = PHASES[project["phase"]]
    mvp = [
        "Implement the narrow happy path with local Docker Compose dependencies.",
        "Expose the primary APIs and a minimal CLI or UI only where it proves the system behavior.",
        "Add metrics, structured logs, and a repeatable seed/load script.",
    ]
    production = [
        "Add multi-node or multi-worker behavior, backpressure, retries, and idempotency.",
        "Document capacity estimates, SLOs, data ownership, and operational runbooks.",
        "Harden security, authorization, quotas, and failure recovery where relevant.",
    ]
    stretch = [
        "Add a realistic dashboard or simulator for demos.",
        "Run load, soak, and failure-injection tests and record results in the project README.",
        "Write an interview narrative that explains the design from MVP to scale.",
    ]
    return f"""# {project['id']:02d}. Design a {project['title']}

## Project Brief
{project['problem']}

Primary users: {project['users']}

Phase: {phase['name']} ({phase['range']})

Recommended stack: {project['stack']}

## Why This Stack
{wrap_list(TECH_RATIONALES[project['id']])}

## Learning Objectives
{wrap_list(project['objectives'])}

## Requirements
Functional requirements:
- Provide the core user workflow end to end.
- Include admin or operator controls needed to demonstrate the system.
- Persist enough state to recover from restarts.
- Publish domain events for asynchronous work and observability.

Non-functional requirements:
- Define target p50, p95, and p99 latency for the read and write paths.
- Define availability and durability expectations for the MVP and production design.
- Include rate limits, quotas, or backpressure for overload protection.
- Make data retention, privacy, and audit behavior explicit.

## Scope
MVP:
{wrap_list(mvp)}

Production version:
{wrap_list(production)}

Stretch goals:
{wrap_list(stretch)}

## Architecture
Diagram to draw:
- Client or demo UI calls the public API layer.
- API layer validates requests, checks auth or tenant context, and writes to the primary store.
- Async events flow through the message bus to workers, projectors, or processors.
- Cache and read models serve hot read paths.
- Observability pipeline collects logs, metrics, traces, and alerts.

Core components:
- API service for synchronous user and admin workflows.
- Storage layer: {project['storage']}
- Worker or stream processor for asynchronous processing and retries.
- Cache/read path: {project['caching']}
- Monitoring stack with Prometheus metrics, Grafana dashboards, structured logs, and OpenTelemetry traces.

## APIs, Events, and Data Model
Core APIs:
{wrap_list(project['apis'])}

Core data model:
{wrap_list(project['models'])}

Events:
{wrap_list(project['events'])}

## Design Decisions
Storage: {project['storage']}

Consistency: {project['consistency']}

Caching: {project['caching']}

Scaling strategy:
- Partition by the natural high-cardinality key for this project.
- Separate control-plane writes from high-throughput data-plane reads where applicable.
- Add horizontal workers for async processing before scaling the synchronous API tier.
- Use load tests to identify the first bottleneck before introducing more infrastructure.

Failure modes to design for:
{wrap_list(project['failures'])}

## Build Milestones
{numbered_milestones(project['milestones'])}

## Testing and Verification
Unit tests:
- Validate domain rules, state transitions, serialization, and edge cases.

Integration tests:
- Run API plus storage plus cache/message bus in Docker Compose.
- Verify idempotency, retries, and recovery after process restart.

Load tests:
- Establish baseline throughput and latency.
- Include one hotspot scenario and one steady-state scenario.

Failure tests:
- Kill a worker during processing.
- Restart the cache or database dependency.
- Inject duplicate events or requests.
- Verify alerts fire for error rate, latency, saturation, and lag.

## Observability
Logs:
- Emit structured logs with request ID, actor, resource ID, and decision reason.

Metrics:
- Request rate, error rate, latency, saturation, queue lag, cache hit rate, and storage latency.

Traces:
- Trace the synchronous API path and at least one async workflow end to end.

Dashboards and alerts:
- Golden signals dashboard.
- Dependency health dashboard.
- Alert on SLO burn, queue lag, failed retries, and elevated p99 latency.

## Interview Talking Points
{wrap_list(project['interview'])}
- Start from the MVP, then explain which bottleneck forces each production design change.
- Call out what is strongly consistent, what is eventually consistent, and why.
- Estimate rough scale: users, requests per second, storage growth, and hot-key behavior.

## Done Criteria
- A new engineer can run the MVP locally from the README.
- APIs and data model are documented with examples.
- Load-test numbers and known bottlenecks are recorded.
- At least one failure drill is documented with expected and actual behavior.
- The interview narrative fits in 10-15 minutes with tradeoffs clearly stated.
"""


def readme():
    phase_rows = "\n".join(
        f"| Phase {pid} | {phase['name']} | {phase['range']} | {phase['outcome']} |"
        for pid, phase in PHASES.items()
    )
    project_rows = "\n".join(
        f"| {p['id']:02d} | [{p['title']}](projects/{p['id']:02d}-{slugify(p['title'])}/plan.md) | {PHASES[p['phase']]['name']} | {p['stack']} |"
        for p in sorted(PROJECTS, key=lambda x: x["id"])
    )
    return f"""# System Design Portfolio Roadmap

This repository turns the 50 ideas in `system-design-ideas` into full-fledged project briefs for learning and portfolio building. The target level is mid-senior backend/system-design interviews, with the final phase stretching into senior/staff-level architecture.

The goal is not to memorize diagrams. The goal is to build enough of each system that you can explain requirements, APIs, data models, tradeoffs, bottlenecks, failure modes, and operational behavior from experience.

## How to Use This Roadmap
- Read [docs/roadmap.md](docs/roadmap.md) to follow the phases in order.
- Use [docs/project-template.md](docs/project-template.md) when expanding a project into implementation docs.
- Pick one project at a time and create an implementation repo or subfolder from its `plan.md`.
- For every project, keep a short build journal: what worked, what failed, benchmarks, and the interview story.

## Phases
| Phase | Name | Projects | Outcome |
|---|---|---:|---|
{phase_rows}

## Technology Strategy
Use multiple technologies intentionally:

- Go for networking, infrastructure, storage engines, caches, schedulers, and low-level distributed systems.
- Java/Spring Boot for enterprise backend systems, workflows, product APIs, Kafka-heavy services, and transactional domains.
- Python/FastAPI for analytics, fraud, recommendation, and ML-serving systems.
- TypeScript/React for dashboards, admin panels, simulators, and visual demos when useful.
- Docker Compose for local demos; Kubernetes and Terraform notes for production variants.
- Prometheus, Grafana, OpenTelemetry, and structured logs in every serious build.

## Project Index
| # | Project | Phase | Recommended Stack |
|---:|---|---|---|
{project_rows}

## Portfolio Standard
For a project to count as portfolio-ready, it should have:

- Clear problem statement and scale assumptions.
- Runnable MVP with local dependencies.
- Documented APIs, data model, events, and architecture diagram.
- A clear explanation of why the chosen technologies fit the workload.
- Tests covering domain logic, integrations, and at least one failure case.
- Load test or simulation showing the first bottleneck.
- Observability dashboard or documented metrics.
- A 10-15 minute interview narrative with tradeoffs.
"""


def roadmap():
    sections = []
    for pid, phase in PHASES.items():
        projects = [p for p in sorted(PROJECTS, key=lambda x: x["id"]) if p["phase"] == pid]
        links = "\n".join(
            f"- {p['id']:02d}. [{p['title']}](../projects/{p['id']:02d}-{slugify(p['title'])}/plan.md): {', '.join(p['objectives'][:2])}."
            for p in projects
        )
        sections.append(
            f"""## Phase {pid}: {phase['name']}
Range: {phase['range']}

Outcome: {phase['outcome']}

Projects:
{links}
"""
        )
    return """# Roadmap

Follow the phases in order if you want the cleanest learning curve. Each phase reuses concepts from the previous one, so the later projects become easier to explain when the foundations are already built.

Suggested cadence:
- Small projects: 3-5 focused days each.
- Product systems: 1-2 weeks each.
- Infrastructure systems: 2-3 weeks each.
- Senior/staff systems: design first, then build one narrow but convincing MVP slice.

""" + "\n".join(sections)


def template():
    return """# Project Brief Template

Use this when expanding a plan into implementation docs.

## Problem
- What user or business problem does this system solve?
- What scale and workload are assumed?
- What is explicitly out of scope?

## Requirements
Functional:
- Core workflows.
- Admin/operator workflows.
- Events and async workflows.

Non-functional:
- Latency, availability, durability, and consistency targets.
- Security, privacy, compliance, and abuse constraints.
- Cost and operational constraints.

## Architecture
- Public APIs and clients.
- Synchronous services.
- Storage systems.
- Caches and read models.
- Message bus and workers.
- Observability pipeline.

## Why This Stack
- Why the primary language fits the latency, concurrency, ecosystem, or hiring goal.
- Why each data store fits the access pattern and consistency requirement.
- Why each async, cache, search, analytics, or infrastructure tool is worth its operational cost.

## APIs, Events, and Data Model
- REST/gRPC/WebSocket endpoints.
- Event names and payload ownership.
- Tables, indexes, keys, partitions, and retention.
- Idempotency and versioning strategy.

## Design Decisions
- Storage choice and access patterns.
- Caching strategy and invalidation.
- Consistency model.
- Scaling and partitioning strategy.
- Backpressure and overload behavior.
- Failure handling and recovery.

## Build Plan
1. MVP happy path.
2. Durable storage and recovery.
3. Async workflow.
4. Scaling path.
5. Observability and operations.
6. Load and failure testing.

## Testing
- Unit tests for domain rules.
- Integration tests with local dependencies.
- Contract tests for APIs/events.
- Load tests for normal and hotspot traffic.
- Failure-injection tests for dependencies and workers.

## Interview Story
- Start with requirements and assumptions.
- Draw the MVP.
- Identify bottlenecks.
- Add scale changes one by one.
- Explain consistency and failure tradeoffs.
- Close with monitoring, rollout, and future work.
"""


def main():
    (ROOT / "docs").mkdir(exist_ok=True)
    (ROOT / "projects").mkdir(exist_ok=True)
    (ROOT / "README.md").write_text(readme(), encoding="utf-8")
    (ROOT / "docs" / "roadmap.md").write_text(roadmap(), encoding="utf-8")
    (ROOT / "docs" / "project-template.md").write_text(template(), encoding="utf-8")

    for project in sorted(PROJECTS, key=lambda x: x["id"]):
        directory = project_dir(project)
        directory.mkdir(parents=True, exist_ok=True)
        (directory / "plan.md").write_text(project_plan(project), encoding="utf-8")

    print(f"Generated {len(PROJECTS)} project plans.")


if __name__ == "__main__":
    main()
