-- 197 — Impor DA media dari laporan Bulk DA PA Checker (guestpostlinks).
--
-- Sekali jalan: 93 domain media aktif dicek lewat
-- https://tools.guestpostlinks.net/bulk-da-pa-checker-tool/ dan hasilnya (kolom
-- URLs + DA) ditanam di bawah ini sebagai laporan. Tidak ada kolom/tabel baru —
-- nilai DA masuk ke public.media_seo_metrics yang sudah dipakai listing media
-- (admin, partner, katalog klien).
--
-- Aturan (tanpa tebakan):
--   * Pencocokan HANYA dari host website_url media, dinormalisasi sama seperti
--     aplikasi (buang skema, path, port, awalan www). Nama media tidak dipakai.
--   * Domain yang cocok ke LEBIH DARI SATU media.id dilewati, bukan dipilih acak.
--   * Domain yang muncul di laporan dengan DA berbeda dilewati; DA identik disatukan.
--   * Hanya kolom da/checked_at/source yang ditulis. pa, spam_score, dan sinyal
--     AI TIDAK disentuh, jadi nilainya tetap seperti semula.
--   * checked_at memakai waktu laporan (2026-10-07T17:27:43+00:00), bukan waktu migrasi, dan
--     WHERE pada DO UPDATE membuat eksekusi ulang jadi no-op.
--
-- Aditif & idempoten (dijalankan oleh cmd/migrate).

BEGIN;

WITH report_raw(domain, da) AS (
  VALUES
  ('arusberita.com', 8),
  ('asiaberita.com', 9),
  ('bekasi.pojoksatu.id', 53),
  ('bewarajabar.com', 44),
  ('bicaramusik.com', 6),
  ('bimata.id', 16),
  ('kolektifmedia.id', 7),
  ('bloggerborneo.com', 51),
  ('bogor.pojoksatu.id', 53),
  ('bogorinsider.com', 17),
  ('bukusekolah.id', 18),
  ('cakrawalaindonesia.id', 12),
  ('cirebonbagus.id', 39),
  ('conference.co.id', 9),
  ('cryptomedia.id', 36),
  ('danuadji.com', 13),
  ('depokpos.com', 27),
  ('elektro.web.id', 15),
  ('eranasional.com', 15),
  ('fajarharapan.id', 39),
  ('jatiblora.com', 26),
  ('inafurn.com', 14),
  ('gigscorner.com', 16),
  ('giwangkara.com', 25),
  ('globmag.id', 4),
  ('hangout.id', 13),
  ('idnberita.com', 9),
  ('indramayubagus.id', 6),
  ('infogigs.id', 10),
  ('infolistrik.com', 6),
  ('inilahnews.com', 47),
  ('jabarbagus.id', 8),
  ('jabar.pojoksatu.id', 53),
  ('jakpos.id', 6),
  ('jasarelease.co.id', 6),
  ('javasatu.com', 22),
  ('journalbroadcast.co', 10),
  ('kabarbanyuwangi.co.id', 49),
  ('kanalwisata.com', 23),
  ('kawanbisnis.com', 9),
  ('kisarantoday.com', 10),
  ('kolektif.id', 17),
  ('kpopsquad.com', 11),
  ('lapenmi.id', 7),
  ('lensanusantara.co.id', 50),
  ('lenterabisnis.com', 28),
  ('lenterakecil.com', 27),
  ('lenteratimes.com', 15),
  ('liganesia.com', 8),
  ('liputania.com', 7),
  ('lombokepo.site', 10),
  ('magelangraya.id', 11),
  ('majalahbogor.com', 3),
  ('majalahekonomi.com', 6),
  ('medanjurnalis.com', 11),
  ('medanlia.com', 11),
  ('banyumas24jam.com', 9),
  ('melansir.com', 13),
  ('mimbaropini.com', 9),
  ('otolifestyle.com', 4),
  ('patinews.com', 29),
  ('pepo.id', 18),
  ('pewarta.co.id', 54),
  ('pojokbandung.com', 12),
  ('pojoksatu.id', 53),
  ('postkotantb.com', 32),
  ('pramugari-pramugara.blogspot.com', 1),
  ('purworejomedia.com', 44),
  ('radarbogor.jawapos.com', 80),
  ('radarcianjur.com', 25),
  ('radardepok.com', 31),
  ('radarsukabumi.com', 39),
  ('rbg.id', 22),
  ('reaksinasional.com', 18),
  ('reportase7.com', 2),
  ('sewaktu.com', 23),
  ('sewaktu.id', 5),
  ('siar.co.id', 18),
  ('sindikasiindonesia.id', 8),
  ('skulizm.com', 10),
  ('suarapantau.com', 8),
  ('sulsel.suarapantau.com', 8),
  ('suaraparepare.com', 6),
  ('sulsel.pojoksatu.id', 53),
  ('sumutnesia.com', 6),
  ('sundaurang.id', 10),
  ('sutoro.web.id', 33),
  ('semarangzoo.com', 17),
  ('tripasahan.com', 9),
  ('tripsumut.com', 6),
  ('wartaaktual.com', 5),
  ('wisatasiana.com', 20),
  ('wisenfin.com', 8)
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
  '2026-10-07T17:27:43+00:00'::timestamp with time zone,
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
