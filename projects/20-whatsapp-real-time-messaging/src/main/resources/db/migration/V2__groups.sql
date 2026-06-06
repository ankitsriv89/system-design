-- Project 20: group messaging schema.

CREATE TABLE chat_group (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    owner_id   BIGINT NOT NULL REFERENCES app_user(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE group_member (
    id        BIGSERIAL PRIMARY KEY,
    group_id  BIGINT NOT NULL REFERENCES chat_group(id) ON DELETE CASCADE,
    user_id   BIGINT NOT NULL REFERENCES app_user(id)  ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (group_id, user_id)
);
CREATE INDEX idx_group_member_user ON group_member(user_id);
