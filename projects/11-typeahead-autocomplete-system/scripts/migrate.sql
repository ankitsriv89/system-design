-- Schema for the typeahead autocomplete service.

CREATE TABLE IF NOT EXISTS suggest_items (
    id          BIGSERIAL PRIMARY KEY,
    text        TEXT        NOT NULL,
    category    TEXT        NOT NULL DEFAULT 'general',
    popularity  FLOAT8      NOT NULL DEFAULT 0,
    locale      TEXT        NOT NULL DEFAULT 'en',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_suggest_items_locale_pop ON suggest_items (locale, popularity DESC);
CREATE INDEX IF NOT EXISTS idx_suggest_items_text_lower ON suggest_items (lower(text) text_pattern_ops);

CREATE TABLE IF NOT EXISTS query_logs (
    id               BIGSERIAL PRIMARY KEY,
    prefix           TEXT        NOT NULL,
    selected_item_id BIGINT,
    latency_ms       BIGINT      NOT NULL DEFAULT 0,
    locale           TEXT        NOT NULL DEFAULT 'en',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_query_logs_created ON query_logs (created_at DESC);
