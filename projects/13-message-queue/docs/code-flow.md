# Code Flow — Message Queue (Project 13)

## Publish Flow

```mermaid
flowchart TD
    A["POST /v1/topics/{topic}/messages"] --> B["api.Handler.publish()"]
    B --> C{payload empty?}
    C -- yes --> D["400 Bad Request"]
    C -- no --> E["cache.GetTopicPartitions(topic)"]
    E --> F{cache hit?}
    F -- no --> G["db.GetTopic(topic)"]
    G --> H{topic exists?}
    H -- no --> I["404 Not Found"]
    H -- yes --> J["cache.SetTopicPartitions()"]
    F -- yes --> K{key empty?}
    J --> K
    K -- yes --> L["cache.IncrPublishCounter(topic) → counter"]
    K -- no --> M["PartitionFor(key, partitions, 0) → FNV-1a hash"]
    L --> N["PartitionFor('', partitions, counter) → counter%N"]
    M --> O["db.PublishMessage(msg)"]
    N --> O
    O --> P["INSERT INTO messages\nRETURNING offset"]
    P --> Q["metrics.MessagesPublished.Inc()"]
    Q --> R["201 {id, partition, offset}"]
```

### Why each step matters
- **Cache-first partition lookup**: avoids a DB round-trip on the hot publish path (PG read for a rarely-changing value).
- **`IncrPublishCounter`**: Redis INCR is atomic and O(1). Without it, keyless round-robin would require a sequence or random assignment with skew.
- **`BIGSERIAL offset`**: PostgreSQL sequences are non-blocking. The `offset` column acts as the monotonic log position per-partition.

## Poll Flow

```mermaid
flowchart TD
    A["POST /v1/topics/{topic}/messages:poll"] --> B["api.Handler.poll()"]
    B --> C{consumer_group empty?}
    C -- yes --> D["400 Bad Request"]
    C -- no --> E["db.PollMessages(PollRequest)"]
    E --> F["CTE: SELECT id FOR UPDATE SKIP LOCKED\nWHERE visible_at <= NOW() AND acked_at IS NULL\nLIMIT N"]
    F --> G["UPDATE messages SET\n  visible_at = NOW() + timeout\n  consumer_group = group\n  delivery_attempts = delivery_attempts + 1"]
    G --> H["RETURNING message rows"]
    H --> I["metrics.MessagesPolled.Add(len)"]
    I --> J["200 {messages:[...], count:N}"]
```

### Why `FOR UPDATE SKIP LOCKED`
Without this, two concurrent consumers polling the same topic would see the same rows, process them both, and produce duplicate work. `SKIP LOCKED` tells PG to skip rows already locked by another transaction, giving each consumer a unique, non-overlapping set.

## Ack Flow

```mermaid
flowchart TD
    A["POST /v1/messages/{id}:ack"] --> B["api.Handler.ack()"]
    B --> C["db.AckMessage(AckRequest)"]
    C --> D["UPDATE messages SET acked_at = NOW()\nWHERE id=$1 AND consumer_group=$2 AND acked_at IS NULL"]
    D --> E{rows affected = 0?}
    E -- yes --> F["404 Not Found / already acked"]
    E -- no --> G["metrics.MessagesAcked.Inc()"]
    G --> H["200 {status:'acked'}"]
```

### Why `AND acked_at IS NULL`
Prevents double-ack from silently succeeding. Without this guard a retry from a crashed consumer could overwrite `acked_at` and corrupt audit logs.

## Reaper Flow

```mermaid
flowchart TD
    A["reaper.Run(ctx)"] --> B["ticker = 5s"]
    B --> C["reaper.sweep(ctx)"]
    C --> D["db.MoveExpiredToDeadLetter()"]
    D --> E["UPDATE dead_lettered=true\nWHERE delivery_attempts >= 5\nAND visible_at <= NOW()\nAND acked_at IS NULL"]
    E --> F["metrics.MessagesDeadLettered.Add(n)"]
    F --> G["db.RestoreExpiredMessages()"]
    G --> H["UPDATE visible_at=NOW(), consumer_group=NULL\nWHERE delivery_attempts < 5\nAND visible_at <= NOW()\nAND acked_at IS NULL"]
    H --> I["metrics.MessagesRestored.Add(n)"]
    I --> B
```

### Why DLQ promotion runs before restore
If the order were reversed, a message at exactly attempt 5 could be restored first (making it visible again) and then immediately DLQ'd on the next sweep, leading to a phantom extra delivery. Promoting first prevents this off-by-one.

## Call Graph Summary

```mermaid
graph LR
    main --> api.NewHandler
    main --> store.NewDB
    main --> store.NewCache
    main --> worker.New
    main --> worker.Reaper.Run

    api.Handler.publish --> store.Cache.GetTopicPartitions
    api.Handler.publish --> store.Cache.IncrPublishCounter
    api.Handler.publish --> store.DB.GetTopic
    api.Handler.publish --> store.DB.PublishMessage
    api.Handler.publish --> queue.PartitionFor

    api.Handler.poll --> store.DB.PollMessages

    api.Handler.ack --> store.DB.AckMessage

    worker.Reaper.sweep --> store.DB.MoveExpiredToDeadLetter
    worker.Reaper.sweep --> store.DB.RestoreExpiredMessages
```
