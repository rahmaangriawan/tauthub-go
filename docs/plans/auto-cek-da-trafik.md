# Plan: Scrollbar sidebar menempel kanan + cek DA/trafik otomatis untuk domain baru

Status: **rencana, belum dikerjakan.** Dua pekerjaan terpisah; bagian 2 butuh
keputusan user di beberapa titik (ditandai ❓).

---

# Bagian 1 — Scrollbar sidebar dipepetkan ke kanan

## Temuan (hasil pengukuran di browser, 1484×900)

```
sidebar      : left 0   right 264  width 264   (border-right 1px)
sidebar-inner: left 0   right 263  width 263   padding 16px 10px 12px
sidebar-nav  : left 10  right 253  width 243
sidebar-link : left 10  right 253  width 243   (sejajar nav)
```

Scrollbar ada di dalam kotak `nav`, yaitu di `x ≈ 245–253`. Karena
`sidebar-inner` punya `padding: … 10px`, ada **11px ruang kosong** antara
scrollbar dan tepi sidebar. Itu yang membuat scrollbar terlihat "melayang" di
tengah, bukan menempel di kanan.

Catatan: `navScrollbar: 0` — di Chromium headless scrollbar overlay tidak
memakan lebar layout, jadi pergeseran hanya visual. Di Firefox/Windows klasik
scrollbar memakan 8px dan menggeser link; solusi di bawah menangani keduanya.

## Perubahan

`web/src/app.css` — hanya `.app-sidebar-nav`:

```css
.app-sidebar-nav {
  /* Scrollbar menempel ke tepi kanan sidebar: nav ditarik melewati padding
     sidebar-inner, lalu padding-right mengembalikan lebar konten ke posisi
     semula supaya link tidak bergeser. */
  margin-right: -10px;
  padding-right: 10px;
}
.app-sidebar[data-collapsed='true'] .app-sidebar-nav {
  margin-right: -12px;   /* padding sidebar-inner saat collapsed = 12px */
  padding-right: 12px;
}
```

Plus penyempurnaan scrollbar:

- `scrollbar-width: thin;` + `scrollbar-color:` untuk Firefox (sekarang hanya
  `::-webkit-scrollbar` yang ditangani, jadi Firefox memakai scrollbar default
  yang tebal dan terang).
- Kurangi `border` thumb dari `2px` → `1px` supaya thumb terlihat lebih ramping
  saat menempel tepi, dan naikkan `width` track `8px` → `10px` agar area hover
  tetap nyaman.

## Verifikasi (jalankan hanya bila user minta)

1. Buka `/admin/media`, ukur `nav.getBoundingClientRect().right` harus ≈ 263
   (menempel `sidebar-inner`), bukan 253.
2. `link` width tetap 243 (konten tidak bergeser).
3. Ulangi saat sidebar collapsed (lebar 68px) dan di Firefox.
4. Cek sidebar mitra & klien — kelasnya sama, jadi ikut berubah; pastikan
   scrollbar di sana juga wajar.

Risiko: rendah. Satu blok CSS, tidak menyentuh markup.

---

# Bagian 2 — Cek DA & trafik otomatis untuk domain baru

## Temuan penting: sebagian sudah otomatis

Jangan bangun ulang. Yang sudah ada:

| Pemicu | DA + AI Trust | Trafik (SimilarWeb) |
|---|---|---|
| `POST /v1/platform/media` (media baru, status `active`) | ✅ `mpEnqueueSeoRefreshForMedia` | ❌ |
| `POST …/media/{id}/review` decision=approve | ✅ | ❌ |
| Import XLSX mitra (status `pending_review`) | ✅ | ❌ |
| Sweep scheduler tiap 60 detik | ✅ media tanpa skor | ✅ media tanpa snapshot <6 bln, limit 5/siklus |
| **`PUT /v1/platform/media/{id}` saat `website_url` berubah** | ❌ | ❌ |
| **Restore `archived` → `active`** (bulk & per-media) | ❌ | ❌ |

Jadi untuk "media baru" DA sudah otomatis. Yang benar-benar bocor:

1. **Domain diganti pada media yang sudah ada.** Ini kasus paling relevan
   dengan "platform mendeteksi ada domain baru yang ditambahkan". Saat
   `website_url` diedit, DA lama dan `traffic_estimate` lama tetap tersimpan →
   angka menyesatkan (DA milik domain lama). Tidak ada re-enqueue.
2. **Trafik tidak dipicu saat media masuk direktori**, hanya menunggu sweep
   (≤60 detik, tapi limit 5 per siklus → 92 media butuh ~18 menit).
3. **Restore dari arsip** tidak memicu apa pun.

## Temuan blocker: penyedia trafik sedang tidak bisa dipakai

```
jobs kind=media_similarweb_refresh : 184 baris, SEMUA status=failed
last_error (seragam, 184×):
  "Provider Similarweb API Pro mewajibkan DataBee Token. Provider menolak
   tanpa itu — lengkapi di kartu SimilarWeb, panel Konfigurasi."

platform_seo_config: sealed_similarweb_api_key = ada, sealed_databee_token = KOSONG
media aktif: 92 | punya traffic_estimate > 0: 31 | snapshot valid (visits > 0): 0
```

Artinya: otomatisasi trafik **sudah berjalan** (2 hari × 92 media), tapi
selalu gagal di penyedia karena token belum diisi. Selama ini belum beres,
menambah pemicu otomatis hanya akan menambah baris `failed` dan menghabiskan
kuota RapidAPI.

**Langkah 0 (wajib, di luar kode):** ~~isi DataBee Token~~ — **sudah tidak
relevan**. Provider trafik diganti ke "Similar Web API" (belchiorarkad) yang
hanya butuh satu key RapidAPI (lihat `docs/plans/similarweb-belchiorarkad.md`),
jadi penghambat token gugur dengan sendirinya.

