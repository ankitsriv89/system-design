# Architecture — 14: One-to-One Chat System

## System diagram

```mermaid
graph TD
    Browser["Browser / curl"]

    subgraph "14-one-to-one-chat-system (java-runner :8095)"
        API["REST API\n/api/v1/**\nConversationController\nPresenceController\nAuthController"]
        WS["WebSocket Hub\nSTOMP over SockJS\n/ws endpoint"]
        ChatCtrl["ChatController\n@MessageMapping"]
        ConvSvc["ConversationService"]
        MsgSvc["MessageService"]
        PresenceSvc["PresenceService"]
        IdGen["IdGenerator\ntime-based snowflake"]
        JwtFilter["JwtAuthFilter\nJwtUtil"]
        WsInterceptor["WebSocketAuthInterceptor\nSTOMP CONNECT auth"]
        SessionListener["WebSocketSessionListener\nonConnect / onDisconnect"]
        Hub["WebSocketHub\nSimpMessagingTemplate\nconvertAndSendToUser"]
        Producer["MessageProducer\nKafkaTemplate"]
        Consumer["MessageConsumer\n@KafkaListener"]
    end

    subgraph "Storage (infra instance)"
        PG[("PostgreSQL\nconversations\nmessages")]
        Redis[("Redis\npresence:{userId} TTL")]
        Kafka[["Kafka\nchat.messages\nchat.receipts"]]
    end

    subgraph "Observability"
        Prometheus["Prometheus\n:9090"]
        Grafana["Grafana\n:3000"]
    end

    Browser -->|"HTTP REST"| API
    Browser -->|"SockJS / STOMP"| WS
    API --> JwtFilter
    WS --> WsInterceptor
    JwtFilter --> ConvSvc
    JwtFilter --> MsgSvc
    JwtFilter --> PresenceSvc
    WS --> ChatCtrl
    ChatCtrl --> MsgSvc
    ChatCtrl --> PresenceSvc
    ChatCtrl --> Producer
    MsgSvc --> ConvSvc
    MsgSvc --> PG
    MsgSvc --> Producer
    ConvSvc --> PG
    ConvSvc --> IdGen
    PresenceSvc --> Redis
    SessionListener --> PresenceSvc
    Producer --> Kafka
    Kafka --> Consumer
    Consumer --> Hub
    Consumer --> MsgSvc
    Hub --> WS
    API -->|"metrics"| Prometheus
    Prometheus --> Grafana
```

## Happy-path sequence: Alice sends a message to Bob

```mermaid
sequenceDiagram
    participant A as Alice (Browser)
    participant S as Spring Server
    participant PG as PostgreSQL
    participant K as Kafka
    participant B as Bob (Browser)

    A->>S: STOMP SEND /app/chat.send {recipientId:"bob", body:"Hello!"}
    S->>S: JwtAuthFilter validates Bearer token → principal="alice"
    S->>PG: getOrCreate conversation(alice, bob)
    PG-->>S: conversation id=42, lastSeq=7
    S->>PG: nextSeq → UPDATE conversations SET last_seq=8
    S->>PG: INSERT messages (id, conv=42, sender=alice, body, seq=8, SENT)
    S->>K: produce chat.messages key=bob {message, recipientId="bob"}
    K-->>S: consumer receives event (same JVM, chat-service group)
    S->>S: hub.deliver("bob", message)
    alt Bob is online
        S->>B: STOMP /user/queue/inbox {type:MESSAGE, payload:{...seq:8}}
        B->>S: STOMP SEND /app/chat.receipt {messageId, status:DELIVERED}
        S->>K: produce chat.receipts key=alice
        K-->>S: consumer receives receipt
        S->>PG: UPDATE messages SET status=DELIVERED
        S->>A: STOMP /user/queue/inbox {type:RECEIPT, payload:{messageId, DELIVERED}}
    else Bob is offline
        Note over S,PG: message persisted, status=SENT<br/>Bob pulls on reconnect via GET /messages?before=cursor
    end
```

## Components

### REST API (`/api/v1/`)

| Endpoint | Controller | Description |
|---|---|---|
| `POST /auth/token?userId=X` | `AuthController` | Issues demo JWT (no password check) |
| `GET /conversations` | `ConversationController` | Lists caller's conversations |
| `POST /conversations?recipientId=Y` | `ConversationController` | Gets or creates a conversation |
| `POST /conversations/{id}/messages` | `ConversationController` | Sends a message (REST path) |
| `GET /conversations/{id}/messages` | `ConversationController` | Cursor-paged message history |
| `GET /presence?users=a,b` | `PresenceController` | Bulk presence lookup |
| `GET /presence/{userId}` | `PresenceController` | Single user presence |

### WebSocket / STOMP

Clients connect to `/ws` (SockJS) and authenticate via the `Authorization: Bearer <token>` STOMP header on the CONNECT frame. After connecting, clients:
- **Subscribe** to `/user/queue/inbox` to receive inbound messages and receipts.
- **Send** to `/app/chat.send` to deliver a message.
- **Send** to `/app/chat.receipt` to acknowledge delivery or read.
- **Send** to `/app/chat.heartbeat` to refresh presence.

### Domain

- **`Conversation`** — canonical pair `(userA < userB)` with a monotonic `lastSeq` counter. Enforced unique constraint prevents duplicate conversations.
- **`Message`** — belongs to a conversation, has a `seq` (assigned under transaction), and progresses through `SENT → DELIVERED → READ` (transitions are append-only).
- **`IdGenerator`** — time-based 64-bit IDs (timestamp-shifted + sequence), monotonically increasing within a process.

### Kafka fanout

Every persisted message is published to `chat.messages` partitioned by `recipientId`. This decouples message persistence from WebSocket delivery:
- If the recipient is online → consumer delivers immediately via `SimpMessagingTemplate`.
- If offline → message stays in Postgres; the client fetches history on reconnect.

Read/delivery receipts flow through `chat.receipts`, partitioned by sender userId so the sender's receipt notification lands on the same partition as their other events.

### Presence (Redis TTL)

`PresenceStore` writes `presence:{userId} = epochMs` with a 30 s TTL on every heartbeat. The key expires automatically if the connection drops without a clean disconnect. `WebSocketSessionListener` also calls `markOffline()` on the `SessionDisconnectEvent` for immediate eviction.

### Security

`JwtAuthFilter` validates HTTP REST requests. `WebSocketAuthInterceptor` validates STOMP CONNECT frames — this sets the `Principal` on the session so `convertAndSendToUser` can route by userId. Both use the same `JwtUtil` (HS256, configurable secret + expiry).

## Capacity estimates

| Metric | Value | Notes |
|---|---|---|
| WebSocket connections | ~1 000 per JVM | t4g.large has 8 GB; each STOMP session ~50 KB |
| Message throughput | ~500 msg/s per node | Bounded by Postgres write latency (~2 ms p50) |
| Kafka lag | < 100 ms | Single-partition consumer; same JVM as producer |
| Presence TTL | 30 s | Stale presence window after unexpected disconnect |
| Message history | Unlimited | Postgres with index on (conversation_id, seq DESC) |
| Storage growth | ~1 KB/message | body + metadata; 1 M messages/day ≈ 1 GB/day |
| p50 REST send | ~5 ms | 1 DB write + 1 Kafka produce |
| p99 REST send | ~30 ms | Kafka produce latency tail |
| p50 WS delivery | ~10 ms end-to-end | REST send + Kafka round-trip + WS push |
