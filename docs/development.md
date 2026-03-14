# Développement local

## Prérequis
- Docker Desktop en fonctionnement
- Go récent (si exécution hors conteneur)
- Node.js récent (si exécution frontend hors conteneur)

## Démarrage de la stack
1. Copier les variables d'environnement:
   - `cp .env.example .env`
2. Lancer tous les services:
   - `docker compose up --build`
3. Arrêter la stack:
   - `docker compose down`

## Cibles Make utiles
- `make help`: liste des cibles
- `make env`: initialise `.env` si absent
- `make up`: démarre la stack
- `make down`: arrête la stack
- `make stack-smoke`: vérification rapide API + frontend
- `make backend-test`: tests backend
- `make security-check`: vérifications de sécurité dépendances/runtime

## Initialisation base de données RAG
- Démarrer PostgreSQL:
  - `docker compose up -d postgres`
- Initialiser la base (idempotent):
  - `bash scripts/init-db.sh`
- Alternative tout-en-un:
  - `make bootstrap`

## Bonnes pratiques
- Garder les secrets uniquement en variables d'environnement.
- Valider les changements avec des commandes reproductibles (`make`).
- Préférer des itérations courtes avec smoke tests fréquents.
