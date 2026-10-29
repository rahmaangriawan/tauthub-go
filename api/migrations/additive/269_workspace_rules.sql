-- 269 — Aturan default per workspace.
--
-- Agency punya gaya penulisan dan pedoman SEO yang sama untuk banyak brand
-- klien. Nilainya disemai ke brief PERTAMA brand baru (lihat
-- workspace.SeedBrandBriefDefaults), jadi brand yang sudah punya nilai sendiri
-- tidak tertimpa: seeding hanya menulis kunci yang masih kosong.
--
-- Aditif. Runner: api/cmd/migrate (migrate up).

CREATE TABLE IF NOT EXISTS public.workspace_rules (
    workspace_id character(36) NOT NULL PRIMARY KEY
        REFERENCES public.agency_workspaces (id) ON DELETE CASCADE,
    entity_defaults jsonb NOT NULL DEFAULT '{}'::jsonb,
    style_defaults jsonb NOT NULL DEFAULT '{}'::jsonb,
    seo_defaults jsonb NOT NULL DEFAULT '{}'::jsonb,
    forbidden_words text,
    forbidden_claims text,
    updated_by character(36),
    updated_at timestamp with time zone NOT NULL DEFAULT now()
);

-- Penandaan asal nilai brief dari workspace harus sah.
--
-- Nama constraint asli dari migrasi 212 adalah ck_taut_brief_field_state_source,
-- bukan taut_brief_field_state_source_check. Keduanya di-drop dengan IF EXISTS
-- supaya file ini aman baik di DB yang memakai definisi 212 maupun yang pernah
-- memakai nama lain.
ALTER TABLE public.taut_brief_field_state
    DROP CONSTRAINT IF EXISTS ck_taut_brief_field_state_source;
ALTER TABLE public.taut_brief_field_state
    DROP CONSTRAINT IF EXISTS taut_brief_field_state_source_check;
ALTER TABLE public.taut_brief_field_state
    ADD CONSTRAINT ck_taut_brief_field_state_source
    CHECK ((source)::text = ANY (ARRAY['user'::text, 'url'::text, 'document'::text, 'ai_suggested'::text, 'workspace'::text]));
