-- 134 — jatah generate artikel per akun (The Writer).
-- Kolom di users: total maksimal generate per artikel (generate pertama + regenerasi).
-- NULL = default aplikasi (3 = 1x generate pertama + 2x regenerasi).
-- Diatur platform admin; contoh: akun tester diberi 999 agar praktis tanpa batas.
BEGIN;

ALTER TABLE public.users ADD COLUMN IF NOT EXISTS writer_generate_quota smallint;
COMMENT ON COLUMN public.users.writer_generate_quota IS 'Jatah generate artikel Writer per artikel (pertama+regen); NULL = default 3';

COMMIT;
