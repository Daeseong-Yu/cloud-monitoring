#!/bin/sh
set -eu

test -d db/migrations
test -f scripts/apply-schema.sh

set -- db/migrations/*.sql
if [ "$1" = 'db/migrations/*.sql' ]; then
  echo "no migration files found" >&2
  exit 1
fi

for migration in "$@"; do
  test -s "$migration"
done

grep -q 'for migration in db/migrations/\*.sql' scripts/apply-schema.sh
grep -q 'psql --set ON_ERROR_STOP=1 "$DATABASE_URL" -f "$migration"' scripts/apply-schema.sh

if [ -n "${DATABASE_URL:-}" ]; then
  sh scripts/apply-schema.sh
else
  echo "DATABASE_URL is not set; skipping live migration replay"
fi

echo "migration validation passed"
