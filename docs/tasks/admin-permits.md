# Admin Permits Endpoint and Docs - Task Breakdown

This document tracks the work for the feature/admin-permits-endpoint-and-docs initiative.

Tracker note: If you use GitHub Issues, consider running the helper script in scripts/gh_create_admin_permits_issues.sh after configuring gh auth and editing placeholders. If another tracker is used (Jira/Linear/etc.), port these titles/descriptions accordingly.

Epic: Admin Permits Endpoint and Documentation

Tasks

- Title: DB Migrations for Admin Permits
  Labels: backend, db, migration
  Description:
  - Design and create migrations for admin permits tables and relations.
  - Include indexes, constraints, and seed data if applicable.
  - Rollback script included and tested.
  Acceptance:
  - Migration up/down works locally and in CI.

- Title: Repository Layer for Admin Permits
  Labels: backend, repository
  Description:
  - Implement repository interfaces and adapters for admin permits CRUD and queries.
  - Cover concurrency-safe operations and transactions.
  Acceptance:
  - Unit tests cover happy paths and edge cases.

- Title: AuthZ/AuthN Middleware for Admin Endpoints
  Labels: backend, security, middleware
  Description:
  - Implement middleware to authenticate admin users and enforce RBAC for permits operations.
  - Support token-based auth (JWT/OIDC) and service-to-service auth (if applicable).
  Acceptance:
  - Unauthorized/forbidden cases handled with correct responses. Tests included.

- Title: HTTP Handler for Admin Permits
  Labels: backend, handler, api
  Description:
  - Implement REST endpoints for admin permits (list, get, create, update, revoke). Validate payloads.
  - Return consistent error shapes. Pagination for list endpoints.
  Acceptance:
  - OpenAPI aligns with handler behavior. Unit tests pass.

- Title: Cache Invalidation for Permits Changes
  Labels: backend, cache
  Description:
  - Implement cache layer and invalidation strategy on create/update/revoke.
  - Consider per-tenant/per-admin scoping.
  Acceptance:
  - Stale reads minimized and measured in tests.

- Title: OpenAPI Spec for Admin Permits
  Labels: api, docs, openapi
  Description:
  - Define/extend OpenAPI with admin permits endpoints, schemas, errors, and security.
  - Include examples and enums.
  Acceptance:
  - Spec validates with openapi linter.

- Title: Serve Swagger UI
  Labels: api, docs, tooling
  Description:
  - Wire up Swagger UI (or Redoc) behind an authenticated route or in non-prod only.
  Acceptance:
  - UI renders and fetches OpenAPI JSON correctly.

- Title: Tests - Unit and Integration (Admin Permits)
  Labels: test, backend, integration
  Description:
  - Unit tests for repository, middleware, handlers.
  - Integration tests using ephemeral DB and seed data. Consider http tests.
  Acceptance:
  - Coverage thresholds met in CI. Flakes minimized.

- Title: Docker Compose Update
  Labels: devops, docker
  Description:
  - Update docker-compose to include any new services or env vars for admin permits.
  - Provide make targets for bringing up stack locally.
  Acceptance:
  - Local env starts and endpoints work.

- Title: Helm Chart Update
  Labels: devops, k8s, helm
  Description:
  - Update Helm chart values/templates for new env vars, ports, and config.
  - Document rollout strategy.
  Acceptance:
  - Helm template lints and installs to a test namespace.

- Title: CI Pipeline Additions
  Labels: ci, devops
  Description:
  - Add checks for migration validation, openapi validation, security linters.
  - Ensure tests, build, and image publish steps run on PRs to feature branch.
  Acceptance:
  - CI is green with new gates.

- Title: Documentation (Admin Permits)
  Labels: docs
  Description:
  - Update README and docs site with endpoint usage, auth requirements, and examples.
  - Add runbooks for on-call around permits operations.
  Acceptance:
  - Docs reviewed and published.

Security Reviews and Ownership

- Areas in scope:
  - Auth middleware
  - Secret handling (env vars, config, KMS/injectors)

- Code owners and reviewers (placeholders):
  - Owners: @org/security-team @org/platform-team
  - Reviewers: @org/appsec @org/sre-team

Replace placeholders with the actual GitHub org/team handles or your tracker-specific identities.

