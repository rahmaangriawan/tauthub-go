-- 052: Content Type Editorial Standards & AI Writing Guidance (RAG)
-- Mengelola pedoman editorial per jenis konten (CSR, Event, Launching Produk, dll)
-- yang diindeks sebagai semantic RAG chunks untuk AI Engine Tulisio.

BEGIN;

CREATE TABLE IF NOT EXISTS public.content_type_standards (
    id character(36) NOT NULL,
    content_type_code character varying(50) NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT content_type_standards_pkey PRIMARY KEY (id),
    CONSTRAINT uq_content_type_standards_code UNIQUE (content_type_code),
    CONSTRAINT ck_content_type_standards_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'inactive'::character varying])::text[])))
);

CREATE TABLE IF NOT EXISTS public.content_type_standard_versions (
    id character(36) NOT NULL,
    standard_id character(36) NOT NULL,
    content_type_code character varying(50) NOT NULL,
    version integer NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    change_note character varying(255),
    content_purpose text,
    news_angle text,
    journalistic_style text,
    required_information text,
    optional_information text,
    structure_guidance text,
    lead_guidance text,
    fact_attribution_rules text,
    quote_guidance text,
    language_tone text,
    what_to_avoid text,
    angle_variation_rules text,
    source_priority text,
    fact_conflict_handling text,
    editorial_quality_checklist text,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT content_type_standard_versions_pkey PRIMARY KEY (id),
    CONSTRAINT uq_content_type_std_ver UNIQUE (standard_id, version),
    CONSTRAINT ck_content_type_std_ver_status CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'active'::character varying, 'archived'::character varying])::text[])))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_content_type_std_ver_active
    ON public.content_type_standard_versions (standard_id) WHERE (status = 'active'::character varying);

CREATE TABLE IF NOT EXISTS public.content_type_standard_chunks (
    id character(36) NOT NULL,
    version_id character(36) NOT NULL,
    content_type_code character varying(50) NOT NULL,
    section character varying(60) NOT NULL,
    chunk_index integer NOT NULL,
    content text NOT NULL,
    embedding jsonb NOT NULL,
    priority smallint DEFAULT 3 NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT content_type_standard_chunks_pkey PRIMARY KEY (id),
    CONSTRAINT uq_content_type_std_chunks UNIQUE (version_id, section, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_content_type_std_chunks_version ON public.content_type_standard_chunks (version_id);
CREATE INDEX IF NOT EXISTS idx_content_type_std_chunks_code ON public.content_type_standard_chunks (content_type_code);

ALTER TABLE ONLY public.content_type_standard_versions
    ADD CONSTRAINT fk_content_type_std_versions_standard FOREIGN KEY (standard_id) REFERENCES public.content_type_standards(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.content_type_standard_chunks
    ADD CONSTRAINT fk_content_type_std_chunks_version FOREIGN KEY (version_id) REFERENCES public.content_type_standard_versions(id) ON DELETE CASCADE;

COMMIT;
