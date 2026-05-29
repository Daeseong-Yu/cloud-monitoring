#!/bin/sh
set -eu

test -d grafana/dashboards
test -f configs/product-metric-catalog.json
test -f grafana/provisioning/dashboards/cloud-monitor.yaml
test -f grafana/provisioning/datasources/postgres.yaml

jq . grafana/dashboards/*.json >/dev/null

node <<'NODE'
const fs = require('fs');
const path = require('path');

const catalog = JSON.parse(fs.readFileSync('configs/product-metric-catalog.json', 'utf8'));
const allowedPairs = new Set(catalog.metrics.map((metric) => `${metric.namespace}\t${metric.metricName}`));
const allowedMetricNames = new Set(catalog.metrics.map((metric) => metric.metricName));
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

  for (const panel of dashboard.panels || []) {
    validateDatasource(file, panel.datasource, failures);
    for (const target of panel.targets || []) {
      validateDatasource(file, target.datasource, failures);
      const sql = target.rawSql || '';
      if (/cloudwatch|logs/i.test(sql)) {
        failures.push(`${file}: panel ${panel.title || panel.id} references CloudWatch or Logs`);
      }
      validateResourceSeries(file, panel, sql, failures);
      validateCatalogMetrics(file, panel, sql, failures);
    }
  }

  for (const variable of (((dashboard.templating || {}).list) || [])) {
    validateDatasource(file, variable.datasource, failures);
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
      if (!/md\.enabled = TRUE/.test(query)) {
        failures.push(`${file}: variable ${variable.name} should only list enabled metric definitions`);
      }
    }
    if (/\$\{[^}]*resource_id:sqlstring\}/.test(query) && /resource_id\s*=\s*\$\{[^}]*resource_id:sqlstring\}/.test(query)) {
      failures.push(`${file}: variable ${variable.name} must support multi resource selection`);
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

function validateDatasource(file, datasource, failures) {
  if (!datasource) return;
  const uid = datasource.uid || '';
  const type = datasource.type || '';
  if (uid === '-- Grafana --' && type === 'grafana') return;
  if (uid !== 'cloud-monitor-postgres' || type !== 'postgres') {
    failures.push(`${file}: datasource must be cloud-monitor-postgres or Grafana dashboard datasource`);
  }
}

function validateResourceSeries(file, panel, sql, failures) {
  if (!sql.includes('metric_points') || !sql.includes('mp.timestamp AS "time"')) return;
  if (/md\.resource_id\s*=\s*\$\{[^}]*resource_id:sqlstring\}/.test(sql)) {
    failures.push(`${file}: panel ${panel.title || panel.id} must not force a single resource_id`);
  }
  if (/\$\{[^}]*resource_id:sqlstring\}/.test(sql) && !/md\.resource_id\s+IN\s+\(\$\{[^}]*resource_id:sqlstring\}\)/.test(sql)) {
    failures.push(`${file}: panel ${panel.title || panel.id} must support multi resource selection`);
  }
  if (/\$\{[^}]*resource_id:sqlstring\}/.test(sql) && !/JOIN resources r/.test(sql)) {
    failures.push(`${file}: panel ${panel.title || panel.id} should join resources for display labels`);
  }
  if (/\$\{[^}]*resource_id:sqlstring\}/.test(sql) && !/COALESCE\(NULLIF\(r\.display_name, ''\), md\.resource_id\) AS metric/.test(sql)) {
    failures.push(`${file}: panel ${panel.title || panel.id} should emit one series per resource label`);
  }
  if (!/md\.enabled = TRUE/.test(sql)) {
    failures.push(`${file}: panel ${panel.title || panel.id} should query enabled metric definitions only`);
  }
}

function validateCatalogMetrics(file, panel, sql, failures) {
  const namespaces = [...sql.matchAll(/md\.namespace\s*=\s*'([^']+)'/g)].map((match) => match[1]);
  const metricNames = [...sql.matchAll(/md\.metric_name\s*=\s*'([^']+)'/g)].map((match) => match[1]);
  if (namespaces.length > 0 && metricNames.length > 0) {
    for (const metricName of metricNames) {
      if (!allowedPairs.has(`${namespaces[0]}\t${metricName}`)) {
        failures.push(`${file}: panel ${panel.title || panel.id} metric ${namespaces[0]}/${metricName} is not in product catalog`);
      }
    }
    return;
  }
  for (const metricName of metricNames) {
    if (!allowedMetricNames.has(metricName)) {
      failures.push(`${file}: panel ${panel.title || panel.id} metric ${metricName} is not in product catalog`);
    }
  }
}
NODE

grep -q 'allowUiUpdates: true' grafana/provisioning/dashboards/cloud-monitor.yaml
grep -q 'updateIntervalSeconds: 600' grafana/provisioning/dashboards/cloud-monitor.yaml
grep -q 'uid: cloud-monitor-postgres' grafana/provisioning/datasources/postgres.yaml
grep -q 'type: postgres' grafana/provisioning/datasources/postgres.yaml

if grep -RInE 'type: cloudwatch|uid:.*cloudwatch|CloudWatch Logs|logs panel' grafana >/dev/null; then
  echo "Grafana contains CloudWatch datasource or Logs panel references." >&2
  exit 1
fi

grep -q 'Grafana dashboard가 관리자/공개 조회 surface' README.md
grep -q '관리자 Grafana는 로그인 후 dashboard 편집을 허용' README.md
grep -q 'Grafana Public Dashboard' README.md
grep -q 'public-safe view' README.md

if grep -q 'public portfolio surface로 사용하지 않습니다' README.md; then
  echo "Outdated Grafana-not-public wording remains." >&2
  exit 1
fi

if grep -q '제품 API/UI surface' README.md; then
  echo "Outdated product API/UI public surface wording remains." >&2
  exit 1
fi

sh scripts/validate-common.sh

echo "step25 validation passed"
