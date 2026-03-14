# OpenParlData fetch (votes)

This document describes the OpenParlData collection used for local tests, RAG corpus feeding, and extraction pipelines.

## Main command
- `cd backend && go run ./cmd/data-fetch`

Outputs:
- raw: `data/raw/openparldata/`
- normalized: `data/normalized/openparldata/`
- persistent cache: `data/fetch-cache/openparldata/`

## Endpoints
1. `https://api.openparldata.ch/v1/votings?limit=5&offset=0`
2. `https://api.openparldata.ch/v1/votings/{votingId}/affairs`
3. `https://api.openparldata.ch/v1/affairs/{affairId}/contributors`
4. `https://api.openparldata.ch/v1/affairs/{affairId}/docs`
5. `https://api.openparldata.ch/v1/affairs/{affairId}/texts`

## Test selection strategy
1. Fetch list (`limit=5`, `offset=0`).
2. Client-side filter `voting.date > nowUTC`.
3. If future votes exist, process those.
4. Otherwise, explicit fallback to fetched past/recent votes.

## Security and privacy notes
- HTTP requests are bounded (timeouts and size limits).
- Explicit failure on network/non-2xx errors.
- No personal user data is processed or persisted.
