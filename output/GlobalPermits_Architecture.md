GlobalPermits API — Technical Architecture Blueprint

High-Level System Overview
Summary
GlobalPermits API aggregates global public-right-of-way and public-space permit information from numerous municipal and national sources into a normalized, geospatially searchable API for B2B applications (e.g., logistics, mobility, infrastructure planning). The system consists of:
- Scraper fleet for ingestion from heterogeneous sources (HTML, JSON, PDFs, CSV).
- Event-driven processing pipeline for parsing, NER, geocoding, normalization, and validation.
- Postgres/PostGIS for authoritative data storage optimized for spatial and temporal queries.
- A Go-based, low-latency API service fronted by an API gateway for auth, rate limiting, and tiering.
- Caching and observability layers to support performance and operations at scale.

C4 Model — Container Diagram (Level 2)
Containers and Responsibilities
- Scraper Fleet
  - Responsibilities: Fetch data from public websites and APIs; download HTML/PDF/CSV/JSON; detect updates.
  - Tech: Playwright (Python) for dynamic sites; Scrapy for lightweight HTTP; Dockerized runners.
  - Output: raw artifacts to Object Storage; metadata references to Message Queue.
- Ingestion Gateway
  - Responsibilities: Receive pushed payloads or scraper outputs; validate; store raw artifacts; enqueue jobs.
  - Tech: Go or Python FastAPI; integrates with S3/MinIO and SQS/RabbitMQ.
- Object Storage
  - Responsibilities: Durable storage of raw artifacts for audit and reprocessing.
  - Tech: AWS S3, GCS, or MinIO (on-prem).
- Message Queue
  - Responsibilities: Decouple ingestion from processing; backpressure; retries.
  - Tech: AWS SQS (managed, simple, scalable) or RabbitMQ (routing, on-prem).
- Processing Workers
  - Responsibilities: Parse/extract text; run NER; geocode; normalize/validate; write to DB; log processing status.
  - Tech: Go (spaGO) for lightweight inference, or Python with ONNX Runtime; concurrency via k8s Deployments.
- PostgreSQL/PostGIS
  - Responsibilities: Authoritative store for normalized permit records with spatial indexes.
- Cache
  - Responsibilities: Response caching for hot queries; geocoding cache; job deduplication.
  - Tech: Redis.
- API Service
  - Responsibilities: Serve REST/JSON with spatial/temporal filtering; enforce auth; integrate caching.
  - Tech: Go (Gin).
- API Gateway
  - Responsibilities: API key auth, rate limiting, analytics, routing, tiered usage plans.
  - Tech: Kong, Tyk, or AWS API Gateway.
- Admin Console
  - Responsibilities: Source configuration management, human-in-the-loop review, pipeline metrics, audit trail.
  - Tech: React UI + backend (Go or Node); internal-only.

Key Interactions/Protocols
- Scraper Fleet -> Ingestion Gateway: HTTPS uploads; signed requests.
- Ingestion Gateway -> Object Storage: S3 API (PUT).
- Ingestion Gateway -> Message Queue: SQS SendMessage or AMQP publish.
- Processing Workers -> Object Storage: S3 GET to retrieve artifacts.
- Processing Workers -> Postgres: SQL upserts, transactions.
- API Service -> Postgres: read-only SQL with spatial operators; connection pool management.
- API Service -> Redis: read-through caching; cache stampede prevention.
- Clients -> API Gateway -> API Service: HTTPS/JSON; API key headers; rate limits enforced at gateway.

Data Ingestion Layer
Scraper Fleet Management
- Framework Choice
  - Playwright (Python): Best for dynamic, JS-heavy sites; supports headless browsers, stealth, and automation robustness.
  - Scrapy: Efficient for static or API-based sources with minimal JS; strong crawling/pagination support.
  - Justification: Combining both accommodates a wide breadth of sources while optimizing cost/performance.
- Configuration-as-Code
  - One YAML per source (checked into Git):
    - Source metadata: id, name, jurisdiction, language, run cadence, dependencies.
    - Fetch config: base URLs, selectors/locators, pagination rules, rate limits, auth parameters.
    - Output mapping: target fields (title, description, dates, location, issuing_entity, attachments).
    - Change detection: content hash strategy; ETag/Last-Modified support; diff thresholds.
  - Example YAML keys: source_id, base_urls, playwright_flows, selectors, pagination, auth, expected_fields, schedule_cron, retries, backoff, artifact_types.
