#!/bin/sh
set -eu

policy_file="aws/iam/collector-readonly-policy.json"
verify_script="scripts/verify-aws-permissions.sh"

test -f "$policy_file"
test -f "$verify_script"

jq -e '.Version == "2012-10-17"' "$policy_file" >/dev/null

jq -e '
  ([
    .Statement[]
    | select(.Effect == "Allow")
    | .Action
  ]
  | flatten
  | sort)
  == ([
    "cloudwatch:GetMetricData",
    "cloudwatch:GetMetricStatistics",
    "cloudwatch:ListMetrics",
    "ec2:DescribeInstances",
    "ec2:DescribeRegions",
    "ec2:DescribeTags"
  ]
  | sort)
' "$policy_file" >/dev/null

jq -e '
  all(.Statement[];
    .Effect == "Allow"
    and (
      .Resource == "*"
      or (.Resource | type == "array" and all(.[]; . == "*"))
    )
  )
' "$policy_file" >/dev/null

if jq -r '.Statement[].Action[]?' "$policy_file" | grep -E ':(Put|Create|Delete|Update|Terminate|Start|Stop|Reboot|Run|Attach|Detach|Modify|Assume|Pass)' >/dev/null; then
  echo "Collector policy contains a non-read action." >&2
  exit 1
fi

aws_key='A''KIA'
secret_word='S''ECRET'
private_word='P''RIVATE'
openssh_word='O''PENSSH'
arn_prefix='arn'':aws'
pattern="${aws_key}|${secret_word}|BEGIN (RSA|${openssh_word}|${private_word})|${arn_prefix}|[0-9]{12}|i-[0-9a-f]{8,}"

if grep -R -E "$pattern" .ai README.md .env.example aws "$verify_script" >/dev/null; then
  echo "민감 정보로 보이는 패턴이 발견되었습니다." >&2
  exit 1
fi

sh scripts/scan-secrets.sh .ai README.md .env.example aws "$verify_script"

echo "step2 validation passed"
