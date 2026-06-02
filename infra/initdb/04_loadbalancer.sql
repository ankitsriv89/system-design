-- 04_loadbalancer.sql
-- Creates the database and user for project 06 (Load Balancer).
-- Runs automatically on postgres first start via docker-entrypoint-initdb.d.

CREATE USER lb WITH PASSWORD 'lb';
CREATE DATABASE loadbalancer OWNER lb;
\connect loadbalancer lb

CREATE TABLE IF NOT EXISTS backends (
    id          SERIAL PRIMARY KEY,
    service     TEXT NOT NULL,
    url         TEXT NOT NULL,
    weight      INTEGER NOT NULL DEFAULT 1,
    status      TEXT NOT NULL DEFAULT 'healthy',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(service, url)
);

CREATE TABLE IF NOT EXISTS health_events (
    id          BIGSERIAL PRIMARY KEY,
    service     TEXT NOT NULL,
    backend_url TEXT NOT NULL,
    status      TEXT NOT NULL,
    latency_ms  INTEGER,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_health_events_service_time
    ON health_events(service, recorded_at DESC);
