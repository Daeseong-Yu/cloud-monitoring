#!/bin/sh
set -eu

dashboard_file="grafana/dashboards/cloud-monitor-ec2-mvp.json"
datasource_file="grafana/provisioning/datasources/postgres.yaml"
provider_file="grafana/provisioning/dashboards/cloud-monitor.yaml"

test -f "$dashboard_file"
test -f "$datasource_file"
test -f "$provider_file"

for query_file in \
  grafana/sql/cpu-utilization.sql \
  grafana/sql/memory-usage.sql \
  grafana/sql/disk-usage.sql \
  grafana/sql/network-in.sql \
  grafana/sql/network-out.sql \
  grafana/sql/status-check-failed.sql \
  grafana/sql/regions.sql \
  grafana/sql/resources.sql \
  grafana/sql/empty-state.sql
do
  test -f "$query_file"
done

jq -e '.uid == "cloud-monitor-ec2-mvp"' "$dashboard_file" >/dev/null
jq -e '.refresh == "10m"' "$dashboard_file" >/dev/null
jq -e '.time.from == "now-24h"' "$dashboard_file" >/dev/null
jq -e '[.panels[] | select(.type == "timeseries")] | length == 6' "$dashboard_file" >/dev/null
jq -e '[.panels[] | select(.type == "table")] | length >= 1' "$dashboard_file" >/dev/null
jq -e 'all(.panels[]; .datasource.uid == "cloud-monitor-postgres" and .datasource.type == "postgres")' "$dashboard_file" >/dev/null
jq -e 'all(.panels[].targets[]?; .datasource.uid == "cloud-monitor-postgres" and .datasource.type == "postgres")' "$dashboard_file" >/dev/null
jq -e 'all(.panels[].targets[]?; (.rawSql | contains("metric_points")) and (.rawSql | contains("metric_definitions")))' "$dashboard_file" >/dev/null
jq -e 'all(.panels[].targets[]?; (.rawSql | contains("$__timeFilter")))' "$dashboard_file" >/dev/null
jq -e 'all([.panels[].targets[]? | select(.format == "time_series")][];
  (.rawSql | contains("md.resource_id IN (${resource_id:sqlstring})"))
  and (.rawSql | contains("md.region = ${region:sqlstring}"))
)' "$dashboard_file" >/dev/null
jq -e '[
  .panels[]
  | select(.type == "timeseries")
  | .targets[0].rawSql
  | capture("md\\.metric_name = '\''(?<metric>[^'\'']+)'\''").metric
] | sort == ([
  "CPUUtilization",
  "NetworkIn",
  "NetworkOut",
  "StatusCheckFailed",
  "disk_used_percent",
  "mem_used_percent"
] | sort)' "$dashboard_file" >/dev/null

grep -q 'type: postgres' "$datasource_file"
grep -q 'uid: cloud-monitor-postgres' "$datasource_file"
grep -q '${GRAFANA_POSTGRES_PASSWORD}' "$datasource_file"
grep -q 'allowUiUpdates: true' "$provider_file"

if grep -R -i 'type:[[:space:]]*cloudwatch\|"type":[[:space:]]*"cloudwatch"\|datasource.*cloudwatch' grafana >/dev/null; then
  echo "Grafana 산출물에 CloudWatch datasource 참조가 있습니다." >&2
  exit 1
fi

sh scripts/validate-common.sh

echo "step5 validation passed"
