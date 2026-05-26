#!/bin/sh
set -eu

find .ai -maxdepth 4 -type f | sort >/dev/null

jq . .ai/phases/index.json .ai/phases/cloud-monitor/index.json >/dev/null

aws_key='A''KIA'
secret_word='S''ECRET'
private_word='P''RIVATE'
openssh_word='O''PENSSH'
arn_prefix='arn'':aws'
pattern="${aws_key}|${secret_word}|BEGIN (RSA|${openssh_word}|${private_word})|${arn_prefix}|[0-9]{12}|i-[0-9a-f]{8,}"

if grep -R -E "$pattern" .ai README.md .env.example >/dev/null; then
  echo "민감 정보로 보이는 패턴이 발견되었습니다." >&2
  exit 1
fi

echo "step1 validation passed"