- Resilience
  - Proxy rotation & geo-targeting via provider pools; per-source IP pools; custom headers; randomized delays.
  - CAPTCHA mitigation: Playwright stealth plugins; external CAPTCHA solving with budget limits and alerting.
  - Anti-bot/ban handling: circuit breakers; gradual backoff; automated hold on high error rates.
  - Layout change detection: DOM health metrics; HTML structure diffing; auto-disable broken scrapers; alert engineers.
  - Robust retries, jitter, idempotency tokens; dead-letter queues for repeated failures.
- Deployment
  - Kubernetes CronJobs for scheduled runs; Jobs for ad-hoc reprocessing.
  - Per-framework container images; source config via ConfigMaps and Secrets (for credentials).
  - Observability: logs to Loki/ELK, metrics to Prometheus; tracing headers on gateway interactions.

Ingestion Gateway & Queue
- Message Queue Technology
  - AWS SQS: managed, high durability, dead-letter queues, easy autoscaling consumers.
  - RabbitMQ: richer routing (topics), priority queues; suitable for self-managed/on-prem.
  - Choice: Prefer SQS for cloud-first simplicity and scale; RabbitMQ for complex routing or on-prem constraints.
- Message Format (JSON)
  - source_id: string (e.g., “nyc-dot”)
  - source_url: string (canonical item URL)
  - artifact_key: string (S3 path: raw/yyyy/mm/source_id/uuid.ext)
  - content_type: string (text/html, application/pdf, text/csv, application/json)
  - retrieval_ts: ISO-8601 timestamp (UTC)
  - hash: sha256 hex of raw bytes
  - metadata: object (jurisdiction, language, auth_mode, etc.)
- Raw Data Handling
  - Artifacts stored in Object Storage (S3/MinIO). Queue contains references only (artifact_key + metadata).
  - Size guardrails: big artifacts never inline into messages.
  - Idempotency: artifact_key is deterministic; duplicate messages lead to safe upserts in processing.

Data Processing & Enrichment Pipeline
Step 1: Parsing & Raw Text Extraction
- HTML: BeautifulSoup/lxml; boilerplate removal; readability scoring; extract relevant sections and attachments.
- JSON: Schema inference, map fields to internal names; handle nested arrays; flatten where appropriate.
- CSV: pandas read_csv with types; column mapping via config; normalize encodings/timezones.
- PDF: PyMuPDF (fitz) for text extraction; OCR fallback via OCRmyPDF/Tesseract for scanned docs. Extract tables when present.
- Provenance: persist artifact_key, byte hash, parse_version, parser timing; attach warnings for low-quality text.

Step 2: NLP for Entity Extraction (NER)
- Model Choice
  - spaGO (Go-native): lightweight and deployable in the API or workers.
  - ONNX Runtime: Distilled transformer fine-tuned for domain entities with good performance/accuracy balance.
- Custom Model Strategy
  - Data: Build a labeled corpus from historical permits and curated samples across jurisdictions; bootstrap with regex-based weak labels.
  - Entities: permit_type, start_date, end_date, issuing_entity, location_string, jurisdiction.
  - Training: Fine-tune on domain text; apply data augmentation (date formats, location patterns).
  - Versioning: Model artifacts versioned in a registry; A/B testing and canary deployments.
- Confidence & Human-in-the-Loop
  - Emit per-entity probabilities; define critical thresholds (e.g., <0.7).
  - Low-confidence -> Review Queue; Admin Console UI renders context and suggested values.
  - Feedback loop: approved corrections stored as training data; periodic re-training.

Step 3: Geocoding Enrichment
- Primary provider: Google Geocoding or Mapbox; fallback to Nominatim.
- Strategy:
  - Normalize location_string (strip noise, expand abbreviations, add jurisdiction context).
  - Query primary; if ambiguous, request more context or query fallback.
  - Store multiple candidates with scores; pick best constrained by jurisdiction boundary.
  - Cache results in Redis and Postgres (materialized cache table) keyed by normalized string + jurisdiction.
- Error Handling:
  - If no match: geocode to jurisdiction centroid with low confidence; flag for review.
  - Strict quotas and exponential backoff; alert on elevated error rates.

Step 4: Normalization & Validation
- Controlled Vocabulary
  - Maintain mapping table for permit types: e.g., ROAD_CLOSURE, OCCUPANCY, EVENT, CONSTRUCTION.
  - Use synonyms and fuzzy matching to map variants (“Street Closure”, “Detour”) to canonical types.
