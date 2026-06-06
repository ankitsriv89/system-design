-- Project 20: WhatsApp Real-Time Messaging — initial schema.
-- The server stores ciphertext and delivery metadata only; plaintext never
-- reaches Postgres (E2EE boundary lives in the client). See docs/architecture.md.

-- A user account. Auth is via JWT; a user owns one or more devices.
CREATE TABLE app_user (
    id          BIGSERIAL PRIMARY KEY,
    username    TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A device (a browser tab acts as a device in the web demo). Holds the public
-- key other devices use to encrypt to it.
CREATE TABLE device (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    public_key  TEXT NOT NULL,
    label       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen   TIMESTAMPTZ
);
CREATE INDEX idx_device_user ON device(user_id);

-- A message. Partition-friendly on chat_id for high-volume history.
-- ciphertext is opaque to the server.
CREATE TABLE message (
    id          BIGSERIAL PRIMARY KEY,
    chat_id     TEXT NOT NULL,
    sender_id   BIGINT NOT NULL REFERENCES app_user(id),
    ciphertext  BYTEA NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_message_chat_time ON message(chat_id, created_at);

-- Per-device delivery state for a message (sent → delivered → read).
CREATE TABLE receipt (
    message_id  BIGINT NOT NULL REFERENCES message(id) ON DELETE CASCADE,
    device_id   BIGINT NOT NULL REFERENCES device(id) ON DELETE CASCADE,
    state       TEXT NOT NULL DEFAULT 'sent',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, device_id)
);
CREATE INDEX idx_receipt_device ON receipt(device_id);
