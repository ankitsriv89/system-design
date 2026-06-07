# API Reference — E-Commerce Platform (Project 23)

Base URL: `http://localhost:8104` (or `/p23/` via Caddy)

All request/response bodies are JSON. Timestamps are ISO-8601 UTC.

---

## Products

### List / Search Products
```
GET /v1/products
GET /v1/products?category=books
GET /v1/products?q=pragmatic
```
**Response 200**
```json
[
  {
    "id": "b3f1a2d4-...",
    "sku": "BOOK-001",
    "name": "The Pragmatic Programmer",
    "price": 42.99,
    "stock": 50,
    "category": "books",
    "createdAt": "2026-06-07T10:00:00Z"
  }
]
```

### Get Product
```
GET /v1/products/{id}
```
| Code | Meaning |
|------|---------|
| 200 | Product found |
| 404 | Not found |

### Create Product
```
POST /v1/products
```
```json
{ "sku": "BOOK-001", "name": "The Pragmatic Programmer", "price": 42.99, "stock": 50, "category": "books" }
```
**Response 201** — created product with assigned `id`.

```bash
curl -X POST http://localhost:8104/v1/products \
  -H "Content-Type: application/json" \
  -d '{"sku":"BOOK-001","name":"The Pragmatic Programmer","price":42.99,"stock":50,"category":"books"}'
```

---

## Cart

### View Cart
```
GET /v1/cart/{userId}
```
```json
{
  "userId": "user-42",
  "items": [
    { "productId": "b3f1...", "sku": "BOOK-001", "name": "The Pragmatic Programmer",
      "quantity": 2, "unitPrice": 42.99, "lineTotal": 85.98 }
  ],
  "total": 85.98,
  "updatedAt": "2026-06-07T10:05:00Z"
}
```

### Add Item to Cart
```
POST /v1/cart/{userId}/items
```
```json
{ "productId": "b3f1a2d4-...", "quantity": 2 }
```
Returns the updated Cart.

```bash
curl -X POST http://localhost:8104/v1/cart/user-42/items \
  -H "Content-Type: application/json" \
  -d '{"productId":"b3f1a2d4-...","quantity":2}'
```

### Remove Item
```
DELETE /v1/cart/{userId}/items/{productId}
```

### Clear Cart
```
DELETE /v1/cart/{userId}
```
Response 204 No Content.

---

## Orders

### Checkout (Place Order)
```
POST /v1/orders
```
```json
{ "userId": "user-42", "idempotencyKey": "checkout-2026-06-07-001" }
```
`idempotencyKey` is optional — a random UUID is generated if omitted. Providing the same key a second time returns the existing order.

**Response 201**
```json
{
  "id": "a1b2c3d4-...",
  "userId": "user-42",
  "status": "INVENTORY_RESERVED",
  "total": 85.98,
  "items": [ { "sku": "BOOK-001", "quantity": 2, "unitPrice": 42.99 } ],
  "createdAt": "2026-06-07T10:06:00Z"
}
```

| Code | Meaning |
|------|---------|
| 201 | Order created (saga will proceed async) |
| 400 | Cart is empty |
| 409 | Insufficient stock for one or more items |

```bash
curl -X POST http://localhost:8104/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"userId":"user-42","idempotencyKey":"my-unique-key-001"}'
```

### Get Order
```
GET /v1/orders/{id}
```

### List Orders by User
```
GET /v1/orders?userId=user-42
```

---

## Admin

### List All Orders
```
GET /v1/admin/orders
```

### Ship Order
```
POST /v1/admin/orders/{id}/ship
```
Transitions order from `CONFIRMED` → `SHIPPED`. Returns updated Order.

### Deliver Order
```
POST /v1/admin/orders/{id}/deliver
```
Transitions order from `SHIPPED` → `DELIVERED`.

### Restock Product
```
POST /v1/admin/products/{id}/restock?qty=50
```
Atomically increments stock by `qty`.

---

## Order Status State Machine

```
PENDING
  → INVENTORY_RESERVED  (inventory locked)
  → INVENTORY_FAILED    (out of stock — terminal)

INVENTORY_RESERVED
  → PAYMENT_AUTHORIZED  (payment mock approved)
  → PAYMENT_FAILED      (payment failed — stock released)

PAYMENT_AUTHORIZED
  → CONFIRMED

CONFIRMED
  → SHIPPED             (admin action)

SHIPPED
  → DELIVERED           (admin action)
```

---

## Health & Metrics

```
GET /actuator/health      → {"status":"UP"}
GET /actuator/prometheus  → Prometheus text format
```
