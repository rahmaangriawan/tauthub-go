-- 169: Jalur "Kampanye Kilat" untuk taut_entry_mode.
--
-- Kampanye Kilat adalah pintu masuk ringkas (URL website + anggaran, lalu AI
-- memilih medianya) yang melewati The Profiler. Router kampanye perlu tahu
-- sebuah kampanye lahir dari jalur ini supaya "Buka Kampanye" mengarah ke
-- /kilat, bukan /profiler: tanpa penanda, kampanye kilat berstatus 'draft'
-- sama persis dengan kampanye AI Pintar yang belum diisi.
--
-- Pola sama seperti 079: DROP + ADD constraint (hanya daftar nilai). Nilai
-- 'kilat' menambah pilihan; baris lama 'ai'/'manual' tetap sah tanpa backfill.

BEGIN;

ALTER TABLE public.campaigns DROP CONSTRAINT IF EXISTS ck_campaigns_taut_entry_mode;

ALTER TABLE public.campaigns ADD CONSTRAINT ck_campaigns_taut_entry_mode CHECK (
  taut_entry_mode IN ('ai', 'manual', 'kilat')
);

COMMIT;
