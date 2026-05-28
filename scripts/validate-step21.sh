#!/bin/sh
set -eu

test -f internal/discovery/provider.go
test -f internal/discovery/provider_test.go
test -f configs/product-metric-catalog.json
test -f aws/iam/discovery-readonly-policy.json

grep -q 'type Provider interface' internal/discovery/provider.go
grep -q 'type Registry struct' internal/discovery/provider.go
grep -q 'AvailabilityChecker' internal/discovery/provider.go
grep -q 'ec2-provider' internal/discovery/provider.go
grep -q 'lambda-provider' internal/discovery/provider.go
grep -q 'api-gateway-provider' internal/discovery/provider.go
grep -q 'amplify-provider' internal/discovery/provider.go
grep -q 'ses-provider' internal/discovery/provider.go
grep -q 's3-provider' internal/discovery/provider.go

grep -q 'product-metric-catalog.json' cmd/resource-discovery/main.go
grep -q 'DefaultRegistry' cmd/resource-discovery/main.go
grep -q 'availability_status' db/migrations/002_discovery.sql
grep -q 'provider_source' db/migrations/002_discovery.sql
grep -q 'requires_setup' db/migrations/002_discovery.sql
grep -q 'unsupported' db/migrations/002_discovery.sql
grep -q 'ListAdminMetricCandidates' internal/store/store.go
grep -q '/api/metric-candidates' internal/admin/admin.go

jq -e '
  .Version == "2012-10-17"
  and ([
    .Statement[]
    | select(.Effect == "Allow")
    | .Action
  ] | flatten | sort) == ([
    "cloudwatch:ListMetrics",
    "tag:GetResources",
    "tag:GetTagKeys",
    "tag:GetTagValues"
  ] | sort)
' aws/iam/discovery-readonly-policy.json >/dev/null

if jq -r '.Statement[].Action[]?' aws/iam/discovery-readonly-policy.json | grep -E ':(Put|Create|Delete|Update|Terminate|Start|Stop|Reboot|Run|Attach|Detach|Modify|Assume|Pass)' >/dev/null; then
  echo "Discovery policy contains a non-read action." >&2
  exit 1
fi

GOCACHE="${GOCACHE:-$(pwd)/.cache/go-build}" go test ./internal/discovery ./internal/store ./internal/admin ./cmd/resource-discovery

sh scripts/validate-common.sh

echo "step21 validation passed"
