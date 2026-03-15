# Civika

Civika è un PoC open source per aiutare a comprendere l'impatto delle votazioni in Svizzera.  
Il progetto combina un'API backend in Go, un frontend TypeScript e una pipeline RAG basata su dati pubblici ufficiali.

## Perché Civika
- Spiegare le votazioni in modo accessibile, verificabile e multilingue.
- Approccio privacy-first: nessun account, nessuna profilazione, nessuna persistenza di dati utente.
- Sicurezza fin dal PoC: validazione rigorosa degli input, errori controllati, logging minimo.

## Stack
- Backend: Go (`net/http` + `chi`)
- Frontend: Next.js + TypeScript strict
- Dati/RAG: PostgreSQL + `pgvector`
- Infrastruttura locale: Docker + `docker compose`

## Avvio rapido
1. Copiare la configurazione:
   - `cp .env.example .env`
2. Avviare i servizi:
   - `docker compose up --build`
3. Verificare:
   - API: `GET /health` e `GET /info`
   - Frontend: `http://localhost:3000`

## Documentazione
- Indice documentazione: [`docs/README.it.md`](docs/README.it.md)
- Sviluppo locale: [`docs/development.it.md`](docs/development.it.md)
- Debugging: [`docs/debugging.it.md`](docs/debugging.it.md)
- Uso avanzato (RAG, indicizzazione, CI): [`docs/advanced-usage.it.md`](docs/advanced-usage.it.md)
- Design e architettura: [`docs/design.it.md`](docs/design.it.md)
- Data fetch OpenParlData: [`docs/data-fetch.it.md`](docs/data-fetch.it.md)

## Sicurezza e privacy
- Selezione esplicita della modalità RAG (`RAG_MODE=local|llm`), senza fallback silenzioso.
- Reindicizzazione completa obbligatoria dopo modifiche al modello o alle dimensioni degli embedding.
- Nessun segreto nel codice; configurazione sensibile tramite variabili d'ambiente.
- Nessuna persistenza di dati personali utente nel backend.

## Trasparenza sull'uso dell'IA
- Una parte significativa del codice è stata generata con assistenza IA, sotto supervisione umana.
- Test e validazione sono stati eseguiti manualmente.
- Sono state svolte anche sessioni di debugging assistite da IA.
- Dettagli: [`docs/ai-usage.it.md`](docs/ai-usage.it.md)

## Deploy OpenShift (Helm)
- Chart Helm: [`deploy/helm/civika`](deploy/helm/civika)
- Il chart supporta:
  - backend e frontend con `replicaCount=1` come valore predefinito,
  - servizi `LoadBalancer` e route OpenShift opzionali,
  - PostgreSQL in modalità `managed` (CloudNativePG) o `external`,
  - `rag_chunker` come Job parallelo e CronJob opzionale.
- Dettagli di configurazione ed esempi: [`docs/advanced-usage.it.md`](docs/advanced-usage.it.md)

## Licenza
Questo progetto è distribuito con licenza Apache 2.0.
- Testo completo: [`LICENSE`](LICENSE)
- Avvisi aggiuntivi: [`NOTICE`](NOTICE)
