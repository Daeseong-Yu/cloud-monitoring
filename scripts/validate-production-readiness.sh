#!/bin/sh
set -eu

sh scripts/validate-step12.sh
sh scripts/validate-step13.sh
sh scripts/validate-step14.sh
DATABASE_URL= sh scripts/validate-step15.sh
sh scripts/validate-step16.sh
sh scripts/validate-step17.sh

docker compose \
  --profile setup \
  --profile discovery \
  --profile collector \
  --profile admin-ui \
  --profile jobs \
  config >/dev/null

RUN_GITLEAKS=1 sh scripts/scan-secrets.sh .ai README.md .env.example Dockerfile docker-compose.yml .dockerignore .github aws configs db scripts internal cmd grafana docs deploy

echo "production readiness validation passed"
