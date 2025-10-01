#!/usr/bin/env bash
set -euo pipefail

# Apply SQL migrations in the migrations/ folder to the Postgres container.
# Requires docker and the compose stack running (make compose-up).

DB_CONTAINER=${DB_CONTAINER:-congopay-db}
DB_USER=${POSTGRES_USER:-congopay}
DB_NAME=${POSTGRES_DB:-congopay}

if ! docker ps --format '{{.Names}}' | grep -q "^${DB_CONTAINER}$"; then
  echo "ERROR: Postgres container '${DB_CONTAINER}' not running. Start it with: make compose-up" >&2
  exit 1
fi

shopt -s nullglob
files=(migrations/*.sql)
if [ ${#files[@]} -eq 0 ]; then
  echo "No migration files found in migrations/"
  exit 0
fi

for f in "${files[@]}"; do
  base=$(basename "$f")
  echo "Applying migration: $base"
  docker cp "$f" "${DB_CONTAINER}:/tmp/${base}"
  docker exec -i "${DB_CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" -v ON_ERROR_STOP=1 -f "/tmp/${base}"
done

echo "Migrations applied successfully."

