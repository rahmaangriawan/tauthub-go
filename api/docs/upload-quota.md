# Upload Quota & Siklus Hidup Berkas Sementara — Chatbot Taut Brief

Implementasi di `internal/domains/profiler/` (migrasi `247_chat_attachments_quota.sql`).
Berkas fisik di `UPLOAD_DIR` bersifat **sementara**; teks hasil ekstraksi dokumen
tersimpan di `profiler_documents.extracted_text` dan tetap dibaca AI setelah berkas
fisiknya dibersihkan.

## Konfigurasi (env, semua opsional)

| Env | Default | Arti |
|---|---|---|
| `UPLOAD_MAX_FILE_MB` | 10 | Ukuran maksimum satu berkas |
| `UPLOAD_MAX_PER_CONVERSATION_FILES` | 5 | Berkas aktif per brief (jalur chat) |
| `UPLOAD_MAX_PER_CONVERSATION_MB` | 25 | Total ukuran aktif per brief (jalur chat) |
| `UPLOAD_MAX_DAILY_FILES` | 30 | Berkas per user per hari (semua jalur) |
| `UPLOAD_MAX_DAILY_MB` | 100 | Total ukuran per user per hari |
| `UPLOAD_MAX_ACTIVE_TEMP_MB` | 100 | Total berkas aktif per user |
| `UPLOAD_RETENTION_HOURS` | 24 | Umur berkas **dokumen** setelah diproses |
| `UPLOAD_IMAGE_RETENTION_HOURS` | 720 | Umur berkas **gambar** setelah diproses |
| `UPLOAD_MAX_UNPROCESSED_HOURS` | 6 | Umur maksimum berkas tertinggal `uploaded`/`processing` |
| `UPLOAD_CLEANUP_INTERVAL_MINUTES` | 15 | Interval worker cleanup |
| `UPLOAD_ORPHAN_GRACE_MINUTES` | 60 | Grace period berkas tanpa metadata |
| `UPLOAD_ORPHAN_CLEANUP` | false | `true` = izinkan hapus fisik berkas yatim |

## Cara kerja

**Unggah** (`DocumentUpload`/`ImageUpload`):
1. `http.MaxBytesReader` membatasi body (berkas + overhead multipart).
2. Ukuran **aktual**, allowlist MIME, dan **magic number** (`%PDF`, `PK` zip,
   `http.DetectContentType` untuk gambar) divalidasi — ekstensi menyamar ditolak.
3. Dedup checksum (file identik tidak memakan kuota).
4. **Reservasi kuota** dalam satu transaksi dengan `pg_advisory_xact_lock` per
   user: hitung pemakaian lalu insert baris ledger `chat_attachments` berstatus
   `processing`. Unggahan paralel user yang sama tidak bisa lolos bersamaan.
5. Simpan berkas (nama internal `uuid.ext`, tidak pernah nama user) → ekstrak
   teks → insert baris domain → finalisasi ledger (`processed`/`failed` + `expires_at`).

Pelanggaran kuota: **413** (basis ukuran) atau **429** (basis jumlah). Jalur
unggah dibedakan field `source`: `chat` (composer Copilot — memakan kuota
percakapan) vs `library` (tab Media — hanya kuota harian/aktif; pustaka media
memang bisa menampung banyak aset per brief).

**Cleanup worker** (goroutine di `cmd/scheduler`, berhenti rapi saat SIGTERM):
1. Berkas tertinggal `uploaded`/`processing` > `UPLOAD_MAX_UNPROCESSED_HOURS`
   → dipaksa `failed` (proses pemiliknya mati di tengah request).
2. `processed`/`failed` yang lewat `expires_at`, **dan** lampiran milik profil
   (brief) yang sudah dihapus → `delete_pending`.
