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

## Token metrics (no cost)
- Persisted AI usage metrics are stored in:
  - `ai_usage_events` (detailed events),
  - `ai_usage_daily_agg` (daily aggregates),
  - `rag_index_document_metrics` (per-indexed-document summary).
- JSON export endpoint:
  - `GET /api/v1/metrics/ai-usage?granularity=day`
  - `GET /api/v1/metrics/ai-usage?granularity=event&flow=qa_query&operation=summarization&limit=100`
- Supported filters:
  - `granularity=event|day`
  - `from` / `to` (RFC3339)
  - `flow=rag_index|qa_query`
  - `operation=embedding|translation|summarization`
  - `mode=local|llm`
  - `limit` (1-1000), `offset` (>= 0)

## Mandatory re-index
Any change to embedding model/provider/dimensions requires full re-indexing.

Minimal flow:
1. `make init-db`
2. `make rag-index`
3. `make rag-query Q="..."`

## Helm deployment on OpenShift
- Chart: `deploy/helm/civika`
- Install/upgrade:
  - `helm upgrade --install civika deploy/helm/civika -n civika --create-namespace`

### PostgreSQL (RW/RO) with CloudNativePG
- Managed mode (cluster created by the chart):
  - `helm upgrade --install civika deploy/helm/civika -n civika --set postgresql.mode=managed`
- External mode (pre-existing cluster):
  - `helm upgrade --install civika deploy/helm/civika -n civika --set postgresql.mode=external --set postgresql.external.rwHost=pg-rw.example --set postgresql.external.roHost=pg-ro.example`
- In `managed` mode, CloudNativePG exposes:
  - RW service: `<release>-civika-postgres-rw`,
  - RO service: `<release>-civika-postgres-ro`.

### Backend and frontend
- Default values:
  - `backend.replicaCount=1`
  - `frontend.replicaCount=1`
- Both services default to `LoadBalancer`.
- OpenShift routes are configurable via:
  - `openshift.routes.enabled`
  - `openshift.routes.backend.enabled`
  - `openshift.routes.frontend.enabled`

### Temporary `rag_chunker` pods
- Ad-hoc parallel Job (enabled by default):
  - `ragChunker.job.enabled=true`
  - `ragChunker.job.parallelism=<n>`
  - `ragChunker.job.completions=<n>`
- CronJob (disabled by default):
  - `ragChunker.cron.enabled=true`
  - `ragChunker.cron.schedule="0 2 * * *"`
- Default command:
  - `/app/data-fetch && /app/rag-cli index --corpus /app/data/normalized --workers 4`
- RAG data volume:
  - `ragChunker.dataVolume.enabled=true`
  - `ragChunker.dataVolume.existingClaim=<pvc>` (optional)
