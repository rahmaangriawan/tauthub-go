#!/usr/bin/env bash
# Bootstrap VPS Tauthub Go — dijalankan di VPS (root/sudo) oleh GitHub
# Actions workflow SEBELUM deploy, atau manual sekali via SSH. IDEMPOTEN:
# aman dijalankan berulang tiap deploy.
#
#   1. Paket sistem: postgresql, nginx, certbot, unzip + Node 22 (runtime web)
#   2. Swap 2GB (build Vite butuh memori di atas RAM 2GB)
#   3. Bun (build web) di /usr/local/bin/bun
#   4. User sistem $APP_USER pemilik $APP_DIR
#   5. .env di $APP_DIR/.env — dibuat SEKALI dengan secret acak, tidak pernah
#      disentuh lagi (rsync workflow meng-exclude .env)
#   6. Database PostgreSQL: role + db, lalu baseline schema-full.sql (159
#      tabel) hanya saat DB masih kosong
#   7. Unit systemd: tauthub-api, tauthub-web, tauthub-scheduler
#   8. Nginx: $DOMAIN → web :$WEB_PORT, /v1 /uploads /t /feed /healthz
#      → api :8080 + sertifikat Let's Encrypt (jika DNS sudah mengarah ke sini)
#
# Env (semua opsional, default = staging):
#   APP_DIR       default /opt/tauthub/app
#   DOMAIN        default staging.tauthub.com
#   APP_USER      default tauthub
#   WEB_PORT      default 3000   (port tauthub-web SvelteKit)
#   ORIGIN        default https://$DOMAIN (ORIGIN SvelteKit: CSRF/cookie)
#   MANAGE_SWAP   default 1      (0 = jangan buat /swapfile — host sudah punya)
#   MANAGE_NGINX  default 1      (0 = nginx/TLS dikelola CloudPanel — JANGAN
#                                sentuh vhost & certbot, site sudah didaftarkan
#                                lewat clpctl di production)
#   UNIT_PREFIX   default tauthub (prefix nama unit systemd. Di host CloudPanel
#                                yang sudah punya production tauthub-*, staging
#                                memakai tauthub-staging)
#   SKIP_APT      default 0      (1 = lewati apt install — paket sudah lengkap,
#                                hindari menyentuh nginx milik CloudPanel)
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/tauthub/app}"
DOMAIN="${DOMAIN:-staging.tauthub.com}"
APP_USER="${APP_USER:-tauthub}"
UNIT_PREFIX="${UNIT_PREFIX:-tauthub}"
WEB_PORT="${WEB_PORT:-3000}"
ORIGIN="${ORIGIN:-https://$DOMAIN}"
MANAGE_SWAP="${MANAGE_SWAP:-1}"
MANAGE_NGINX="${MANAGE_NGINX:-1}"
SKIP_APT="${SKIP_APT:-0}"

log() { printf '\n\033[1;34m==>\033[0m %s\n' "$*"; }

if [ ! -d "$APP_DIR/api" ]; then
  echo "api/ tidak ditemukan di $APP_DIR — sinkronkan kode dulu (rsync workflow)." >&2
  exit 1
fi
SUDO=""
if [ "$(id -u)" != "0" ]; then SUDO="sudo"; fi

# Helper PostgreSQL — $SUDO kosong + argumen "-u" tidak valid sebagai command,
# jadi panggilan -u postgres dibungkus: root pakai runuser, selain itu sudo.
pg_psql() {
  if [ "$(id -u)" = "0" ]; then runuser -u postgres -- psql "$@"; else sudo -u postgres psql "$@"; fi
}
pg_createdb() {
  if [ "$(id -u)" = "0" ]; then runuser -u postgres -- createdb "$@"; else sudo -u postgres createdb "$@"; fi
}

log "1/8 Paket sistem (postgresql, nginx, certbot, unzip)"
if [ "$SKIP_APT" != "1" ]; then
  export DEBIAN_FRONTEND=noninteractive
  $SUDO apt-get update -qq
  $SUDO apt-get install -y -qq postgresql nginx rsync certbot python3-certbot-nginx unzip ca-certificates curl >/dev/null
else
  echo "  SKIP_APT=1 — paket diasumsikan sudah terpasang (host CloudPanel)."
fi

# Swap 2GB: build Vite/rollup bisa melebihi RAM 2GB VPS — tanpa swap proses
# build dibunuh OOM killer. Idempoten: lewati jika /swapfile sudah ada.
if [ "$MANAGE_SWAP" = "1" ] && [ ! -f /swapfile ]; then
  $SUDO fallocate -l 2G /swapfile
  $SUDO chmod 600 /swapfile
  $SUDO mkswap /swapfile >/dev/null
  $SUDO swapon /swapfile
  grep -q '^/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' | $SUDO tee -a /etc/fstab >/dev/null
