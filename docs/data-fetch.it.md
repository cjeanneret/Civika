# Data fetch OpenParlData (votazioni)

Questo documento descrive la raccolta OpenParlData usata per test locali, alimentazione del corpus RAG e pipeline di estrazione.

## Comando principale
- `cd backend && go run ./cmd/data-fetch`

Output:
- raw: `data/raw/openparldata/`
- normalizzati: `data/normalized/openparldata/`
- cache persistente: `data/fetch-cache/openparldata/`

## Endpoint
1. `https://api.openparldata.ch/v1/votings?limit=5&offset=0`
2. `https://api.openparldata.ch/v1/votings/{votingId}/affairs`
3. `https://api.openparldata.ch/v1/affairs/{affairId}/contributors`
4. `https://api.openparldata.ch/v1/affairs/{affairId}/docs`
5. `https://api.openparldata.ch/v1/affairs/{affairId}/texts`

## Strategia di selezione (test)
1. Recuperare la lista (`limit=5`, `offset=0`).
2. Filtrare lato client su `voting.date > nowUTC`.
3. Se esistono votazioni future, elaborare quelle.
4. Altrimenti fallback esplicito su votazioni passate/recenti.

## Note sicurezza e privacy
- Richieste HTTP con limiti (timeout e dimensione risposta).
- Errore esplicito su rete o status non-2xx.
- Nessun dato personale utente viene trattato o persistito.
