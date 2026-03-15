# Tests funcziunals API

## Intent
Questa documentaziun descriva la suita da tests funcziunals API che valida:
- la conformitad cun il contract OpenAPI,
- la stabilitad da las respostas JSON,
- il control strict d'inputs nunvalids.

L'intent e da furnir ina prova reproducibla en CI che l'API ei validada.

## Utensils duvrai
- **OpenAPI 3**: contract da referenza (`docs/openapi/civika-api-v1.yaml`).
- **Schemathesis**: generaziun da tests tenor il contract (property-based).
- **Checks Python mirai**: scenaris critics che vegnan buca cuvrai avunda be cun fuzzing.

## Portada
- Endpoints da basa: `/`, `/health`, `/info`.
- Endpoints da mastergn V1: votaziuns, objects, taxonomias, metricas.
- Endpoint QA:
  - validaus cun checks mirai per inputs nunvalids,
  - exclus dal fuzzing contractual per reducir noise en CI.

## Restricziuns da test
- **Privacy-first**: neginas datas persunalas en payloads da test.
- **Security-first**: verificaziun dal format d'errors JSON e dals headers da segirezza.
- **Spertadad CI**: preparaziun RAG cun **mo ina fixture**:
  - `tests/fixtures/normalized/openparldata/01-voting-101357.json`

## Execuziun locala
1. Avrir PostgreSQL da test ed inizialisar il schema:
   - `docker compose --env-file .env.test -f docker-compose.test.yml up -d postgres`
   - `ENV_FILE=.env.test COMPOSE_FILE=docker-compose.test.yml make init-db`
2. Preparar in corpus minimal:
   - `mkdir -p data/normalized/openparldata`
   - `cp tests/fixtures/normalized/openparldata/01-voting-101357.json data/normalized/openparldata/`
3. Indexar en modus local:
   - `RAG_MODE=local LLM_ENABLED=false LLM_EMBEDDING_ENABLED=false POSTGRES_HOST=127.0.0.1 POSTGRES_PORT=5432 POSTGRES_USER=postgres POSTGRES_PASSWORD=civika-test-pass POSTGRES_DB=civika_test POSTGRES_SSLMODE=disable RAG_EMBEDDING_DIMENSIONS=768 make rag-index CORPUS=data/normalized/openparldata`
4. Avrir l'API localmain (modus RAG `local`) e suenter executar:
   - `API_BASE_URL=http://127.0.0.1:8080 make test-api-contract`

## Integraziun GitHub Actions
Workflow `.github/workflows/api-contract-tests.yml`:
- avra PostgreSQL,
- inizialisescha il schema,
- indexescha in corpus minimal (1 fixture),
- avra l'API en modus `local`,
- executescha:
  - tests contractuais Schemathesis,
  - checks Python mirai.

Il job frunta sche:
- il contract OpenAPI vegn violau,
- status HTTP ein nuncorrects,
- la validaziun d'input regressescha.