3. Penghapusan fisik per batch (`FOR UPDATE SKIP LOCKED`, aman multi-proses):
   `os.Remove` dulu, baru metadata `deleted`. Gagal menghapus → dibiarkan
   `delete_pending`, dicoba lagi siklus berikutnya. Berkas `processing` tidak
   pernah dihapus di langkah ini.
4. Konsistensi UI/AI saat berkas dokumen dibersihkan:
   `profiler_documents.file_deleted_at` ditandai (baris + teks ekstraksi
   **dipertahankan** sebagai memori AI; tab Media menampilkan badge
   "Berkas dibersihkan"). Gambar → baris `profiler_images` di-soft-delete
   (berkasnya memang aset medianya).
5. Sapuan yatim: berkas di `UPLOAD_DIR` tanpa referensi metadata mana pun
   (ledger, profiler, kampanye, featured image, HTML content_versions) setelah
   grace period. Default **hanya melapor**; hapus fisik butuh
   `UPLOAD_ORPHAN_CLEANUP=true` dan hanya untuk nama berkas pola internal
   `uuid.ext`.
6. Log metrik per siklus (`[upload-cleanup]`): jumlah dipaksa-gagal,
   ditandai kadaluarsa, terhapus, gagal hapus, yatim, total aktif + MB,
   dan pemakaian disk partisi `UPLOAD_DIR`.

**Sweep 30-hari lama** (`schPurgeStaleUploads`) kini juga memasukkan
`chat_attachments`/`profiler_documents`/`profiler_images` ke himpunan
"terpakai" — berkas profiler tidak lagi berisiko dianggap yatim oleh sweep itu.

## Keputusan desain

- **"Conversation" = `profiler_profiles`** (satu brief = satu percakapan chat).
- **Retensi gambar default 30 hari, bukan 24 jam.** Gambar adalah aset pustaka
  media/GEO; 24 jam akan mengosongkan pustaka pengguna. Dokumen 24 jam sesuai
  spesifikasi karena AI membaca teksnya dari DB. Keduanya bisa diubah via env.
- **Bukan "attachment permanen"**: tidak ada fitur penyimpanan permanen di
  codebase ini, sehingga seluruh berkas fisik diperlakukan sementara; teks
  ekstraksi dokumen adalah satu-satunya bentuk persistensi yang dipertahankan.
- **Daily quota menghitung semua status** (termasuk `deleted`) untuk mencegah
  pola hapus-unggah-ulang demi mem-bypass batas harian.
- Tanpa Redis/S3: ledger PostgreSQL + advisory lock + worker Go murni.

## Deployment

1. `migrate up` (migrasi 247 membuat tabel + backfill ledger dari data lama;
   backfill diberi retensi penuh dihitung DARI WAKTU MIGRASI supaya file lama
   tidak langsung tersapu pada siklus pertama).
2. Deploy binary baru; **scheduler harus berjalan** (`tauthub-scheduler` /
   `tauthub-staging-scheduler`) — di situlah worker cleanup hidup.
3. Opsional: set env di atas (mis. perpanjang `UPLOAD_IMAGE_RETENTION_HOURS`).
4. Aktifkan `UPLOAD_ORPHAN_CLEANUP=true` hanya bila ingin pembersihan yatim
   otomatis; pantau dulu angka `yatim_ditemukan` di log scheduler.
5. Reverse proxy sudah membatasi ukuran request; `MaxBytesReader` menjadi
   lapis kedua di aplikasi.

## Verifikasi manual (pemilik aplikasi)

- Unggah >10 MB → 413; file salah jenis lewat popover → 422/ditolak klien.
- Unggah 6 berkas chat di satu brief → yang ke-6 ditolak (429) + pesan jelas.
- Bunuh API di tengah unggah → baris `processing` dipaksa `failed` ≤ 6 jam.
- Setelah `UPLOAD_RETENTION_HOURS` terlewati → berkas fisik hilang, badge
  "Berkas dibersihkan" muncul, AI masih ingat isinya.
- Log scheduler memuat baris `[upload-cleanup] siklus selesai` tiap interval.
