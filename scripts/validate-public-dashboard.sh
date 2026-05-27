#!/bin/sh
set -eu

dashboard_file="grafana/dashboards/cloud-monitor-ec2-mvp.json"
datasource_file="grafana/provisioning/datasources/postgres.yaml"
provider_file="grafana/provisioning/dashboards/cloud-monitor.yaml"

test -f "$dashboard_file"
test -f "$datasource_file"
test -f "$provider_file"
test -f docs/operations/public-dashboard-checklist.md
test -f docs/operations/aws-budget.md
test -f docs/operations/retention.md

jq -e '.editable == false' "$dashboard_file" >/dev/null
jq -e '.refresh | test("^[0-9]+m$")' "$dashboard_file" >/dev/null
jq -e '(.refresh | sub("m$"; "") | tonumber) >= 10 and (.refresh | sub("m$"; "") | tonumber) <= 15' "$dashboard_file" >/dev/null
jq -e '.time.from == "now-24h"' "$dashboard_file" >/dev/null
jq -e 'all(.panels[]; .datasource.type == "postgres" and .datasource.uid == "cloud-monitor-postgres")' "$dashboard_file" >/dev/null
jq -e 'all(.panels[].targets[]?; .datasource.type == "postgres" and .datasource.uid == "cloud-monitor-postgres")' "$dashboard_file" >/dev/null
jq -e '[.panels[] | select(.type == "timeseries")] | length == 6' "$dashboard_file" >/dev/null
jq -e 'all(.templating.list[]; .datasource.type == "postgres" and .datasource.uid == "cloud-monitor-postgres")' "$dashboard_file" >/dev/null

grep -q 'allowUiUpdates: false' "$provider_file"
grep -q 'editable: false' "$datasource_file"
grep -q 'secureJsonData:' "$datasource_file"
grep -q '${GRAFANA_POSTGRES_PASSWORD}' "$datasource_file"
grep -q "interval '30 days'" db/retention/delete-old-metric-points.sql

if grep -R -i 'cloudwatch\|logs' grafana >/dev/null; then
  echo "공개 dashboard 산출물에 CloudWatch 또는 Logs 참조가 있습니다." >&2
  exit 1
fi

if [ "${SKIP_COMMON_VALIDATION:-0}" != "1" ]; then
  sh scripts/validate-common.sh
fi

echo "public dashboard validation passed"
