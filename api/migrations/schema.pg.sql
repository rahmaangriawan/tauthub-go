-- ====================================================================

-- 1. CREATE ALL TABLES

-- ====================================================================

CREATE TABLE public.audit_logs (
    id character(36) NOT NULL,
    brand_id character(36),
    actor_user_id character(36),
    action character varying(100) NOT NULL,
    target_type character varying(50),
    target_id character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    ip_address character varying(45),
    user_agent character varying(500)
);

CREATE TABLE public.blogger_oauth_states (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    state character varying(200) NOT NULL,
    code_verifier character varying(200),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT blogger_oauth_states_pkey PRIMARY KEY (id),
    CONSTRAINT uq_blogger_oauth_states_state UNIQUE (state),
    CONSTRAINT fk_blogger_oauth_states_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE,
    CONSTRAINT fk_blogger_oauth_states_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE
);

CREATE TABLE public.brand_ai_quotas (
    brand_id character(36) NOT NULL,
    daily_call_limit integer NOT NULL,
    monthly_token_limit integer NOT NULL,
    plan_code character varying(40) DEFAULT 'starter'::character varying NOT NULL,
    monthly_article_limit integer DEFAULT 50 NOT NULL,
    per_batch_limit integer DEFAULT 50 NOT NULL,
    warning_percent integer DEFAULT 80 NOT NULL,
    generation_enabled boolean DEFAULT true NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL
);

CREATE TABLE public.brand_memberships (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    role character varying(20) DEFAULT 'writer'::character varying NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.brand_profiles (
    brand_id character(36) NOT NULL,
    positioning text,
    value_proposition text,
    target_audience text,
    products_services text,
    differentiation text,
    default_cta text,
    default_target_words integer DEFAULT 1000 NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL
);

CREATE TABLE public.pr_config (
    id text DEFAULT 'default'::text NOT NULL,
    rate_per_1k_reach integer DEFAULT 5000 NOT NULL,
    rate_tier1 integer,
    rate_tier2 integer,
    rate_tier3 integer,
    updated_by character(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_pr_config_id CHECK ((id = 'default'::text)),
    CONSTRAINT ck_pr_config_rate CHECK ((rate_per_1k_reach >= 0)),
    CONSTRAINT pr_config_pkey PRIMARY KEY (id)
);

CREATE TABLE public.brand_pr_standards (
    brand_id character(36) NOT NULL,
    rate_per_1k_reach integer,
    rate_tier1 integer,
    rate_tier2 integer,
    rate_tier3 integer,
    updated_by character(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_brand_pr_rates CHECK ((((rate_per_1k_reach IS NULL) OR (rate_per_1k_reach >= 0)) AND ((rate_tier1 IS NULL) OR (rate_tier1 >= 0)) AND ((rate_tier2 IS NULL) OR (rate_tier2 >= 0)) AND ((rate_tier3 IS NULL) OR (rate_tier3 >= 0)))),
    CONSTRAINT brand_pr_standards_pkey PRIMARY KEY (brand_id)
);

CREATE TABLE public.brands (
    id character(36) NOT NULL,
    name character varying(160) NOT NULL,
    slug character varying(160) NOT NULL,
    public_id character varying(180),
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    ownership_type character varying(20) DEFAULT 'internal'::character varying NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    client_name character varying(160),
    industry character varying(120),
    primary_language character varying(20) DEFAULT 'id-ID'::character varying NOT NULL,
    updated_at timestamp(6) with time zone,
    deleted_at timestamp(6) with time zone,
    CONSTRAINT ck_brands_ownership_type CHECK (((ownership_type)::text = ANY ((ARRAY['internal'::character varying, 'client'::character varying])::text[]))),
    CONSTRAINT ck_brands_status CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'active'::character varying, 'archived'::character varying])::text[])))
);

CREATE TABLE public.bug_reports (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    brand_id character(36),
    message text NOT NULL,
    context_info character varying(500),
    status character varying(20) DEFAULT 'open'::character varying NOT NULL,
    admin_notes text,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.chat_learnings (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    conversation_id character varying(100) NOT NULL,
    user_id character(36) NOT NULL,
    question text NOT NULL,
    question_embedding jsonb DEFAULT '[]'::jsonb NOT NULL,
    intent_tag character varying(50) DEFAULT 'general'::character varying NOT NULL,
    answer text NOT NULL,
    context_used jsonb DEFAULT '{}'::jsonb NOT NULL,
    benchmark_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.cms_connections (
    id character(36) NOT NULL,
    brand_id character(36),
    partner_id character(36),
    website_id character(36),
    provider character varying(30) NOT NULL,
    name character varying(120) NOT NULL,
    base_url character varying(500) NOT NULL,
    username character varying(160),
    sealed_secret text NOT NULL,
    status character varying(20) NOT NULL,
    last_test_status character varying(30),
    last_test_message character varying(500),
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_cms_connections_owner CHECK (((brand_id IS NOT NULL) <> (partner_id IS NOT NULL)))
);

CREATE TABLE public.content_batch_items (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    batch_id character(36) NOT NULL,
    row_number integer NOT NULL,
    fingerprint character varying(64) NOT NULL,
    brief_payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    status character varying(20) NOT NULL,
    job_id character(36),
    retry_count integer DEFAULT 0 NOT NULL,
    error_category character varying(80),
    error_message character varying(500),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.content_batches (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    catalog_id character(36),
    name character varying(200) NOT NULL,
    status character varying(20) NOT NULL,
    model_tier character varying(20) DEFAULT 'default'::character varying NOT NULL,
    concurrency_limit integer DEFAULT 3 NOT NULL,
    total_items integer DEFAULT 0 NOT NULL,
    estimated_tokens integer DEFAULT 0 NOT NULL,
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.content_briefs (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    website_id character(36),
    content_type character varying(50) NOT NULL,
    topic character varying(300) NOT NULL,
    angle text,
    goal text NOT NULL,
    audience text,
    search_intent character varying(80),
    primary_keyword character varying(200),
    secondary_keywords jsonb DEFAULT '[]'::jsonb NOT NULL,
    required_points jsonb DEFAULT '[]'::jsonb NOT NULL,
    required_content jsonb DEFAULT '[]'::jsonb NOT NULL,
    forbidden_content jsonb DEFAULT '[]'::jsonb NOT NULL,
    cta text,
    target_words integer DEFAULT 1000 NOT NULL,
    language character varying(20) DEFAULT 'id-ID'::character varying NOT NULL,
    require_outline_approval boolean DEFAULT false NOT NULL,
    include_table boolean DEFAULT false NOT NULL,
    include_list boolean DEFAULT false NOT NULL,
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.content_documents (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    job_id character(36) NOT NULL,
    status character varying(30) NOT NULL,
    title character varying(300) NOT NULL,
    slug character varying(300),
    meta_title character varying(255),
    meta_description character varying(500),
    assigned_reviewer_id character(36),
    current_version integer DEFAULT 1 NOT NULL,
    created_by character(36) NOT NULL,
    approved_by character(36),
    approved_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.content_jobs (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    brief_id character(36) NOT NULL,
    status character varying(40) NOT NULL,
    current_step character varying(50) NOT NULL,
    progress integer DEFAULT 0 NOT NULL,
    idempotency_key character varying(100) NOT NULL,
    model_tier character varying(20) DEFAULT 'default'::character varying NOT NULL,
    resolved_model character varying(200),
    config_snapshot jsonb DEFAULT '{}'::jsonb NOT NULL,
    prompt_snapshot jsonb DEFAULT '{}'::jsonb NOT NULL,
    source_snapshot jsonb DEFAULT '{}'::jsonb NOT NULL,
    outline text,
    outline_structure jsonb,
    draft_content text,
    draft_format character varying(16) DEFAULT 'markdown'::character varying NOT NULL,
    evaluation jsonb DEFAULT '{}'::jsonb NOT NULL,
    error_message character varying(500),
    cancel_requested boolean DEFAULT false NOT NULL,
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.content_versions (
    id character(36) NOT NULL,
    document_id character(36) NOT NULL,
    version integer NOT NULL,
    content text NOT NULL,
    content_format character varying(16) DEFAULT 'markdown'::character varying NOT NULL,
    attribution character varying(30) NOT NULL,
    change_note character varying(255),
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.editorial_rule_sets (
    brand_id character(36) NOT NULL,
    required_rules jsonb NOT NULL,
    forbidden_rules jsonb NOT NULL,
    required_disclaimer text,
    citation_policy text
);

CREATE TABLE public.jobs (
    id character(36) NOT NULL,
    kind character varying(50) NOT NULL,
    payload jsonb NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    max_attempts integer DEFAULT 3 NOT NULL,
    run_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    locked_at timestamp(6) with time zone,
    locked_by character varying(100),
    last_error character varying(1000),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.knowledge_chunks (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    collection_id character(36) NOT NULL,
    source_id character(36) NOT NULL,
    chunk_index integer NOT NULL,
    content text NOT NULL,
    token_estimate integer NOT NULL,
    embedding jsonb NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.knowledge_collections (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    name character varying(160) NOT NULL,
    description text,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_by character(36)
);

CREATE TABLE public.knowledge_crawl_runs (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    collection_id character(36) NOT NULL,
    sitemap_url character varying(2000) NOT NULL,
    max_pages integer DEFAULT 50 NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    total_urls integer,
    processed integer,
    failed_urls jsonb,
    error_message character varying(1000),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.knowledge_sources (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    collection_id character(36) NOT NULL,
    title character varying(255) NOT NULL,
    source_type character varying(50) DEFAULT 'manual'::character varying NOT NULL,
    original_filename character varying(255),
    origin_sitemap character varying(500),
    content text NOT NULL,
    checksum character(64) NOT NULL,
    category character varying(60) DEFAULT 'lainnya'::character varying,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    error_message character varying(1000),
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp(6) with time zone,
    storage_key character varying(500),
    valid_from timestamp(6) with time zone,
    valid_until timestamp(6) with time zone
);

CREATE TABLE public.master_prompt_versions (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    version integer NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    content text NOT NULL,
    change_note character varying(255),
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_master_prompt_versions_status CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'active'::character varying, 'archived'::character varying])::text[])))
);

CREATE TABLE public.knowledge_base_rule_chunks (
    id character(36) NOT NULL,
    rule_id character(36) NOT NULL,
    chunk_index integer NOT NULL,
    content text NOT NULL,
    embedding jsonb NOT NULL,
    CONSTRAINT knowledge_base_rule_chunks_pkey PRIMARY KEY (id),
    CONSTRAINT uq_kb_rule_chunks_rule_index UNIQUE (rule_id, chunk_index)
);

CREATE TABLE public.knowledge_base_rules (
    id character(36) NOT NULL,
    knowledge_base_id character(36) NOT NULL,
    version_id character(36) NOT NULL,
    category character varying(80) NOT NULL,
    title character varying(255) NOT NULL,
    instruction text NOT NULL,
    examples jsonb DEFAULT '[]'::jsonb NOT NULL,
    forbidden_examples jsonb DEFAULT '[]'::jsonb NOT NULL,
    tags jsonb DEFAULT '[]'::jsonb NOT NULL,
    audience jsonb DEFAULT '[]'::jsonb NOT NULL,
    content_types jsonb DEFAULT '[]'::jsonb NOT NULL,
    priority smallint DEFAULT 5 NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_knowledge_base_rules_priority CHECK ((priority BETWEEN 1 AND 8)),
    CONSTRAINT ck_knowledge_base_rules_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'inactive'::character varying])::text[]))),
    CONSTRAINT knowledge_base_rules_pkey PRIMARY KEY (id)
);

CREATE TABLE public.knowledge_base_sources (
    id character(36) NOT NULL,
    knowledge_base_id character(36) NOT NULL,
    source_name character varying(255) NOT NULL,
    source_url text,
    source_type character varying(30) DEFAULT 'regulation'::character varying NOT NULL,
    notes text,
    accessed_at date,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT knowledge_base_sources_pkey PRIMARY KEY (id)
);

CREATE TABLE public.knowledge_base_versions (
    id character(36) NOT NULL,
    knowledge_base_id character(36) NOT NULL,
    version integer NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    change_note character varying(255),
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_knowledge_base_versions_status CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'active'::character varying, 'archived'::character varying])::text[]))),
    CONSTRAINT knowledge_base_versions_pkey PRIMARY KEY (id),
    CONSTRAINT uq_knowledge_base_versions_version UNIQUE (knowledge_base_id, version)
);

CREATE TABLE public.knowledge_bases (
    id character(36) NOT NULL,
    code character varying(60) NOT NULL,
    name character varying(200) NOT NULL,
    description text,
    scope character varying(20) DEFAULT 'global'::character varying NOT NULL,
    scope_ref character varying(36),
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_knowledge_bases_scope CHECK (((scope)::text = ANY ((ARRAY['global'::character varying, 'media'::character varying, 'brand'::character varying])::text[]))),
    CONSTRAINT ck_knowledge_bases_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'inactive'::character varying])::text[]))),
    CONSTRAINT knowledge_bases_pkey PRIMARY KEY (id),
    CONSTRAINT uq_knowledge_bases_code UNIQUE (code)
);

