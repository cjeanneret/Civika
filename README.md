# Civika

Civika est un PoC open source pour aider à comprendre l'impact des votations en Suisse.  
Le projet combine une API backend en Go, un frontend TypeScript et une chaîne RAG orientée données publiques officielles.

## Pourquoi Civika
- Expliquer les votations de manière accessible, vérifiable et multilingue.
- Rester privacy-first: pas de compte, pas de profilage, pas de persistance de données utilisateur.
- Sécuriser dès le PoC: validation stricte des entrées, erreurs maîtrisées, journalisation minimale.

## Stack
- Backend: Go (`net/http` + `chi`)
- Frontend: Next.js + TypeScript strict
- Données/RAG: PostgreSQL + `pgvector`
- Infra locale: Docker + `docker compose`

## Démarrage rapide
1. Copier la configuration:
   - `cp .env.example .env`
2. Lancer la stack:
   - `docker compose up --build`
3. Vérifier rapidement:
   - API: `GET /health` et `GET /info`
   - Frontend: `http://localhost:3000`

## Documentation
- Vue d'ensemble et navigation: [`docs/README.md`](docs/README.md)
- Développement local: [`docs/development.md`](docs/development.md)
- Debugging et troubleshooting: [`docs/debugging.md`](docs/debugging.md)
- Usage avancé (RAG, indexation, CI): [`docs/advanced-usage.md`](docs/advanced-usage.md)
- Design et architecture: [`docs/design.md`](docs/design.md)
- Collecte OpenParlData: [`docs/data-fetch.md`](docs/data-fetch.md)
- Audits de sécurité: [`docs/audits/20260314.md`](docs/audits/20260314.md)

## Principes sécurité et privacy
- Sélection de mode RAG explicite (`RAG_MODE=local|llm`), sans fallback silencieux.
- Réindexation obligatoire après changement de modèle ou de dimensions d'embedding.
- Aucun secret dans le code; configuration sensible via variables d'environnement.
- Aucun stockage de données personnelles utilisateur côté backend.

## Transparence sur l'usage de l'IA
- Une part importante du code a été générée avec assistance IA, sous supervision humaine.
- Les tests et la validation ont été réalisés manuellement.
- Des sessions de debugging assistées par IA ont aussi été menées.
- Détails: [`docs/ai-usage.md`](docs/ai-usage.md)

## Langues
La documentation principale est disponible en:
- Français: `README.md`
- Anglais: `README.en.md`
- Italien: `README.it.md`
- Allemand: `README.de.md`
- Romanche: `README.rm.md`

## Licence
Ce projet est distribué sous licence Apache 2.0.
- Texte complet: [`LICENSE`](LICENSE)
- Mentions additionnelles: [`NOTICE`](NOTICE)
