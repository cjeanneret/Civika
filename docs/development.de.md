# Lokale Entwicklung

## Voraussetzungen
- Docker Desktop läuft
- Aktuelle Go-Version (bei Ausführung außerhalb von Containern)
- Aktuelle Node.js-Version (bei Frontend-Ausführung außerhalb von Containern)

## Stack starten
1. Umgebungsvariablen kopieren:
   - `cp .env.example .env`
2. Dienste starten:
   - `docker compose up --build`
3. Stack stoppen:
   - `docker compose down`

## Nützliche Make-Targets
- `make help`
- `make env`
- `make up`
- `make down`
- `make stack-smoke`
- `make backend-test`
- `make security-check`

## RAG-Datenbank initialisieren
- PostgreSQL starten:
  - `docker compose up -d postgres`
- DB initialisieren (idempotent):
  - `bash scripts/init-db.sh`
- One-shot-Alternative:
  - `make bootstrap`
