# Arsitektur Brief Tauthub

Dokumen ini menjelaskan model data brief, alur AI, kontrol keamanan, dan
batasan retensi setelah rangkaian perbaikan (P0–P3). Ditulis untuk siapa pun
yang menyentuh fitur Taut Brief berikutnya.

## 1. Dua konsep brief yang berbeda

| | **Profil Brand (brand brief)** | **Kebutuhan Kampanye (order brief)** |
|---|---|---|
| Tabel | `profiler_profiles` | `taut_profiler_submissions`, `campaigns.strategy_brief`, `campaigns.business_profile` |
| Cakupan | Satu brand, dipakai ulang lintas kampanye | Satu kampanye/order |
| Isi | Entity Core (nama, deskripsi kanonik, fakta resmi, klaim, kata terlarang, pertanyaan AI), identitas, gaya, SEO, instruksi, blok knowledge, dokumen/URL/gambar, timeline | Tujuan, audiens, guardrails, budget, jumlah media, syarat artikel |
| Dibuat oleh | Chat Brief atau form Instan Brief | Wizard kampanye / AI Matchmaker |
| Dipakai AI Writer | Ya (konteks kanonik) | Ya (brief spesifik order) |

`campaigns.profiler_id` menunjuk profil brand yang dipakai; `profiler_snapshot`
menyimpan potongan konteks saat kampanye dibuat (agar perubahan profil tidak
diam-diam mengubah artikel yang sudah berjalan).

**Keputusan:** model ini dipertahankan. Tidak ada tabel baru. Yang diperbaiki
adalah gap validasi dan kejelasan pemisahan di UI (form Instan Brief hanya
mengisi profil brand; budget/jumlah media ada di Matchmaker).

## 2. Skema field & renderer

- `taut_brief_field_versions.fields` menyimpan JSON daftar field:
  `key, layer, section, column?, label, widget, required, sort_order,
  example_question, enabled`.
- Lapis 1 (Entity Core) → kolom `entity_core`; Lapis 2 → `identity` / `style` /
  `seo`; Lapis 3 dihitung sistem (tidak disimpan).
- Nilai field disimpan di JSONB kolom tersebut, statusnya di
  `taut_brief_field_state` (status + sumber, bukan nilai).
- Kosakata widget resmi: `text, longtext, number, url, select, multiselect,
  boolean` (`brief/schema.go: WidgetKinds`, `NormalizeWidget`,
  `ValidateWidgetValue`). Widget tak dikenal diperlakukan sebagai `text`.
- Renderer klien: `web/src/lib/components/brief-field-input.svelte` — memilih
  kontrol dari `widget`, dipakai form Instan Brief (dan siap dipakai panel
  Entitas bila mode edit dibuka).

## 3. Jalur tulis & validasi

| Jalur | LLM? | Validasi |
|---|---|---|
| `POST /brief/fields` | Tidak | `ValidateUpdates`: key ada di skema, field aktif, bukan lapis 3, seksi punya kolom, nilai tidak kosong, tipe sesuai widget |
| `POST /brief/chat` / `/chat/stream` | Ya | Sama (`ValidateUpdates`) + status confirmed tidak boleh turun |
| `PUT /profiler/profiles` | Tidak | Setiap key di `identity/style/seo/entity_core` harus ada di skema aktif DAN di kolom yang sama; tipe nilai diperiksa per widget |
| `POST .../documents` / `/images` | Tidak | Ukuran, MIME/magic byte, kuota, dedup checksum |

## 4. Pipeline dokumen

1. Unggah (sinkron di handler, setelah cek akses brand + profil).
2. Ekstraksi teks deterministik lokal: TXT/CSV/MD apa adanya, DOCX via ZIP+XML,
   PDF via `ledongthuc/pdf` (tanpa OCR — PDF hasil scan gagal dan dilaporkan).
3. `parse_status` = `ready` / `failed`; teks disimpan di
   `profiler_documents.extracted_text` (maks 200.000 karakter).
4. Berkas fisik disimpan sementara (default 24 jam), lalu dibersihkan worker.
5. **Tidak ada AI** di langkah 1–4.

## 5. Titik panggil AI (semua eksplisit)

| Fitur | Berkas | Dipicu |
|---|---|---|
| Chat brief | `brief/complete.go` | Kirim pesan |
| Ringkasan entity/file | `brief/summary/generate.go` | Tombol ringkas/regenerate |
| Analyze profiler kampanye | `clienttaut/analyze.go` | Tombol analisis |
| AI Matchmaker (interpret + pilih) | `brandflow/media_matchmaker.go` | Tombol cari media |
| AI Writer | `brandflow/taut_writer.go` | Tombol produksi |
| Gatekeeper (perbaikan/audit) | `brandflow/taut_gatekeeper.go` | Bagian alur produksi |

