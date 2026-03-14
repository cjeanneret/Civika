# Erweiterte Nutzung

## Explizite RAG-Modi
- `RAG_MODE=local`: deterministisches Verhalten, keine LLM-Netzwerkaufrufe.
- `RAG_MODE=llm`: Embeddings und Zusammenfassungen über externe LLM-API.
- Kein stiller Fallback: unvollständige `llm`-Konfiguration führt zu explizitem Fehler.

## Wichtige Variablen
- `RAG_MODE`
- `RAG_SUPPORTED_LANGUAGES`
- `RAG_DEFAULT_LANGUAGE`
- `RAG_FALLBACK_LANGUAGE`
- `LLM_*` und `LLM_EMBEDDING_*` (für `llm`-Modus)

## Indexierung und Abfrage
- Indexieren:
  - `cd backend && go run ./cmd/rag-cli index`
  - oder `make rag-index`
- Abfragen:
  - `cd backend && go run ./cmd/rag-cli query --q "Was ist die wichtigste Auswirkung dieser Abstimmung?"`
  - oder `make rag-query Q="Was ist die wichtigste Auswirkung dieser Abstimmung?"`

## Pflicht zur Neuindexierung
Jede Änderung an Embedding-Modell/Provider/Dimensionen erfordert eine vollständige Neuindexierung.

Minimaler Ablauf:
1. `make init-db`
2. `make rag-index`
3. `make rag-query Q="..."`
