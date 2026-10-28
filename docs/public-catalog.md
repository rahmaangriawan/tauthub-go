# Katalog Media Publik — data.tauthub.com

Direktori media yang bisa diakses siapa pun dan diindeks mesin pencari. Sumber
datanya sama dengan katalog klien di dashboard, tetapi disaring dan dipetakan
ulang supaya tidak membocorkan sisi suplai.

## Arsitektur

Satu aplikasi SvelteKit melayani dua host:

| Host | Isi |
|---|---|
| `app.tauthub.com` | dashboard (perilaku lama) |
| `data.tauthub.com` | katalog publik |
| host lain (localhost, staging) | keduanya, tanpa redirect lintas host |

`web/src/hooks.server.ts` memakai `web/src/lib/server/data-site.ts`:

- Host publik yang membuka path dashboard (`/login`, `/admin`, `/klien`, `/mitra`,
  `/register*`, `/payments*`, `/berita-acara*`, `/docs`, …) → 302 ke host aplikasi.
- Host aplikasi yang membuka `/media*` atau `/kategori*` → 308 ke host publik,
  supaya konten tidak punya dua URL.
- `robots.txt` dan `sitemap.xml` **tidak** dialihkan: keduanya sadar-host dan
  melayani aturan berbeda di tiap host.

Halaman dirender server-side (`+page.server.ts` memanggil API internal lewat
`API_URL`), jadi HTML yang diterima crawler sudah berisi daftar media, harga, dan
metrik — bukan hasil fetch di browser.

## Halaman

| URL | Keterangan |
|---|---|
| `/` | landing + katalog: pencarian, filter (kategori, tier, DA minimal, harga maks, urutan), pagination |
| `/media/{public_slug}` | detail media: metrik, ketentuan publikasi, kebijakan tautan, CTA |
| `/kategori/{slug}` | listing per kategori (landing SEO) |
| `/robots.txt` | aturan crawler per host |
| `/sitemap.xml` | sitemap index |
| `/sitemap-pages.xml` | beranda + semua kategori |
| `/sitemap-media-{n}.xml` | potongan daftar media, 5.000 URL per berkas |

Filter dikirim lewat query string (`?q=&kategori=&tier=&da=&harga=&urut=&page=`).
Halaman dengan filter aktif diberi `noindex,follow` dan canonical ke URL dasar;
pagination (`?page=n`) tetap boleh diindeks dengan canonical ke halaman itu
sendiri.

## API publik

Tanpa autentikasi, hanya baca, `Access-Control-Allow-Origin: *`, dan
`Cache-Control: public`.

| Endpoint | Parameter |
|---|---|
| `GET /v1/public/media` | `q`, `kategori`, `tier`, `da_awal`, `harga_maks`, `urut` (`nama`/`da`/`trafik`/`harga_asc`/`harga_desc`/`baru`), `page`, `limit` (maks 60) |
| `GET /v1/public/media/{public_slug}` | — |
| `GET /v1/public/kategori` | — |
| `GET /v1/public/statistik` | — |
| `GET /v1/public/sitemap` | daftar `{slug, updated_at}` untuk sitemap |

Contoh:

```bash
curl -s 'https://data.tauthub.com/v1/public/media?kategori=teknologi&da_awal=50&urut=da&limit=5'
```

Batas: 240 permintaan/menit per IP (limiter in-memory; tidak menulis ke DB).
Media yang tampil **hanya** `status='active' AND public_listed` dan sudah punya
`public_slug`.

## Field yang dibuka vs ditutup

Dibuka: nama, slug, domain/website, deskripsi, kategori, tier, DA/PA/spam/AI
trust, estimasi trafik (Similarweb bila ada), harga klien, tipe harga
(nett/nego), kebijakan tautan (`max_links`, dofollow, durasi), mode jumlah kata,
wajib gambar, embed, label sponsor, verifikasi Dewan Pers/domain, jumlah artikel
terbit, rating klien, contoh artikel, `updated_at`, dan ringkasan situs
(`ai_summary`, hanya di detail).

**Tidak pernah dibuka**: UUID internal, identitas mitra (nama/ID/nomor WA),
harga mitra dan markup, `notes`, topik terlarang, `health_breakdown`, endpoint
integrasi/kredensial, status harga internal, dan `pending_changes`.

Kontrak ini dijaga di `api/internal/domains/publiccatalog/` — daftar kolom ada di
`publicSelect`, jadi menambah field publik harus lewat berkas itu.

## Kontrol admin

Dua kolom di `media` (migrasi `258_media_public_catalog.sql`):

- `public_listed boolean NOT NULL DEFAULT true` — matikan untuk menyembunyikan
  media dari situs publik **tanpa** mengubah statusnya di katalog klien.
  Hanya admin (modal edit Media → "Tampilkan di katalog publik").
- `public_slug varchar(160) UNIQUE` — slug URL publik, dipisah dari `slug`
  internal. `slug` ikut berubah saat mitra mengganti nama media; `public_slug`
  tidak, supaya URL yang sudah terindeks tidak putus. Mengubahnya manual di
  modal admin akan memutus tautan lama (peringatan tampil di form).

