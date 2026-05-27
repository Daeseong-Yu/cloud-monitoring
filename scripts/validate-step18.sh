#!/bin/sh
set -eu

test -f docs/operations/production-acceptance.md
test -f scripts/validate-production-readiness.sh
test -f scripts/validate-step18.sh

sh -n scripts/validate-production-readiness.sh
sh -n scripts/validate-step18.sh

grep -q '배포 전 Checklist' docs/operations/production-acceptance.md
grep -q '최초 기동 Checklist' docs/operations/production-acceptance.md
grep -q 'Grafana Dashboard 확인' docs/operations/production-acceptance.md
grep -q 'Admin UI 접근 경계 확인' docs/operations/production-acceptance.md
grep -q 'Collector, Alert, Summary, Retention 확인' docs/operations/production-acceptance.md
grep -q 'Backup/Restore 확인' docs/operations/production-acceptance.md
grep -q 'Rollback/Stop Command' docs/operations/production-acceptance.md
grep -q '운영 미반영 항목 기록 형식' docs/operations/production-acceptance.md
grep -q 'Volume 삭제는 rollback 절차에 포함하지 않는다' docs/operations/production-acceptance.md

grep -q 'validate-step12.sh' scripts/validate-production-readiness.sh
grep -q 'validate-step13.sh' scripts/validate-production-readiness.sh
grep -q 'validate-step14.sh' scripts/validate-production-readiness.sh
grep -q 'validate-step15.sh' scripts/validate-production-readiness.sh
grep -q 'validate-step16.sh' scripts/validate-production-readiness.sh
grep -q 'validate-step17.sh' scripts/validate-production-readiness.sh
grep -q 'RUN_GITLEAKS=1 sh scripts/scan-secrets.sh' scripts/validate-production-readiness.sh

jq -e '.status == "completed"' .ai/phases/cloud-monitor-production-readiness/index.json >/dev/null
jq -e '.currentFocus == null' .ai/phases/cloud-monitor-production-readiness/index.json >/dev/null
jq -e '.steps[] | select(.number == 18) | .status == "completed"' .ai/phases/cloud-monitor-production-readiness/index.json >/dev/null
jq -e '.tasks[] | select(.name == "cloud-monitor-production-readiness") | .status == "completed"' .ai/phases/index.json >/dev/null

db_scheme='postgres'
db_url="${db_scheme}://cloud_monitor:prod-password@postgres:5432/cloud_monitor?sslmode=disable"

env \
  AWS_REGION=us-east-1 \
  DATABASE_URL="$db_url" \
  POSTGRES_PASSWORD=prod-password \
  GRAFANA_ADMIN_PASSWORD=grafana-prod-password \
  ADMIN_USERNAME=operator \
  ADMIN_PASSWORD=admin-prod-password \
  sh scripts/validate-production-readiness.sh

echo "step18 validation passed"
