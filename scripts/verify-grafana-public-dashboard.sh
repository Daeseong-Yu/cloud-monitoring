#!/bin/sh
set -eu

grafana_url="${GRAFANA_URL:-http://127.0.0.1:${GRAFANA_PORT:-3000}}"
admin_url="${ADMIN_URL:-http://127.0.0.1:${ADMIN_HTTP_PORT:-8080}}"
dashboard_uid="${GRAFANA_PUBLIC_DASHBOARD_UID:-cloud-monitor-public-overview}"

fail() {
  echo "$1" >&2
  exit 1
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "$1 command not found"
  fi
}

grafana_api() {
  path="$1"

  if [ -n "${GRAFANA_ADMIN_USER:-}" ] && [ -n "${GRAFANA_ADMIN_PASSWORD:-}" ]; then
    curl -fsS -u "$GRAFANA_ADMIN_USER:$GRAFANA_ADMIN_PASSWORD" "$grafana_url$path"
    return
  fi

  curl -fsS "$grafana_url$path"
}

require_command curl
require_command jq

curl -fsS "$grafana_url/api/health" >/dev/null

admin_status="$(curl -sS -o /dev/null -w '%{http_code}' "$admin_url/" 2>/dev/null || true)"
if [ "$admin_status" != "401" ]; then
  fail "Admin UI unauthenticated check failed: expected 401"
fi

if [ -n "${GRAFANA_ADMIN_USER:-}" ] && [ -n "${GRAFANA_ADMIN_PASSWORD:-}" ]; then
  dashboard_json="$(grafana_api "/api/dashboards/uid/$dashboard_uid")"

  printf '%s\n' "$dashboard_json" | jq -e --arg uid "$dashboard_uid" '.dashboard.uid == $uid' >/dev/null
  printf '%s\n' "$dashboard_json" | jq -e '.dashboard.editable == false' >/dev/null
  printf '%s\n' "$dashboard_json" | jq -e '(.dashboard.templating.list // []) | length == 0' >/dev/null
  printf '%s\n' "$dashboard_json" | jq -e '[
    .dashboard.panels[]?.datasource,
    .dashboard.panels[]?.targets[]?.datasource
  ] | map(select(. != null)) | all(.type == "postgres" and .uid == "cloud-monitor-postgres")' >/dev/null
  printf '%s\n' "$dashboard_json" | jq -e 'all(.dashboard.panels[]?.targets[]?; (.rawSql // "") | test("public_grafana_metric_(points|summary)"))' >/dev/null

  dashboard_sql="$(printf '%s\n' "$dashboard_json" | jq -r '.dashboard.panels[]?.targets[]?.rawSql // empty')"
  if printf '%s\n' "$dashboard_sql" | grep -E '\$\{|(^|[^A-Za-z0-9_])(resources|metric_definitions|metric_points|metric_collection_status|discovered_metrics|collection_runs)([^A-Za-z0-9_]|$)|resource_id|account_id|arn|tags|sanitized_error|collector_error|dimensions|namespace|AWS/|CloudWatch|Logs' >/dev/null; then
    fail "Public Grafana dashboard SQL must query public-safe views only."
  fi
else
  echo "Grafana admin credentials are not set; skipping provisioned dashboard API check"
fi

if [ -n "${DATABASE_URL:-}" ]; then
  if ! command -v psql >/dev/null 2>&1; then
    fail "psql command not found"
  fi

  unsafe_columns="$(psql --set ON_ERROR_STOP=1 -At "$DATABASE_URL" <<'SQL'
SELECT column_name
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name IN ('public_grafana_metric_summary', 'public_grafana_metric_points')
  AND column_name IN (
    'resource_id',
    'account_id',
    'arn',
    'tags',
    'sanitized_error',
    'collector_error',
    'dimensions',
    'namespace'
  )
ORDER BY column_name;
SQL
)"

  if [ -n "$unsafe_columns" ]; then
    printf '%s\n' "$unsafe_columns" >&2
    fail "Public Grafana views expose unsafe columns."
  fi

  psql --set ON_ERROR_STOP=1 "$DATABASE_URL" >/dev/null <<'SQL'
SELECT COUNT(*) FROM public_grafana_metric_summary;
SELECT COUNT(*) FROM public_grafana_metric_points WHERE "time" >= now() - interval '24 hours';
SQL
else
  echo "DATABASE_URL is not set; skipping public Grafana view database check"
fi

if [ -n "${GRAFANA_PUBLIC_DASHBOARD_URL:-}" ]; then
  public_page="$(curl -fsSL "$GRAFANA_PUBLIC_DASHBOARD_URL")"
  if printf '%s\n' "$public_page" | grep -E 'resource_id|account_id|arn:[a]ws|sanitized_error|collector_error|/api/public/metrics|/public/overview' >/dev/null; then
    fail "Public Grafana page contains unsafe identifiers or legacy public API references."
  fi
else
  echo "GRAFANA_PUBLIC_DASHBOARD_URL is not set; skipping shared public URL check"
fi

echo "public grafana dashboard runtime verification passed"
