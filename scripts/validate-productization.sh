#!/bin/sh
set -eu

for step in 19 20 21 22 23 24 25 26 27; do
  sh "scripts/validate-step${step}.sh"
done

sh scripts/validate-common.sh

echo "productization validation passed"
