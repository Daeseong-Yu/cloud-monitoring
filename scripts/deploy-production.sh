#!/bin/sh
set -eu

DEPLOY_PATH="${DEPLOY_PATH:-/opt/cloud-monitor}"
DEPLOY_ENV_FILE="${DEPLOY_ENV_FILE:-/etc/cloud-monitor/cloud-monitor.env}"
DEPLOY_REMOTE="${DEPLOY_REMOTE:-origin}"
DEPLOY_REF="${DEPLOY_REF:-main}"
DEPLOY_SHA="${DEPLOY_SHA:-}"

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 command not found" >&2
    exit 1
  fi
}

fail() {
  echo "$1" >&2
  exit 1
}

validate_safe_ref() {
  value="$1"

  case "$value" in
    -*|''|*[!A-Za-z0-9._/@-]*)
      fail "deploy ref contains unsupported characters"
      ;;
  esac
}

validate_safe_sha() {
  value="$1"

  if [ -z "$value" ]; then
    return
  fi

  case "$value" in
    *[!A-Fa-f0-9]*)
      fail "deploy sha must be hexadecimal"
      ;;
  esac
}

resolve_target_sha() {
  sha=""

  if [ -n "$DEPLOY_SHA" ]; then
    if sha="$(git rev-parse --verify "$DEPLOY_SHA^{commit}" 2>/dev/null)"; then
      printf '%s\n' "$sha"
      return
    fi
    fail "deploy sha was not found in repository"
  fi

  if sha="$(git rev-parse --verify "$DEPLOY_REF^{commit}" 2>/dev/null)"; then
    printf '%s\n' "$sha"
    return
  fi

  if sha="$(git rev-parse --verify "$DEPLOY_REMOTE/$DEPLOY_REF^{commit}" 2>/dev/null)"; then
    printf '%s\n' "$sha"
    return
  fi

  if sha="$(git rev-parse --verify "refs/tags/$DEPLOY_REF^{commit}" 2>/dev/null)"; then
    printf '%s\n' "$sha"
    return
  fi

  fail "deploy ref was not found in repository"
}

load_runtime_env() {
  if [ ! -r "$DEPLOY_ENV_FILE" ]; then
    fail "runtime env file is not readable: $DEPLOY_ENV_FILE"
  fi

  set -a
  # shellcheck disable=SC1090
  . "$DEPLOY_ENV_FILE"
  set +a
}

wait_for_service_health() {
  service="$1"
  attempts="${2:-60}"

  container_id="$(docker compose ps -q "$service")"
  if [ -z "$container_id" ]; then
    fail "$service container is not running"
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

  fail "$service did not become healthy"
}

check_admin_unauthenticated() {
  admin_url="${ADMIN_URL:-http://127.0.0.1:${ADMIN_HTTP_PORT:-8080}}"
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

  fail "Admin UI unauthenticated check failed: expected 401"
}

record_deployment() {
  target_sha="$1"
  previous_sha="$2"
  deploy_dir="$DEPLOY_PATH/.deploy"
  timestamp="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

  mkdir -p "$deploy_dir"

  if [ -n "$previous_sha" ] && [ "$previous_sha" != "$target_sha" ]; then
    printf '%s\n' "$previous_sha" >"$deploy_dir/previous"
  fi

  printf '%s\n' "$target_sha" >"$deploy_dir/current"
  printf '%s commit_sha=%s ref=%s status=success\n' "$timestamp" "$target_sha" "$DEPLOY_REF" >>"$deploy_dir/releases.log"
}

require_command git
require_command docker
require_command curl
require_command date

validate_safe_ref "$DEPLOY_REF"
validate_safe_sha "$DEPLOY_SHA"

cd "$DEPLOY_PATH"

deploy_dir="$DEPLOY_PATH/.deploy"
mkdir -p "$deploy_dir"

lock_dir="$deploy_dir/lock"
if ! mkdir "$lock_dir" 2>/dev/null; then
  fail "another deployment is already running"
fi
trap 'rmdir "$lock_dir"' EXIT

previous_sha="$(git rev-parse --verify HEAD 2>/dev/null || true)"

git fetch --prune "$DEPLOY_REMOTE" "+refs/heads/*:refs/remotes/$DEPLOY_REMOTE/*" "+refs/tags/*:refs/tags/*"

target_sha="$(resolve_target_sha)"
echo "deploying commit $target_sha"

git checkout --force "$target_sha"

load_runtime_env

sh scripts/validate-production-env.sh

docker compose \
  --profile setup \
  --profile discovery \
  --profile collector \
  --profile admin-ui \
  --profile jobs \
  config >/dev/null

docker compose up -d --build postgres grafana admin-ui
wait_for_service_health postgres 60
wait_for_service_health grafana 60

docker compose --profile setup run --rm schema
docker compose --profile collector up -d --build collector

GRAFANA_URL="${GRAFANA_URL:-http://127.0.0.1:${GRAFANA_PORT:-3000}}" sh scripts/verify-grafana-health.sh

if [ -n "${DATABASE_URL:-}" ]; then
  sh scripts/verify-db-health.sh
else
  echo "DATABASE_URL is not set; skipping host database health check"
fi

check_admin_unauthenticated
record_deployment "$target_sha" "$previous_sha"

echo "production deployment completed"
