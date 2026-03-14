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
