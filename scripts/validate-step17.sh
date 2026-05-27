#!/bin/sh
set -eu

test -f scripts/validate-security-hardening.sh

sh -n scripts/validate-security-hardening.sh
sh scripts/validate-security-hardening.sh
sh scripts/validate-common.sh

echo "step17 validation passed"
