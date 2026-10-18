-- Kategori publikasi yang mungkin diubah mitra di editor draf; kosong = mengikuti
-- kategori media pilihan klien saat order.
ALTER TABLE public.publications ADD COLUMN IF NOT EXISTS category_override text;
