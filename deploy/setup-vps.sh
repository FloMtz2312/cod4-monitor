#!/bin/sh
# Prepares a fresh Debian / Ubuntu VPS (run once, as root): Docker, firewall, automatic security updates.
#   ssh root@IP 'sh -s' < deploy/setup-vps.sh
set -eu
export DEBIAN_FRONTEND=noninteractive
apt-get update -q
apt-get install -yq ca-certificates curl ufw unattended-upgrades
# Docker from snap (offered by the Ubuntu installer) cannot read /opt: replace it with the official package.
if command -v snap >/dev/null && snap list docker >/dev/null 2>&1; then snap remove --purge docker; fi
command -v docker >/dev/null || curl -fsSL https://get.docker.com | sh
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
# Other services already reachable from outside (game servers, web sites, SSH on another port…) would be cut off
# by the firewall: in that case leave it off and let the admin open their ports first.
others=$(ss -Htuln | awk '{print $5}' | grep -vE '^(127\.|\[::1\]|::1)' | sed 's/.*://' | grep -vxE '22|53|68|80|443|546' | sort -un | tr '\n' ' ')
if ufw status | grep -q 'Status: active'; then
  firewall="déjà actif, ports 80 et 443 ajoutés"
elif [ -n "$others" ]; then
  firewall="NON activé : d'autres services écoutent sur les ports $others(voir README)"
else
  ufw --force enable
  firewall="SSH, HTTP, HTTPS"
fi
dpkg-reconfigure -f noninteractive unattended-upgrades
mkdir -p /opt/cod4-monitor
echo "VPS prêt : Docker $(docker --version | cut -d' ' -f3) — pare-feu : $firewall"
