-- 268 — Agency workspace: pengelompokan brand + fallback otorisasi.
--
-- Satu agency mengelola banyak brand klien. Keanggotaan per brand
-- (brand_memberships) tetap jadi izin yang paling spesifik, sedangkan
-- keanggotaan workspace jadi lapis kedua: anggota agency tidak perlu didaftarkan
-- ulang setiap kali brand baru dibuat. Baris brand_memberships yang eksplisit
-- SELALU menang; workspace hanya fallback (lihat brands.EffectiveRole).
--
-- brands.workspace_id sengaja ON DELETE SET NULL: menghapus workspace tidak
-- boleh ikut menghapus brand klien beserta riwayat publikasinya, hanya melepas
-- pengelompokannya.
--
-- Aditif dan idempoten. Runner: api/cmd/migrate (migrate up).

CREATE TABLE IF NOT EXISTS public.agency_workspaces (
    id character(36) NOT NULL PRIMARY KEY,
    name character varying(160) NOT NULL,
    slug character varying(160) NOT NULL UNIQUE,
    owner_user_id character(36) NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    deleted_at timestamp with time zone
);

CREATE TABLE IF NOT EXISTS public.workspace_members (
    workspace_id character(36) NOT NULL REFERENCES public.agency_workspaces (id) ON DELETE CASCADE,
    user_id character(36) NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    role character varying(20) NOT NULL DEFAULT 'writer',
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id)
);

ALTER TABLE public.brands
    ADD COLUMN IF NOT EXISTS workspace_id character(36)
    REFERENCES public.agency_workspaces (id) ON DELETE SET NULL;

-- Query peredam cache & daftar brand workspace hanya menyentuh baris yang
-- punya workspace, jadi index parsial cukup.
CREATE INDEX IF NOT EXISTS ix_brands_workspace
    ON public.brands (workspace_id) WHERE workspace_id IS NOT NULL;

-- Kebalikan dari index di atas: "workspace apa saja milik user ini" dipanggil
-- setiap GET /v1/workspaces dan GET /v1/brands.
CREATE INDEX IF NOT EXISTS ix_workspace_members_user
    ON public.workspace_members (user_id);
