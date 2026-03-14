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

## Metriche token (senza costo)
- Le metriche di utilizzo IA persistite sono salvate in:
  - `ai_usage_events` (eventi dettagliati),
  - `ai_usage_daily_agg` (aggregati giornalieri),
  - `rag_index_document_metrics` (sintesi per documento indicizzato).
- Endpoint di esportazione JSON:
  - `GET /api/v1/metrics/ai-usage?granularity=day`
  - `GET /api/v1/metrics/ai-usage?granularity=event&flow=qa_query&operation=summarization&limit=100`
- Filtri supportati:
  - `granularity=event|day`
  - `from` / `to` (RFC3339)
  - `flow=rag_index|qa_query`
  - `operation=embedding|translation|summarization`
  - `mode=local|llm`
  - `limit` (1-1000), `offset` (>= 0)

## Reindicizzazione obbligatoria
Ogni modifica di modello/provider/dimensioni embedding richiede reindicizzazione completa.

Flusso minimo:
1. `make init-db`
2. `make rag-index`
3. `make rag-query Q="..."`
