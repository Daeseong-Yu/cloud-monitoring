#!/bin/sh
set -eu

grep -q 'publicPageTemplate' internal/admin/admin.go
grep -q 'Cloud Monitor Portfolio' internal/admin/admin.go
grep -q 'fetch("/api/public/metrics"' internal/admin/admin.go
grep -q 'fetch("/api/public/metrics/"' internal/admin/admin.go
grep -q 'forbiddenKeys = \["resource" + "Id", "account" + "Id", "a" + "rn", "ta" + "gs", "sanitized" + "Error", "name" + "space", "re" + "gion"\]' internal/admin/admin.go
grep -q 'assertPublicPayload' internal/admin/admin.go
grep -q 'TestPublicOverviewUsesPublicAPIOnly' internal/admin/admin_test.go

public_template="$(sed -n '/const publicPageTemplate =/,/`$/p' internal/admin/admin.go)"
if printf '%s\n' "$public_template" | grep -E '/admin|metric-candidates|metric-definitions|Discovery 실행|수집 시작|삭제|Disable|Enable|public_enabled' >/dev/null; then
  echo "Public portfolio view must not expose Admin actions or internal controls." >&2
  exit 1
fi

if printf '%s\n' "$public_template" | grep -E 'resourceId|accountId|full ARN|raw tags|sanitizedError|AWS/' >/dev/null; then
  echo "Public portfolio view must not expose internal identifiers or namespaces." >&2
  exit 1
fi

grep -q 'GET /public/overview' README.md
grep -q '공개 전 확인' README.md
grep -q 'Admin UI는 내부망, VPN, 또는 IP allowlist 뒤에 두고' README.md
grep -q 'Grafana public dashboard를 portfolio surface로 사용하지 않습니다' README.md
grep -q 'validate-productization.sh' README.md

GOCACHE="${GOCACHE:-$(pwd)/.cache/go-build}" go test ./internal/admin ./internal/store

if [ "${SKIP_PRODUCTIZATION_VALIDATION:-0}" != "1" ]; then
  sh scripts/validate-productization.sh
fi

echo "step27 validation passed"
