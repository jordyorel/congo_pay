#!/usr/bin/env bash
set -euo pipefail

# DROP all tables by recreating the public schema, then re-apply migrations locally.
# Uses DATABASE_URL (or POSTGRES_* vars) and local psql.

if ! command -v psql >/dev/null 2>&1; then
  echo "ERROR: psql not found. Install the Postgres client (e.g., 'brew install libpq && brew link --force libpq')." >&2
  exit 1
fi

# Load .env if present
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

# Build DATABASE_URL from POSTGRES_* if not provided
if [ -z "${DATABASE_URL:-}" ] && [ -n "${POSTGRES_HOST:-}" ]; then
  DATABASE_URL="postgresql://${POSTGRES_USER:-postgres}:${POSTGRES_PASSWORD:-}${POSTGRES_HOST:+@${POSTGRES_HOST}}:${POSTGRES_PORT:-5432}/${POSTGRES_DB:-postgres}?sslmode=disable"
fi

if [ -z "${DATABASE_URL:-}" ]; then
  echo "ERROR: DATABASE_URL is not set. Export it or add it to .env (or set POSTGRES_* vars)." >&2
  echo "Example: DATABASE_URL=postgresql://congopay:congopay@localhost:5432/congopay?sslmode=disable" >&2
  exit 1
fi

echo "Dropping and recreating schema 'public'…"
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c "DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;"

echo "Re-applying migrations…"
bash "$(dirname "$0")/migrate_local.sh"

echo "Database reset complete."

