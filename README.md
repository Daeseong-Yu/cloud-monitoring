# Cloud Monitor

Cloud Monitor는 AWS CloudWatch metric을 자체 서버의 PostgreSQL에 수집하고, Grafana가 PostgreSQL만 조회하도록 만든 모니터링 스택입니다.

Grafana는 CloudWatch를 직접 조회하지 않습니다. CloudWatch API 호출은 Go collector와 resource discovery 경로로 제한되며, dashboard 조회수와 CloudWatch API 호출량이 직접 연결되지 않도록 분리합니다.

## 문서

- [Installation.md](Installation.md): 운영 서버 설치, 환경 변수, AWS profile, 기동, 검증 절차
- [Architecture.md](Architecture.md): 인프라 구성, 데이터 흐름, 보안 경계
- [docs/operations/scheduling.md](docs/operations/scheduling.md): collector와 systemd timer 운영
- [docs/operations/backup-restore.md](docs/operations/backup-restore.md): PostgreSQL backup, restore, migration
- [docs/operations/security-hardening.md](docs/operations/security-hardening.md): reverse proxy, secret, IAM hardening
- [docs/operations/deployment.md](docs/operations/deployment.md): GitHub Actions SSH source 배포와 rollback

## 구현 범위

- AWS CloudWatch metric discovery
- EC2, Lambda, API Gateway, Amplify, SES, S3 resource discovery
- Admin UI를 통한 resource discovery 실행과 서비스/리소스 모니터링 여부 관리
- Product metric catalog 기반 기본 metric 자동 적용
- Go collector의 주기적 CloudWatch `GetMetricData` 수집
- PostgreSQL metric 저장소
- Grafana PostgreSQL datasource provisioning
- 관리자용 Grafana dashboard provisioning과 편집
- 공개용 Grafana Public Dashboard 기준
- Alert runner, summary rollup, retention one-shot job
- PostgreSQL backup, migration, health validation script
- Docker Compose 기반 운영 runtime
- systemd timer 기반 one-shot job scheduling
- GitHub Actions 기반 SSH source deployment

## 주요 설계 원칙

- Grafana datasource는 PostgreSQL만 사용합니다.
- Grafana CloudWatch datasource와 CloudWatch Logs panel은 사용하지 않습니다.
- AWS credential은 direct environment key가 아니라 shared profile 파일로 전달합니다.
- Admin UI는 운영자 전용이며 public internet에 직접 노출하지 않습니다.
- Grafana dashboard가 관리자/공개 조회 surface입니다.
- 관리자 Grafana는 로그인 후 dashboard 편집을 허용하고, 공개 Grafana dashboard는 read-only로만 노출합니다.
- 공개 dashboard는 public metadata로 opt-in된 리소스만 표시하며 raw resource id, account id, full ARN, raw tags, raw collector error를 노출하지 않는 public-safe view만 조회합니다.
- Admin UI는 개별 metric 선택 화면이 아니라 서비스/리소스의 모니터링 여부와 공개 여부를 관리합니다.
- AWS IAM 권한은 read-only 조회 권한만 사용합니다.
- 실제 secret, account id, full ARN, resource id는 repository에 commit하지 않습니다.

## Runtime 구성

Docker Compose 서비스:

| Service | 역할 |
| --- | --- |
| `postgres` | metric, resource, alert, summary 저장소 |
| `grafana` | PostgreSQL datasource 기반 dashboard |
| `admin-ui` | resource discovery, 서비스/리소스 모니터링 여부와 공개 metadata 관리 |
| `collector` | enabled 기본 metric definition을 CloudWatch에서 수집 |
| `schema` | DB migration one-shot |
| `resource-discovery` | CloudWatch metric과 tag 기반 resource discovery |
| `alert-runner` | alert rule 평가 one-shot |
| `summary-rollup` | hourly/daily summary 생성 one-shot |
| `retention-job` | raw metric retention 적용 one-shot |

기본 운영 경로는 다음과 같습니다.

1. PostgreSQL, Grafana, Admin UI를 기동합니다.
2. Schema migration을 적용합니다.
3. Admin UI에서 `Discovery 실행`을 누릅니다.
4. 서비스/리소스 목록에서 모니터링할 대상을 선택합니다.
5. 선택된 리소스에는 product metric catalog의 기본 metric이 자동 적용됩니다.
6. 공개 dashboard에 표시할 리소스만 public metadata로 별도 opt-in합니다.
7. Collector를 실행해 CloudWatch metric을 PostgreSQL에 저장합니다.
8. 관리자 Grafana dashboard에서 PostgreSQL에 저장된 metric을 조회하고 필요하면 dashboard를 편집합니다.
9. 공개용 Grafana Public Dashboard는 public-safe view를 통해 opt-in된 데이터만 read-only로 표시합니다.

## 빠른 설치 흐름

자세한 절차는 [Installation.md](Installation.md)를 기준으로 진행합니다.

```bash
cd /opt
git clone https://github.com/Daeseong-Yu/cloud-monitoring.git cloud-monitor
cd /opt/cloud-monitor

set -a
. /etc/cloud-monitor/cloud-monitor.env
set +a

sh scripts/validate-production-env.sh
docker compose --profile setup --profile discovery --profile collector --profile admin-ui --profile jobs config >/tmp/cloud-monitor-compose.yml

docker compose up -d --build postgres grafana admin-ui
docker compose --profile setup run --rm schema
docker compose --profile collector up -d --build collector
```

