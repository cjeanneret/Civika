# Usage avancé

## Modes RAG explicites
- `RAG_MODE=local`: fonctionnement déterministe, sans appel réseau LLM.
- `RAG_MODE=llm`: embeddings et résumés via API externe.
- Aucun fallback silencieux: une configuration incomplète en mode `llm` provoque une erreur explicite.

## Variables clés
- Mode et langues:
  - `RAG_MODE`
  - `RAG_SUPPORTED_LANGUAGES`
  - `RAG_DEFAULT_LANGUAGE`
  - `RAG_FALLBACK_LANGUAGE`
- LLM (mode `llm`):
  - `LLM_ENABLED`
  - `LLM_BASE_URL`
  - `LLM_MODEL_NAME`
  - `LLM_EMBEDDING_ENABLED`
  - `LLM_EMBEDDING_BASE_URL`
  - `LLM_EMBEDDING_MODEL_NAME`

## Indexation et requête
- Indexer le corpus:
  - `cd backend && go run ./cmd/rag-cli index`
  - ou `make rag-index`
- Interroger:
  - `cd backend && go run ./cmd/rag-cli query --q "Quel est l'impact principal de la votation ?"`
  - ou `make rag-query Q="Quel est l'impact principal de la votation ?"`

## Réindexation obligatoire
Toute modification de modèle d'embedding, de fournisseur, ou de dimensions (`RAG_EMBEDDING_DIMENSIONS`) impose une réindexation complète.

Procédure minimale:
1. Réinitialiser la base en dev: `make init-db`
2. Réindexer: `make rag-index`
3. Vérifier une requête: `make rag-query Q="..."`

## Pipelines RAG
- Niveau 1 (smoke sans DB):
  - `make rag-debug-pipeline`
- Niveau 2 (intégration avec DB):
  - `make rag-integration-pipeline Q="Quels sont les arguments principaux de cette votation ?"`
- Pipeline complet:
  - `make rag-ci Q="Quels sont les arguments principaux de cette votation ?"`
