#!/bin/sh
set -eu

env_file="${DEPLOY_ENV_FILE:-/etc/cloud-monitor/cloud-monitor.env}"

if ! command -v psql >/dev/null 2>&1; then
  echo "psql command not found" >&2
  exit 1
fi

if [ -z "${DATABASE_URL:-}" ] && [ -r "$env_file" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$env_file"
  set +a
fi

if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is required; set it or load $env_file" >&2
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
