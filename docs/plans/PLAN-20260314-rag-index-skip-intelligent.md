# Plan d'implementation - Skip intelligent pour `rag-cli index`

## Contexte

Le pipeline actuel reingere, retraduit et re-embedde tout le corpus a chaque run. En mode `llm`, cela augmente fortement latence et couts.

## Objectifs

- Eviter le retraitement d'un document deja indexe s'il est inchange.
- Eviter de relancer une traduction deja disponible et coherente en base.
- Garder une politique non destructive (pas de purge des orphelins dans cette iteration).
- Ajouter des compteurs techniques de skip pour l'observabilite.

## Decisions principales

- Strategie hybride:
  - fingerprint source (metadata stable),
  - fallback hash de contenu normalise par langue quand la source est ambiguë.
- Reutilisation des traductions existantes `ready` quand `translation_source_hash` correspond.
- Chunking/embeddings uniquement pour les groupes de documents a retraiter.
- Conserver les enregistrements absents du corpus courant.

## Arborescence cible

- `backend/internal/rag/index_skip.go` pour la logique de comparaison d'etat et de skip.
- Extension de `backend/internal/rag/store.go` pour lire l'etat existant document + traductions.
- Adaptation de `backend/cmd/rag-cli/main.go` pour orchestration selective.

## Modifications de fichiers prevues

- `backend/internal/rag/index_skip.go`
- `backend/internal/rag/store.go`
- `backend/internal/rag/translate.go`
- `backend/cmd/rag-cli/main.go`
- `backend/internal/rag/index_skip_test.go`
- `backend/internal/rag/translate_test.go`
- `README.md`
- `.env.example`

## Verification post-generation

- `go test ./backend/internal/rag`
- `go test ./backend/cmd/rag-cli`
- `go test ./backend/...`
- Run 1 inchange: baisse nette des appels LLM.
- Run 2 avec 1 document modifie: retraitement cible.
- Verification non-destruction: pas de suppression hors corpus.

## Contraintes securite

- Aucun log de contenu sensible, prompt complet, token API.
- Logs strictement techniques (`ids`, compteurs, durees, decisions de skip).
