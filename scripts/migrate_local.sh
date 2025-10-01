#!/usr/bin/env bash
set -euo pipefail

# Apply SQL migrations in the migrations/ folder using local psql via DATABASE_URL.
# Prereq: psql installed. DATABASE_URL can be set in the environment or .env.

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

shopt -s nullglob
files=(migrations/*.sql)
if [ ${#files[@]} -eq 0 ]; then
  echo "No migration files found in migrations/"
  exit 0
fi

for f in "${files[@]}"; do
  echo "Applying migration: $(basename "$f")"
  if grep -qE "^-- \+migrate Up" "$f"; then
    tmpfile=$(mktemp)
    # Extract only the Up section between markers
    awk '/^-- \+migrate Up/{flag=1; next} /^-- \+migrate Down/{flag=0} flag' "$f" > "$tmpfile"
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$tmpfile"
    rm -f "$tmpfile"
  else
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$f"
  fi
done

echo "Migrations applied successfully."
