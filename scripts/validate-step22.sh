#!/bin/sh
set -eu

migration="db/migrations/004_product_state.sql"

test -f "$migration"

grep -q 'ADD COLUMN IF NOT EXISTS discovery_source' "$migration"
grep -q 'ADD COLUMN IF NOT EXISTS arn' "$migration"
grep -q 'ADD COLUMN IF NOT EXISTS account_id' "$migration"
grep -q 'ADD COLUMN IF NOT EXISTS internal_region_label' "$migration"
grep -q 'public_enabled BOOLEAN NOT NULL DEFAULT FALSE' "$migration"
grep -q 'public_display_name TEXT NOT NULL DEFAULT' "$migration"
grep -q 'public_description TEXT NOT NULL DEFAULT' "$migration"
grep -q 'public_label TEXT NOT NULL DEFAULT' "$migration"
grep -q 'public_sort_order INTEGER NOT NULL DEFAULT 0' "$migration"
grep -q 'CREATE TABLE IF NOT EXISTS metric_collection_status' "$migration"
grep -q 'last_success_at TIMESTAMPTZ' "$migration"
grep -q 'last_failure_at TIMESTAMPTZ' "$migration"
grep -q 'latest_point_at TIMESTAMPTZ' "$migration"
grep -q 'recent_point_count BIGINT NOT NULL DEFAULT 0' "$migration"
grep -q 'sanitized_error TEXT NOT NULL DEFAULT' "$migration"
grep -q 'CREATE OR REPLACE VIEW public_portfolio_metric_view' "$migration"

public_view_columns="$(sed -n '/CREATE OR REPLACE VIEW public_portfolio_metric_view/,/FROM resources r/p' "$migration")"
if printf '%s\n' "$public_view_columns" | grep -E '\b(resource_id|account_id|arn|tags|sanitized_error|region)\b' >/dev/null; then
  echo "public portfolio view exposes an internal identifier or diagnostic column" >&2
  exit 1
fi

grep -q 'availability_status' db/migrations/002_discovery.sql
grep -q "availability_status IN ('available', 'not_seen', 'requires_setup', 'unsupported')" db/migrations/002_discovery.sql

grep -q 'RecordMetricCollectionSuccess' internal/store/store.go
grep -q 'RecordMetricCollectionFailure' internal/store/store.go
grep -q 'metric_collection_status' internal/store/store.go
grep -q 'RecordMetricCollectionSuccess' internal/collector/collector.go
grep -q 'RecordMetricCollectionFailure' internal/collector/collector.go
grep -q 'sanitize.Message' internal/collector/collector.go

sh scripts/validate-migrations.sh

GOCACHE="${GOCACHE:-$(pwd)/.cache/go-build}" go test ./internal/store ./internal/collector ./internal/admin

sh scripts/validate-common.sh

echo "step22 validation passed"
