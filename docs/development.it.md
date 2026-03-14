# Sviluppo locale

## Prerequisiti
- Docker Desktop attivo
- Versione recente di Go (se eseguito fuori container)
- Versione recente di Node.js (se frontend fuori container)

## Avvio della stack
1. Copiare le variabili ambiente:
   - `cp .env.example .env`
2. Avviare i servizi:
   - `docker compose up --build`
3. Arrestare la stack:
   - `docker compose down`

## Target Make utili
- `make help`
- `make env`
- `make up`
- `make down`
- `make stack-smoke`
- `make backend-test`
- `make security-check`

## Inizializzazione database RAG
- Avviare PostgreSQL:
  - `docker compose up -d postgres`
- Inizializzare il DB (idempotente):
  - `bash scripts/init-db.sh`
- Alternativa one-shot:
  - `make bootstrap`
