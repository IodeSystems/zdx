#!/usr/bin/env bash
# Idempotent provisioning for zdx (Go)
# Run as root: sudo bash provision.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_USER="zdx"
APP_HOME="/home/$APP_USER"

echo "=== zdx Provision ==="

########################################################################
# 1. System packages
########################################################################
if ! command -v docker &>/dev/null; then
  echo "  Installing Docker..."
  curl -fsSL https://get.docker.com | sh
fi

for pkg in rsync curl; do
  if ! command -v "$pkg" &>/dev/null; then
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "$pkg" > /dev/null
  fi
done

echo "  System packages OK."

########################################################################
# 2. App user
########################################################################
if ! id "$APP_USER" &>/dev/null; then
  echo "  Creating user $APP_USER..."
  useradd -m -s /bin/bash "$APP_USER"
fi

if ! groups "$APP_USER" | grep -q docker; then
  echo "  Adding $APP_USER to docker group..."
  usermod -aG docker "$APP_USER"
fi

echo "  User $APP_USER OK."

########################################################################
# 3. Home directory structure
########################################################################
sudo -u "$APP_USER" mkdir -p \
  "$APP_HOME/home/versions/current/public" \
  "$APP_HOME/home/versions/next/public" \
  "$APP_HOME/home/data/postgres" \
  "$APP_HOME/home/data/valkey" \
  "$APP_HOME/home/log" \
  "$APP_HOME/home/var" \
  "$APP_HOME/home/files" \
  "$APP_HOME/home/provision"

# Repair ownership of the directories bin/ship's sudo-as-zdx rsync writes to.
# Prior ship runs (or unrelated hands) may have left entries under these
# trees owned by a non-zdx UID — e.g. rsync -a preserving the local
# operator's UID, which happens to collide with a local user on the
# server. Subsequent sudo-as-zdx rsyncs then fail with EACCES on mkstemp.
#
# IMPORTANT: do NOT chown $APP_HOME/home/data/* — those dirs are volumes
# for docker services (postgres UID 999, valkey UID 999) and rechowning
# to zdx breaks those services on the next container restart.
for dir in provision versions/current versions/next; do
  chown -R "$APP_USER:$APP_USER" "$APP_HOME/home/$dir"
done

echo "  Home directories OK."

########################################################################
# 4. Docker compose (PG + Valkey)
########################################################################
if [ ! -f "$APP_HOME/docker-compose.yml" ] || \
   ! diff -q "$SCRIPT_DIR/files/etc/docker-compose.yml" "$APP_HOME/docker-compose.yml" &>/dev/null; then
  echo "  Installing docker-compose.yml..."
  cp "$SCRIPT_DIR/files/etc/docker-compose.yml" "$APP_HOME/docker-compose.yml"
  chown "$APP_USER:$APP_USER" "$APP_HOME/docker-compose.yml"
fi

sudo -u "$APP_USER" docker compose -f "$APP_HOME/docker-compose.yml" up -d 2>/dev/null

echo "  Postgres + Valkey OK."

########################################################################
# 5. Systemd services (blue/green: current on 7600, next on 7602)
########################################################################
RELOAD_SYSTEMD=false

for svc in zdx-current zdx-next; do
  SVC_SRC="$SCRIPT_DIR/files/etc/systemd/$svc.service"
  SVC_DEST="/etc/systemd/system/$svc.service"
  if [ ! -f "$SVC_DEST" ] || ! diff -q "$SVC_SRC" "$SVC_DEST" &>/dev/null; then
    echo "  Installing $svc.service..."
    cp "$SVC_SRC" "$SVC_DEST"
    RELOAD_SYSTEMD=true
  fi
done

if $RELOAD_SYSTEMD; then
  systemctl daemon-reload
fi

systemctl enable zdx-current 2>/dev/null || true

echo "  Systemd services OK (current:7600, next:7602)."

########################################################################
# 6. Start current if deployed (skip during rolling deploys)
########################################################################
if [ "${SKIP_SERVICE_START:-}" = "true" ]; then
  echo "  Skipping service start (rolling deploy)."
elif [ -f "$APP_HOME/home/versions/current/dx-server" ]; then
  echo "  Starting zdx-current..."
  systemctl restart zdx-current
  sleep 3
  if systemctl is-active --quiet zdx-current; then
    echo "  zdx-current running on :7600."
  else
    echo "  WARNING: zdx-current failed to start."
    journalctl -u zdx-current --no-pager -n 10
  fi
else
  echo "  No version deployed yet. Run bin/ship to deploy."
fi

echo "=== Provision complete ==="