## 운영 확인

```bash
docker compose --profile collector --profile admin-ui ps
GRAFANA_URL="${GRAFANA_URL:-http://127.0.0.1:${GRAFANA_PORT:-3000}}" sh scripts/verify-grafana-health.sh
sh scripts/verify-db-health.sh
docker compose logs --tail=100 collector
```

Admin UI 인증 확인:

```bash
curl -sS -o /dev/null -w '%{http_code}\n' \
  "${ADMIN_URL:-http://127.0.0.1:${ADMIN_HTTP_PORT:-8080}}/"
```

기대값은 `401`입니다.

수집 데이터 확인:

```bash
docker compose exec postgres psql -U "${POSTGRES_USER:-cloud_monitor}" \
  -d "${POSTGRES_DB:-cloud_monitor}" \
  -c "select count(*) from metric_definitions where enabled = true; select count(*) from metric_points;"
```

## CI/CD

`main` branch에 push하면 `Cloud Monitor CI`가 자동으로 실행됩니다. CI가 성공하면 `Cloud Monitor Deploy`가 `workflow_run`으로 이어서 실행되어 같은 commit SHA를 운영 서버에 SSH source deploy합니다.

필요하면 GitHub Actions에서 `Cloud Monitor Deploy`를 수동 실행하고 `ref`에 branch, tag, 또는 commit을 지정할 수 있습니다.

## Grafana Dashboards

Grafana dashboard는 관리자용과 공개용으로 나눕니다.

- 관리자용 dashboard는 내부망, VPN, 또는 IP allowlist 뒤에서 운영하며 실제 resource id와 운영 진단 정보를 볼 수 있습니다.
- 관리자 로그인에서는 dashboard 편집을 허용합니다. 편집 저장본은 Grafana DB에 남깁니다.
- 공개용 dashboard는 Grafana Public Dashboard 또는 externally shared dashboard로 read-only 노출합니다.
- 공개용 dashboard는 Grafana shared dashboard 제약에 맞춰 variable 없이 구성하고, public-safe SQL view만 조회합니다.
- 실제 공개 URL, 공유 token, 서버 IP, domain, credential은 tracked file에 기록하지 않습니다.

Provisioning 대상:

- `grafana/dashboards/cloud-monitor-ec2-mvp.json`
- `grafana/dashboards/cloud-monitor-lambda.json`
- `grafana/dashboards/cloud-monitor-aws-resource.json`
- `grafana/dashboards/cloud-monitor-postgres-ops.json`
- `grafana/dashboards/cloud-monitor-overview.json`
- `grafana/dashboards/cloud-monitor-api-gateway.json`
- `grafana/dashboards/cloud-monitor-amplify.json`
- `grafana/dashboards/cloud-monitor-ses.json`
- `grafana/dashboards/cloud-monitor-s3.json`

공개용 provisioning 대상:

- `grafana/public-dashboards/cloud-monitor-public-overview.json`

Grafana는 `cloud-monitor-postgres` datasource만 사용합니다.

## Public Grafana Dashboard

외부 공개 surface는 별도 제품 frontend/API가 아니라 Grafana Public Dashboard입니다. 공개 dashboard는 `public_enabled=true`로 명시한 리소스의 기본 metric만 표시하고, public label 중심으로 series를 구성합니다.

공개 dashboard는 `public_grafana_metric_points`, `public_grafana_metric_summary` view만 조회합니다. 공개 dashboard SQL/view에는 raw resource id, AWS account id, full ARN, raw tags, credential, raw collector error를 포함하지 않습니다.

공개 전 확인:

- 공개 URL, domain, 서버 IP, credential은 tracked file에 기록하지 않습니다.
- Admin UI와 Grafana 관리자/edit access는 내부망, VPN, 또는 IP allowlist 뒤에 둡니다.
- Public Dashboard 공유 링크만 외부 노출 대상으로 검토합니다.
- 공개 여부는 모니터링 여부와 별개이며, public metadata로 명시적으로 opt-in된 리소스만 공개합니다.
- 배포 전 `sh scripts/validate-step27.sh`, `sh scripts/validate-productization.sh`, `RUN_GITLEAKS=1 sh scripts/scan-secrets.sh`를 실행합니다.

## AWS 권한

Collector와 discovery에는 read-only 권한만 필요합니다.

- `cloudwatch:Get*`
- `cloudwatch:List*`
- `ec2:Describe*`
- `tag:Get*`

정책 template:

- `aws/iam/collector-readonly-policy.json`
- `aws/iam/discovery-readonly-policy.json`

AWS write 권한, Grafana CloudWatch datasource, CloudWatch Logs collection은 이 프로젝트 범위에 포함하지 않습니다.

## 개발 검증

```bash
go test ./...
sh scripts/validate-production-env.sh
sh scripts/smoke-compose.sh
```

운영 서버에 `.ai` phase 파일이 없는 GitHub clone에서는 phase validation script 대신 [Installation.md](Installation.md)의 runtime 검증 절차를 사용합니다.
