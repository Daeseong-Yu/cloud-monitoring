#!/bin/sh
set -eu

test -f .github/workflows/deploy.yml
test -f scripts/deploy-production.sh
test -f scripts/rollback-production.sh
test -f docs/operations/deployment.md

sh -n scripts/deploy-production.sh
sh -n scripts/rollback-production.sh

grep -q 'workflow_dispatch' .github/workflows/deploy.yml
grep -q 'environment: production' .github/workflows/deploy.yml
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
grep -q 'docker compose up -d --build postgres grafana admin-ui' scripts/deploy-production.sh
grep -q 'docker compose --profile setup run --rm schema' scripts/deploy-production.sh
grep -q 'docker compose --profile collector up -d --build collector' scripts/deploy-production.sh
grep -q 'verify-grafana-health.sh' scripts/deploy-production.sh
grep -q 'verify-db-health.sh' scripts/deploy-production.sh
grep -q 'Admin UI unauthenticated check failed' scripts/deploy-production.sh
grep -q '.deploy/current' docs/operations/deployment.md
grep -q 'GHCR를 사용하지 않는다' docs/operations/deployment.md
grep -q 'GitHub Packages를 사용하지 않는다' docs/operations/deployment.md
grep -q 'rollback-production.sh' docs/operations/deployment.md
grep -q '.deploy/' .gitignore
grep -q '.deploy/' .dockerignore

if grep -R -E 'ghcr\.io|docker/build-push-action|docker/login-action|upload-artifact' \
  .github/workflows/deploy.yml scripts/deploy-production.sh scripts/rollback-production.sh docs/operations/deployment.md >/dev/null; then
  echo "CI/CD step must not use GHCR, Docker image push, or Actions artifacts" >&2
  exit 1
fi

sh scripts/validate-common.sh

echo "step19 validation passed"
