#!/bin/sh
set -eu

test -f docs/operations/smoke-test.md
test -f scripts/smoke-compose.sh
test -f scripts/verify-grafana-health.sh
test -f scripts/verify-db-health.sh

sh -n scripts/smoke-compose.sh
sh -n scripts/verify-grafana-health.sh
sh -n scripts/verify-db-health.sh

grep -q 'infra-only mode' docs/operations/smoke-test.md
grep -q 'Admin UI unauthenticated `401`' docs/operations/smoke-test.md
grep -q 'schema migration' docs/operations/smoke-test.md
grep -q 'collector one-shot' docs/operations/smoke-test.md
grep -q 'SLACK_WEBHOOK_URL=' docs/operations/smoke-test.md
grep -q '기존 compose volume을 삭제하지 않는다' docs/operations/smoke-test.md

grep -q 'docker compose up -d --build postgres grafana admin-ui' scripts/smoke-compose.sh
grep -q 'validate-production-env.sh' scripts/smoke-compose.sh
grep -q 'verify-grafana-health.sh' scripts/smoke-compose.sh
grep -q 'verify-db-health.sh' scripts/smoke-compose.sh
grep -q 'expected 401' scripts/smoke-compose.sh
grep -q 'SLACK_WEBHOOK_URL=' scripts/smoke-compose.sh
grep -q 'infra-only mode' scripts/smoke-compose.sh
grep -q 'resource-discovery -dry-run' scripts/smoke-compose.sh
grep -q '/app/collector --once' scripts/smoke-compose.sh
grep -q 'metricdefs-sync.*-dry-run' scripts/smoke-compose.sh

if grep -E 'docker compose .*down.*-v|docker volume rm|rm -rf' scripts/smoke-compose.sh >/dev/null; then
  echo "smoke-compose.sh must not delete compose volumes or perform destructive cleanup" >&2
  exit 1
fi

docker compose \
  --profile setup \
  --profile discovery \
  --profile collector \
  --profile admin-ui \
  --profile jobs \
  config >/dev/null

sh scripts/validate-common.sh

echo "step14 validation passed"
