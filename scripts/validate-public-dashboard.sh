#!/bin/sh
set -eu

dashboard_dir="grafana/dashboards"
datasource_file="grafana/provisioning/datasources/postgres.yaml"
provider_file="grafana/provisioning/dashboards/cloud-monitor.yaml"

test -d "$dashboard_dir"
test -f "$datasource_file"
test -f "$provider_file"
test -f docs/operations/public-dashboard-checklist.md
test -f docs/operations/aws-budget.md
test -f docs/operations/retention.md

jq . "$dashboard_dir"/*.json >/dev/null

node <<'NODE'
const fs = require('fs');
const path = require('path');

const dashboardFiles = fs.readdirSync('grafana/dashboards')
  .filter((file) => file.endsWith('.json'))
  .map((file) => path.join('grafana/dashboards', file));
const failures = [];

for (const file of dashboardFiles) {
  const dashboard = JSON.parse(fs.readFileSync(file, 'utf8'));
  const tags = Array.isArray(dashboard.tags) ? dashboard.tags : [];
  if (dashboard.editable !== true) {
    failures.push(`${file}: admin dashboard must be editable`);
  }
  if (!tags.includes('admin') || !tags.includes('internal')) {
    failures.push(`${file}: admin dashboard must carry admin and internal tags`);
  }
  if (!refreshAtLeastTenMinutes(dashboard.refresh || '')) {
    failures.push(`${file}: refresh must be at least 10m`);
  }
  if ((dashboard.time || {}).from !== 'now-24h') {
    failures.push(`${file}: default time range must be now-24h`);
  }

  for (const panel of dashboard.panels || []) {
    validateDatasource(file, panel.datasource);
    for (const target of panel.targets || []) {
      validateDatasource(file, target.datasource);
      const sql = target.rawSql || '';
      if (/cloudwatch|logs/i.test(sql)) {
        failures.push(`${file}: panel ${panel.title || panel.id} references CloudWatch or Logs`);
      }
      if (/md\.resource_id\s*=\s*\$\{[^}]*resource_id:sqlstring\}/.test(sql)) {
        failures.push(`${file}: panel ${panel.title || panel.id} must not force a single resource_id`);
      }
      if (/\$\{[^}]*resource_id:sqlstring\}/.test(sql) && !/md\.resource_id\s+IN\s+\(\$\{[^}]*resource_id:sqlstring\}\)/.test(sql)) {
        failures.push(`${file}: panel ${panel.title || panel.id} must support multi resource selection`);
      }
    }
  }

  for (const variable of (((dashboard.templating || {}).list) || [])) {
    validateDatasource(file, variable.datasource);
    const query = variable.query || variable.definition || '';
    if (/cloudwatch|logs/i.test(query)) {
      failures.push(`${file}: variable ${variable.name} references CloudWatch or Logs`);
    }
    if (/FROM metric_definitions WHERE md\./.test(query)) {
      failures.push(`${file}: variable ${variable.name} references md alias without declaring it`);
    }
    if (variable.name && variable.name.includes('resource_id')) {
      if (variable.multi !== true || variable.includeAll !== true) {
        failures.push(`${file}: variable ${variable.name} must allow multi/all resource selection`);
      }
      if (!/JOIN resources r/.test(query)) {
        failures.push(`${file}: variable ${variable.name} should display resource labels from resources`);
      }
    }
  }
}

if (failures.length > 0) {
  console.error(failures.join('\n'));
  process.exit(1);
}

function refreshAtLeastTenMinutes(value) {
  const match = String(value).trim().match(/^(\d+)([smhd])$/);
  if (!match) return false;
  const amount = Number(match[1]);
  const unit = match[2];
  const seconds = unit === 's' ? amount : unit === 'm' ? amount * 60 : unit === 'h' ? amount * 3600 : amount * 86400;
  return seconds >= 600;
}

function validateDatasource(file, datasource) {
  if (!datasource) return;
  const uid = datasource.uid || '';
  const type = datasource.type || '';
  if (uid === '-- Grafana --' && type === 'grafana') return;
  if (uid !== 'cloud-monitor-postgres' || type !== 'postgres') {
    failures.push(`${file}: datasource must be cloud-monitor-postgres or Grafana dashboard datasource`);
  }
}
NODE

grep -q 'allowUiUpdates: true' "$provider_file"
grep -q 'editable: false' "$datasource_file"
grep -q 'secureJsonData:' "$datasource_file"
grep -q '${GRAFANA_POSTGRES_PASSWORD}' "$datasource_file"
grep -q "interval '30 days'" db/retention/delete-old-metric-points.sql

if grep -R -i 'cloudwatch\|logs' grafana >/dev/null; then
  echo "Grafana 산출물에 CloudWatch 또는 Logs 참조가 있습니다." >&2
  exit 1
fi

if [ "${SKIP_COMMON_VALIDATION:-0}" != "1" ]; then
  sh scripts/validate-common.sh
fi

echo "grafana dashboard boundary validation passed"
