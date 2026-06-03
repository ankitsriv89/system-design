# API Reference — Basic Key-Value Store (08)

Base URL: `http://localhost:8088`

---

## PUT /v1/kv/{key}

Store a value. The request body is stored verbatim (raw bytes, no encoding).

```bash
curl -X PUT http://localhost:8088/v1/kv/my-key \
     --data "hello world"
```

**Constraints**
- Key must not be empty.
- Value must not exceed 1 MiB.

**Response 200**
```json
{"ok":true}
```

**Response 400**
```json
{"error":"bad request"}
```

**Response 413**
```json
{"error":"value exceeds 1 MiB limit"}
```

---

## GET /v1/kv/{key}

Retrieve a value. The response body is the raw stored bytes.

```bash
curl http://localhost:8088/v1/kv/my-key
```

**Response 200** — `Content-Type: application/octet-stream`
```
hello world
```

**Response 404**
```json
{"error":"key not found"}
```

---

## DELETE /v1/kv/{key}

Delete a key by writing a tombstone. The key will not be returned by subsequent GETs.

```bash
curl -X DELETE http://localhost:8088/v1/kv/my-key
```

**Response 200**
```json
{"ok":true}
```

---

## POST /v1/admin/compact

Manually trigger a compaction. Merges all L0 SSTables into a single L1 file,
resolving duplicates (newest wins) and dropping tombstones.

```bash
curl -X POST http://localhost:8088/v1/admin/compact
```

**Response 200**
```json
{"ok":true}
```

---

## GET /v1/admin/stats

Returns live engine statistics.

```bash
curl http://localhost:8088/v1/admin/stats | jq .
```

**Response 200**
```json
{
  "writes": 1042,
  "reads": 830,
  "deletes": 12,
  "flushes": 2,
  "compactions": 1,
  "memtable_bytes": 204800,
  "memtable_keys": 512,
  "sst_count": 1,
  "wal_entries": 512,
  "sstables": [
    {
      "seq": 4,
      "level": 1,
      "min_key": "key-0000",
      "max_key": "key-9999",
      "count": 987
    }
  ]
}
```

| Field | Description |
|---|---|
| `writes` | Total SET operations since start |
| `reads` | Total GET operations since start |
| `deletes` | Total DELETE operations since start |
| `flushes` | Number of memtable flushes to L0 SSTables |
| `compactions` | Number of compaction runs |
| `memtable_bytes` | Approximate size of active memtable in bytes |
| `memtable_keys` | Number of entries in active memtable |
| `sst_count` | Total number of SSTable files on disk |
| `wal_entries` | Number of entries currently in the WAL |
| `sstables` | Array of SSTable metadata (seq, level, key range, count) |

---

## GET /healthz

Liveness check.

```bash
curl http://localhost:8088/healthz
```

**Response 200**
```json
{"status":"ok"}
```

---

## GET /metrics

Prometheus metrics endpoint.

```bash
curl http://localhost:8088/metrics
```

Key metrics:

| Metric | Type | Labels | Description |
|---|---|---|---|
| `basic_key_value_store_operation_duration_seconds` | Histogram | `op`, `result` | Latency per operation type |
| `basic_key_value_store_operations_total` | Counter | `op`, `result` | Total operation count |

`op` values: `get`, `set`, `delete`, `compact`
`result` values: `ok`, `hit`, `miss`, `error`
