# Debugging

## Basisprüfungen
- API:
  - `GET /health`
  - `GET /info`
- Frontend:
  - `http://localhost:3000` prüfen
- Globaler Smoke-Test:
  - `make stack-smoke`

## Logs und Debug-Modus
- Das Backend unterstützt optionales NDJSON-Debug-Logging (standardmäßig deaktiviert).
- Variablen:
  - `DEBUG_LOG_ENABLED` (`true|false`, Standard `false`)
  - `DEBUG_LOG_PATH` (Standard `/tmp/debug-2055fd.log`)

## Vollständige Testinfrastruktur
- Start: `make test-infra-up`
- Logs: `make test-infra-logs`
- Stop: `make test-infra-down`
- Volumes löschen: `make test-infra-purge`

## Häufige Probleme
- DB-Verbindungsfehler: `postgres` und `POSTGRES_*` prüfen.
- Leere RAG-Antwort: `make rag-index`, danach `make rag-query Q="..."`.
- Fehler im `llm`-Modus: `LLM_*` und `LLM_EMBEDDING_*` prüfen; kein automatischer Fallback.

## Prüfung der KI-Metriken
- Letzte Ereignisse prüfen:
  - `SELECT event_id, created_at, flow, operation, mode, total_tokens, status FROM ai_usage_events ORDER BY created_at DESC LIMIT 20;`
- Tägliche Aggregate prüfen:
  - `SELECT day, flow, operation, mode, total_tokens_sum, events_count FROM ai_usage_daily_agg ORDER BY day DESC LIMIT 20;`
- Indexierungsstatistiken pro Dokument prüfen:
  - `SELECT run_id, document_id, chunks_count, llm_total_tokens_sum, embedding_total_tokens_sum FROM rag_index_document_metrics ORDER BY indexed_at DESC LIMIT 20;`
- JSON-Export-Endpunkt:
  - `GET /api/v1/metrics/ai-usage?granularity=day`
