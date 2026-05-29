#!/bin/sh
set -eu

test -f grafana/dashboards/cloud-monitor-ec2-mvp.json
test -f grafana/dashboards/cloud-monitor-lambda.json
test -f grafana/dashboards/cloud-monitor-aws-resource.json
test -f grafana/dashboards/cloud-monitor-postgres-ops.json
test -f configs/recommended-metric-sets.json

for dashboard in grafana/dashboards/*.json; do
  jq -e '.editable == true' "$dashboard" >/dev/null
  jq -e '(.tags // []) | index("admin") and index("internal")' "$dashboard" >/dev/null
  jq -e '.refresh | test("^[0-9]+m$")' "$dashboard" >/dev/null
  jq -e '(.refresh | sub("m$"; "") | tonumber) >= 10' "$dashboard" >/dev/null
  jq -e 'all(.panels[]; .datasource.type == "postgres" and .datasource.uid == "cloud-monitor-postgres")' "$dashboard" >/dev/null
  jq -e 'all(.panels[].targets[]?; .datasource.type == "postgres" and .datasource.uid == "cloud-monitor-postgres")' "$dashboard" >/dev/null
  jq -e 'all(.templating.list[]?; .datasource.type == "postgres" and .datasource.uid == "cloud-monitor-postgres")' "$dashboard" >/dev/null
done

jq -e '[
  "region",
  "namespace",
  "resource_id",
  "metric_name"
] - [.templating.list[].name] | length == 0' grafana/dashboards/cloud-monitor-aws-resource.json >/dev/null

jq -e '[
  "Invocations",
  "Errors",
  "Duration",
  "Throttles"
] - [
  .panels[].targets[]?.rawSql
  | capture("md\\.metric_name = '\''(?<metric>[^'\'']+)'\''").metric
] | length == 0' grafana/dashboards/cloud-monitor-lambda.json >/dev/null

jq -e '
  ([.metricSets[] | select(.serviceName == "lambda" and .namespace == "AWS/Lambda") | .metrics[].metricName] | sort)
  == (["Duration", "Errors", "Invocations", "Throttles"] | sort)
' configs/recommended-metric-sets.json >/dev/null

grep -q 'pg_database_size' grafana/dashboards/cloud-monitor-postgres-ops.json
grep -q 'pg_total_relation_size' grafana/dashboards/cloud-monitor-postgres-ops.json
grep -q 'generic-resource-metrics.sql' /dev/null 2>/dev/null || test -f grafana/sql/generic-resource-metrics.sql

if grep -R -i 'type:[[:space:]]*cloudwatch\|"type":[[:space:]]*"cloudwatch"\|datasource.*cloudwatch\|logs' grafana >/dev/null; then
  echo "Grafana 산출물에 CloudWatch datasource 또는 Logs 참조가 있습니다." >&2
  exit 1
fi

sh scripts/validate-common.sh
SKIP_COMMON_VALIDATION=1 sh scripts/validate-public-dashboard.sh

echo "step11 validation passed"
