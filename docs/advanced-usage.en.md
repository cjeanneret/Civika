# Advanced usage

## Explicit RAG modes
- `RAG_MODE=local`: deterministic behavior, no LLM network calls.
- `RAG_MODE=llm`: embeddings and summaries through external LLM API.
- No silent fallback: incomplete `llm` config fails explicitly.

## Key variables
- `RAG_MODE`
- `RAG_SUPPORTED_LANGUAGES`
- `RAG_DEFAULT_LANGUAGE`
- `RAG_FALLBACK_LANGUAGE`
- `LLM_*` and `LLM_EMBEDDING_*` (for `llm` mode)

## Index and query
- Index:
  - `cd backend && go run ./cmd/rag-cli index`
  - or `make rag-index`
- Query:
  - `cd backend && go run ./cmd/rag-cli query --q "What is the main impact of this vote?"`
  - or `make rag-query Q="What is the main impact of this vote?"`

## Mandatory re-index
Any change to embedding model/provider/dimensions requires full re-indexing.

Minimal flow:
1. `make init-db`
2. `make rag-index`
3. `make rag-query Q="..."`
