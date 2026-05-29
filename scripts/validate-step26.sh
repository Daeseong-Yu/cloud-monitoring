#!/bin/sh
set -eu

migration="db/migrations/006_public_grafana_dashboard.sql"
dashboard="grafana/public-dashboards/cloud-monitor-public-overview.json"
provider="grafana/provisioning/dashboards/cloud-monitor.yaml"

test -f "$migration"
test -f "$dashboard"

grep -q 'CREATE OR REPLACE VIEW public_grafana_default_metric_catalog' "$migration"
grep -q 'CREATE OR REPLACE VIEW public_grafana_metric_summary' "$migration"
grep -q 'CREATE OR REPLACE VIEW public_grafana_metric_points' "$migration"
grep -q 'JOIN public_grafana_default_metric_catalog catalog' "$migration"
grep -q 'r.public_enabled = TRUE' "$migration"
grep -q 'r.enabled = TRUE' "$migration"
grep -q 'md.public_enabled = TRUE' "$migration"
grep -q 'md.enabled = TRUE' "$migration"
grep -q "r.public_label <> ''" "$migration"
grep -q "md.public_label <> ''" "$migration"

public_view_select="$(sed -n '/CREATE OR REPLACE VIEW public_grafana_metric_points/,/FROM metric_points mp/p' "$migration")"
if printf '%s\n' "$public_view_select" | grep -E 'resource_id|account_id|arn|tags|sanitized_error|dimensions|namespace' >/dev/null; then
  echo "Public Grafana view output must not expose raw identifiers or diagnostics." >&2
  exit 1
fi

jq -e '.uid == "cloud-monitor-public-overview"' "$dashboard" >/dev/null
jq -e '.editable == false' "$dashboard" >/dev/null
jq -e '(.tags // []) | index("public") and index("cloud-monitor")' "$dashboard" >/dev/null
jq -e '(.templating.list // []) | length == 0' "$dashboard" >/dev/null
jq -e '.refresh | test("^[0-9]+m$") and (. | sub("m$"; "") | tonumber) >= 10' "$dashboard" >/dev/null
jq -e '.time.from == "now-24h"' "$dashboard" >/dev/null
jq -e 'all(.panels[]; .datasource.type == "postgres" and .datasource.uid == "cloud-monitor-postgres")' "$dashboard" >/dev/null
jq -e 'all(.panels[].targets[]?; .datasource.type == "postgres" and .datasource.uid == "cloud-monitor-postgres")' "$dashboard" >/dev/null
jq -e 'all(.panels[].targets[]?; (.rawSql | contains("public_grafana_metric_")))' "$dashboard" >/dev/null

dashboard_sql="$(jq -r '.panels[].targets[]?.rawSql // empty' "$dashboard")"
if printf '%s\n' "$dashboard_sql" | grep -E '\$\{|(^|[^A-Za-z0-9_])metric_points([^A-Za-z0-9_]|$)|metric_definitions|(^|[^A-Za-z0-9_])resources([^A-Za-z0-9_]|$)|resource_id|account_id|arn|tags|sanitized_error|dimensions|namespace|AWS/' >/dev/null; then
  echo "Public Grafana dashboard must query public-safe views only." >&2
  exit 1
fi

grep -q 'Cloud Monitor Public' "$provider"
grep -q 'allowUiUpdates: false' "$provider"
grep -q '/var/lib/grafana/dashboards/cloud-monitor-public' "$provider"
grep -q './grafana/public-dashboards:/var/lib/grafana/dashboards/cloud-monitor-public:ro' docker-compose.yml

if grep -nE 'HandleFunc\("/public/overview"|HandleFunc\("/api/public/metrics"|root.Handle\("/public/overview"|root.Handle\("/api/public/metrics"' internal/admin/admin.go >/dev/null; then
  echo "Admin UI must not expose legacy public portfolio API/UI routes." >&2
  exit 1
fi

if grep -nE 'fetch\("/api/public/metrics"|Cloud Monitor Portfolio|publicPageTemplate' internal/admin/admin.go >/dev/null; then
  echo "Legacy public portfolio UI implementation remains in Admin UI." >&2
  exit 1
fi

grep -q 'public_grafana_metric_points' README.md
grep -q 'grafana/public-dashboards/cloud-monitor-public-overview.json' README.md
grep -q 'Grafana Public Dashboard' README.md

GOCACHE="${GOCACHE:-$(pwd)/.cache/go-build}" go test ./internal/admin ./internal/store

sh scripts/validate-common.sh

echo "step26 validation passed"
