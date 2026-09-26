-- Verifikasi kepemilikan domain per media.
--
-- Mitra menempel token unik di websitenya (meta tag / script /
-- komentar HTML), lalu worker memverifikasi via fetch SSRF-safe.
-- Kolom recheck (domain_verified + domain_last_checked_at) dipakai
-- sweep scheduler mingguan; fail_count 3x mencabut status verified.
-- Data lama otomatis unverified (default false) — tidak ada data
-- yang diubah/dihapus.

ALTER TABLE public.media
    ADD COLUMN IF NOT EXISTS domain_verify_token varchar(64),
    ADD COLUMN IF NOT EXISTS domain_verified boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS domain_verified_at timestamptz,
    ADD COLUMN IF NOT EXISTS domain_last_checked_at timestamptz,
    ADD COLUMN IF NOT EXISTS domain_check_fail_count integer NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_media_domain_recheck
    ON public.media (domain_verified, domain_last_checked_at);
