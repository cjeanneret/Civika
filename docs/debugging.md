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
