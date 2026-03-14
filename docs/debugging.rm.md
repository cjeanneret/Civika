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
