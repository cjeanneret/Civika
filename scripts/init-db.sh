#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SQL_FILE="${ROOT_DIR}/scripts/sql/init_pgvector.sql"
RAW_ENV_FILE="${ENV_FILE:-.env}"
RAW_COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"

if [[ "${RAW_ENV_FILE}" = /* ]]; then
  ENV_FILE="${RAW_ENV_FILE}"
else
  ENV_FILE="${ROOT_DIR}/${RAW_ENV_FILE}"
fi

if [[ "${RAW_COMPOSE_FILE}" = /* ]]; then
  COMPOSE_FILE="${RAW_COMPOSE_FILE}"
else
  COMPOSE_FILE="${ROOT_DIR}/${RAW_COMPOSE_FILE}"
fi

COMPOSE_CMD=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")

if [[ ! -f "${SQL_FILE}" ]]; then
  echo "Erreur: fichier SQL introuvable: ${SQL_FILE}" >&2
  exit 1
fi

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Erreur: fichier env introuvable: ${ENV_FILE}" >&2
  exit 1
fi

if [[ ! -f "${COMPOSE_FILE}" ]]; then
  echo "Erreur: fichier compose introuvable: ${COMPOSE_FILE}" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

POSTGRES_USER="${POSTGRES_USER:-postgres}"
POSTGRES_DB="${POSTGRES_DB:-civika}"
RAG_EMBEDDING_DIMENSIONS="${RAG_EMBEDDING_DIMENSIONS:-128}"

if ! [[ "${RAG_EMBEDDING_DIMENSIONS}" =~ ^[0-9]+$ ]] || [[ "${RAG_EMBEDDING_DIMENSIONS}" -le 0 ]]; then
  echo "Erreur: RAG_EMBEDDING_DIMENSIONS doit etre un entier positif." >&2
  exit 1
fi

cd "${ROOT_DIR}"

echo "[init-db] Utilisation env=${ENV_FILE}"
echo "[init-db] Utilisation compose=${COMPOSE_FILE}"
echo "[init-db] Demarrage du service postgres..."
"${COMPOSE_CMD[@]}" up -d postgres

echo "[init-db] Attente de disponibilite PostgreSQL..."
attempts=0
until "${COMPOSE_CMD[@]}" exec -T postgres pg_isready -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" >/dev/null 2>&1; do
  attempts=$((attempts + 1))
  if (( attempts > 30 )); then
    echo "Erreur: PostgreSQL indisponible apres attente." >&2
    exit 1
  fi
  sleep 2
done

echo "[init-db] Application du schema SQL de reference (${SQL_FILE})..."
TMP_SQL="$(mktemp)"
trap 'rm -f "${TMP_SQL}"' EXIT
sed "s/__RAG_EMBEDDING_DIMENSIONS__/${RAG_EMBEDDING_DIMENSIONS}/g" "${SQL_FILE}" > "${TMP_SQL}"

"${COMPOSE_CMD[@]}" exec -T postgres psql \
  -v ON_ERROR_STOP=1 \
  -U "${POSTGRES_USER}" \
  -d "${POSTGRES_DB}" < "${TMP_SQL}"

echo "[init-db] Schema Civika initialise avec succes."
