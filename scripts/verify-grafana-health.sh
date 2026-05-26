#!/bin/sh
set -eu

grafana_url="${GRAFANA_URL:-http://127.0.0.1:3000}"

curl -fsS "$grafana_url/api/health" >/dev/null

echo "grafana health check passed"
