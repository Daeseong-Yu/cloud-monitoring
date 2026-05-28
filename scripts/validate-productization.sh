#!/bin/sh
set -eu

for step in 19 20 21 22 23 24 25 26; do
  sh "scripts/validate-step${step}.sh"
done

SKIP_PRODUCTIZATION_VALIDATION=1 sh scripts/validate-step27.sh

sh scripts/validate-common.sh

echo "productization validation passed"
