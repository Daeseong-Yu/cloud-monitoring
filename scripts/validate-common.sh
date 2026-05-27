#!/bin/sh
set -eu

go_cache="${GOCACHE:-/Users/09mac/project/cloud-monitor/.cache/go-build}"

jq . \
  .ai/phases/index.json \
  .ai/phases/cloud-monitor/index.json \
  configs/metric-definitions.example.json \
  aws/iam/collector-readonly-policy.json \
  grafana/dashboards/cloud-monitor-ec2-mvp.json >/dev/null

GOCACHE="$go_cache" /usr/local/go/bin/go test ./...

sh scripts/scan-secrets.sh .ai README.md .env.example aws configs db scripts internal cmd grafana docs

echo "common validation passed"
