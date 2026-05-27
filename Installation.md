# Installation

이 문서는 GitHub repository를 새 서버에 clone한 뒤 Cloud Monitor를 운영 가능한 상태로 기동하는 절차입니다.

기준 runtime:

- Repository: `/opt/cloud-monitor`
- Runtime env file: `/etc/cloud-monitor/cloud-monitor.env`
- AWS shared profile directory: `/etc/cloud-monitor/aws`
- Backup directory: `/srv/cloud-monitor/backups`
- Runtime: Docker Compose
- One-shot scheduler: systemd timer

## 1. 사전 준비

서버에 아래 도구가 필요합니다.

- `git`
- Docker Engine
- Docker Compose plugin
- `curl`
- `jq`
- `psql` 또는 `postgresql-client`

설치 여부 확인:

```bash
git --version
docker --version
docker compose version
curl --version
jq --version
psql --version
```

현재 사용자가 Docker를 실행할 수 있어야 합니다.

```bash
docker ps
```

권한 오류가 나면 docker group에 사용자를 추가하고 다시 로그인합니다.

```bash
sudo usermod -aG docker "$USER"
```

## 2. 운영 group과 directory 생성

```bash
sudo groupadd --system cloud-monitor || true
sudo usermod -aG cloud-monitor "$USER"
```

group 변경은 새 로그인 세션부터 적용됩니다. 가능하면 로그아웃 후 다시 로그인합니다.

directory를 생성합니다.

```bash
sudo mkdir -p /opt/cloud-monitor
sudo mkdir -p /etc/cloud-monitor/aws
sudo mkdir -p /srv/cloud-monitor/backups
```

권한을 설정합니다.

```bash
sudo chown "$USER:$USER" /opt/cloud-monitor
sudo chown "$USER:$USER" /srv/cloud-monitor/backups
sudo chown root:cloud-monitor /etc/cloud-monitor
sudo chown root:cloud-monitor /etc/cloud-monitor/aws

sudo chmod 755 /opt/cloud-monitor
sudo chmod 750 /etc/cloud-monitor
sudo chmod 750 /etc/cloud-monitor/aws
sudo chmod 750 /srv/cloud-monitor/backups
```

현재 shell에서 group이 아직 안 보이면 새로 로그인합니다.

```bash
id -nG | tr ' ' '\n' | grep -Fx cloud-monitor
```

## 3. Repository clone

```bash
cd /opt
git clone https://github.com/Daeseong-Yu/cloud-monitoring.git cloud-monitor
cd /opt/cloud-monitor
```

`/opt/cloud-monitor`는 운영 사용자가 소유해야 합니다.

```bash
ls -ld /opt/cloud-monitor
```

## 4. AWS shared profile 생성

Cloud Monitor는 direct AWS key environment variable을 사용하지 않습니다.

사용하지 않는 값:

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_SESSION_TOKEN`

운영 기준은 shared profile입니다.

`/etc/cloud-monitor/aws/config`:

```bash
sudo tee /etc/cloud-monitor/aws/config >/dev/null <<'EOF'
[profile grafana]
region = us-east-1
output = json
EOF
```

`/etc/cloud-monitor/aws/credentials`:

```bash
sudo tee /etc/cloud-monitor/aws/credentials >/dev/null <<'EOF'
[grafana]
aws_access_key_id = REPLACE_WITH_ACCESS_KEY_ID
aws_secret_access_key = REPLACE_WITH_SECRET_ACCESS_KEY
EOF
```

파일 권한을 제한합니다.

```bash
sudo chown root:cloud-monitor /etc/cloud-monitor/aws/config /etc/cloud-monitor/aws/credentials
sudo chmod 640 /etc/cloud-monitor/aws/config /etc/cloud-monitor/aws/credentials
```

profile section 이름이 정확해야 합니다.

```bash
sudo grep -Fx "[profile grafana]" /etc/cloud-monitor/aws/config
sudo grep -Fx "[grafana]" /etc/cloud-monitor/aws/credentials
```

## 5. Runtime env file 생성

운영 env file을 만듭니다.

아래 값 중 password는 실제 운영 값으로 바꿉니다. public repository에 기록하지 않습니다.

```bash
sudo tee /etc/cloud-monitor/cloud-monitor.env >/dev/null <<'EOF'
AWS_REGION=us-east-1
AWS_PROFILE=grafana
AWS_SHARED_CONFIG_DIR=/etc/cloud-monitor/aws

