# Civika

Civika ist ein Open-Source-PoC, das hilft, die Auswirkungen von Schweizer Abstimmungen zu verstehen.  
Das Projekt kombiniert eine Go-Backend-API, ein TypeScript-Frontend und eine RAG-Pipeline auf Basis offizieller öffentlicher Daten.

## Warum Civika
- Abstimmungen zugänglich, nachvollziehbar und mehrsprachig erklären.
- Privacy-first: keine Konten, kein Profiling, keine Persistenz von Nutzerdaten.
- Sicherheit ab dem PoC: strikte Eingabevalidierung, kontrollierte Fehler, minimales Logging.

## Stack
- Backend: Go (`net/http` + `chi`)
- Frontend: Next.js + striktes TypeScript
- Daten/RAG: PostgreSQL + `pgvector`
- Lokale Infrastruktur: Docker + `docker compose`

## Schnellstart
1. Konfiguration kopieren:
   - `cp .env.example .env`
2. Stack starten:
   - `docker compose up --build`
3. Prüfen:
   - API: `GET /health` und `GET /info`
   - Frontend: `http://localhost:3000`

## Dokumentation
- Dokumentationsindex: [`docs/README.de.md`](docs/README.de.md)
- Lokale Entwicklung: [`docs/development.de.md`](docs/development.de.md)
- Debugging: [`docs/debugging.de.md`](docs/debugging.de.md)
- Erweiterte Nutzung (RAG, Indexierung, CI): [`docs/advanced-usage.de.md`](docs/advanced-usage.de.md)
- Design und Architektur: [`docs/design.de.md`](docs/design.de.md)
- OpenParlData-Import: [`docs/data-fetch.de.md`](docs/data-fetch.de.md)

## Sicherheit und Datenschutz
- Explizite RAG-Moduswahl (`RAG_MODE=local|llm`), kein stiller Fallback.
- Vollständige Neuindexierung nach Änderungen am Embedding-Modell oder an Dimensionen.
- Keine Secrets im Code; sensible Konfiguration über Umgebungsvariablen.
- Keine Persistenz personenbezogener Nutzerdaten im Backend.

## Transparenz zur KI-Nutzung
- Ein wesentlicher Teil der Codebasis wurde mit KI-Unterstützung unter menschlicher Aufsicht generiert.
- Tests und Validierung wurden manuell durchgeführt.
- Es wurden auch KI-gestützte Debugging-Sessions durchgeführt.
- Details: [`docs/ai-usage.de.md`](docs/ai-usage.de.md)

## OpenShift-Bereitstellung (Helm)
- Helm-Chart: [`deploy/helm/civika`](deploy/helm/civika)
- Das Chart unterstützt:
  - Backend und Frontend mit Standardwert `replicaCount=1`,
  - `LoadBalancer`-Services und optionale OpenShift-Routes,
  - PostgreSQL im Modus `managed` (CloudNativePG) oder `external`,
  - `rag_chunker` als parallelen Job und optionalen CronJob.
- Konfigurationsdetails und Beispiele: [`docs/advanced-usage.de.md`](docs/advanced-usage.de.md)

## Lizenz
Dieses Projekt wird unter der Apache License 2.0 veröffentlicht.
- Volltext: [`LICENSE`](LICENSE)
- Zusätzliche Hinweise: [`NOTICE`](NOTICE)
