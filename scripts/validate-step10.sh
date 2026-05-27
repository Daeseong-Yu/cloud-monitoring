#!/bin/sh
set -eu

test -f internal/cloudwatchmetrics/client_factory.go

grep -q 'type RegionClientFactory' internal/cloudwatchmetrics/client_factory.go
grep -q 'FetcherForRegion' internal/cloudwatchmetrics/client_factory.go
grep -q 'NewRegionClientFactory' cmd/collector/main.go
grep -q 'DimensionsJSON' internal/store/store.go
grep -q 'cloudWatchDimensions' internal/cloudwatchmetrics/cloudwatchmetrics.go
grep -q 'PartialFailure' internal/cloudwatchmetrics/cloudwatchmetrics.go
grep -q 'fetchBatchTolerant' internal/cloudwatchmetrics/cloudwatchmetrics.go
grep -q 'SkippedDefinitions' internal/collector/collector.go
grep -q 'collector partial metric fetch failure' internal/collector/collector.go
grep -q 'TestFetchUsesStoredGenericDimensions' internal/cloudwatchmetrics/cloudwatchmetrics_test.go
grep -q 'TestFetchRetriesBatchAsSingleDefinitionsAndReturnsPartialFailure' internal/cloudwatchmetrics/cloudwatchmetrics_test.go
grep -q 'TestCollectOnceStoresPointsAfterPartialFetchFailure' internal/collector/collector_test.go
grep -q 'AND region = $1' internal/store/store.go

if grep -R -i 'logs:|filterlogevents|startquery|cloudwatch logs' cmd internal docker-compose.yml >/dev/null; then
  echo "Collector 산출물에 CloudWatch Logs 수집 경로가 있습니다." >&2
  exit 1
fi

sh scripts/validate-common.sh

echo "step10 validation passed"
