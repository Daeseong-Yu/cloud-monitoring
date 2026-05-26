#!/bin/sh
set -eu

if [ "$#" -gt 0 ]; then
  targets="$*"
else
  targets=".ai README.md .env.example aws configs db scripts internal cmd"
fi

tmp_file="$(mktemp)"
trap 'rm -f "$tmp_file"' EXIT

scan_pattern() {
  pattern="$1"
  shift

  # shellcheck disable=SC2086
  grep -RInE \
    --exclude-dir .git \
    --exclude-dir .cache \
    --exclude-dir vendor \
    "$pattern" \
    $targets >>"$tmp_file" 2>/dev/null || true
}

aws_key='A''KIA'
arn_prefix='arn'':aws'
openai_key='s''k-'
openai_project_key='s''k-proj-'
github_token='g''hp_'
github_pat='g''ithub_pat_'
slack_token='x''ox[baprs]-'
stripe_secret='s''k_live_'
stripe_restricted='r''k_live_'
google_api='A''Iza[0-9A-Za-z_-]{20,}'
sendgrid_key='S''G\.[0-9A-Za-z_-]{10,}\.[0-9A-Za-z_-]{10,}'
jwt_token='e''yJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}'
private_key_header='BEGIN (RSA |OPENSSH |EC |DSA |)?PRIVATE KEY'
db_url_with_password='(postgres|postgresql|mysql|mongodb)://[^:/@[:space:]]+:[^@[:space:]]+@'
generic_assignment='(^|[^A-Za-z0-9_])(API_KEY|TOKEN|PASSWORD|PASSWD|PWD|SECRET|SECRET_KEY|CLIENT_SECRET|PRIVATE_KEY)[[:space:]]*[:=][[:space:]]*['"'"'"]?[^'"'"'"[:space:]]{8,}'

scan_pattern "${aws_key}"
scan_pattern "${arn_prefix}"
scan_pattern "[0-9]{12}"
scan_pattern "i-[0-9a-f]{8,}"
scan_pattern "${openai_project_key}|${openai_key}[A-Za-z0-9_-]{20,}"
scan_pattern "${github_token}|${github_pat}"
scan_pattern "${slack_token}"
scan_pattern "${stripe_secret}|${stripe_restricted}"
scan_pattern "${google_api}"
scan_pattern "${sendgrid_key}"
scan_pattern "${jwt_token}"
scan_pattern "${private_key_header}"
scan_pattern "${db_url_with_password}"
scan_pattern "${generic_assignment}"

if [ ! -s "$tmp_file" ]; then
  if [ "${RUN_GITLEAKS:-0}" = "1" ] && command -v gitleaks >/dev/null 2>&1; then
    gitleaks detect --source . --no-git --redact
  fi
  echo "secret scan passed"
  exit 0
fi

allowed_pattern='REPLACE_WITH|change-me|CHANGE_ME|placeholder|PLACEHOLDER|example|EXAMPLE|dummy|DUMMY|fake|FAKE|test|TEST|localhost|127\.0\.0\.1'

if grep -Ev "$allowed_pattern" "$tmp_file" >/tmp/cloud-monitor-secret-findings.txt; then
  echo "민감 정보로 보이는 패턴이 발견되었습니다." >&2
  cat /tmp/cloud-monitor-secret-findings.txt >&2
  exit 1
fi

if [ "${RUN_GITLEAKS:-0}" = "1" ] && command -v gitleaks >/dev/null 2>&1; then
  gitleaks detect --source . --no-git --redact
fi

echo "secret scan passed"
