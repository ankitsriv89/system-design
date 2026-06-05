-- Enable PostGIS extension
CREATE EXTENSION IF NOT EXISTS postgis;

-- User locations table (durable, audit, history)
CREATE TABLE IF NOT EXISTS locations (
    id          BIGSERIAL PRIMARY KEY,
    user_id     VARCHAR(128) NOT NULL,
    lat         DOUBLE PRECISION NOT NULL,
    lng         DOUBLE PRECISION NOT NULL,
    geohash     VARCHAR(12) NOT NULL,
    geom        GEOMETRY(Point, 4326),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_locations_user_id ON locations(user_id);
CREATE INDEX IF NOT EXISTS idx_locations_geohash ON locations(geohash);
CREATE INDEX IF NOT EXISTS idx_locations_geom ON locations USING GIST(geom);
CREATE INDEX IF NOT EXISTS idx_locations_updated_at ON locations(updated_at);

-- Visibility / privacy settings
CREATE TABLE IF NOT EXISTS visibility_settings (
    user_id    VARCHAR(128) PRIMARY KEY,
    mode       VARCHAR(32) NOT NULL DEFAULT 'friends',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Nearby query log (analytics)
CREATE TABLE IF NOT EXISTS nearby_queries (
    id           BIGSERIAL PRIMARY KEY,
    querier_id   VARCHAR(128) NOT NULL,
    lat          DOUBLE PRECISION NOT NULL,
    lng          DOUBLE PRECISION NOT NULL,
    radius_m     INT NOT NULL,
    result_count INT NOT NULL DEFAULT 0,
    queried_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_nearby_queries_querier ON nearby_queries(querier_id);
CREATE INDEX IF NOT EXISTS idx_nearby_queries_time ON nearby_queries(queried_at);