CREATE TABLE public.model_usage_events (
    id character(36) NOT NULL,
    brand_id character(36),
    user_id character(36),
    task_type character varying(50) NOT NULL,
    provider character varying(50) NOT NULL,
    model character varying(160) NOT NULL,
    request_id character varying(160),
    input_tokens integer DEFAULT 0 NOT NULL,
    output_tokens integer DEFAULT 0 NOT NULL,
    total_tokens integer DEFAULT 0 NOT NULL,
    latency_ms integer NOT NULL,
    success boolean NOT NULL,
    error_category character varying(80),
    cache_hit boolean DEFAULT false NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.platform_admins (
    user_id character(36) NOT NULL,
    role character varying(20) DEFAULT 'superadmin'::character varying NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.platform_ai_configuration (
    id character(36) DEFAULT 'default'::bpchar NOT NULL,
    base_url character varying(500) NOT NULL,
    sealed_api_key text NOT NULL,
    api_key_hint character varying(12) NOT NULL,
    default_model character varying(200),
    economy_model character varying(200),
    premium_model character varying(200),
    updated_by character(36) NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.publication_jobs (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    document_id character(36) NOT NULL,
    connection_id character(36) NOT NULL,
    publication_id character(36),
    status character varying(20) NOT NULL,
    publish_status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    payload_snapshot jsonb DEFAULT '{}'::jsonb NOT NULL,
    remote_post_id character varying(120),
    remote_url character varying(500),
    error_message character varying(500),
    error_code character varying(80),
    attempts integer DEFAULT 0 NOT NULL,
    progress integer DEFAULT 0 NOT NULL,
    started_at timestamp(6) with time zone,
    completed_at timestamp(6) with time zone,
    next_retry_at timestamp(6) with time zone,
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.quality_evaluations (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    document_id character(36) NOT NULL,
    version integer NOT NULL,
    rubric_version character varying(20) NOT NULL,
    score integer NOT NULL,
    status character varying(20) NOT NULL,
    checks jsonb DEFAULT '{}'::jsonb NOT NULL,
    summary jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.review_comments (
    id character(36) NOT NULL,
    document_id character(36) NOT NULL,
    version integer NOT NULL,
    body text NOT NULL,
    selection_quote character varying(1000),
    line_start integer,
    line_end integer,
    resolved boolean DEFAULT false NOT NULL,
    created_by character(36) NOT NULL,
    resolved_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.system_metadata (
    key character varying(100) NOT NULL,
    value character varying(500) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.user_daily_quotas (
    user_id character(36) NOT NULL,
    daily_token_limit integer DEFAULT 0 NOT NULL,
    updated_by character(36) NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL
);

CREATE TABLE public.user_sessions (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    refresh_jti character(36) NOT NULL,
    expires_at timestamp(6) with time zone NOT NULL,
    revoked boolean DEFAULT false NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.users (
    id character(36) NOT NULL,
    email character varying(320) NOT NULL,
    password_hash character varying(255) NOT NULL,
    display_name character varying(120) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    approved_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.voice_profiles (
    brand_id character(36) NOT NULL,
    personality jsonb NOT NULL,
    formality integer DEFAULT 3 NOT NULL,
    enthusiasm integer DEFAULT 3 NOT NULL,
    directness integer DEFAULT 3 NOT NULL,
    humor integer DEFAULT 1 NOT NULL,
    point_of_view character varying(40) DEFAULT 'second_person'::character varying NOT NULL,
    preferred_terms jsonb NOT NULL,
    banned_terms jsonb NOT NULL,
    cta_style text
);

CREATE TABLE public.websites (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    domain character varying(253) NOT NULL,
    label character varying(120),
    language character varying(20) DEFAULT 'id-ID'::character varying NOT NULL,
    is_primary boolean DEFAULT false NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL
);

CREATE TABLE public.media (
    id character(36) NOT NULL,
    name character varying(160) NOT NULL,
    slug character varying(160),
    category character varying(120),
    description text,
    website_url character varying(500),
    price numeric(14,2) DEFAULT 0 NOT NULL,
    credit_cost integer DEFAULT 1 NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    integration_type character varying(20) DEFAULT 'manual'::character varying NOT NULL,
    partner_id character(36),
    publish_defaults jsonb,
    notes text,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    traffic_estimate bigint DEFAULT 0 NOT NULL,
    health_score smallint,
    health_breakdown jsonb,
    health_computed_at timestamp(6) with time zone,
    dewan_pers_verified boolean NOT NULL DEFAULT false,
    dewan_pers_note text,
    dewan_pers_cert_no character varying(200),
    dewan_pers_doc_key character varying(500),
    publication_mode character varying(20) NOT NULL DEFAULT 'draft'::character varying,
    rejection_reason character varying(1000),
    reviewed_by character(36),
    reviewed_at timestamp(6) with time zone,
    pricing_type character varying(10) NOT NULL DEFAULT 'nett'::character varying,
    price_status character varying(20) NOT NULL DEFAULT 'menunggu_review'::character varying,
    negotiated_price numeric(14,2),
    custom_web_endpoint_url character varying(500),
    custom_web_api_key_sealed text,
    custom_web_status character varying(20) DEFAULT 'disconnected'::character varying NOT NULL,
    custom_web_last_test_at timestamp(6) with time zone,
    custom_web_last_test_message character varying(500),
    blogger_credentials_sealed text,
    blogger_blog_id character varying(200),
    blogger_blog_name character varying(300),
    blogger_status character varying(20) DEFAULT 'disconnected'::character varying NOT NULL,
    blogger_last_test_at timestamp(6) with time zone,
    blogger_last_test_message character varying(500),
    ai_summary text,
    ai_summary_generated_at timestamp(6) with time zone,
    bahasa_artikel text,
    label_sponsor character varying(20) DEFAULT 'tidak_ada'::character varying NOT NULL,
    topik_terlarang text,
    contoh_artikel_url character varying(500),
    CONSTRAINT ck_media_status CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'active'::character varying, 'inactive'::character varying, 'archived'::character varying, 'hidden'::character varying, 'pending_review'::character varying, 'rejected'::character varying])::text[]))),
    CONSTRAINT ck_media_credit_cost CHECK (credit_cost >= 0),
    CONSTRAINT ck_media_integration_type CHECK (((integration_type)::text = ANY ((ARRAY['manual'::character varying, 'wordpress'::character varying, 'rss'::character varying, 'webhook'::character varying, 'custom_web'::character varying, 'blogger'::character varying])::text[]))),
    CONSTRAINT ck_media_publication_mode CHECK (((publication_mode)::text = ANY ((ARRAY['draft'::character varying, 'publish'::character varying])::text[]))),
    CONSTRAINT ck_media_label_sponsor CHECK (((label_sponsor)::text = ANY ((ARRAY['tidak_ada'::character varying, 'ada_sponsor'::character varying])::text[]))),
    CONSTRAINT ck_media_pricing_type CHECK (((pricing_type)::text = ANY ((ARRAY['nett'::character varying, 'nego'::character varying])::text[]))),
    CONSTRAINT ck_media_price_status CHECK (((price_status)::text = ANY ((ARRAY['menunggu_review'::character varying, 'disetujui'::character varying, 'negosiasi'::character varying, 'ditolak'::character varying, 'final'::character varying])::text[])))
);

CREATE TABLE public.media_packages (
    id character(36) NOT NULL,
    name character varying(120) NOT NULL,
    slug character varying(140) NOT NULL,
    description text,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT media_packages_pkey PRIMARY KEY (id),
    CONSTRAINT ck_media_package_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'inactive'::character varying])::text[])))
);

CREATE TABLE public.media_package_items (
    package_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    position integer DEFAULT 0 NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT media_package_items_pkey PRIMARY KEY (package_id, media_id)
);

CREATE TABLE public.partners (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    name character varying(160),
    public_id character varying(180),
    category character varying(60) DEFAULT 'lainnya'::character varying,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    rss_enabled boolean DEFAULT false NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_partner_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'inactive'::character varying])::text[])))
);

CREATE TABLE public.publications (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    document_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    client_price numeric(14,2) DEFAULT 0 NOT NULL,
    partner_price numeric(14,2),
    markup_percent numeric(5,2),
    publish_status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    connection_id character(36),
    scheduled_at timestamp(6) with time zone,
    published_at timestamp(6) with time zone,
    published_url character varying(500),
    featured_image_url character varying(500),
    notes character varying(1000),
    cancel_reason character varying(500),
    rejection_reason character varying(500),
    reviewed_by character(36),
    reviewed_at timestamp(6) with time zone,
    campaign_id character(36),
    replaces_id character(36),
    rss_import_status character varying(20) DEFAULT 'not_imported'::character varying NOT NULL,
    rss_imported_at timestamp(6) with time zone,
    rss_published_at timestamp(6) with time zone,
    partner_draft_edits jsonb,
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_publication_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'waiting_partner'::character varying, 'partner_approved'::character varying, 'partner_rejected'::character varying, 'queued'::character varying, 'processing'::character varying, 'published'::character varying, 'failed'::character varying, 'retrying'::character varying, 'cancelled'::character varying])::text[]))),
    CONSTRAINT ck_publication_publish_status CHECK (((publish_status)::text = ANY ((ARRAY['draft'::character varying, 'publish'::character varying])::text[])))
);

CREATE TABLE public.auto_replace_configs (
    id character(36) NOT NULL,
    brand_id character(36),
    enabled boolean NOT NULL DEFAULT TRUE,
    max_replaces smallint NOT NULL DEFAULT 3,
    created_by character(36),
    updated_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT auto_replace_configs_pkey PRIMARY KEY (id),
    CONSTRAINT ck_auto_replace_max_replaces CHECK ((max_replaces >= 1) AND (max_replaces <= 10))
);

CREATE TABLE public.publication_attempts (
    id character(36) NOT NULL,
    publication_id character(36) NOT NULL,
    attempt_number integer NOT NULL,
    status character varying(20) NOT NULL,
    error_code character varying(80),
    error_message character varying(500),
    started_at timestamp(6) with time zone,
    completed_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.rss_feeds (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    media_id character(36),
    name character varying(160) NOT NULL,
    slug character varying(160),
    token character(64) NOT NULL,
    category character varying(60) DEFAULT 'lainnya'::character varying,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    description text,
    item_status character varying(20) DEFAULT 'published'::character varying NOT NULL,
    install_status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    install_error text,
    installed_at timestamp(6) with time zone,
    verified_by character(36),
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_rss_feed_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'revoked'::character varying])::text[]))),
    CONSTRAINT ck_rss_feed_item_status CHECK (((item_status)::text = ANY ((ARRAY['published'::character varying, 'draft'::character varying])::text[])))
);

CREATE TABLE public.rss_access_logs (
    id character(36) NOT NULL,
    feed_id character(36) NOT NULL,
    ip_address character varying(64),
    user_agent character varying(300),
    status smallint,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE public.notification_webhooks (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    url character varying(500) NOT NULL,
    secret character varying(200),
    active boolean DEFAULT true NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT notification_webhooks_pkey PRIMARY KEY (id),
    CONSTRAINT ck_notification_webhooks_url CHECK (((url)::text ~* '^https?://'::text))
);

CREATE TABLE public.notifications (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    type character varying(40) NOT NULL,
    title character varying(200) NOT NULL,
    body character varying(1000),
    link character varying(500),
    publication_id character(36),
    dedupe_key character varying(200),
    read_at timestamp(6) with time zone,
    expired_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT notifications_pkey PRIMARY KEY (id),
    CONSTRAINT ck_notifications_type CHECK (((type)::text = ANY ((ARRAY[
        'publication.waiting_partner'::character varying,
        'publication.partner_approved'::character varying,
        'publication.partner_rejected'::character varying,
        'publication.auto_replaced'::character varying,
        'publication.published'::character varying,
        'publication.failed'::character varying,
        'publication.url_requested'::character varying,
        'publication.reminder'::character varying,
        'variant_batch.failed'::character varying,
        'pr.score_updated'::character varying,
        'pr.verification_completed'::character varying,
        'media.submitted'::character varying,
        'media.approved'::character varying,
        'media.rejected'::character varying,
        'media.resubmitted'::character varying,
        'media.negotiated'::character varying,
        'withdrawal.requested'::character varying,
        'withdrawal.processing'::character varying,
        'withdrawal.paid'::character varying,
        'withdrawal.rejected'::character varying
    ])::text[])))
);

CREATE TABLE public.campaigns (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    name character varying(160) NOT NULL,
    description text,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    article_quota integer DEFAULT 0 NOT NULL,
    budget numeric(14,2) DEFAULT 0 NOT NULL,
    starts_at timestamp(6) with time zone,
    ends_at timestamp(6) with time zone,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_campaign_status CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'active'::character varying, 'paused'::character varying, 'completed'::character varying, 'cancelled'::character varying])::text[]))),
    CONSTRAINT ck_campaign_quota CHECK (article_quota >= 0),
    CONSTRAINT ck_campaign_budget CHECK (budget >= 0)
);

CREATE TABLE public.campaign_media_items (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    source_document_id character(36),
    variant_document_id character(36),
    variant_type character varying(20) DEFAULT 'original'::character varying NOT NULL,
    angle_label character varying(200),
    thumbnail_url character varying(500),
    tc_cost integer DEFAULT 0 NOT NULL,
    status character varying(30) DEFAULT 'pending'::character varying NOT NULL,
    validation jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT campaign_media_items_pkey PRIMARY KEY (id),
    CONSTRAINT uq_campaign_media UNIQUE (campaign_id, media_id),
    CONSTRAINT ck_campaign_media_variant_type CHECK (((variant_type)::text = ANY ((ARRAY['original'::character varying, 'paraphrase'::character varying, 'angle'::character varying])::text[]))),
    CONSTRAINT ck_campaign_media_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'article_ready'::character varying, 'needs_rewrite'::character varying, 'rewriting'::character varying, 'angle_select'::character varying, 'ready'::character varying, 'processing'::character varying, 'published'::character varying, 'failed'::character varying, 'cancelled'::character varying])::text[])))
);

CREATE TABLE public.campaign_report_shares (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    token character(64) NOT NULL,
    created_by character(36),
    revoked_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT campaign_report_shares_pkey PRIMARY KEY (id),
    CONSTRAINT uq_campaign_report_token UNIQUE (token),
    CONSTRAINT uq_campaign_report_campaign UNIQUE (campaign_id)
);

CREATE TABLE public.packages (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    name character varying(160) NOT NULL,
    credit_total integer NOT NULL,
    credit_used integer DEFAULT 0 NOT NULL,
    price numeric(14,2) DEFAULT 0 NOT NULL,
    category character varying(60) DEFAULT 'lainnya'::character varying,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    starts_at timestamp(6) with time zone,
    ends_at timestamp(6) with time zone,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_package_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'exhausted'::character varying, 'revoked'::character varying])::text[]))),
    CONSTRAINT ck_package_credit_total CHECK (credit_total > 0),
    CONSTRAINT ck_package_credit_used CHECK (credit_used >= 0)
);

CREATE TABLE public.credit_transactions (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    package_id character(36),
    publication_id character(36),
    amount integer NOT NULL,
    reason character varying(50) NOT NULL,
    balance_after integer NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_credit_tx_reason CHECK (((reason)::text = ANY ((ARRAY['package.grant'::character varying, 'publication.usage'::character varying, 'publication.refund'::character varying, 'adjustment'::character varying])::text[]))),
    CONSTRAINT ck_credit_tx_amount CHECK (amount <> 0)
);

CREATE TABLE public.revenue_shares (
    id character(36) NOT NULL,
    partner_id character(36),
    media_id character(36),
    name character varying(160) NOT NULL,
    share_type character varying(20) DEFAULT 'percentage'::character varying NOT NULL,
    tulisio_percent numeric(5,2) DEFAULT 75 NOT NULL,
    partner_percent numeric(5,2) DEFAULT 25 NOT NULL,
    partner_fixed numeric(14,2) DEFAULT 0 NOT NULL,
    category character varying(60) DEFAULT 'lainnya'::character varying,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_revenue_share_type CHECK (((share_type)::text = ANY ((ARRAY['percentage'::character varying, 'fixed'::character varying])::text[]))),
    CONSTRAINT ck_revenue_share_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'archived'::character varying])::text[]))),
    CONSTRAINT ck_revenue_share_percent CHECK ((partner_percent >= 0 AND partner_percent <= 100 AND tulisio_percent >= 0 AND tulisio_percent <= 100)),
    CONSTRAINT ck_revenue_share_fixed CHECK (partner_fixed >= 0)
);

