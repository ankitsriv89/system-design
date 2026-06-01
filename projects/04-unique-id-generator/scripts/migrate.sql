-- Migration: unique-id-generator schema
-- Run once against the `uniqueid` database after creating it.
--
-- Tables:
--   worker_leases   — 1024 pre-seeded rows (one per valid worker_id).
--                     Each row is claimed by exactly one running instance.
--   clock_incidents — append-only audit log of backward clock drift events.

-- worker_leases holds one row per possible worker_id (0–1023).
-- Rows are pre-seeded so claim logic is always an UPDATE, never an INSERT —
-- this avoids gaps and makes the SELECT FOR UPDATE SKIP LOCKED pattern reliable.
CREATE TABLE IF NOT EXISTS worker_leases (
    worker_id  SMALLINT     PRIMARY KEY CHECK (worker_id >= 0 AND worker_id <= 1023),
    holder     TEXT         NOT NULL DEFAULT '',   -- empty string means unclaimed
    region     TEXT         NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ  NOT NULL DEFAULT NOW() - INTERVAL '1 second'
);

-- Pre-seed all 1024 worker_id slots as unclaimed.
-- INSERT ... ON CONFLICT DO NOTHING is idempotent so this migration is safe to re-run.
INSERT INTO worker_leases (worker_id)
SELECT generate_series(0, 1023)
ON CONFLICT DO NOTHING;

-- Index used by the lease-claim query: find the lowest unclaimed or expired slot quickly.
CREATE INDEX IF NOT EXISTS idx_worker_leases_available
    ON worker_leases (worker_id)
    WHERE expires_at < NOW() OR holder = '';

-- clock_incidents is an append-only audit log.
-- The generator writes a row here whenever it detects the wall clock moving backward.
-- A rising count should trigger a PagerDuty / Alertmanager alert.
CREATE TABLE IF NOT EXISTS clock_incidents (
    id          BIGSERIAL    PRIMARY KEY,
    worker_id   SMALLINT     NOT NULL REFERENCES worker_leases (worker_id),
    drift_ms    BIGINT       NOT NULL,   -- magnitude of the backward drift in ms
    detected_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_clock_incidents_worker
    ON clock_incidents (worker_id, detected_at DESC);
