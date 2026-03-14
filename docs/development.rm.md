# Svilup local

## Pretensiuns
- Docker Desktop activ
- Versiun recenta da Go (sche l'execuziun è ordaifer containers)
- Versiun recenta da Node.js (sche il frontend va ordaifer containers)

## Avrir la stack
1. Copiar las variablas d'ambient:
   - `cp .env.example .env`
2. Avrir ils servetschs:
   - `docker compose up --build`
3. Fermar la stack:
   - `docker compose down`

## Targets Make utils
- `make help`
- `make env`
- `make up`
- `make down`
- `make stack-smoke`
- `make backend-test`
- `make security-check`

## Inizialisaziun da la banca da datas RAG
- Avrir PostgreSQL:
  - `docker compose up -d postgres`
- Inizialisar la DB (idempotent):
  - `bash scripts/init-db.sh`
- Alternativa one-shot:
  - `make bootstrap`
