#!/bin/sh
set -eu

test -f internal/admin/admin.go
test -f internal/admin/admin_test.go
test -f internal/store/store.go

grep -q 'HandleFunc("/api/services"' internal/admin/admin.go
grep -q 'HandleFunc("/api/resources"' internal/admin/admin.go
grep -q 'HandleFunc("/api/metric-candidates"' internal/admin/admin.go
grep -q 'HandleFunc("/api/metric-definitions"' internal/admin/admin.go
grep -q 'SelectMetricCandidate' internal/admin/admin.go
grep -q 'UpdateResourcePublicMetadata' internal/admin/admin.go
grep -q 'UpdateMetricDefinitionPublicMetadata' internal/admin/admin.go
grep -q 'Advanced manual metric definition' internal/admin/admin.go
grep -q 'AvailabilityReason' internal/admin/admin.go
grep -q 'CostWarning' internal/admin/admin.go
grep -q 'public_enabled' internal/admin/admin.go

grep -q 'ListAdminServices' internal/store/store.go
grep -q 'SelectMetricCandidate' internal/store/store.go
grep -q "metric candidate is not available" internal/store/store.go
grep -q 'UpdateResourcePublicMetadata' internal/store/store.go
grep -q 'UpdateMetricDefinitionPublicMetadata' internal/store/store.go
grep -q 'public_label is required when public_enabled is true' internal/store/store.go

if grep -E 'HandleFunc\("/admin|HandleFunc\("/api' internal/admin/admin.go | grep -v 'basicAuth' >/dev/null; then
  grep -q 'return s.basicAuth(mux)' internal/admin/admin.go
fi

api_handlers="$(sed -n '/func (s \*Server) handleAPI/,/func (s \*Server) requestRegion/p' internal/admin/admin.go)"
if printf '%s\n' "$api_handlers" | grep -E '\b(arn|account_id|accountId|secret|credential|password)\b' >/dev/null; then
  echo "Admin API handler appears to expose sensitive fields." >&2
  exit 1
fi

GOCACHE="${GOCACHE:-$(pwd)/.cache/go-build}" go test ./internal/admin ./internal/store

sh scripts/validate-common.sh

echo "step23 validation passed"
