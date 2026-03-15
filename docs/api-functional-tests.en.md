# API Functional Tests

## Goal
This document describes the API functional test suite that validates:
- OpenAPI contract conformance,
- JSON response stability,
- strict handling of invalid inputs.

The goal is to provide reproducible CI evidence that the API is validated.

## Tooling
- **OpenAPI 3**: source-of-truth contract (`docs/openapi/civika-api-v1.yaml`).
- **Schemathesis**: property-based test generation from the contract.
- **Targeted Python checks**: critical scenarios not reliably covered by fuzzing alone.

## Covered scope
- Core endpoints: `/`, `/health`, `/info`.
- V1 business endpoints: votations, objects, taxonomies, metrics.
- QA endpoint:
  - validated via targeted invalid-input checks,
  - excluded from contract fuzzing to reduce CI noise.

## Test constraints
- **Privacy-first**: no personal data in test payloads.
- **Security-first**: JSON error format and security headers are checked.
- **CI speed**: RAG preparation uses **one single fixture**:
  - `tests/fixtures/normalized/openparldata/01-voting-101357.json`

## Local run
1. Start test PostgreSQL and initialize schema:
   - `docker compose --env-file .env.test -f docker-compose.test.yml up -d postgres`
   - `ENV_FILE=.env.test COMPOSE_FILE=docker-compose.test.yml make init-db`
2. Prepare a minimal corpus:
   - `mkdir -p data/normalized/openparldata`
   - `cp tests/fixtures/normalized/openparldata/01-voting-101357.json data/normalized/openparldata/`
3. Index in local mode:
   - `RAG_MODE=local LLM_ENABLED=false LLM_EMBEDDING_ENABLED=false POSTGRES_HOST=127.0.0.1 POSTGRES_PORT=5432 POSTGRES_USER=postgres POSTGRES_PASSWORD=civika-test-pass POSTGRES_DB=civika_test POSTGRES_SSLMODE=disable RAG_EMBEDDING_DIMENSIONS=768 make rag-index CORPUS=data/normalized/openparldata`
4. Start API locally (local RAG mode), then run:
   - `API_BASE_URL=http://127.0.0.1:8080 make test-api-contract`

## GitHub Actions integration
Workflow `.github/workflows/api-contract-tests.yml`:
- starts PostgreSQL,
- initializes schema,
- indexes a minimal corpus (1 fixture),
- starts API in `local` mode,
- runs:
  - Schemathesis contract tests,
  - targeted Python checks.

The job fails on:
- OpenAPI contract violations,
- unexpected HTTP statuses,
- input validation regressions.
