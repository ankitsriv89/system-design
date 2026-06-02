-- migrate.sql: idempotent schema for the API gateway.
-- Apply with:
--   docker exec -i infra-postgres-1 psql -U gw -d apigateway < scripts/migrate.sql

CREATE TABLE IF NOT EXISTS api_keys (
    id            TEXT PRIMARY KEY,
    owner         TEXT        NOT NULL,
    hashed_key    TEXT        NOT NULL UNIQUE,
    scopes        TEXT        NOT NULL DEFAULT '',
    quota_per_min INTEGER     NOT NULL DEFAULT 0,
    active        BOOLEAN     NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS routes (
    id             TEXT PRIMARY KEY,
    path_prefix    TEXT        NOT NULL,
    upstream       TEXT        NOT NULL,
    strip_prefix   BOOLEAN     NOT NULL DEFAULT false,
    auth_required  BOOLEAN     NOT NULL DEFAULT false,
    required_scope TEXT        NOT NULL DEFAULT '',
    max_body_bytes BIGINT      NOT NULL DEFAULT 0,
    timeout_secs   INTEGER     NOT NULL DEFAULT 0,
    active         BOOLEAN     NOT NULL DEFAULT true,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS gateway_decisions (
    id          BIGSERIAL   PRIMARY KEY,
    request_id  TEXT        NOT NULL,
    route_id    TEXT        NOT NULL DEFAULT '',
    key_id      TEXT        NOT NULL DEFAULT '',
    outcome     TEXT        NOT NULL,
    status_code INTEGER     NOT NULL DEFAULT 0,
    latency_ms  BIGINT      NOT NULL DEFAULT 0,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index for time-range analytics queries on decisions.
CREATE INDEX IF NOT EXISTS idx_decisions_recorded_at ON gateway_decisions (recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_decisions_outcome     ON gateway_decisions (outcome);
CREATE INDEX IF NOT EXISTS idx_decisions_key_id      ON gateway_decisions (key_id);
