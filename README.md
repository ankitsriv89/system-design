# System Design Portfolio Roadmap

This repository turns the 50 ideas in `system-design-ideas` into full-fledged, working implementations for learning and showcasing distributed systems. The projects go from foundational primitives to large-scale distributed architecture.

The goal is not to memorize diagrams. The goal is to build enough of each system that you can explain requirements, APIs, data models, tradeoffs, bottlenecks, failure modes, and operational behavior from experience.

## How to Use This Roadmap
- Read [docs/roadmap.md](docs/roadmap.md) to follow the phases in order.
- Use [docs/project-template.md](docs/project-template.md) when expanding a project into implementation docs.
- Pick one project at a time and create an implementation repo or subfolder from its `plan.md`.
- For every project, keep a short build journal: what worked, what failed, and benchmarks.
- See [docs/deployment.md](docs/deployment.md) for the plan to host all 50 projects on a single Oracle Cloud free-tier server under one domain, and how to use AWS Spot / GitHub Codespaces for cloud-based testing.

## Phases
| Phase | Name | Projects | Outcome |
|---|---|---:|---|
| Phase 1 | Core Building Blocks | 01-13 | Build the primitives that appear inside larger distributed systems: traffic control, storage, caching, queues, crawling, routing, and gateway behavior. |
| Phase 2 | Real-Time and Product Systems | 14-24 | Compose the primitives into user-facing systems with realtime delivery, fanout, ranking, transactional flows, and product-facing APIs. |
| Phase 3 | Distributed Infrastructure | 25-35 | Design infrastructure-scale systems around partitioning, replication, media pipelines, search, routing, collaborative state, and distributed storage. |
| Phase 4 | Advanced Data and Platform Systems | 36-46 | Work through streaming analytics, marketplaces, fraud, exchanges, schedulers, CQRS, multi-tenancy, gaming, and ML serving. |
| Phase 5 | Global Scale Systems | 47-50 | Practice global architecture decisions: multi-region latency, consensus, capital-market latency, and planet-scale availability. |

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
| 01 | [Rate Limiter](projects/01-rate-limiter/plan.md) | Core Building Blocks | Go, Redis, PostgreSQL, Docker, REST/gRPC, Prometheus |
| 02 | [URL Shortener](projects/02-url-shortener/plan.md) | Core Building Blocks | Go, PostgreSQL, Redis, Docker, REST, React admin dashboard |
| 03 | [Pastebin](projects/03-pastebin/plan.md) | Core Building Blocks | Go, PostgreSQL, S3-compatible object storage, Redis, REST |
| 04 | [Unique ID Generator](projects/04-unique-id-generator/plan.md) | Core Building Blocks | Go, gRPC, PostgreSQL, Docker, Prometheus |
| 05 | [Consistent Hashing](projects/05-consistent-hashing/plan.md) | Core Building Blocks | Go, CLI visualizer, Docker, optional React ring explorer |
| 06 | [Load Balancer](projects/06-load-balancer/plan.md) | Core Building Blocks | Go, HTTP reverse proxy, Docker Compose, Prometheus, React traffic console |
| 07 | [API Gateway](projects/07-api-gateway/plan.md) | Core Building Blocks | Go, Redis, PostgreSQL, OpenTelemetry, REST/gRPC, Docker |
| 08 | [Basic Key-Value Store](projects/08-basic-key-value-store/plan.md) | Core Building Blocks | Go, gRPC, write-ahead log, SSTable-style files, Docker |
| 09 | [Caching System](projects/09-caching-system/plan.md) | Core Building Blocks | Go, Redis-compatible protocol subset, Docker, Prometheus |
| 10 | [Notification System](projects/10-notification-system/plan.md) | Core Building Blocks | Java/Spring Boot, Kafka, PostgreSQL, Redis, provider mocks, Docker |
| 11 | [Typeahead Autocomplete System](projects/11-typeahead-autocomplete-system/plan.md) | Core Building Blocks | Go, Redis, PostgreSQL, OpenSearch optional, React demo |
| 12 | [Web Crawler](projects/12-web-crawler/plan.md) | Core Building Blocks | Go, Kafka or NATS, PostgreSQL, Redis, object storage, Docker |
| 13 | [Message Queue](projects/13-message-queue/plan.md) | Core Building Blocks | Go, gRPC/HTTP, PostgreSQL or log segments, Docker, Prometheus |
| 14 | [1:1 Chat System](projects/14-one-to-one-chat-system/plan.md) | Real-Time and Product Systems | Java/Spring Boot, WebSockets, Kafka, PostgreSQL, Redis |
| 15 | [Group Chat System](projects/15-group-chat-system/plan.md) | Real-Time and Product Systems | Java/Spring Boot, WebSockets, Kafka, PostgreSQL, Redis |
| 16 | [News Feed System](projects/16-news-feed-system/plan.md) | Real-Time and Product Systems | Java/Spring Boot, Kafka, PostgreSQL, Redis, OpenSearch optional |
| 17 | [Proximity Service](projects/17-proximity-service/plan.md) | Real-Time and Product Systems | Java/Spring Boot, Redis GEO, PostgreSQL/PostGIS, Kafka |
| 18 | [Instagram Photo/Video Sharing and Feed](projects/18-instagram-photo-video-sharing-and-feed/plan.md) | Real-Time and Product Systems | Java/Spring Boot, Kafka, PostgreSQL, Redis, S3-compatible storage, CDN |
| 19 | [Twitter/X Timeline and Posts](projects/19-twitter-x-timeline-and-posts/plan.md) | Real-Time and Product Systems | Java/Spring Boot, Kafka, PostgreSQL, Redis, OpenSearch |
| 20 | [WhatsApp Real-Time Messaging](projects/20-whatsapp-real-time-messaging/plan.md) | Real-Time and Product Systems | Java/Spring Boot, Netty/WebSockets, Kafka, PostgreSQL/Cassandra, Redis |
| 21 | [Dropbox File Storage and Sync](projects/21-dropbox-file-storage-and-sync/plan.md) | Real-Time and Product Systems | Java/Spring Boot, PostgreSQL, S3-compatible storage, Kafka, Redis, desktop sync mock |
| 22 | [Ticket Booking System](projects/22-ticket-booking-system/plan.md) | Real-Time and Product Systems | Java/Spring Boot, PostgreSQL, Redis, Kafka, payment mock |
| 23 | [E-commerce Platform](projects/23-e-commerce-platform/plan.md) | Real-Time and Product Systems | Java/Spring Boot, Kafka, PostgreSQL, Redis, OpenSearch, payment mock |
| 24 | [Recommendation System](projects/24-recommendation-system/plan.md) | Real-Time and Product Systems | Python/FastAPI, Kafka, PostgreSQL, Redis, feature store, vector DB optional |
| 25 | [Distributed Cache](projects/25-distributed-cache/plan.md) | Distributed Infrastructure | Go, Redis protocol subset, consistent hashing, gossip, Docker |
| 26 | [Uber Ride-Sharing and Matching](projects/26-uber-ride-sharing-and-matching/plan.md) | Distributed Infrastructure | Java/Spring Boot, Kafka, Redis GEO, PostgreSQL/PostGIS, WebSockets |
| 27 | [Netflix Video Streaming Platform](projects/27-netflix-video-streaming-platform/plan.md) | Distributed Infrastructure | Java/Spring Boot, S3-compatible storage, CDN, Kafka, PostgreSQL, FFmpeg workers |
| 28 | [YouTube Video Upload and Streaming](projects/28-youtube-video-upload-and-streaming/plan.md) | Distributed Infrastructure | Java/Spring Boot, Kafka, S3-compatible storage, CDN, OpenSearch, PostgreSQL |
| 29 | [TikTok Short-Video Platform](projects/29-tiktok-short-video-platform/plan.md) | Distributed Infrastructure | Java/Spring Boot, Kafka, Redis, S3-compatible storage, CDN, Python ranking service |
| 30 | [Facebook-Like Social Network News Feed](projects/30-facebook-like-social-network-news-feed/plan.md) | Distributed Infrastructure | Java/Spring Boot, Kafka, PostgreSQL, Redis, graph store optional |
| 31 | [Google Docs Real-Time Collaborative Editing](projects/31-google-docs-real-time-collaborative-editing/plan.md) | Distributed Infrastructure | TypeScript/Node or Java, WebSockets, PostgreSQL, Redis, CRDT library, React editor |
| 32 | [Content Delivery Network CDN](projects/32-content-delivery-network-cdn/plan.md) | Distributed Infrastructure | Go, reverse proxy, object storage origin, Redis, Prometheus, Terraform notes |
| 33 | [Search Engine](projects/33-search-engine/plan.md) | Distributed Infrastructure | Java/Python, crawler, Kafka, OpenSearch/Lucene, PostgreSQL |
| 34 | [Google Maps Routing and Location Services](projects/34-google-maps-routing-and-location-services/plan.md) | Distributed Infrastructure | Java/Go, PostgreSQL/PostGIS, graph engine, Redis, Kafka |
| 35 | [Distributed Database](projects/35-distributed-database/plan.md) | Distributed Infrastructure | Go or Java, Raft library, gRPC, LSM storage, Docker Compose |
| 36 | [Real-Time Analytics System](projects/36-real-time-analytics-system/plan.md) | Advanced Data and Platform Systems | Python/FastAPI, Kafka, Flink-style workers, ClickHouse, Redis, Grafana |
| 37 | [Ad Serving and Tracking System](projects/37-ad-serving-and-tracking-system/plan.md) | Advanced Data and Platform Systems | Java/Spring Boot, Kafka, Redis, PostgreSQL, ClickHouse |
| 38 | [Fraud Detection System](projects/38-fraud-detection-system/plan.md) | Advanced Data and Platform Systems | Python/FastAPI, Kafka, Redis, feature store, PostgreSQL, model service |
| 39 | [Stock Trading Exchange System](projects/39-stock-trading-exchange-system/plan.md) | Advanced Data and Platform Systems | Java or Rust, Kafka for market data, PostgreSQL audit, in-memory matching engine |
| 40 | [Distributed Job Scheduler](projects/40-distributed-job-scheduler/plan.md) | Advanced Data and Platform Systems | Go, PostgreSQL, Redis, gRPC, Docker, Kubernetes notes |
| 41 | [Event Sourcing and CQRS Architecture](projects/41-event-sourcing-and-cqrs-architecture/plan.md) | Advanced Data and Platform Systems | Java/Spring Boot, Kafka, PostgreSQL event store, Redis, React admin |
| 42 | [Multi-Tenant SaaS Platform](projects/42-multi-tenant-saas-platform/plan.md) | Advanced Data and Platform Systems | Java/Spring Boot, PostgreSQL, Redis, Kafka, React admin, Kubernetes notes |
| 43 | [Live Video Streaming at Scale](projects/43-live-video-streaming-at-scale/plan.md) | Advanced Data and Platform Systems | Go/Java, WebRTC or HLS, Kafka, Redis, CDN, FFmpeg workers |
| 44 | [Highly Scalable NoSQL Database](projects/44-highly-scalable-nosql-database/plan.md) | Advanced Data and Platform Systems | Go/Java, LSM storage, gossip, consistent hashing, Raft optional |
| 45 | [Real-Time Multiplayer Game Backend](projects/45-real-time-multiplayer-game-backend/plan.md) | Advanced Data and Platform Systems | Go, WebSockets/UDP, Redis, PostgreSQL, Kafka, Kubernetes notes |
| 46 | [Machine Learning Model Serving Infrastructure](projects/46-ml-model-serving-infrastructure/plan.md) | Advanced Data and Platform Systems | Python/FastAPI, model registry, Redis, Kafka, Kubernetes, Prometheus |
| 47 | [Geo-Distributed Low-Latency System](projects/47-geo-distributed-low-latency-system/plan.md) | Global Scale Systems | Go/Java, Kubernetes, Terraform, Redis, PostgreSQL/CockroachDB notes, service mesh |
| 48 | [Strongly Consistent Global Database](projects/48-strongly-consistent-global-database/plan.md) | Global Scale Systems | Go/Java, Raft/Paxos concepts, TrueTime-style notes, Kubernetes, Jepsen-style tests |
| 49 | [High-Frequency Trading Platform](projects/49-high-frequency-trading-platform/plan.md) | Global Scale Systems | Rust or Java, low-latency networking, in-memory matching, append-only journal, market data feed |
| 50 | [Planet-Scale Distributed System](projects/50-planet-scale-distributed-system/plan.md) | Global Scale Systems | Go/Java/Rust design, Kubernetes, Terraform, multi-region data stores, service mesh, observability stack |

## Project Standard
For a project to count as complete, it should have:

- Clear problem statement and scale assumptions.
- Runnable MVP with local dependencies.
- Documented APIs, data model, events, and architecture diagram.
- A clear explanation of why the chosen technologies fit the workload.
- Tests covering domain logic, integrations, and at least one failure case.
- Load test or simulation showing the first bottleneck.
- Observability dashboard or documented metrics.
- A design narrative covering tradeoffs and scaling decisions.
