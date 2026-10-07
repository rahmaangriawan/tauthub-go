-- 193 — media_seo_metrics: izinkan baris tanpa nilai sama sekali.
--
-- Konteks: sejak skor AI Trust tidak lagi diisi angka default, satu pemeriksaan
-- bisa berakhir tanpa satu pun nilai terukur — DA/PA gagal (penyedia menolak),
-- kecepatan situs tidak bisa diukur, dan Google tidak menampilkan AI Overview.
-- Barisnya tetap HARUS tersimpan karena checked_at-nya yang menahan agar media
-- tidak diperiksa berulang tiap siklus, tapi CHECK lama
-- (ck_media_seo_metrics_value dari migrasi 147) menuntut minimal satu nilai
-- terisi sehingga INSERT-nya gagal:
--   ERROR: new row for relation "media_seo_metrics" violates check constraint
--
-- Kolomnya memang nullable semua; "belum bisa ditentukan" adalah keadaan yang
-- sah. Constraint dibuang, bukan diganti bentuk lain, supaya tidak ada lagi
-- daftar kolom yang harus ikut diperbarui tiap kali sinyal baru ditambah.
-- Batas rentang per kolom (ck_media_seo_metrics_ai_trust / _spam) tetap berlaku.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'ck_media_seo_metrics_value'
      AND conrelid = 'public.media_seo_metrics'::regclass
  ) THEN
    ALTER TABLE public.media_seo_metrics DROP CONSTRAINT ck_media_seo_metrics_value;
  END IF;
END $$;
