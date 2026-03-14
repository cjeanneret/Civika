# Debugging

## Verifiche di base
- API:
  - `GET /health`
  - `GET /info`
- Frontend:
  - verificare `http://localhost:3000`
- Smoke globale:
  - `make stack-smoke`

## Log e modalità debug
- Il backend supporta logging debug NDJSON opzionale (disattivato di default).
- Variabili:
  - `DEBUG_LOG_ENABLED` (`true|false`, default `false`)
  - `DEBUG_LOG_PATH` (default `/tmp/debug-2055fd.log`)

## Infrastruttura test completa
- Avvio: `make test-infra-up`
- Log: `make test-infra-logs`
- Stop: `make test-infra-down`
- Pulizia volumi: `make test-infra-purge`

## Problemi frequenti
- Errore connessione DB: verificare `postgres` e `POSTGRES_*`.
- Risposta RAG vuota: eseguire `make rag-index`, poi `make rag-query Q="..."`.
- Errore modalità `llm`: verificare `LLM_*` e `LLM_EMBEDDING_*`; nessun fallback automatico.

## Ispezione metriche IA
- Verificare gli eventi recenti:
  - `SELECT event_id, created_at, flow, operation, mode, total_tokens, status FROM ai_usage_events ORDER BY created_at DESC LIMIT 20;`
- Verificare gli aggregati giornalieri:
  - `SELECT day, flow, operation, mode, total_tokens_sum, events_count FROM ai_usage_daily_agg ORDER BY day DESC LIMIT 20;`
- Verificare le statistiche di indicizzazione per documento:
  - `SELECT run_id, document_id, chunks_count, llm_total_tokens_sum, embedding_total_tokens_sum FROM rag_index_document_metrics ORDER BY indexed_at DESC LIMIT 20;`
- Endpoint di esportazione JSON:
  - `GET /api/v1/metrics/ai-usage?granularity=day`