Media baru otomatis mendapat `public_slug` dari nama/slug saat dibuat.

## SEO

- `robots.txt` host publik: `Allow: /` + `Sitemap: https://data.tauthub.com/sitemap.xml`.
- `sitemap.xml` dibuat dari API (`/v1/public/sitemap` + `/v1/public/kategori`),
  di-cache 10 menit di memori proses web.
- Metadata per halaman: title, meta description, canonical absolut ke host
  publik, Open Graph/Twitter, dan JSON-LD (`WebSite` + `SearchAction` +
  `ItemList` di beranda; `Product` + `Offer` + `BreadcrumbList` di detail;
  `CollectionPage` + `ItemList` di kategori).
- Tautan ke situs media memakai `rel="nofollow noopener noreferrer"`.
- Halaman dashboard tetap `noindex` secara tidak langsung (robots host aplikasi
  men-disallow `/admin`, `/klien`, `/mitra`, `/berita-acara/`).

## Variabel lingkungan

| Variabel | Default | Fungsi |
|---|---|---|
| `PUBLIC_CATALOG_ENABLED` | `true` | kill switch. `false` → API `/v1/public/*` 503, robots `Disallow: /`, halaman kosong |
| `PUBLIC_CATALOG_HOST` | `data.tauthub.com` | host katalog; subdomain `data.*` lain juga dikenali |
| `PUBLIC_CATALOG_ORIGIN` | `https://data.tauthub.com` | origin untuk canonical/sitemap |
| `APP_HOST` | `app.tauthub.com` | host dashboard |
| `APP_ORIGIN` | `https://app.tauthub.com` | tujuan redirect dari host publik |
| `PUBLIC_APP_ORIGIN` | `https://app.tauthub.com` | dipakai tombol "Masuk / Daftar" (dibaca browser) |

`PUBLIC_*` di atas dibaca SvelteKit (`$env/dynamic/*`), jadi harus ada di `.env`
proses web. API hanya butuh `PUBLIC_CATALOG_ENABLED`.

### Unit systemd web: jangan set `ORIGIN`

`deploy/tauthub-web.service` sengaja **tidak** memakai `Environment=ORIGIN=...`.
adapter-node memakai `ORIGIN` sebagai basis URL untuk semua request dan
mengabaikan header host; akibatnya SvelteKit melihat satu host saja dan
`data.tauthub.com` ikut diperlakukan sebagai host aplikasi (307 ke `/login`,
robots versi dashboard). Origin per request dihitung dari
`PROTOCOL_HEADER`/`HOST_HEADER` yang di-set nginx, jadi dua header itu yang
wajib ada. `deploy/setup-vps.sh` menulis ulang unit dari template ini pada
setiap deploy — jangan menambahkan `ORIGIN` kembali.

### Cache Cloudflare untuk robots.txt

Cloudflare meng-cache berkas `.txt` berdasarkan ekstensi (Edge TTL 4 jam) dan
mengabaikan `max-age` origin. Route `/robots.txt` mengirim `no-store` supaya
tidak ter-cache; kalau aturan robots berubah dan versi lama masih tersaji,
purge URL `https://data.tauthub.com/robots.txt` di dashboard Cloudflare.

## Setup subdomain (langkah manual, di luar repo)

1. **DNS**: buat record `A data` → IP VPS (atau CNAME ke host yang sama),
   proxy Cloudflare boleh aktif.
2. **nginx**: produksi dikelola CloudPanel. Daftarkan `data.tauthub.com` sebagai
   site yang menunjuk aplikasi yang sama, atau pasang
   `deploy/nginx-data.conf` (ganti `__SSL_CERT__`/`__SSL_KEY__`) lalu
   `nginx -t && systemctl reload nginx`.
3. **TLS**: `certbot --nginx -d data.tauthub.com` (lewati langkah ini bila
   Cloudflare menangani TLS di edge).
4. **Env web**: pastikan `PUBLIC_CATALOG_ORIGIN=https://data.tauthub.com` dan
   `PUBLIC_APP_ORIGIN=https://app.tauthub.com`, lalu restart service web.
5. **Migrasi**: `./bin/migrate up` (deploy normal sudah menjalankannya).
6. **Verifikasi**:
   ```bash
   curl -sI https://data.tauthub.com/ | head -5
   curl -s https://data.tauthub.com/robots.txt
   curl -s https://data.tauthub.com/sitemap.xml
   curl -s https://data.tauthub.com/media/<slug> | grep -o '<title>[^<]*'
   ```
7. **Google Search Console**: tambahkan properti `data.tauthub.com`, submit
   `https://data.tauthub.com/sitemap.xml`, lalu minta indexing untuk beberapa
   halaman kategori.

## Batasan yang diketahui

- Limiter API publik berbasis memori proses: dengan lebih dari satu instance API,
  batas efektifnya berlipat.
- `public_slug` bisa diubah admin; tidak ada riwayat slug/redirect otomatis
  untuk URL lama.
- Media hanya bisa diindeks bila punya `public_slug`; baris lama sudah dibackfill
  oleh migrasi 258.
- Halaman detail memuat `ai_summary` bila ada; bila belum pernah dibuat, konten
  halaman lebih tipis (deskripsi + metrik).
