# Debugging

## Verificaziuns da basa
- API:
  - `GET /health`
  - `GET /info`
- Frontend:
  - controllar `http://localhost:3000`
- Smoke global:
  - `make stack-smoke`

## Logs e modus debug
- Il backend sustegna logging debug NDJSON opziunal (deactivà per default).
- Variablas:
  - `DEBUG_LOG_ENABLED` (`true|false`, default `false`)
  - `DEBUG_LOG_PATH` (default `/tmp/debug-2055fd.log`)

## Infrastructura da tests cumpletta
- Start: `make test-infra-up`
- Logs: `make test-infra-logs`
- Stop: `make test-infra-down`
- Schubregiar volumes: `make test-infra-purge`

## Problems frequents
- Sbagl da connexiun DB: controllar `postgres` e `POSTGRES_*`.
- Resposta RAG vida: far `make rag-index`, suenter `make rag-query Q="..."`.
- Sbagl en modus `llm`: controllar `LLM_*` e `LLM_EMBEDDING_*`; nagin fallback automatic.

## Inspectiun da las metricas IA
- Controllar ils eveniments recents:
  - `SELECT event_id, created_at, flow, operation, mode, total_tokens, status FROM ai_usage_events ORDER BY created_at DESC LIMIT 20;`
- Controllar ils agregats dal di:
  - `SELECT day, flow, operation, mode, total_tokens_sum, events_count FROM ai_usage_daily_agg ORDER BY day DESC LIMIT 20;`
- Controllar las statisticas d'indexaziun per document:
  - `SELECT run_id, document_id, chunks_count, llm_total_tokens_sum, embedding_total_tokens_sum FROM rag_index_document_metrics ORDER BY indexed_at DESC LIMIT 20;`
- Endpoint d'exportaziun JSON:
  - `GET /api/v1/metrics/ai-usage?granularity=day`
