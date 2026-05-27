#!/bin/sh
set -eu

nginx_example="deploy/nginx/cloud-monitor.conf.example"

test -f docs/operations/security-hardening.md
test -f "$nginx_example"
test -f scripts/validate-public-dashboard.sh
test -f scripts/verify-aws-permissions.sh
test -f aws/iam/collector-readonly-policy.json
test -f aws/iam/discovery-readonly-policy.json

grep -q 'reverse proxy TLS' docs/operations/security-hardening.md
grep -q 'Admin UI는 public internet에 직접 노출하지 않는다' docs/operations/security-hardening.md
grep -q 'deny all' docs/operations/security-hardening.md
grep -q 'validate-public-dashboard.sh' docs/operations/security-hardening.md
grep -q 'Secret Rotation' docs/operations/security-hardening.md
grep -q 'verify-aws-permissions.sh' docs/operations/security-hardening.md
grep -q 'RUN_GITLEAKS=1 sh scripts/scan-secrets.sh' docs/operations/security-hardening.md
grep -q 'SSO' docs/operations/security-hardening.md

grep -q 'server_name REPLACE_WITH_GRAFANA_DOMAIN;' "$nginx_example"
grep -q 'server_name REPLACE_WITH_ADMIN_DOMAIN;' "$nginx_example"
grep -q 'ssl_certificate REPLACE_WITH_GRAFANA_CERT_PATH;' "$nginx_example"
grep -q 'ssl_certificate_key REPLACE_WITH_GRAFANA_KEY_PATH;' "$nginx_example"
grep -q 'ssl_certificate REPLACE_WITH_ADMIN_CERT_PATH;' "$nginx_example"
grep -q 'ssl_certificate_key REPLACE_WITH_ADMIN_KEY_PATH;' "$nginx_example"
grep -q 'allow REPLACE_WITH_ADMIN_ALLOWED_CIDR;' "$nginx_example"
grep -q 'deny all;' "$nginx_example"
grep -q 'proxy_pass http://127.0.0.1:3000;' "$nginx_example"
grep -q 'proxy_pass http://127.0.0.1:8080;' "$nginx_example"

if grep -R -E 'https?://[^[:space:]]+|arn:aws|[0-9]{12}|i-[0-9a-f]{8,}' "$nginx_example" docs/operations/security-hardening.md \
  | grep -Ev 'http://127\.0\.0\.1:(3000|8080);' >/dev/null; then
  echo "security hardening artifacts must not contain real URLs, AWS ids, ARNs, or resource ids" >&2
  exit 1
fi

jq -e '
  [.Statement[].Action] | flatten |
  all(.[];
    test("^(cloudwatch:(Get|List)|ec2:Describe|tag:Get)[A-Za-z]*$")
  )
' aws/iam/collector-readonly-policy.json aws/iam/discovery-readonly-policy.json >/dev/null

if jq -e '
  [.Statement[].Action] | flatten |
  any(.[];
    test(":(Put|Create|Update|Delete|Terminate|Start|Stop)|^iam:|^sts:AssumeRole$")
  )
' aws/iam/collector-readonly-policy.json aws/iam/discovery-readonly-policy.json >/dev/null; then
  echo "AWS IAM policy contains non-read-only action" >&2
  exit 1
fi

SKIP_COMMON_VALIDATION=1 sh scripts/validate-public-dashboard.sh
RUN_GITLEAKS=1 sh scripts/scan-secrets.sh .ai README.md .env.example Dockerfile docker-compose.yml .dockerignore .github aws configs db scripts internal cmd grafana docs deploy

echo "security hardening validation passed"