fi

log "2/8 Node 22 (runtime adapter-node) + Bun (build web)"
if ! command -v node >/dev/null 2>&1 || [ "$(node -v | cut -dv -f2 | cut -d. -f1)" -lt 20 ]; then
  curl -fsSL https://deb.nodesource.com/setup_22.x | $SUDO bash - >/dev/null
  $SUDO apt-get install -y -qq nodejs >/dev/null
fi
if [ ! -x /usr/local/bin/bun ]; then
  curl -fsSL https://bun.sh/install | BUN_INSTALL=/tmp/bun-install bash >/dev/null
  $SUDO cp /tmp/bun-install/bin/bun /usr/local/bin/bun
  $SUDO chmod +x /usr/local/bin/bun
fi

log "3/8 User sistem $APP_USER + kepemilikan $APP_DIR"
id -u "$APP_USER" >/dev/null 2>&1 || $SUDO useradd --system --create-home --shell /bin/bash "$APP_USER"
$SUDO chown -R "$APP_USER:$APP_USER" "$APP_DIR"

log "4/8 File .env"
ENV_FILE="$APP_DIR/.env"
if [ ! -f "$ENV_FILE" ]; then
  $SUDO tee "$ENV_FILE" >/dev/null <<EOF
# Tauthub (backend Go) — dibuat otomatis oleh deploy/setup-vps.sh.
# Isi nilai CHANGE_ME (AI provider, Google OAuth) lalu deploy ulang.
APP_ENV=production
APP_NAME=Tauthub
APP_VERSION=0.2.0-go
API_PORT=8080

APP_SECRET=$(openssl rand -hex 32)
CMS_ENCRYPTION_SECRET=$(openssl rand -hex 32)
ACCESS_TOKEN_MINUTES=30
REFRESH_TOKEN_MINUTES=43200

DB_HOST=127.0.0.1
DB_PORT=5432
DB_NAME=tauthub_go
DB_USER=$APP_USER
DB_PASSWORD=$(openssl rand -hex 16)
DB_POOL_SIZE=10

PLATFORM_ADMIN_EMAILS=wantek@gmail.com
AUTH_RATE_LIMIT_PER_MINUTE=10
WRITE_RATE_LIMIT_PER_MINUTE=120
SSR_GATE_RATE_LIMIT_PER_MINUTE=120

# JOB_INLINE=false: job domain content tidak jalan inline — antrean hanya
# terkuras oleh tauthub-scheduler.service (wajib hidup, lihat .env.example).
JOB_INLINE=false
SCHEDULER_INTERVAL_SECONDS=10
SCHEDULER_MAX_JOBS=20
WORKER_POLL_SECONDS=5
JOB_RUNNER_KEY=$(openssl rand -hex 24)
CRON_SECRET=$(openssl rand -hex 24)

AI_PROVIDER=sumopod
AI_BASE_URL=CHANGE_ME
AI_API_KEY=CHANGE_ME
AI_DEFAULT_MODEL=CHANGE_ME
AI_ECONOMY_MODEL=CHANGE_ME
AI_PREMIUM_MODEL=CHANGE_ME
AI_EMBEDDING_MODEL=CHANGE_ME
AI_REQUEST_TIMEOUT_SECONDS=120
AI_MAX_RETRIES=2

NEXT_PUBLIC_APP_URL=https://$DOMAIN
GOOGLE_CLIENT_ID=CHANGE_ME
GOOGLE_CLIENT_SECRET=CHANGE_ME
GOOGLE_REDIRECT_URI=https://$DOMAIN/login/google-callback
EOF
  $SUDO chmod 600 "$ENV_FILE"
  $SUDO chown "$APP_USER:$APP_USER" "$ENV_FILE"
  echo "  .env dibuat. WAJIB isi nilai CHANGE_ME (AI_API_KEY, GOOGLE_*):"
  echo "    nano $ENV_FILE"
else
  echo "  .env sudah ada — tidak disentuh."
fi
# shellcheck disable=SC1090
set -a; . "$ENV_FILE"; set +a

# Helper: jalankan command sebagai user aplikasi.
run_as_app() {
  if [ "$(id -u)" = "0" ]; then runuser -u "$APP_USER" -- bash -c "$1"; else sudo -u "$APP_USER" bash -c "$1"; fi
}

log "5/8 Database PostgreSQL (role + db + baseline schema)"
$SUDO systemctl enable --now postgresql
if ! pg_psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='$DB_USER'" | grep -q 1; then
  pg_psql -c "CREATE ROLE $DB_USER LOGIN PASSWORD '$DB_PASSWORD';"