- Temporal Validation
  - Ensure end_date >= start_date; clamp extreme durations; normalize to UTC.
  - Handle single-day or open-ended permits; coerce invalid dates to pending review.
- Spatial Validation
  - Validate geometries within jurisdiction polygons; resolve projections; fix invalid shapes.
  - If geocode_confidence < threshold, mark as review.
- Deduplication
  - Source-level checksum (source_url + extracted fields) to prevent duplicate records on re-ingest.
  - Upsert semantics keyed by source_id + source_url or artifact hash.

Data Storage Layer
PostgreSQL/PostGIS Schema
- data_sources
  - id: uuid PK
  - slug: text unique
  - name: text
  - jurisdiction: text
  - config_hash: text
  - last_run_at: timestamptz
  - last_status: text check in ('ok','warning','error')
  - created_at: timestamptz default now()
- permits
  - id: uuid PK
  - source_id: uuid FK -> data_sources.id
  - source_url: text
  - permit_type: text
  - title: text
  - description: text
  - issuing_entity: text
  - start_date: date
  - end_date: date
  - location_str: text
  - location_geom: geometry(Geometry, 4326)  // point/line/polygon
  - geocode_confidence: numeric
  - status: text check in ('pending','active','expired','review')
  - artifact_key: text
  - hash: text
  - created_at: timestamptz default now()
  - updated_at: timestamptz default now()
- processing_logs
  - id: bigserial PK
  - source_id: uuid FK
  - artifact_key: text
  - step: text
  - status: text check in ('ok','retry','failed')
  - message: text
  - duration_ms: int
  - created_at: timestamptz default now()
- vocab_permit_types
  - canonical_type: text PK
  - synonyms: text[]  // for admin-managed mappings
  - updated_at: timestamptz default now()

Indexing Strategy
- permits
  - GIST index on location_geom for spatial filters.
  - B-tree on start_date, end_date for date range filters.
  - B-tree on permit_type for categorical filters.
  - B-tree on source_id for source-specific queries.
  - Optional: BRIN on created_at for ingestion-time queries at scale.
- processing_logs
  - B-tree on source_id, created_at.
- data_sources
  - unique(slug).

API & Serving Layer
API Service (Go, Gin)
- Endpoints (v1)
  - GET /permits
    - Query params:
      - bbox=minLon,minLat,maxLon,maxLat or lat,lon,radius_m
      - permit_type=… (repeated or comma-separated)
      - start_date_from, start_date_to, end_date_from, end_date_to
      - status=active|expired|pending|review
      - limit (default 100, max 1000), cursor for keyset pagination
    - SQL translation:
      - Spatial: ST_Intersects(location_geom, ST_MakeEnvelope(..., 4326)) or ST_DWithin(geom, ST_MakePoint(lon,lat)::geography, radius_m)
      - Temporal: daterange(start_date, end_date) && daterange($from, $to)
      - Types: permit_type = ANY($types)
    - Caching: Normalize query to a cache key; check Redis before DB.
  - GET /permits/{id}
  - GET /health
- Middleware
  - Request logging, recovery, correlation IDs, API key validation (from gateway header), rate-limit header propagation.
- DB Access
  - Read-only role; prepared statements; statement timeouts; connection pooling (pgx).

