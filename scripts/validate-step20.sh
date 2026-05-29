#!/bin/sh
set -eu

catalog="configs/product-metric-catalog.json"
recommended="configs/recommended-metric-sets.json"

test -f "$catalog"
test -f "$recommended"
test -f internal/productcatalog/catalog.go
test -f internal/productcatalog/catalog_test.go

jq -e '
  .version == 1
  and (.metrics | length >= 1)
  and ([.metrics[] | select(.serviceName == "ec2")] | length >= 1)
  and ([.metrics[] | select(.serviceName == "lambda")] | length >= 1)
  and ([.metrics[] | select(.serviceName == "api-gateway")] | length >= 1)
  and ([.metrics[] | select(.serviceName == "amplify")] | length >= 1)
  and ([.metrics[] | select(.serviceName == "ses")] | length >= 1)
  and ([.metrics[] | select(.serviceName == "s3")] | length >= 1)
  and all(.metrics[]; (
    (.key | type == "string" and length > 0)
    and (.serviceName | type == "string" and length > 0)
    and (.namespace | type == "string" and length > 0)
    and (.metricName | type == "string" and length > 0)
    and ([.statistic] | inside(["Average", "Sum", "Minimum", "Maximum", "SampleCount"]))
    and ([.periodSeconds] | inside([60, 300, 900, 3600, 86400]))
    and (.requiredDimensions | type == "array")
    and (.recommended | type == "boolean")
    and (.axis | type == "object")
    and (.prerequisite | type == "string")
    and (.costWarning | type == "string")
  ))
' "$catalog" >/dev/null

duplicate_keys="$(jq -r '.metrics[].key' "$catalog" | sort | uniq -d)"
if [ -n "$duplicate_keys" ]; then
  echo "Duplicate product metric catalog keys:" >&2
  echo "$duplicate_keys" >&2
  exit 1
fi

duplicate_definitions="$(jq -r '
  .metrics[]
  | [
      .serviceName,
      .namespace,
      .metricName,
      .statistic,
      (.periodSeconds | tostring),
      (.requiredDimensions | sort | join(","))
    ]
  | @tsv
' "$catalog" | sort | uniq -d)"
if [ -n "$duplicate_definitions" ]; then
  echo "Duplicate product metric catalog definitions:" >&2
  echo "$duplicate_definitions" >&2
  exit 1
fi

jq -e '
  all(.metrics[] | select(.serviceName == "ec2" and .namespace == "AWS/EC2"); (.requiredDimensions | index("InstanceId") != null))
  and all(.metrics[] | select(.serviceName == "lambda"); (.requiredDimensions | index("FunctionName") != null))
  and all(.metrics[] | select(.serviceName == "api-gateway"); ((.requiredDimensions | index("ApiName") != null) and (.requiredDimensions | index("Stage") != null)))
  and all(.metrics[] | select(.serviceName == "amplify"); ((.requiredDimensions | index("App") != null) and (.requiredDimensions | index("Branch") != null)))
  and all(.metrics[] | select(.serviceName == "s3" and (.metricName == "BucketSizeBytes" or .metricName == "NumberOfObjects")); ((.requiredDimensions | index("BucketName") != null) and (.requiredDimensions | index("StorageType") != null)))
' "$catalog" >/dev/null

catalog_default_ec2_lambda="$(mktemp)"
legacy_recommended_ec2_lambda="$(mktemp)"
cleanup() {
  rm -f "$catalog_default_ec2_lambda" "$legacy_recommended_ec2_lambda"
}
trap cleanup EXIT

jq -r '
  .metrics[]
  | select(.recommended == true and (.serviceName == "ec2" or .serviceName == "lambda"))
  | [.serviceName, .namespace, .metricName, .statistic, (.periodSeconds | tostring), .unit]
  | @tsv
' "$catalog" | sort >"$catalog_default_ec2_lambda"

jq -r '
  .metricSets[]
  | select(.serviceName == "ec2" or .serviceName == "lambda")
  | . as $set
  | .metrics[]
  | [$set.serviceName, $set.namespace, .metricName, .statistic, (.periodSeconds | tostring), .unit]
  | @tsv
' "$recommended" | sort >"$legacy_recommended_ec2_lambda"

if ! cmp -s "$catalog_default_ec2_lambda" "$legacy_recommended_ec2_lambda"; then
  echo "EC2/Lambda default metric compatibility drifted between product catalog and legacy recommended metric sets." >&2
  diff -u "$legacy_recommended_ec2_lambda" "$catalog_default_ec2_lambda" >&2 || true
  exit 1
fi

grep -q 'DefaultMetricSets' internal/productcatalog/catalog.go
grep -q 'LoadDefaultMetricSetsFromProductCatalog' internal/admin/config.go
grep -q 'configs/product-metric-catalog.json' cmd/admin-server/main.go
grep -q '기본 metric' .ai/phases/cloud-monitor-productization/step20.md

GOCACHE="${GOCACHE:-$(pwd)/.cache/go-build}" go test ./internal/productcatalog ./internal/admin

sh scripts/validate-common.sh

echo "step20 validation passed"
