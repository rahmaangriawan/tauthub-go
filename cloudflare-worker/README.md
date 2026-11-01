# Worker Tes Crawl — Cloudflare (gratis)

Membuktikan apakah request keluar dari jaringan Cloudflare lolos dari WAF
situs mitra yang memblokir IP datacenter (HTTP 403 dari VPS).

## Cara deploy — A. Dashboard (paling cepat, tanpa install apa pun)

1. Buka https://dash.cloudflare.com → **Workers & Pages** → **Create** → pilih **"Hello World"** → **Deploy**.
2. Buka worker yang baru dibuat → **Edit code** → hapus semua isinya, tempel seluruh isi `crawl-test.js` → **Deploy**.
3. Buka URL worker-nya (bentuknya `https://<nama>.<subdomain>.workers.dev/`) dengan parameter:

```
https://<nama>.<subdomain>.workers.dev/?domains=bekasikinian.com,kpopsquad.com,patinews.com,edisi.co.id
```

Atau tes satu URL bebas:

```
https://<nama>.<subdomain>.workers.dev/?url=https://bekasikinian.com/sitemap.xml
```

## Cara deploy — B. Wrangler (CLI)

```bash
cd cloudflare-worker
npx wrangler login   # sekali saja
npx wrangler deploy  # pakai wrangler.toml di folder ini
```

## Cara membaca hasil (JSON)

- `egress` — dari negara/colo/ASN mana request keluar. Kalau `country` bukan
  `ID`, catatan: saat membuka URL dari browser, Worker jalan di colo
  terdekat pengunjung. Nanti kalau jalan sebagai Cron Trigger, colonya bisa
  berbeda — verifikasi ulang lewat cron setelah tes browser lolos.
- `results[].status` — **200 = lolos** (Cloudflare egress tidak diblokir),
  **403 = diblokir** (sama seperti VPS), `error` = gagal koneksi/DNS.
- `results[].cf_mitigated` — `challenge` berarti kena JS challenge
  (Playwright/retry bisa berpeluang), kosong + 403 berarti hard block.

## Keputusan

- **Semua 200** → lanjut: worker ini dikembangkan jadi pengambil pekerjaan
  (claim/result) dari API Tauthub dengan Cron Triggers — crawler fallback
  gratis tanpa listrik dan tanpa proxy.
- **Masih 403** → Workers bukan jalannya; opsi tersisa: free tier scraping
  service, whitelist IP oleh mitra, atau jalur "Isi manual / AI".

Hapus worker ini kapan saja setelah tes — tidak menyimpan data apa pun.
