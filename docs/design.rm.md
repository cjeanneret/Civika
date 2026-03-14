# Design ed architectura

## Survista
- `backend/`: API HTTP, logica da fatschenta, segirezza, RAG, integraziuns externas.
- `frontend/`: interfaccia publica Next.js cun TypeScript strict.
- `docs/`: documentaziun, plans ed audits.

## Principis centrals
- Separaziun clera da las parts:
  - parsing/validaziun HTTP
  - logica da fatschenta
  - access a datas
  - clomadas externas
- Simplicitad: tips explicits, funcziuns curtas, convenziuns reproduciblas.
- Segirezza e privacy sco pretensiuns da design.

## Architectura RAG
- Ingestiun e normalisaziun da datas politicas uffizialas.
- Chunking, embeddings e vector store en `pgvector`.
- Retrieval e sintesa controllada.
- Selecziun explicita dal modus (`local`/`llm`) via configuraziun.
