#!/usr/bin/env bash
# dev.sh — nyalakan seluruh stack Tauthub lokal dalam satu perintah.
#
# Kenapa ada skrip ini: proses `go run` dan `bun run dev` yang dijalankan dari
# sesi chat/terminal mati begitu sesi induknya berakhir (SIGTERM). Sepanjang
# 3 Nov 2026 API mati 5 kali dengan pola yang sama, dan dua kali sempat
# tertinggal proses yatim yang memegang port 8080 sehingga instance baru gagal
# dengan "bind: address already in use" sambil tetap tampak hidup.
#
# Skrip ini menangani hal itu: bersihkan sisa proses, cek port, nyalakan satu
# instance per service, lalu tunggu sampai benar-benar siap sebelum bilang
# "selesai" — bukan sekadar "sudah dijalankan".
#
# Pakai:
#   ./dev.sh            nyalakan semua yang belum jalan
#   ./dev.sh restart    matikan semua lalu nyalakan ulang
#   ./dev.sh stop       matikan semua
#   ./dev.sh status     lihat keadaan sekarang
#
# Log: $TMPDIR/tauthub-logs/{api,worker,scheduler,web}.log

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
API_DIR="$ROOT/api"
WEB_DIR="$ROOT/web"
LOG_DIR="${TMPDIR:-/tmp}/tauthub-logs"
API_PORT=8080
WEB_PORT=3001

export PATH="$HOME/sdk/go1.27.1/bin:$HOME/.bun/bin:$PATH"

mkdir -p "$LOG_DIR"

c_g=$'\033[0;32m'; c_r=$'\033[0;31m'; c_y=$'\033[0;33m'; c_b=$'\033[1;34m'; c_0=$'\033[0m'
ok()   { printf '%s✓%s %s\n' "$c_g" "$c_0" "$*"; }
bad()  { printf '%s✗%s %s\n' "$c_r" "$c_0" "$*"; }
warn() { printf '%s!%s %s\n' "$c_y" "$c_0" "$*"; }
step() { printf '\n%s==>%s %s\n' "$c_b" "$c_0" "$*"; }

# port_owner <port> → cetak PID yang LISTEN, kosong bila bebas.
port_owner() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN 2>/dev/null | awk 'NR>1 {print $2; exit}'
}

# kill_matching <pola-pgrep> — matikan proses beserta anak-anaknya.
kill_matching() {
  local pattern="$1"
  pgrep -f "$pattern" 2>/dev/null | while read -r pid; do
    [ -n "$pid" ] || continue
    kill -9 "$pid" 2>/dev/null
  done
}

stop_all() {
  step "Menghentikan proses lama"
  # Urutan penting: matikan pembungkus `go run` dan binary hasil build-nya,
  # termasuk proses yatim yang ppid-nya sudah 1 (tidak terlihat pgrep -f 'go run').
  kill_matching 'go run ./cmd/api'
  kill_matching 'go run ./cmd/worker'
  kill_matching 'go run ./cmd/scheduler'
  kill_matching 'exe/api'
  kill_matching 'exe/worker'
  kill_matching 'exe/scheduler'
  kill_matching 'bun run dev'
  kill_matching 'vite'
  sleep 3
  # Sisa yang masih memegang port (bisa proses yatim dengan nama lain).
  for p in "$API_PORT" "$WEB_PORT"; do
    local pid; pid="$(port_owner "$p")"
    if [ -n "$pid" ]; then
      warn "port $p masih dipegang pid $pid — dihentikan paksa"
      kill -9 "$pid" 2>/dev/null
    fi
  done
  sleep 2
  ok "semua proses lama dihentikan"
}

show_status() {
  step "Status"
  local any=0
  for p in "$API_PORT" "$WEB_PORT"; do
    local pid; pid="$(port_owner "$p")"
    if [ -n "$pid" ]; then
      ok "port $p — pid $pid"
      any=1
    else
      bad "port $p — kosong"
    fi
  done
  if [ "$any" = 1 ]; then
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 4 "http://127.0.0.1:$API_PORT/healthz" 2>/dev/null)"
    [ "$code" = "200" ] && ok "healthz API $code" || bad "healthz API ${code:-000}"
    code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 6 "http://localhost:$WEB_PORT/" 2>/dev/null)"
    [ "$code" = "200" ] && ok "web HTTP $code" || bad "web HTTP ${code:-000}"
  fi
  printf '\nLog: %s\n' "$LOG_DIR"
}

