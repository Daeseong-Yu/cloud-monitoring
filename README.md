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
- EC2, Lambda, CWAgent metric 후보 탐색
- Admin UI를 통한 resource discovery 실행과 metric definition 관리
- EC2/Lambda 추천 metric set 적용
- Go collector의 주기적 CloudWatch `GetMetricData` 수집
- PostgreSQL metric 저장소
- Grafana PostgreSQL datasource provisioning
- Grafana dashboard provisioning
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
- 공개 가능한 영역은 Grafana read-only dashboard로 제한합니다.
- AWS IAM 권한은 read-only 조회 권한만 사용합니다.
- 실제 secret, account id, full ARN, resource id는 repository에 commit하지 않습니다.

## Runtime 구성

Docker Compose 서비스:

| Service | 역할 |
| --- | --- |
| `postgres` | metric, resource, alert, summary 저장소 |
| `grafana` | PostgreSQL datasource 기반 dashboard |
| `admin-ui` | resource discovery, metric definition 관리 |
| `collector` | enabled metric definition을 CloudWatch에서 수집 |
| `schema` | DB migration one-shot |
| `resource-discovery` | CloudWatch metric 후보와 tag 기반 resource discovery |
| `alert-runner` | alert rule 평가 one-shot |
| `summary-rollup` | hourly/daily summary 생성 one-shot |
| `retention-job` | raw metric retention 적용 one-shot |

기본 운영 경로는 다음과 같습니다.

1. PostgreSQL, Grafana, Admin UI를 기동합니다.
2. Schema migration을 적용합니다.
3. Admin UI에서 `Discovery 실행`을 누릅니다.
4. 발견된 resource에 추천 metric set을 적용합니다.
5. Collector를 실행해 CloudWatch metric을 PostgreSQL에 저장합니다.
6. Grafana dashboard에서 PostgreSQL에 저장된 metric을 조회합니다.

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

## Dashboard

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

Grafana는 `cloud-monitor-postgres` datasource만 사용합니다.

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
