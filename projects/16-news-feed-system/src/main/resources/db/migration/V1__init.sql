-- News feed system: durable source of truth for posts and the social graph.
-- Redis holds materialized home timelines; this schema holds what must survive
-- a restart (the "persist enough state to recover from restarts" requirement).

CREATE TABLE posts (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    author_id  VARCHAR(128) NOT NULL,
    body       VARCHAR(1000) NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL,
    deleted    BOOLEAN      NOT NULL DEFAULT FALSE
);

-- Read-path pulls (celebrity fanout-on-read, backfill) query recent posts by a
-- set of authors, newest first. This composite index serves that access pattern.
CREATE INDEX idx_posts_author_created ON posts (author_id, created_at DESC);

CREATE TABLE follows (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    follower_id VARCHAR(128) NOT NULL,
    followee_id VARCHAR(128) NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL,
    CONSTRAINT uq_follow UNIQUE (follower_id, followee_id)
);

-- Fanout-on-write needs "who follows author X" (by followee_id); the read path
-- and backfill need "who does X follow" (by follower_id). Index both directions.
CREATE INDEX idx_follows_followee ON follows (followee_id);
CREATE INDEX idx_follows_follower ON follows (follower_id);
