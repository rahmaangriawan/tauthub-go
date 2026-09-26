-- 157: Dukungan embed (Video/Maps) per media.
--
-- Kapabilitas yang diberikan MITRA untuk medianya: bila menyala, artikel yang
-- tayang di media itu boleh memuat embed video/peta. Klien melihat statusnya di
-- popup detail media (katalog klien): "Diizinkan" / "Tidak Diizinkan".
--
-- Aditif: satu kolom baru. DEFAULT false -> seluruh media yang sudah ada
-- otomatis "tidak diizinkan", jadi tidak ada perilaku yang berubah sampai mitra
-- menyalakannya sendiri.

ALTER TABLE public.media
    ADD COLUMN IF NOT EXISTS allow_embed boolean NOT NULL DEFAULT false;
