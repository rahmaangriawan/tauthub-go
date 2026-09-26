-- 153 — Mode otomatis (terbit tanpa ACC mitra) untuk media mitra terpilih.
-- Aditif: dua kolom boolean + satu indeks parsial, tanpa DROP/ubah data.
--
-- Kenapa kolom eksplisit, bukan "partner_id IS NULL": ada puluhan media seed
-- tanpa mitra yang JELAS bukan media otomatis kita. Menumpangkan arti pada
-- partner_id membuat keduanya mustahil dibedakan lagi di kemudian hari.
--
-- Gerbang izin dua lapis, sengaja dipisah:
--   partners.auto_publish_enabled = ADMIN mengizinkan MITRA memakai fitur ini
--   media.auto_publish            = MITRA menyalakan mode untuk MEDIA tertentu
-- Publikasi yang memakai media dengan kedua syarat terpenuhi langsung masuk
-- antrean terbit ('queued') tanpa melewati 'waiting_partner'.

ALTER TABLE public.partners
  ADD COLUMN IF NOT EXISTS auto_publish_enabled boolean NOT NULL DEFAULT false;

ALTER TABLE public.media
  ADD COLUMN IF NOT EXISTS auto_publish boolean NOT NULL DEFAULT false;

-- Daftar media otomatis (halaman admin) memfilter kolom ini.
CREATE INDEX IF NOT EXISTS ix_media_auto_publish
  ON public.media (auto_publish) WHERE auto_publish = TRUE;
