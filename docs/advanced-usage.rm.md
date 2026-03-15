# Utilisaziun avanzada

## Modus RAG explicits
- `RAG_MODE=local`: cumportament deterministic, naginas clomadas da rait LLM.
- `RAG_MODE=llm`: embeddings e resumaziuns via API LLM externa.
- Nagin fallback silencius: configuraziun `llm` incompletta producescha in sbagl explicit.

## Variablas centralas
- `RAG_MODE`
- `RAG_SUPPORTED_LANGUAGES`
- `RAG_DEFAULT_LANGUAGE`
- `RAG_FALLBACK_LANGUAGE`
- `LLM_*` e `LLM_EMBEDDING_*` (per il modus `llm`)

## Indexaziun e query
- Indexar:
  - `cd backend && go run ./cmd/rag-cli index`
  - u `make rag-index`
- Dumandar:
  - `cd backend && go run ./cmd/rag-cli query --q "Tge è l'effect principal da questa votaziun?"`
  - u `make rag-query Q="Tge è l'effect principal da questa votaziun?"`

## Metricas da token (senza custs)
- Las metricas persistidas da l'usage d'IA vegnan memorisadas en:
  - `ai_usage_events` (eveniments detagliads),
  - `ai_usage_daily_agg` (agregats dal di),
  - `rag_index_document_metrics` (sinteisa per document indexà).
- Endpoint d'exportaziun JSON:
  - `GET /api/v1/metrics/ai-usage?granularity=day`
  - `GET /api/v1/metrics/ai-usage?granularity=event&flow=qa_query&operation=summarization&limit=100`
- Filters sustegnids:
  - `granularity=event|day`
  - `from` / `to` (RFC3339)
  - `flow=rag_index|qa_query`
  - `operation=embedding|translation|summarization`
  - `mode=local|llm`
  - `limit` (1-1000), `offset` (>= 0)

## Reindexaziun obligatorica
Mintga midada dal model/provider/dimensiuns d'embedding pretenda ina reindexaziun cumpletta.

Flux minimal:
1. `make init-db`
2. `make rag-index`
3. `make rag-query Q="..."`

## Deploy Helm sin OpenShift
- Chart: `deploy/helm/civika`
- Installar/actualisar:
  - `helm upgrade --install civika deploy/helm/civika -n civika --create-namespace`

### PostgreSQL (RW/RO) cun CloudNativePG
- Modus managed (cluster creau dal chart):
  - `helm upgrade --install civika deploy/helm/civika -n civika --set postgresql.mode=managed`
- Modus external (cluster gia existent):
  - `helm upgrade --install civika deploy/helm/civika -n civika --set postgresql.mode=external --set postgresql.external.rwHost=pg-rw.example --set postgresql.external.roHost=pg-ro.example`
- En modus `managed`, CloudNativePG expuna:
  - servetsch RW: `<release>-civika-postgres-rw`,
  - servetsch RO: `<release>-civika-postgres-ro`.

### Backend e frontend
- Valurs standard:
  - `backend.replicaCount=1`
  - `frontend.replicaCount=1`
- Omisdus servetschs stattan sin `LoadBalancer` sco standard.
- Routes OpenShift ein configurablas cun:
  - `openshift.routes.enabled`
  - `openshift.routes.backend.enabled`
  - `openshift.routes.frontend.enabled`

### Pods temporars `rag_chunker`
- Job parallel ad hoc (activ sco standard):
  - `ragChunker.job.enabled=true`
  - `ragChunker.job.parallelism=<n>`
  - `ragChunker.job.completions=<n>`
- CronJob (deactivau sco standard):
  - `ragChunker.cron.enabled=true`
  - `ragChunker.cron.schedule="0 2 * * *"`
- Cummonda standard:
  - `/app/data-fetch && /app/rag-cli index --corpus /app/data/normalized --workers 4`
- Volumen da datas RAG:
  - `ragChunker.dataVolume.enabled=true`
  - `ragChunker.dataVolume.existingClaim=<pvc>` (opziunal)
