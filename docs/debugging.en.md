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

## AI metrics inspection
- Check recent events:
  - `SELECT event_id, created_at, flow, operation, mode, total_tokens, status FROM ai_usage_events ORDER BY created_at DESC LIMIT 20;`
- Check daily aggregates:
  - `SELECT day, flow, operation, mode, total_tokens_sum, events_count FROM ai_usage_daily_agg ORDER BY day DESC LIMIT 20;`
- Check per-document indexing statistics:
  - `SELECT run_id, document_id, chunks_count, llm_total_tokens_sum, embedding_total_tokens_sum FROM rag_index_document_metrics ORDER BY indexed_at DESC LIMIT 20;`
- JSON export endpoint:
  - `GET /api/v1/metrics/ai-usage?granularity=day`
