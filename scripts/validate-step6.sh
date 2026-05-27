#!/bin/sh
set -eu

SKIP_COMMON_VALIDATION=1 sh scripts/validate-public-dashboard.sh
sh scripts/validate-common.sh

jq -e '.steps[] | select(.number == 6) | .status == "completed"' .ai/phases/cloud-monitor/index.json >/dev/null
jq -e 'all(.steps[]; .status == "completed")' .ai/phases/cloud-monitor/index.json >/dev/null

echo "step6 validation passed"
