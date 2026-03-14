# Civika

Civika is an open-source PoC that helps people understand the impact of Swiss popular votes.  
It combines a Go backend API, a TypeScript frontend, and a RAG pipeline built on official public data.

## Why Civika
- Explain votes in an accessible, verifiable, multilingual way.
- Stay privacy-first: no accounts, no profiling, no user-data persistence.
- Enforce security from day one: strict input validation, controlled errors, minimal logging.

## Stack
- Backend: Go (`net/http` + `chi`)
- Frontend: Next.js + strict TypeScript
- Data/RAG: PostgreSQL + `pgvector`
- Local infra: Docker + `docker compose`

## Quick start
1. Copy configuration:
   - `cp .env.example .env`
2. Start the stack:
   - `docker compose up --build`
3. Check services:
   - API: `GET /health` and `GET /info`
   - Frontend: `http://localhost:3000`

## Documentation
- Documentation index: [`docs/README.en.md`](docs/README.en.md)
- Local development: [`docs/development.en.md`](docs/development.en.md)
- Debugging: [`docs/debugging.en.md`](docs/debugging.en.md)
- Advanced usage (RAG, indexing, CI): [`docs/advanced-usage.en.md`](docs/advanced-usage.en.md)
- Design and architecture: [`docs/design.en.md`](docs/design.en.md)
- OpenParlData fetch: [`docs/data-fetch.en.md`](docs/data-fetch.en.md)

## Security and privacy
- Explicit RAG mode selection (`RAG_MODE=local|llm`), no silent fallback.
- Full re-index required after embedding model or dimension changes.
- No secrets in code; sensitive configuration via environment variables.
- No personal user data persisted on the backend.

## AI usage transparency
- A significant part of the codebase was generated with AI assistance, under human supervision.
- Testing and validation were performed manually.
- AI-assisted debugging sessions were also conducted.
- Details: [`docs/ai-usage.en.md`](docs/ai-usage.en.md)

## License
This project is released under the Apache License 2.0.
- Full text: [`LICENSE`](LICENSE)
- Additional notices: [`NOTICE`](NOTICE)
