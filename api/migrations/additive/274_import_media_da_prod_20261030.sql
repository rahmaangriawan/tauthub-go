-- 274 — Impor DA media production (dump 2026-10-30 09:00) dari Bulk DA PA Checker.
--
-- 68 domain media AKTIF di production yang belum punya DA dicek lewat
-- https://tools.guestpostlinks.net/bulk-da-pa-checker-tool/ pada 2026-10-30,
-- hasilnya (kolom URLs + DA) ditanam di bawah ini. Tidak ada kolom/tabel baru —
-- nilai DA masuk ke public.media_seo_metrics yang sudah dipakai listing media.
--
-- Aturan (tanpa tebakan):
--   * Pencocokan HANYA dari host website_url media, dinormalisasi sama seperti
--     aplikasi (buang skema, path, port, awalan www, titik akhir).
--   * Domain yang cocok ke LEBIH DARI SATU media.id dilewati, bukan dipilih acak.
--   * Domain yang muncul di laporan dengan DA berbeda dilewati; DA identik disatukan.
--   * Hanya kolom da/checked_at/source yang ditulis. pa, spam_score, dan sinyal
--     AI TIDAK disentuh, jadi nilainya tetap seperti semula.
--   * checked_at memakai waktu laporan (2026-10-30T09:41:35+07:00), bukan waktu
--     migrasi; WHERE pada DO UPDATE membuat eksekusi ulang jadi no-op.
--
-- Aditif & idempoten (dijalankan oleh cmd/migrate).

BEGIN;

WITH report_raw(domain, da) AS (
  VALUES
  ('bekasivoice.com', 11),
  ('beritaindramayu.com', 11),
  ('beritaradar.com', 7),
  ('bisniscirebon.com', 8),
  ('cirebonbribin.com', 14),
  ('cirebonevent.com', 19),
  ('cirebonniaga.com', 10),
  ('cirebononline.com', 5),
  ('cirebonraya.com', 22),
  ('ciremainews.com', 11),
  ('ciremaitoday.com', 13),
  ('citrust.id', 19),
  ('deltanews.com', 7),
  ('deskjabar.com', 10),
  ('dialektika.com', 37),
  ('dialogcirebon.com', 4),
  ('dialogpublik.com', 47),
  ('djalapaksi.com', 4),
  ('esatu.com', 7),
  ('etnikom.net', 12),
  ('etnologi.com', 7),
  ('fajarcirebon.co', 1),
  ('fokuscirebon.com', 6),
  ('fokusjabar.com', 36),
  ('fokusmedia.id', 34),
  ('fokusmuria.co.id', 41),
  ('gemamadani.com', 8),
  ('gerbangberita.com', 7),
  ('gragepolitan.com', 17),
  ('halobogor.com', 10),
  ('hariandialog.com', 8),
  ('harianpelitanews.com', 1),
  ('idejabar.com', 12),
  ('indomedianews.com', 5),
  ('indramayujeh.com', 49),
  ('investigasi.co.id', 12),
  ('jabarinfo.com', 7),
  ('jabarnews.com', 34),
  ('jabartandang.com', 8),
  ('kartinikuningan.com', 4),
  ('kejarfakta.id', 6),
  ('kilascirebon.com', 10),
  ('kuninganjabarinfo.com', 3),
  ('kuninganonline.com', 40),
  ('kuninganreligi.com', 10),
  ('lifestyle.suaramerdeka.com', 57),
  ('likecirebon.com', 5),
  ('literasi.co.id', 5),
  ('mediapatriotindonesia.com', 10),
  ('mitranews.com', 7),
  ('opsi.id', 19),
  ('pesanjabar.com', 3),
  ('pikirancirebon.com', 3),
  ('pintasnews.com', 5),
  ('ringkasnews.id', 10),
  ('siaranbekasi.com', 4),
  ('siwindu.com', 4),
  ('snipernews.id', 4),
  ('suaracirebon.com', 20),
  ('sumbermedia.com', 4),
  ('suryagrageonline.com', 11),
  ('talentanews.id', 1),
  ('temporatur.com', 11),
  ('teraskata.com', 18),
  ('veritanews.id', 1),
  ('warpol.id', 2),
  ('yzab.id', 1),
  ('zonabandung.com', 17)
),
report AS (
  SELECT
    regexp_replace(
      regexp_replace(
        regexp_replace(
          regexp_replace(
            regexp_replace(
              lower(btrim(domain)),
              '^[a-z][a-z0-9+.-]*://', ''),   -- buang skema
            '[/?#].*$', ''),                  -- buang path/query/fragment
          ':[0-9]+$', ''),                    -- buang port
        '^www\.', ''),                       -- buang awalan www.
      '\.$', '')                             -- buang titik akhir
      AS host,
    da
  FROM report_raw
),
report_dedup AS (
  SELECT host, min(da) AS da
  FROM report
  WHERE host <> '' AND da BETWEEN 0 AND 100
  GROUP BY host
  HAVING count(DISTINCT da) = 1
),
media_norm AS (
  SELECT
    m.id,
    regexp_replace(
      regexp_replace(
        regexp_replace(
          regexp_replace(
            regexp_replace(
              lower(btrim(m.website_url)),
              '^[a-z][a-z0-9+.-]*://', ''),
            '[/?#].*$', ''),
          ':[0-9]+$', ''),
        '^www\.', ''),
      '\.$', '')
      AS host
  FROM public.media m
  WHERE m.status = 'active'
    AND NULLIF(btrim(m.website_url), '') IS NOT NULL
),
target AS (
  SELECT host, (array_agg(id))[1] AS media_id
  FROM media_norm
  WHERE host <> ''
  GROUP BY host
  HAVING count(*) = 1
)
INSERT INTO public.media_seo_metrics (media_id, da, checked_at, source, created_at, updated_at)
SELECT
  t.media_id,
  rd.da,
  '2026-10-30T09:41:35+07:00'::timestamp with time zone,
  'guestpostlinks-da-tool',
  now(),
  now()
FROM target t
JOIN report_dedup rd ON rd.host = t.host
ON CONFLICT (media_id) DO UPDATE
SET da         = EXCLUDED.da,
    checked_at = EXCLUDED.checked_at,
    source     = EXCLUDED.source,
    updated_at = now()
WHERE (media_seo_metrics.da, media_seo_metrics.checked_at, media_seo_metrics.source)
      IS DISTINCT FROM (EXCLUDED.da, EXCLUDED.checked_at, EXCLUDED.source);

COMMIT;
