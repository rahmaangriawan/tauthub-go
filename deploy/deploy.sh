#!/usr/bin/env bash
# Deploy Tauthub Go di VPS — dipanggil GitHub Actions workflow
# (.github/workflows/deploy.yml) SETELAH kode + binary linux dirsync. Juga
# aman dijalankan manual via SSH. Aman terhadap data:
#   - migrasi memakai runner additive (tidak pernah drop tabel/data)
#   - .env dan .data/ tidak pernah disentuh (rsync meng-exclude keduanya)
#
# Binary di api/bin/{api,scheduler,migrate} dibangun di runner CI
# (CGO_ENABLED=0 GOOS=linux GOARCH=amd64). Web dibangun di VPS (bun).
#
# Env: APP_DIR (default /opt/tauthub/app). Harus dijalankan sebagai root
# (atau user dengan sudo) karena me-restart service systemd.
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/tauthub/app}"
APP_USER="tauthub"

log() { printf '\n\033[1;34m==>\033[0m %s\n' "$*"; }

if [ ! -f "$APP_DIR/.env" ]; then
  echo "$APP_DIR/.env belum ada — jalankan setup-vps.sh dulu." >&2
  exit 1
fi
SUDO=""
if [ "$(id -u)" != "0" ]; then SUDO="sudo"; fi

as_user() {
  if [ "$(id -un)" = "$APP_USER" ]; then
    bash -lc "$1"
  elif [ "$(id -u)" = "0" ]; then
    runuser -u "$APP_USER" -- bash -lc "$1"
  else
    $SUDO -u "$APP_USER" bash -lc "$1"
  fi
}

log "1/4 kepemilikan file -> $APP_USER"
$SUDO chown -R "$APP_USER:$APP_USER" "$APP_DIR"

log "2/4 migrasi DB (additive — toleran duplikat, tidak pernah drop)"
as_user "set -a; . '$APP_DIR/.env'; set +a; cd '$APP_DIR/api' && ./bin/migrate up"

log "3/4 build web (SvelteKit adapter-node)"
as_user "cd '$APP_DIR/web' && bun install --frozen-lockfile && bun run build"

log "4/4 restart service"
$SUDO systemctl restart tauthub-api tauthub-web tauthub-scheduler

# Health check: api /healthz (ping DB) + respons web di :3000.
for i in $(seq 1 30); do
  api_code="$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/healthz || true)"
  web_code="$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:3000/ || true)"
  if [ "$api_code" = "200" ] && [ "$web_code" != "000" ] && [ "$web_code" -lt 500 ]; then
    log "Sehat: api /healthz=200, web HTTP $web_code. Deploy selesai."
    exit 0
  fi
  sleep 2
done
echo "Service belum sehat setelah 60 detik — cek:" >&2
echo "  journalctl -u tauthub-api -u tauthub-web -u tauthub-scheduler -n 50" >&2
exit 1
