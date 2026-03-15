# Funktionale API-Tests

## Ziel
Dieses Dokument beschreibt die funktionale API-Testsuite zur Validierung von:
- OpenAPI-Vertragskonformitat,
- Stabilitat der JSON-Antworten,
- strikter Behandlung ungueltiger Eingaben.

Ziel ist ein reproduzierbarer CI-Nachweis, dass die API validiert ist.

## Verwendete Werkzeuge
- **OpenAPI 3**: verbindlicher Vertrag (`docs/openapi/civika-api-v1.yaml`).
- **Schemathesis**: eigenschaftsbasierte Testfallgenerierung aus dem Vertrag.
- **Gezielte Python-Checks**: kritische Szenarien, die durch Fuzzing allein nicht stabil genug abgedeckt sind.

## Abgedeckter Umfang
- Kernendpunkte: `/`, `/health`, `/info`.
- Fachendpunkte V1: Abstimmungen, Objekte, Taxonomien, Metriken.
- QA-Endpunkt:
  - ueber gezielte Invalid-Input-Checks validiert,
  - vom Vertrags-Fuzzing ausgeschlossen, um CI-Rauschen zu reduzieren.

## Testvorgaben
- **Privacy-first**: keine personenbezogenen Daten in Test-Payloads.
- **Security-first**: JSON-Fehlerformat und Security-Header werden geprueft.
- **CI-Geschwindigkeit**: RAG-Vorbereitung mit **genau einer Fixture**:
  - `tests/fixtures/normalized/openparldata/01-voting-101357.json`

## Lokale Ausfuehrung
1. Test-PostgreSQL starten und Schema initialisieren:
   - `docker compose --env-file .env.test -f docker-compose.test.yml up -d postgres`
   - `ENV_FILE=.env.test COMPOSE_FILE=docker-compose.test.yml make init-db`
2. Minimales Korpus vorbereiten:
   - `mkdir -p data/normalized/openparldata`
   - `cp tests/fixtures/normalized/openparldata/01-voting-101357.json data/normalized/openparldata/`
3. In lokalem Modus indexieren:
   - `RAG_MODE=local LLM_ENABLED=false LLM_EMBEDDING_ENABLED=false POSTGRES_HOST=127.0.0.1 POSTGRES_PORT=5432 POSTGRES_USER=postgres POSTGRES_PASSWORD=civika-test-pass POSTGRES_DB=civika_test POSTGRES_SSLMODE=disable RAG_EMBEDDING_DIMENSIONS=768 make rag-index CORPUS=data/normalized/openparldata`
4. API lokal starten (RAG-Modus `local`) und danach ausfuehren:
   - `API_BASE_URL=http://127.0.0.1:8080 make test-api-contract`

## GitHub-Actions-Integration
Workflow `.github/workflows/api-contract-tests.yml`:
- startet PostgreSQL,
- initialisiert das Schema,
- indexiert ein minimales Korpus (1 Fixture),
- startet die API im Modus `local`,
- fuehrt aus:
  - Schemathesis-Vertragstests,
  - gezielte Python-Checks.

Der Job schlaegt fehl bei:
- OpenAPI-Vertragsverletzungen,
- unerwarteten HTTP-Statuscodes,
- Regressionen in der Eingabevalidierung.
