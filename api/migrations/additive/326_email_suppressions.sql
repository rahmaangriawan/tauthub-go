-- 326 — Daftar alamat email yang terbukti mati (hard bounce). Alamat di sini
-- tidak dikirimi email lagi supaya reputasi SMTP tidak rusak. Diisi otomatis saat
-- server SMTP menolak penerima secara permanen (mis. 550 5.1.1 user unknown).
-- Additive dan idempoten.

BEGIN;

CREATE TABLE IF NOT EXISTS public.email_suppressions (
  email      text PRIMARY KEY,
  reason     text NOT NULL,
  detail     text,
  created_at timestamptz NOT NULL DEFAULT now()
);

COMMIT;
