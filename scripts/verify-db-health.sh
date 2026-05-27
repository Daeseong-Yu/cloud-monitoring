#!/bin/sh
set -eu

if ! command -v psql >/dev/null 2>&1; then
  echo "psql command not found" >&2
  exit 1
fi

if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is required" >&2
  exit 2
fi

psql --set ON_ERROR_STOP=1 "$DATABASE_URL" >/dev/null <<'SQL'
SELECT 1;
SELECT pg_database_size(current_database());
SELECT
    schemaname,
    relname,
    pg_total_relation_size(quote_ident(schemaname) || '.' || quote_ident(relname)) AS total_bytes
FROM pg_stat_user_tables
WHERE schemaname = 'public'
ORDER BY total_bytes DESC
LIMIT 10;
SQL

echo "database health check passed"
