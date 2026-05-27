#!/bin/sh
set -eu

test -f cmd/resource-discovery/main.go
test -f internal/discovery/discovery.go
test -f db/migrations/002_discovery.sql
test -f configs/recommended-metric-sets.json
test -f aws/iam/discovery-readonly-policy.json

jq -e '.Version == "2012-10-17"' aws/iam/discovery-readonly-policy.json >/dev/null
jq -e '
  ([
    .Statement[]
    | select(.Effect == "Allow")
    | .Action
  ]
  | flatten
  | sort)
  == ([
    "cloudwatch:ListMetrics",
    "tag:GetResources",
    "tag:GetTagKeys",
    "tag:GetTagValues"
  ]
  | sort)
' aws/iam/discovery-readonly-policy.json >/dev/null

if jq -r '.Statement[].Action[]?' aws/iam/discovery-readonly-policy.json | grep -E ':(Put|Create|Delete|Update|Terminate|Start|Stop|Reboot|Run|Attach|Detach|Modify|Assume|Pass)' >/dev/null; then
  echo "Discovery policy contains a non-read action." >&2
  exit 1
fi

grep -q 'CREATE TABLE IF NOT EXISTS resources' db/migrations/002_discovery.sql
grep -q 'CREATE TABLE IF NOT EXISTS discovered_metrics' db/migrations/002_discovery.sql
grep -q 'ADD COLUMN IF NOT EXISTS dimensions JSONB' db/migrations/002_discovery.sql
grep -q 'DROP CONSTRAINT IF EXISTS metric_definitions_unique_metric' db/migrations/002_discovery.sql
grep -q 'selected BOOLEAN NOT NULL DEFAULT FALSE' db/migrations/002_discovery.sql
grep -q 'enabled BOOLEAN NOT NULL DEFAULT FALSE' db/migrations/002_discovery.sql
grep -q 'GetResources' cmd/resource-discovery/main.go
grep -q 'ListMetrics' cmd/resource-discovery/main.go
grep -q 'resource-discovery' docker-compose.yml

jq -e '
  .version == 1
  and ([.metricSets[] | select(.serviceName == "ec2" and .namespace == "AWS/EC2")] | length == 1)
  and ([.metricSets[] | select(.serviceName == "lambda" and .namespace == "AWS/Lambda")] | length == 1)
' configs/recommended-metric-sets.json >/dev/null

sh scripts/validate-common.sh

echo "step8 validation passed"
