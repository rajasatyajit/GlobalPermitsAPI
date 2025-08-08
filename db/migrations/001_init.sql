-- 001_init.sql: PostgreSQL/PostGIS schema for GlobalPermits
-- Note: Ensure the PostGIS extension is installed and enabled.

BEGIN;

CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS data_sources (
    id UUID PRIMARY KEY,
    slug TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    jurisdiction TEXT,
    config_hash TEXT,
    last_run_at TIMESTAMPTZ,
    last_status TEXT CHECK (last_status IN ('ok','warning','error')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS permits (
    id UUID PRIMARY KEY,
    source_id UUID NOT NULL REFERENCES data_sources(id) ON DELETE RESTRICT,
    source_url TEXT,
    permit_type TEXT NOT NULL,
    title TEXT,
    description TEXT,
    issuing_entity TEXT,
    start_date DATE,
    end_date DATE,
    location_str TEXT,
    location_geom geometry(Geometry, 4326),
    geocode_confidence NUMERIC,
    status TEXT CHECK (status IN ('pending','active','expired','review')),
    artifact_key TEXT,
    hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_permits_geom ON permits USING GIST (location_geom);
CREATE INDEX IF NOT EXISTS idx_permits_start_date ON permits (start_date);
CREATE INDEX IF NOT EXISTS idx_permits_end_date ON permits (end_date);
CREATE INDEX IF NOT EXISTS idx_permits_type ON permits (permit_type);
CREATE INDEX IF NOT EXISTS idx_permits_source_id ON permits (source_id);

CREATE TABLE IF NOT EXISTS processing_logs (
    id BIGSERIAL PRIMARY KEY,
    source_id UUID REFERENCES data_sources(id) ON DELETE SET NULL,
    artifact_key TEXT,
    step TEXT,
    status TEXT CHECK (status IN ('ok','retry','failed')),
    message TEXT,
    duration_ms INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Optional: controlled vocabulary table
CREATE TABLE IF NOT EXISTS vocab_permit_types (
    canonical_type TEXT PRIMARY KEY,
    synonyms TEXT[],
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMIT;

