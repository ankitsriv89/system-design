-- Project 18: Instagram Photo/Video Sharing and Feed
-- Metadata lives in Postgres; media bytes live in object storage (MinIO/CDN).

-- ── Users ────────────────────────────────────────────────────────────────────
CREATE TABLE users (
    id          BIGSERIAL PRIMARY KEY,
    username    VARCHAR(64)  NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- ── Social graph (follower -> followee) ──────────────────────────────────────
CREATE TABLE follows (
    follower_id BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, followee_id)
);
CREATE INDEX idx_follows_followee ON follows(followee_id);

-- ── Media ────────────────────────────────────────────────────────────────────
-- status: PENDING -> PROCESSED (variants ready) | FAILED
CREATE TABLE media (
    id          BIGSERIAL PRIMARY KEY,
    owner_id    BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    object_key  VARCHAR(512) NOT NULL,
    content_type VARCHAR(128) NOT NULL,
    status      VARCHAR(16)  NOT NULL DEFAULT 'PENDING',
    variants    JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_media_owner ON media(owner_id);

-- ── Posts ────────────────────────────────────────────────────────────────────
CREATE TABLE posts (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_id    BIGINT       NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    caption     TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_posts_user_created ON posts(user_id, created_at DESC);

-- ── Engagement (likes) ───────────────────────────────────────────────────────
CREATE TABLE engagements (
    post_id     BIGINT       NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id     BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        VARCHAR(16)  NOT NULL DEFAULT 'LIKE',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, user_id, type)
);
CREATE INDEX idx_engagements_post ON engagements(post_id);
