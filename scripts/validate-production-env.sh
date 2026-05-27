#!/bin/sh
set -eu

failed=0

fail() {
  var_name="$1"
  reason="$2"
  echo "$var_name: $reason" >&2
  failed=1
}

get_env() {
  eval "printf '%s' \"\${$1-}\""
}

is_empty() {
  [ -z "$1" ]
}

has_placeholder() {
  value="$1"

  case "$value" in
    change-me|CHANGE_ME|placeholder|PLACEHOLDER)
      return 0
      ;;
    *change-me*|*CHANGE_ME*|*placeholder*|*PLACEHOLDER*|*REPLACE_WITH_*)
      return 0
      ;;
  esac

  return 1
}

has_sensitive_identifier() {
  value="$1"
  arn_prefix='arn'':aws'

  if printf '%s' "$value" | grep -Eq '(^|[^0-9])[0-9]{12}([^0-9]|$)'; then
    return 0
  fi

  if printf '%s' "$value" | grep -Eq "${arn_prefix}"'[a-zA-Z-]*:'; then
    return 0
  fi

  if printf '%s' "$value" | grep -Eq '(^|[^A-Za-z0-9])(i|vol|sg|subnet|vpc|eni|nat|rtb|igw)-[0-9a-f]{8,}([^A-Za-z0-9]|$)'; then
    return 0
  fi

  return 1
}

validate_required() {
  var_name="$1"
  value="$(get_env "$var_name")"

  if is_empty "$value"; then
    fail "$var_name" "missing required environment variable"
    return
  fi

  if has_placeholder "$value"; then
    fail "$var_name" "placeholder value is not allowed"
  fi

  if has_sensitive_identifier "$value"; then
    fail "$var_name" "sensitive infrastructure identifier is not allowed"
  fi
}

validate_nonempty() {
  var_name="$1"
  value="$(get_env "$var_name")"

  if is_empty "$value"; then
    fail "$var_name" "missing required environment variable"
  fi
}

validate_optional_if_set() {
  var_name="$1"
  value="$(get_env "$var_name")"

  if is_empty "$value"; then
    return
  fi

  if has_placeholder "$value"; then
    fail "$var_name" "placeholder value is not allowed"
  fi

  if has_sensitive_identifier "$value"; then
    fail "$var_name" "sensitive infrastructure identifier is not allowed"
  fi
}

validate_required AWS_REGION
validate_required DATABASE_URL
validate_required POSTGRES_PASSWORD
validate_required GRAFANA_ADMIN_PASSWORD
validate_required ADMIN_USERNAME
validate_required ADMIN_PASSWORD

validate_optional_if_set AWS_PROFILE
validate_optional_if_set AWS_ACCESS_KEY_ID
validate_optional_if_set AWS_SECRET_ACCESS_KEY
validate_optional_if_set AWS_SESSION_TOKEN
validate_optional_if_set TARGET_INSTANCE_ID

slack_webhook_url="$(get_env SLACK_WEBHOOK_URL)"
if [ -n "$slack_webhook_url" ]; then
  if has_placeholder "$slack_webhook_url"; then
    fail "SLACK_WEBHOOK_URL" "placeholder value is not allowed"
  elif ! printf '%s' "$slack_webhook_url" | grep -Eq '^https://hooks\.slack\.com/services/[^[:space:]]+$'; then
    fail "SLACK_WEBHOOK_URL" "format is invalid"
  fi
fi

if [ "$failed" -ne 0 ]; then
  exit 1
fi

echo "production environment validation passed"
