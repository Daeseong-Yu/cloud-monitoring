#!/bin/sh
set -eu

test -f db/migrations/005_collection_diagnostics.sql
test -f docs/operations/cloudwatch-cost-guardrail.md

grep -q 'fetched_point_count BIGINT NOT NULL DEFAULT 0' db/migrations/005_collection_diagnostics.sql
grep -q 'inserted_point_count BIGINT NOT NULL DEFAULT 0' db/migrations/005_collection_diagnostics.sql
grep -q 'last_run_status TEXT NOT NULL DEFAULT' db/migrations/005_collection_diagnostics.sql
grep -q "last_run_status IN ('unknown', 'success', 'failure')" db/migrations/005_collection_diagnostics.sql

grep -q 'InsertMetricPointsDetailed' internal/store/store.go
grep -q 'FetchedPointCount' internal/store/store.go
grep -q 'InsertedPointCount' internal/store/store.go
grep -q 'CollectionCostEstimate' internal/store/store.go
grep -q 'GetMetricDataPricePerThousand' internal/store/store.go
grep -q 'EstimatedMonthlyCostUSD' internal/store/store.go
grep -q 'CostWarningMetricCount' internal/store/store.go

grep -q 'FailedDefinitionIDs' internal/cloudwatchmetrics/cloudwatchmetrics.go
grep -q 'recordPartialFailures' internal/collector/collector.go
grep -q 'sanitize.Message' internal/collector/collector.go
grep -q 'FetchedPointCount' internal/collector/collector.go
grep -q 'InsertedPointCount' internal/collector/collector.go

grep -q 'HandleFunc("/api/cost-estimate"' internal/admin/admin.go
grep -q 'Estimated GetMetricData' internal/admin/admin.go
grep -q 'SanitizedError' internal/admin/admin.go
grep -q 'CostWarningMetricCount' internal/admin/admin.go

grep -q '2026-05-28' docs/operations/cloudwatch-cost-guardrail.md
grep -q '0.01 per 1,000 metrics requested' docs/operations/cloudwatch-cost-guardrail.md
grep -q 'does not call AWS Billing or Pricing APIs' docs/operations/cloudwatch-cost-guardrail.md

if grep -RInE --exclude validate-step24.sh 'billing:Get|pricing:Get|ce:Get|budgets:' internal cmd scripts aws db >/dev/null; then
  echo "Step 24 must not add billing, pricing, cost explorer, or budgets API calls." >&2
  exit 1
fi

if grep -RInE 'COLLECTOR_INTERVAL_SECONDS.*[1-5][0-9][^0-9]' internal cmd scripts docker-compose.yml .env.example >/dev/null; then
  echo "Collector interval must not be lowered below 60 seconds." >&2
  exit 1
fi

GOCACHE="${GOCACHE:-$(pwd)/.cache/go-build}" go test ./internal/store ./internal/admin ./internal/collector ./internal/cloudwatchmetrics ./cmd/admin-server

sh scripts/validate-common.sh

echo "step24 validation passed"