else
  # Jangan me-reset password role milik deployment LAIN: kalau role ini memiliki
  # database selain $DB_NAME, artinya dia dipakai aplikasi lain di host yang sama
  # (mis. production). Hentikan kecuali FORCE=1.
  owns_other="$(pg_psql -tAc "SELECT 1 FROM pg_database WHERE datdba = (SELECT oid FROM pg_roles WHERE rolname='$DB_USER') AND datname <> '$DB_NAME' LIMIT 1")"
  if [ "$owns_other" = "1" ] && [ "${FORCE:-0}" != "1" ]; then
    echo "REFUSED: role $DB_USER memiliki database lain di host ini (bukan $DB_NAME)." >&2
    echo "  Kelihatannya milik deployment lain — pakai FORCE=1 hanya kalau yakin." >&2
    exit 1
  fi
  pg_psql -c "ALTER ROLE $DB_USER WITH LOGIN PASSWORD '$DB_PASSWORD';"
fi
if ! pg_psql -tAc "SELECT 1 FROM pg_database WHERE datname='$DB_NAME'" | grep -q 1; then
  pg_createdb -O "$DB_USER" "$DB_NAME"
fi
# Baseline hanya saat DB masih kosong: schema-full.sql = dump schema DB yang
# menjadi acuan sqlc/Go API (159 tabel). Migrasi yang sudah tercakup di dump
# ditandai applied tanpa dieksekusi (deploy/prebaseline.txt, dihitung dari
# riwayat git vs regenerasi terakhir schema-full.sql); `migrate up` di
# deploy.sh hanya mengeksekusi migrasi pasca-dump. JANGAN replay seluruh
# riwayat migrasi di atas dump — state antaranya tidak cocok (drift).
if [ "$(pg_psql -d "$DB_NAME" -tAc "SELECT 1 FROM pg_tables WHERE schemaname='public' AND tablename='users'")" != "1" ]; then
  log "  DB kosong — terapkan api/migrations/schema-full.sql"
  # Salin dulu ke /tmp: di host CloudPanel home site user 750, postgres tidak
  # bisa membaca file langsung dari $APP_DIR.
  SCHEMA_TMP="/tmp/tauthub-schema-full.$$.sql"
  $SUDO install -m 644 "$APP_DIR/api/migrations/schema-full.sql" "$SCHEMA_TMP"
  pg_psql -d "$DB_NAME" -v ON_ERROR_STOP=1 -f "$SCHEMA_TMP" >/dev/null
  $SUDO rm -f "$SCHEMA_TMP"
  # Buku besar migrasi (runner Go tidak membuat tabel ini).
  pg_psql -d "$DB_NAME" -c "CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());" >/dev/null
  # Dump diterapkan sebagai superuser postgres — pindahkan kepemilikan semua
  # objek di schema public ke role aplikasi (REASSIGN OWNED global tidak bisa:
  # menabrak objek sistem milik postgres).
  pg_psql -d "$DB_NAME" <<EOSQL
ALTER SCHEMA public OWNER TO $DB_USER;
SELECT format('ALTER TABLE public.%I OWNER TO $DB_USER', c.relname) FROM pg_class c
  JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname = 'public' AND c.relkind IN ('r','p','f')
    AND pg_get_userbyid(c.relowner) = 'postgres' \gexec
SELECT format('ALTER SEQUENCE public.%I OWNER TO $DB_USER', c.relname) FROM pg_class c
  JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname = 'public' AND c.relkind = 'S'
    AND pg_get_userbyid(c.relowner) = 'postgres' \gexec
SELECT format('ALTER VIEW public.%I OWNER TO $DB_USER', c.relname) FROM pg_class c
  JOIN pg_namespace n ON n.oid = c.relnamespace
  WHERE n.nspname = 'public' AND c.relkind IN ('v','m')
    AND pg_get_userbyid(c.relowner) = 'postgres' \gexec
SELECT format('ALTER FUNCTION %s OWNER TO $DB_USER', (p.oid::regprocedure)::text) FROM pg_proc p
  JOIN pg_namespace n ON n.oid = p.pronamespace
  WHERE n.nspname = 'public' AND pg_get_userbyid(p.proowner) = 'postgres' \gexec
EOSQL
  # Tandai migrasi pra-baseline sebagai applied (tanpa eksekusi — objeknya
  # sudah ada di dump). Sisanya (pasca-dump) dieksekusi `migrate up`.
  log "  Tandai migrasi pra-baseline sebagai applied (deploy/prebaseline.txt)"
  run_as_app "cd '$APP_DIR/api' && set -a && . '$APP_DIR/.env' && set +a && xargs -r -a '$APP_DIR/deploy/prebaseline.txt' ./bin/migrate mark" >/dev/null
