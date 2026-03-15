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
  - `LLM_MAX_PROMPT_CHARS`
  - `LLM_TRANSLATION_MAX_RETRIES`
  - `LLM_EMBEDDING_ENABLED`
  - `LLM_EMBEDDING_BASE_URL`
  - `LLM_EMBEDDING_MODEL_NAME`
- Cache QA (reduction usage LLM):
  - `QA_CACHE_ENABLED`
  - `QA_CACHE_EXACT_TTL`
  - `QA_CACHE_EXACT_MAX_ENTRIES`
  - `QA_CACHE_SEMANTIC_ENABLED`
  - `QA_CACHE_SEMANTIC_TTL`
  - `QA_CACHE_SEMANTIC_MAX_ENTRIES`
  - `QA_CACHE_SEMANTIC_SIMILARITY_THRESHOLD`
  - `QA_CACHE_SEMANTIC_MIN_QUESTION_CHARS`

## Reduction tokens (quick wins)
- Resume QA contraint a une sortie courte (1 a 2 phrases) cote prompt.
- Les retries de traduction sont controles par `LLM_TRANSLATION_MAX_RETRIES`.
- Les logs techniques n'exposent pas de preview brute de reponse LLM.

## Caching QA (phase 2)
- Le cache exact (L1) et semantique (L2) est configurable via `QA_CACHE_*`.
- Le cache semantique reutilise les questions sanitisees pour retrouver des requetes proches.
- Aucune metadonnee personnelle n'est stockee (pas d'IP, pas d'identifiant utilisateur).
- En cas de doute (score faible ou contexte incompatible), le cache est ignore et la requete suit le chemin LLM normal.

## Indexation et requête
- Indexer le corpus:
  - `cd backend && go run ./cmd/rag-cli index`
  - `cd backend && go run ./cmd/rag-cli index --workers 4`
  - ou `make rag-index`
- Interroger:
  - `cd backend && go run ./cmd/rag-cli query --q "Quel est l'impact principal de la votation ?"`
  - ou `make rag-query Q="Quel est l'impact principal de la votation ?"`
- Pipeline d'indexation:
  - Le traitement est fait par document: traduction (si mode `llm`) -> chunking -> embeddings -> insertion.
  - Un document termine devient visible en base sans attendre la fin complete du lot.
  - `--workers` permet une parallelisation controlee (borne a 1..8, defaut `1`).

## Métriques de tokens (sans coût)
- Les appels IA persistés sont stockés dans :
  - `ai_usage_events` (événements détaillés),
  - `ai_usage_daily_agg` (agrégats journaliers),
  - `rag_index_document_metrics` (synthèse par document indexé).
- Endpoint d'export JSON :
  - `GET /api/v1/metrics/ai-usage?granularity=day`
  - `GET /api/v1/metrics/ai-usage?granularity=event&flow=qa_query&operation=summarization&limit=100`
- Filtres pris en charge :
  - `granularity=event|day`
  - `from` / `to` (RFC3339)
  - `flow=rag_index|qa_query`
  - `operation=embedding|translation|summarization`
  - `mode=local|llm`
  - `limit` (1-1000), `offset` (>= 0)

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
