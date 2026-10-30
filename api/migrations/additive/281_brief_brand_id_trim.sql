-- 281 — Rapikan brand_id brief yang tersimpan berpad spasi.
--
-- profiler_profiles.brand_id bertipe character(36), bukan varchar: Postgres
-- mem-pad nilai pendek dengan spasi sampai 36 karakter. Public ID brand hanya
-- 6 karakter, jadi yang tersimpan sebenarnya "cza87b" + 30 spasi.
--
-- Dampak nyata:
--   - job ringkasan brief membandingkan brand dari payload dengan nilai dari
--     baris profil, selalu tidak sama, dan setiap job ditolak sebagai
--     "payload brand tidak cocok" — ringkasan tidak pernah bisa diperbarui;
--   - brief_memory_facts.brand_id (varchar) menerima nilai berpad itu, dan
--     RecallFacts mencari dengan public_id tanpa pad — memori lintas brief
--     tidak pernah ketemu.
--
-- Perbaikan di kode: setiap pembacaan brand_id dari profiler_profiles memakai
-- btrim(brand_id::text). Migrasi ini hanya merapikan salinan yang SUDAH
-- tertulis berpad di kolom varchar, karena kolom character akan selalu
-- kembali berpad bila di-update.
--
-- Idempoten: baris yang sudah bersih tidak berubah.

UPDATE public.brief_memory_facts
SET brand_id = btrim(brand_id)
WHERE brand_id <> btrim(brand_id);
