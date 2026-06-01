-- migrate.sql — Pastebin schema
-- Idempotent: safe to run multiple times.

CREATE TABLE IF NOT EXISTS pastes (
    id              TEXT        PRIMARY KEY,
    owner_id        TEXT,                           -- NULL for anonymous
    title           TEXT,
    language        TEXT,
    visibility      TEXT        NOT NULL DEFAULT 'public'
                                CHECK (visibility IN ('public','unlisted','private')),
    size_bytes      BIGINT      NOT NULL,
    object_key      TEXT        NOT NULL,
    expires_at      TIMESTAMPTZ,
    burn_after_read BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Fast expiry sweep: only rows that actually have an expiry.
CREATE INDEX IF NOT EXISTS idx_pastes_expires_at
    ON pastes (expires_at)
    WHERE expires_at IS NOT NULL;

-- Owner lookup (list your pastes).
CREATE INDEX IF NOT EXISTS idx_pastes_owner_id
    ON pastes (owner_id)
    WHERE owner_id IS NOT NULL;

-- Created-at for time-ordered listing.
CREATE INDEX IF NOT EXISTS idx_pastes_created_at
    ON pastes (created_at DESC);
