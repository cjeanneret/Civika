# Design et architecture

## Vue d'ensemble
- `backend/`: API HTTP, logique métier, sécurité, RAG, intégrations externes.
- `frontend/`: interface publique Next.js en TypeScript strict.
- `docs/`: documentation, plans et audits.

## Principes structurants
- Séparation claire des couches:
  - parsing/validation HTTP,
  - logique métier,
  - accès données,
  - appels externes.
- Simplicité: types explicites, fonctions courtes, conventions reproductibles.
- Sécurité et privacy intégrées dès le design.

## Architecture RAG
- Ingestion et normalisation de données politiques publiques.
- Découpage en chunks, embeddings, puis stockage vectoriel `pgvector`.
- Requêtage vectoriel + synthèse contrôlée.
- Mode d'exécution explicite (`local`/`llm`) via configuration.

## Données et conformité
- Sources autorisées: portails et API officiels (OpenParlData, OFS/FSO, opendata.swiss, etc.).
- Aucune donnée utilisateur identifiante dans le cœur métier.
- Aucune persistance de profils, sessions ou historiques utilisateurs.

## Surface API
- Endpoints de base:
  - `GET /health`
  - `GET /info`
- Endpoints métier sous `/api/v1` pour votations, objets, taxonomies et QA.

## Internationalisation
- Frontend avec routage par préfixe de langue.
- Langues actives: `fr`, `de`, `it`, `rm`, `en`.
- Fallback d'interface normalisé côté frontend.