COLLECTOR_INTERVAL_SECONDS=60
CLOUDWATCH_LOOKBACK_MINUTES=15
METRIC_RETENTION_DAYS=30

POSTGRES_DB=cloud_monitor
POSTGRES_USER=cloud_monitor
POSTGRES_PASSWORD=REPLACE_WITH_POSTGRES_PASSWORD
POSTGRES_PORT=15432
DATABASE_URL=postgres://cloud_monitor:REPLACE_WITH_POSTGRES_PASSWORD@127.0.0.1:15432/cloud_monitor?sslmode=disable

GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=REPLACE_WITH_GRAFANA_ADMIN_PASSWORD
GRAFANA_PORT=13000
GRAFANA_URL=http://127.0.0.1:13000
GRAFANA_ANONYMOUS_ENABLED=false

ADMIN_HTTP_PORT=8080
ADMIN_URL=http://127.0.0.1:8080
ADMIN_USERNAME=operator
ADMIN_PASSWORD=REPLACE_WITH_ADMIN_PASSWORD

SLACK_WEBHOOK_URL=
EOF
```

권한을 제한합니다.

```bash
sudo chown root:cloud-monitor /etc/cloud-monitor/cloud-monitor.env
sudo chmod 640 /etc/cloud-monitor/cloud-monitor.env
```

현재 shell에 값을 로드합니다.

```bash
set -a
. /etc/cloud-monitor/cloud-monitor.env
set +a
```

`. /etc/cloud-monitor/cloud-monitor.env`에서 permission denied가 나면 현재 사용자가 `cloud-monitor` group이 아니거나 새 로그인 세션이 아닙니다. `id -nG`로 group을 확인한 뒤 다시 로그인합니다.

## 6. Preflight 검증

```bash
cd /opt/cloud-monitor

sh scripts/validate-production-env.sh

docker compose \
  --profile setup \
  --profile discovery \
  --profile collector \
  --profile admin-ui \
  --profile jobs \
  config >/tmp/cloud-monitor-compose.yml
```

`scripts/validate-production-env.sh`는 direct AWS credential environment variable이 있으면 실패합니다. shell에 남아 있다면 제거합니다.

```bash
unset AWS_ACCESS_KEY_ID
unset AWS_SECRET_ACCESS_KEY
unset AWS_SESSION_TOKEN
```

## 7. PostgreSQL, Grafana, Admin UI 기동

```bash
docker compose up -d --build postgres grafana admin-ui
```

상태 확인:

```bash
docker compose ps
```

기대 상태:

- `postgres`: healthy
- `grafana`: healthy
- `admin-ui`: running

Schema migration:

```bash
docker compose --profile setup run --rm schema
```

Grafana health:

```bash
GRAFANA_URL="${GRAFANA_URL:-http://127.0.0.1:${GRAFANA_PORT:-3000}}" \
  sh scripts/verify-grafana-health.sh
```

DB health:

```bash
sh scripts/verify-db-health.sh
```

Admin UI unauthenticated check:

```bash
curl -sS -o /dev/null -w '%{http_code}\n' \
  "${ADMIN_URL:-http://127.0.0.1:${ADMIN_HTTP_PORT:-8080}}/"
