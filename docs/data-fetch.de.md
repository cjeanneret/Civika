# OpenParlData-Import (Abstimmungen)

Dieses Dokument beschreibt die OpenParlData-Erfassung für lokale Tests, RAG-Korpusaufbau und Extraktionspipelines.

## Hauptbefehl
- `cd backend && go run ./cmd/data-fetch`

Ausgaben:
- Rohdaten: `data/raw/openparldata/`
- Normalisiert: `data/normalized/openparldata/`
- Persistenter Cache: `data/fetch-cache/openparldata/`

## Endpoints
1. `https://api.openparldata.ch/v1/votings?limit=5&offset=0`
2. `https://api.openparldata.ch/v1/votings/{votingId}/affairs`
3. `https://api.openparldata.ch/v1/affairs/{affairId}/contributors`
4. `https://api.openparldata.ch/v1/affairs/{affairId}/docs`
5. `https://api.openparldata.ch/v1/affairs/{affairId}/texts`

## Auswahlstrategie (Tests)
1. Liste laden (`limit=5`, `offset=0`).
2. Clientseitig nach `voting.date > nowUTC` filtern.
3. Wenn zukünftige Abstimmungen vorhanden sind, diese verarbeiten.
4. Sonst expliziter Fallback auf geladene vergangene/aktuelle Abstimmungen.

## Hinweise zu Sicherheit und Datenschutz
- HTTP-Aufrufe mit Begrenzungen (Timeouts und Größenlimits).
- Expliziter Fehler bei Netzwerk-/Non-2xx-Fehlern.
- Es werden keine personenbezogenen Nutzerdaten verarbeitet oder persistiert.
