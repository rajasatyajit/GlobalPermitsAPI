#!/usr/bin/env bash
set -euo pipefail

# Requires: GitHub CLI (gh) authenticated to the repository.
# Edit the placeholders for assignees, labels, and team mentions as needed.

if ! command -v gh >/dev/null 2>&1; then
  echo "GitHub CLI (gh) not found. Install and authenticate first: https://cli.github.com/" >&2
  exit 1
fi

create_issue() {
  local title="$1"; shift
  local body="$1"; shift
  local labels="$1"; shift
  gh issue create --title "$title" --body "$body" --label "$labels"
}

create_issue "DB Migrations for Admin Permits" \
"Design and create migrations for admin permits tables and relations. Include indexes, constraints, seed data, and rollback tested." \
"backend,db,migration"

create_issue "Repository Layer for Admin Permits" \
"Implement repository interfaces/adapters for admin permits CRUD and queries. Include unit tests and transaction handling." \
"backend,repository"

create_issue "AuthZ/AuthN Middleware for Admin Endpoints" \
"Implement middleware to authenticate admin users and enforce RBAC for permits operations. Include tests for unauthorized/forbidden cases." \
"backend,security,middleware"

create_issue "HTTP Handler for Admin Permits" \
"Implement REST endpoints for admin permits (list, get, create, update, revoke). Validate payloads and provide consistent errors." \
"backend,handler,api"

create_issue "Cache Invalidation for Permits Changes" \
"Implement cache layer and invalidation strategy on create/update/revoke. Consider per-tenant scoping." \
"backend,cache"

create_issue "OpenAPI Spec for Admin Permits" \
"Define/extend OpenAPI with admin permits endpoints, schemas, errors, and security. Validate spec." \
"api,docs,openapi"

create_issue "Serve Swagger UI" \
"Wire up Swagger UI (or Redoc) behind authenticated route or only in non-prod. Ensure it loads the OpenAPI JSON correctly." \
"api,docs,tooling"

create_issue "Tests - Unit and Integration (Admin Permits)" \
"Unit tests for repository, middleware, handlers; integration tests with ephemeral DB and seed data." \
"test,backend,integration"

create_issue "Docker Compose Update" \
"Update docker-compose with new services/env vars for admin permits. Add Makefile targets for local dev." \
"devops,docker"

create_issue "Helm Chart Update" \
"Update Helm chart values/templates for new env vars, ports, and config. Document rollout strategy." \
"devops,k8s,helm"

create_issue "CI Pipeline Additions" \
"Add checks for migration validation, OpenAPI validation, and security linters. Ensure tests/build/image publish on PRs." \
"ci,devops"

create_issue "Documentation (Admin Permits)" \
"Update README and docs with endpoint usage, auth requirements, examples, and runbooks for permits operations." \
"docs"