API Gateway
- Function
  - Auth: API key validation; keys stored hashed; rotation support; usage analytics.
  - Rate limiting: per plan and per key; global and per-route limits.
  - Request logging: export to ELK/CloudWatch; audit trails.
  - Routing: map /v1/* to API service; canary routing for new versions.
- Tiered Access (usage plans)
  - Free: 60 req/min, 5,000 req/month; basic endpoints; lower priority.
  - Startup: 600 req/min, 500,000 req/month; broader endpoints; higher concurrency.
  - Pro: 3,000 req/min, 5,000,000 req/month; priority routing; dedicated support SLAs.
  - Overages: 429 Too Many Requests; optional paid burst pools.
- Key Management
  - Self-service portal for key issuance and rotation; webhook notifications for approaching limits.

Caching Strategy
- Redis cache GET /permits responses keyed by normalized query (sorted params, reduced precision bbox).
- TTL: 30–300s depending on endpoint and staleness tolerance.
- Cache stampede prevention: single-flight within API; request coalescing.
- Invalidation
  - Time-based TTL covers most cases.
  - Event-driven partial invalidation: processing workers publish “permit-changed” events with spatial cell and date range; API layer evicts matching keys (using spatial cell prefixing).

Infrastructure, Operations, & CI/CD
Containerization & Orchestration (Docker + Kubernetes)
- Separate Deployments
  - api-service (Go)
  - processor-workers (Go/Python mix)
  - ingestion-gateway (Go/Python)
  - admin-console (UI + backend)
  - redis (or managed)
  - rabbitmq (if not using SQS)
  - postgres (StatefulSet) + PostGIS
  - minio (if not using S3)
- Scrapers
  - CronJobs per source (schedule from source config).
  - Jobs for reprocessing and backfills.
- Autoscaling
  - HPA on cpu/memory/queue-depth/custom metrics (e.g., SQS ApproximateNumberOfMessages).
  - Worker concurrency controlled via env vars; safe shutdown with in-flight ack handling.
- Networking & Security
  - mTLS between internal services where appropriate; network policies for least privilege.
  - Secrets via Kubernetes Secrets or External Secrets Operator backed by cloud secret manager.
  - IAM roles/service accounts for cloud resources (S3, SQS, etc.).

CI/CD Pipeline
- Tooling: GitHub Actions or GitLab CI; ArgoCD or Flux for GitOps deploy.
- Pipeline Stages
  - Lint: golangci-lint; Python linters (ruff, black).
  - Test: unit, integration (DB containers), contract tests for API.
  - Build: Go binaries; Docker multi-arch images; SBOM and signatures (cosign).
  - Scan: Trivy or Grype for image scanning.
  - Deploy: Helm chart render; apply to staging; smoke tests; canary; promote to prod.
- Migrations
  - Managed via goose or golang-migrate; run as pre-deploy job with rollback on failure.

Monitoring & Alerting
- Stack
  - Metrics: Prometheus + Grafana dashboards (or Datadog).
  - Logs: Loki or ELK; structured JSON logs.
  - Tracing: OpenTelemetry -> Tempo/Jaeger.
- Key Metrics
  - Scrapers: success rate, HTTP status distribution, time per run, CAPTCHA rate, content-change rate.
  - Queue: depth, oldest message age, consumer lag, DLQ rate.
  - Pipeline: parse latency, NER confidence distributions, geocode success, normalization/validation failure counts.
  - API: p50/p95/p99 latency, 5xx rate, throughput, cache hit ratio, top routes, DB query timings.
  - DB: CPU/IO, active connections, slow queries, autovacuum activity, index bloat, replication lag.
- Alerts (examples)
  - >10% scraper failure for 15m; >5% CAPTCHA solve failure.
  - Queue depth above threshold for 10m or DLQ > N/min.
  - API p95 > 500ms for 10m; 5xx rate > 1% for 5m; cache hit ratio < 60% sustained.
  - Geocoding error rate > 5% or provider quota exhaustion.
  - DB CPU > 80% for 10m; replication lag > 60s; deadlock spike.

Security, Compliance, and Governance
- Data Governance: Track provenance via artifact_key and processing_logs; immutable raw artifacts retained for audit.
- PII: Expect minimal PII; if present, mask in logs; encrypt at rest (S3 SSE, Postgres TDE or disk encryption).
- Access Controls: RBAC in k8s; least-privilege IAM; read-only DB roles for API.
- Compliance: Logging and retention policies; change management for scraper configs; incident response runbooks.

Cost & Performance Considerations
- Scraper efficiency: Prefer Scrapy where possible; batch Playwright sessions; reuse browser contexts; terminate on idle.
- Queue-based backpressure: scale workers based on queue depth; cap geocoding QPS.
- Caching: High hit ratio on common city/permit queries; prewarm for big customers.
- Storage: Move cold artifacts to cheaper storage tier; periodic DB vacuum and partitioning by month for large tables.

Rollout & Migration
- Phased onboarding of jurisdictions; per-source toggles.
- Feature flags for NER model versions; fallback to rules if model confidence poor.
- Blue/green or canary API deployments via gateway routing.

Appendix: Example SQL Snippets
- Spatial bbox filter:
  WHERE ST_Intersects(location_geom, ST_MakeEnvelope($1, $2, $3, $4, 4326))
- Temporal overlap:
  WHERE daterange(start_date, end_date, '[]') && daterange($from::date, $to::date, '[]')
- Point+radius:
  WHERE ST_DWithin(location_geom::geography, ST_SetSRID(ST_Point($lon, $lat), 4326)::geography, $radius_m)

