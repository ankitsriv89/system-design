# Code Flow — WhatsApp Real-Time Messaging (Project 20)

## 1. Register + Login

```mermaid
flowchart TD
    A[POST /v1/auth/register] --> B[AuthController.register]
    B --> C{username exists?}
    C -- yes --> D[400 Bad Request]
    C -- no --> E[AppUser saved to Postgres]
    E --> F[password hash stored in-memory map]
    F --> G[JwtService.generate\nHS256 JWT, uid claim]
    G --> H[AuthResponse{token, userId, username}]

    I[POST /v1/auth/login] --> J[AuthController.login]
    J --> K[UserRepository.findByUsername]
    K --> L{found?}
    L -- no --> M[401 Bad Credentials]
    L -- yes --> N[BCrypt.matches\npassword vs hash]
    N -- fail --> M
    N -- ok --> G
```

**Why**: JWT is stateless — the server signs once and never stores sessions. `uid` claim lets downstream code avoid an extra DB lookup for userId.

---

## 2. Device registration + WS ticket

```mermaid
flowchart TD
    A[POST /v1/devices] --> B[SecurityConfig JWT filter\nextract username from Bearer]
    B --> C[DeviceController.register]
    C --> D[DeviceService.register]
    D --> E[Device saved\npublic_key stored as spki B64]
    E --> F[DeviceResponse{id, userId, publicKey}]

    G[POST /v1/ws-ticket?deviceId=N] --> H[WsTicketController.issue]
    H --> I[WsTicketStore.issue]
    I --> J[UUID ticket → Redis\nKEY ws:ticket:uuid VALUE username:deviceId TTL 30s]
    J --> K[{ticket: uuid}]
```

**Why**: The ticket ensures the JWT never appears in the WebSocket URL (which would be logged by reverse proxies and appear in browser history). The 30-second TTL bounds the window of ticket theft.

---

## 3. WebSocket connect

```mermaid
flowchart TD
    A["WS UPGRADE /ws/v1/session?ticket=uuid"] --> B[SessionHandler.afterConnectionEstablished]
    B --> C[WsTicketStore.redeem\nGETDEL ws:ticket:uuid]
    C --> D{ticket found?}
    D -- no/expired --> E[close POLICY_VIOLATION]
    D -- yes → username:deviceId --> F[DeviceRepository.findById]
    F --> G{device.user.username == username?}
    G -- no --> E
    G -- yes --> H[device.touch — update last_seen]
    H --> I[SessionStore.register\ndeviceId → WebSocketSession in-process\nSET ws:route:deviceId NODE_ID TTL 90s Redis]
    I --> J[drain pending receipts\nReceiptService.pendingForDevice]
    J --> K[WS push: connected + backlog count]
```

**Why**: `GETDEL` makes the ticket single-use atomically. `ws:route:deviceId` in Redis is the foundation for future multi-node routing — other nodes can see which node owns a device connection.

---

## 4. Send a DM message

```mermaid
flowchart TD
    A["POST /v1/messages\n{chatId:'dm:1:2', ciphertext:B64}"] --> B[MessageController.send]
    B --> C[MessageService.send]
    C --> D[assertParticipant\ncallerUserId must be in dm:1:2 parts]
    D --> E[Base64.decode ciphertext]
    E --> F[Message saved\nchat_id, sender_id, ciphertext BYTEA]
    F --> G[resolveRecipients\nparse dm: → filter sender out]
    G --> H[for each recipient device\nreceipts.save Receipt state=SENT]
    H --> I[KafkaTemplate.send\nwhatsapp.messages topic]
    I --> J[200 MessageResponse]
```

**Why**: Message is committed to Postgres before Kafka publish — durability is guaranteed even if Kafka is temporarily unavailable. Receipts are pre-created in SENT state so offline devices have a record to drain on reconnect.

---

## 5. Kafka fan-out (MessageRouter)

```mermaid
flowchart TD
    A[Kafka consumer\nwhatsapp.messages] --> B[MessageRouter.onMessage]
    B --> C[deserialize KafkaMessageEvent]
    C --> D[for each recipientUserId]
    D --> E[DeviceRepository.findByUserId]
    E --> F[SessionHandler.push\nWS frame type=message]
    F --> G{session open?}
    G -- yes --> H[send TextMessage]
    G -- no --> I[no-op — device offline\nreceipt stays SENT]
    H --> J[ReceiptService.markDelivered\nadvance SENT→DELIVERED]
    J --> K[publish whatsapp.receipts\nwith participantUserIds]
```

**Why**: Fan-out happens asynchronously in the Kafka consumer so the sender's HTTP call returns immediately. Offline devices keep `state=SENT` and drain via `/v1/messages/sync` on reconnect.

---

## 6. Receipt state machine

```mermaid
flowchart LR
    S[SENT] -->|delivery push| D[DELIVERED]
    D -->|client acks READ| R[READ]
    R -->|any| R
    D -->|SENT| D
    S -->|SENT| S
```

```mermaid
flowchart TD
    A[advance called] --> B{ownership\nverified?}
    B -- no --> C[403 AccessDenied]
    B -- yes --> D[Receipt.findById]
    D --> E{current.canAdvanceTo next?}
    E -- no --> F[no-op]
    E -- yes --> G[r.advance — update state + updatedAt]
    G --> H[receipts.save]
    H --> I[publish KafkaReceiptEvent\nchatId + participantUserIds]
    I --> J[MessageRouter.onReceipt\npush only to participant devices]
```

**Why**: Forward-only invariant prevents a READ receipt from being downgraded to DELIVERED by a duplicate Kafka delivery. Scoped push (finding 2) prevents one user seeing another chat's receipt updates.

---

## 7. Offline sync

```mermaid
flowchart TD
    A["GET /v1/messages/sync?chatId=dm:1:2&since=T"] --> B[MessageController.sync]
    B --> C[MessageService.sync\nassertParticipant first]
    C --> D[MessageRepository\nfindByChatIdAndCreatedAtAfter\nORDER BY created_at ASC LIMIT 200]
    D --> E[List<MessageResponse> ciphertext B64]
    E --> F[client decrypts with chatKey]
    F --> G[WS receipt acknowledge DELIVERED]
```

**Why**: `since` allows incremental sync — client passes the `createdAt` of the last known message. The 200-item page cap prevents a single reconnect from saturating the DB.

---

## 8. Group message send

```mermaid
flowchart TD
    A["POST /v1/messages\n{chatId:'group:5', ciphertext:B64}"] --> B[assertParticipant\ngroupMembers.existsByGroupIdAndUserId]
    B --> C[Message saved]
    C --> D[resolveRecipients for group:5\nGET all GroupMember.userId\nexcluding sender]
    D --> E[receipts for all member devices]
    E --> F[Kafka publish → fan-out\nto all member devices via WS]
```

---

## Call graph summary

```mermaid
graph LR
    HTTP[HTTP Controllers] --> SVC[Services]
    WS[SessionHandler] --> STORE[Store / SessionStore / WsTicketStore]
    SVC --> REPO[Repositories JPA]
    SVC --> KAFKA[KafkaTemplate]
    REPO --> PG[(PostgreSQL)]
    STORE --> REDIS[(Redis)]
    KAFKA --> KBROKER[[Kafka broker]]
    KBROKER --> ROUTER[MessageRouter]
    ROUTER --> WS
    ROUTER --> SVC
```
