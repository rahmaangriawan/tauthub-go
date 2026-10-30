-- 286 — Izin subjudul per media (allow_heading).
--
-- Semua 13 angle mainstream punya aturan "LOGIKA KONDISIONAL PENGGUNAAN
-- SUBJUDUL": artikel di bawah ~500 kata ditulis tanpa subjudul, artikel lebih
-- panjang boleh memakai 2-3 subjudul deklaratif. Tapi kode selama ini mengubah
-- SEMUA heading pada media mainstream menjadi paragraf (demoteHeadingBlocks),
-- sehingga aturan subjudul di prompt angle tidak pernah berpengaruh.
--
-- Kolom ini membuat izin subjudul mengikuti keputusan pemilik media, bukan
-- dipaksa satu aturan global:
--   NULL  = ikut bawaan tipe media (mainstream: tanpa subjudul, blogger: boleh)
--   TRUE  = media ini boleh memakai subjudul walau bertipe mainstream
--   FALSE = media ini dilarang memakai subjudul walau bertipe blogger
--
-- Default NULL menjaga perilaku lama tetap sama sampai pemilik media mengubah
-- pengaturannya.
ALTER TABLE public.media
  ADD COLUMN IF NOT EXISTS allow_heading boolean;

COMMENT ON COLUMN public.media.allow_heading IS
  'Izin subjudul artikel. NULL = ikut tipe media (mainstream tanpa subjudul, blogger boleh).';
