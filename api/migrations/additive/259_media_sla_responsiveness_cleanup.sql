-- 259 — SLA Responsiveness: buang publikasi menggantung >7 hari + batas
-- tampilan 72 jam.
--
-- Dua perubahan di engine health media (scheduler brandflow +
-- platform media_support):
--   1) AVG publish-time hanya dari publikasi yang terbit <= 7 hari sejak
--      dibuat. Publikasi yang menggantung karena mitra lupa ACC (atau baru
--      ACC setelah lewat seminggu) tidak lagi menyeret rata-rata SLA.
--   2) Nilai avg_publish_hours yang DISIMPAN untuk tampilan dibatasi 72 jam
--      (3 hari) untuk media berintegrasi API (wordpress/blogger). Media
--      integrasi manual (butuh ACC mitra) dikecualikan. Nilai mentah tetap
--      disimpan di avg_publish_hours_raw untuk skor & peringatan admin.
--
-- Migrasi ini hanya memaksa hitung ulang: health_computed_at dikosongkan
-- supaya scheduler menyegarkan breakdown semua media aktif pada siklus
-- berikutnya (limit 50 per siklus).
UPDATE media
SET health_computed_at = NULL
WHERE status IN ('active', 'draft');
