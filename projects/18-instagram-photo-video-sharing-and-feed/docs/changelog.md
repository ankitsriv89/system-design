# Changelog

All notable changes to this project are documented here. Format based on
[Keep a Changelog](https://keepachangelog.com/).

## [0.1.0] — 2026-06-06

### Added
- **Milestone 1 — Upload + metadata**: two-phase presigned upload
  (`POST /v1/media/uploads` → presigned MinIO PUT, `POST /v1/media/{id}/complete`),
  PENDING media records, `media.uploaded` Kafka event. Seed users + `X-User-Id`
  identity model.
- **Milestone 2 — Variant worker**: Kafka consumer of `media.uploaded` generates
  real thumbnail/small/medium image variants (Thumbnailator), writes them to
  MinIO, marks media PROCESSED, emits `media.processed`. Video accepted as
  original-only.
- **Milestone 3 — Feed + engagement**: posts, social graph (follows), hybrid
  fanout (push to Redis ZSET timelines for normal authors, pull at read time for
  celebrities above the follower threshold), time-decay ranking, idempotent
  like/unlike with durable Postgres rows + atomic Redis counters
  (`post.created`, `post.liked` events).
- **Milestone 4 — CDN / cache invalidation**: `MediaUrlService` (object key →
  public URL; single swap point between local path and Cloudflare host), local
  CDN-shaped serving at `/media/**` with immutable Cache-Control, and a
  cache-invalidation seam (`CdnInvalidationService`) that purges on reprocess
  (logs locally; Cloudflare `purge_cache` in production).
- Interactive tutorial UI: live pipeline visualization, in-browser image
  generation, upload→post→feed→like flow, API log panel.
- Flyway `V1__init.sql`: users, follows, media (status + variants JSONB), posts,
  engagements.
- 15 unit tests across media, variants, engagement, fanout, and URL building.

### Changed
- Actuator endpoints blocked at the public Caddy proxy; `health.show-details`
  set to `when-authorized` (scrape via internal network only).

### Performance
- Hybrid fanout bounds write amplification to O(followers) and skips celebrities
  entirely on the write path.
- Home timelines trimmed to 500 entries per user to bound Redis memory.
- Variants are immutable with year-long immutable Cache-Control for edge reuse.
