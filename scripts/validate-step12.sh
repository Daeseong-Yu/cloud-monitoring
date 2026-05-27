#!/bin/sh
set -eu

test -f db/migrations/003_alert_summary.sql
test -f cmd/alert-runner/main.go
test -f cmd/summary-rollup/main.go
test -f internal/alert/alert.go
test -f internal/summary/summary.go
test -f scripts/verify-db-health.sh
test -f .github/workflows/ci.yml

grep -q 'CREATE TABLE IF NOT EXISTS alert_rules' db/migrations/003_alert_summary.sql
grep -q 'CREATE TABLE IF NOT EXISTS alert_events' db/migrations/003_alert_summary.sql
grep -q 'CREATE TABLE IF NOT EXISTS metric_hourly_summary' db/migrations/003_alert_summary.sql
grep -q 'CREATE TABLE IF NOT EXISTS metric_daily_summary' db/migrations/003_alert_summary.sql
grep -q 'metric_hourly_summary_unique' db/migrations/003_alert_summary.sql
grep -q 'metric_daily_summary_unique' db/migrations/003_alert_summary.sql
grep -q 'idx_alert_events_one_open' db/migrations/003_alert_summary.sql

grep -q 'SLACK_WEBHOOK_URL' cmd/alert-runner/main.go
grep -q 'json.Marshal' cmd/alert-runner/main.go
grep -q 'sanitize.Message' cmd/alert-runner/main.go
grep -q 'status = '\''resolved'\''' cmd/alert-runner/main.go
grep -q 'ON CONFLICT DO NOTHING' cmd/alert-runner/main.go

grep -q 'ON CONFLICT ON CONSTRAINT metric_hourly_summary_unique DO UPDATE' cmd/summary-rollup/main.go
grep -q 'ON CONFLICT ON CONSTRAINT metric_daily_summary_unique DO UPDATE' cmd/summary-rollup/main.go
grep -q 'pg_database_size' scripts/verify-db-health.sh
grep -q 'pg_total_relation_size' scripts/verify-db-health.sh
grep -q 'gitleaks/gitleaks-action' .github/workflows/ci.yml
grep -q 'go test ./...' .github/workflows/ci.yml
grep -q 'validate-step12.sh' .github/workflows/ci.yml
grep -q 'go build -o /out/alert-runner' Dockerfile
grep -q 'go build -o /out/summary-rollup' Dockerfile

if grep -R -E 'https://hooks\.slack\.com/services/[A-Za-z0-9/]+' .github cmd internal scripts db docs README.md .env.example >/dev/null 2>&1; then
  echo "Slack webhook URL appears to be committed." >&2
  exit 1
fi

sh scripts/validate-common.sh
RUN_GITLEAKS=1 sh scripts/scan-secrets.sh

echo "step12 validation passed"
