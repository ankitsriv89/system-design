# Roadmap

Follow the phases in order if you want the cleanest learning curve. Each phase reuses concepts from the previous one, so the later projects become easier to explain when the foundations are already built.

Suggested cadence:
- Small projects: 3-5 focused days each.
- Product systems: 1-2 weeks each.
- Infrastructure systems: 2-3 weeks each.
- Senior/staff systems: design first, then build one narrow but convincing MVP slice.

## Phase 1: Core Building Blocks
Range: 01-13

Outcome: Build the primitives that appear inside larger distributed systems: traffic control, storage, caching, queues, crawling, routing, and gateway behavior.

Projects:
- 01. [Rate Limiter](../projects/01-rate-limiter/plan.md): token bucket and sliding-window algorithms, hot-key mitigation.
- 02. [URL Shortener](../projects/02-url-shortener/plan.md): base62 encoding, read-heavy API design.
- 03. [Pastebin](../projects/03-pastebin/plan.md): blob storage boundaries, metadata indexing.
- 04. [Unique ID Generator](../projects/04-unique-id-generator/plan.md): Snowflake-style IDs, worker coordination.
- 05. [Consistent Hashing](../projects/05-consistent-hashing/plan.md): hash rings, virtual nodes.
- 06. [Load Balancer](../projects/06-load-balancer/plan.md): round-robin and least-connections, health checks.
- 07. [API Gateway](../projects/07-api-gateway/plan.md): auth middleware, routing rules.
- 08. [Basic Key-Value Store](../projects/08-basic-key-value-store/plan.md): WAL durability, memtable and segments.
- 09. [Caching System](../projects/09-caching-system/plan.md): LRU and LFU eviction, TTL expiration.
- 10. [Notification System](../projects/10-notification-system/plan.md): fanout pipeline, template rendering.
- 11. [Typeahead Autocomplete System](../projects/11-typeahead-autocomplete-system/plan.md): trie/prefix index, ranking features.
- 12. [Web Crawler](../projects/12-web-crawler/plan.md): frontier scheduling, robots.txt politeness.
- 13. [Message Queue](../projects/13-message-queue/plan.md): append-only logs, consumer offsets.

## Phase 2: Real-Time and Product Systems
Range: 14-24

Outcome: Compose the primitives into user-facing systems with realtime delivery, fanout, ranking, transactional flows, and product-facing APIs.

Projects:
- 14. [1:1 Chat System](../projects/14-one-to-one-chat-system/plan.md): WebSocket session management, message ordering.
- 15. [Group Chat System](../projects/15-group-chat-system/plan.md): fanout strategies, membership authorization.
- 16. [News Feed System](../projects/16-news-feed-system/plan.md): fanout-on-write/read, ranking.
- 17. [Proximity Service](../projects/17-proximity-service/plan.md): geohash, spatial indexing.
- 18. [Instagram Photo/Video Sharing and Feed](../projects/18-instagram-photo-video-sharing-and-feed/plan.md): media upload pipeline, fanout feed.
- 19. [Twitter/X Timeline and Posts](../projects/19-twitter-x-timeline-and-posts/plan.md): timeline fanout, celebrity accounts.
- 20. [WhatsApp Real-Time Messaging](../projects/20-whatsapp-real-time-messaging/plan.md): mobile connection lifecycle, multi-device sync.
- 21. [Dropbox File Storage and Sync](../projects/21-dropbox-file-storage-and-sync/plan.md): chunked uploads, delta sync.
- 22. [Ticket Booking System](../projects/22-ticket-booking-system/plan.md): inventory locking, payment saga.
- 23. [E-commerce Platform](../projects/23-e-commerce-platform/plan.md): catalog search, cart state.
- 24. [Recommendation System](../projects/24-recommendation-system/plan.md): candidate generation, feature pipelines.

## Phase 3: Distributed Infrastructure
Range: 25-35

Outcome: Design infrastructure-scale systems around partitioning, replication, media pipelines, search, routing, collaborative state, and distributed storage.

