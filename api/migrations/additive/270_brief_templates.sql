-- 270 — Pustaka template brief.
--
-- Brief yang sudah matang di satu brand sering dipakai ulang untuk brand lain
-- di agency yang sama. Sebelum ini satu-satunya cara adalah duplikasi profil di
-- dalam brand itu sendiri. Template menyimpan SNAPSHOT isi brief (bukan tautan
-- ke profil sumber), jadi mengubah profil asal tidak mengubah template yang
-- sudah tersimpan.
--
-- Cakupan: brand_id untuk template milik satu brand, workspace_id untuk
-- template yang dibagikan ke seluruh brand dalam satu agency. Keduanya boleh
-- kosong bersamaan hanya bila baris dibuat di luar alur handler; handler selalu
-- mengisi brand_id.
--
-- Soft delete dipakai karena template yang sudah dipakai brief lain tetap perlu
-- jejaknya, dan DELETE keras akan menghapus riwayat yang dirujuk audit.
--
-- Aditif dan idempoten. Runner: api/cmd/migrate (migrate up).

CREATE TABLE IF NOT EXISTS public.brief_templates (
    id character(36) NOT NULL PRIMARY KEY,
    workspace_id character(36) REFERENCES public.agency_workspaces (id) ON DELETE CASCADE,
    brand_id character(36) REFERENCES public.brands (id) ON DELETE CASCADE,
    name character varying(160) NOT NULL,
    description text,
    entity_core jsonb NOT NULL DEFAULT '{}'::jsonb,
    identity jsonb NOT NULL DEFAULT '{}'::jsonb,
    style jsonb NOT NULL DEFAULT '{}'::jsonb,
    seo jsonb NOT NULL DEFAULT '{}'::jsonb,
    instructions text,
    created_by character(36),
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    deleted_at timestamp with time zone
);

-- Daftar template selalu difilter "belum dihapus", jadi index parsial cukup.
CREATE INDEX IF NOT EXISTS ix_brief_templates_workspace
    ON public.brief_templates (workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_brief_templates_brand
    ON public.brief_templates (brand_id) WHERE deleted_at IS NULL;
