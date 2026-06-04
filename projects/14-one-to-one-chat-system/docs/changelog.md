# Changelog — 14: One-to-One Chat System

All notable changes to this project will be documented in this file.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)

## [0.1.0] — 2026-06-04

### Added

- **Demo auth**: `POST /api/v1/auth/token?userId=X` issues HS256 JWT; no password store needed for demo
- **Conversation API**: `POST /api/v1/conversations?recipientId=Y` — idempotent get-or-create with canonical ordering (`userA < userB`)
- **Message send (REST)**: `POST /api/v1/conversations/{id}/messages` with participant guard
- **Message history**: `GET /api/v1/conversations/{id}/messages?before={seq}&limit={n}` — cursor-based pagination, max 100 per page
- **WebSocket STOMP endpoint**: `/ws` (SockJS) with JWT auth on CONNECT frame via `WebSocketAuthInterceptor`
- **Real-time send**: `STOMP SEND /app/chat.send` — persists and fans out via Kafka
- **Read/delivery receipts**: `STOMP SEND /app/chat.receipt` — flows through `chat.receipts` Kafka topic back to sender
- **Presence heartbeat**: `STOMP SEND /app/chat.heartbeat` + automatic heartbeat on every inbound frame
- **Presence API**: `GET /api/v1/presence?users=a,b,c` bulk + `GET /api/v1/presence/{userId}` single
- **Redis presence**: `presence:{userId}` key with 30 s TTL; online/offline transitions on connect/disconnect events
- **Kafka fanout**: `chat.messages` topic partitioned by `recipientId`; consumer delivers to WebSocket or leaves in DB for offline
- **Offline delivery**: messages persisted with `SENT` status; client fetches history on reconnect
- **Message ordering**: per-conversation monotonic `seq` counter assigned under `@Transactional`
- **Flyway migrations**: `V1__init.sql` — `conversations` and `messages` tables with proper indexes
- **Prometheus metrics**: `chat_messages_sent_total`, `chat_messages_delivered_total`, `chat_messages_read_total`; exposed at `/actuator/prometheus`
- **Tutorial frontend**: three-panel UI — controls+concept panel, animated Canvas message flow (A ↔ Hub ↔ Kafka ↔ Postgres ↔ B), live message threads with receipt badges, event log
- **Docker**: multi-stage build (`eclipse-temurin:21-jdk-alpine` builder, `eclipse-temurin:21-jre-alpine` runtime); env-var injection for all shared infra coordinates

### Performance

- HikariCP pool capped at 10 connections — sufficient for demo; avoids saturating shared Postgres
- Kafka producer partitioned by `recipientId` — all messages to the same user land on the same partition for ordering
- Cursor-based pagination uses `(conversation_id, seq DESC)` index — O(log n) regardless of history depth
- JVM heap capped at 512 MB for co-existence on shared `java-runner` instance
