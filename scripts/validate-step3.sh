#!/bin/sh
set -eu

test -f db/migrations/001_init.sql
test -f db/retention/delete-old-metric-points.sql
test -f configs/metric-definitions.example.json
test -f cmd/metricdefs-sync/main.go

jq -e '.version == 1' configs/metric-definitions.example.json >/dev/null
jq -e '.resources | length >= 1' configs/metric-definitions.example.json >/dev/null
jq -e '.metricSets | length >= 1' configs/metric-definitions.example.json >/dev/null
jq -e '.bindings | length >= 1' configs/metric-definitions.example.json >/dev/null

grep -q 'CREATE TABLE IF NOT EXISTS metric_definitions' db/migrations/001_init.sql
grep -q 'CREATE TABLE IF NOT EXISTS metric_points' db/migrations/001_init.sql
grep -q 'metric_definitions_unique_metric' db/migrations/001_init.sql
grep -q 'metric_points_unique_point' db/migrations/001_init.sql
grep -q 'idx_metric_points_definition_time' db/migrations/001_init.sql
grep -q 'trg_metric_definitions_updated_at' db/migrations/001_init.sql
grep -q "interval '30 days'" db/retention/delete-old-metric-points.sql

AWS_REGION=REPLACE_WITH_AWS_REGION \
TARGET_INSTANCE_ID=REPLACE_WITH_INSTANCE_ID \
GOCACHE=/Users/09mac/project/cloud-monitor/.cache/go-build \
/usr/local/go/bin/go run ./cmd/metricdefs-sync -config configs/metric-definitions.example.json -dry-run >/tmp/cloud-monitor-step3-metricdefs.sql

grep -q 'ON CONFLICT ON CONSTRAINT metric_definitions_unique_metric DO UPDATE' /tmp/cloud-monitor-step3-metricdefs.sql
grep -q 'CPUUtilization' /tmp/cloud-monitor-step3-metricdefs.sql
grep -q 'disk_used_percent' /tmp/cloud-monitor-step3-metricdefs.sql

aws_key='A''KIA'
secret_word='S''ECRET'
private_word='P''RIVATE'
openssh_word='O''PENSSH'
arn_prefix='arn'':aws'
pattern="${aws_key}|${secret_word}|BEGIN (RSA|${openssh_word}|${private_word})|${arn_prefix}|[0-9]{12}|i-[0-9a-f]{8,}"

if grep -R --exclude scan-secrets.sh -E "$pattern" .ai README.md .env.example aws configs db scripts internal cmd >/dev/null; then
  echo "민감 정보로 보이는 패턴이 발견되었습니다." >&2
  exit 1
fi

sh scripts/scan-secrets.sh .ai README.md .env.example aws configs db scripts internal cmd

echo "step3 validation passed"
