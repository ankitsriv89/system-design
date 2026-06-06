# API Reference — WhatsApp Real-Time Messaging (Project 20)

Base URL (local): `http://localhost:8101`  
Base URL (deployed): `https://<host>/p20`

All endpoints except `/v1/auth/*` require `Authorization: Bearer <jwt>`.

---

## Auth

### Register

```bash
curl -s -X POST http://localhost:8101/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123"}' | jq
```

**Response 200**
```json
{
  "token": "eyJhbGc...",
  "userId": 1,
  "username": "alice"
}
```

**Error 400** — username taken
```json
{ "error": "Username already taken" }
```

---

### Login

```bash
curl -s -X POST http://localhost:8101/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"secret123"}' | jq
```

**Response 200** — same shape as register  
**Error 401** — bad credentials

---

## Devices

### Register a device (store ECDH public key)

```bash
curl -s -X POST http://localhost:8101/v1/devices \
  -H 'Authorization: Bearer $TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"publicKey":"MFkwEwYHKoZIzj0CAQ...","label":"browser-tab-1"}' | jq
```

**Response 200**
```json
{
  "id": 3,
  "userId": 1,
  "publicKey": "MFkwEwYH...",
  "label": "browser-tab-1",
  "lastSeen": null
}
```

---

### List my devices

```bash
curl -s http://localhost:8101/v1/devices \
  -H 'Authorization: Bearer $TOKEN' | jq
```

**Response 200** — array of `DeviceResponse`

---

## WebSocket ticket

A one-time ticket must be obtained before opening a WebSocket connection.  
The ticket expires in 30 seconds and is consumed on first use.

```bash
curl -s -X POST "http://localhost:8101/v1/ws-ticket?deviceId=3" \
  -H 'Authorization: Bearer $TOKEN' | jq
```

**Response 200**
```json
{ "ticket": "f47ac10b-58cc-4372-a567-0e02b2c3d479" }
```

**Error 403** — JWT missing or expired

---

## WebSocket session

Connect with the ticket (not the JWT):

```
ws://localhost:8101/ws/v1/session?ticket=<uuid>
```

### Incoming frame types (server → client)

| Type | Payload | When |
|---|---|---|
| `connected` | `{deviceId}` | After successful ticket redemption |
| `backlog` | `{count, messageIds[]}` | Pending offline messages on connect |
| `message` | `{id, chatId, senderId, ciphertext, createdAt}` | New message pushed by router |
| `receipt` | `{messageId, deviceId, state}` | Receipt state change for a chat you're in |
| `pong` | `null` | Response to ping |
| `error` | `"reason string"` | Bad envelope or receipt advance failure |

### Outgoing frame types (client → server)

| Type | Payload | Purpose |
|---|---|---|
| `ping` | `null` | Keepalive (every 30 s); refreshes Redis route TTL |
| `receipt` | `{messageId, state}` | Advance receipt state for this device |

**Example ping/pong**
```json
→ {"type":"ping","payload":null}
← {"type":"pong","payload":null}
```

**Example read receipt**
```json
→ {"type":"receipt","payload":{"messageId":42,"state":"READ"}}
```

---

## Messages

### Send a message

`chatId` formats:
- DM: `dm:{smallerUserId}:{largerUserId}` e.g. `dm:1:2`
- Group: `group:{groupId}` e.g. `group:5`

`ciphertext` is Base64-encoded AES-GCM ciphertext (IV prepended, 12 bytes).  
The server stores it opaquely — no decryption ever occurs server-side.

```bash
# Plaintext "hello" encrypted client-side → B64 ciphertext
CIPHER=$(echo -n "hello" | base64)   # demo only — real client uses WebCrypto

curl -s -X POST http://localhost:8101/v1/messages \
  -H 'Authorization: Bearer $TOKEN' \
  -H 'Content-Type: application/json' \
  -d "{\"chatId\":\"dm:1:2\",\"ciphertext\":\"$CIPHER\"}" | jq
```

**Response 200**
```json
{
  "id": 101,
  "chatId": "dm:1:2",
  "senderId": 1,
  "ciphertext": "aGVsbG8=",
  "createdAt": "2026-06-07T10:00:00Z"
}
```

**Error 403** — sender is not a participant of the chat  
**Error 400** — invalid Base64 ciphertext

---

### Sync offline messages

