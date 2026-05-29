#!/bin/sh
set -eu

test -f .github/workflows/ci.yml
test -f .github/workflows/deploy.yml
test -f docker-compose.yml
test -f scripts/deploy-production.sh
test -f scripts/rollback-production.sh
test -f scripts/verify-grafana-public-dashboard.sh

sh -n scripts/deploy-production.sh
sh -n scripts/rollback-production.sh
sh -n scripts/verify-grafana-public-dashboard.sh

grep -q 'workflow_dispatch' .github/workflows/deploy.yml
grep -Fq 'workflow_run:' .github/workflows/deploy.yml
grep -Fq 'workflows: ["Cloud Monitor CI"]' .github/workflows/deploy.yml
grep -Fq 'types: [completed]' .github/workflows/deploy.yml
grep -Fq 'branches: [main]' .github/workflows/deploy.yml
grep -q "github.event.workflow_run.conclusion == 'success'" .github/workflows/deploy.yml
grep -q 'github.event.workflow_run.head_sha' .github/workflows/deploy.yml
grep -q 'DEPLOY_SSH_KEY' .github/workflows/deploy.yml
grep -q 'DEPLOY_PATH' .github/workflows/deploy.yml
grep -q 'production-deploy' .github/workflows/deploy.yml
grep -q 'git fetch --prune' .github/workflows/deploy.yml
grep -q 'scripts/deploy-production.sh' .github/workflows/deploy.yml

if grep -E 'actions/setup-go|go test|validate-step|validate-common|validate-public-dashboard|docs/operations|docker compose' .github/workflows/deploy.yml >/dev/null; then
  echo "deploy workflow must not run non-deployment validation" >&2
  exit 1
fi

grep -q 'git checkout --force' scripts/deploy-production.sh
grep -q 'validate-production-env.sh' scripts/deploy-production.sh
grep -q 'docker compose' scripts/deploy-production.sh
grep -q 'config >/dev/null' scripts/deploy-production.sh
grep -q 'docker compose up -d --build postgres grafana admin-ui' scripts/deploy-production.sh
grep -q 'docker compose --profile setup run --rm schema' scripts/deploy-production.sh
grep -q 'docker compose --profile collector up -d --build collector' scripts/deploy-production.sh
grep -q 'verify-grafana-health.sh' scripts/deploy-production.sh
grep -q 'verify-db-health.sh' scripts/deploy-production.sh
grep -q 'Admin UI unauthenticated check failed' scripts/deploy-production.sh

if grep -R -E 'ghcr\.io|docker/build-push-action|docker/login-action|upload-artifact' \
  .github/workflows/deploy.yml scripts/deploy-production.sh scripts/rollback-production.sh >/dev/null; then
  echo "CI/CD must not use GHCR, Docker image push, or Actions artifacts" >&2
  exit 1
fi

docker compose \
  --profile setup \
  --profile discovery \
  --profile collector \
  --profile admin-ui \
  --profile jobs \
  config >/dev/null

sh scripts/validate-common.sh

echo "ci validation passed"
