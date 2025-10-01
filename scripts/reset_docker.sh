#!/usr/bin/env bash
set -euo pipefail

# DROP all tables by recreating the public schema in the Docker Postgres, then re-apply migrations.

DB_CONTAINER=${DB_CONTAINER:-congopay-db}
DB_USER=${POSTGRES_USER:-congopay}
DB_NAME=${POSTGRES_DB:-congopay}

if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: docker not found. Install Docker Desktop or docker CLI." >&2
  exit 1
fi

if ! docker ps --format '{{.Names}}' | grep -q "^${DB_CONTAINER}$"; then
  echo "ERROR: Postgres container '${DB_CONTAINER}' not running. Start it with: make compose-up" >&2
  exit 1
fi

echo "Dropping and recreating schema 'public' in ${DB_NAME}…"
docker exec -i "${DB_CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" -v ON_ERROR_STOP=1 -c "DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;"

echo "Re-applying migrations…"
bash "$(dirname "$0")/migrate.sh"

echo "Database reset complete."

