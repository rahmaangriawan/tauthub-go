-- 294 — ID grup WA admin disimpan terenkripsi seperti token Fonnte.
--
-- Sebelumnya admin_group_target disimpan apa adanya (plaintext), padahal ID
-- grup adalah identitas tujuan notifikasi: siapa pun yang bisa membaca tabel
-- bisa memakainya untuk menyadap/menyalahgunakan tujuan. Sekarang nilai baru
-- ditulis ke sealed_admin_group_target dengan format sealed yang sama seperti
-- sealed_token (t2:, AES-GCM dengan CMS_ENCRYPTION_SECRET/APP_SECRET).
--
-- Kolom lama TIDAK dihapus: data yang sudah ada dibaca sebagai fallback dan
-- dienkripsi ulang oleh BackfillWAGroupTarget saat API start. Setelah backfill
-- berhasil, admin_group_target diset NULL.
ALTER TABLE public.platform_wa_config
  ADD COLUMN IF NOT EXISTS sealed_admin_group_target text;
