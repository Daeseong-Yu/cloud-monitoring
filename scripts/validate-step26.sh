#!/bin/sh
set -eu

grep -q 'ListPublicMetrics' internal/store/store.go
grep -q 'ListPublicMetricSeries' internal/store/store.go
grep -q 'PublicMetricID' internal/store/store.go
grep -q 'base64.RawURLEncoding' internal/store/store.go
grep -q 'r.public_enabled = TRUE' internal/store/store.go
grep -q 'md.public_enabled = TRUE' internal/store/store.go
grep -q 'md.enabled = TRUE' internal/store/store.go

grep -q 'HandleFunc("/public/overview"' internal/admin/admin.go
grep -q 'HandleFunc("/api/public/metrics"' internal/admin/admin.go
grep -q 'HandleFunc("/api/public/metrics/"' internal/admin/admin.go
grep -q 'handleAPIPublicMetricSeries' internal/admin/admin.go
grep -q 'root.Handle("/", s.basicAuth(adminMux))' internal/admin/admin.go

grep -q 'TestPublicMetricsDoNotRequireBasicAuth' internal/admin/admin_test.go
grep -q 'TestPublicMetricSeriesIsReadOnly' internal/admin/admin_test.go
grep -q 'TestPublicMetricIDUsesPublicAliasesOnly' internal/store/store_test.go

grep -q 'GET /api/public/metrics' README.md
grep -q 'raw resource id, AWS account id, full ARN, raw tags, credential, raw collector error' README.md

if grep -nE 'json:"(resourceId|accountId|arn|tags|sanitizedError)"' internal/store/store.go | grep -E 'PublicMetric|PublicMetricSeries' >/dev/null; then
  echo "Public response types must not expose internal identifiers or raw diagnostics." >&2
  exit 1
fi

if grep -nE '/api/public/metrics|/public/overview' internal/admin/admin.go | grep -q 'Set|Update|Delete|Upsert|Apply'; then
  echo "Public API handlers must stay read-only." >&2
  exit 1
fi

GOCACHE="${GOCACHE:-$(pwd)/.cache/go-build}" go test ./internal/store ./internal/admin

sh scripts/validate-common.sh

echo "step26 validation passed"
