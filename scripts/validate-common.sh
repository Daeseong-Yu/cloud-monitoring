#!/bin/sh
set -eu

go_cache="${GOCACHE:-$(pwd)/.cache/go-build}"
go_bin="${GO_BIN:-go}"

json_files="
  configs/metric-definitions.example.json
  configs/recommended-metric-sets.json
  configs/product-metric-catalog.json
  aws/iam/collector-readonly-policy.json
  aws/iam/discovery-readonly-policy.json
  grafana/dashboards/cloud-monitor-ec2-mvp.json
  grafana/dashboards/cloud-monitor-lambda.json
  grafana/dashboards/cloud-monitor-aws-resource.json
  grafana/dashboards/cloud-monitor-postgres-ops.json
  grafana/dashboards/cloud-monitor-overview.json
  grafana/dashboards/cloud-monitor-api-gateway.json
  grafana/dashboards/cloud-monitor-amplify.json
  grafana/dashboards/cloud-monitor-ses.json
  grafana/dashboards/cloud-monitor-s3.json
  grafana/public-dashboards/cloud-monitor-public-overview.json
"

for optional_json in \
  .ai/phases/index.json \
  .ai/phases/cloud-monitor/index.json \
  .ai/phases/cloud-monitor-post-mvp/index.json \
  .ai/phases/cloud-monitor-production-readiness/index.json \
  .ai/phases/cloud-monitor-productization/index.json; do
  if [ -f "$optional_json" ]; then
    json_files="$json_files $optional_json"
  fi
done

# shellcheck disable=SC2086
jq . $json_files >/dev/null

GOCACHE="$go_cache" "$go_bin" test ./...

scan_targets="README.md .env.example Dockerfile docker-compose.yml .dockerignore .github aws configs db scripts internal cmd grafana docs deploy"
if [ -d .ai ]; then
  scan_targets=".ai $scan_targets"
fi

# shellcheck disable=SC2086
sh scripts/scan-secrets.sh $scan_targets

echo "common validation passed"
