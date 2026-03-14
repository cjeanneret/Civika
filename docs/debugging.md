# Debugging

## Vérifications de base
- API:
  - `GET /health`
  - `GET /info`
- Frontend:
  - vérifier l'accès sur `http://localhost:3000`
- Smoke global:
  - `make stack-smoke`

## Logs et mode debug
- Le backend expose un logging debug NDJSON optionnel (désactivé par défaut).
- Variables:
  - `DEBUG_LOG_ENABLED` (`true|false`, défaut `false`)
  - `DEBUG_LOG_PATH` (défaut `/tmp/debug-2055fd.log`)
- Recommandation: n'activer ce mode qu'en debug local.

## Infra de tests complète
- Démarrage:
  - `make test-infra-up`
- Logs:
  - `make test-infra-logs`
- Arrêt:
  - `make test-infra-down`
- Nettoyage volumes:
  - `make test-infra-purge`

## Smoke API dédié
- Commande:
  - `make test-api-smoke`
- Cette cible vérifie les endpoints backend clés et prépare les données RAG de test.

## Incidents fréquents
- Erreur de connexion DB:
  - vérifier que `postgres` est lancé et que `POSTGRES_*` est cohérent.
- Réponse RAG vide:
  - vérifier l'indexation (`make rag-index`) puis retester (`make rag-query Q="..."`).
- Échec mode `llm`:
  - vérifier les variables `LLM_*` et `LLM_EMBEDDING_*`; aucun fallback automatique n'est appliqué.

## Inspection métriques IA
- Vérifier les derniers événements:
  - `SELECT event_id, created_at, flow, operation, mode, total_tokens, status FROM ai_usage_events ORDER BY created_at DESC LIMIT 20;`
- Vérifier les agrégats journaliers:
  - `SELECT day, flow, operation, mode, total_tokens_sum, events_count FROM ai_usage_daily_agg ORDER BY day DESC LIMIT 20;`
- Vérifier les statistiques d'indexation par document:
  - `SELECT run_id, document_id, chunks_count, llm_total_tokens_sum, embedding_total_tokens_sum FROM rag_index_document_metrics ORDER BY indexed_at DESC LIMIT 20;`
- Endpoint JSON d'export:
  - `GET /api/v1/metrics/ai-usage?granularity=day`
