# Cek DA media mingguan

Alur untuk memperbarui Domain Authority (DA) media TautHub memakai
<https://tools.guestpostlinks.net/bulk-da-pa-checker-tool/> (gratis, sumber Moz).

## Kenapa lapisan browser tetap ada

Situs itu memanggil `admin-ajax.php` (action `dapa_checker_function`) dan
melindungi dirinya dengan **Cloudflare Turnstile + FingerprintJS**. Permintaan
HTTP biasa dari server akan mentok captcha, jadi pengambilan nilainya dilakukan
lewat browser asli (sesi ZCode), bukan `curl`. Skrip di folder ini sengaja tidak
menyentuh jaringan sama sekali: hanya membaca DB, mengelola CSV, dan menulis DA.

## Berkas

| Berkas | Isi |
| --- | --- |
| `media-da-report.csv` | **Satu** laporan kumulatif. Kolom: `nama_media, domain, da, checked_at, source`. Domain lama diperbarui, domain baru ditambahkan, file tidak pernah dibuat ulang. |
| `media_da.py` | Perintah `prepare`, `merge`, `apply`. |

## Alur tiap minggu

```sh
# 1. domain mana yang perlu dicek (DA kosong atau lebih tua dari 7 hari)
python3 scripts/media-da/media_da.py prepare --out /tmp/domains.txt

# 2. tempel isi /tmp/domains.txt ke textarea di situs checker, klik Check Authority,
#    lalu salin tabel hasilnya ke /tmp/da_result.json (bentuk [{domain, da}] atau TSV domain<TAB>da)

# 3. gabungkan ke CSV kumulatif, lalu tulis ke database
python3 scripts/media-da/media_da.py merge --from /tmp/da_result.json
python3 scripts/media-da/media_da.py apply
```

Semua perintah idempoten: dijalankan dua kali tidak mengubah apa pun.

## Bila media sudah banyak (ratusan sampai ribuan)

Situs checker punya dua batas: **100 URL per submit** dan **1 kredit per URL**.
Akun gratis mendapat kuota kredit harian (lihat `reset_msg` di halaman, mis.
"reset in 9h"), jadi 1000 media tidak bisa diselesaikan sekali jalan.

Antre dengan memecah pekerjaan. `prepare` mendukung `--chunk-size` dan `--offset`,
dan `merge`/`apply` boleh dipanggil berkali-kali karena idempoten:

```sh
# malam ini: bagian ke-0
python3 scripts/media-da/media_da.py prepare --chunk-size 100 --offset 0 --out /tmp/domains.txt
#   ... submit ke situs, simpan hasil ...
python3 scripts/media-da/media_da.py merge --from /tmp/da_result.json
python3 scripts/media-da/media_da.py apply

# malam berikutnya: bagian ke-1, dan seterusnya
python3 scripts/media-da/media_da.py prepare --chunk-size 100 --offset 1 --out /tmp/domains.txt
```

Karena `prepare` hanya mengambil domain yang DA-nya kosong atau sudah basi,
bagian yang sudah selesai otomatis keluar dari daftar. Jadi cukup ulangi bagian
yang sama tiap kali sampai `prepare` melaporkan `0 domain perlu dicek`, tanpa
harus melacak offset secara manual.

- Cek berapa yang tersisa: `python3 scripts/media-da/media_da.py prepare` (angka di stderr).
- Paksa cek ulang semua (mis. setelah yakin DA bergeser): `DA_FRESH_DAYS=0 python3 scripts/media-da/media_da.py prepare`.

## Catatan

- Pencocokan ke media dilakukan lewat host `website_url` (skema/path/port/`www`
  dibuang). Domain yang tidak punya media, atau yang cocok ke lebih dari satu
  media, dilewati, tidak ditebak.
- `apply` hanya menulis `da`, `checked_at`, dan `source`. `pa`, `spam_score`, dan
  sinyal AI tidak disentuh.
- Kredensial DB dibaca dari `api/.env`; tidak ada rahasia di CSV ini.