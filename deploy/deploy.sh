#!/usr/bin/env bash
# Deploy Tauthub Go di VPS — dipanggil GitHub Actions workflow
# (.github/workflows/deploy.yml) SETELAH kode + binary linux dirsync. Juga
# aman dijalankan manual via SSH. Aman terhadap data:
#   - migrasi memakai runner additive (tidak pernah drop tabel/data)
#   - .env dan .data/ tidak pernah disentuh (rsync meng-exclude keduanya)
#
# Binary di api/bin/{api,scheduler,worker,migrate} dibangun di runner CI
# (CGO_ENABLED=0 GOOS=linux GOARCH=amd64). Web dibangun di VPS (bun).
#
# Env: APP_DIR (default /opt/tauthub/app). Harus dijalankan sebagai root
# (atau user dengan sudo) karena me-restart service systemd.
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/tauthub/app}"
APP_USER="${APP_USER:-tauthub}"
# Prefix unit systemd. Host polos: tauthub. Di host CloudPanel yang sudah
# menjalankan production tauthub-*, staging memakai tauthub-staging supaya unit
# production tidak pernah tertimpa atau ter-restart oleh deploy ini.
UNIT_PREFIX="${UNIT_PREFIX:-tauthub}"

log() { printf '\n\033[1;34m==>\033[0m %s\n' "$*"; }

if [ ! -f "$APP_DIR/.env" ]; then
  echo "$APP_DIR/.env belum ada — jalankan setup-vps.sh dulu." >&2
  exit 1
fi
# Port health check ikut .env (API_PORT ditulis setup-vps.sh); WEB_PORT tidak
# ada di .env — baca dari unit ${UNIT_PREFIX}-web (di-sed per environment oleh
# setup-vps.sh), fallback ke unit default :3000.
set -a; . "$APP_DIR/.env"; set +a
API_PORT="${API_PORT:-8080}"
WEB_PORT="${WEB_PORT:-$(systemctl cat "${UNIT_PREFIX}-web" 2>/dev/null | sed -n 's/^Environment=PORT=//p' | tail -1)}"
WEB_PORT="${WEB_PORT:-3000}"
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
$SUDO systemctl restart "${UNIT_PREFIX}-api" "${UNIT_PREFIX}-web" "${UNIT_PREFIX}-scheduler" "${UNIT_PREFIX}-worker"

# Health check: api /healthz (ping DB) + respons web di :$WEB_PORT.
for i in $(seq 1 30); do
  api_code="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$API_PORT/healthz" || true)"
  web_code="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$WEB_PORT/" || true)"
  if [ "$api_code" = "200" ] && [ "$web_code" != "000" ] && [ "$web_code" -lt 500 ]; then
    log "Sehat: api /healthz=200, web HTTP $web_code. Deploy selesai."
    exit 0
  fi
  sleep 2
done
echo "Service belum sehat setelah 60 detik — cek:" >&2
echo "  journalctl -u ${UNIT_PREFIX}-api -u ${UNIT_PREFIX}-web -u ${UNIT_PREFIX}-scheduler -u ${UNIT_PREFIX}-worker -n 50" >&2
exit 1