Projects:
- 25. [Distributed Cache](../projects/25-distributed-cache/plan.md): consistent hashing, replication.
- 26. [Uber Ride-Sharing and Matching](../projects/26-uber-ride-sharing-and-matching/plan.md): geo indexing, matching algorithms.
- 27. [Netflix Video Streaming Platform](../projects/27-netflix-video-streaming-platform/plan.md): transcoding pipeline, adaptive bitrate.
- 28. [YouTube Video Upload and Streaming](../projects/28-youtube-video-upload-and-streaming/plan.md): resumable uploads, processing workflow.
- 29. [TikTok Short-Video Platform](../projects/29-tiktok-short-video-platform/plan.md): low-latency feed serving, ranking feedback loop.
- 30. [Facebook-Like Social Network News Feed](../projects/30-facebook-like-social-network-news-feed/plan.md): privacy-aware feed generation, graph relationships.
- 31. [Google Docs Real-Time Collaborative Editing](../projects/31-google-docs-real-time-collaborative-editing/plan.md): CRDT/OT concepts, presence.
- 32. [Content Delivery Network CDN](../projects/32-content-delivery-network-cdn/plan.md): edge caching, cache keys.
- 33. [Search Engine](../projects/33-search-engine/plan.md): inverted index, ranking.
- 34. [Google Maps Routing and Location Services](../projects/34-google-maps-routing-and-location-services/plan.md): road graph modeling, shortest path.
- 35. [Distributed Database](../projects/35-distributed-database/plan.md): Raft consensus, partitioning.

## Phase 4: Advanced Data and Platform Systems
Range: 36-46

Outcome: Work through streaming analytics, marketplaces, fraud, exchanges, schedulers, CQRS, multi-tenancy, gaming, and ML serving.

Projects:
- 36. [Real-Time Analytics System](../projects/36-real-time-analytics-system/plan.md): stream processing, windowed aggregations.
- 37. [Ad Serving and Tracking System](../projects/37-ad-serving-and-tracking-system/plan.md): auction/ranking, budget pacing.
- 38. [Fraud Detection System](../projects/38-fraud-detection-system/plan.md): streaming features, rules engine.
- 39. [Stock Trading Exchange System](../projects/39-stock-trading-exchange-system/plan.md): order books, price-time priority.
- 40. [Distributed Job Scheduler](../projects/40-distributed-job-scheduler/plan.md): leases, cron scheduling.
- 41. [Event Sourcing and CQRS Architecture](../projects/41-event-sourcing-and-cqrs-architecture/plan.md): event modeling, projections.
- 42. [Multi-Tenant SaaS Platform](../projects/42-multi-tenant-saas-platform/plan.md): tenant isolation, RBAC.
- 43. [Live Video Streaming at Scale](../projects/43-live-video-streaming-at-scale/plan.md): low-latency ingest, transcoding ladders.
- 44. [Highly Scalable NoSQL Database](../projects/44-highly-scalable-nosql-database/plan.md): quorum consistency, gossip membership.
- 45. [Real-Time Multiplayer Game Backend](../projects/45-real-time-multiplayer-game-backend/plan.md): authoritative server, tick loop.
- 46. [Machine Learning Model Serving Infrastructure](../projects/46-ml-model-serving-infrastructure/plan.md): model registry, online inference.

## Phase 5: Senior/Staff-Level Scale
Range: 47-50

Outcome: Practice global architecture decisions: multi-region latency, consensus, capital-market latency, and planet-scale availability.

Projects:
- 47. [Geo-Distributed Low-Latency System](../projects/47-geo-distributed-low-latency-system/plan.md): regional routing, data locality.
- 48. [Strongly Consistent Global Database](../projects/48-strongly-consistent-global-database/plan.md): consensus across regions, transaction timestamps.
- 49. [High-Frequency Trading Platform](../projects/49-high-frequency-trading-platform/plan.md): low-latency path, deterministic matching.
- 50. [Planet-Scale Distributed System](../projects/50-planet-scale-distributed-system/plan.md): global control/data planes, multi-region HA.
