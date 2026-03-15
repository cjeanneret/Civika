# Tests fonctionnels API

## Objectif
Cette documentation decrit la batterie de tests fonctionnels API qui valide:
- la conformite au contrat OpenAPI,
- la stabilite des reponses JSON,
- le controle des entrees invalides.

Le dispositif vise a montrer que l'API est validee de facon reproductible en CI.

## Outils utilises
- **OpenAPI 3**: contrat source de verite (`docs/openapi/civika-api-v1.yaml`).
- **Schemathesis**: generation de cas de test a partir du contrat.
- **Checks Python cibles**: verification de scenarios critiques non couverts de facon suffisante par le fuzzing.

## Portee couverte
- Endpoints coeur: `/`, `/health`, `/info`.
- Endpoints metier V1: votations, objets, taxonomies, metrics.
- Endpoint QA:
  - valide via checks cibles sur les cas d'entree invalides,
  - exclu du fuzzing contractuel pour limiter le bruit CI.

## Contraintes de test
- **Privacy-first**: aucune donnee personnelle dans les payloads de test.
- **Security-first**: verification des erreurs API JSON et des headers de securite.
- **Performance CI**: preparation RAG avec **une seule fixture**:
  - `tests/fixtures/normalized/openparldata/01-voting-101357.json`

## Execution locale
1. Demarrer PostgreSQL de test et initialiser la base:
   - `docker compose --env-file .env.test -f docker-compose.test.yml up -d postgres`
   - `ENV_FILE=.env.test COMPOSE_FILE=docker-compose.test.yml make init-db`
2. Preparer un corpus minimal:
   - `mkdir -p data/normalized/openparldata`
   - `cp tests/fixtures/normalized/openparldata/01-voting-101357.json data/normalized/openparldata/`
3. Indexer en mode local:
   - `RAG_MODE=local LLM_ENABLED=false LLM_EMBEDDING_ENABLED=false POSTGRES_HOST=127.0.0.1 POSTGRES_PORT=5432 POSTGRES_USER=postgres POSTGRES_PASSWORD=civika-test-pass POSTGRES_DB=civika_test POSTGRES_SSLMODE=disable RAG_EMBEDDING_DIMENSIONS=768 make rag-index CORPUS=data/normalized/openparldata`
4. Lancer l'API en local (mode RAG local), puis executer:
   - `API_BASE_URL=http://127.0.0.1:8080 make test-api-contract`

## Integration GitHub Actions
Le workflow `.github/workflows/api-contract-tests.yml`:
- demarre PostgreSQL,
- initialise la base,
- indexe un corpus minimal (1 fixture),
- lance l'API en mode `local`,
- execute:
  - tests Schemathesis,
  - checks cibles Python.

Le job echoue sur:
- violation du contrat OpenAPI,
- statut HTTP inattendu,
- regression de validation d'entree.
