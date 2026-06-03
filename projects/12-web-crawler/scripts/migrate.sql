-- Web crawler schema migration

CREATE TABLE IF NOT EXISTS crawl_jobs (
    id         BIGSERIAL PRIMARY KEY,
    seed_url   TEXT NOT NULL,
    max_depth  INT  NOT NULL DEFAULT 2,
    status     TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS url_frontier (
    id           BIGSERIAL PRIMARY KEY,
    url          TEXT NOT NULL UNIQUE,
    host         TEXT NOT NULL,
    priority     INT  NOT NULL DEFAULT 1,
    depth        INT  NOT NULL DEFAULT 0,
    status       TEXT NOT NULL DEFAULT 'pending',
    next_fetch_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_frontier_status_fetch
    ON url_frontier (status, next_fetch_at, priority DESC)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_frontier_host ON url_frontier (host);

CREATE TABLE IF NOT EXISTS page_fetches (
    url_hash     CHAR(64)    PRIMARY KEY,
    url          TEXT        NOT NULL,
    status_code  INT         NOT NULL DEFAULT 0,
    content_hash CHAR(64)    NOT NULL DEFAULT '',
    body_size    INT         NOT NULL DEFAULT 0,
    fetched_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    error        TEXT
);

CREATE INDEX IF NOT EXISTS idx_page_fetches_fetched_at ON page_fetches (fetched_at DESC);

CREATE TABLE IF NOT EXISTS robots_rules (
    host        TEXT PRIMARY KEY,
    disallowed  TEXT[] NOT NULL DEFAULT '{}',
    crawl_delay INT   NOT NULL DEFAULT 0,
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
