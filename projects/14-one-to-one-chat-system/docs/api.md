# API Reference — 14: One-to-One Chat System

Base URL: `http://localhost:8095` (or via Caddy: `http://<VM_IP>/p14`)

All authenticated endpoints require: `Authorization: Bearer <token>`

---

## Auth

### Issue token

```bash
curl -X POST "http://localhost:8095/api/v1/auth/token?userId=alice"
```

**Response 200**
```json
{
  "token": "eyJhbGciOiJIUzI1NiJ9...",
  "userId": "alice"
}
```

**Error 400** — userId blank or > 64 chars
```json
{ "error": "userId must be 1-64 chars" }
```

---

## Conversations

### List conversations

```bash
curl "http://localhost:8095/api/v1/conversations" \
  -H "Authorization: Bearer $TOKEN"
```

**Response 200**
```json
[
  {
    "id": 1234567890123,
    "userA": "alice",
    "userB": "bob",
    "createdAt": "2026-06-04T12:00:00Z",
    "lastSeq": 5
  }
]
```

### Create (or get) conversation

```bash
curl -X POST "http://localhost:8095/api/v1/conversations?recipientId=bob" \
  -H "Authorization: Bearer $ALICE_TOKEN"
```

**Response 201** — same shape as list item above.

**Error 401** — missing or invalid token
```json
(empty body, HTTP 401)
```

---

## Messages

### Send message (REST)

```bash
curl -X POST "http://localhost:8095/api/v1/conversations/1234567890123/messages" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"body": "Hello Bob!"}'
```

**Request fields**

| Field | Type | Required | Description |
|---|---|---|---|
| `body` | string | yes | Message text |

**Response 201**
```json
{
  "id": 9876543210001,
  "conversationId": 1234567890123,
  "senderId": "alice",
  "body": "Hello Bob!",
  "seq": 1,
  "status": "SENT",
  "createdAt": "2026-06-04T12:01:00Z"
}
```

**Error 403** — caller is not a participant
```json
{ "error": "not a participant" }
```

**Error 400** — conversation not found
```json
{ "error": "conversation not found: 999" }
```

### Get message history (cursor-based)

```bash
# First page (newest first)
curl "http://localhost:8095/api/v1/conversations/1234567890123/messages?limit=20" \
  -H "Authorization: Bearer $ALICE_TOKEN"

# Next page — pass seq of last message as cursor
curl "http://localhost:8095/api/v1/conversations/1234567890123/messages?before=3&limit=20" \
  -H "Authorization: Bearer $ALICE_TOKEN"
```

**Query parameters**

| Parameter | Type | Default | Description |
|---|---|---|---|
| `before` | long | — | Return messages with seq < this value |
| `limit` | int | 50 | Max messages to return (capped at 100) |

**Response 200** — array of message objects, newest first
```json
[
  { "id": 9876543210003, "seq": 3, "senderId": "bob", "body": "Hey!", "status": "READ", ... },
  { "id": 9876543210002, "seq": 2, "senderId": "alice", "body": "How are you?", "status": "READ", ... },
  { "id": 9876543210001, "seq": 1, "senderId": "alice", "body": "Hello Bob!", "status": "SENT", ... }
]
```

**Message `status` values**

| Value | Meaning |
|---|---|
| `SENT` | Persisted, not yet delivered to recipient's WebSocket session |
| `DELIVERED` | Pushed to recipient's active WebSocket session |
| `READ` | Recipient sent explicit read receipt |

---

## Presence

### Bulk presence lookup

```bash
curl "http://localhost:8095/api/v1/presence?users=alice,bob,charlie" \
  -H "Authorization: Bearer $TOKEN"
```

**Response 200**
```json
[
  { "userId": "alice",   "online": true,  "lastSeenEpochMs": 1717502460000 },
  { "userId": "bob",     "online": false, "lastSeenEpochMs": 1717502340000 },
  { "userId": "charlie", "online": false, "lastSeenEpochMs": null }
]
```

### Single user presence

```bash
curl "http://localhost:8095/api/v1/presence/alice" \
  -H "Authorization: Bearer $TOKEN"
```

**Response 200**
```json
{ "userId": "alice", "online": true, "lastSeenEpochMs": 1717502460000 }
```

---

## WebSocket / STOMP

### Connect

```javascript
const client = new StompJs.Client({
  webSocketFactory: () => new SockJS('http://localhost:8095/ws'),
  connectHeaders: { Authorization: 'Bearer ' + token },
});
client.activate();
```

### Subscribe to inbox

```javascript
client.subscribe('/user/queue/inbox', (frame) => {
  const envelope = JSON.parse(frame.body);
  // envelope.type: "MESSAGE" | "RECEIPT" | "PRESENCE" | "ERROR"
  // envelope.payload: MessageDto | ReceiptEvent | PresenceDto | string
});
```

### Send a message

```javascript
client.publish({
  destination: '/app/chat.send',
  body: JSON.stringify({ recipientId: 'bob', body: 'Hello!' }),
});
```

### Send a receipt

```javascript
client.publish({
  destination: '/app/chat.receipt',
  body: JSON.stringify({ messageId: 9876543210001, userId: 'alice', status: 'READ' }),
});
```

### Send heartbeat

```javascript
client.publish({ destination: '/app/chat.heartbeat', body: '{}' });
```

### Inbound envelope types

**MESSAGE** — new message delivered in real time
```json
{
  "type": "MESSAGE",
  "payload": {
    "id": 9876543210001,
    "conversationId": 1234567890123,
    "senderId": "alice",
    "body": "Hello Bob!",
    "seq": 1,
    "status": "SENT",
    "createdAt": "2026-06-04T12:01:00Z"
  }
}
```

**RECEIPT** — delivery or read acknowledgement
```json
{
  "type": "RECEIPT",
  "payload": { "messageId": 9876543210001, "userId": "bob", "status": "DELIVERED" }
}
```

**PRESENCE** — presence change notification (future use)
```json
{
  "type": "PRESENCE",
  "payload": { "userId": "bob", "online": false, "lastSeenEpochMs": 1717502340000 }
}
```

---

## Health / Observability

```bash
# Health
curl http://localhost:8095/actuator/health
# {"status":"UP","components":{"db":{"status":"UP"},"redis":{"status":"UP"},...}}

# Prometheus metrics
curl http://localhost:8095/actuator/prometheus | grep chat_
# chat_messages_sent_total 42.0
# chat_messages_delivered_total 39.0
# chat_messages_read_total 18.0
```
