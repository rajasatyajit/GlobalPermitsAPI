package generator

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// Generate creates the architecture blueprint by injecting the raw product document
// into a predefined template structure.
func Generate(productDoc string, tpl string) (string, error) {
	productDoc = strings.TrimSpace(productDoc)
	data := map[string]any{
		"ProductDoc": productDoc,
	}
	t, err := template.New("arch").Funcs(template.FuncMap{
		"code": func(s string) string { return fmt.Sprintf("`%s`", s) },
	}).Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// DefaultTemplate returns a comprehensive architecture template covering all required sections.
func DefaultTemplate() string {
	return `# GlobalPermits API — Technical Architecture Blueprint

## High-Level System Overview
Summary: Briefly describe the overall architecture and its primary function.

- Product context: GlobalPermits API aggregates worldwide right-of-way and public-space permits into a single normalized API for B2B customers.
- Overall architecture: event-driven pipeline with decoupled ingestion, processing/enrichment, geospatial storage, and a low-latency serving tier.

## C4 Model — Container Diagram (Level 2)
Describe each container and interaction succinctly. Example interactions are given; refine to match the product document you provided.

- Scraper Fleet (container): Headless browser and HTTP scrapers per jurisdiction. Responsibilities: fetch HTML/PDF/JSON/CSV. Emits messages to Message Queue via AMQP/HTTPS.
- Ingestion Gateway (container): Receives pushed payloads or scraper results; persists raw artifacts to Object Storage; enqueues references to Message Queue.
- Message Queue (container): Durable buffering to decouple ingestion from processing.
- Processing Workers (container): Stateless consumers performing parsing, NER, geocoding, normalization, validation. Writes to PostgreSQL/PostGIS.
- Object Storage (container): Stores raw files (HTML, PDFs) for audit/reprocessing.
- Postgres/PostGIS (container): Primary OLTP store for normalized permits with geospatial indexes.
- API Service (container): Go HTTP service executing parameterized spatial/temporal queries.
- API Gateway (container): AuthN/Z (API keys), rate limiting, metrics, routing.
- Cache (container): Redis for hot query caching and request coalescing.
- Admin Console (container): Human-in-the-loop review UI, source configs, pipeline observability.

Interactions:
- Scraper Fleet -> Ingestion Gateway: HTTPS (signed), uploads raw artifacts to Object Storage.
- Ingestion Gateway -> Message Queue: publish JSON refs via AMQP/HTTPS.
- Processing Workers -> Object Storage: GET raw artifact by key.
- Processing Workers -> Postgres: SQL writes; upserts normalized permits.
- API Service -> Postgres: read-only SQL with spatial predicates.
- API Service -> Redis: read-through/write-through cache.
- Clients -> API Gateway -> API Service: HTTPS + API key, tiered limits.

## Data Ingestion Layer

### Scraper Fleet Management
- Framework Choice: Prefer Playwright (Python/Node) for robustness against dynamic sites; fallback to Scrapy for lightweight HTTP sites. Use Dockerized runners.
- Configuration-as-Code: One YAML per source containing: base_urls, navigation scripts/locators, CSS/XPath selectors, pagination rules, auth, expected fields, update cadence, change-detection hints.
- Resilience:
  - Proxy rotation (residential/datacenter pools) and geo-targeting.
  - CAPTCHA handling via Playwright stealth + third-party solve service with budget guardrails.
  - Layout change detection using DOM diffs and selector health metrics. Auto-disable sick scrapers and alert.
  - Backoff retries, jitter, circuit breakers; per-source concurrency limits.
- Deployment: Kubernetes Job/CronJob per source; image-per-framework; configuration mounted via ConfigMaps/Secrets.

### Ingestion Gateway & Queue
- Technology: RabbitMQ (self-managed) or AWS SQS (managed). Choose SQS for simplicity/scale; RabbitMQ for on-prem, needing routing keys and priority queues.
- Data Format (queue message):
  {
    "source_id": "nyc-dot",
    "source_url": "https://example.gov/permits/123",
    "artifact_key": "raw/2025/08/nyc-dot/123.pdf",
    "content_type": "application/pdf",
    "retrieval_ts": "2025-08-08T11:00:00Z",
    "hash": "sha256:...",
    "metadata": {"jurisdiction": "NYC", "lang": "en"}
  }
- Raw artifacts stored in Object Storage (S3/GCS/MinIO). Enqueue only references and minimal metadata.

## Data Processing & Enrichment Pipeline

### Step 1: Parsing & Raw Text Extraction
- HTML: BeautifulSoup/lxml; readability heuristics; preserve source section anchors.
- JSON/CSV: Schema inference with type hints; Pandas for tabular transform.
- PDF: PyMuPDF (fitz) primary; Tesseract OCR via OCRmyPDF fallback for scanned docs.
- Attach provenance (artifact_key, byte hash, parse duration) to downstream records.

### Step 2: NLP for Entity Extraction (NER)
- Model Choice: spaGO in Go for lightweight inference or ONNX Runtime with a distilled transformer model fine-tuned for domain entities.
- Training Strategy: Curate labeled dataset from historical permits; weak supervision via regex/heuristics to bootstrap; fine-tune for entities: permit_type, start_date, end_date, location_string, issuing_entity.
- Confidence & Fallback: Emit probabilities per entity; if any critical entity < threshold (e.g., 0.7), send to Review Queue and mark record as pending. Provide Admin Console UI for correction and active learning feedback loop.

### Step 3: Geocoding Enrichment
- Geocoding API: Primary (e.g., Mapbox/Google), fallback (Nominatim). Cache results in Postgres + Redis. Handle ambiguous results by storing multiple candidates with scores and choosing best within jurisdiction bounds.
- Error Handling: If geocode fails, store centroid of jurisdiction with low confidence and flag for review.

### Step 4: Normalization & Validation
- Controlled vocabulary mapping (e.g., ROAD_CLOSURE). Maintain mapping table editable via Admin Console. Use fuzzy synonyms and rules.
- Temporal validation: ensure end_date >= start_date; clamp absurd durations; normalize timezones to UTC.
- Spatial validation: ensure point/geometry within jurisdiction boundary; reproject to EPSG:4326.

## Data Storage Layer

### PostgreSQL/PostGIS Schema
- Table: data_sources
  - id (uuid, PK)
  - slug (text, unique)
  - name (text)
  - jurisdiction (text)
  - config_hash (text)
  - last_run_at (timestamptz)
  - last_status (text check in ('ok','warning','error'))
  - created_at (timestamptz, default now())

- Table: permits
  - id (uuid, PK)
  - source_id (uuid, FK -> data_sources.id)
  - source_url (text)
  - permit_type (text) -- normalized enum-like
  - title (text)
  - description (text)
  - issuing_entity (text)
  - start_date (date)
  - end_date (date)
  - location_str (text)
  - location_geom (geometry(Geometry, 4326)) -- point or polygon
  - geocode_confidence (numeric)
  - status (text check in ('pending','active','expired','review'))
  - artifact_key (text)
  - hash (text)
  - created_at (timestamptz default now())
  - updated_at (timestamptz default now())

- Table: processing_logs
  - id (bigserial, PK)
  - source_id (uuid, FK)
  - artifact_key (text)
  - step (text)
  - status (text check in ('ok','retry','failed'))
  - message (text)
  - duration_ms (int)
  - created_at (timestamptz default now())

- Indices:
  - permits: GIST index on location_geom; btree on start_date, end_date, permit_type, source_id.
  - processing_logs: btree on source_id, created_at.
  - data_sources: unique(slug).

## API & Serving Layer

### API Service (Go)
- Framework: Gin for performance and ecosystem. Middlewares: request logging, recovery, API key check (from gateway-forwarded header), cache middleware.
- Query translation:
  - location: accept bbox (minLon,minLat,maxLon,maxLat) or point+radius; convert to ST_MakeEnvelope or ST_DWithin.
  - date_range: WHERE daterange(start_date, end_date) && daterange($from,$to).
  - permit_type: WHERE permit_type = ANY($1) with array param.
  - Pagination: keyset pagination using created_at, id.

### API Gateway
- Use Kong or AWS API Gateway.
  - Auth: API key plugin; keys stored hashed. Forward key metadata to API via headers.
  - Rate limiting: tiered policies (Free: 60 req/min, 5k/mo; Startup: 600 req/min, 500k/mo; Pro: 3000 req/min, 5M/mo) with burst controls.
  - Observability: request/response logging to ELK/CloudWatch; tracing headers.
  - Routing: /v1/permits to API service; /v1/health public.

### Caching Strategy
- Redis: cache GET responses keyed by normalized query (bbox/params). TTL 30–300s. Use cache stampede prevention (single-flight).
- Invalidation: time-based + event hook from processing pipeline to invalidate by spatial bucket and date window when new permits arrive.

## Infrastructure, Operations, & CI/CD

### Containerization & Orchestration
- Docker images per component. Kubernetes:
  - Deployments: api-service, processor-workers, ingestion-gateway, admin-console, redis, rabbitmq (or use managed), postgres (statefulset), minio (if self-hosted object storage).
  - Scrapers: CronJobs per source; Jobs on-demand for reprocessing.
  - Horizontal Pod Autoscaler based on CPU, queue depth, or custom metrics.

### CI/CD Pipeline
- Steps: lint (golangci-lint), unit tests, build binaries, docker buildx with provenance, push to registry, helm chart package, deploy to K8s via ArgoCD or GitHub Actions environment.
- Secrets: use external secret manager (AWS Secrets Manager or K8s External Secrets).

### Monitoring & Alerting
- Stack: Prometheus + Grafana; Loki for logs; Tempo/Jaeger for tracing. Alternatively Datadog.
- Key metrics:
  - Scraper success rate, median duration, HTTP error distribution, CAPTCHA hit rate.
  - Queue depth, processing throughput, parse/NER/geocode latency.
  - API p50/p95 latency, error rate, cache hit ratio, DB CPU/IO, slow query count.
- Alerts:
  - >10% scraper failure for 15m.
  - Queue depth > threshold for 10m.
  - API p95 > 500ms for 10m or 5xx rate > 1%.
  - Geocoding error rate > 5%.

---

The following is the source product document you provided. Use it to refine and fill each section with product-specific details during editing and review:

---
{{.ProductDoc}}
