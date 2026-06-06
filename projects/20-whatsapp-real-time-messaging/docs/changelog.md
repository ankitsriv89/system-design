# Changelog — WhatsApp Real-Time Messaging (Project 20)

All notable changes follow [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] — 2026-06-07

### Added

**Milestone 1 — Device / session model**
- `AppUser`, `Device` domain entities; Flyway V1 schema (`app_user`, `device`, `message`, `receipt`)
- JWT auth: `POST /v1/auth/register`, `POST /v1/auth/login` (HS256, JJWT 0.12)
- Device registration: `POST /v1/devices` — stores ECDH public key per device
- `SessionStore` — in-process `deviceId→WebSocketSession` map + Redis route TTL (90 s)
- `WsTicketStore` — 30 s single-use Redis ticket so JWT never appears in WebSocket URL
- `WsTicketController` — `POST /v1/ws-ticket?deviceId=N`
- `SessionHandler` — Spring WebSocket handler at `/ws/v1/session?ticket=…`; ticket redemption, heartbeat refresh, offline backlog drain on connect, `ping`/`receipt` frame dispatch

**Milestone 2 — Encrypted message routing**
- `Message` entity; `POST /v1/messages` — accepts Base64 ciphertext, Postgres commit, Kafka publish
- `KafkaMessageEvent` / `KafkaReceiptEvent` DTOs with String serialisation (Jackson)
- `MessageRouter` Kafka consumer — per-device WebSocket fan-out, marks DELIVERED for online devices

**Milestone 3 — Receipts and sync**
- `Receipt`, `ReceiptId`, `ReceiptState` (SENT → DELIVERED → READ, forward-only)
- `ReceiptService` — owner-verified `advance()`, internal `markDelivered()` / `advanceFromDevice()`
- `POST /v1/receipts` — advance receipt state with device ownership check
- `GET /v1/messages/sync?chatId=…&since=…` — participant-scoped offline backlog drain (page 200)

**Milestone 4 — Group messaging**
- `ChatGroup`, `GroupMember` entities; Flyway V2 schema (`chat_group`, `group_member`)
- `POST /v1/groups`, `POST /v1/groups/{id}/members` (owner-only), `GET /v1/groups`
- Group fan-out via `resolveRecipients("group:N")` using `GroupMemberRepository`

**Web UI**
- Dark WhatsApp-style three-panel layout (sidebar / chat / API log)
- WebCrypto ECDH P-256 key pair generated per browser tab; AES-GCM-256 encryption before every send
- Real-time WebSocket push; `✓` / `✓✓` / `✓✓` (blue) receipt tick indicators
- Offline toggle — closes WebSocket, simulates device going offline, reconnects and drains sync
- Group creation modal
- All DOM insertions use `textContent` / safe DOM methods (no `innerHTML` with user data)

**Infrastructure**
- Port 8101, Caddy `/p20/`, actuator blocked at proxy
- `infra/initdb/20_whatsapp.sql` — DB + user provisioning
- Docker multi-stage build (`gradle:8-jdk21-alpine` → `eclipse-temurin:21-jre-alpine`)
- Prometheus metrics via Micrometer, `management.endpoints.web.exposure.include: health,prometheus`

### Fixed (security — applied before tagging)

- **IDOR on sync**: `assertParticipant()` enforces caller is a DM member or group member before returning messages
- **IDOR on send**: `assertParticipant()` called at top of `MessageService.send()` — unknown/unjoined chatIds are rejected with 403
- **IDOR on receipts**: device ownership verified (`device.getUser().getUsername().equals(callerUsername)`) before advancing state
- **JWT in WS URL**: replaced `?token=JWT` with `?ticket=<uuid>` (30 s single-use Redis ticket)
- **Receipt broadcast leak**: `KafkaReceiptEvent` carries `participantUserIds`; router pushes only to participant devices, not all connected sessions
- **Weak default JWT secret**: `JwtService` enforces ≥ 32 bytes on startup; logs warning if dev placeholder is active
