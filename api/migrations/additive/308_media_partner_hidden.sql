-- 308 — Mitra bisa menyembunyikan medianya dari katalog klien sendiri.
--
-- media.partner_hidden = TRUE: media tidak tampil di katalog klien, tidak
-- direkomendasikan AI, dan tidak bisa dipesan baru. Order yang sudah berjalan
-- tidak terpengaruh. Ini terpisah dari media.status (dikelola admin lewat
-- review), suspended_at (kendali admin), dan kesiapan integrasi (otomatis),
-- jadi mitra menghidupkannya kembali tanpa persetujuan siapa pun.
-- Aditif dan idempoten.

BEGIN;

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS partner_hidden boolean NOT NULL DEFAULT false;

COMMIT;
