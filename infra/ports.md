# Port Registry — System Design Projects

Each project gets a fixed host port. This registry prevents conflicts across all 50 projects.

| Port | Project | Caddy Path | Status | Notes |
|------|---------|------------|--------|-------|
| **Shared infra** | | | | |
| 5432 | postgres (shared) | — | running | localhost only, not exposed publicly |
| **Built projects** | | | | |
| 8081 | [01-rate-limiter](../projects/01-rate-limiter/) | `/p01/` | ✅ built | |
| 8085 | [02-url-shortener](../projects/02-url-shortener/) | `/p02/` | ✅ built | |
| 8082 | [03-pastebin](../projects/03-pastebin/) | `/p03/` | ✅ built | |
| 8083 | [04-unique-id-generator](../projects/04-unique-id-generator/) | `/p04/` | ✅ built | |
| 8084 | [05-consistent-hashing](../projects/05-consistent-hashing/) | `/p05/` | ✅ built | |
| 8086 | [06-load-balancer](../projects/06-load-balancer/) | `/p06/` | ✅ built | proxy port |
| 8087 | [06-load-balancer](../projects/06-load-balancer/) | — | ✅ built | admin port (loopback only) |
| 8088 | [07-api-gateway](../projects/07-api-gateway/) | `/p07/` | ✅ built | |
| 8089 | [08-basic-key-value-store](../projects/08-basic-key-value-store/) | `/p08/` | ✅ built | |
| **Reserved — not yet built** | | | | |
| 8090 | [09-caching-system](../projects/09-caching-system/) | `/p09/` | ✅ built | |
| 8091 | [10-notification-system](../projects/10-notification-system/) | `/p10/` | ✅ built | |
| 8092 | [11-typeahead-autocomplete-system](../projects/11-typeahead-autocomplete-system/) | `/p11/` | ✅ built | |
| 8093 | [12-web-crawler](../projects/12-web-crawler/) | `/p12/` | ✅ built | |
| 8094 | [13-message-queue](../projects/13-message-queue/) | `/p13/` | ✅ built | |
| 8095 | [14-one-to-one-chat-system](../projects/14-one-to-one-chat-system/) | `/p14/` | ✅ built | WebSocket |
| 8096 | [15-group-chat-system](../projects/15-group-chat-system/) | `/p15/` | 🔧 in progress | WebSocket |
| 8097 | [16-news-feed-system](../projects/16-news-feed-system/) | `/p16/` | ✅ built | hybrid fanout |
| 8098 | [17-proximity-service](../projects/17-proximity-service/) | `/p17/` | ✅ built | Redis GEO + PostGIS |
| 8099 | [18-instagram-photo-video-sharing-and-feed](../projects/18-instagram-photo-video-sharing-and-feed/) | `/p18/` | ✅ built | media: MinIO origin + Cloudflare CDN |
| 8100 | [19-twitter-x-timeline-and-posts](../projects/19-twitter-x-timeline-and-posts/) | `/p19/` | 🔧 in progress | hybrid fanout + OpenSearch search/trends |
| 8101 | [20-whatsapp-real-time-messaging](../projects/20-whatsapp-real-time-messaging/) | `/p20/` | ✅ built | WebSocket · E2EE · Kafka fan-out |
| 8102 | 21-dropbox-file-storage-and-sync | `/p21/` | 🔧 in progress | |
| 8103 | [22-ticket-booking-system](../projects/22-ticket-booking-system/) | `/p22/` | 🔧 in progress | inventory lock · hold TTL · payment saga |
| 8104 | 23-e-commerce-platform | `/p23/` | 🔲 planned | |
| 8105 | 24-recommendation-system | `/p24/` | 🔲 planned | |
| 8106 | 25-distributed-cache | `/p25/` | 🔲 planned | |
| 8107 | 26-uber-ride-sharing-and-matching | `/p26/` | 🔲 planned | WebSocket |
| 8108 | 27-netflix-video-streaming-platform | `/p27/` | 🔲 planned | |
| 8109 | 28-youtube-video-upload-and-streaming | `/p28/` | 🔲 planned | |
| 8110 | 29-tiktok-short-video-platform | `/p29/` | 🔲 planned | |
| 8111 | 30-facebook-like-social-network-news-feed | `/p30/` | 🔲 planned | |
| 8112 | 31-google-docs-real-time-collaborative-editing | `/p31/` | 🔲 planned | WebSocket |
| 8113 | 32-content-delivery-network-cdn | `/p32/` | 🔲 planned | |
| 8114 | 33-search-engine | `/p33/` | 🔲 planned | |
| 8115 | 34-google-maps-routing-and-location-services | `/p34/` | 🔲 planned | |
| 8116 | 35-distributed-database | `/p35/` | 🔲 planned | |
| 8117 | 36-real-time-analytics-system | `/p36/` | 🔲 planned | |
| 8118 | 37-ad-serving-and-tracking-system | `/p37/` | 🔲 planned | |
| 8119 | 38-fraud-detection-system | `/p38/` | 🔲 planned | |
| 8120 | 39-stock-trading-exchange-system | `/p39/` | 🔲 planned | WebSocket |
| 8121 | 40-distributed-job-scheduler | `/p40/` | 🔲 planned | |
| 8122 | 41-event-sourcing-and-cqrs-architecture | `/p41/` | 🔲 planned | |
| 8123 | 42-multi-tenant-saas-platform | `/p42/` | 🔲 planned | |
| 8124 | 43-live-video-streaming-at-scale | `/p43/` | 🔲 planned | WebSocket |
| 8125 | 44-highly-scalable-nosql-database | `/p44/` | 🔲 planned | |
| 8126 | 45-real-time-multiplayer-game-backend | `/p45/` | 🔲 planned | WebSocket |
| 8127 | 46-ml-model-serving-infrastructure | `/p46/` | 🔲 planned | |
| 8128 | 47-geo-distributed-low-latency-system | `/p47/` | 🔲 planned | |
| 8129 | 48-strongly-consistent-global-database | `/p48/` | 🔲 planned | |
| 8130 | 49-high-frequency-trading-platform | `/p49/` | 🔲 planned | WebSocket |
| 8131 | 50-planet-scale-distributed-system | `/p50/` | 🔲 planned | |
| **Monitoring** | | | | |
| 9090 | Prometheus (shared infra) | — | running | |
| 3000 | Grafana (shared infra) | — | running | |

## Convention

- Ports are assigned sequentially as projects are built starting from 8081 (project 01).
- For projects 09 onwards: next available port above 8089, assigned at build time.
- Multi-port projects (e.g. 06 load-balancer with proxy + admin) each get their own port.
- Prometheus (shared infra): 9090. Grafana (shared infra): 3000.
- Postgres is shared on port 5432 (host), exposed on `localhost` only.
- When adding a new project, claim the next free port in this table before writing the docker-compose.

## WebSocket notes

Projects using WebSockets still work through Caddy's `reverse_proxy` directive —
Caddy automatically upgrades the connection. No extra config needed.
