-- Message queue schema migration v1

CREATE TABLE IF NOT EXISTS topics (
    name             TEXT PRIMARY KEY,
    partitions       INT NOT NULL DEFAULT 1,
    retention_seconds BIGINT NOT NULL DEFAULT 86400,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS messages (
    id                TEXT PRIMARY KEY,
    topic             TEXT NOT NULL REFERENCES topics(name),
    partition         INT NOT NULL DEFAULT 0,
    offset            BIGSERIAL,
    key               TEXT NOT NULL DEFAULT '',
    payload           BYTEA NOT NULL,
    published_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    visible_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivery_attempts INT NOT NULL DEFAULT 0,
    acked_at          TIMESTAMPTZ,
    dead_lettered     BOOLEAN NOT NULL DEFAULT FALSE,
    consumer_group    TEXT
);

-- Polling query: fetch visible, unacked, non-DLQ messages ordered by partition+offset.
CREATE INDEX IF NOT EXISTS idx_messages_poll
    ON messages (topic, partition, offset)
    WHERE acked_at IS NULL AND dead_lettered = FALSE;

-- Reaper query: find expired leases.
CREATE INDEX IF NOT EXISTS idx_messages_reaper
    ON messages (visible_at)
    WHERE acked_at IS NULL AND dead_lettered = FALSE;

-- DLQ query.
CREATE INDEX IF NOT EXISTS idx_messages_dlq
    ON messages (topic, offset DESC)
    WHERE dead_lettered = TRUE;
