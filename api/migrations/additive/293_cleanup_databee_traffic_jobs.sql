-- 293 — Bersihkan job cek trafik yang gagal karena kredensial provider lama.
--
-- Latar: provider trafik diganti dari "Similarweb API Pro" (DataBee, wajib dua
-- kredensial) ke "Similar Web API" (belchiorarkad, cukup satu key RapidAPI).
-- Job `media_similarweb_refresh` yang gagal karena syarat kredensial provider
-- lama tidak akan pernah berhasil bila di-retry — penyebabnya konfigurasi, bukan
-- gangguan sesaat — dan hanya menutupi kegagalan baru di tab Log.
--
-- Yang dihapus HANYA job dengan pesan khas kredensial provider lama. Job gagal
-- karena sebab lain (domain mati, timeout, rate limit) dibiarkan supaya masih
-- bisa ditelusuri di tab Log. Retensi harian tetap memangkas job >3 hari.
DELETE FROM public.jobs
WHERE kind = 'media_similarweb_refresh'
  AND status = 'failed'
  AND (
        last_error ILIKE '%DataBee Token%'
     OR last_error ILIKE '%Similarweb API Pro%'
     OR last_error ILIKE '%similarweb-api-pro%'
  );

-- Snapshot lama TIDAK disentuh: trafik yang sudah tersimpan tetap tampil di
-- katalog. Media yang belum punya snapshot akan masuk antrean sweep berikutnya.