fi

log "6/8 Unit systemd (${UNIT_PREFIX}-api, ${UNIT_PREFIX}-web, ${UNIT_PREFIX}-scheduler)"
# Template di repo tetap bernama tauthub-<svc>.service; nama unit di host mengikuti
# UNIT_PREFIX supaya bisa berdampingan dengan production tauthub-* di host yang sama.
for svc in api web scheduler; do
  target="/etc/systemd/system/${UNIT_PREFIX}-${svc}.service"
  # Unit dianggap milik deployment ini kalau menyebut APP_DIR di mana pun: unit
  # web memakai ExecStart=/usr/bin/node build (APP_DIR hanya muncul di
  # WorkingDirectory/EnvironmentFile), jadi tidak boleh dicek lewat ExecStart saja.
  if [ -f "$target" ] && [ "${FORCE:-0}" != "1" ] && ! grep -qF "$APP_DIR" "$target"; then
    echo "REFUSED: $target sudah ada dan bukan milik $APP_DIR." >&2
    echo "  Set FORCE=1 kalau memang mau menimpanya." >&2
    exit 1
  fi
  sed -e "s|__APP_DIR__|$APP_DIR|g" \
    -e "s|__WEB_PORT__|$WEB_PORT|g" \
    -e "s|__ORIGIN__|$ORIGIN|g" \
    "$APP_DIR/deploy/tauthub-$svc.service" > "$target"
done
$SUDO systemctl daemon-reload
$SUDO systemctl enable "${UNIT_PREFIX}-api" "${UNIT_PREFIX}-web" "${UNIT_PREFIX}-scheduler" >/dev/null 2>&1 || true

# Nginx + TLS hanya dikelola di sini untuk host polos (staging). Di host
# CloudPanel (MANAGE_NGINX=0) vhost & sertifikat dibuat/dikelola lewat
# clpctl — menimpa conf di sini bisa merusak site lain di host yang sama.
if [ "$MANAGE_NGINX" = "1" ]; then
log "7/8 Nginx reverse proxy untuk $DOMAIN"
# Sertifikat: Let's Encrypt bila sudah ada, kalau belum self-signed — port 443
# wajib listen karena Cloudflare SSL mode Full menyambung origin via HTTPS
# (tanpa itu Cloudflare menampilkan error 521).
if [ -d "/etc/letsencrypt/live/$DOMAIN" ]; then
  SSL_CERT="/etc/letsencrypt/live/$DOMAIN/fullchain.pem"
  SSL_KEY="/etc/letsencrypt/live/$DOMAIN/privkey.pem"
else
  $SUDO mkdir -p /etc/nginx/ssl
  if [ ! -f /etc/nginx/ssl/self.crt ]; then
    $SUDO openssl req -x509 -nodes -newkey rsa:2048 -days 3650 \
      -keyout /etc/nginx/ssl/self.key -out /etc/nginx/ssl/self.crt \
      -subj "/CN=$DOMAIN" -addext "subjectAltName=DNS:$DOMAIN" >/dev/null 2>&1
  fi
  SSL_CERT="/etc/nginx/ssl/self.crt"
  SSL_KEY="/etc/nginx/ssl/self.key"
fi
$SUDO sed -e "s|__DOMAIN__|$DOMAIN|g" \
  -e "s|__SSL_CERT__|$SSL_CERT|g" -e "s|__SSL_KEY__|$SSL_KEY|g" \
  "$APP_DIR/deploy/nginx-staging.conf" > "/etc/nginx/sites-available/tauthub"
$SUDO ln -sf /etc/nginx/sites-available/tauthub /etc/nginx/sites-enabled/tauthub
$SUDO nginx -t
$SUDO systemctl reload nginx

log "8/8 Sertifikat TLS (Let's Encrypt) — hanya bila DNS sudah mengarah ke sini"
if [ -d "/etc/letsencrypt/live/$DOMAIN" ]; then
  echo "  Sertifikat sudah ada."
elif [ "$(getent hosts "$DOMAIN" | awk '{print $1; exit}')" = "$(hostname -I | awk '{print $1}')" ]; then
  $SUDO certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos \
    --register-unsafely-without-email || echo "  certbot gagal — ulangi manual: certbot --nginx -d $DOMAIN" >&2
else
  echo "  DNS $DOMAIN belum mengarah ke VPS ini — lewati. Setelah DNS diarahkan"
  echo "  (atau record di-grey-cloud), jalankan: certbot --nginx -d $DOMAIN"
fi
fi # MANAGE_NGINX

log "Setup VPS selesai."
