#!/bin/sh
set -eu

test -f cmd/admin-server/main.go
test -f internal/admin/admin.go
test -f internal/admin/admin_test.go

grep -q 'ADMIN_USERNAME is required' internal/admin/admin.go
grep -q 'ADMIN_PASSWORD is required' internal/admin/admin.go
grep -q 'WWW-Authenticate' internal/admin/admin.go
grep -q '/api/resources' internal/admin/admin.go
grep -q '/api/metric-definitions' internal/admin/admin.go
grep -q 'ApplyRecommendedMetricSet' internal/admin/admin.go
grep -q 'Discovery 실행' internal/admin/admin.go
grep -q 'resource-discovery' cmd/admin-server/main.go
grep -q 'sanitize.Message' cmd/admin-server/main.go
grep -q 'go build -o /out/admin-server' Dockerfile
grep -q 'command: \["/app/admin-server"\]' docker-compose.yml

if grep -R -i 'grafana.*cloudwatch\|cloudwatch.*datasource' cmd/admin-server internal/admin docker-compose.yml >/dev/null; then
  echo "Admin UI 산출물에 CloudWatch datasource 설정 경로가 있습니다." >&2
  exit 1
fi

sh scripts/validate-common.sh

echo "step9 validation passed"
