# Design e architettura

## Panoramica
- `backend/`: API HTTP, logica di business, sicurezza, RAG, integrazioni esterne.
- `frontend/`: interfaccia pubblica Next.js con TypeScript strict.
- `docs/`: documentazione, piani e audit.

## Principi chiave
- Separazione chiara dei layer:
  - parsing/validazione HTTP
  - logica di business
  - accesso dati
  - chiamate esterne
- Semplicità: tipi espliciti, funzioni brevi, convenzioni riproducibili.
- Sicurezza e privacy come requisiti di design.

## Architettura RAG
- Ingestion e normalizzazione di dati politici ufficiali.
- Chunking, embedding e storage vettoriale in `pgvector`.
- Retrieval e sintesi controllata.
- Selezione esplicita della modalità (`local`/`llm`) via configurazione.