## Desain usulan

### A. Satu pintu: `mpEnqueueMediaChecks(mediaID, reason)`

Helper baru di `platform/media_support.go` yang memanggil dua enqueue sekaligus
dengan guard yang sama:

```
mpEnqueueMediaChecks(ctx, mediaID, reason)
  ├─ guard: media ada, status = 'active', website_url tidak kosong
  ├─ guard: tidak ada job pending/running untuk kind tsb (sudah ada)
  ├─ guard BARU: kredensial penyedia tersedia
  │    ├─ DA   : platform_seo_config punya sealed_similarweb_api_key
  │    │        ATAU provider DA alternatif aktif (guestpostlinks)
  │    └─ trafik: punya key RapidAPI SimilarWeb (cukup satu kredensial)
  └─ enqueue media_seo_refresh + media_similarweb_refresh
```

Guard kredensial adalah perbaikan terpenting: mencegah 92 job gagal tercipta
setiap kali sweep berjalan. Kalau kredensial belum lengkap, **jangan enqueue**;
catat sekali ke log dan biarkan UI memberi tahu admin.

### B. Titik panggil yang ditambah

| Lokasi | Alasan |
|---|---|
| `MediaCreate` | tambah trafik (DA sudah) |
| `MediaApprove` (decision=approve) | tambah trafik (DA sudah) |
| `MediaUpdate` — `website_url` berubah | **domain baru** → DA + trafik |
| `MediaUpdate` — status jadi `active` | restore manual → DA + trafik |
| `MediaRestoreArchived` (bulk) | DA + trafik per media |
| Import XLSX mitra | tetap DA saja (masih `pending_review`, sesuai aturan user: bukan tab pending) |

### C. Domain berubah: data lama diapakan? ❓

Tiga opsi:

1. **Simpan riwayat, tampilkan yang terbaru** (rekomendasi). `media_seo_metrics`
   dan `media_similarweb_snapshots` sudah punya `created_at`; cukup enqueue
   ulang dan biarkan baris baru masuk. UI menampilkan "sedang diperbarui"
   selama job berjalan. `traffic_estimate` di-reset ke 0 saat domain berubah
   supaya tidak menampilkan angka domain lama.
2. Reset keras: hapus baris metrik/snapshot lama untuk media itu.
3. Biarkan angka lama sampai hasil baru masuk (paling murah, tapi menyesatkan
   untuk sementara).

### D. Kebijakan retry ❓

Sekarang: media yang job-nya `failed` didiamkan 24 jam (hardcode di query
sweep & bulk). Usul:

- Error **konfigurasi** (`NOT_CONFIGURED`, token kosong) → tidak masuk antrean
  sama sekali (guard A). Tidak perlu retry.
- Error **rate limit / 5xx penyedia** → backoff eksponensial: 15 mnt → 1 jam →
  6 jam → 24 jam, maksimal 4 percobaan. Jangan hardcode 24 jam untuk semua.
- Error **domain mati / NXDOMAIN** → tandai `unreachable`, jangan diulang
  otomatis; tampilkan badge di tabel supaya admin tahu.

### E. Tampilan (opsional, tapi disarankan)

Tab Direktori sudah punya indikator antrean. Tambahkan satu chip ringkas:

> `Cek otomatis · DA 3 antre · Trafik 5 antre` — atau, bila kredensial kurang:
> `Cek otomatis berhenti — Key RapidAPI SimilarWeb belum diisi` + tautan ke
> Konfigurasi.

Tanpa ini, "otomatis" tidak terlihat: admin tidak tahu apakah sistem bekerja
atau diam.

### F. Yang TIDAK diubah

- Frekuensi sweep (60 detik, limit 5) — biarkan, karena provider berbayar.
- Aturan "media yang sudah berskor tidak dicek ulang otomatis" untuk DA.
  ❓ Kalau user ingin DA di-refresh berkala (mis. 3 bulan), itu perubahan
  kebijakan terpisah — perlu keputusan.

## Urutan pengerjaan

1. ~~**[non-kode]** Isi DataBee Token di panel Konfigurasi.~~ Diganti: provider
   trafik sekarang "Similar Web API" (belchiorarkad), cukup satu key RapidAPI.
2. `mpEnqueueMediaChecks` + guard kredensial (backend).
3. Sambungkan ke 5 titik panggil di tabel B.
4. Kebijakan retry bertingkat (D).
5. Reset `traffic_estimate` saat domain berubah (C opsi 1).
6. Chip status di tab Direktori (E).
7. Bersihkan 184 job `failed` lama supaya indikator tidak merah selamanya:
   `UPDATE jobs SET status='cancelled' ...` — atau biarkan sebagai riwayat. ❓

## Risiko

| Risiko | Mitigasi |
|---|---|
| Kuota RapidAPI jebol | limit per siklus + dedupe + guard kredensial |
| Banjir job saat domain massal diubah | enqueue tetap lewat helper dengan dedupe; sweep tetap limit 5 |
| Angka lama tampil menyesatkan | reset `traffic_estimate` + label "sedang diperbarui" |
| 184 job gagal menutupi kegagalan baru | retensi/arsip job `failed` > 7 hari |

## Pertanyaan untuk user

1. Domain berubah → data DA/trafik lama: simpan riwayat (rekomendasi), hapus,
   atau biarkan?
2. DA perlu di-refresh berkala (mis. tiap 3 bulan) atau tetap "sekali saja
   sampai ada skor"?
3. 184 job trafik `failed` lama: dibersihkan atau disimpan sebagai riwayat?
4. Chip status "Cek otomatis" di tab Direktori: perlu atau cukup diam?
