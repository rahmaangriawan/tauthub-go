# Local Retrieval Worker — Tauthub

Menarik halaman media lewat **IP ISP rumahan** untuk domain yang memblokir IP
datacenter (VPS production kena 403 dari Cloudflare WAF — terverifikasi).
Worker HANYA menarik + mengekstrak teks; AI dan penyimpanan tetap di server
production. Tidak membuka port apa pun — semuanya koneksi keluar.

## Setup sekali

1. Di server production, set token yang sama di `.env`:
   ```
   LOCAL_WORKER_TOKEN=<string-acak-panjang>
   ```
   lalu restart `tauthub-api`. Kosong = fitur worker lokal nonaktif.
2. Di komputer lokal, jalankan dengan token yang sama:
   ```bash
   LOCAL_WORKER_TOKEN=<string-acak-panjang> \
   TAUTHUB_API_BASE=https://app.tauthub.com \
   bun run local-worker/worker.ts
   ```

Opsi: `LOCAL_WORKER_INTERVAL_MS` (default 20000) — jeda polling saat antrean
kosong.

## Alur

1. Job crawl VPS gagal total (semua halaman ditolak WAF) → request `pending`
   dibuat otomatis, monitoring menampilkan "Menunggu worker lokal".
2. Worker polling `GET /v1/platform/local-worker/claim` → dapat 1 pekerjaan.
3. Worker menarik halaman lewat IP ISP rumah (https, fallback http), tolak
   host yang mengarah ke IP privat (anti-SSRF), ekstraksi teks.
4. Worker kirim teks ke `POST …/requests/{id}/result` → server memanggil AI
   (prompt sama dengan job biasa) dan menyimpan `media.ai_summary` — termasuk
   menyebar ke media lain dengan domain sama.
5. Gagal di mana pun → status `failed` + event di monitoring; admin tetap
   bisa memakai "Isi manual / AI".

Worker bisa dimatikan kapan saja — pekerjaan menunggu (claim stale >15 menit
otomatis dikembalikan ke antrean).