```bash
# Full history for a chat
curl -s "http://localhost:8101/v1/messages/sync?chatId=dm:1:2" \
  -H 'Authorization: Bearer $TOKEN' | jq

# Incremental — messages after a timestamp
curl -s "http://localhost:8101/v1/messages/sync?chatId=dm:1:2&since=2026-06-07T10:00:00Z" \
  -H 'Authorization: Bearer $TOKEN' | jq
```

**Response 200** — array of `MessageResponse`, up to 200 items, ordered by `createdAt ASC`  
**Error 403** — caller is not a participant

---

## Receipts

### Advance receipt state

```bash
curl -s -X POST "http://localhost:8101/v1/receipts?deviceId=3" \
  -H 'Authorization: Bearer $TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"messageId":101,"state":"READ"}' -o /dev/null -w '%{http_code}'
# → 204
```

**States**: `SENT` → `DELIVERED` → `READ` (forward-only, non-reversible)  
**Error 403** — device is not owned by the caller  
**Error 400** — receipt not found or state transition not allowed

---

## Groups

### Create a group

```bash
curl -s -X POST http://localhost:8101/v1/groups \
  -H 'Authorization: Bearer $TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"name":"Project Team","memberUserIds":[2,3]}' | jq
```

**Response 200**
```json
{
  "id": 5,
  "name": "Project Team",
  "chatId": "group:5",
  "ownerId": 1
}
```

---

### Add a member (owner only)

```bash
curl -s -X POST "http://localhost:8101/v1/groups/5/members?userId=4" \
  -H 'Authorization: Bearer $TOKEN' -o /dev/null -w '%{http_code}'
# → 204
```

**Error 403** — caller is not the group owner

---

### List my groups

```bash
curl -s http://localhost:8101/v1/groups \
  -H 'Authorization: Bearer $TOKEN' | jq
```

**Response 200** — array of `GroupResponse`

---

## Observability

```bash
# Health
curl http://localhost:8101/actuator/health
# → {"status":"UP"}

# Prometheus metrics (Caddy blocks /pNN/actuator/prometheus externally)
curl http://localhost:8101/actuator/prometheus | grep whatsapp
```

Key metrics exposed via Micrometer:
- `http_server_requests_seconds` — request latency by endpoint
- `spring_kafka_consumer_records_lag` — Kafka consumer lag
- `jvm_memory_used_bytes` — JVM heap usage
- `hikaricp_connections_active` — DB connection pool saturation

---

## End-to-end demo script

```bash
# 1. Register two users
ALICE=$(curl -s -X POST http://localhost:8101/v1/auth/register \
  -H 'Content-Type: application/json' -d '{"username":"alice","password":"password1"}')
BOB=$(curl -s -X POST http://localhost:8101/v1/auth/register \
  -H 'Content-Type: application/json' -d '{"username":"bob","password":"password2"}')

ALICE_TOKEN=$(echo $ALICE | jq -r .token)
ALICE_ID=$(echo $ALICE | jq -r .userId)
BOB_TOKEN=$(echo $BOB | jq -r .token)
BOB_ID=$(echo $BOB | jq -r .userId)

# 2. Register devices (one each)
ALICE_DEV=$(curl -s -X POST http://localhost:8101/v1/devices \
  -H "Authorization: Bearer $ALICE_TOKEN" -H 'Content-Type: application/json' \
  -d '{"publicKey":"ALICE_PUB_KEY","label":"alice-tab"}')
ALICE_DEV_ID=$(echo $ALICE_DEV | jq -r .id)

BOB_DEV=$(curl -s -X POST http://localhost:8101/v1/devices \
  -H "Authorization: Bearer $BOB_TOKEN" -H 'Content-Type: application/json' \
  -d '{"publicKey":"BOB_PUB_KEY","label":"bob-tab"}')

# 3. Derive chatId (smaller ID first)
CHAT_ID="dm:${ALICE_ID}:${BOB_ID}"
[ "$BOB_ID" -lt "$ALICE_ID" ] && CHAT_ID="dm:${BOB_ID}:${ALICE_ID}"

# 4. Send message from Alice to Bob
curl -s -X POST http://localhost:8101/v1/messages \
  -H "Authorization: Bearer $ALICE_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"chatId\":\"$CHAT_ID\",\"ciphertext\":\"$(echo -n 'hello bob' | base64)\"}" | jq

# 5. Bob syncs (simulating offline reconnect)
curl -s "http://localhost:8101/v1/messages/sync?chatId=$CHAT_ID" \
  -H "Authorization: Bearer $BOB_TOKEN" | jq
```
