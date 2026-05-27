#!/bin/sh
set -eu

compose_profiles="--profile setup --profile discovery --profile collector --profile admin-ui --profile jobs"
grafana_url="${GRAFANA_URL:-http://127.0.0.1:${GRAFANA_PORT:-3000}}"
admin_url="${ADMIN_URL:-http://127.0.0.1:${ADMIN_HTTP_PORT:-8080}}"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 command not found" >&2
    exit 1
  fi
}

has_aws_credentials() {
  if [ -n "${AWS_PROFILE:-}" ]; then
    return 0
  fi

  if [ -n "${AWS_ACCESS_KEY_ID:-}" ] && [ -n "${AWS_SECRET_ACCESS_KEY:-}" ]; then
    return 0
  fi

  return 1
}

wait_for_service_health() {
  service="$1"
  attempts="${2:-60}"

  container_id="$(docker compose ps -q "$service")"
  if [ -z "$container_id" ]; then
    echo "$service container is not running" >&2
    exit 1
  fi

  i=1
  while [ "$i" -le "$attempts" ]; do
    status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container_id")"
    if [ "$status" = "healthy" ] || [ "$status" = "running" ]; then
      echo "$service health check passed"
      return
    fi
    sleep 2
    i=$((i + 1))
  done

  echo "$service did not become healthy" >&2
  exit 1
}

check_admin_unauthenticated() {
  attempts="${1:-30}"
  i=1

  while [ "$i" -le "$attempts" ]; do
    status_code="$(curl -sS -o /dev/null -w '%{http_code}' "$admin_url/" 2>/dev/null || true)"
    if [ "$status_code" = "401" ]; then
      echo "admin unauthenticated 401 check passed"
      return
    fi
    sleep 2
    i=$((i + 1))
  done

  echo "Admin UI unauthenticated check failed: expected 401" >&2
  exit 1
}

require_command docker
require_command curl

sh scripts/validate-production-env.sh

# shellcheck disable=SC2086
docker compose $compose_profiles config >/dev/null

docker compose up -d --build postgres grafana admin-ui

wait_for_service_health postgres 60
wait_for_service_health grafana 60

docker compose --profile setup run --rm schema
docker compose --profile setup run --rm metricdefs-sync /app/metricdefs-sync -config /app/configs/metric-definitions.example.json -dry-run >/dev/null

GRAFANA_URL="$grafana_url" sh scripts/verify-grafana-health.sh
check_admin_unauthenticated

if [ -n "${DATABASE_URL:-}" ]; then
  sh scripts/verify-db-health.sh
else
  echo "DATABASE_URL is not set; skipping host database health check"
fi

if has_aws_credentials; then
  docker compose --profile discovery run --rm resource-discovery /app/resource-discovery -dry-run >/dev/null
  docker compose --profile collector run --rm collector /app/collector --once
else
  echo "AWS credentials are not set; running infra-only mode"
fi

docker compose --profile jobs run --rm -e SLACK_WEBHOOK_URL= alert-runner
docker compose --profile jobs run --rm summary-rollup
docker compose --profile jobs run --rm retention-job

echo "compose smoke test passed"