# wait_http <url> <detik> — tunggu sampai ada respons HTTP apa pun.
wait_http() {
  local url="$1" limit="${2:-90}" i=0
  while [ "$i" -lt "$limit" ]; do
    local code
    code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 3 "$url" 2>/dev/null)"
    if [ -n "$code" ] && [ "$code" != "000" ]; then return 0; fi
    sleep 2
    i=$((i + 2))
  done
  return 1
}

start_api() {
  if [ -n "$(port_owner "$API_PORT")" ]; then
    ok "API sudah jalan di :$API_PORT"
    return 0
  fi
  step "Menyalakan API (: $API_PORT)"
  ( cd "$API_DIR" && nohup go run ./cmd/api >"$LOG_DIR/api.log" 2>&1 & )
  if wait_http "http://127.0.0.1:$API_PORT/healthz" 120; then
    local code; code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 4 "http://127.0.0.1:$API_PORT/healthz")"
    if [ "$code" = "200" ]; then ok "API sehat (healthz 200)"; else warn "API menjawab HTTP $code"; fi
  else
    bad "API tidak merespons. 20 baris terakhir:"
    tail -20 "$LOG_DIR/api.log" 2>/dev/null
    return 1
  fi
}

start_worker_scheduler() {
  step "Menyalakan worker & scheduler"
  if pgrep -f 'go run ./cmd/worker' >/dev/null 2>&1; then
    ok "worker sudah jalan"
  else
    ( cd "$API_DIR" && nohup go run ./cmd/worker >"$LOG_DIR/worker.log" 2>&1 & )
    ok "worker dijalankan"
  fi
  if pgrep -f 'go run ./cmd/scheduler' >/dev/null 2>&1; then
    ok "scheduler sudah jalan"
  else
    ( cd "$API_DIR" && nohup go run ./cmd/scheduler >"$LOG_DIR/scheduler.log" 2>&1 & )
    ok "scheduler dijalankan"
  fi
  sleep 2
  # Verifikasi dari log, bukan dari exit code: `go run` langsung "sukses"
  # walau proses di dalamnya kemudian mati.
  grep -q 'worker aktif' "$LOG_DIR/worker.log" 2>/dev/null && ok "worker aktif (log)" || warn "worker belum tercatat aktif"
  grep -q 'scheduler aktif' "$LOG_DIR/scheduler.log" 2>/dev/null && ok "scheduler aktif (log)" || warn "scheduler belum tercatat aktif"
}

start_web() {
  if [ -n "$(port_owner "$WEB_PORT")" ]; then
    ok "web sudah jalan di :$WEB_PORT"
    return 0
  fi
  step "Menyalakan web (Vite :$WEB_PORT)"
  if [ ! -d "$WEB_DIR/node_modules" ]; then
    warn "node_modules belum ada — jalankan: cd web && bun install"
    return 1
  fi
  ( cd "$WEB_DIR" && nohup bun run dev >"$LOG_DIR/web.log" 2>&1 & )
  if wait_http "http://localhost:$WEB_PORT/" 120; then
    ok "web siap di http://localhost:$WEB_PORT"
  else
    bad "web tidak merespons. 20 baris terakhir:"
    tail -20 "$LOG_DIR/web.log" 2>/dev/null
    return 1
  fi
}

case "${1:-start}" in
  stop)
    stop_all
    show_status
    ;;
  status)
    show_status
    ;;
  restart)
    stop_all
    start_api && start_worker_scheduler && start_web
    echo
    show_status
    ;;
  start)
    start_api && start_worker_scheduler && start_web
    echo
    show_status
    ;;
  *)
    echo "Pakai: $0 [start|restart|stop|status]" >&2
    exit 2
    ;;
esac