CREATE TABLE public.partner_earnings (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    publication_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    rule_id character(36),
    amount numeric(14,2) NOT NULL,
    gross_amount numeric(14,2),
    client_price numeric(14,2) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    settlement_id character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT uq_partner_earnings_publication UNIQUE (publication_id),
    CONSTRAINT ck_partner_earnings_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'settled'::character varying, 'voided'::character varying])::text[]))),
    CONSTRAINT ck_partner_earnings_amount CHECK (amount >= 0)
);

CREATE TABLE public.settlements (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    period_start date NOT NULL,
    period_end date NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    total_amount numeric(14,2) DEFAULT 0 NOT NULL,
    notes text,
    rejection_reason character varying(1000),
    payment_reference character varying(200),
    paid_at timestamp(6) with time zone,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_settlement_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'approved'::character varying, 'processing'::character varying, 'paid'::character varying, 'failed'::character varying])::text[]))),
    CONSTRAINT ck_settlement_amount CHECK (total_amount >= 0),
    CONSTRAINT ck_settlement_period CHECK (period_end >= period_start)
);

CREATE TABLE public.transactions (
    id character(36) NOT NULL,
    brand_id character(36),
    partner_id character(36),
    publication_id character(36),
    campaign_id character(36),
    amount numeric(14,2) NOT NULL,
    type character varying(30) NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_transaction_type CHECK (((type)::text = ANY ((ARRAY['publication.revenue'::character varying, 'publication.refund'::character varying, 'settlement.payout'::character varying, 'adjustment'::character varying])::text[]))),
    CONSTRAINT ck_transaction_amount CHECK (amount <> 0)
);

CREATE TABLE public.invoices (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    campaign_id character(36),
    number character varying(40) NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    amount numeric(14,2) NOT NULL,
    issued_at timestamp(6) with time zone,
    due_at timestamp(6) with time zone,
    paid_at timestamp(6) with time zone,
    notes text,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT uq_invoice_number UNIQUE (number),
    CONSTRAINT ck_invoice_status CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'issued'::character varying, 'paid'::character varying, 'void'::character varying])::text[]))),
    CONSTRAINT ck_invoice_amount CHECK (amount >= 0)
);

CREATE TABLE public.tracking_links (
    id character(36) NOT NULL,
    code character varying(24) NOT NULL,
    publication_id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    destination_url character varying(2000),
    category character varying(60) DEFAULT 'lainnya'::character varying,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT uq_tracking_link_code UNIQUE (code),
    CONSTRAINT uq_tracking_link_publication UNIQUE (publication_id),
    CONSTRAINT ck_tracking_link_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'disabled'::character varying])::text[])))
);

CREATE TABLE public.tracking_events (
    id character(36) NOT NULL,
    link_id character(36),
    publication_id character(36),
    campaign_id character(36),
    brand_id character(36) NOT NULL,
    event_type character varying(30) NOT NULL,
    session_id character varying(80),
    ip_address character varying(64),
    user_agent character varying(300),
    referrer character varying(1000),
    url character varying(1000),
    metadata jsonb,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_tracking_event_type CHECK (((event_type)::text = ANY ((ARRAY['article_view'::character varying, 'article_scroll'::character varying, 'outbound_click'::character varying, 'cta_click'::character varying, 'tracking_link_click'::character varying])::text[])))
);

CREATE TABLE public.analytics_daily (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    publication_id character(36),
    campaign_id character(36),
    event_type character varying(30) NOT NULL,
    event_date date NOT NULL,
    count bigint DEFAULT 0 NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_analytics_daily_event_type CHECK (((event_type)::text = ANY ((ARRAY['article_view'::character varying, 'article_scroll'::character varying, 'outbound_click'::character varying, 'cta_click'::character varying, 'tracking_link_click'::character varying])::text[])))
);

CREATE TABLE public.ga4_configs (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    measurement_id character varying(40) NOT NULL,
    api_secret character varying(64),
    event_mapping jsonb,
    enabled boolean DEFAULT false NOT NULL,
    consent_mode character varying(20) DEFAULT 'required'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT uq_ga4_config_brand UNIQUE (brand_id),
    CONSTRAINT ck_ga4_consent_mode CHECK (((consent_mode)::text = ANY ((ARRAY['required'::character varying, 'none'::character varying])::text[]))),
    CONSTRAINT ck_ga4_measurement_id CHECK ((measurement_id ~ '^G-[A-Z0-9]+$'::text))
);

CREATE TABLE public.payment_gateway_configs (
    id character(36) NOT NULL,
    provider character varying(20) NOT NULL,
    environment character varying(10) NOT NULL DEFAULT 'sandbox',
    encrypted_credentials text NOT NULL,
    is_configured boolean NOT NULL DEFAULT FALSE,
    is_active boolean NOT NULL DEFAULT FALSE,
    last_tested_at timestamp(6) with time zone,
    last_test_ok boolean,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pk_payment_gateway_configs PRIMARY KEY (id),
    CONSTRAINT uq_payment_gateway_provider UNIQUE (provider),
    CONSTRAINT ck_payment_gateway_provider CHECK (((provider)::text = ANY ((ARRAY['xendit'::character varying, 'duitku'::character varying, 'midtrans'::character varying, 'tripay'::character varying, 'sandbox'::character varying, 'sumopod'::character varying])::text[]))),
    CONSTRAINT ck_payment_gateway_environment CHECK (((environment)::text = ANY ((ARRAY['sandbox'::character varying, 'production'::character varying])::text[])))
);

CREATE TABLE public.payment_transactions (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    brand_id character(36),
    invoice_id character(36),
    idempotency_key character varying(100),
    gateway_provider character varying(20) NOT NULL,
    gateway_config_id character(36),
    gateway_environment character varying(10) NOT NULL,
    external_transaction_id character varying(120),
    amount numeric(14,2) NOT NULL,
    currency character varying(8) NOT NULL DEFAULT 'IDR',
    status character varying(20) NOT NULL DEFAULT 'pending',
    payment_method character varying(60),
    payment_url character varying(2000),
    purpose character varying(20) NOT NULL DEFAULT 'generic',
    purpose_ref_id character(36),
    invoice_data jsonb,
    paid_at timestamp(6) with time zone,
    expired_at timestamp(6) with time zone,
    raw_callback jsonb,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pk_payment_transactions PRIMARY KEY (id),
    CONSTRAINT ck_payment_transaction_provider CHECK (((gateway_provider)::text = ANY ((ARRAY['xendit'::character varying, 'duitku'::character varying, 'midtrans'::character varying, 'tripay'::character varying, 'sandbox'::character varying, 'sumopod'::character varying])::text[]))),
    CONSTRAINT ck_payment_transaction_environment CHECK (((gateway_environment)::text = ANY ((ARRAY['sandbox'::character varying, 'production'::character varying])::text[]))),
    CONSTRAINT ck_payment_transaction_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'processing'::character varying, 'paid'::character varying, 'failed'::character varying, 'expired'::character varying, 'cancelled'::character varying, 'refunded'::character varying])::text[]))),
    CONSTRAINT ck_payment_transaction_amount CHECK (amount > 0)
);

CREATE TABLE public.pricing_config (
    id integer NOT NULL DEFAULT 1,
    markup_percent numeric(5,2) NOT NULL DEFAULT 15,
    updated_by character(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_pricing_config_singleton CHECK (id = 1),
    CONSTRAINT ck_pricing_config_markup CHECK (markup_percent >= 0),
    CONSTRAINT pricing_config_pkey PRIMARY KEY (id)
);

INSERT INTO public.pricing_config (id, markup_percent) VALUES (1, 15) ON CONFLICT (id) DO NOTHING;

CREATE TABLE public.tc_config (
    id integer NOT NULL DEFAULT 1,
    rate_idr integer NOT NULL DEFAULT 1000,
    topup_packages jsonb NOT NULL DEFAULT '[{"tc": 500, "idr": 500000}, {"tc": 1000, "idr": 1000000}, {"tc": 5000, "idr": 5000000}]'::jsonb,
    minimum_withdraw_idr integer NOT NULL DEFAULT 10000,
    updated_by character(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_tc_config_id CHECK (id = 1),
    CONSTRAINT ck_tc_config_rate CHECK (rate_idr > 0),
    CONSTRAINT ck_tc_config_min_withdraw CHECK (minimum_withdraw_idr > 0),
    CONSTRAINT tc_config_pkey PRIMARY KEY (id)
);

CREATE TABLE public.credit_wallets (
    brand_id character(36) NOT NULL,
    available_tc integer NOT NULL DEFAULT 0,
    reserved_tc integer NOT NULL DEFAULT 0,
    captured_tc integer NOT NULL DEFAULT 0,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_wallet_available CHECK (available_tc >= 0),
    CONSTRAINT ck_wallet_reserved CHECK (reserved_tc >= 0),
    CONSTRAINT ck_wallet_captured CHECK (captured_tc >= 0),
    CONSTRAINT credit_wallets_pkey PRIMARY KEY (brand_id)
);

CREATE TABLE public.credit_ledger_entries (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    ref_type character varying(20) NOT NULL,
    ref_id character(36),
    amount_tc integer NOT NULL,
    balance_after_tc integer NOT NULL,
    rate_idr integer NOT NULL,
    amount_idr integer,
    reason text,
    idempotency_key character varying(160) NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_credit_ledger_type CHECK (((ref_type)::text = ANY ((ARRAY['topup'::character varying, 'reserve'::character varying, 'release'::character varying, 'capture'::character varying, 'refund'::character varying, 'adjustment'::character varying])::text[]))),
    CONSTRAINT credit_ledger_entries_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS public.content_intake_types (
    id character(36) NOT NULL,
    code character varying(50) NOT NULL,
    name character varying(120) NOT NULL,
    description text,
    questions jsonb DEFAULT '[]'::jsonb NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT content_intake_types_pkey PRIMARY KEY (id),
    CONSTRAINT uq_content_intake_type_code UNIQUE (code),
    CONSTRAINT ck_content_intake_type_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'archived'::character varying])::text[])))
);

CREATE TABLE IF NOT EXISTS public.content_intakes (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    mode character varying(20) NOT NULL,
    content_type_code character varying(50),
    topic character varying(300),
    answers jsonb DEFAULT '{}'::jsonb NOT NULL,
    article_text text,
    article_filename character varying(255),
    readiness_score integer,
    readiness_checks jsonb DEFAULT '[]'::jsonb NOT NULL,
    recommendation jsonb DEFAULT '{}'::jsonb NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    job_id character(36),
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT content_intakes_pkey PRIMARY KEY (id),
    CONSTRAINT ck_content_intake_mode CHECK (((mode)::text = ANY ((ARRAY['bahan'::character varying, 'artikel'::character varying, 'keduanya'::character varying, 'rekomendasi'::character varying])::text[]))),
    CONSTRAINT ck_content_intake_status CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'ready'::character varying, 'job_created'::character varying])::text[])))
);

CREATE TABLE IF NOT EXISTS public.media_content_standards (
    id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    min_words integer DEFAULT 400 NOT NULL,
    max_words integer DEFAULT 1500 NOT NULL,
    journalistic_style character varying(20) DEFAULT 'news'::character varying NOT NULL,
    hard_selling_tolerance character varying(10) DEFAULT 'low'::character varying NOT NULL,
    quote_required boolean DEFAULT false NOT NULL,
    min_quotes integer DEFAULT 0 NOT NULL,
    image_required boolean DEFAULT false NOT NULL,
    category_required boolean DEFAULT false NOT NULL,
    required_info jsonb DEFAULT '[]'::jsonb NOT NULL,
    forbidden_content jsonb DEFAULT '[]'::jsonb NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT media_content_standards_pkey PRIMARY KEY (id),
    CONSTRAINT uq_media_content_standard_media UNIQUE (media_id),
    CONSTRAINT ck_media_standard_style CHECK (((journalistic_style)::text = ANY ((ARRAY['news'::character varying, 'feature'::character varying, 'magazine'::character varying, 'blog'::character varying])::text[]))),
    CONSTRAINT ck_media_standard_tolerance CHECK (((hard_selling_tolerance)::text = ANY ((ARRAY['none'::character varying, 'low'::character varying, 'medium'::character varying, 'high'::character varying])::text[]))),
    CONSTRAINT ck_media_standard_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'archived'::character varying])::text[]))),
    CONSTRAINT ck_media_standard_words CHECK ((min_words >= 0 AND max_words >= min_words)),
    CONSTRAINT ck_media_standard_quotes CHECK (min_quotes >= 0)
);

CREATE TABLE IF NOT EXISTS public.publishing_api_keys (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    media_id character(36),
    label character varying(120) DEFAULT 'Default'::character varying NOT NULL,
    api_key_hash character(64) NOT NULL,
    api_key_sealed text NOT NULL,
    secret_sealed text NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    last_used_at timestamp(6) with time zone,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT publishing_api_keys_pkey PRIMARY KEY (id),
    CONSTRAINT uq_publishing_api_key_hash UNIQUE (api_key_hash),
    CONSTRAINT uq_publishing_api_key_partner_label UNIQUE (partner_id, label),
    CONSTRAINT ck_publishing_api_key_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'revoked'::character varying])::text[])))
);

CREATE TABLE IF NOT EXISTS public.publishing_callback_logs (
    id character(36) NOT NULL,
    api_key_id character(36),
    partner_id character(36),
    publication_id character(36),
    ip_address character varying(64),
    user_agent character varying(300),
    status_code integer,
    accepted boolean,
    error_message character varying(500),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT publishing_callback_logs_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS public.publication_variant_batches (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    mode character varying(20) NOT NULL,
    seed integer NOT NULL,
    status character varying(20) DEFAULT 'queued'::character varying NOT NULL,
    source_document_id character(36),
    source_material text,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    completed_at timestamp(6) with time zone,
    CONSTRAINT publication_variant_batches_pkey PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS public.publication_variants (
    id character(36) NOT NULL,
    batch_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    angle_id character varying(60) NOT NULL,
    angle_label character varying(120) NOT NULL,
    status character varying(20) DEFAULT 'queued'::character varying NOT NULL,
    document_id character(36),
    publication_id character(36),
    error_message character varying(1000),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT publication_variants_pkey PRIMARY KEY (id),
    CONSTRAINT uq_variant_batch_media UNIQUE (batch_id, media_id)
);

CREATE TABLE IF NOT EXISTS public.pr_scoring_profiles (
    id character(36) NOT NULL,
    version integer NOT NULL,
    name character varying(120) NOT NULL,
    weights jsonb NOT NULL,
    thresholds jsonb NOT NULL DEFAULT '{}'::jsonb,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pr_scoring_profiles_pkey PRIMARY KEY (id),
    CONSTRAINT uq_pr_scoring_profile_version UNIQUE (version),
    CONSTRAINT ck_pr_scoring_profile_status CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'active'::character varying, 'archived'::character varying])::text[])))
);

