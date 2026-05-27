#!/bin/sh
set -eu

test -f docs/operations/production-env.md
test -f scripts/validate-production-env.sh

grep -q '현재 process environment' docs/operations/production-env.md
grep -q 'AWS_REGION' docs/operations/production-env.md
grep -q 'DATABASE_URL' docs/operations/production-env.md
grep -q 'POSTGRES_PASSWORD' docs/operations/production-env.md
grep -q 'GRAFANA_ADMIN_PASSWORD' docs/operations/production-env.md
grep -q 'ADMIN_USERNAME' docs/operations/production-env.md
grep -q 'ADMIN_PASSWORD' docs/operations/production-env.md
grep -q 'SLACK_WEBHOOK_URL' docs/operations/production-env.md
grep -q 'secret 값이 나오면 안 된다' docs/operations/production-env.md

sh scripts/validate-production-env.sh

docker compose \
  --profile setup \
  --profile discovery \
  --profile collector \
  --profile admin-ui \
  --profile jobs \
  config >/dev/null

sh scripts/validate-common.sh

echo "step13 validation passed"
