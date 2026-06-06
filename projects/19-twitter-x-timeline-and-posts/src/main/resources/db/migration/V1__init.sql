-- Project 19: Twitter/X Timeline and Posts
-- PostgreSQL is the durable source of truth for tweets and the social graph.
-- Redis holds materialized home timelines; OpenSearch holds the search index
-- and trend aggregation — both are eventually-consistent projections built off
-- the tweet.created event stream.

-- ── Social graph (follower -> followee) ──────────────────────────────────────
-- user ids are free-form strings minted by the demo auth endpoint; there is no
-- separate users table in the MVP (mirrors the news-feed project's model).
CREATE TABLE follows (
    id           BIGSERIAL    PRIMARY KEY,
    follower_id  VARCHAR(128) NOT NULL,
    followee_id  VARCHAR(128) NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_follow UNIQUE (follower_id, followee_id)
);
-- "who follows X" — the fanout-on-write target lookup.
CREATE INDEX idx_follows_followee ON follows(followee_id);
-- "who does X follow" — the read-path pull + backfill lookup.
CREATE INDEX idx_follows_follower ON follows(follower_id);

-- ── Tweets ───────────────────────────────────────────────────────────────────
-- A tweet is committed here transactionally before any tweet.created event is
-- published, so we never fan out / index a tweet that wasn't durably stored.
-- Soft delete: the row stays for audit; the read path filters it out.
CREATE TABLE tweets (
    id          BIGSERIAL    PRIMARY KEY,
    author_id   VARCHAR(128) NOT NULL,
    text        VARCHAR(280) NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted     BOOLEAN      NOT NULL DEFAULT false
);
-- Pull a single author's recent tweets newest-first (read-path + user timeline).
CREATE INDEX idx_tweets_author_created ON tweets(author_id, created_at DESC);
