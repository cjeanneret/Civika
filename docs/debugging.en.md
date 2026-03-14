# Debugging

## Basic checks
- API:
  - `GET /health`
  - `GET /info`
- Frontend:
  - verify `http://localhost:3000`
- Global smoke:
  - `make stack-smoke`

## Logs and debug mode
- Backend supports optional NDJSON debug logging (disabled by default).
- Variables:
  - `DEBUG_LOG_ENABLED` (`true|false`, default `false`)
  - `DEBUG_LOG_PATH` (default `/tmp/debug-2055fd.log`)

## Full test infrastructure
- Start: `make test-infra-up`
- Logs: `make test-infra-logs`
- Stop: `make test-infra-down`
- Purge volumes: `make test-infra-purge`

## Typical issues
- DB connection error: check `postgres` service and `POSTGRES_*`.
- Empty RAG response: run `make rag-index`, then `make rag-query Q="..."`.
- `llm` mode failure: validate `LLM_*` and `LLM_EMBEDDING_*`; no automatic fallback.
