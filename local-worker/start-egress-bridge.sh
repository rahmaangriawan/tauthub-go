#!/usr/bin/env bash
# Nyalakan egress bridge lokal: proxy (Mac) + SSH reverse tunnel ke VPS.
# Jalankan dari mana saja: bash local-worker/start-egress-bridge.sh
#
# Env opsional:
#   CRAWL_EGRESS_TOKEN  (default: dibaca dari api/.env)
#   CF_WORKER_URL       (default: worker tauthub-crawl-egress)
#   VPS_SSH             (default: ubuntu@43.156.237.97)
set -euo pipefail
cd "$(dirname "$0")/.."

TOKEN="${CRAWL_EGRESS_TOKEN:-$(grep -E '^CRAWL_EGRESS_TOKEN=' api/.env | head -1 | cut -d= -f2-)}"
CF_URL="${CF_WORKER_URL:-https://tauthub-crawl-egress.rahmaangriawan78.workers.dev}"
VPS="${VPS_SSH:-ubuntu@43.156.237.97}"
BUN="${BUN:-$HOME/.bun/bin/bun}"

if [ -z "$TOKEN" ]; then
  echo "CRAWL_EGRESS_TOKEN kosong (isi di api/.env atau env)." >&2
  exit 1
fi

if lsof -nP -iTCP:8787 -sTCP:LISTEN >/dev/null 2>&1; then
  echo "proxy sudah jalan di :8787"
else
  CRAWL_EGRESS_TOKEN="$TOKEN" CF_WORKER_URL="$CF_URL" nohup "$BUN" run local-worker/egress-proxy.ts \
    > /tmp/tauthub-egress-proxy.log 2>&1 &
  echo "proxy dimulai (log /tmp/tauthub-egress-proxy.log)"
fi

if pgrep -f "ssh -N -R 127.0.0.1:8787:127.0.0.1:8787" >/dev/null 2>&1; then
  echo "tunnel sudah jalan"
else
  nohup ssh -N -R 127.0.0.1:8787:127.0.0.1:8787 \
    -o BatchMode=yes -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o ExitOnForwardFailure=yes \
    "$VPS" > /tmp/tauthub-egress-tunnel.log 2>&1 &
  echo "tunnel dimulai (log /tmp/tauthub-egress-tunnel.log)"
fi

sleep 2
echo -n "health: "
curl -s --max-time 5 http://127.0.0.1:8787/health || true
echo
