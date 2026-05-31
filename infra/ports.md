# Port Registry — System Design Projects

Each project gets a fixed host port. This registry prevents conflicts across all 50 projects.

| Port | Project | Caddy Path | Notes |
|------|---------|------------|-------|
| **Shared infra** | | | |
| 5432 | postgres (shared) | — | localhost only, not exposed publicly |
| **Projects** | | | |
| 8080 | [01-rate-limiter](../projects/01-rate-limiter/) | `/p01/` | |
| 8081 | [02-url-shortener](../projects/02-url-shortener/) | `/p02/` | |
| 8082 | 03-pastebin | `/p03/` | |
| 8083 | 04-unique-id-generator | `/p04/` | |
| 8084 | 05-consistent-hashing | `/p05/` | |
| 8085 | 06-load-balancer | `/p06/` | |
| 8086 | 07-api-gateway | `/p07/` | |
| 8087 | 08-basic-key-value-store | `/p08/` | |
| 8088 | 09-caching-system | `/p09/` | |
| 8089 | 10-notification-system | `/p10/` | |
| 8090 | 11-typeahead-autocomplete-system | `/p11/` | |
| 8091 | 12-web-crawler | `/p12/` | |
| 8092 | 13-message-queue | `/p13/` | |
| 8093 | 14-one-to-one-chat-system | `/p14/` | WebSocket |
| 8094 | 15-group-chat-system | `/p15/` | WebSocket |
| 8095 | 16-news-feed-system | `/p16/` | |
| 8096 | 17-proximity-service | `/p17/` | |
| 8097 | 18-instagram-photo-video-sharing-and-feed | `/p18/` | |
| 8098 | 19-twitter-x-timeline-and-posts | `/p19/` | |
| 8099 | 20-whatsapp-real-time-messaging | `/p20/` | WebSocket |
| 8100 | 21-dropbox-file-storage-and-sync | `/p21/` | |
| 8101 | 22-ticket-booking-system | `/p22/` | |
| 8102 | 23-e-commerce-platform | `/p23/` | |
| 8103 | 24-recommendation-system | `/p24/` | |
| 8104 | 25-distributed-cache | `/p25/` | |
| 8105 | 26-uber-ride-sharing-and-matching | `/p26/` | WebSocket |
| 8106 | 27-netflix-video-streaming-platform | `/p27/` | |
| 8107 | 28-youtube-video-upload-and-streaming | `/p28/` | |
| 8108 | 29-tiktok-short-video-platform | `/p29/` | |
| 8109 | 30-facebook-like-social-network-news-feed | `/p30/` | |
| 8110 | 31-google-docs-real-time-collaborative-editing | `/p31/` | WebSocket |
| 8111 | 32-content-delivery-network-cdn | `/p32/` | |
| 8112 | 33-search-engine | `/p33/` | |
| 8113 | 34-google-maps-routing-and-location-services | `/p34/` | |
| 8114 | 35-distributed-database | `/p35/` | |
| 8115 | 36-real-time-analytics-system | `/p36/` | |
| 8116 | 37-ad-serving-and-tracking-system | `/p37/` | |
| 8117 | 38-fraud-detection-system | `/p38/` | |
| 8118 | 39-stock-trading-exchange-system | `/p39/` | WebSocket |
| 8119 | 40-distributed-job-scheduler | `/p40/` | |
| 8120 | 41-event-sourcing-and-cqrs-architecture | `/p41/` | |
| 8121 | 42-multi-tenant-saas-platform | `/p42/` | |
| 8122 | 43-live-video-streaming-at-scale | `/p43/` | WebSocket |
| 8123 | 44-highly-scalable-nosql-database | `/p44/` | |
| 8124 | 45-real-time-multiplayer-game-backend | `/p45/` | WebSocket |
| 8125 | 46-ml-model-serving-infrastructure | `/p46/` | |
| 8126 | 47-geo-distributed-low-latency-system | `/p47/` | |
| 8127 | 48-strongly-consistent-global-database | `/p48/` | |
| 8128 | 49-high-frequency-trading-platform | `/p49/` | WebSocket |
| 8129 | 50-planet-scale-distributed-system | `/p50/` | |
| **Monitoring** | | | |
| 9091 | Prometheus (project 02) | — | Per-project instance |
| 3001 | Grafana (project 02) | — | Per-project instance |

## Convention

- App port = `8079 + project_number` (so project N → port `8079+N`)
- Prometheus per project: `9090 + project_number` → `9091`, `9092`, ...
- Grafana per project: `3000 + project_number` → `3001`, `3002`, ...
- Postgres is shared on port 5432 (host), exposed on `localhost` only

## WebSocket notes

Projects using WebSockets still work through Caddy's `reverse_proxy` directive —
Caddy automatically upgrades the connection. No extra config needed.
