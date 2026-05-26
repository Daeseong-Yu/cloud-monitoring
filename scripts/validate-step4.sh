#!/bin/sh
set -eu

test -f cmd/collector/main.go
test -f internal/collector/collector.go
test -f internal/cloudwatchmetrics/cloudwatchmetrics.go
test -f internal/store/store.go
test -f internal/sanitize/sanitize.go

grep -q 'flag.Bool("once"' cmd/collector/main.go
grep -q 'GetMetricData' internal/cloudwatchmetrics/cloudwatchmetrics.go
grep -q 'ON CONFLICT ON CONSTRAINT metric_points_unique_point DO NOTHING' internal/store/store.go
grep -q 'enabled = TRUE' internal/store/store.go
grep -q 'AND region = $1' internal/store/store.go
grep -q 'sanitize.Message' cmd/collector/main.go

GOCACHE=/Users/09mac/project/cloud-monitor/.cache/go-build /usr/local/go/bin/go test ./...

sh scripts/scan-secrets.sh .ai README.md .env.example aws configs db scripts internal cmd

echo "step4 validation passed"