Tidak ada unggahan atau penyimpanan field yang memicu panggilan AI.

## 6. Perakitan konteks

- **Chat**: prompt admin + 20 pesan terakhir (+ ringkasan percakapan bila
  dipotong), blok state field, kandidat field berikutnya, memori lintas brief,
  maks 20 dokumen + 20 URL (masing-masing dipotong 1.200 karakter, total 5.000).
- **Ringkasan**: entity = JSON field + instruksi + blok (cap 12k); file =
  timeline + 5 dokumen (4.000 char) + 5 URL (3.000 char) + metadata gambar
  (cap 24k).
- **AI Writer**: `profiler.BuildContextTextForTopic` — deterministik, tanpa
  LLM, retrieval lokal; 4 blok + 4 dokumen (1.500 char) + 4 URL (1.200 char),
  cap 9.000.
- **Matchmaker**: hanya teks brief kampanye + nama/industri/bahasa brand +
  kategori katalog; tidak membaca dokumen privat.

## 7. Kontrol keamanan (setelah P0)

- Akses brand lewat `brand_memberships` (404 lintas tenant), cache 30 detik.
- **IDOR hasil profiler ditutup**: result harus milik campaign di URL dan brand
  terverifikasi.
- **`/uploads/{key}`**: sesi login saja tidak cukup untuk berkas yang tercatat
  di `chat_attachments` / `taut_campaign_files` / `partner_documents` — harus
  pemiliknya (anggota brand, akun mitra, atau admin ber-izin `partners` dengan
  audit log). Jalur publik tetap dua: gambar publikasi terbit dan signed URL.
- Job ringkasan memakai `brand_id` dari baris profil; payload yang tidak cocok
  ditolak.
- Menghapus dokumen mengosongkan `extracted_text`; baris soft-deleted tidak
  bisa diubah lewat ID.
- API key AI disimpan terenkripsi (`SealV3` dengan `CmsEncryptionSecret`),
  respons admin hanya menampilkan `api_key_configured`.

## 8. Retensi & privasi (jujur soal batasan)

| Data | Masa simpan |
|---|---|
| Berkas fisik unggahan | Default 24 jam, lalu dihapus worker |
| Teks ekstraksi dokumen | Selama brief ada; dikosongkan saat dokumen dihapus |
| Ringkasan (`profiler_brief_summaries`) | Selama brief ada |
| Log run (`taut_brief_runs`) | Potongan pesan user (chat) atau sidik sha256 + panjang (ringkasan) |
| Log agent (`taut_agent_runs`, training samples) | Prompt/output penuh — superadmin-only; retensi mengikuti kebijakan operasional |
| Backup database | Di luar kendali aplikasi; penghapusan tidak surut ke backup |

Keterbatasan yang harus disampaikan ke klien: ringkasan/log yang dibuat sebelum
dokumen dihapus bisa masih memuat kutipan; data yang dikirim ke provider AI
eksternal mengikuti kebijakan provider tersebut.

Klien bisa mengunduh seluruh datanya lewat
`GET /v1/brands/{brandId}/profiler/profiles/{profileId}/export` (tombol
"Ekspor data brief (JSON)" di Instan Brief).

## 9. Deviasi dari spesifikasi & backlog

- **Tanpa tes otomatis**: sesuai `AGENTS.md` ("DO NOT AUTO-TEST"). Verifikasi
  memakai `go build`, `svelte-check`, dan uji manual oleh pemilik produk.
  Bagian 13 spesifikasi (test suite) tidak dikerjakan.
- **`profiler_profiles.brand_id`** masih menyimpan nilai mentah URL (bisa UUID
  atau public_id). Normalisasi butuh migrasi data menyeluruh untuk
  `profiler_profiles` + `campaigns` sekaligus — dicatat sebagai backlog; akses
  file sudah menormalkan lewat JOIN saat memeriksa kepemilikan.
- **Signed URL `/uploads`** yang lama tidak kedaluwarsa (dipakai URL publikasi
  yang sudah beredar). URL baru tetap sebaiknya diberi masa berlaku; perubahan
  itu perlu koordinasi dengan publikasi yang sudah terbit.
- **Panel Entitas** masih read-only untuk sebagian field; renderer baru siap
  dipakai bila mode edit penuh dibuka.
