# Local development

## Prerequisites
- Docker Desktop running
- Recent Go version (if running outside containers)
- Recent Node.js version (if running frontend outside containers)

## Start the stack
1. Copy environment variables:
   - `cp .env.example .env`
2. Start all services:
   - `docker compose up --build`
3. Stop the stack:
   - `docker compose down`

## Useful Make targets
- `make help`
- `make env`
- `make up`
- `make down`
- `make stack-smoke`
- `make backend-test`
- `make security-check`

## RAG database initialization
- Start PostgreSQL:
  - `docker compose up -d postgres`
- Initialize DB (idempotent):
  - `bash scripts/init-db.sh`
- One-shot alternative:
  - `make bootstrap`
