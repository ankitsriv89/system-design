-- V1__init.sql — Schema for the group chat system.

CREATE TABLE IF NOT EXISTS groups (
    id          BIGINT PRIMARY KEY,
    name        VARCHAR(128) NOT NULL,
    owner_id    VARCHAR(64) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS group_members (
    group_id    BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id     VARCHAR(64) NOT NULL,
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    role        VARCHAR(16) NOT NULL DEFAULT 'MEMBER',
    PRIMARY KEY (group_id, user_id),
    CONSTRAINT ck_role CHECK (role IN ('OWNER','ADMIN','MEMBER'))
);

CREATE INDEX IF NOT EXISTS idx_group_members_user ON group_members(user_id);

CREATE TABLE IF NOT EXISTS messages (
    id          BIGINT PRIMARY KEY,
    group_id    BIGINT NOT NULL REFERENCES groups(id),
    sender_id   VARCHAR(64) NOT NULL,
    body        TEXT NOT NULL,
    seq         BIGINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_messages_group_seq ON messages(group_id, seq DESC);
