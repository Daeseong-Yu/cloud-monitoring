#!/bin/sh
set -eu

dashboard="grafana/public-dashboards/cloud-monitor-public-overview.json"
migration="db/migrations/006_public_grafana_dashboard.sql"
provider="grafana/provisioning/dashboards/cloud-monitor.yaml"
secret_targets="README.md Architecture.md Installation.md .env.example aws configs db scripts internal cmd grafana deploy docker-compose.yml Dockerfile .dockerignore .github"

fail() {
  echo "$1" >&2
  exit 1
}

test -f "$dashboard"
test -f "$migration"
test -f "$provider"

sh scripts/validate-step26.sh

jq -e '.uid == "cloud-monitor-public-overview"' "$dashboard" >/dev/null
jq -e '.title == "Cloud Monitor Public Overview"' "$dashboard" >/dev/null
jq -e '.editable == false' "$dashboard" >/dev/null
jq -e '(.tags // []) | index("public") and index("cloud-monitor")' "$dashboard" >/dev/null
jq -e '(.templating.list // []) | length == 0' "$dashboard" >/dev/null
jq -e '(.links // []) | length == 0' "$dashboard" >/dev/null
jq -e '.refresh | test("^[0-9]+m$") and (. | sub("m$"; "") | tonumber) >= 10' "$dashboard" >/dev/null
jq -e '.time.from == "now-24h" and .time.to == "now"' "$dashboard" >/dev/null
jq -e '[
  .panels[]?.datasource,
  .panels[]?.targets[]?.datasource
] | map(select(. != null)) | all(.type == "postgres" and .uid == "cloud-monitor-postgres")' "$dashboard" >/dev/null
jq -e 'all(.panels[]?.targets[]?; (.rawSql // "") | test("public_grafana_metric_(points|summary)"))' "$dashboard" >/dev/null

dashboard_sql="$(jq -r '.panels[]?.targets[]?.rawSql // empty' "$dashboard")"
if [ -z "$dashboard_sql" ]; then
  fail "Public Grafana dashboard must contain SQL targets."
fi

if printf '%s\n' "$dashboard_sql" | grep -E '\$\{|public_grafana_default_metric_catalog|(^|[^A-Za-z0-9_])(resources|metric_definitions|metric_points|metric_collection_status|discovered_metrics|collection_runs)([^A-Za-z0-9_]|$)|resource_id|account_id|arn|tags|sanitized_error|collector_error|dimensions|namespace|AWS/|CloudWatch|Logs' >/dev/null; then
  fail "Public Grafana dashboard SQL must query public-safe views only."
fi

summary_select="$(sed -n '/CREATE OR REPLACE VIEW public_grafana_metric_summary/,/FROM resources r/p' "$migration")"
points_select="$(sed -n '/CREATE OR REPLACE VIEW public_grafana_metric_points/,/FROM metric_points mp/p' "$migration")"

if printf '%s\n%s\n' "$summary_select" "$points_select" | grep -E '(^|[^A-Za-z0-9_])(resource_id|account_id|arn|tags|sanitized_error|collector_error|dimensions|namespace)([^A-Za-z0-9_]|$)' >/dev/null; then
  fail "Public Grafana view output must not expose raw identifiers, raw tags, namespaces, or diagnostics."
fi

grep -q 'r.public_enabled = TRUE' "$migration"
grep -q 'r.enabled = TRUE' "$migration"
grep -q 'md.public_enabled = TRUE' "$migration"
grep -q 'md.enabled = TRUE' "$migration"
grep -q "r.public_label <> ''" "$migration"
grep -q "md.public_label <> ''" "$migration"
grep -q 'JOIN public_grafana_default_metric_catalog catalog' "$migration"

grep -q 'Cloud Monitor Public' "$provider"
grep -q 'allowUiUpdates: false' "$provider"
grep -q '/var/lib/grafana/dashboards/cloud-monitor-public' "$provider"
grep -q './grafana/public-dashboards:/var/lib/grafana/dashboards/cloud-monitor-public:ro' docker-compose.yml

if grep -nE 'publicPageTemplate|Cloud Monitor Portfolio|/api/public/metrics|/public/overview|ListPublicMetrics|ListPublicMetricSeries|PublicMetricSeriesPoint|PublicMetricID' internal/admin/admin.go internal/store/store.go >/dev/null; then
  fail "Legacy public portfolio API/UI implementation must not remain in Admin UI or store code."
fi

if grep -nE 'GET /public/overview|GET /api/public/metrics|/api/public/metrics|/public/overview|Cloud Monitor Portfolio|Public Portfolio|제품 API/UI surface' README.md Architecture.md Installation.md >/dev/null; then
  fail "Documentation must describe Grafana Public Dashboard, not the legacy public API/UI surface."
fi

grep -q 'Grafana Public Dashboard' README.md
grep -q 'public_grafana_metric_points' README.md
grep -q 'public_grafana_metric_summary' README.md
grep -q 'grafana/public-dashboards/cloud-monitor-public-overview.json' README.md
grep -q 'sh scripts/validate-step27.sh' README.md
grep -q 'RUN_GITLEAKS=1 sh scripts/scan-secrets.sh' README.md
grep -q '공유 링크' README.md
grep -q 'tracked file에 기록하지 않습니다' README.md

grep -q 'Grafana Public Dashboard' Architecture.md
grep -q 'public-safe view' Architecture.md
grep -q '공유 링크' Architecture.md

grep -q 'Grafana Public Dashboard' Installation.md
grep -q 'sh scripts/validate-step27.sh' Installation.md
grep -q 'RUN_GITLEAKS=1 sh scripts/scan-secrets.sh' Installation.md
grep -q '공유 링크' Installation.md

tmp_urls="$(mktemp)"
trap 'rm -f "$tmp_urls"' EXIT

grep -RInE 'https?://[^[:space:]")<>]+' README.md Architecture.md Installation.md grafana/public-dashboards grafana/provisioning/dashboards docker-compose.yml >"$tmp_urls" 2>/dev/null || true
if grep -Ev '127\.0\.0\.1|localhost|SERVER_ADDRESS|github\.com/Daeseong-Yu/cloud-monitoring\.git' "$tmp_urls" >/dev/null; then
  cat "$tmp_urls" >&2
  fail "Tracked public dashboard boundary files must not contain real public URLs, domains, or server IPs."
fi

if grep -RInE 'shareToken|publicDashboardAccessToken|accessToken|snapshotUrl|https?://[^[:space:]")<>]*(/public-dashboards|/dashboard/snapshot)/[A-Za-z0-9_-]{6,}' README.md Architecture.md Installation.md grafana/public-dashboards grafana/provisioning/dashboards docker-compose.yml >/dev/null; then
  fail "Tracked files must not contain Grafana public share tokens or generated public dashboard URLs."
fi

GOCACHE="${GOCACHE:-$(pwd)/.cache/go-build}" go test ./internal/admin ./internal/store

RUN_GITLEAKS="${RUN_GITLEAKS:-1}" sh scripts/scan-secrets.sh $secret_targets

sh scripts/validate-common.sh

echo "step27 validation passed"
