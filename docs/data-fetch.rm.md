# Data fetch OpenParlData (votaziuns)

Quest document descriva la collecziun OpenParlData per tests locals, alimentaziun dal corpus RAG e pipelines d'extracziun.

## Cumond principal
- `cd backend && go run ./cmd/data-fetch`

Sortidas:
- brut: `data/raw/openparldata/`
- normalisà: `data/normalized/openparldata/`
- cache persistent: `data/fetch-cache/openparldata/`

## Endpoints
1. `https://api.openparldata.ch/v1/votings?limit=5&offset=0`
2. `https://api.openparldata.ch/v1/votings/{votingId}/affairs`
3. `https://api.openparldata.ch/v1/affairs/{affairId}/contributors`
4. `https://api.openparldata.ch/v1/affairs/{affairId}/docs`
5. `https://api.openparldata.ch/v1/affairs/{affairId}/texts`

## Strategia da selecziun (tests)
1. Cargar la glista (`limit=5`, `offset=0`).
2. Filtrar dal client cun `voting.date > nowUTC`.
3. Sche votaziuns futuras existan, tractar quellas.
4. Uschiglio fallback explicit sin votaziuns passadas/actualas.

## Remartgas segirezza e privacy
- Dumondas HTTP cun limits (timeouts e dimensiun da respostas).
- Sbagl explicit tar errors da rait u status non-2xx.
- Naginas datas persunalas d'utilisaders vegnan tractadas u persistidas.
