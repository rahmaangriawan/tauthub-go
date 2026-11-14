-- 327 — Secret webhook email Kirim.id (bounce/komplain), disimpan ter-seal di
-- smtp_settings dan diisi admin dari halaman Konfigurasi > Email / SMTP. Bila
-- kosong, handler webhook jatuh ke env KIRIM_WEBHOOK_SECRET. Additive dan idempoten.

BEGIN;

ALTER TABLE public.smtp_settings
  ADD COLUMN IF NOT EXISTS webhook_secret_sealed character varying(2000);

COMMIT;
