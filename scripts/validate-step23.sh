#!/bin/sh
set -eu

test -f internal/admin/admin.go
test -f internal/admin/admin_test.go
test -f internal/store/store.go

grep -q 'HandleFunc("/api/services"' internal/admin/admin.go
grep -q 'HandleFunc("/api/resources"' internal/admin/admin.go
grep -q 'HandleFunc("/api/resources/bulk-enabled"' internal/admin/admin.go
grep -q 'HandleFunc("/api/metric-candidates"' internal/admin/admin.go
grep -q 'HandleFunc("/api/metric-definitions"' internal/admin/admin.go
grep -q 'enableResourceMonitoring' internal/admin/admin.go
grep -q 'enableServiceMonitoring' internal/admin/admin.go
grep -q 'UpdateResourcePublicMetadata' internal/admin/admin.go
grep -q 'Advanced manual metric definition' internal/admin/admin.go
grep -q 'Metric candidate diagnostics' internal/admin/admin.go
grep -q 'Metric definition diagnostics' internal/admin/admin.go
grep -q '서비스 모니터링 시작' internal/admin/admin.go
grep -q '기본 metric이 자동 적용' internal/admin/admin.go
grep -q 'AvailabilityReason' internal/admin/admin.go
grep -q 'CostWarning' internal/admin/admin.go
grep -q 'public_enabled' internal/admin/admin.go

grep -q 'ListAdminServices' internal/store/store.go
grep -q 'GetAdminResource' internal/store/store.go
grep -q 'SetResourcesEnabled' internal/store/store.go
grep -q 'ApplyRecommendedMetricSet' internal/store/store.go
grep -q 'UpdateResourcePublicMetadata' internal/store/store.go
grep -q 'public_label is required when public_enabled is true' internal/store/store.go

grep -q 'TestAdminMonitoringFlowUsesServiceResourceSelection' internal/admin/admin_test.go
grep -q 'TestAPIResourcesBulkEnableAppliesDefaultMetricSet' internal/admin/admin_test.go

if grep -q '추천 세트 일괄 적용' internal/admin/admin.go; then
  echo "Admin default flow must not require a separate recommended metric set action." >&2
  exit 1
fi

if grep -q '/admin/metric-candidates/{{.ID}}/select' internal/admin/admin.go; then
  echo "Admin default flow must not expose per-metric candidate selection buttons." >&2
  exit 1
fi

if grep -q 'Advanced public metric metadata' internal/admin/admin.go; then
  echo "Public visibility should be managed at the resource flow, not per metric in the default UI." >&2
  exit 1
fi

if grep -E 'HandleFunc\("/admin|HandleFunc\("/api' internal/admin/admin.go | grep -v 'basicAuth' >/dev/null; then
  grep -Eq 'return s.basicAuth\(mux\)|root.Handle\("/", s.basicAuth\(adminMux\)\)' internal/admin/admin.go
fi

api_handlers="$(sed -n '/func (s \*Server) handleAPI/,/func (s \*Server) requestRegion/p' internal/admin/admin.go)"
if printf '%s\n' "$api_handlers" | grep -E '\b(arn|account_id|accountId|secret|credential|password)\b' >/dev/null; then
  echo "Admin API handler appears to expose sensitive fields." >&2
  exit 1
fi

GOCACHE="${GOCACHE:-$(pwd)/.cache/go-build}" go test ./internal/admin ./internal/store

sh scripts/validate-common.sh

echo "step23 validation passed"
