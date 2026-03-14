# Design und Architektur

## Überblick
- `backend/`: HTTP-API, Fachlogik, Sicherheit, RAG, externe Integrationen.
- `frontend/`: öffentliche Next.js-Oberfläche mit strikt typisiertem TypeScript.
- `docs/`: Dokumentation, Pläne und Audits.

## Kernprinzipien
- Klare Trennung der Schichten:
  - HTTP-Parsing/Validierung
  - Fachlogik
  - Datenzugriff
  - externe Aufrufe
- Einfachheit: explizite Typen, kurze Funktionen, reproduzierbare Konventionen.
- Sicherheit und Datenschutz als Designanforderung.

## RAG-Architektur
- Ingestion und Normalisierung offizieller politischer Daten.
- Chunking, Embeddings und Vektorspeicher in `pgvector`.
- Retrieval und kontrollierte Synthese.
- Explizite Moduswahl (`local`/`llm`) per Konfiguration.
