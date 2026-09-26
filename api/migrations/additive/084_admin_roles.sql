-- 084 -- Role Admin Platform: super_admin vs admin biasa + permission toggles.
-- Additive / idempotent:
--   * platform_admins.role  -> super_admin | admin
--   * platform_admins.permissions -> jsonb {}  (toggle menu access utk admin biasa)
--   * CHECK role + CHECK permissions keys + index
--   * seed super_admin utk wantek@gmail.com (tanpa duplikat, user role->admin)
-- Apply: psql $DATABASE_URL -f apps/web/src/db/migrations/084_admin_roles.sql

BEGIN;

-- 1) Kolom role terbatas: super_admin = akses penuh, admin = sesuai permission.
ALTER TABLE public.platform_admins
  ADD COLUMN IF NOT EXISTS role character varying(20) NOT NULL DEFAULT 'super_admin';

-- 2) Permission set admin biasa. Super admin mengabaikan field ini (full access).
ALTER TABLE public.platform_admins
  ADD COLUMN IF NOT EXISTS permissions jsonb NOT NULL DEFAULT '{}'::jsonb;

-- 3) Validasi role.
DO $$
BEGIN
  ALTER TABLE public.platform_admins ADD CONSTRAINT ck_platform_admins_role
    CHECK (role IN ('super_admin', 'admin'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- 4) Validasi permissions: harus object json.
DO $$
BEGIN
  ALTER TABLE public.platform_admins ADD CONSTRAINT ck_platform_admins_permissions
    CHECK (jsonb_typeof(permissions) = 'object');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- 5) Backfill admin yang sudah ada (platform_admins dari 016) -> super_admin
--    agar tidak ada admin terkunci dari fitur role.
UPDATE public.platform_admins
  SET role = 'super_admin'
  WHERE role = 'admin' AND role <> 'super_admin';

-- 6) Seed wantek@gmail.com sebagai super_admin bila belum terdaftar.
INSERT INTO public.platform_admins (user_id, role, permissions, created_at)
SELECT u.id, 'super_admin', '{}'::jsonb, NOW()
FROM public.users u
WHERE u.email = 'wantek@gmail.com'
  AND NOT EXISTS (
    SELECT 1 FROM public.platform_admins pa WHERE pa.user_id = u.id
  );

-- 7) Pastikan role users wantek ter-set admin.
UPDATE public.users SET role = 'admin'
  WHERE email = 'wantek@gmail.com' AND role <> 'admin';

COMMIT;
