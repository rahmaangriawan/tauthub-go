-- 194 — catat kegagalan penyedia DA/PA terakhir supaya bisa ditampilkan di UI.
--
-- Konteks: kegagalan penyedia (mis. kuota bulanan habis) tidak menggagalkan
-- pemeriksaan karena sinyal lain (AI Trust dari PageSpeed) tetap tersimpan.
-- Akibatnya job selesai "sukses" sementara kolom DA tetap kosong dan admin tidak
-- pernah tahu sebabnya, hanya melihat "media belum punya skor".
ALTER TABLE public.platform_seo_config ADD COLUMN IF NOT EXISTS last_da_error text;
ALTER TABLE public.platform_seo_config ADD COLUMN IF NOT EXISTS last_da_error_at timestamp(6) with time zone;
