# Test funzionali API

## Obiettivo
Questo documento descrive la suite di test funzionali API che valida:
- la conformita al contratto OpenAPI,
- la stabilita delle risposte JSON,
- il controllo rigoroso degli input non validi.

L'obiettivo e fornire una prova CI riproducibile che l'API sia validata.

## Strumenti utilizzati
- **OpenAPI 3**: contratto di riferimento (`docs/openapi/civika-api-v1.yaml`).
- **Schemathesis**: generazione property-based dei test dal contratto.
- **Controlli Python mirati**: scenari critici non coperti in modo sufficiente dal solo fuzzing.

## Ambito coperto
- Endpoint core: `/`, `/health`, `/info`.
- Endpoint business V1: votazioni, oggetti, tassonomie, metriche.
- Endpoint QA:
  - validato con controlli mirati sugli input invalidi,
  - escluso dal fuzzing contrattuale per ridurre rumore in CI.

## Vincoli di test
- **Privacy-first**: nessun dato personale nei payload di test.
- **Security-first**: verifiche sul formato JSON degli errori e sugli header di sicurezza.
- **Velocita CI**: preparazione RAG con **una sola fixture**:
  - `tests/fixtures/normalized/openparldata/01-voting-101357.json`

## Esecuzione locale
1. Avviare PostgreSQL di test e inizializzare lo schema:
   - `docker compose --env-file .env.test -f docker-compose.test.yml up -d postgres`
   - `ENV_FILE=.env.test COMPOSE_FILE=docker-compose.test.yml make init-db`
2. Preparare un corpus minimo:
   - `mkdir -p data/normalized/openparldata`
   - `cp tests/fixtures/normalized/openparldata/01-voting-101357.json data/normalized/openparldata/`
3. Indicizzare in modalita locale:
   - `RAG_MODE=local LLM_ENABLED=false LLM_EMBEDDING_ENABLED=false POSTGRES_HOST=127.0.0.1 POSTGRES_PORT=5432 POSTGRES_USER=postgres POSTGRES_PASSWORD=civika-test-pass POSTGRES_DB=civika_test POSTGRES_SSLMODE=disable RAG_EMBEDDING_DIMENSIONS=768 make rag-index CORPUS=data/normalized/openparldata`
4. Avviare l'API in locale (modalita RAG `local`) e poi eseguire:
   - `API_BASE_URL=http://127.0.0.1:8080 make test-api-contract`

## Integrazione GitHub Actions
Workflow `.github/workflows/api-contract-tests.yml`:
- avvia PostgreSQL,
- inizializza lo schema,
- indicizza un corpus minimo (1 fixture),
- avvia l'API in modalita `local`,
- esegue:
  - test contrattuali Schemathesis,
  - controlli Python mirati.

Il job fallisce in caso di:
- violazioni del contratto OpenAPI,
- status HTTP inattesi,
- regressioni nella validazione degli input.
