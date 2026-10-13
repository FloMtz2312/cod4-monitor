#!/bin/sh
# Prepares a fresh Debian / Ubuntu VPS (run once, as root): Docker, firewall, automatic security updates.
#   ssh root@IP 'sh -s' < deploy/setup-vps.sh
set -eu
export DEBIAN_FRONTEND=noninteractive
apt-get update -q
apt-get install -yq ca-certificates curl ufw unattended-upgrades
command -v docker >/dev/null || curl -fsSL https://get.docker.com | sh
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable
dpkg-reconfigure -f noninteractive unattended-upgrades
mkdir -p /opt/cod4-monitor
echo "VPS prêt : Docker $(docker --version | cut -d' ' -f3) — pare-feu : SSH, HTTP, HTTPS"
