# Civika

Civika è in PoC open source per gidar a chapir l'effect da votaziuns en Svizra.  
Il project combinescha in backend API en Go, in frontend TypeScript e ina pipeline RAG basada sin datas publicas uffizialas.

## Pertge Civika
- Explicar votaziuns en moda accessibla, verificabla e plurilingua.
- Privacy-first: nagins contos, nagin profiling, nagina persistenza da datas d'utilisader.
- Segirezza gia dal PoC: validaziun stricta da las entradas, sbagls controllads, logging minimal.

## Stack
- Backend: Go (`net/http` + `chi`)
- Frontend: Next.js + TypeScript strict
- Datas/RAG: PostgreSQL + `pgvector`
- Infrastructura locala: Docker + `docker compose`

## Start rapid
1. Copiar la configuraziun:
   - `cp .env.example .env`
2. Avrir ils servetschs:
   - `docker compose up --build`
3. Verificar:
   - API: `GET /health` e `GET /info`
   - Frontend: `http://localhost:3000`

## Documentaziun
- Index da la documentaziun: [`docs/README.rm.md`](docs/README.rm.md)
- Svilup local: [`docs/development.rm.md`](docs/development.rm.md)
- Debugging: [`docs/debugging.rm.md`](docs/debugging.rm.md)
- Utilisaziun avanzada (RAG, indexaziun, CI): [`docs/advanced-usage.rm.md`](docs/advanced-usage.rm.md)
- Design ed architectura: [`docs/design.rm.md`](docs/design.rm.md)
- Data fetch OpenParlData: [`docs/data-fetch.rm.md`](docs/data-fetch.rm.md)

## Segirezza e privacy
- Selecziun explicita dal modus RAG (`RAG_MODE=local|llm`), senza fallback silencius.
- Reindexaziun cumpletta obligatorica suenter midadas dal model u da las dimensiuns d'embedding.
- Nagins secrets en il code; configuraziun sensibla via variablas d'ambient.
- Nagina persistenza da datas persunalas d'utilisader en il backend.

## Transparenza davart l'utilisaziun da l'IA
- Ina part impurtonta dal code ei vegnida generada cun assistenza IA sut survigilonza humana.
- Tests e validaziun ein vegni fatgs manualmein.
- Sessions da debugging assistidas da IA ein era vegnidas fatgas.
- Detagls: [`docs/ai-usage.rm.md`](docs/ai-usage.rm.md)

## Licenza
Quest project ei publitgaus sut la licenza Apache 2.0.
- Text cumplet: [`LICENSE`](LICENSE)
- Remartgas supplementaras: [`NOTICE`](NOTICE)
