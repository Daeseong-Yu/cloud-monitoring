#!/bin/sh
set -eu

systemd_dir="deploy/systemd"

test -f docs/operations/scheduling.md
test -f "$systemd_dir/cloud-monitor-alert.service"
test -f "$systemd_dir/cloud-monitor-alert.timer"
test -f "$systemd_dir/cloud-monitor-summary.service"
test -f "$systemd_dir/cloud-monitor-summary.timer"
test -f "$systemd_dir/cloud-monitor-retention.service"
test -f "$systemd_dir/cloud-monitor-retention.timer"

sh -n scripts/validate-step16.sh

grep -q 'docker compose --profile collector up -d collector' docs/operations/scheduling.md
grep -q 'systemd timer' docs/operations/scheduling.md
grep -q '1분' docs/operations/scheduling.md
grep -q 'hourly' docs/operations/scheduling.md
grep -q 'daily' docs/operations/scheduling.md
grep -q 'Summary rollup은 retention보다 먼저 실행한다' docs/operations/scheduling.md
grep -q 'systemctl status' docs/operations/scheduling.md
grep -q 'journalctl -u' docs/operations/scheduling.md
grep -q 'enabled metric 수' docs/operations/scheduling.md
grep -q 'COLLECTOR_INTERVAL_SECONDS' docs/operations/scheduling.md

for service in alert summary retention; do
  unit="$systemd_dir/cloud-monitor-$service.service"
  grep -q '^Type=oneshot$' "$unit"
  grep -q '^WorkingDirectory=/opt/cloud-monitor$' "$unit"
  grep -q '^EnvironmentFile=/etc/cloud-monitor/cloud-monitor.env$' "$unit"
  grep -q '^ExecStart=/usr/bin/docker compose --profile jobs run --rm ' "$unit"
done

grep -q ' alert-runner$' "$systemd_dir/cloud-monitor-alert.service"
grep -q ' summary-rollup$' "$systemd_dir/cloud-monitor-summary.service"
grep -q ' retention-job$' "$systemd_dir/cloud-monitor-retention.service"

grep -q '^OnUnitActiveSec=1min$' "$systemd_dir/cloud-monitor-alert.timer"
grep -q '^OnCalendar=hourly$' "$systemd_dir/cloud-monitor-summary.timer"
grep -q '^OnCalendar=daily$' "$systemd_dir/cloud-monitor-retention.timer"

for timer in alert summary retention; do
  unit="$systemd_dir/cloud-monitor-$timer.timer"
  grep -q '^Persistent=true$' "$unit"
  grep -q '^WantedBy=timers.target$' "$unit"
done

if grep -R -E 'cron|crontab' docs/operations/scheduling.md deploy/systemd >/dev/null; then
  echo "cron fallback must not be introduced in Step 16" >&2
  exit 1
fi

grep -q 'COLLECTOR_INTERVAL_SECONDS=${COLLECTOR_INTERVAL_SECONDS:-60}' docker-compose.yml
if grep -nE 'COLLECTOR_INTERVAL_SECONDS=\$\{COLLECTOR_INTERVAL_SECONDS:-([1-9]|[1-5][0-9])\}' docker-compose.yml >/dev/null; then
  echo "collector interval must not be below 60 seconds" >&2
  exit 1
fi

docker compose --profile collector config >/dev/null
docker compose --profile jobs config >/dev/null

sh scripts/validate-common.sh

echo "step16 validation passed"
