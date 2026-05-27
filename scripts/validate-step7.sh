#!/bin/sh
set -eu

compose_config="${TMPDIR:-/tmp}/cloud-monitor-compose-config.yml"
profile_config="${TMPDIR:-/tmp}/cloud-monitor-compose-config-all-profiles.yml"

test -f Dockerfile
test -f .dockerignore
test -f docker-compose.yml

docker compose config >"$compose_config"
docker compose --profile setup --profile collector --profile admin-ui --profile jobs config >"$profile_config"

for service in \
  postgres \
  grafana \
  schema \
  metricdefs-sync \
  collector \
  admin-ui \
  alert-runner \
  summary-rollup \
  retention-job
do
  grep -q "^  ${service}:" "$profile_config"
done

grep -q 'grafana/grafana-oss' "$profile_config"
grep -q 'postgres:16-alpine' "$profile_config"
grep -q 'cloud-monitor-postgres' grafana/provisioning/datasources/postgres.yaml
grep -q 'AWS_REGION=.*us-east-1' docker-compose.yml

if grep -R -i 'type:[[:space:]]*cloudwatch\|"type":[[:space:]]*"cloudwatch"\|datasource.*cloudwatch' docker-compose.yml grafana >/dev/null; then
  echo "Compose/Grafana 산출물에 CloudWatch datasource 참조가 있습니다." >&2
  exit 1
fi

sh scripts/validate-common.sh

echo "step7 validation passed"
