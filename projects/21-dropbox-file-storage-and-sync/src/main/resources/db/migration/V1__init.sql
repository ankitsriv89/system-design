-- File node: represents files and folders in the metadata tree
CREATE TABLE file_node (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    VARCHAR(64) NOT NULL,
    parent_id   UUID REFERENCES file_node(id) ON DELETE CASCADE,
    name        VARCHAR(512) NOT NULL,
    type        VARCHAR(10) NOT NULL CHECK (type IN ('FILE', 'FOLDER')),
    size_bytes  BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_file_node_owner_parent ON file_node(owner_id, parent_id) WHERE deleted_at IS NULL;

-- File version: every upload creates a new version pointing to an object in S3
CREATE TABLE file_version (
    id          BIGSERIAL PRIMARY KEY,
    file_id     UUID NOT NULL REFERENCES file_node(id) ON DELETE CASCADE,
    version     INT NOT NULL DEFAULT 1,
    object_key  VARCHAR(1024) NOT NULL,
    checksum    VARCHAR(64) NOT NULL,
    size_bytes  BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (file_id, version)
);

CREATE INDEX idx_file_version_file_id ON file_version(file_id, version DESC);

-- Sync cursor: tracks which events each device has consumed
CREATE TABLE sync_cursor (
    device_id       VARCHAR(128) PRIMARY KEY,
    owner_id        VARCHAR(64) NOT NULL,
    last_event_id   BIGINT NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Change event log: append-only feed powering sync cursors
CREATE TABLE change_event (
    id          BIGSERIAL PRIMARY KEY,
    owner_id    VARCHAR(64) NOT NULL,
    file_id     UUID NOT NULL,
    event_type  VARCHAR(32) NOT NULL,
    payload     JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_change_event_owner_id ON change_event(owner_id, id);