CREATE TABLE IF NOT EXISTS public.pr_campaign_scores (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    profile_id character(36) NOT NULL,
    profile_version integer NOT NULL,
    weights jsonb NOT NULL,
    status character varying(20) DEFAULT 'not_ready'::character varying NOT NULL,
    score numeric(5,2),
    label character varying(40),
    breakdown jsonb NOT NULL DEFAULT '{}'::jsonb,
    input_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    calculated_by character(36),
    calculated_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pr_campaign_scores_pkey PRIMARY KEY (id),
    CONSTRAINT uq_pr_campaign_profile UNIQUE (campaign_id, profile_version),
    CONSTRAINT ck_pr_campaign_score_status CHECK (((status)::text = ANY ((ARRAY['not_ready'::character varying, 'calculating'::character varying, 'partial'::character varying, 'ready'::character varying, 'final'::character varying, 'failed'::character varying])::text[])))
);

CREATE TABLE IF NOT EXISTS public.pr_metric_results (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    metric_key character varying(60) NOT NULL,
    metric_value numeric NOT NULL,
    source character varying(120),
    calculated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pr_metric_results_pkey PRIMARY KEY (id),
    CONSTRAINT uq_pr_metric_campaign_key UNIQUE (campaign_id, metric_key)
);

CREATE TABLE IF NOT EXISTS public.pr_message_analyses (
    id character(36) NOT NULL,
    publication_id character(36) NOT NULL,
    campaign_id character(36),
    key_messages jsonb NOT NULL DEFAULT '[]'::jsonb,
    results jsonb NOT NULL DEFAULT '[]'::jsonb,
    consistency_score numeric(5,2),
    analysis_version character varying(60) NOT NULL,
    status character varying(20) DEFAULT 'completed'::character varying NOT NULL,
    error_message character varying(500),
    analyzed_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pr_message_analyses_pkey PRIMARY KEY (id),
    CONSTRAINT uq_pr_message_analysis_publication UNIQUE (publication_id),
    CONSTRAINT ck_pr_message_analysis_status CHECK (((status)::text = ANY ((ARRAY['queued'::character varying, 'running'::character varying, 'completed'::character varying, 'failed'::character varying])::text[])))
);

CREATE TABLE IF NOT EXISTS public.pr_sentiment_analyses (
    id character(36) NOT NULL,
    publication_id character(36) NOT NULL,
    campaign_id character(36),
    sentiment character varying(20),
    confidence numeric(5,2),
    analysis_version character varying(60) NOT NULL,
    status character varying(20) DEFAULT 'completed'::character varying NOT NULL,
    error_message character varying(500),
    analyzed_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pr_sentiment_analyses_pkey PRIMARY KEY (id),
    CONSTRAINT uq_pr_sentiment_analysis_publication UNIQUE (publication_id),
    CONSTRAINT ck_pr_sentiment_analysis_status CHECK (((status)::text = ANY ((ARRAY['queued'::character varying, 'running'::character varying, 'completed'::character varying, 'failed'::character varying])::text[]))),
    CONSTRAINT ck_pr_sentiment_value CHECK ((sentiment IS NULL OR sentiment IN ('positive','neutral','negative')))
);

CREATE TABLE IF NOT EXISTS public.pr_publication_verifications (
    id character(36) NOT NULL,
    publication_id character(36) NOT NULL,
    campaign_id character(36),
    url character varying(500),
    status character varying(20) NOT NULL DEFAULT 'unavailable',
    compliance_score numeric(5,2),
    checks jsonb NOT NULL DEFAULT '[]'::jsonb,
    error_message character varying(500),
    verified_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pr_publication_verifications_pkey PRIMARY KEY (id),
    CONSTRAINT uq_pr_verification_publication UNIQUE (publication_id),
    CONSTRAINT ck_pr_verification_status CHECK (((status)::text = ANY ((ARRAY['passed'::character varying, 'failed'::character varying, 'unavailable'::character varying])::text[])))
);

CREATE TABLE IF NOT EXISTS public.pr_media_rules (
    id character(36) NOT NULL,
    tier_default_quality jsonb NOT NULL DEFAULT '{"tier1": 90, "tier2": 80, "tier3": 70}'::jsonb,
    category_relevance jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_by character(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pr_media_rules_pkey PRIMARY KEY (id)
);



-- ====================================================================

-- 2. PRIMARY KEYS

-- ====================================================================

ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.brand_ai_quotas
    ADD CONSTRAINT brand_ai_quotas_pkey PRIMARY KEY (brand_id);

ALTER TABLE ONLY public.brand_memberships
    ADD CONSTRAINT brand_memberships_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.brand_profiles
    ADD CONSTRAINT brand_profiles_pkey PRIMARY KEY (brand_id);

ALTER TABLE ONLY public.brands
    ADD CONSTRAINT brands_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.bug_reports
    ADD CONSTRAINT bug_reports_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.chat_learnings
    ADD CONSTRAINT chat_learnings_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.cms_connections
    ADD CONSTRAINT cms_connections_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.content_batch_items
    ADD CONSTRAINT content_batch_items_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.content_batches
    ADD CONSTRAINT content_batches_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.content_briefs
    ADD CONSTRAINT content_briefs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.content_documents
    ADD CONSTRAINT content_documents_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.content_jobs
    ADD CONSTRAINT content_jobs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.content_versions
    ADD CONSTRAINT content_versions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.editorial_rule_sets
    ADD CONSTRAINT editorial_rule_sets_pkey PRIMARY KEY (brand_id);

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.knowledge_chunks
    ADD CONSTRAINT knowledge_chunks_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.knowledge_collections
    ADD CONSTRAINT knowledge_collections_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.knowledge_crawl_runs
    ADD CONSTRAINT knowledge_crawl_runs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.knowledge_sources
    ADD CONSTRAINT knowledge_sources_pkey PRIMARY KEY (id);






ALTER TABLE ONLY public.master_prompt_versions
    ADD CONSTRAINT master_prompt_versions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.model_usage_events
    ADD CONSTRAINT model_usage_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.platform_admins
    ADD CONSTRAINT platform_admins_pkey PRIMARY KEY (user_id);

ALTER TABLE ONLY public.platform_ai_configuration
    ADD CONSTRAINT platform_ai_configuration_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.publication_jobs
    ADD CONSTRAINT publication_jobs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.quality_evaluations
    ADD CONSTRAINT quality_evaluations_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.review_comments
    ADD CONSTRAINT review_comments_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.system_metadata
    ADD CONSTRAINT system_metadata_pkey PRIMARY KEY (key);

ALTER TABLE ONLY public.user_daily_quotas
    ADD CONSTRAINT user_daily_quotas_pkey PRIMARY KEY (user_id);

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.voice_profiles
    ADD CONSTRAINT voice_profiles_pkey PRIMARY KEY (brand_id);

ALTER TABLE ONLY public.websites
    ADD CONSTRAINT websites_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.media
    ADD CONSTRAINT media_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.partners
    ADD CONSTRAINT partners_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT publications_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.publication_attempts
    ADD CONSTRAINT publication_attempts_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.rss_feeds
    ADD CONSTRAINT rss_feeds_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.rss_access_logs
    ADD CONSTRAINT rss_access_logs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.campaigns
    ADD CONSTRAINT campaigns_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.packages
    ADD CONSTRAINT packages_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.credit_transactions
    ADD CONSTRAINT credit_transactions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.revenue_shares
    ADD CONSTRAINT revenue_shares_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.partner_earnings
    ADD CONSTRAINT partner_earnings_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.settlements
    ADD CONSTRAINT settlements_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT transactions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT invoices_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.tracking_links
    ADD CONSTRAINT tracking_links_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.tracking_events
    ADD CONSTRAINT tracking_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.analytics_daily
    ADD CONSTRAINT analytics_daily_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.ga4_configs
    ADD CONSTRAINT ga4_configs_pkey PRIMARY KEY (id);



-- ====================================================================

-- 3. OTHER ALTERS & CONSTRAINTS

-- ====================================================================

ALTER TABLE ONLY public.brand_memberships
    ADD CONSTRAINT uq_brand_membership_brand_user UNIQUE (brand_id, user_id);

ALTER TABLE ONLY public.brand_memberships
    ADD CONSTRAINT uq_brand_memberships_brand_user UNIQUE (brand_id, user_id);

ALTER TABLE ONLY public.brands
    ADD CONSTRAINT uq_brands_slug UNIQUE (slug);

ALTER TABLE ONLY public.brands
    ADD CONSTRAINT uq_brands_public_id UNIQUE (public_id);

ALTER TABLE ONLY public.cms_connections
    ADD CONSTRAINT uq_cms_connection_brand_name UNIQUE (brand_id, name);

ALTER TABLE ONLY public.content_batch_items
    ADD CONSTRAINT uq_content_batch_fingerprint UNIQUE (batch_id, fingerprint);

ALTER TABLE ONLY public.content_batch_items
    ADD CONSTRAINT uq_content_batch_row UNIQUE (batch_id, row_number);

ALTER TABLE ONLY public.content_documents
    ADD CONSTRAINT uq_content_document_job UNIQUE (job_id);

ALTER TABLE ONLY public.content_jobs
    ADD CONSTRAINT uq_content_job_idempotency UNIQUE (brand_id, idempotency_key);

ALTER TABLE ONLY public.content_versions
    ADD CONSTRAINT uq_content_versions_document_version UNIQUE (document_id, version);

ALTER TABLE ONLY public.knowledge_collections
    ADD CONSTRAINT uq_knowledge_collection_brand_name UNIQUE (brand_id, name);

ALTER TABLE ONLY public.knowledge_chunks
    ADD CONSTRAINT uq_knowledge_source_chunk UNIQUE (source_id, chunk_index);

ALTER TABLE ONLY public.master_prompt_versions
    ADD CONSTRAINT uq_master_prompt_brand_version UNIQUE (brand_id, version);

ALTER TABLE ONLY public.publication_jobs
    ADD CONSTRAINT uq_publication_document_connection UNIQUE (document_id, connection_id);

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT uq_user_sessions_refresh_jti UNIQUE (refresh_jti);

ALTER TABLE ONLY public.users
    ADD CONSTRAINT uq_users_email UNIQUE (email);

ALTER TABLE ONLY public.websites
    ADD CONSTRAINT uq_website_brand_domain UNIQUE (brand_id, domain);

ALTER TABLE public.campaign_media_items OWNER TO ai_writer;
ALTER TABLE public.campaign_report_shares OWNER TO ai_writer;

ALTER TABLE ONLY public.campaigns ADD COLUMN content_intake_id character(36);

ALTER TABLE ONLY public.campaigns ADD COLUMN campaign_flow_status character varying(30) DEFAULT 'intake'::character varying NOT NULL;

ALTER TABLE ONLY public.media
    ADD CONSTRAINT uq_media_slug UNIQUE (slug);

ALTER TABLE ONLY public.partners
    ADD CONSTRAINT uq_partners_user UNIQUE (user_id);

ALTER TABLE ONLY public.partners
    ADD CONSTRAINT uq_partners_public_id UNIQUE (public_id);

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT uq_publication_document_media UNIQUE (document_id, media_id);

ALTER TABLE ONLY public.publication_attempts
    ADD CONSTRAINT uq_publication_attempt_number UNIQUE (publication_id, attempt_number);

ALTER TABLE ONLY public.rss_feeds
    ADD CONSTRAINT uq_rss_feed_token UNIQUE (token);

ALTER TABLE ONLY public.rss_feeds
    ADD CONSTRAINT uq_rss_feed_partner_name UNIQUE (partner_id, name);

ALTER TABLE public.users ADD COLUMN IF NOT EXISTS role character varying(20) NOT NULL DEFAULT 'client';

ALTER TABLE ONLY public.users
    ADD CONSTRAINT ck_users_role CHECK (((role)::text = ANY ((ARRAY['client'::character varying, 'partner'::character varying, 'admin'::character varying])::text[])));

ALTER TABLE public.users ADD COLUMN IF NOT EXISTS registration_source character varying(20) NOT NULL DEFAULT 'public';

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_registration_source_check CHECK (registration_source IN ('public', 'admin', 'google'));

ALTER TABLE public.audit_logs ADD COLUMN IF NOT EXISTS detail jsonb;

ALTER TABLE ONLY public.media DROP CONSTRAINT IF EXISTS ck_media_integration_type;

ALTER TABLE ONLY public.media ADD CONSTRAINT ck_media_integration_type
    CHECK (integration_type IN ('manual','wordpress','rss','webhook','tulisio_api','custom_web','blogger'));

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS adapter_url character varying(500);

ALTER TABLE public.publication_jobs ALTER COLUMN connection_id DROP NOT NULL;

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS media_tier character varying(20) DEFAULT 'tier3';

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS media_quality_score smallint;

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS media_quality_notes character varying(500);

ALTER TABLE public.publications ADD COLUMN IF NOT EXISTS auto_replace_disabled boolean NOT NULL DEFAULT FALSE;

ALTER TABLE public.content_intakes ADD COLUMN IF NOT EXISTS key_messages jsonb NOT NULL DEFAULT '[]';



-- ====================================================================

-- 4. FOREIGN KEYS

-- ====================================================================

ALTER TABLE ONLY public.cms_connections
    ADD CONSTRAINT fk_cms_connections_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id);

ALTER TABLE ONLY public.brand_ai_quotas
    ADD CONSTRAINT fk_brand_ai_quotas_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.brand_memberships
    ADD CONSTRAINT fk_brand_memberships_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.brand_memberships
    ADD CONSTRAINT fk_brand_memberships_user FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.brand_profiles
    ADD CONSTRAINT fk_brand_profiles_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.brands
    ADD CONSTRAINT fk_brands_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.content_briefs
    ADD CONSTRAINT fk_content_briefs_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.content_documents
    ADD CONSTRAINT fk_content_documents_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.knowledge_crawl_runs
    ADD CONSTRAINT fk_crawl_runs_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.knowledge_crawl_runs
    ADD CONSTRAINT fk_crawl_runs_collection FOREIGN KEY (collection_id) REFERENCES public.knowledge_collections(id);

ALTER TABLE ONLY public.editorial_rule_sets
    ADD CONSTRAINT fk_editorial_rule_sets_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.knowledge_chunks
    ADD CONSTRAINT fk_knowledge_chunks_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.knowledge_chunks
    ADD CONSTRAINT fk_knowledge_chunks_collection FOREIGN KEY (collection_id) REFERENCES public.knowledge_collections(id);

