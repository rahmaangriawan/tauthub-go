-- 167 — Taut Pintar: catat jalur masuk kampanye (AI vs manual) saat dibuat.
--
-- Kenapa perlu kolom eksplisit: UI perlu tahu apakah kampanye melewati The
-- Profiler. Menyimpulkannya dari "ada hasil Profiler atau tidak" salah untuk
-- kampanye AI lama (dibuat sebelum Profiler v2) — kampanye itu akan dikira
-- manual, pill The Profiler-nya terkunci, dan arahan "mulai dari The Profiler"
-- muncul padahal brief-nya sudah ada.
--
-- Aditif & backward compatible: baris lama dibiarkan NULL. NULL = tidak
-- diketahui, diperlakukan sebagai jalur AI (perilaku yang berlaku sekarang).

BEGIN;

ALTER TABLE public.campaigns
  ADD COLUMN IF NOT EXISTS taut_entry_mode character varying(10);

DO $$ BEGIN
  ALTER TABLE public.campaigns ADD CONSTRAINT ck_campaigns_taut_entry_mode
    CHECK (taut_entry_mode IN ('ai', 'manual'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

COMMIT;