```

기대값은 `401`입니다.

## 8. AWS profile mount 확인

`admin-ui`와 `collector` container 안에서 shared profile이 보여야 합니다.

```bash
docker compose exec admin-ui sh -lc '
env | grep "^AWS_"
ls -l /aws
grep -Fx "[profile ${AWS_PROFILE:-grafana}]" /aws/config
grep -Fx "[${AWS_PROFILE:-grafana}]" /aws/credentials
'
```

`/aws`가 없거나 profile이 안 보이면 container가 예전 compose 설정으로 떠 있을 수 있습니다.

```bash
docker compose --profile admin-ui up -d --force-recreate --build admin-ui
```

## 9. Resource discovery 실행

브라우저에서 Admin UI를 엽니다.

```text
http://SERVER_ADDRESS:8080/admin
```

`ADMIN_USERNAME`, `ADMIN_PASSWORD`로 로그인합니다.

상단에서 `Discovery 실행`을 누릅니다. 정상 응답 예시:

```json
{"message":"resource discovery completed: resources=2 metrics=43 region=us-east-1","status":"ok"}
```

Discovery 실행 후 화면 테이블은 자동 갱신되지 않을 수 있습니다. 새로고침하거나 `조회`를 누릅니다.

터미널로도 확인할 수 있습니다.

```bash
curl -fsS -u "$ADMIN_USERNAME:$ADMIN_PASSWORD" \
  "${ADMIN_URL:-http://127.0.0.1:${ADMIN_HTTP_PORT:-8080}}/api/resources?region=$AWS_REGION" | jq .
```

resource가 보이면 각 row에서 추천 metric set을 적용합니다.

- EC2: `ec2-default`
- Lambda: `lambda-default`

`추천 적용` 후 다시 새로고침합니다. 아래 `Metric Definition` 섹션에 `enabled` metric definition이 생겨야 합니다.

터미널 확인:

```bash
curl -fsS -u "$ADMIN_USERNAME:$ADMIN_PASSWORD" \
  "${ADMIN_URL:-http://127.0.0.1:${ADMIN_HTTP_PORT:-8080}}/api/metric-definitions?region=$AWS_REGION" | jq .
```

Collector의 실제 수집 기준은 `metric_definitions.enabled = true`입니다.

## 10. Collector 기동과 수집 검증

먼저 one-shot으로 검증합니다.

```bash
docker compose --profile collector run --rm collector /app/collector --once
```

정상 로그 예시:

```text
collector cycle completed: definitions=4 skipped_definitions=0 fetched=... inserted=...
```

daemon으로 기동합니다.

```bash
docker compose --profile collector up -d --build collector
```

로그 확인:

```bash
docker compose logs --tail=100 collector
```

DB 적재 확인:

```bash
docker compose exec postgres psql -U "${POSTGRES_USER:-cloud_monitor}" \
  -d "${POSTGRES_DB:-cloud_monitor}" \
  -c "select count(*) from metric_definitions where enabled = true; select count(*) from metric_points; select max(timestamp) from metric_points;"
```

`metric_definitions`가 0보다 크면 수집 대상이 설정된 것입니다. `metric_points`가 0보다 크면 실제 CloudWatch 수집까지 성공한 상태입니다.

## 11. Grafana 확인

브라우저에서 Grafana를 엽니다.

```text
http://SERVER_ADDRESS:13000
```

로그인:

- user: `GRAFANA_ADMIN_USER`
- password: `GRAFANA_ADMIN_PASSWORD`

Provisioned dashboard:

- Cloud Monitor EC2 MVP
- Cloud Monitor Lambda
- Cloud Monitor AWS Resource
- Cloud Monitor PostgreSQL Ops

Datasource는 `cloud-monitor-postgres` 하나만 사용합니다.

## 12. One-shot jobs와 systemd timer

Alert, summary, retention job은 Docker Compose one-shot job으로 실행됩니다.

수동 실행:

```bash
docker compose --profile jobs run --rm -e SLACK_WEBHOOK_URL= alert-runner
docker compose --profile jobs run --rm summary-rollup
docker compose --profile jobs run --rm retention-job
```

systemd timer 설치:

```bash
sudo cp deploy/systemd/cloud-monitor-alert.service /etc/systemd/system/
sudo cp deploy/systemd/cloud-monitor-alert.timer /etc/systemd/system/
sudo cp deploy/systemd/cloud-monitor-summary.service /etc/systemd/system/
sudo cp deploy/systemd/cloud-monitor-summary.timer /etc/systemd/system/
sudo cp deploy/systemd/cloud-monitor-retention.service /etc/systemd/system/
sudo cp deploy/systemd/cloud-monitor-retention.timer /etc/systemd/system/

