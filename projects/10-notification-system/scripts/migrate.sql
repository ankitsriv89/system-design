-- Schema for notification-system (project 10)

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS templates (
    id          TEXT        PRIMARY KEY,
    channel     TEXT        NOT NULL,
    subject     TEXT        NOT NULL DEFAULT '',
    body        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS preferences (
    user_id     TEXT        NOT NULL,
    channel     TEXT        NOT NULL,
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    quiet_start INTEGER     NOT NULL DEFAULT -1,
    quiet_end   INTEGER     NOT NULL DEFAULT -1,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, channel)
);

CREATE TABLE IF NOT EXISTS notifications (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         TEXT        NOT NULL,
    channel         TEXT        NOT NULL,
    template_id     TEXT        NOT NULL DEFAULT '',
    params          JSONB       NOT NULL DEFAULT '{}',
    subject         TEXT        NOT NULL DEFAULT '',
    body            TEXT        NOT NULL DEFAULT '',
    priority        INTEGER     NOT NULL DEFAULT 1,
    status          TEXT        NOT NULL DEFAULT 'pending',
    idempotency_key TEXT        UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id   ON notifications (user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_status    ON notifications (status);
CREATE INDEX IF NOT EXISTS idx_notifications_created   ON notifications (created_at DESC);

CREATE TABLE IF NOT EXISTS delivery_attempts (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID        NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    provider        TEXT        NOT NULL,
    attempt_number  INTEGER     NOT NULL DEFAULT 1,
    status          TEXT        NOT NULL,
    error_msg       TEXT,
    latency_ms      BIGINT      NOT NULL DEFAULT 0,
    attempted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_attempts_notification ON delivery_attempts (notification_id);
CREATE INDEX IF NOT EXISTS idx_attempts_provider     ON delivery_attempts (provider, attempted_at DESC);
