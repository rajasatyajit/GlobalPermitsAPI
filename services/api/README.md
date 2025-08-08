# API Service for GlobalPermits (scaffold)

This is a minimal Go Gin API that exposes:
- GET /v1/permits
- GET /v1/permits/:id
- GET /health

It uses pgxpool for PostgreSQL (with PostGIS) and optional Redis caching.

## Build & Run (without system Go)
If `go` is not installed locally, you can still use this code later; for now, treat it as a reference. To build when Go is available:

- go build -o bin/api ./services/api
- POSTGRES_DSN=postgres://user:pass@localhost:5432/dbname?sslmode=disable \
  REDIS_ADDR=localhost:6379 \
  API_ADDR=:8080 \
  ./bin/api

## Environment variables
- API_ADDR: Address to bind (default :8080)
- POSTGRES_DSN: PostgreSQL DSN (required)
- REDIS_ADDR: Redis address (optional; enable cache if set)
- API_KEY_HEADER: Header name forwarded by API gateway (default X-API-Key)

## Notes
- SQL filters use spatial operators: ST_MakeEnvelope and ST_DWithin
- Keyset pagination via created_at and id
- Caching layer is a simple read-through cache for GET /v1/permits

