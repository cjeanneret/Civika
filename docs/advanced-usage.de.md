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

## Token-Metriken (ohne Kosten)
- Persistierte KI-Nutzungsmetriken werden gespeichert in:
  - `ai_usage_events` (detaillierte Ereignisse),
  - `ai_usage_daily_agg` (tägliche Aggregate),
  - `rag_index_document_metrics` (Zusammenfassung pro indexiertem Dokument).
- JSON-Export-Endpunkt:
  - `GET /api/v1/metrics/ai-usage?granularity=day`
  - `GET /api/v1/metrics/ai-usage?granularity=event&flow=qa_query&operation=summarization&limit=100`
- Unterstützte Filter:
  - `granularity=event|day`
  - `from` / `to` (RFC3339)
  - `flow=rag_index|qa_query`
  - `operation=embedding|translation|summarization`
  - `mode=local|llm`
  - `limit` (1-1000), `offset` (>= 0)

## Pflicht zur Neuindexierung
Jede Änderung an Embedding-Modell/Provider/Dimensionen erfordert eine vollständige Neuindexierung.

Minimaler Ablauf:
1. `make init-db`
2. `make rag-index`
3. `make rag-query Q="..."`

## Helm-Bereitstellung auf OpenShift
- Chart: `deploy/helm/civika`
- Installieren/Aktualisieren:
  - `helm upgrade --install civika deploy/helm/civika -n civika --create-namespace`

### PostgreSQL (RW/RO) mit CloudNativePG
- Managed-Modus (Cluster wird vom Chart erstellt):
  - `helm upgrade --install civika deploy/helm/civika -n civika --set postgresql.mode=managed`
- External-Modus (bestehender Cluster):
  - `helm upgrade --install civika deploy/helm/civika -n civika --set postgresql.mode=external --set postgresql.external.rwHost=pg-rw.example --set postgresql.external.roHost=pg-ro.example`
- Im Modus `managed` stellt CloudNativePG bereit:
  - RW-Service: `<release>-civika-postgres-rw`,
  - RO-Service: `<release>-civika-postgres-ro`.

### Backend und Frontend
- Standardwerte:
  - `backend.replicaCount=1`
  - `frontend.replicaCount=1`
- Beide Services sind standardmäßig `LoadBalancer`.
- OpenShift-Routen sind steuerbar über:
  - `openshift.routes.enabled`
  - `openshift.routes.backend.enabled`
  - `openshift.routes.frontend.enabled`

### Temporäre `rag_chunker`-Pods
- Ad-hoc-Parallel-Job (standardmäßig aktiviert):
  - `ragChunker.job.enabled=true`
  - `ragChunker.job.parallelism=<n>`
  - `ragChunker.job.completions=<n>`
- CronJob (standardmäßig deaktiviert):
  - `ragChunker.cron.enabled=true`
  - `ragChunker.cron.schedule="0 2 * * *"`
- Standardbefehl:
  - `/app/data-fetch && /app/rag-cli index --corpus /app/data/normalized --workers 4`
- RAG-Datenvolume:
  - `ragChunker.dataVolume.enabled=true`
  - `ragChunker.dataVolume.existingClaim=<pvc>` (optional)
