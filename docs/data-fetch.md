# Data fetch OpenParlData (votations)

Ce document décrit la collecte OpenParlData utilisée pour les tests locaux, l'alimentation du corpus RAG et les pipelines d'extraction.

## Objectifs
- Construire un corpus minimal, stable et reproductible.
- Exploiter des endpoints structurés (pas de scraping HTML ad hoc).
- Conserver une exécution légère pour le debug local.

## Commande principale
- `cd backend && go run ./cmd/data-fetch`

Sorties:
- brutes: `data/raw/openparldata/`
- normalisées: `data/normalized/openparldata/`
- cache persistant: `data/fetch-cache/openparldata/`

## Endpoints exploités
1. Liste des votations:
   - `https://api.openparldata.ch/v1/votings?limit=5&offset=0`
2. Affaires liées à une votation:
   - `https://api.openparldata.ch/v1/votings/{votingId}/affairs`
3. Contributeurs:
   - `https://api.openparldata.ch/v1/affairs/{affairId}/contributors`
4. Documents:
   - `https://api.openparldata.ch/v1/affairs/{affairId}/docs`
5. Textes structurés:
   - `https://api.openparldata.ch/v1/affairs/{affairId}/texts`

## Stratégie de sélection (mode test)
1. Récupérer la liste (`limit=5`, `offset=0`).
2. Filtrer côté client sur `voting.date > nowUTC`.
3. Si des votations futures existent: traiter ce sous-ensemble.
4. Sinon: fallback explicite sur les votations récupérées (passées/récentes).
5. Appliquer le même pipeline d'enrichissement (`affairs`, `contributors`, `docs`, `texts`).

## Méthode de collecte
- Requêtes HTTP `GET` avec timeout et `User-Agent` explicite.
- Limites de taille pour réduire les risques DoS.
- Cache incrémental par URL canonique:
  - `DATA_FETCH_CACHE_TTL`
  - `DATA_FETCH_CACHE_RETENTION`
  - `DATA_FETCH_FORCE_REFRESH`
- En cas d'erreur réseau ou HTTP non-2xx: échec explicite, sans fallback silencieux.

## Normalisation
Chaque votation produit un JSON normalisé avec:
- `available_languages`
- `voting` (`id`, `date`, `title`, `source_url`)
- `affair` (métadonnées principales)
- `initiants`
- `arguments`
- `texts` et variantes localisées
- `selection_strategy` (`future` ou `fallback_past`)

Priorités de texte:
1. `docs[].text`
2. `texts[].text.*`
3. `docs[].name` + `docs[].url`

## Variables utiles
- `DATA_FETCH_VOTINGS_LIMIT` (défaut `5`)
- `DATA_FETCH_MAX_DEPTH` (défaut `3`)
- `DATA_FETCH_MAX_NODES_PER_VOTING` (défaut `150`)
- `DATA_FETCH_MIN_REQUEST_INTERVAL` (défaut `1s`)
- `DATA_FETCH_MAX_RETRIES` (défaut `3`)
- `DATA_FETCH_BACKOFF_MAX` (défaut `30s`)
- `DATA_FETCH_CACHE_DIR` (défaut `data/fetch-cache/openparldata`)
- `DATA_FETCH_CACHE_TTL` (défaut `72h`)
- `DATA_FETCH_CACHE_RETENTION` (défaut `168h`)
- `DATA_FETCH_FORCE_REFRESH` (défaut `false`)

Priorité de configuration: flags CLI > variables d'environnement > défauts.

## Lien avec la CLI RAG
- Indexer:
  - `cd backend && go run ./cmd/rag-cli index`
  - ou `make rag-index`
- Interroger:
  - `cd backend && go run ./cmd/rag-cli query --q "Quels sont les arguments principaux de cette votation ?"`
  - ou `make rag-query Q="Quels sont les arguments principaux de cette votation ?"`

## Privacy et sécurité
- Aucune donnée personnelle utilisateur n'est traitée.
- Le corpus est limité aux contenus politiques/administratifs publics.
- Les secrets API ne doivent jamais être stockés dans les fichiers versionnés.

## Voir aussi
- Développement local: [`development.md`](development.md)
- Usage avancé RAG: [`advanced-usage.md`](advanced-usage.md)
- Design global: [`design.md`](design.md)
