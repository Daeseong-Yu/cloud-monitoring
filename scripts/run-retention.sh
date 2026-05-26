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

psql --set ON_ERROR_STOP=1 "$DATABASE_URL" -f db/retention/delete-old-metric-points.sql >/dev/null

echo "retention applied"
