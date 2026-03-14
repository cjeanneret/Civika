# Utilisaziun avanzada

## Modus RAG explicits
- `RAG_MODE=local`: cumportament deterministic, naginas clomadas da rait LLM.
- `RAG_MODE=llm`: embeddings e resumaziuns via API LLM externa.
- Nagin fallback silencius: configuraziun `llm` incompletta producescha in sbagl explicit.

## Variablas centralas
- `RAG_MODE`
- `RAG_SUPPORTED_LANGUAGES`
- `RAG_DEFAULT_LANGUAGE`
- `RAG_FALLBACK_LANGUAGE`
- `LLM_*` e `LLM_EMBEDDING_*` (per il modus `llm`)

## Indexaziun e query
- Indexar:
  - `cd backend && go run ./cmd/rag-cli index`
  - u `make rag-index`
- Dumandar:
  - `cd backend && go run ./cmd/rag-cli query --q "Tge è l'effect principal da questa votaziun?"`
  - u `make rag-query Q="Tge è l'effect principal da questa votaziun?"`

## Reindexaziun obligatorica
Mintga midada dal model/provider/dimensiuns d'embedding pretenda ina reindexaziun cumpletta.

Flux minimal:
1. `make init-db`
2. `make rag-index`
3. `make rag-query Q="..."`
