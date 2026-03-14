# Design and architecture

## Overview
- `backend/`: HTTP API, business logic, security, RAG, external integrations.
- `frontend/`: public Next.js UI with strict TypeScript.
- `docs/`: documentation, plans, and audits.

## Core principles
- Clear layering:
  - HTTP parsing/validation
  - business logic
  - data access
  - external calls
- Keep it simple: explicit types, short functions, reproducible conventions.
- Security and privacy are design-time requirements.

## RAG architecture
- Ingestion and normalization of official political data.
- Chunking, embeddings, vector storage in `pgvector`.
- Retrieval and controlled synthesis.
- Explicit mode selection (`local`/`llm`) through configuration.
