# Uso avanzato

## Modalità RAG esplicite
- `RAG_MODE=local`: comportamento deterministico, nessuna chiamata LLM di rete.
- `RAG_MODE=llm`: embedding e sintesi tramite API LLM esterna.
- Nessun fallback silenzioso: configurazione `llm` incompleta => errore esplicito.

## Variabili chiave
- `RAG_MODE`
- `RAG_SUPPORTED_LANGUAGES`
- `RAG_DEFAULT_LANGUAGE`
- `RAG_FALLBACK_LANGUAGE`
- `LLM_*` e `LLM_EMBEDDING_*` (modalità `llm`)

## Indicizzazione e query
- Indicizzare:
  - `cd backend && go run ./cmd/rag-cli index`
  - oppure `make rag-index`
- Interrogare:
  - `cd backend && go run ./cmd/rag-cli query --q "Qual è l'impatto principale di questa votazione?"`
  - oppure `make rag-query Q="Qual è l'impatto principale di questa votazione?"`

## Reindicizzazione obbligatoria
Ogni modifica di modello/provider/dimensioni embedding richiede reindicizzazione completa.

Flusso minimo:
1. `make init-db`
2. `make rag-index`
3. `make rag-query Q="..."`
