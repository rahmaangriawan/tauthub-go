# Tauthub MCP admin — setup Hermes & Claude Code

Ini petunjuk operasional. Arsitektur lengkap ada di
`plans/ini-aku-baru-ngide-gentle-bunny.md` di repo `tauthub-ke-go`.

## 1. Env

Di `.env` hanya **topologi**, tidak ada secret:

```
# cmd/mcp (opsional — default sudah masuk akal)
MCP_PORT=8090
MCP_PUBLIC_URL=https://mcp.tauthub.com     # kosong = link di pesan proaktif pakai host yang ada
API_INTERNAL_URL=http://127.0.0.1:8080     # dicari dari API_PORT bila kosong
```

Service key, bot token, chat allowlist, kill switch tulis, dan batas
penyesuaian saldo **disimpan terenkripsi di tabel `mcp_config`** dan
dikelola via **`/admin/konfigurasi/mcp`** (super admin). Alasannya: `.env`
biasanya terbaca dari `journalctl`, backup directory, dan siapa pun yang
pernah pegang SSH — service key di sana sama saja seperti ditempel di pintu
server. Nilai di DB tersegel dengan `CMS_ENCRYPTION_SECRET`.

## 2. Bot Telegram

1. Chat **@BotFather**, `/newbot`, pilih nama (mis. "Tauthub Approval").
2. Salin token → tempel di `/admin/konfigurasi/mcp` panel Bot Telegram
   (pilih "Ganti", paste, Simpan).
3. **Penting:** set `/setprivacy` → **Disable** supaya bot menerima pesan.
4. Tambahkan bot ke grup admin (atau chat pribadi admin). Kirim `/start`.
5. Dapatkan chat_id dan masukkan di panel "Chat allowlist":
   - chat pribadi: chat ke **@userinfobot**, catat ID.
   - grup: tambahkan **@getidsbot**, kirim pesan di grup, ia akan balas.

Rotasi bot token butuh `sudo systemctl restart tauthub-mcp` supaya loop
polling bot mengambil token baru. Rotasi service key, perubahan chat
allowlist, toggle kill switch, dan batas kredit langsung berlaku tanpa
restart (dibroadcast via pg_notify).

## 3. Admin ↔ Telegram ID

Setiap admin yang akan menekan tombol **Setujui/Tolak** harus dipetakan ke
akun Telegram-nya.

1. Buka `/admin/admins` (Super Admin).
2. Klik "Ubah" di kolom **Telegram** untuk tiap admin, isi numeric user ID
   dari **@userinfobot**. Kolom ini unique; sistem menolak dua admin
   memegang ID yang sama.
3. Admin tanpa Telegram ID tetap bisa melihat kartu (bila tergabung di
   `TELEGRAM_APPROVAL_CHAT_IDS`), tapi tombolnya ditolak dengan pesan
   "belum terdaftar sebagai admin".

## 4. Agent client (Hermes / Claude Code)

Buat satu baris `agent_clients` per instance. Belum ada UI; jalankan SQL
langsung — token hanya terlihat sekali di sini.

```sql
-- Hermes VPS (contoh). Simpan tokennya di password manager.
INSERT INTO agent_clients (name, token_hash, read_as_user_id)
VALUES (
  'hermes-vps',
  encode(digest('<TOKEN_ACAK_32B>', 'sha256'), 'hex'),
  '<USER_UUID_ADMIN_YANG_DIPAKAI_UNTUK_BACA>'
);
```

Token acak: `openssl rand -hex 32`. `read_as_user_id` adalah user admin yang
dipakai server MCP saat tool **baca** dijalankan (`list_media`,
`attention_queue`, dll.). Pilih super admin untuk akses penuh, atau admin
dengan izin tertentu untuk membatasi apa yang terlihat.

Ulang untuk `hermes-macmini` dan `claude-code-local`.

## 5. Hermes Agent

Service key untuk `cmd/mcp ↔ cmd/api` berbeda dari **token klien** yang
Hermes bawa. Hermes mengirim `Authorization: Bearer <TOKEN_KLIEN>` — token
klien yang dicocokkan ke baris `agent_clients` (langkah 4). cmd/mcp sendiri
yang menerjemahkannya menjadi service key saat memanggil `/v1/platform/*`.

Pada config Hermes (`~/.hermes/config.yaml` atau sesuai distribusinya):

```yaml
mcp_servers:
  tauthub:
    url: https://mcp.tauthub.com/mcp     # VPS (behind nginx TLS)
    # atau: http://127.0.0.1:8090/mcp     # Mac mini lokal
    headers:
      Authorization: Bearer <TOKEN_HERMES>
telegram:
  bot_token: <TOKEN_HERMES_BOT>          # bot untuk chat, berbeda dari bot Persetujuan
  allowed_chats: [<CHAT_ID_ADMIN>]
system_prompt: |
  Anda adalah asisten admin Tauthub. Semua tool tulis akan meminta
  konfirmasi di bot Persetujuan terpisah; jangan mencoba menyetujui
  sendiri atau meminta admin mengirim perintah lain di luar flow itu.
  Jawab dalam bahasa Indonesia yang ringkas.
```

Sintaks persis mengikuti versi Hermes yang terpasang; verifikasi dengan
`hermes --help` atau README pada rilis tersebut. Yang penting: dua bot
Telegram **berbeda** — satu untuk percakapan natural (Hermes), satu untuk
tombol Setujui/Tolak (`TELEGRAM_APPROVAL_BOT_TOKEN`).

## 6. Claude Code (Mac mini)

Tambahkan di `~/.claude/mcp.json` atau project `.mcp.json`:

```json
{
  "mcpServers": {
    "tauthub-admin": {
      "transport": "http",
      "url": "https://mcp.tauthub.com/mcp",
      "headers": { "Authorization": "Bearer <TOKEN_CLAUDE_CODE>" }
    }
  }
}
```

## 7. Nginx (VPS)

```nginx
location /mcp {
    proxy_pass http://127.0.0.1:8090/mcp;
    proxy_set_header Host $host;
    proxy_read_timeout 60s;
    # Jangan teruskan Authorization ke backend lain — biarkan
    # authorization sampai ke tauthub-mcp apa adanya.
}
```

Pastikan sertifikat Let's Encrypt sudah dipasang pada subdomain
`mcp.tauthub.com` (atau path mana pun yang dipilih).

## 8. Jalankan

```bash
# VPS
sudo systemctl daemon-reload
sudo systemctl enable --now tauthub-mcp
journalctl -u tauthub-mcp -f
```

Smoke test:

```bash
curl -s -H "Authorization: Bearer <TOKEN_HERMES>" \
  -H "Content-Type: application/json" \
  https://mcp.tauthub.com/mcp \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq .result.tools[].name
```

Harus mengeluarkan daftar tool (`operations_summary`, `attention_queue`,
`finance_report`, …, `review_media`, `adjust_credit`, dll.).

## 9. Uji flow aksi

1. Dari Claude Code / Hermes: panggil `review_media` untuk media test.
2. Kartu muncul di `TELEGRAM_APPROVAL_CHAT_IDS` dengan tombol Setujui/Tolak.
3. Admin dengan Telegram ID tepat menekan Setujui.
4. Media berubah status; audit_logs mencatat `via=agent`,
   `agent_action_code=<kode>`; baris di `/admin/agent-actions` jadi
   `executed`.
5. Set `MCP_WRITES_ENABLED=false` → tool tulis menjawab error, baca jalan
   terus.
