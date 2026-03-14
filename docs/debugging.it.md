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