sudo systemctl daemon-reload
sudo systemctl enable --now cloud-monitor-alert.timer
sudo systemctl enable --now cloud-monitor-summary.timer
sudo systemctl enable --now cloud-monitor-retention.timer
```

확인:

```bash
systemctl list-timers 'cloud-monitor-*'
journalctl -u cloud-monitor-alert.service -n 100 --no-pager
journalctl -u cloud-monitor-summary.service -n 100 --no-pager
journalctl -u cloud-monitor-retention.service -n 100 --no-pager
```

## 13. Backup

```bash
BACKUP_DIR=/srv/cloud-monitor/backups DATABASE_URL="$DATABASE_URL" \
  sh scripts/backup-postgres.sh
```

backup 파일은 private storage로 복사합니다. repository 내부에 dump 파일을 두지 않습니다.

## 14. Smoke test

초기 설치 후 전체 smoke test를 실행합니다.

```bash
sh scripts/smoke-compose.sh
```

기대값:

```text
compose smoke test passed
```

## 15. Reverse proxy와 공개 경계

Nginx 예시는 다음 파일에 있습니다.

```text
deploy/nginx/cloud-monitor.conf.example
```

운영 기준:

- Grafana는 HTTPS reverse proxy 뒤에서 노출합니다.
- Grafana public dashboard는 read-only로 제한합니다.
- Admin UI는 public internet에 직접 노출하지 않습니다.
- Admin UI는 VPN, 내부망, 또는 고정 IP allowlist 뒤에 둡니다.

## 16. Troubleshooting

### `failed to get shared config profile, grafana`

container가 AWS profile 파일을 못 보고 있습니다.

```bash
docker compose exec admin-ui sh -lc 'env | grep "^AWS_"; ls -l /aws'
docker compose --profile admin-ui up -d --force-recreate --build admin-ui
docker compose --profile collector up -d --force-recreate --build collector
```

host 파일도 확인합니다.

```bash
sudo grep -Fx "[profile grafana]" /etc/cloud-monitor/aws/config
sudo grep -Fx "[grafana]" /etc/cloud-monitor/aws/credentials
```

### Grafana 또는 PostgreSQL port conflict

host에서 이미 `3000` 또는 `5432`를 쓰고 있으면 env file에서 port를 바꿉니다.

```text
POSTGRES_PORT=15432
DATABASE_URL=postgres://cloud_monitor:REPLACE_WITH_POSTGRES_PASSWORD@127.0.0.1:15432/cloud_monitor?sslmode=disable
GRAFANA_PORT=13000
GRAFANA_URL=http://127.0.0.1:13000
```

변경 후 재기동합니다.

```bash
set -a
. /etc/cloud-monitor/cloud-monitor.env
set +a

docker compose up -d --force-recreate postgres grafana admin-ui
```

### PostgreSQL password authentication failed

기존 `postgres-data` volume이 예전 password로 초기화된 상태일 수 있습니다. 운영 데이터가 있으면 volume을 삭제하지 말고 기존 DB password를 확인해 env와 맞춥니다.

새 설치이고 데이터가 필요 없을 때만 volume을 삭제합니다.

```bash
docker compose down --remove-orphans
docker volume rm cloud-monitor_postgres-data
docker compose up -d postgres
docker compose --profile setup run --rm schema
```

### `network ... not found`

Docker Compose network가 stale 상태일 수 있습니다. volume은 삭제하지 말고 container/network만 정리합니다.

```bash
docker compose down --remove-orphans
docker compose up -d --build postgres grafana admin-ui
```

### Discovery는 성공했는데 화면이 안 바뀜

현재 Admin UI는 discovery 성공 후 resource table을 자동 갱신하지 않을 수 있습니다. 새로고침하거나 `조회`를 누릅니다.

### `metric_points`가 계속 0

확인 순서:

1. Admin UI에서 추천 metric set을 적용했는지 확인합니다.
2. `/api/metric-definitions?region=$AWS_REGION`에 `enabled: true` 항목이 있는지 확인합니다.
3. `docker compose --profile collector run --rm collector /app/collector --once`를 실행합니다.
4. CloudWatch에 해당 metric과 dimension이 실제로 존재하는지 확인합니다.
5. Lambda/EC2가 최근 기간에 traffic 또는 metric data point를 냈는지 확인합니다.

