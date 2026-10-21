-- 229 — Masa berlaku tautan (backlink) per media.
--
-- Kebijakan tiap media berbeda: sebagian menjamin tautannya PERMANEN (tidak
-- pernah dilepas), sebagian hanya tahan sekian bulan (umumnya 12 bulan / "12 Bln
-- SLA"). Kolom ini jadi satu-satunya sumber kebenaran untuk:
--   - badge di katalog media & kartu media ("Permanen" / "12 Bln SLA"),
--   - masa pemantauan 404 (link monitor) dan masa garansi klaim Link Guardian.
--
-- Nilai: 0 = permanen, >0 = jumlah bulan sejak tanggal tayang.
-- Default 12 supaya perilaku seluruh baris lama TIDAK berubah (sistem sebelumnya
-- memang mengasumsikan garansi tayang 12 bulan untuk semua media).
ALTER TABLE public.media
  ADD COLUMN IF NOT EXISTS link_duration_months integer NOT NULL DEFAULT 12;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
     WHERE conname = 'media_link_duration_months_check'
       AND conrelid = 'public.media'::regclass
  ) THEN
    ALTER TABLE public.media
      ADD CONSTRAINT media_link_duration_months_check
      CHECK (link_duration_months >= 0 AND link_duration_months <= 120);
  END IF;
END $$;

COMMENT ON COLUMN public.media.link_duration_months IS
  'Masa berlaku tautan: 0 = permanen, >0 = bulan sejak terbit. Dipakai pemantauan 404 & kelayakan klaim Link Guardian.';
