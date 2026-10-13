#!/bin/sh
# Copies the project and the current database (history, chat, accounts...) to the VPS, then starts it there.
#   sh deploy/push.sh root@IP
# Stop the local monitor afterwards (docker compose down): two monitors would both send the scheduled commands.
set -eu
VPS=${1:?usage: sh deploy/push.sh root@IP}
DIR=/opt/cod4-monitor
cd "$(dirname "$0")/.."
grep -q '^DOMAIN=.' .env || { echo "ajoute DOMAIN=ton-nom.duckdns.org dans .env"; exit 1; }
# the dashboard will be on the internet: never deploy it without a login
grep -q '^DASHBOARD_PASSWORD=.' .env || { echo "mets un mot de passe fort dans DASHBOARD_PASSWORD (.env)"; exit 1; }

echo "1/4 sauvegarde de la base locale"
mkdir -p backups && rm -f backups/transfer.db
# local volume = <compose project name>_data (the project name comes from the folder name)
LOCAL_VOLUME=${LOCAL_VOLUME:-$(docker compose config 2>/dev/null | sed -n 's/^name: //p')_data}
if docker volume inspect "$LOCAL_VOLUME" >/dev/null 2>&1; then
  docker run --rm -v "$LOCAL_VOLUME":/data -v "$PWD/backups":/b alpine sh -c \
    "apk add -q sqlite >/dev/null && sqlite3 /data/monitor.db \"VACUUM INTO '/b/transfer.db'\""
else
  echo "pas de base locale ($LOCAL_VOLUME) : le VPS démarre avec une base vide"
fi

echo "2/4 envoi du projet"
# backups/ must belong to the container user (uid 1000), or Docker creates it as root and daily backups fail
tar czf - --exclude=./backups --exclude=./.git --exclude='*.db' --exclude='*.db-*' . | ssh "$VPS" \
  "mkdir -p $DIR/backups && chown 1000 $DIR/backups && tar xzf - -C $DIR && chmod 600 $DIR/.env"
if [ -f backups/transfer.db ]; then
  scp -q backups/transfer.db "$VPS:$DIR/transfer.db"

  echo "3/4 import de la base (seulement si le VPS n'en a pas encore)"
  ssh "$VPS" "cd $DIR && docker volume create cod4-monitor_data >/dev/null && \
    docker run --rm -v cod4-monitor_data:/data -v $DIR:/in alpine sh -c 'test -f /data/monitor.db && echo \"base déjà présente, gardée\" || { cp /in/transfer.db /data/monitor.db && chown -R 1000 /data; }' && rm transfer.db"
else
  echo "3/4 import de la base : rien à importer"
fi

echo "4/4 démarrage"
ssh "$VPS" "cd $DIR && docker compose -p cod4-monitor -f docker-compose.prod.yml up -d --build && docker compose -p cod4-monitor -f docker-compose.prod.yml ps"
rm -f backups/transfer.db
echo "En ligne sur https://$(grep '^DOMAIN=' .env | cut -d= -f2)"
