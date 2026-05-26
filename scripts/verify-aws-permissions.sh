#!/bin/sh
set -eu

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "$1 command not found" >&2
    exit 1
  fi
}

trim() {
  printf '%s' "$1" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//'
}

utc_now() {
  date -u '+%Y-%m-%dT%H:%M:%SZ'
}

utc_minus_minutes() {
  minutes="$1"
  if date -u -v-"${minutes}"M '+%Y-%m-%dT%H:%M:%SZ' >/dev/null 2>&1; then
    date -u -v-"${minutes}"M '+%Y-%m-%dT%H:%M:%SZ'
    return
  fi

  date -u -d "${minutes} minutes ago" '+%Y-%m-%dT%H:%M:%SZ'
}

require_command aws
require_command sed
require_command date

aws_region="$(trim "${AWS_REGION:-}")"
target_instance_id="$(trim "${TARGET_INSTANCE_ID:-}")"
cwagent_metric_name="$(trim "${CWAGENT_METRIC_NAME:-mem_used_percent}")"

if [ -z "$aws_region" ]; then
  echo "AWS_REGION is required" >&2
  exit 2
fi

if [ -z "$target_instance_id" ]; then
  echo "TARGET_INSTANCE_ID is required" >&2
  exit 2
fi

if [ -z "$cwagent_metric_name" ]; then
  echo "CWAGENT_METRIC_NAME must not be blank" >&2
  exit 2
fi

start_time="$(utc_minus_minutes 15)"
end_time="$(utc_now)"

echo "AWS permission verification started"

aws ec2 describe-regions \
  --region "$aws_region" \
  --query 'Regions[].RegionName' \
  --output text >/dev/null

aws ec2 describe-instances \
  --region "$aws_region" \
  --instance-ids "$target_instance_id" \
  --query 'Reservations[].Instances[].InstanceId' \
  --output text >/dev/null

aws ec2 describe-tags \
  --region "$aws_region" \
  --filters "Name=resource-id,Values=$target_instance_id" \
  --query 'Tags[].Key' \
  --output text >/dev/null

ec2_metric_count="$(
  aws cloudwatch list-metrics \
    --region "$aws_region" \
    --namespace AWS/EC2 \
    --metric-name CPUUtilization \
    --dimensions "Name=InstanceId,Value=$target_instance_id" \
    --query 'length(Metrics)' \
    --output text
)"

if [ "$ec2_metric_count" = "0" ]; then
  echo "AWS/EC2 CPUUtilization metric was not found for TARGET_INSTANCE_ID" >&2
  exit 1
fi

aws cloudwatch get-metric-statistics \
  --region "$aws_region" \
  --namespace AWS/EC2 \
  --metric-name CPUUtilization \
  --dimensions "Name=InstanceId,Value=$target_instance_id" \
  --start-time "$start_time" \
  --end-time "$end_time" \
  --period 300 \
  --statistics Average \
  --query 'Datapoints[].Timestamp' \
  --output text >/dev/null

metric_data_queries='[
  {
    "Id": "cpu",
    "MetricStat": {
      "Metric": {
        "Namespace": "AWS/EC2",
        "MetricName": "CPUUtilization",
        "Dimensions": [
          {
            "Name": "InstanceId",
            "Value": "'"$target_instance_id"'"
          }
        ]
      },
      "Period": 300,
      "Stat": "Average"
    },
    "ReturnData": true
  }
]'

aws cloudwatch get-metric-data \
  --region "$aws_region" \
  --metric-data-queries "$metric_data_queries" \
  --start-time "$start_time" \
  --end-time "$end_time" \
  --query 'MetricDataResults[].Id' \
  --output text >/dev/null

cwagent_metric_count="$(
  aws cloudwatch list-metrics \
    --region "$aws_region" \
    --namespace CWAgent \
    --metric-name "$cwagent_metric_name" \
    --dimensions "Name=InstanceId,Value=$target_instance_id" \
    --query 'length(Metrics)' \
    --output text
)"

if [ "$cwagent_metric_count" = "0" ]; then
  echo "CWAgent metric was not found for TARGET_INSTANCE_ID. Check CloudWatch Agent installation and EC2 instance role." >&2
  exit 1
fi

echo "AWS permission verification passed"
