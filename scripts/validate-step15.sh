#!/bin/sh
set -eu

test -f docs/operations/backup-restore.md
test -f scripts/backup-postgres.sh
test -f scripts/validate-migrations.sh
test -f scripts/validate-step15.sh

sh -n scripts/backup-postgres.sh
sh -n scripts/validate-migrations.sh

grep -q 'pg_dump --format=custom' docs/operations/backup-restore.md
grep -q 'pg_restore' docs/operations/backup-restore.md
grep -q -- '--clean' docs/operations/backup-restore.md
grep -q -- '--if-exists' docs/operations/backup-restore.md
grep -q 'daily 7개' docs/operations/backup-restore.md
grep -q 'weekly 4개' docs/operations/backup-restore.md
grep -q 'Summary rollup은 raw retention보다 먼저 실행한다' docs/operations/backup-restore.md
grep -q '실제 dump 파일을 commit하지 않는다' docs/operations/backup-restore.md

grep -q 'DATABASE_URL is required' scripts/backup-postgres.sh
grep -q 'BACKUP_DIR:-./backups' scripts/backup-postgres.sh
grep -q -- '--format=custom' scripts/backup-postgres.sh
grep -q -- '--no-owner' scripts/backup-postgres.sh
grep -q -- '--no-privileges' scripts/backup-postgres.sh
grep -q 'printf' scripts/backup-postgres.sh

if grep -E 'echo "?\$DATABASE_URL|printf .*DATABASE_URL' scripts/backup-postgres.sh >/dev/null; then
  echo "backup script must not print DATABASE_URL" >&2
  exit 1
fi

grep -q '^backups/$' .gitignore

sh scripts/validate-migrations.sh
sh scripts/validate-common.sh

echo "step15 validation passed"