ALTER TABLE ONLY public.knowledge_chunks
    ADD CONSTRAINT fk_knowledge_chunks_source FOREIGN KEY (source_id) REFERENCES public.knowledge_sources(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.knowledge_collections
    ADD CONSTRAINT fk_knowledge_collections_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.knowledge_sources
    ADD CONSTRAINT fk_knowledge_sources_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.knowledge_sources
    ADD CONSTRAINT fk_knowledge_sources_collection FOREIGN KEY (collection_id) REFERENCES public.knowledge_collections(id);

ALTER TABLE ONLY public.knowledge_sources
    ADD CONSTRAINT fk_knowledge_sources_creator FOREIGN KEY (created_by) REFERENCES public.users(id);

ALTER TABLE ONLY public.knowledge_base_rule_chunks
    ADD CONSTRAINT fk_kb_rule_chunks_rule FOREIGN KEY (rule_id) REFERENCES public.knowledge_base_rules(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.knowledge_base_rules
    ADD CONSTRAINT fk_kb_rules_base FOREIGN KEY (knowledge_base_id) REFERENCES public.knowledge_bases(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.knowledge_base_rules
    ADD CONSTRAINT fk_kb_rules_version FOREIGN KEY (version_id) REFERENCES public.knowledge_base_versions(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.knowledge_base_sources
    ADD CONSTRAINT fk_kb_sources_base FOREIGN KEY (knowledge_base_id) REFERENCES public.knowledge_bases(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.knowledge_base_versions
    ADD CONSTRAINT fk_kb_versions_base FOREIGN KEY (knowledge_base_id) REFERENCES public.knowledge_bases(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.master_prompt_versions
    ADD CONSTRAINT fk_master_prompt_versions_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.master_prompt_versions
    ADD CONSTRAINT fk_master_prompt_versions_creator FOREIGN KEY (created_by) REFERENCES public.users(id);

ALTER TABLE ONLY public.platform_admins
    ADD CONSTRAINT fk_platform_admins_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.platform_ai_configuration
    ADD CONSTRAINT fk_platform_ai_config_updated_by FOREIGN KEY (updated_by) REFERENCES public.users(id);

ALTER TABLE ONLY public.user_daily_quotas
    ADD CONSTRAINT fk_user_daily_quota_updated_by FOREIGN KEY (updated_by) REFERENCES public.users(id);

ALTER TABLE ONLY public.user_daily_quotas
    ADD CONSTRAINT fk_user_daily_quota_user FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT fk_user_sessions_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.voice_profiles
    ADD CONSTRAINT fk_voice_profiles_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.websites
    ADD CONSTRAINT fk_websites_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.campaign_media_items
    ADD CONSTRAINT fk_campaign_media_items_campaign
    FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.campaign_media_items
    ADD CONSTRAINT fk_campaign_media_items_media
    FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE RESTRICT;

ALTER TABLE ONLY public.campaign_media_items
    ADD CONSTRAINT fk_campaign_media_items_source_document
    FOREIGN KEY (source_document_id) REFERENCES public.content_documents(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.campaign_media_items
    ADD CONSTRAINT fk_campaign_media_items_variant_document
    FOREIGN KEY (variant_document_id) REFERENCES public.content_documents(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.campaign_report_shares
    ADD CONSTRAINT fk_campaign_report_share_campaign
    FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.campaign_report_shares
    ADD CONSTRAINT fk_campaign_report_share_brand
    FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.campaign_report_shares
    ADD CONSTRAINT fk_campaign_report_share_creator
    FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.media
    ADD CONSTRAINT fk_media_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.media
    ADD CONSTRAINT fk_media_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.partners
    ADD CONSTRAINT fk_partners_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_document FOREIGN KEY (document_id) REFERENCES public.content_documents(id);

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_media FOREIGN KEY (media_id) REFERENCES public.media(id);

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_creator FOREIGN KEY (created_by) REFERENCES public.users(id);

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_connection FOREIGN KEY (connection_id) REFERENCES public.cms_connections(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_replaces FOREIGN KEY (replaces_id) REFERENCES public.publications(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_reviewed_by FOREIGN KEY (reviewed_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.publication_attempts
    ADD CONSTRAINT fk_publication_attempts_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.rss_feeds
    ADD CONSTRAINT fk_rss_feeds_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.rss_feeds
    ADD CONSTRAINT fk_rss_feeds_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.rss_feeds
    ADD CONSTRAINT fk_rss_feeds_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.rss_access_logs
    ADD CONSTRAINT fk_rss_access_logs_feed FOREIGN KEY (feed_id) REFERENCES public.rss_feeds(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.publication_jobs
    ADD CONSTRAINT fk_publication_jobs_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.campaigns
    ADD CONSTRAINT fk_campaigns_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.campaigns
    ADD CONSTRAINT fk_campaigns_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.packages
    ADD CONSTRAINT fk_packages_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.packages
    ADD CONSTRAINT fk_packages_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.credit_transactions
    ADD CONSTRAINT fk_credit_tx_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.credit_transactions
    ADD CONSTRAINT fk_credit_tx_package FOREIGN KEY (package_id) REFERENCES public.packages(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.credit_transactions
    ADD CONSTRAINT fk_credit_tx_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.credit_transactions
    ADD CONSTRAINT fk_credit_tx_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.revenue_shares
    ADD CONSTRAINT fk_revenue_shares_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.revenue_shares
    ADD CONSTRAINT fk_revenue_shares_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.revenue_shares
    ADD CONSTRAINT fk_revenue_shares_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.partner_earnings
    ADD CONSTRAINT fk_partner_earnings_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.partner_earnings
    ADD CONSTRAINT fk_partner_earnings_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.partner_earnings
    ADD CONSTRAINT fk_partner_earnings_media FOREIGN KEY (media_id) REFERENCES public.media(id);

ALTER TABLE ONLY public.partner_earnings
    ADD CONSTRAINT fk_partner_earnings_rule FOREIGN KEY (rule_id) REFERENCES public.revenue_shares(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.partner_earnings
    ADD CONSTRAINT fk_partner_earnings_settlement FOREIGN KEY (settlement_id) REFERENCES public.settlements(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.settlements
    ADD CONSTRAINT fk_settlements_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.settlements
    ADD CONSTRAINT fk_settlements_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT fk_transactions_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT fk_transactions_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id);

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT fk_transactions_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT fk_transactions_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT fk_transactions_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT fk_invoices_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT fk_invoices_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT fk_invoices_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.tracking_links
    ADD CONSTRAINT fk_tracking_links_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.tracking_links
    ADD CONSTRAINT fk_tracking_links_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.tracking_links
    ADD CONSTRAINT fk_tracking_links_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.tracking_events
    ADD CONSTRAINT fk_tracking_events_link FOREIGN KEY (link_id) REFERENCES public.tracking_links(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.tracking_events
    ADD CONSTRAINT fk_tracking_events_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.tracking_events
    ADD CONSTRAINT fk_tracking_events_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.analytics_daily
    ADD CONSTRAINT fk_analytics_daily_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.analytics_daily
    ADD CONSTRAINT fk_analytics_daily_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.ga4_configs
    ADD CONSTRAINT fk_ga4_configs_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);

ALTER TABLE ONLY public.ga4_configs
    ADD CONSTRAINT fk_ga4_configs_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.tracking_events
    ADD CONSTRAINT fk_tracking_events_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.analytics_daily
    ADD CONSTRAINT fk_analytics_daily_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;



-- ====================================================================

-- 5. UNIQUE & REGULAR INDEXES

-- ====================================================================

CREATE UNIQUE INDEX uq_cms_connection_partner_name ON public.cms_connections USING btree (partner_id, name) WHERE (partner_id IS NOT NULL);

CREATE UNIQUE INDEX uq_knowledge_source_url ON public.knowledge_sources USING btree (collection_id, source_type, original_filename)
  WHERE (source_type = 'url' AND original_filename IS NOT NULL AND deleted_at IS NULL);

CREATE UNIQUE INDEX uq_knowledge_base_versions_active ON public.knowledge_base_versions USING btree (knowledge_base_id) WHERE (status = 'active'::character varying);

CREATE UNIQUE INDEX uq_media_packages_slug ON public.media_packages USING btree (slug);

CREATE UNIQUE INDEX uq_auto_replace_global ON public.auto_replace_configs USING btree ((TRUE)) WHERE (brand_id IS NULL);

CREATE UNIQUE INDEX uq_auto_replace_brand ON public.auto_replace_configs USING btree (brand_id) WHERE (brand_id IS NOT NULL);

CREATE UNIQUE INDEX uq_notification_webhook_user_url ON public.notification_webhooks (user_id, url);

CREATE UNIQUE INDEX uq_analytics_daily ON public.analytics_daily USING btree (brand_id, event_date, event_type, COALESCE(publication_id, ''));

CREATE UNIQUE INDEX uq_payment_gateway_single_active ON public.payment_gateway_configs ((TRUE)) WHERE is_active = TRUE;

CREATE UNIQUE INDEX uq_payment_transaction_external ON public.payment_transactions (gateway_provider, external_transaction_id) WHERE external_transaction_id IS NOT NULL;

CREATE UNIQUE INDEX uq_payment_transactions_brand_idem ON public.payment_transactions (brand_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE UNIQUE INDEX uq_credit_ledger_idempotency ON public.credit_ledger_entries (idempotency_key);

CREATE INDEX ix_bug_report_status_created ON public.bug_reports USING btree (status, created_at);

CREATE INDEX ix_bug_report_user ON public.bug_reports USING btree (user_id);

CREATE INDEX ix_chat_learning_brand_created ON public.chat_learnings USING btree (brand_id, created_at);

CREATE INDEX ix_chat_learning_conv ON public.chat_learnings USING btree (conversation_id, created_at);

CREATE INDEX ix_chat_learning_intent ON public.chat_learnings USING btree (brand_id, intent_tag);

CREATE INDEX ix_cms_connection_brand_status ON public.cms_connections USING btree (brand_id, status);

CREATE INDEX ix_cms_connection_partner_status ON public.cms_connections USING btree (partner_id, status);

CREATE INDEX ix_content_batch_brand_created ON public.content_batches USING btree (brand_id, created_at);

CREATE INDEX ix_content_batch_item_status ON public.content_batch_items USING btree (batch_id, status);

CREATE INDEX ix_content_brief_brand_created ON public.content_briefs USING btree (brand_id, created_at);

CREATE INDEX ix_content_document_brand_status ON public.content_documents USING btree (brand_id, status);

CREATE INDEX ix_content_job_brand_created ON public.content_jobs USING btree (brand_id, created_at);

CREATE INDEX ix_content_versions_document_version ON public.content_versions USING btree (document_id, version);

CREATE INDEX ix_jobs_claim ON public.jobs USING btree (status, run_at);

CREATE INDEX ix_knowledge_chunk_brand ON public.knowledge_chunks USING btree (brand_id);

CREATE INDEX ix_master_prompt_brand_status ON public.master_prompt_versions USING btree (brand_id, status);

CREATE INDEX idx_kb_rule_chunks_rule ON public.knowledge_base_rule_chunks USING btree (rule_id);

CREATE INDEX idx_kb_rules_base_status ON public.knowledge_base_rules USING btree (knowledge_base_id, status);

CREATE INDEX idx_kb_rules_version ON public.knowledge_base_rules USING btree (version_id);

CREATE INDEX idx_kb_sources_base ON public.knowledge_base_sources USING btree (knowledge_base_id);

CREATE INDEX ix_model_usage_brand_created ON public.model_usage_events USING btree (brand_id, created_at);

CREATE INDEX ix_model_usage_provider_model ON public.model_usage_events USING btree (provider, model);

CREATE INDEX ix_model_usage_user_date ON public.model_usage_events USING btree (user_id, created_at);

CREATE INDEX ix_publication_jobs_brand_created ON public.publication_jobs USING btree (brand_id, created_at);

CREATE INDEX ix_quality_brand_created ON public.quality_evaluations USING btree (brand_id, created_at);

CREATE INDEX ix_quality_document_created ON public.quality_evaluations USING btree (document_id, created_at);

CREATE INDEX ix_review_comment_document_created ON public.review_comments USING btree (document_id, created_at);

CREATE INDEX ix_media_package_items_media ON public.media_package_items USING btree (media_id);

CREATE INDEX ix_notification_webhooks_active ON public.notification_webhooks (active);

CREATE INDEX ix_notifications_user_read ON public.notifications (user_id, read_at, created_at DESC);

CREATE INDEX ix_notifications_publication ON public.notifications (publication_id);

CREATE INDEX ix_notifications_user_read_expired ON public.notifications (user_id, expired_at, read_at, created_at DESC);

CREATE INDEX ix_campaign_media_items_campaign ON public.campaign_media_items (campaign_id);

CREATE INDEX ix_campaign_media_items_media ON public.campaign_media_items (media_id);

CREATE INDEX ix_campaigns_intake ON public.campaigns (content_intake_id);

CREATE INDEX ix_campaign_report_shares_brand ON public.campaign_report_shares USING btree (brand_id);

CREATE INDEX ix_media_status ON public.media USING btree (status, name);

CREATE INDEX ix_media_partner ON public.media USING btree (partner_id, status);

CREATE INDEX ix_publication_status_created ON public.publications USING btree (status, created_at);

CREATE INDEX ix_publication_brand_created ON public.publications USING btree (brand_id, created_at);

CREATE INDEX ix_publication_brand_status ON public.publications USING btree (brand_id, status);

CREATE INDEX ix_publication_media_status ON public.publications USING btree (media_id, status);

CREATE INDEX ix_publications_replaces ON public.publications USING btree (replaces_id) WHERE (replaces_id IS NOT NULL);

CREATE INDEX ix_publication_attempt_publication ON public.publication_attempts USING btree (publication_id, attempt_number);

CREATE INDEX ix_publication_jobs_publication ON public.publication_jobs USING btree (publication_id);

CREATE INDEX ix_rss_feed_partner ON public.rss_feeds USING btree (partner_id, status);

CREATE INDEX ix_rss_feed_media ON public.rss_feeds USING btree (media_id);

CREATE INDEX ix_rss_access_logs_feed_time ON public.rss_access_logs USING btree (feed_id, created_at);

CREATE INDEX ix_campaign_brand_status ON public.campaigns USING btree (brand_id, status);

CREATE INDEX ix_package_brand_status ON public.packages USING btree (brand_id, status);

CREATE INDEX ix_credit_tx_brand_time ON public.credit_transactions USING btree (brand_id, created_at DESC);

CREATE INDEX ix_credit_tx_publication ON public.credit_transactions USING btree (publication_id);

CREATE INDEX ix_publication_campaign ON public.publications USING btree (campaign_id);

CREATE INDEX ix_revenue_share_lookup ON public.revenue_shares USING btree (status, media_id, partner_id);

CREATE INDEX ix_partner_earnings_partner_status ON public.partner_earnings USING btree (partner_id, status, created_at);

CREATE INDEX ix_settlement_partner_status ON public.settlements USING btree (partner_id, status);

CREATE INDEX ix_transaction_brand_time ON public.transactions USING btree (brand_id, created_at DESC);

CREATE INDEX ix_transaction_partner_time ON public.transactions USING btree (partner_id, created_at DESC);

CREATE INDEX ix_invoice_brand_status ON public.invoices USING btree (brand_id, status);

CREATE INDEX ix_tracking_link_brand ON public.tracking_links USING btree (brand_id, created_at DESC);

CREATE INDEX ix_tracking_events_brand_time ON public.tracking_events USING btree (brand_id, created_at DESC);

CREATE INDEX ix_tracking_events_link_time ON public.tracking_events USING btree (link_id, created_at DESC);

CREATE INDEX ix_tracking_events_type_time ON public.tracking_events USING btree (event_type, created_at DESC);

CREATE INDEX ix_ga4_config_status ON public.ga4_configs USING btree (brand_id, enabled);

CREATE INDEX ix_tracking_events_campaign_time ON public.tracking_events USING btree (campaign_id, created_at DESC);

CREATE INDEX ix_analytics_daily_campaign_date ON public.analytics_daily USING btree (brand_id, campaign_id, event_date);

CREATE INDEX ix_payment_transactions_user ON public.payment_transactions USING btree (user_id, created_at DESC);

CREATE INDEX ix_credit_ledger_brand ON public.credit_ledger_entries USING btree (brand_id, created_at DESC);

CREATE INDEX ix_credit_ledger_ref ON public.credit_ledger_entries USING btree (ref_id, ref_type);

CREATE INDEX ix_content_intakes_brand_created ON public.content_intakes (brand_id, created_at DESC);

CREATE INDEX ix_publishing_api_keys_partner ON public.publishing_api_keys (partner_id);

CREATE INDEX IF NOT EXISTS ix_callback_logs_key_time ON public.publishing_callback_logs (api_key_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_callback_logs_pub_time ON public.publishing_callback_logs (publication_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_variant_batch_brand_time ON public.publication_variant_batches (brand_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_variant_publication ON public.publication_variants (publication_id);

CREATE INDEX IF NOT EXISTS ix_variant_batch ON public.publication_variants (batch_id, created_at);

CREATE INDEX IF NOT EXISTS idx_pr_verifications_campaign ON public.pr_publication_verifications (campaign_id);



-- ====================================================================

-- 6. INITIAL SEED INSERTS

-- ====================================================================

INSERT INTO public.tc_config (id, rate_idr) VALUES (1, 1000) ON CONFLICT (id) DO NOTHING;

INSERT INTO public.knowledge_bases (id, code, name, description, scope, scope_ref, status, created_by)
VALUES ('00000000-0000-4000-8000-00000000a001', 'journalistic-writing-id', 'Standar Penulisan Jurnalistik Indonesia',
        'Aturan editorial dan gaya penulisan yang digunakan AI Tulisio untuk menghasilkan konten berita/press release yang layak dibaca dalam konteks media Indonesia. Baseline editorial global Tulisio.',
        'global', NULL, 'active', '00000000-0000-4000-8000-000000000001')
ON CONFLICT (code) DO NOTHING;

INSERT INTO public.knowledge_base_versions (id, knowledge_base_id, version, status, change_note, created_by)
VALUES ('00000000-0000-4000-8000-00000000a002', '00000000-0000-4000-8000-00000000a001', 1, 'active', 'Baseline jurnalistik Indonesia', '00000000-0000-4000-8000-000000000001')
ON CONFLICT (knowledge_base_id, version) DO NOTHING;

INSERT INTO public.knowledge_base_sources (id, knowledge_base_id, source_name, source_url, source_type, notes, accessed_at) VALUES
  ('00000000-0000-4000-8000-00000000a101', '00000000-0000-4000-8000-00000000a001', 'Peraturan Dewan Pers Nomor 6/Peraturan-DP/V/2008', 'https://dewanpers.or.id', 'regulation', 'Pengesahan Kode Etik Jurnalistik', '2026-01-01'),
  ('00000000-0000-4000-8000-00000000a102', '00000000-0000-4000-8000-00000000a001', 'Peraturan Dewan Pers Nomor 1/Peraturan-DP/III/2012', 'https://dewanpers.or.id', 'regulation', 'Pedoman Pemberitaan Media Siber', '2026-01-01'),
  ('00000000-0000-4000-8000-00000000a103', '00000000-0000-4000-8000-00000000a001', 'Peraturan Dewan Pers Nomor 03/Peraturan-DP/IV/2024', 'https://dewanpers.or.id', 'regulation', 'Pedoman Perilaku dan Standar Pers Profesional', '2026-01-01'),
  ('00000000-0000-4000-8000-00000000a104', '00000000-0000-4000-8000-00000000a001', 'Peraturan Dewan Pers Nomor 1/PERATURAN-DP/I/2025', 'https://dewanpers.or.id', 'regulation', 'Pedoman Penggunaan Kecerdasan Buatan dalam Karya Jurnalistik', '2026-01-01'),
  ('00000000-0000-4000-8000-00000000a105', '00000000-0000-4000-8000-00000000a001', 'EYD Edisi V', 'https://badanbahasa.kemdikbud.go.id', 'dictionary', 'Pedoman Umum Ejaan Bahasa Indonesia Edisi V (Badan Pengembangan dan Pembinaan Bahasa)', '2026-01-01'),
  ('00000000-0000-4000-8000-00000000a106', '00000000-0000-4000-8000-00000000a001', 'KBBI Daring', 'https://kbbi.kemdikbud.go.id', 'dictionary', 'Referensi kosakata bahasa Indonesia baku', '2026-01-01')
ON CONFLICT DO NOTHING;

INSERT INTO public.knowledge_base_rules
  (id, knowledge_base_id, version_id, category, title, instruction, examples, forbidden_examples, tags, audience, content_types, priority, status)
VALUES
  ('00000000-0000-4000-8000-00000000a201', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Editorial Principles', 'Tulisan penulis berita Indonesia',
   'Konten harus faktual, informatif, ringkas, padat, jelas, lugas, dan natural — seperti naskah yang ditulis penulis/editor media Indonesia. Bukan iklan, bukan artikel SEO generik, bukan opini AI, bukan copywriting korporat yang dipanjangkan.',
   '["Perusahaan meluncurkan platform baru untuk membantu UMKM mengelola transaksi."]', '["Di era digital yang semakin berkembang, kebutuhan akan solusi inovatif menjadi semakin penting."]', '["baseline","editorial"]', '[]', '[]', 2, 'active'),
  ('00000000-0000-4000-8000-00000000a202', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Journalistic Structure', 'Inverted pyramid',
   'Susun informasi dengan prinsip piramida terbalik: informasi paling penting di awal, lalu fakta utama, detail pendukung, kutipan, konteks, dan informasi tambahan. Paragraf awal harus membuat pembaca memahami inti tanpa membaca seluruh artikel.',
   '["JAKARTA, KOMPAS.com - PT XYZ meluncurkan platform digital baru untuk 500 pelaku UMKM di Indonesia pada Senin (10/8/2026)."]', '["Perusahaan yang telah berdiri sejak 1990 dan melalui berbagai perjalanan panjang ini, kini akhirnya meluncurkan sebuah platform yang diharapkan dapat membantu masyarakat."]', '["struktur","pyramid"]', '[]', '[]', 4, 'active'),
  ('00000000-0000-4000-8000-00000000a203', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Lead', 'Lead langsung ke inti peristiwa',
   'Lead/teras berita harus langsung menuju inti peristiwa: informatif, tidak bertele-tele, dapat dipahami tanpa paragraf berikutnya. Prioritaskan WHAT + WHO + WHEN/WHERE; tambahkan WHY/HOW jika tersedia dan relevan. Hindari kalimat pembuka generik atau klise.',
   '["PT ABC meluncurkan layanan pembayaran digital baru pada Senin di Jakarta untuk mempercepat transaksi UMKM."]', '["PT ABC dengan bangga mengumumkan sebuah terobosan luar biasa yang akan mengubah masa depan Indonesia."]', '["lead","teras"]', '[]', '[]', 4, 'active'),
  ('00000000-0000-4000-8000-00000000a204', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Headline', 'Judul informatif dan spesifik',
   'Judul harus informatif, spesifik, ringkas, dan menggambarkan isi. Jangan clickbait, jangan berlebihan, jangan menipu ekspektasi pembaca, jangan ALL CAPS (kecuali nama resmi), jangan terlalu banyak tanda seru. Judul tidak boleh memuat informasi yang tidak ada di isi.',
   '["XYZ Meluncurkan Platform Baru untuk Mendukung UMKM"]', '["TEROBOSAN LUAR BIASA! XYZ UBAH MASA DEPAN INDONESIA"]', '["headline","judul"]', '[]', '[]', 4, 'active'),
  ('00000000-0000-4000-8000-00000000a205', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', '5W+1H', 'Manfaatkan seluruh unsur yang tersedia',
   'Analisis bahan dengan 5W+1H: What, Who, When, Where, Why, How. Manfaatkan semua unsur yang tersedia dari brief/bahan. Jika ada unsur yang tidak tersedia, jangan mengarang, menebak, atau membuat fakta sintetis — gunakan formulasi jujur atau tandai sebagai informasi yang belum tersedia.',
   '["Berdasarkan keterangan perusahaan, jumlah peserta mencapai 500 orang, namun detail lokasi belum dirilis."]', '["Acara dihadiri ribuan peserta dari berbagai daerah di Indonesia."]', '["5w1h","fakta"]', '[]', '[]', 2, 'active'),
  ('00000000-0000-4000-8000-00000000a206', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Language Style', 'Bahasa jurnalistik yang efisien',
   'Gunakan bahasa Indonesia baku dengan gaya jurnalistik: singkat, padat, jelas, lugas, sederhana, komunikatif, dan menarik melalui informasi bukan hiperbola. Prioritaskan "jelas dan cepat dipahami" daripada "terdengar canggih".',
   '["Pemerintah menaikkan subsidi energi untuk 10 juta keluarga berpenghasilan rendah."]', '["Pemerintah mengambil langkah strategis yang monumental dalam rangka memberikan kontribusi signifikan bagi kesejahteraan masyarakat luas."]', '["bahasa","gaya"]', '[]', '[]', 6, 'active'),
  ('00000000-0000-4000-8000-00000000a207', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Paragraph', 'Paragraf pendek, satu ide',
   'Gunakan paragraf pendek (satu ide utama per paragraf untuk konten digital). Hindari paragraf sangat panjang dan jangan memaksakan heading. Untuk press release/news pendek, gunakan struktur artikel tanpa heading secara default; gunakan heading hanya jika konten panjang atau diminta.',
   '["Perusahaan menggelar pelatihan digital bagi 500 UMKM. Pelatihan berlangsung selama tiga hari di Jakarta."]', '["Dalam kegiatan yang diselenggarakan oleh perusahaan tersebut yang bertujuan untuk memberikan berbagai macam manfaat kepada para peserta yang hadir, perusahaan menyampaikan bahwa program ini merupakan wujud nyata dari komitmen perusahaan dalam mendorong transformasi digital di Indonesia yang lebih maju dan berkelanjutan."]', '["paragraf","struktur"]', '[]', '[]', 5, 'active'),
  ('00000000-0000-4000-8000-00000000a208', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Fact & Opinion', 'Pisahkan fakta dan opini',
   'AI WAJIB membedakan fakta dan opini. Opini (terutama dari narasumber/perusahaan) harus diatribusikan, misalnya "Menurut CEO XYZ...". Jangan mengubah opini menjadi fakta dan jangan menghapus atribusi dari klaim subjektif.',
   '["Menurut CEO XYZ, produk tersebut merupakan solusi terbaik di pasar saat ini."]', '["Produk XYZ adalah yang terbaik di Indonesia."]', '["fakta","opini","atribusi"]', '[]', '[]', 1, 'active'),
  ('00000000-0000-4000-8000-00000000a209', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Attribution', 'Pertahankan atribusi sumber',
   'Jika informasi berasal dari narasumber, perusahaan, lembaga, laporan, penelitian, dokumen, atau pernyataan resmi, gunakan atribusi yang sesuai ("Menurut data...", "Berdasarkan laporan...", "Dalam keterangannya...", "CEO XYZ mengatakan..."). Jangan menghapus sumber demi membuat tulisan lebih singkat.',
   '["Berdasarkan laporan Bank Indonesia, inflasi inti tercatat 2,8 persen pada Juli 2026."]', '["Inflasi inti tercatat 2,8 persen."]', '["atribusi","sumber"]', '[]', '[]', 3, 'active'),
  ('00000000-0000-4000-8000-00000000a210', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Quote', 'Jangan mengarang kutipan',
   'DILARANG membuat direct quote yang tidak diberikan user. Jika hanya tersedia informasi/parafrasa, gunakan bentuk atribusi parafrasa ("CEO XYZ mengatakan langkah tersebut merupakan bagian dari strategi perusahaan"), bukan tanda kutip langsung. Jangan mengubah makna kutipan yang memang diberikan.',
   '["CEO XYZ mengatakan langkah tersebut merupakan bagian dari strategi ekspansi perusahaan."]', '["“Ini merupakan langkah strategis terbesar kami,” kata CEO XYZ."]', '["kutipan","quote"]', '[]', '[]', 1, 'active'),
  ('00000000-0000-4000-8000-00000000a211', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Data & Numbers', 'Pertahankan angka sesuai bahan',
   'Pertahankan angka, tanggal, dan satuan sesuai bahan. Jangan mengubah, membulatkan tanpa alasan, membuat persentase, atau menghitung statistik baru. Gunakan format mata uang natural Indonesia: Rp5 juta, Rp5,2 juta, Rp1,5 miliar.',
   '["Perusahaan menginvestasikan Rp5,2 miliar untuk program tersebut."]', '["Perusahaan menginvestasikan sekitar Rp5 miliar untuk program tersebut."]', '["angka","data"]', '[]', '[]', 2, 'active'),
  ('00000000-0000-4000-8000-00000000a212', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Press Release', 'Press release bergaya berita',
   'Tulis press release dengan gaya news/jurnalistik: informasi dari pihak/klien, tujuan menyampaikan informasi ke media, informatif, bukan iklan terang-terangan. Boleh menyebut brand/produk/layanan, tetapi klaim subjektif tetap diatribusikan. Struktur default: Headline, Lead, Detail utama, Kutipan narasumber, Detail/konteks, Informasi penutup. Tanpa heading H2/H3 secara default. Target maksimal 600 kata.',
   '["PT XYZ meluncurkan aplikasi baru untuk petani pada Rabu (12/8/2026) di Bandung. Aplikasi tersebut menyediakan data harga komoditas secara real-time."]', '["PT XYZ, perusahaan terdepan yang telah dipercaya jutaan pelanggan, dengan bangga mempersembahkan inovasi revolusioner yang akan mengubah wajah pertanian Indonesia selamanya."]', '["press-release","siaran-pers"]', '[]', '["press_release","event","csr","product_launch"]', 4, 'active'),
  ('00000000-0000-4000-8000-00000000a213', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'News Article', 'Straight news yang faktual',
   'Untuk artikel berita: sajikan straight news yang faktual dengan piramida terbalik, tanpa opini AI, tanpa menambahkan sudut pandang pribadi. Informasi utama di paragraf pertama.',
   '["Dinas Pendidikan DKI Jakarta mengumumkan pembukaan pendaftaran sekolah negeri tahap kedua pada 15 Agustus."]', '["Pendaftaran sekolah negeri yang dinanti-nantikan akhirnya dibuka. Ini kabar baik bagi orang tua yang selama ini menunggu dengan harapan besar."]', '["berita","news"]', '[]', '["news","article"]', 4, 'active'),
  ('00000000-0000-4000-8000-00000000a214', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'SEO', 'SEO mengikuti kualitas editorial',
   'SEO hanya digunakan setelah struktur editorial benar. Jika keyword diberikan, masukkan secara natural; jangan keyword stuffing, jangan memasukkan keyword jika merusak kalimat, jangan membuat judul aneh hanya untuk keyword. Prioritas: READABILITY > ACCURACY > RELEVANCE > SEO.',
   '["Artikel tentang pelatihan UMKM menyebut kata kunci “pelatihan UMKM digital” dua kali secara natural."]', '["Pelatihan UMKM digital adalah solusi pelatihan UMKM digital terbaik. Dengan pelatihan UMKM digital, pelaku UMKM bisa berkembang. Pelatihan UMKM digital..."]', '["seo"]', '[]', '[]', 7, 'active'),
  ('00000000-0000-4000-8000-00000000a215', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Anti Clickbait', 'Tanpa judul berlebihan',
   'Judul dan konten tidak boleh melebih-lebihkan atau menjanjikan sesuatu yang tidak ada di isi. Jangan gunakan hiperbola, kata berlebihan (luar biasa, spektakuler, revolusioner), atau tanda seru berlebihan.',
   '["Bank XYZ Meluncurkan Layanan Pinjaman untuk Nelayan"]', '["RAHASIA BESAR TERBONGKAR! BANK XYZ SELAMATKAN NELAYAN INDONESIA"]', '["clickbait","judul"]', '[]', '[]', 1, 'active'),
  ('00000000-0000-4000-8000-00000000a216', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Anti Hallucination', 'Dilarang mengarang fakta',
   'AI TIDAK BOLEH mengarang nama orang, jabatan, tanggal, lokasi, angka, statistik, jumlah peserta, omzet, revenue, market share, penghargaan, pelanggan, partner, sumber, kutipan, hasil penelitian, data survei, atau klaim performa (nomor satu, terbesar, pertama, terbaik, paling inovatif) jika tidak diberikan. Jika informasi penting tidak tersedia, gunakan formulasi netral atau minta informasi tambahan.',
   '["Jumlah investasi akan diumumkan perusahaan pada konferensi pers, Jumat (14/8/2026)."]', '["Perusahaan menginvestasikan Rp100 miliar untuk program ini."]', '["anti-hallucination","fakta"]', '[]', '[]', 1, 'active'),
  ('00000000-0000-4000-8000-00000000a217', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Ethics', 'Etika dan bahasa yang hati-hati',
   'Jangan menghasilkan konten fitnah, bohong, diskriminatif, menghakimi, menyerang kelompok berdasarkan SARA, atau merendahkan kelompok tertentu. Untuk kasus sensitif, gunakan bahasa hati-hati dan faktual. Jangan menyimpulkan seseorang bersalah hanya karena dituduh, dilaporkan, diperiksa, ditangkap, atau menjadi tersangka — gunakan status hukum yang tepat sesuai sumber.',
   '["Tersangka berinisial A kini ditahan di Polres Jakarta Selatan untuk pemeriksaan lebih lanjut."]', '["A terbukti bersalah melakukan penipuan terhadap puluhan korbannya."]', '["etika"]', '[]', '[]', 1, 'active'),
  ('00000000-0000-4000-8000-00000000a218', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Privacy', 'Lindungi data pribadi',
   'Jangan memasukkan data pribadi yang tidak diperlukan untuk memahami berita: nomor telepon, alamat pribadi, email pribadi, NIK, nomor identitas, atau data sensitif lainnya — kecuali relevan, diberikan secara sah, dan diperlukan oleh konteks.',
   '["Seorang warga melaporkan kehilangan sepeda motor ke Polsek Kebayoran Baru."]', '["Korban bernama Budi (45), warga Jalan Mawar No. 12, RT 05/RW 03, dengan nomor HP 0812xxxxxxx, melaporkan kehilangan sepeda motor."]', '["privasi","privacy"]', '[]', '[]', 1, 'active'),
  ('00000000-0000-4000-8000-00000000a219', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'AI Governance', 'Jangan mengklaim yang tidak dilakukan',
   'AI dilarang mengklaim telah mewawancarai seseorang, membaca dokumen tertentu, menghubungi narasumber, melakukan konfirmasi, fact-check, mengecek website/data resmi, atau verifikasi lapangan jika tidak dilakukan sistem. Konten AI bukan hasil verifikasi jurnalistik; jangan menulis seolah-olah data telah diverifikasi.',
   '["Menurut data yang diberikan perusahaan, jumlah pengguna mencapai 2 juta."]', '["Setelah melakukan verifikasi langsung, jumlah pengguna mencapai 2 juta."]', '["ai-governance","verifikasi"]', '[]', '[]', 1, 'active'),
  ('00000000-0000-4000-8000-00000000a220', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Quality Control', 'Editorial check sebelum output',
   'Lakukan pemeriksaan editorial internal sebelum output final: pastikan semua fakta berasal dari input/bahan, tidak ada fakta yang dikarang, lead menyampaikan inti berita, seluruh unsur 5W+1H yang tersedia dimanfaatkan, fakta dan opini terpisah, klaim subjektif ber-atribusi, tidak ada promosi berlebihan, tidak ada clickbait, tidak ada jargon yang tidak perlu, artikel tidak terlalu panjang, bahasa mudah dipahami, ejaan dan tanda baca benar, tidak ada kutipan yang dibuat-buat, dan hasil terasa seperti berita/press release. Perbaiki masalah apa pun sebelum menyerahkan output.',
   '["(Setelah menulis draft, periksa ulang setiap klaim terhadap bahan baku sebelum mengirim.)"]', '["(Mengirim draft langsung tanpa pemeriksaan ulang.)"]', '["quality","kontrol"]', '[]', '[]', 5, 'active'),
  ('00000000-0000-4000-8000-00000000a221', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Corporate Language', 'Hindari buzzword korporat',
   'Kurangi corporate buzzwords: "hadir sebagai solusi inovatif", "langkah strategis", "komitmen nyata", "transformasi digital", "ekosistem berkelanjutan", "memberikan dampak positif", "mendorong pertumbuhan", "memperkuat sinergi". Kata-kata ini boleh dipakai jika relevan dan berasal dari sumber, tetapi jangan otomatis hanya karena artikel membahas perusahaan. Utamakan fakta konkret.',
   '["Program tersebut menyediakan pelatihan digital bagi 500 pelaku UMKM."]', '["Langkah strategis ini menjadi bentuk komitmen nyata perusahaan dalam mendorong transformasi digital."]', '["jargon","buzzword"]', '[]', '[]', 6, 'active'),
  ('00000000-0000-4000-8000-00000000a222', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Names & Titles', 'Nama dan jabatan yang benar',
   'Pertahankan nama resmi dan jangan membuat variasi sembarangan. Penyebutan pertama: Nama Lengkap, Jabatan (mis. "Rama Angriawan, Direktur PT XYZ, mengatakan..."). Selanjutnya boleh "Rama" atau "dia" jika konteks jelas. Jangan salah mengganti gender/pronomina.',
   '["Rama Angriawan, Direktur PT XYZ, mengatakan program ini akan berjalan selama satu tahun."]', '["Bapak Rama, direktur perusahaan tersebut, bilang programnya jalan satu tahun."]', '["nama","jabatan"]', '[]', '[]', 3, 'active'),
  ('00000000-0000-4000-8000-00000000a223', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Foreign Terms', 'Utamakan padanan Indonesia',
   'Jika ada istilah asing dengan padanan Indonesia yang umum digunakan, prioritaskan bahasa Indonesia. Istilah teknis yang lazim boleh dipertahankan; jangan dipaksakan diterjemahkan jika hasilnya tidak natural. Jika istilah asing penting dan baru pertama kali digunakan, boleh diberi padanan dalam tanda kurung.',
   '["Perusahaan membuka pusat layanan (customer service) baru di Surabaya."]', '["Perusahaan membuka customer service center baru di Surabaya untuk memberikan best experience kepada customer."]', '["istilah"]', '[]', '[]', 6, 'active'),
  ('00000000-0000-4000-8000-00000000a224', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Digital Style', 'Gaya untuk pembaca digital',
   'Tulis untuk pembaca media digital Indonesia: inti cepat dipahami, paragraf relatif pendek, informasi utama lebih awal, hindari intro panjang, hindari repetisi, jangan mengulang judul di setiap paragraf, jangan memaksakan keyword, jangan membuat artikel terasa seperti SEO article generik. Variasikan struktur paragraf secara natural.',
   '["Kementerian ESDM menargetkan 10.000 SPKLU baru pada 2027. Hingga Juli 2026, baru 4.200 unit terpasang."]', '["Di era digital yang serba cepat ini, banyak hal berubah dengan pesat. Perubahan ini membawa berbagai dampak yang signifikan..."]', '["digital","gaya"]', '[]', '[]', 5, 'active')
ON CONFLICT (id) DO NOTHING;



ALTER TABLE ONLY public.media_package_items
    ADD CONSTRAINT media_package_items_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.media_packages(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.media_package_items
    ADD CONSTRAINT media_package_items_media_id_fkey FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.auto_replace_configs
    ADD CONSTRAINT fk_auto_replace_configs_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.auto_replace_configs
    ADD CONSTRAINT fk_auto_replace_configs_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.auto_replace_configs
    ADD CONSTRAINT fk_auto_replace_configs_updater FOREIGN KEY (updated_by) REFERENCES public.users(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.notification_webhooks
    ADD CONSTRAINT fk_notification_webhooks_user FOREIGN KEY (user_id) REFERENCES public.users (id);
ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT fk_notifications_user FOREIGN KEY (user_id) REFERENCES public.users (id);
ALTER TABLE ONLY public.credit_wallets
    ADD CONSTRAINT fk_credit_wallets_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.credit_ledger_entries
    ADD CONSTRAINT fk_credit_ledger_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.content_intakes
    ADD CONSTRAINT fk_content_intakes_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);
ALTER TABLE ONLY public.content_intakes
    ADD CONSTRAINT fk_content_intakes_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.media_content_standards
    ADD CONSTRAINT fk_media_content_standards_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.media_content_standards
    ADD CONSTRAINT fk_media_content_standards_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.publishing_api_keys
    ADD CONSTRAINT fk_publishing_api_keys_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.publishing_api_keys
    ADD CONSTRAINT fk_publishing_api_keys_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.publishing_api_keys
    ADD CONSTRAINT fk_publishing_api_keys_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.publishing_callback_logs
    ADD CONSTRAINT fk_publishing_callback_logs_key FOREIGN KEY (api_key_id) REFERENCES public.publishing_api_keys(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.publishing_callback_logs
    ADD CONSTRAINT fk_publishing_callback_logs_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.pr_scoring_profiles
    ADD CONSTRAINT fk_pr_scoring_profiles_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.pr_campaign_scores
    ADD CONSTRAINT fk_pr_campaign_scores_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.pr_campaign_scores
    ADD CONSTRAINT fk_pr_campaign_scores_profile FOREIGN KEY (profile_id) REFERENCES public.pr_scoring_profiles(id);
ALTER TABLE ONLY public.pr_campaign_scores
    ADD CONSTRAINT fk_pr_campaign_scores_calculator FOREIGN KEY (calculated_by) REFERENCES public.users(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.pr_metric_results
    ADD CONSTRAINT fk_pr_metric_results_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.pr_message_analyses
    ADD CONSTRAINT fk_pr_message_analyses_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.pr_message_analyses
    ADD CONSTRAINT fk_pr_message_analyses_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.pr_sentiment_analyses
    ADD CONSTRAINT fk_pr_sentiment_analyses_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.pr_sentiment_analyses
    ADD CONSTRAINT fk_pr_sentiment_analyses_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.pr_publication_verifications
    ADD CONSTRAINT fk_pr_verifications_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.pr_publication_verifications
    ADD CONSTRAINT fk_pr_verifications_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;
ALTER TABLE ONLY public.pr_media_rules
    ADD CONSTRAINT fk_pr_media_rules_updater FOREIGN KEY (updated_by) REFERENCES public.users(id) ON DELETE SET NULL;

-- ====================================================================
-- Tables from incremental migrations not present in the pg_dump baseline
-- (kept in sync with apps/web/src/db/migrations/*.sql)
-- ====================================================================
-- from 042_publication_plans.sql
CREATE TABLE IF NOT EXISTS public.publication_plans (
  id          VARCHAR(36) PRIMARY KEY,
  brand_id    VARCHAR(36) NOT NULL REFERENCES public.brands(id) ON DELETE CASCADE,
  name        VARCHAR(160) NOT NULL,
  document_id VARCHAR(36) NOT NULL REFERENCES public.content_documents(id),
  cadence     VARCHAR(20) NOT NULL DEFAULT 'daily' CHECK (cadence IN ('daily')),
  start_at    DATE NOT NULL,
  end_at      DATE,
  time_of_day TIME NOT NULL DEFAULT '09:00',
  timezone    VARCHAR(64) NOT NULL DEFAULT 'Asia/Jakarta',
  status      VARCHAR(20) NOT NULL DEFAULT 'active'
              CHECK (status IN ('draft','active','paused','completed','cancelled')),
  created_by  VARCHAR(36),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT ck_publication_plans_dates CHECK (end_at IS NULL OR end_at >= start_at)
);;

-- from 042_publication_plans.sql
CREATE TABLE IF NOT EXISTS public.publication_plan_media (
  plan_id  VARCHAR(36) NOT NULL REFERENCES public.publication_plans(id) ON DELETE CASCADE,
  media_id VARCHAR(36) NOT NULL REFERENCES public.media(id) ON DELETE CASCADE,
  position INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (plan_id, media_id)
);;

-- from 042_publication_plans.sql
CREATE TABLE IF NOT EXISTS public.publication_plan_occurrences (
  id             VARCHAR(36) PRIMARY KEY,
  plan_id        VARCHAR(36) NOT NULL REFERENCES public.publication_plans(id) ON DELETE CASCADE,
  media_id       VARCHAR(36) NOT NULL REFERENCES public.media(id),
  scheduled_at   TIMESTAMPTZ NOT NULL,
  status         VARCHAR(20) NOT NULL DEFAULT 'scheduled'
                 CHECK (status IN ('scheduled','materialized','skipped','failed')),
  publication_id VARCHAR(36),
  error_message  TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (plan_id, media_id, scheduled_at)
);;

CREATE INDEX IF NOT EXISTS ix_publication_plans_brand ON public.publication_plans (brand_id, status);

CREATE INDEX IF NOT EXISTS ix_publication_plans_due ON public.publication_plans (status, start_at);

CREATE INDEX IF NOT EXISTS ix_plan_occurrences_due
  ON public.publication_plan_occurrences (status, scheduled_at);

-- from 052_content_type_standards.sql
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
);;

-- from 052_content_type_standards.sql
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
);;

-- from 052_content_type_standards.sql
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
);;

CREATE UNIQUE INDEX IF NOT EXISTS uq_content_type_std_ver_active
    ON public.content_type_standard_versions (standard_id) WHERE (status = 'active'::character varying);

CREATE INDEX IF NOT EXISTS idx_content_type_std_chunks_version ON public.content_type_standard_chunks (version_id);

CREATE INDEX IF NOT EXISTS idx_content_type_std_chunks_code ON public.content_type_standard_chunks (content_type_code);

-- from 053_ai_model_knowledge.sql
CREATE TABLE IF NOT EXISTS public.ai_model_knowledge (
  id VARCHAR(36) PRIMARY KEY,
  model_id VARCHAR(120) NOT NULL UNIQUE,
  model_name VARCHAR(150),
  provider VARCHAR(60),
  reasoning_level VARCHAR(20) DEFAULT 'sedang', -- 'rendah' | 'sedang' | 'tinggi'
  summary TEXT,
  characteristics TEXT,
  suitable_for TEXT,
  unsuitable_for TEXT,
  guidelines TEXT,
  limitations TEXT,
  editorial_context TEXT,
  status VARCHAR(20) DEFAULT 'active', -- 'active' | 'draft' | 'archived'
  chunk_count INTEGER DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);;

-- from 053_ai_model_knowledge.sql
CREATE TABLE IF NOT EXISTS public.ai_model_knowledge_chunks (
  id VARCHAR(36) PRIMARY KEY,
  knowledge_id VARCHAR(36) NOT NULL REFERENCES public.ai_model_knowledge(id) ON DELETE CASCADE,
  model_id VARCHAR(120) NOT NULL,
  section VARCHAR(60) NOT NULL,
  chunk_index INTEGER NOT NULL DEFAULT 0,
  content TEXT NOT NULL,
  embedding JSONB,
  priority INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_ai_model_knowledge_chunks UNIQUE (knowledge_id, section, chunk_index)
);;

CREATE INDEX IF NOT EXISTS idx_ai_model_knowledge_model ON public.ai_model_knowledge(model_id);

CREATE INDEX IF NOT EXISTS idx_ai_model_knowledge_status ON public.ai_model_knowledge(status);

CREATE INDEX IF NOT EXISTS idx_ai_model_chunks_model ON public.ai_model_knowledge_chunks(model_id);

CREATE INDEX IF NOT EXISTS idx_ai_model_chunks_section ON public.ai_model_knowledge_chunks(section);

-- from 055_partner_documents.sql
CREATE TABLE IF NOT EXISTS public.partner_documents (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    name character varying(255) NOT NULL,
    type character varying(10) DEFAULT 'pdf'::character varying NOT NULL,
    size_bytes integer DEFAULT 0 NOT NULL,
    storage_key character varying(500) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    note character varying(500),
    uploaded_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    reviewed_at timestamp(6) with time zone,
    CONSTRAINT partner_documents_pkey PRIMARY KEY (id),
    CONSTRAINT ck_partner_documents_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'verified'::character varying, 'revision'::character varying])::text[])))
);;

CREATE INDEX IF NOT EXISTS ix_partner_documents_partner
    ON public.partner_documents (partner_id, uploaded_at DESC);

-- from 057_credit_topup_reports.sql
CREATE TABLE IF NOT EXISTS public.credit_topup_reports (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    claimed_tc integer NOT NULL,
    amount_idr integer,
    payment_reference character varying(200),
    description text NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    review_note text,
    reviewed_by character(36),
    reviewed_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT credit_topup_reports_pkey PRIMARY KEY (id),
    CONSTRAINT ck_credit_topup_report_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'approved'::character varying, 'rejected'::character varying])::text[]))),
    CONSTRAINT ck_credit_topup_report_claimed CHECK (claimed_tc > 0),
    CONSTRAINT fk_credit_topup_reports_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE,
    CONSTRAINT fk_credit_topup_reports_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE
);;

CREATE INDEX IF NOT EXISTS ix_credit_topup_reports_brand
    ON public.credit_topup_reports (brand_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_credit_topup_reports_status
    ON public.credit_topup_reports (status, created_at DESC);

-- from 071_media_pricing_gsc.sql
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS pricing_type character varying(10) NOT NULL DEFAULT 'nett'::character varying;
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS price_status character varying(20) NOT NULL DEFAULT 'menunggu_review'::character varying;
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS negotiated_price numeric(14,2);
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_media_pricing_type') THEN ALTER TABLE public.media ADD CONSTRAINT ck_media_pricing_type CHECK (pricing_type IN ('nett','nego')); END IF; END $$;
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_media_price_status') THEN ALTER TABLE public.media ADD CONSTRAINT ck_media_price_status CHECK (price_status IN ('menunggu_review','disetujui','negosiasi','ditolak','final')); END IF; END $$;
CREATE TABLE IF NOT EXISTS public.media_price_history (id character(36) NOT NULL, media_id character(36) NOT NULL, actor_id character(36), from_price numeric(14,2), to_price numeric(14,2), from_pricing_type character varying(10), to_pricing_type character varying(10), from_price_status character varying(20), to_price_status character varying(20), note character varying(1000), created_at timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6), CONSTRAINT media_price_history_pkey PRIMARY KEY (id));
CREATE INDEX IF NOT EXISTS ix_media_price_history_media_created ON public.media_price_history USING btree (media_id, created_at DESC);
CREATE TABLE IF NOT EXISTS public.gsc_connections (id character(36) NOT NULL, partner_id character(36) NOT NULL, provider character varying(20) NOT NULL DEFAULT 'google', email character varying(320), sealed_refresh_token text, scope character varying(500), status character varying(20) NOT NULL DEFAULT 'pending', created_at timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6), updated_at timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6), CONSTRAINT gsc_connections_pkey PRIMARY KEY (id), CONSTRAINT uq_gsc_connections_partner UNIQUE (partner_id));
CREATE TABLE IF NOT EXISTS public.gsc_properties (id character(36) NOT NULL, connection_id character(36) NOT NULL, site_url character varying(500) NOT NULL, permission_level character varying(20), created_at timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6), CONSTRAINT gsc_properties_pkey PRIMARY KEY (id), CONSTRAINT uq_gsc_properties_connection_site UNIQUE (connection_id, site_url));
CREATE TABLE IF NOT EXISTS public.gsc_snapshots (id character(36) NOT NULL, property_id character(36) NOT NULL, media_id character(36), period_days integer NOT NULL, clicks integer NOT NULL DEFAULT 0, impressions integer NOT NULL DEFAULT 0, ctr numeric(6,4) NOT NULL DEFAULT 0, position numeric(6,2) NOT NULL DEFAULT 0, fetched_at timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6), CONSTRAINT gsc_snapshots_pkey PRIMARY KEY (id));
CREATE INDEX IF NOT EXISTS ix_gsc_snapshots_property_fetched ON public.gsc_snapshots USING btree (property_id, fetched_at DESC);
CREATE INDEX IF NOT EXISTS ix_gsc_snapshots_media_fetched ON public.gsc_snapshots USING btree (media_id, fetched_at DESC);
-- 072 — Platform GSC OAuth config
CREATE TABLE IF NOT EXISTS public.platform_gsc_config (
  id                text NOT NULL DEFAULT 'default',
  client_id         text,
  sealed_client_secret text,
  redirect_uri      text,
  updated_by        character(36),
  updated_at        timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT platform_gsc_config_pkey PRIMARY KEY (id),
  CONSTRAINT ck_platform_gsc_config_id CHECK (id = 'default')
);
CREATE TABLE IF NOT EXISTS public.gsc_oauth_states (
  id          character(36) NOT NULL,
  partner_id  character(36) NOT NULL,
  state       character varying(128) NOT NULL,
  code_verifier character varying(128),
  created_at  timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT gsc_oauth_states_pkey PRIMARY KEY (id),
  CONSTRAINT uq_gsc_oauth_state UNIQUE (state)
);
CREATE INDEX IF NOT EXISTS ix_gsc_oauth_states_created ON public.gsc_oauth_states USING btree (created_at DESC);

-- 080 — Media SEO metrics cache + platform RapidAPI credential
CREATE TABLE IF NOT EXISTS public.media_seo_metrics (
  media_id    character(36) NOT NULL,
  da          smallint,
  pa          smallint,
  spam_score  smallint,
  ai_trust_score smallint,
  ai_citation boolean,
  ai_citation_checked_at timestamp(6) with time zone,
  checked_at  timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  source      character varying(40) NOT NULL DEFAULT 'rapidapi-domain-metrics',
  created_at  timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at  timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT media_seo_metrics_pkey PRIMARY KEY (media_id),
  CONSTRAINT fk_media_seo_metrics_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE,
  CONSTRAINT ck_media_seo_metrics_da CHECK (da IS NULL OR da BETWEEN 0 AND 100),
  CONSTRAINT ck_media_seo_metrics_pa CHECK (pa IS NULL OR pa BETWEEN 0 AND 100),
  CONSTRAINT ck_media_seo_metrics_spam CHECK (spam_score IS NULL OR spam_score BETWEEN 0 AND 100),
  CONSTRAINT ck_media_seo_metrics_ai_trust CHECK (ai_trust_score IS NULL OR ai_trust_score BETWEEN 0 AND 100),
  CONSTRAINT ck_media_seo_metrics_value CHECK (da IS NOT NULL OR pa IS NOT NULL OR spam_score IS NOT NULL OR ai_trust_score IS NOT NULL OR ai_citation IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS ix_media_seo_metrics_checked ON public.media_seo_metrics (checked_at DESC);

CREATE TABLE IF NOT EXISTS public.platform_seo_config (
  id              text NOT NULL DEFAULT 'default',
  sealed_api_key  text,
  api_key_hint    character varying(16),
  sealed_similarweb_api_key text,
  sealed_databee_token text,
  sealed_serpens_api_key text,
  updated_by      character(36),
  updated_at      timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT platform_seo_config_pkey PRIMARY KEY (id),
  CONSTRAINT ck_platform_seo_config_id CHECK (id = 'default')
);

CREATE TABLE IF NOT EXISTS public.media_similarweb_snapshots (
  id              character(36) NOT NULL,
  media_id        character(36) NOT NULL,
  insights        jsonb NOT NULL,
  snapshot_date   date,
  created_by      character(36),
  created_at      timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT media_similarweb_snapshots_pkey PRIMARY KEY (id),
  CONSTRAINT fk_media_similarweb_media FOREIGN KEY (media_id)
    REFERENCES public.media (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS ix_media_similarweb_media
  ON public.media_similarweb_snapshots (media_id, created_at DESC);

-- From migrations/133_registration_guard.sql (Registration Abuse Guard)
CREATE TABLE IF NOT EXISTS public.registration_history (
  id                character(36) NOT NULL,
  user_id           character(36) REFERENCES public.users (id) ON DELETE SET NULL,
  registration_source varchar(20) NOT NULL,
  email_hash        character varying(64),
  google_sub_hash   character varying(64),
  ip_hash           character varying(64),
  device_hash       character varying(64),
  domain            character varying(253),
  registered_at     timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
  risk_score        integer NOT NULL DEFAULT 0,
  risk_level        character varying(20) NOT NULL DEFAULT 'low',
  reason_codes      jsonb NOT NULL DEFAULT '[]'::jsonb,
  CONSTRAINT registration_history_pkey PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS ix_registration_history_email_hash
  ON public.registration_history (email_hash);

CREATE INDEX IF NOT EXISTS ix_registration_history_google_sub_hash
  ON public.registration_history (google_sub_hash);

CREATE INDEX IF NOT EXISTS ix_registration_history_ip_hash_time
  ON public.registration_history (ip_hash, registered_at DESC);

CREATE INDEX IF NOT EXISTS ix_registration_history_device_hash_time
  ON public.registration_history (device_hash, registered_at DESC);

CREATE TABLE IF NOT EXISTS public.registration_guard_config (
  id          integer NOT NULL DEFAULT 1,
  scores      jsonb NOT NULL,
  thresholds  jsonb NOT NULL,
  rate_limits jsonb NOT NULL,
  updated_at  timestamp(6) with time zone NOT NULL DEFAULT NOW(),
  CONSTRAINT registration_guard_config_pkey PRIMARY KEY (id),
  CONSTRAINT registration_guard_config_id_check CHECK (id = 1)
);

INSERT INTO public.registration_guard_config (id, scores, thresholds, rate_limits)
VALUES (
  1,
  '{"disposable":50,"known_abusive_domain":70,"very_new_domain":20,"workspace":10,"ip_many_registrations":20,"device_many_registrations":25,"recent_registration":100,"suspicious_pattern":30,"previously_blocked":70,"existing_google_sub":100,"mx_missing":15}'::jsonb,
  '{"verify":30,"review":60,"block":80,"critical":110}'::jsonb,
  '{"register_email_per_hour":5,"register_device_per_hour":10,"register_ip_per_minute":10,"google_callback_ip_per_minute":30}'::jsonb
)
ON CONFLICT (id) DO NOTHING;

-- From migrations/139_media_ai_summary_events.sql (tracking Kesimpulan Website)
CREATE TABLE IF NOT EXISTS public.media_ai_summary_events (
    id                character(36) NOT NULL,
    media_id          character(36) NOT NULL REFERENCES public.media (id) ON DELETE CASCADE,
    media_name        character varying(160),
    website_url       character varying(500),
    status            character varying(20) NOT NULL,
    pages_tried       jsonb NOT NULL DEFAULT '[]'::jsonb,
    error_message     character varying(1000),
    attempt           integer NOT NULL DEFAULT 0,
    created_at        timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT media_ai_summary_events_pkey PRIMARY KEY (id),
    CONSTRAINT ck_media_ai_summary_events_status CHECK (((status)::text = ANY ((ARRAY['queued'::character varying, 'success'::character varying, 'failed'::character varying])::text[])))
);

CREATE INDEX IF NOT EXISTS ix_mase_media_time
  ON public.media_ai_summary_events (media_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_mase_status_time
  ON public.media_ai_summary_events (status, created_at DESC);

-- From migrations/140_payment_idempotency.sql (proteksi double request/payment)
CREATE TABLE IF NOT EXISTS public.payment_webhook_events (
    id                      character(36) NOT NULL,
    gateway_provider        character varying(20) NOT NULL,
    external_transaction_id character varying(120) NOT NULL,
    status                  character varying(20) NOT NULL,
    payload                 jsonb,
    created_at              timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT payment_webhook_events_pkey PRIMARY KEY (id),
    CONSTRAINT uq_payment_webhook_event UNIQUE (gateway_provider, external_transaction_id, status)
);
