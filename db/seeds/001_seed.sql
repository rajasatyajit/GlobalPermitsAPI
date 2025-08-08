-- 001_seed.sql: Example seed data for GlobalPermits
-- Requires schema from 001_init.sql

BEGIN;

-- Enable pgcrypto for UUID generation if available
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Upsert a sample data source
INSERT INTO data_sources (id, slug, name, jurisdiction, config_hash, last_run_at, last_status)
VALUES (
    COALESCE((SELECT id FROM data_sources WHERE slug='sample-city'), gen_random_uuid()),
    'sample-city',
    'Sample City DOT',
    'Sample City, SC',
    'seed', now(), 'ok'
)
ON CONFLICT (slug) DO UPDATE SET last_run_at=EXCLUDED.last_run_at, last_status=EXCLUDED.last_status
RETURNING id INTO STRICT _seed_source_id;

-- If the above RETURNING INTO is not supported, fallback to select
-- Note: psql won't support INTO here; we'll derive the source id explicitly
-- Fallback derive source id
WITH s AS (
  SELECT id FROM data_sources WHERE slug='sample-city'
)
INSERT INTO permits (
  id, source_id, source_url, permit_type, title, description, issuing_entity,
  start_date, end_date, location_str, location_geom, geocode_confidence,
  status, artifact_key, hash
)
VALUES
  (
    gen_random_uuid(),
    (SELECT id FROM data_sources WHERE slug='sample-city'),
    'https://sample.gov/permits/road-closure-123',
    'ROAD_CLOSURE',
    'Road work on Main St',
    'Repaving Main St between 1st and 3rd Ave',
    'Sample City DOT',
    CURRENT_DATE + INTERVAL '1 day',
    CURRENT_DATE + INTERVAL '7 days',
    'Main St between 1st and 3rd Ave, Sample City',
    ST_SetSRID(ST_Point(-122.400, 37.790), 4326),
    0.92,
    'active',
    'raw/seed/sample-city/road-closure-123.html',
    'sha256:seed-123'
  ),
  (
    gen_random_uuid(),
    (SELECT id FROM data_sources WHERE slug='sample-city'),
    'https://sample.gov/permits/event-456',
    'EVENT',
    'City parade downtown',
    'Annual parade causing detours',
    'Sample City Events',
    CURRENT_DATE + INTERVAL '10 days',
    CURRENT_DATE + INTERVAL '10 days',
    'Downtown, Sample City',
    ST_SetSRID(ST_Point(-122.405, 37.785), 4326),
    0.85,
    'pending',
    'raw/seed/sample-city/event-456.pdf',
    'sha256:seed-456'
  );

COMMIT;

