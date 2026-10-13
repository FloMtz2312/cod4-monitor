#!/bin/bash
# Automatic deployment (optional, README "Automatic deployment from GitHub"): copied to /opt/cod4-monitor/deploy.sh on
# the VPS, run over SSH by .github/workflows/deploy.yml on every push to main. Follows main exactly (.env and the
# data are not tracked by git, so they are kept), rebuilds the monitor and waits until it is healthy.
set -euo pipefail
cd /opt/cod4-monitor
git fetch origin main
git reset --hard origin/main
docker compose -p cod4-monitor -f docker-compose.prod.yml up -d --build
# wait until Docker reports the monitor "healthy" (its healthcheck knows the port), 2 min max
c=$(docker compose -p cod4-monitor -f docker-compose.prod.yml ps -q monitor)
for i in $(seq 1 40); do
  s=$(docker inspect -f '{{.State.Health.Status}}' "$c" 2>/dev/null || echo unknown)
  if [ "$s" = healthy ]; then echo "OK: monitor online"; docker image prune -f >/dev/null; exit 0; fi
  sleep 3
done
echo "ERROR: the monitor is not healthy (state: $s)"
docker compose -p cod4-monitor -f docker-compose.prod.yml logs --tail 30 monitor
exit 1
