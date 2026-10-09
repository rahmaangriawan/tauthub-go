-- 202 — Profiler (Structured Knowledge Workspace) tingkat klien.
--
-- Klien menyimpan profil bisnis/identitas brand, gaya penulisan, pedoman konten
-- & SEO, instruksi khusus, dokumen referensi, gambar referensi, URL referensi,
-- dan blok knowledge kustom. Data disimpan TERSTRUKTUR (bukan satu blob prompt);
-- backend menyusunnya menjadi konteks untuk The Writer saat artikel dibuat.
--
-- Catatan: prefix `profiler_*` dipakai (beda dari `taut_profiler_*` yang sudah
-- ada = agent "Profiler" per-kampanye di alur Taut Pintar). Keduanya entitas
-- berbeda; yang ini milik klien (brand) dan reusable lintas kampanye.

CREATE TABLE IF NOT EXISTS public.profiler_profiles (
    id character(36) NOT NULL PRIMARY KEY,
    brand_id character(36) NOT NULL,
    name character varying(160) NOT NULL,
    description text,
    -- Kelompok field terstruktur (jsonb):
    --   identity: brand_name, industry, business_description, products, audience,
    --             problems_solved, usp, values, vision, mission, region, website,
    --             contact, key_facts[]
    --   style:    tone, brand_character, formality, greeting, point_of_view,
    --             depth, length_preference, language, preferred_terms[],
    --             forbidden_terms[], eyd_guideline, opening_preference, closing_preference
    --   seo:      article_goal, search_intent, primary_keyword, secondary_keywords[],
    --             required_topics[], forbidden_topics[], heading_structure,
    --             use_tables, use_lists, use_faq, use_examples, internal_link_rules,
    --             brand_mention_rules, meta_title_preference, meta_description_preference,
    --             allowed_cta, ideal_references
    identity jsonb NOT NULL DEFAULT '{}'::jsonb,
    style jsonb NOT NULL DEFAULT '{}'::jsonb,
    seo jsonb NOT NULL DEFAULT '{}'::jsonb,
    -- Instruksi bebas klien; disimpan TERPISAH dari field terstruktur.
    instructions text,
    created_by character(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by character(36),
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);

CREATE INDEX IF NOT EXISTS profiler_profiles_brand_idx
    ON public.profiler_profiles (brand_id, updated_at DESC)
    WHERE deleted_at IS NULL;

-- Blok knowledge kustom: judul + jenis + isi teks + catatan (sederhana & fleksibel).
CREATE TABLE IF NOT EXISTS public.profiler_blocks (
    id character(36) NOT NULL PRIMARY KEY,
    profile_id character(36) NOT NULL,
    title character varying(200) NOT NULL,
    kind character varying(60),
    content text,
    note text,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE IF NOT EXISTS public.profiler_images (
    id character(36) NOT NULL PRIMARY KEY,
    profile_id character(36) NOT NULL,
    storage_key character varying(255) NOT NULL,
    url text NOT NULL,
    file_name character varying(255),
    mime_type character varying(120),
    size_bytes bigint,
    note text,
    created_by character(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);

CREATE TABLE IF NOT EXISTS public.profiler_documents (
    id character(36) NOT NULL PRIMARY KEY,
    profile_id character(36) NOT NULL,
    original_filename character varying(255) NOT NULL,
    storage_key character varying(255) NOT NULL,
    mime_type character varying(120),
    size_bytes bigint,
    checksum character varying(128),
    parse_status character varying(20) DEFAULT 'pending' NOT NULL,
    extracted_text text,
    parse_error text,
    created_by character(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_profiler_documents_parse_status CHECK (
        (parse_status)::text = ANY (ARRAY['pending'::text, 'parsing'::text, 'ready'::text, 'failed'::text])
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS profiler_documents_checksum_uidx
    ON public.profiler_documents (profile_id, checksum)
    WHERE deleted_at IS NULL AND checksum IS NOT NULL;

CREATE TABLE IF NOT EXISTS public.profiler_urls (
    id character(36) NOT NULL PRIMARY KEY,
    profile_id character(36) NOT NULL,
    url text NOT NULL,
    title character varying(255),
    note text,
    fetch_status character varying(20) DEFAULT 'pending' NOT NULL,
    extracted_text text,
    fetch_error text,
    last_synced_at timestamp with time zone,
    created_by character(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_profiler_urls_fetch_status CHECK (
        (fetch_status)::text = ANY (ARRAY['pending'::text, 'fetching'::text, 'ready'::text, 'failed'::text])
    )
);

-- FK (additive & toleran bila sudah ada).
DO $$
BEGIN
    ALTER TABLE public.profiler_blocks
        ADD CONSTRAINT profiler_blocks_profile_fk
        FOREIGN KEY (profile_id) REFERENCES public.profiler_profiles (id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE public.profiler_images
        ADD CONSTRAINT profiler_images_profile_fk
        FOREIGN KEY (profile_id) REFERENCES public.profiler_profiles (id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE public.profiler_documents
        ADD CONSTRAINT profiler_documents_profile_fk
        FOREIGN KEY (profile_id) REFERENCES public.profiler_profiles (id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE public.profiler_urls
        ADD CONSTRAINT profiler_urls_profile_fk
        FOREIGN KEY (profile_id) REFERENCES public.profiler_profiles (id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Kampanye menyimpan Profiler yang dipakai + SNAPSHOT konfigurasi saat kampanye
-- dibuat, supaya artikel konsisten walau Profiler diubah belakangan.
ALTER TABLE public.campaigns
    ADD COLUMN IF NOT EXISTS profiler_id character(36);
ALTER TABLE public.campaigns
    ADD COLUMN IF NOT EXISTS profiler_snapshot jsonb;

CREATE INDEX IF NOT EXISTS campaigns_profiler_idx
    ON public.campaigns (profiler_id);
