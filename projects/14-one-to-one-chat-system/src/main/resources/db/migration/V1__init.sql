-- V1__init.sql — Schema for the 1:1 chat system.

CREATE TABLE IF NOT EXISTS conversations (
    id          BIGINT PRIMARY KEY,
    user_a      VARCHAR(64) NOT NULL,
    user_b      VARCHAR(64) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seq    BIGINT NOT NULL DEFAULT 0,
    CONSTRAINT uq_conversation UNIQUE (user_a, user_b),
    CONSTRAINT ck_conversation_order CHECK (user_a < user_b)
);

CREATE INDEX IF NOT EXISTS idx_conversations_user_a ON conversations(user_a);
CREATE INDEX IF NOT EXISTS idx_conversations_user_b ON conversations(user_b);

CREATE TABLE IF NOT EXISTS messages (
    id              BIGINT PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id),
    sender_id       VARCHAR(64) NOT NULL,
    body            TEXT NOT NULL,
    seq             BIGINT NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'SENT',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_status CHECK (status IN ('SENT','DELIVERED','READ'))
);

CREATE INDEX IF NOT EXISTS idx_messages_conv_seq ON messages(conversation_id, seq DESC);
