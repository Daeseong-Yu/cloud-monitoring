#!/bin/sh
set -eu

if ! command -v pg_dump >/dev/null 2>&1; then
  echo "pg_dump command not found" >&2
  exit 1
fi

if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is required" >&2
  exit 2
fi

backup_dir="${BACKUP_DIR:-./backups}"
timestamp="$(date -u '+%Y%m%dT%H%M%SZ')"
backup_file="$backup_dir/cloud-monitor-$timestamp.dump"

mkdir -p "$backup_dir"

pg_dump \
  --format=custom \
  --no-owner \
  --no-privileges \
  --file "$backup_file" \
  "$DATABASE_URL"

printf '%s\n' "$backup_file"
