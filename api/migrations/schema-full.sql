--
-- PostgreSQL database dump
--


-- Dumped from database version 16.15 (Postgres.app)
-- Dumped by pg_dump version 16.15 (Postgres.app)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: _random_short_id(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public._random_short_id() RETURNS text
    LANGUAGE sql
    AS $$
  SELECT string_agg(substr('23456789abcdefghjkmnpqrstuvwxyz',
                           (floor(random() * 31) + 1)::int, 1), '')
    FROM generate_series(1, 6);
$$;


--
-- Name: enforce_client_package_media_limit(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.enforce_client_package_media_limit() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE
  pkg_source VARCHAR(20);
  media_count INTEGER;
BEGIN
  SELECT source INTO pkg_source FROM public.media_packages WHERE id = NEW.package_id;
  IF pkg_source = 'client' THEN
    SELECT COUNT(*) INTO media_count FROM public.media_package_items WHERE package_id = NEW.package_id;
    IF media_count >= 5 THEN
      RAISE EXCEPTION 'Paket media custom maksimal 5 media (client package limit)';
    END IF;
  END IF;
  RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: affiliate_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.affiliate_settings (
    partner_id character(36) NOT NULL,
    commission_rate_bp integer DEFAULT 100 NOT NULL,
    cpa_rate integer DEFAULT 0 NOT NULL,
    entitlements jsonb DEFAULT '{"seo_tools": true, "ai_premium": true, "client_workspace": true, "keyword_research": true}'::jsonb NOT NULL,
    updated_by character(36),
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    cpa_tier_threshold integer DEFAULT 1000 NOT NULL,
    cpa_tier1_rate integer DEFAULT 10000 NOT NULL,
    cpa_tier2_rate integer DEFAULT 5000 NOT NULL,
    CONSTRAINT affiliate_settings_commission_rate_bp_check CHECK (((commission_rate_bp >= 0) AND (commission_rate_bp <= 10000))),
    CONSTRAINT affiliate_settings_cpa_rate_check CHECK ((cpa_rate >= 0))
);


--
-- Name: ai_model_knowledge; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_model_knowledge (
    id character varying(36) NOT NULL,
    model_id character varying(120) NOT NULL,
    model_name character varying(150),
    provider character varying(60),
    reasoning_level character varying(20) DEFAULT 'sedang'::character varying,
    summary text,
    characteristics text,
    suitable_for text,
    unsuitable_for text,
    guidelines text,
    limitations text,
    editorial_context text,
    status character varying(20) DEFAULT 'active'::character varying,
    chunk_count integer DEFAULT 0,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: ai_model_knowledge_chunks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_model_knowledge_chunks (
    id character varying(36) NOT NULL,
    knowledge_id character varying(36) NOT NULL,
    model_id character varying(120) NOT NULL,
    section character varying(60) NOT NULL,
    chunk_index integer DEFAULT 0 NOT NULL,
    content text NOT NULL,
    embedding jsonb,
    priority integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: analytics_daily; Type: TABLE; Schema: public; Owner: -
--

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
    CONSTRAINT ck_analytics_daily_event_type CHECK (((event_type)::text = ANY (ARRAY[('article_view'::character varying)::text, ('article_scroll'::character varying)::text, ('outbound_click'::character varying)::text, ('cta_click'::character varying)::text, ('tracking_link_click'::character varying)::text])))
);


--
-- Name: audit_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.audit_logs (
    id character(36) NOT NULL,
    brand_id character(36),
    actor_user_id character(36),
    action character varying(100) NOT NULL,
    target_type character varying(50),
    target_id character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    ip_address character varying(45),
    user_agent character varying(500),
    detail jsonb
);


--
-- Name: auth_rate_limits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.auth_rate_limits (
    id character(36) NOT NULL,
    bucket character varying(120) NOT NULL,
    count integer DEFAULT 0 NOT NULL,
    window_start timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: auto_replace_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.auto_replace_configs (
    id character(36) NOT NULL,
    brand_id character(36),
    enabled boolean DEFAULT true NOT NULL,
    max_replaces smallint DEFAULT 3 NOT NULL,
    created_by character(36),
    updated_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_auto_replace_max_replaces CHECK (((max_replaces >= 1) AND (max_replaces <= 10)))
);


--
-- Name: blogger_oauth_states; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.blogger_oauth_states (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    state character varying(200) NOT NULL,
    code_verifier character varying(200),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: brand_ai_quotas; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: brand_memberships; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.brand_memberships (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    role character varying(20) DEFAULT 'writer'::character varying NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: brand_pr_standards; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.brand_pr_standards (
    brand_id character(36) NOT NULL,
    rate_per_1k_reach integer,
    rate_tier1 integer,
    rate_tier2 integer,
    rate_tier3 integer,
    updated_by character(36),
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT ck_brand_pr_rates CHECK ((((rate_per_1k_reach IS NULL) OR (rate_per_1k_reach >= 0)) AND ((rate_tier1 IS NULL) OR (rate_tier1 >= 0)) AND ((rate_tier2 IS NULL) OR (rate_tier2 >= 0)) AND ((rate_tier3 IS NULL) OR (rate_tier3 >= 0))))
);


--
-- Name: brand_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.brand_profiles (
    brand_id character(36) NOT NULL,
    positioning text,
    value_proposition text,
    target_audience text,
    products_services text,
    differentiation text,
    default_cta text,
    default_target_words integer DEFAULT 1000 NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL,
    CONSTRAINT ck_brand_profile_words CHECK (((default_target_words >= 300) AND (default_target_words <= 10000)))
);


--
-- Name: brands; Type: TABLE; Schema: public; Owner: -
--

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
    CONSTRAINT ck_brands_ownership_type CHECK (((ownership_type)::text = ANY (ARRAY[('internal'::character varying)::text, ('client'::character varying)::text]))),
    CONSTRAINT ck_brands_status CHECK (((status)::text = ANY (ARRAY[('draft'::character varying)::text, ('active'::character varying)::text, ('archived'::character varying)::text])))
);


--
-- Name: bug_reports; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: campaign_media_items; Type: TABLE; Schema: public; Owner: -
--

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
    CONSTRAINT ck_campaign_media_status CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('article_ready'::character varying)::text, ('needs_rewrite'::character varying)::text, ('rewriting'::character varying)::text, ('angle_select'::character varying)::text, ('ready'::character varying)::text, ('processing'::character varying)::text, ('published'::character varying)::text, ('failed'::character varying)::text, ('cancelled'::character varying)::text]))),
    CONSTRAINT ck_campaign_media_variant_type CHECK (((variant_type)::text = ANY (ARRAY[('original'::character varying)::text, ('paraphrase'::character varying)::text, ('angle'::character varying)::text])))
);


--
-- Name: campaign_order_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.campaign_order_seq
    START WITH 1000
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: campaign_report_shares; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.campaign_report_shares (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    token character(64) NOT NULL,
    created_by character(36),
    revoked_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: campaigns; Type: TABLE; Schema: public; Owner: -
--

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
    content_intake_id character(36),
    campaign_flow_status character varying(30) DEFAULT 'intake'::character varying NOT NULL,
    media_type character varying(20),
    selected_media_id character(36),
    editorial_standard_snapshot jsonb,
    media_locked_at timestamp(6) with time zone,
    business_profile jsonb,
    hyperlink_config jsonb,
    trigger_keywords text[],
    budget_strategy character varying(20),
    budget_config jsonb,
    strategy_brief jsonb,
    timeline_plan jsonb,
    taut_status character varying(20),
    profiler_summary jsonb,
    category_input jsonb,
    archived_at timestamp with time zone,
    tracking_mode text DEFAULT 'direct'::text NOT NULL,
    primary_url text,
    primary_anchor character varying(120),
    backup_url text,
    backup_anchor character varying(120),
    deleted_at timestamp(6) with time zone,
    spokepersons jsonb,
    order_code character varying(40),
    taut_entry_mode character varying(10),
    CONSTRAINT campaigns_tracking_mode_check CHECK ((tracking_mode = ANY (ARRAY['direct'::text, 'tauthub'::text]))),
    CONSTRAINT ck_campaign_budget CHECK ((budget >= (0)::numeric)),
    CONSTRAINT ck_campaign_media_type CHECK (((media_type)::text = ANY (ARRAY[('blogger'::character varying)::text, ('mainstream'::character varying)::text, ('kombinasi'::character varying)::text]))),
    CONSTRAINT ck_campaign_quota CHECK ((article_quota >= 0)),
    CONSTRAINT ck_campaign_status CHECK (((status)::text = ANY (ARRAY[('draft'::character varying)::text, ('active'::character varying)::text, ('paused'::character varying)::text, ('completed'::character varying)::text, ('cancelled'::character varying)::text]))),
    CONSTRAINT ck_campaigns_budget_strategy CHECK (((budget_strategy)::text = ANY (ARRAY[('efisien'::character varying)::text, ('optimal'::character varying)::text, ('brutal'::character varying)::text]))),
    CONSTRAINT ck_campaigns_taut_entry_mode CHECK (((taut_entry_mode)::text = ANY (ARRAY[('ai'::character varying)::text, ('manual'::character varying)::text]))),
    CONSTRAINT ck_campaigns_taut_status CHECK (((taut_status)::text = ANY (ARRAY[('draft'::character varying)::text, ('profiling'::character varying)::text, ('profiled'::character varying)::text, ('materials'::character varying)::text, ('budget'::character varying)::text, ('placement'::character varying)::text, ('production'::character varying)::text, ('review'::character varying)::text, ('scheduled'::character varying)::text, ('submitted'::character varying)::text])))
);


--
-- Name: chat_learnings; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: cms_connections; Type: TABLE; Schema: public; Owner: -
--

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
    enc_key_ver integer DEFAULT 0 NOT NULL,
    CONSTRAINT ck_cms_connections_owner CHECK (((brand_id IS NOT NULL) <> (partner_id IS NOT NULL)))
);


--
-- Name: content_ai_analyses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.content_ai_analyses (
    id character(36) NOT NULL,
    content_intake_id character(36) NOT NULL,
    campaign_id character(36),
    media_id character(36),
    media_type character varying(20),
    editorial_score integer,
    diagnosis text,
    strengths jsonb DEFAULT '[]'::jsonb NOT NULL,
    violations jsonb DEFAULT '[]'::jsonb NOT NULL,
    content_type_detected character varying(80),
    editorial_fit_score integer,
    readability integer,
    clarity integer,
    search_intent character varying(120),
    structure_score integer,
    potential_value character varying(200),
    headline_suggestions jsonb DEFAULT '[]'::jsonb NOT NULL,
    revised_draft text,
    standard_source character varying(200),
    standard_version character varying(80),
    fallback_type character varying(50),
    content_hash character varying(64),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT content_ai_analyses_clarity_check CHECK (((clarity >= 0) AND (clarity <= 100))),
    CONSTRAINT content_ai_analyses_editorial_fit_score_check CHECK (((editorial_fit_score >= 0) AND (editorial_fit_score <= 100))),
    CONSTRAINT content_ai_analyses_editorial_score_check CHECK (((editorial_score >= 0) AND (editorial_score <= 100))),
    CONSTRAINT content_ai_analyses_readability_check CHECK (((readability >= 0) AND (readability <= 100))),
    CONSTRAINT content_ai_analyses_structure_score_check CHECK (((structure_score >= 0) AND (structure_score <= 100)))
);


--
-- Name: content_batch_items; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: content_batches; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: content_briefs; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: content_documents; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: content_intake_types; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.content_intake_types (
    id character(36) NOT NULL,
    code character varying(50) NOT NULL,
    name character varying(120) NOT NULL,
    description text,
    questions jsonb DEFAULT '[]'::jsonb NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_content_intake_type_status CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('archived'::character varying)::text])))
);


--
-- Name: content_intakes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.content_intakes (
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
    key_messages jsonb DEFAULT '[]'::jsonb NOT NULL,
    CONSTRAINT ck_content_intake_mode CHECK (((mode)::text = ANY (ARRAY[('bahan'::character varying)::text, ('artikel'::character varying)::text, ('keduanya'::character varying)::text, ('rekomendasi'::character varying)::text]))),
    CONSTRAINT ck_content_intake_status CHECK (((status)::text = ANY (ARRAY[('draft'::character varying)::text, ('ready'::character varying)::text, ('job_created'::character varying)::text])))
);


--
-- Name: content_jobs; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: content_moderation_audits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.content_moderation_audits (
    id character(36) NOT NULL,
    entity_type character varying(20) NOT NULL,
    entity_id character(36) NOT NULL,
    brand_id character(36),
    content_hash character varying(64) NOT NULL,
    result_id character(36),
    status character varying(20) NOT NULL,
    risk_score integer NOT NULL,
    category character varying(30) NOT NULL,
    reason_code character varying(80) NOT NULL,
    signals jsonb DEFAULT '[]'::jsonb NOT NULL,
    moderation_version character varying(20) NOT NULL,
    model character varying(160),
    ai_called boolean DEFAULT false NOT NULL,
    token_input integer DEFAULT 0 NOT NULL,
    token_output integer DEFAULT 0 NOT NULL,
    token_total integer DEFAULT 0 NOT NULL,
    reviewed_by character(36),
    reviewed_at timestamp with time zone,
    review_decision character varying(20),
    review_note text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT ck_moderation_audit_entity CHECK (((entity_type)::text = ANY (ARRAY[('intake'::character varying)::text, ('document'::character varying)::text, ('publication'::character varying)::text, ('content_job'::character varying)::text]))),
    CONSTRAINT ck_moderation_audit_status CHECK (((status)::text = ANY (ARRAY[('safe'::character varying)::text, ('review_required'::character varying)::text, ('blocked'::character varying)::text]))),
    CONSTRAINT ck_moderation_review_decision CHECK (((review_decision IS NULL) OR ((review_decision)::text = ANY (ARRAY[('approved'::character varying)::text, ('rejected'::character varying)::text]))))
);


--
-- Name: content_moderation_results; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.content_moderation_results (
    id character(36) NOT NULL,
    content_hash character varying(64) NOT NULL,
    moderation_version character varying(20) NOT NULL,
    status character varying(20) NOT NULL,
    risk_score integer NOT NULL,
    category character varying(30) NOT NULL,
    confidence numeric(3,2) DEFAULT 0.85 NOT NULL,
    reason_code character varying(80) NOT NULL,
    signals jsonb DEFAULT '[]'::jsonb NOT NULL,
    model character varying(160),
    ai_called boolean DEFAULT false NOT NULL,
    token_input integer DEFAULT 0 NOT NULL,
    token_output integer DEFAULT 0 NOT NULL,
    token_total integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT ck_moderation_result_status CHECK (((status)::text = ANY (ARRAY[('safe'::character varying)::text, ('review_required'::character varying)::text, ('blocked'::character varying)::text])))
);


--
-- Name: content_type_standard_chunks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.content_type_standard_chunks (
    id character(36) NOT NULL,
    version_id character(36) NOT NULL,
    content_type_code character varying(50) NOT NULL,
    section character varying(60) NOT NULL,
    chunk_index integer NOT NULL,
    content text NOT NULL,
    embedding jsonb NOT NULL,
    priority smallint DEFAULT 3 NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: content_type_standard_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.content_type_standard_versions (
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
    CONSTRAINT ck_content_type_std_ver_status CHECK (((status)::text = ANY (ARRAY[('draft'::character varying)::text, ('active'::character varying)::text, ('archived'::character varying)::text])))
);


--
-- Name: content_type_standards; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.content_type_standards (
    id character(36) NOT NULL,
    content_type_code character varying(50) NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_content_type_standards_status CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('inactive'::character varying)::text])))
);


--
-- Name: content_versions; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: crawl_targets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.crawl_targets (
    media_id character(36) NOT NULL,
    website_url character varying(500),
    active boolean DEFAULT true NOT NULL,
    articles_7d integer,
    confidence character varying(50),
    source character varying(100),
    last_crawled_at timestamp(6) with time zone,
    last_status character varying(50),
    consecutive_failures integer DEFAULT 0 NOT NULL,
    last_error character varying(500),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_crawl_targets_failures CHECK ((consecutive_failures >= 0))
);


--
-- Name: crawler_credentials; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.crawler_credentials (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    media_id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    domain text NOT NULL,
    crawler_id text NOT NULL,
    secret_enc bytea NOT NULL,
    secret_nonce bytea NOT NULL,
    secret_tag bytea NOT NULL,
    secret_fingerprint character(16) NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    allowed_hosts text[] DEFAULT '{}'::text[] NOT NULL,
    allowed_paths text[] DEFAULT ARRAY['/*'::text] NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    rotated_at timestamp with time zone,
    revoked_at timestamp with time zone,
    last_successful_crawl timestamp with time zone,
    last_failed_crawl timestamp with time zone,
    fail_count integer DEFAULT 0 NOT NULL,
    consent_at timestamp with time zone,
    consent_doc_version text,
    CONSTRAINT crawler_credentials_status_check CHECK ((status = ANY (ARRAY['active'::text, 'rotating'::text, 'disabled'::text, 'revoked'::text])))
);


--
-- Name: crawler_diagnostics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.crawler_diagnostics (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    credential_id uuid,
    media_id character(36) NOT NULL,
    domain text NOT NULL,
    url text NOT NULL,
    http_status integer,
    cf_ray text,
    cf_mitigated text,
    server_header text,
    body_fingerprint character(64),
    verdict text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: crawler_nonces; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.crawler_nonces (
    nonce character(32) NOT NULL,
    crawler_id text NOT NULL,
    seen_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL
);


--
-- Name: crawler_request_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.crawler_request_log (
    id bigint NOT NULL,
    crawler_id text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: crawler_request_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.crawler_request_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: crawler_request_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.crawler_request_log_id_seq OWNED BY public.crawler_request_log.id;


--
-- Name: credit_ledger_entries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.credit_ledger_entries (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    ref_type character varying(20) NOT NULL,
    ref_id text,
    amount_tc integer NOT NULL,
    balance_after_tc integer NOT NULL,
    rate_idr integer NOT NULL,
    amount_idr integer,
    reason text,
    idempotency_key character varying(160) NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_credit_ledger_type CHECK (((ref_type)::text = ANY (ARRAY[('topup'::character varying)::text, ('reserve'::character varying)::text, ('release'::character varying)::text, ('capture'::character varying)::text, ('refund'::character varying)::text, ('adjustment'::character varying)::text])))
);


--
-- Name: credit_topup_reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.credit_topup_reports (
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
    CONSTRAINT ck_credit_topup_report_claimed CHECK ((claimed_tc > 0)),
    CONSTRAINT ck_credit_topup_report_status CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('approved'::character varying)::text, ('rejected'::character varying)::text])))
);


--
-- Name: credit_transactions; Type: TABLE; Schema: public; Owner: -
--

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
    CONSTRAINT ck_credit_tx_amount CHECK ((amount <> 0)),
    CONSTRAINT ck_credit_tx_reason CHECK (((reason)::text = ANY (ARRAY[('package.grant'::character varying)::text, ('publication.usage'::character varying)::text, ('publication.refund'::character varying)::text, ('adjustment'::character varying)::text])))
);


--
-- Name: credit_wallets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.credit_wallets (
    brand_id character(36) NOT NULL,
    available_tc integer DEFAULT 0 NOT NULL,
    reserved_tc integer DEFAULT 0 NOT NULL,
    captured_tc integer DEFAULT 0 NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_wallet_available CHECK ((available_tc >= 0)),
    CONSTRAINT ck_wallet_captured CHECK ((captured_tc >= 0)),
    CONSTRAINT ck_wallet_reserved CHECK ((reserved_tc >= 0))
);


--
-- Name: editorial_rule_sets; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.editorial_rule_sets (
    brand_id character(36) NOT NULL,
    required_rules jsonb NOT NULL,
    forbidden_rules jsonb NOT NULL,
    required_disclaimer text,
    citation_policy text
);


--
-- Name: email_deliveries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.email_deliveries (
    id character(36) NOT NULL,
    template character varying(120) NOT NULL,
    recipient_email character varying(200) NOT NULL,
    user_id character(36),
    reference_id character(36),
    event character varying(60),
    status character varying(20) DEFAULT 'queued'::character varying NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    max_attempts integer DEFAULT 3 NOT NULL,
    next_attempt_at timestamp with time zone DEFAULT now() NOT NULL,
    provider character varying(20) DEFAULT 'smtp'::character varying NOT NULL,
    provider_message_id character varying(200),
    error_code character varying(40),
    error_message character varying(500),
    sent_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    subject character varying(500),
    html text,
    dedupe_key character varying(255),
    CONSTRAINT email_deliveries_status_check CHECK (((status)::text = ANY (ARRAY[('queued'::character varying)::text, ('sending'::character varying)::text, ('sent'::character varying)::text, ('failed'::character varying)::text, ('rate_limited'::character varying)::text, ('skipped'::character varying)::text])))
);


--
-- Name: ga4_configs; Type: TABLE; Schema: public; Owner: -
--

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
    sealed_api_secret text,
    CONSTRAINT ck_ga4_consent_mode CHECK (((consent_mode)::text = ANY (ARRAY[('required'::character varying)::text, ('none'::character varying)::text]))),
    CONSTRAINT ck_ga4_measurement_id CHECK (((measurement_id)::text ~ '^G-[A-Z0-9]+$'::text))
);


--
-- Name: gsc_connections; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gsc_connections (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    provider character varying(20) DEFAULT 'google'::character varying NOT NULL,
    email character varying(320),
    sealed_refresh_token text,
    scope character varying(500),
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL,
    selected_property_id character(36),
    CONSTRAINT ck_gsc_connections_status CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('active'::character varying)::text, ('revoked'::character varying)::text, ('error'::character varying)::text])))
);


--
-- Name: gsc_oauth_states; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gsc_oauth_states (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    state character varying(128) NOT NULL,
    code_verifier character varying(128),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL
);


--
-- Name: gsc_properties; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gsc_properties (
    id character(36) NOT NULL,
    connection_id character(36) NOT NULL,
    site_url character varying(500) NOT NULL,
    permission_level character varying(20),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL
);


--
-- Name: gsc_snapshots; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gsc_snapshots (
    id character(36) NOT NULL,
    property_id character(36) NOT NULL,
    media_id character(36),
    period_days integer NOT NULL,
    clicks integer DEFAULT 0 NOT NULL,
    impressions integer DEFAULT 0 NOT NULL,
    ctr numeric(6,4) DEFAULT 0 NOT NULL,
    "position" numeric(6,2) DEFAULT 0 NOT NULL,
    fetched_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL,
    CONSTRAINT ck_gsc_snapshots_period CHECK ((period_days = ANY (ARRAY[7, 28, 30, 90])))
);


--
-- Name: invoices; Type: TABLE; Schema: public; Owner: -
--

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
    CONSTRAINT ck_invoice_amount CHECK ((amount >= (0)::numeric)),
    CONSTRAINT ck_invoice_status CHECK (((status)::text = ANY (ARRAY[('draft'::character varying)::text, ('issued'::character varying)::text, ('paid'::character varying)::text, ('void'::character varying)::text])))
);


--
-- Name: jobs; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: knowledge_base_rule_chunks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_base_rule_chunks (
    id character(36) NOT NULL,
    rule_id character(36) NOT NULL,
    chunk_index integer NOT NULL,
    content text NOT NULL,
    embedding jsonb NOT NULL
);


--
-- Name: knowledge_base_rules; Type: TABLE; Schema: public; Owner: -
--

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
    CONSTRAINT ck_knowledge_base_rules_priority CHECK (((priority >= 1) AND (priority <= 8))),
    CONSTRAINT ck_knowledge_base_rules_status CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('inactive'::character varying)::text])))
);


--
-- Name: knowledge_base_sources; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_base_sources (
    id character(36) NOT NULL,
    knowledge_base_id character(36) NOT NULL,
    source_name character varying(255) NOT NULL,
    source_url text,
    source_type character varying(30) DEFAULT 'regulation'::character varying NOT NULL,
    notes text,
    accessed_at date,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: knowledge_base_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_base_versions (
    id character(36) NOT NULL,
    knowledge_base_id character(36) NOT NULL,
    version integer NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    change_note character varying(255),
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_knowledge_base_versions_status CHECK (((status)::text = ANY (ARRAY[('draft'::character varying)::text, ('active'::character varying)::text, ('archived'::character varying)::text])))
);


--
-- Name: knowledge_bases; Type: TABLE; Schema: public; Owner: -
--

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
    CONSTRAINT ck_knowledge_bases_scope CHECK (((scope)::text = ANY (ARRAY[('global'::character varying)::text, ('media'::character varying)::text, ('brand'::character varying)::text]))),
    CONSTRAINT ck_knowledge_bases_status CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('inactive'::character varying)::text])))
);


--
-- Name: knowledge_chunks; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: knowledge_collections; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.knowledge_collections (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    name character varying(160) NOT NULL,
    description text,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_by character(36)
);


--
-- Name: knowledge_crawl_runs; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: knowledge_sources; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: master_prompt_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.master_prompt_versions (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    version integer NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    content text NOT NULL,
    change_note character varying(255),
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_master_prompt_versions_status CHECK (((status)::text = ANY (ARRAY[('draft'::character varying)::text, ('active'::character varying)::text, ('archived'::character varying)::text])))
);


--
-- Name: media; Type: TABLE; Schema: public; Owner: -
--

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
    adapter_url character varying(500),
    media_tier character varying(20) DEFAULT 'tier3'::character varying,
    media_quality_score smallint,
    media_quality_notes character varying(500),
    dewan_pers_verified boolean DEFAULT false NOT NULL,
    dewan_pers_note text,
    dewan_pers_cert_no character varying(200),
    dewan_pers_doc_key character varying(500),
    publication_mode character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    rejection_reason character varying(1000),
    reviewed_by character(36),
    reviewed_at timestamp(6) with time zone,
    pricing_type character varying(10) DEFAULT 'nett'::character varying NOT NULL,
    price_status character varying(20) DEFAULT 'menunggu_review'::character varying NOT NULL,
    negotiated_price numeric(14,2),
    custom_web_endpoint_url character varying(500),
    custom_web_api_key_sealed text,
    custom_web_status character varying(20) DEFAULT 'disconnected'::character varying NOT NULL,
    custom_web_last_test_at timestamp with time zone,
    custom_web_last_test_message character varying(500),
    blogger_credentials_sealed text,
    blogger_blog_id character varying(200),
    blogger_blog_name character varying(300),
    blogger_status character varying(20) DEFAULT 'disconnected'::character varying NOT NULL,
    blogger_last_test_at timestamp with time zone,
    blogger_last_test_message character varying(500),
    media_type character varying(20) DEFAULT 'mainstream'::character varying NOT NULL,
    allow_hyperlink boolean DEFAULT false NOT NULL,
    enc_key_ver integer DEFAULT 0 NOT NULL,
    max_links integer DEFAULT 1,
    ai_summary text,
    ai_summary_generated_at timestamp(6) with time zone,
    domain_verify_token character varying(64),
    domain_verified boolean DEFAULT false NOT NULL,
    domain_verified_at timestamp with time zone,
    domain_last_checked_at timestamp with time zone,
    domain_check_fail_count integer DEFAULT 0 NOT NULL,
    bahasa_artikel text,
    label_sponsor character varying(20) DEFAULT 'tidak_ada'::character varying NOT NULL,
    topik_terlarang text,
    contoh_artikel_url character varying(500),
    domain_health character varying(20) DEFAULT 'unknown'::character varying NOT NULL,
    auto_publish boolean DEFAULT false NOT NULL,
    domain_last_error text,
    domain_last_error_at timestamp with time zone,
    domain_last_result character varying(20),
    domain_last_stage character varying(20),
    domain_last_trace text,
    word_count_mode character varying(20) DEFAULT 'sesuai_permintaan'::character varying NOT NULL,
    allow_embed boolean DEFAULT false NOT NULL,
    pending_changes jsonb,
    CONSTRAINT ck_media_credit_cost CHECK ((credit_cost >= 0)),
    CONSTRAINT ck_media_domain_health CHECK (((domain_health)::text = ANY (ARRAY[('unknown'::character varying)::text, ('live'::character varying)::text, ('unreachable'::character varying)::text, ('token_missing'::character varying)::text]))),
    CONSTRAINT ck_media_domain_last_result CHECK (((domain_last_result IS NULL) OR ((domain_last_result)::text = ANY (ARRAY[('verified'::character varying)::text, ('token_not_found'::character varying)::text, ('token_mismatch'::character varying)::text, ('unreachable'::character varying)::text])))),
    CONSTRAINT ck_media_domain_last_stage CHECK (((domain_last_stage IS NULL) OR ((domain_last_stage)::text = ANY (ARRAY[('no_website'::character varying)::text, ('dns'::character varying)::text, ('connect'::character varying)::text, ('http'::character varying)::text, ('bot_challenge'::character varying)::text, ('token'::character varying)::text, ('ok'::character varying)::text])))),
    CONSTRAINT ck_media_integration_type CHECK (((integration_type)::text = ANY (ARRAY[('manual'::character varying)::text, ('wordpress'::character varying)::text, ('rss'::character varying)::text, ('webhook'::character varying)::text, ('tulisio_api'::character varying)::text, ('custom_web'::character varying)::text, ('blogger'::character varying)::text]))),
    CONSTRAINT ck_media_label_sponsor CHECK (((label_sponsor)::text = ANY (ARRAY[('tidak_ada'::character varying)::text, ('ada_sponsor'::character varying)::text]))),
    CONSTRAINT ck_media_price_status CHECK (((price_status)::text = ANY (ARRAY[('menunggu_review'::character varying)::text, ('disetujui'::character varying)::text, ('negosiasi'::character varying)::text, ('ditolak'::character varying)::text, ('final'::character varying)::text]))),
    CONSTRAINT ck_media_pricing_type CHECK (((pricing_type)::text = ANY (ARRAY[('nett'::character varying)::text, ('nego'::character varying)::text]))),
    CONSTRAINT ck_media_publication_mode CHECK (((publication_mode)::text = ANY (ARRAY[('draft'::character varying)::text, ('publish'::character varying)::text]))),
    CONSTRAINT ck_media_quality_score CHECK (((media_quality_score IS NULL) OR ((media_quality_score >= 0) AND (media_quality_score <= 100)))),
    CONSTRAINT ck_media_status CHECK (((status)::text = ANY (ARRAY[('draft'::character varying)::text, ('active'::character varying)::text, ('inactive'::character varying)::text, ('archived'::character varying)::text, ('hidden'::character varying)::text, ('pending_review'::character varying)::text, ('rejected'::character varying)::text]))),
    CONSTRAINT ck_media_tier CHECK (((media_tier)::text = ANY (ARRAY[('tier1'::character varying)::text, ('tier2'::character varying)::text, ('tier3'::character varying)::text]))),
    CONSTRAINT ck_media_type CHECK (((media_type)::text = ANY (ARRAY[('blogger'::character varying)::text, ('mainstream'::character varying)::text]))),
    CONSTRAINT ck_media_word_count_mode CHECK (((word_count_mode)::text = ANY (ARRAY[('sesuai_permintaan'::character varying)::text, ('kata_300_700'::character varying)::text, ('kata_800_1500'::character varying)::text])))
);


--
-- Name: media_ai_summary_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.media_ai_summary_events (
    id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    media_name character varying(160),
    website_url character varying(500),
    status character varying(20) NOT NULL,
    pages_tried jsonb DEFAULT '[]'::jsonb NOT NULL,
    error_message character varying(1000),
    attempt integer DEFAULT 0 NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_media_ai_summary_events_status CHECK (((status)::text = ANY (ARRAY[('queued'::character varying)::text, ('success'::character varying)::text, ('failed'::character varying)::text])))
);


--
-- Name: media_categories; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.media_categories (
    id character(36) NOT NULL,
    name character varying(120) NOT NULL,
    normalized character varying(120) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: media_category_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.media_category_links (
    media_id character(36) NOT NULL,
    category_id character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: media_content_standards; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.media_content_standards (
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
    CONSTRAINT ck_media_standard_quotes CHECK ((min_quotes >= 0)),
    CONSTRAINT ck_media_standard_status CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('archived'::character varying)::text]))),
    CONSTRAINT ck_media_standard_style CHECK (((journalistic_style)::text = ANY (ARRAY[('news'::character varying)::text, ('feature'::character varying)::text, ('magazine'::character varying)::text, ('blog'::character varying)::text]))),
    CONSTRAINT ck_media_standard_tolerance CHECK (((hard_selling_tolerance)::text = ANY (ARRAY[('none'::character varying)::text, ('low'::character varying)::text, ('medium'::character varying)::text, ('high'::character varying)::text]))),
    CONSTRAINT ck_media_standard_words CHECK (((min_words >= 0) AND (max_words >= min_words)))
);


--
-- Name: media_dewan_pers_verifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.media_dewan_pers_verifications (
    id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    certificate_no character varying(200) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    rejection_reason text,
    conflict_with character(36),
    submitted_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL,
    reviewed_by character(36),
    reviewed_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL,
    CONSTRAINT ck_mdpv_status CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('verified'::character varying)::text, ('rejected'::character varying)::text, ('conflict'::character varying)::text, ('superseded'::character varying)::text])))
);


--
-- Name: media_package_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.media_package_items (
    package_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    "position" integer DEFAULT 0 NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: media_packages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.media_packages (
    id character(36) NOT NULL,
    name character varying(120) NOT NULL,
    slug character varying(140) NOT NULL,
    description text,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    discount_percent integer DEFAULT 0 NOT NULL,
    source character varying(20) DEFAULT 'admin'::character varying NOT NULL,
    brand_id character varying(36),
    CONSTRAINT ck_media_package_status CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('inactive'::character varying)::text]))),
    CONSTRAINT ck_media_packages_discount CHECK (((discount_percent >= 0) AND (discount_percent <= 100))),
    CONSTRAINT media_packages_source_check CHECK (((source)::text = ANY (ARRAY[('admin'::character varying)::text, ('client'::character varying)::text])))
);


--
-- Name: media_price_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.media_price_history (
    id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    actor_id character(36),
    from_price numeric(14,2),
    to_price numeric(14,2),
    from_pricing_type character varying(10),
    to_pricing_type character varying(10),
    from_price_status character varying(20),
    to_price_status character varying(20),
    note character varying(1000),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL
);


--
-- Name: media_seo_metrics; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.media_seo_metrics (
    media_id character(36) NOT NULL,
    da smallint,
    pa smallint,
    checked_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL,
    source character varying(40) DEFAULT 'rapidapi-domain-metrics'::character varying NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL,
    spam_score smallint,
    ai_trust_score smallint,
    ai_citation boolean,
    ai_citation_checked_at timestamp(6) with time zone,
    CONSTRAINT ck_media_seo_metrics_ai_trust CHECK (((ai_trust_score IS NULL) OR ((ai_trust_score >= 0) AND (ai_trust_score <= 100)))),
    CONSTRAINT ck_media_seo_metrics_da CHECK (((da IS NULL) OR ((da >= 0) AND (da <= 100)))),
    CONSTRAINT ck_media_seo_metrics_pa CHECK (((pa IS NULL) OR ((pa >= 0) AND (pa <= 100)))),
    CONSTRAINT ck_media_seo_metrics_spam CHECK (((spam_score IS NULL) OR ((spam_score >= 0) AND (spam_score <= 100)))),
    CONSTRAINT ck_media_seo_metrics_value CHECK (((da IS NOT NULL) OR (pa IS NOT NULL) OR (spam_score IS NOT NULL) OR (ai_trust_score IS NOT NULL) OR (ai_citation IS NOT NULL)))
);


--
-- Name: media_similarweb_snapshots; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.media_similarweb_snapshots (
    id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    insights jsonb NOT NULL,
    snapshot_date date,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL
);


--
-- Name: model_usage_events; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: notification_preferences; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notification_preferences (
    user_id character(36) NOT NULL,
    email_security boolean DEFAULT true NOT NULL,
    email_payments boolean DEFAULT true NOT NULL,
    email_campaign boolean DEFAULT true NOT NULL,
    email_partner boolean DEFAULT true NOT NULL,
    email_marketing boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    email_alt1 character varying(320),
    email_alt2 character varying(320),
    wa_number character varying(30),
    wa_consent boolean DEFAULT false NOT NULL
);


--
-- Name: notification_webhooks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notification_webhooks (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    url character varying(500) NOT NULL,
    secret character varying(200),
    active boolean DEFAULT true NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_notification_webhooks_url CHECK (((url)::text ~* '^https?://'::text))
);


--
-- Name: notifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.notifications (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    type character varying(40) NOT NULL,
    title character varying(200) NOT NULL,
    body character varying(1000),
    link character varying(500),
    publication_id character(36),
    read_at timestamp(6) with time zone,
    expired_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    dedupe_key character varying(200),
    CONSTRAINT ck_notifications_type CHECK (((type)::text = ANY (ARRAY[('publication.waiting_partner'::character varying)::text, ('publication.partner_approved'::character varying)::text, ('publication.partner_rejected'::character varying)::text, ('publication.auto_replaced'::character varying)::text, ('publication.published'::character varying)::text, ('publication.failed'::character varying)::text, ('publication.url_requested'::character varying)::text, ('publication.reminder'::character varying)::text, ('publication.late_approval'::character varying)::text, ('publication.link_lost'::character varying)::text, ('publication.link_replacement_requested'::character varying)::text, ('publication.revision_requested'::character varying)::text, ('publication.client_approved'::character varying)::text, ('variant_batch.failed'::character varying)::text, ('pr.score_updated'::character varying)::text, ('pr.verification_completed'::character varying)::text, ('media.submitted'::character varying)::text, ('media.approved'::character varying)::text, ('media.rejected'::character varying)::text, ('media.resubmitted'::character varying)::text, ('media.negotiated'::character varying)::text, ('media.domain_down'::character varying)::text, ('media.dewan_pers.submitted'::character varying)::text, ('media.dewan_pers.approved'::character varying)::text, ('media.dewan_pers.rejected'::character varying)::text, ('withdrawal.requested'::character varying)::text, ('withdrawal.processing'::character varying)::text, ('withdrawal.paid'::character varying)::text, ('withdrawal.rejected'::character varying)::text, ('referral.domain.approved'::character varying)::text, ('referral.domain.rejected'::character varying)::text, ('referral.cpa.credited'::character varying)::text, ('referral.commission.credited'::character varying)::text, ('referral.commission.reversed'::character varying)::text, ('vip.enabled'::character varying)::text, ('vip.disabled'::character varying)::text])))
);


--
-- Name: packages; Type: TABLE; Schema: public; Owner: -
--

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
    CONSTRAINT ck_package_credit_total CHECK ((credit_total > 0)),
    CONSTRAINT ck_package_credit_used CHECK ((credit_used >= 0)),
    CONSTRAINT ck_package_status CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('exhausted'::character varying)::text, ('revoked'::character varying)::text])))
);


--
-- Name: partner_documents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.partner_documents (
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
    CONSTRAINT ck_partner_documents_status CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('verified'::character varying)::text, ('revision'::character varying)::text])))
);


--
-- Name: partner_earnings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.partner_earnings (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    publication_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    rule_id character(36),
    amount numeric(14,2) NOT NULL,
    client_price numeric(14,2) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    settlement_id character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    kind character varying(20) DEFAULT 'publication'::character varying NOT NULL,
    source_earning_id character(36),
    source_referred_domain_id character(36),
    CONSTRAINT ck_partner_earnings_amount CHECK ((amount >= (0)::numeric)),
    CONSTRAINT ck_partner_earnings_kind CHECK (((kind)::text = ANY (ARRAY[('publication'::character varying)::text, ('referral'::character varying)::text, ('cpa'::character varying)::text, ('transaction_commission'::character varying)::text]))),
    CONSTRAINT ck_partner_earnings_status CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('settled'::character varying)::text, ('voided'::character varying)::text])))
);


--
-- Name: partner_referral_commissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.partner_referral_commissions (
    id character(36) NOT NULL,
    referrer_partner_id character(36) NOT NULL,
    referee_partner_id character(36) NOT NULL,
    referee_user_id character(36) NOT NULL,
    earning_id character(36) NOT NULL,
    amount numeric(14,2) NOT NULL,
    rate_bps integer NOT NULL,
    status character varying(20) NOT NULL,
    reversal_of character(36),
    notes text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    paid_at timestamp with time zone,
    CONSTRAINT partner_referral_commissions_amount_check CHECK ((amount >= (0)::numeric)),
    CONSTRAINT partner_referral_commissions_rate_bps_check CHECK (((rate_bps >= 0) AND (rate_bps <= 10000))),
    CONSTRAINT partner_referral_commissions_status_check CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('available'::character varying)::text, ('paid'::character varying)::text, ('reversed'::character varying)::text, ('review_needed'::character varying)::text])))
);


--
-- Name: partner_seo_check_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.partner_seo_check_history (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    domain character varying(190) NOT NULL,
    da smallint,
    pa smallint,
    spam_score smallint,
    checked_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: partner_service_charges; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.partner_service_charges (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    service character varying(40) NOT NULL,
    amount_idr numeric(14,2) NOT NULL,
    reference_id character varying(100),
    reference_type character varying(40),
    notes text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT partner_service_charges_amount_idr_check CHECK ((amount_idr >= (0)::numeric))
);


--
-- Name: partner_speed_audits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.partner_speed_audits (
    id uuid NOT NULL,
    partner_id uuid NOT NULL,
    url text NOT NULL,
    normalized_url text NOT NULL,
    domain text NOT NULL,
    strategy text DEFAULT 'mobile'::text NOT NULL,
    performance_score integer,
    accessibility_score integer,
    best_practices_score integer,
    seo_score integer,
    lcp_ms integer,
    inp_ms integer,
    cls numeric,
    audits jsonb DEFAULT '[]'::jsonb NOT NULL,
    provider text DEFAULT 'pagespeed'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: partners; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.partners (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    name character varying(160),
    public_id character varying(180),
    category character varying(60) DEFAULT 'lainnya'::character varying,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    auto_approve boolean DEFAULT false NOT NULL,
    rss_enabled boolean DEFAULT false NOT NULL,
    referral_code character varying(32),
    referral_code_normalized character varying(32),
    is_vip boolean DEFAULT false NOT NULL,
    vip_tier character varying(32) DEFAULT 'regular'::character varying NOT NULL,
    vip_enabled_at timestamp with time zone,
    vip_contract_months integer,
    vip_expires_at timestamp with time zone,
    integration_terms_accepted_at timestamp with time zone,
    payout_type character varying(16) DEFAULT 'bank'::character varying NOT NULL,
    bank_name character varying(120),
    bank_account_number character varying(40),
    bank_account_name character varying(160),
    auto_publish_enabled boolean DEFAULT false NOT NULL,
    vip_contract_days integer,
    CONSTRAINT ck_partner_status CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('inactive'::character varying)::text]))),
    CONSTRAINT partners_vip_tier_check CHECK (((vip_tier)::text = ANY (ARRAY[('regular'::character varying)::text, ('brand_ambassador'::character varying)::text, ('vip_reguler'::character varying)::text])))
);


--
-- Name: password_reset_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.password_reset_tokens (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    token_hash character(64) NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    consumed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: payment_gateway_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_gateway_configs (
    id character(36) NOT NULL,
    provider character varying(20) NOT NULL,
    environment character varying(10) DEFAULT 'sandbox'::character varying NOT NULL,
    encrypted_credentials text NOT NULL,
    is_configured boolean DEFAULT false NOT NULL,
    is_active boolean DEFAULT false NOT NULL,
    last_tested_at timestamp(6) with time zone,
    last_test_ok boolean,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_payment_gateway_environment CHECK (((environment)::text = ANY (ARRAY[('sandbox'::character varying)::text, ('production'::character varying)::text]))),
    CONSTRAINT ck_payment_gateway_provider CHECK (((provider)::text = ANY (ARRAY[('xendit'::character varying)::text, ('duitku'::character varying)::text, ('midtrans'::character varying)::text, ('tripay'::character varying)::text, ('sandbox'::character varying)::text, ('sumopod'::character varying)::text])))
);


--
-- Name: payment_transaction_ref_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.payment_transaction_ref_seq
    START WITH 1000
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: payment_transactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_transactions (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    brand_id character(36),
    invoice_id character(36),
    gateway_provider character varying(20) NOT NULL,
    gateway_config_id character(36),
    gateway_environment character varying(10) NOT NULL,
    external_transaction_id character varying(120),
    amount numeric(14,2) NOT NULL,
    currency character varying(8) DEFAULT 'IDR'::character varying NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    payment_method character varying(60),
    payment_url character varying(2000),
    purpose character varying(20) DEFAULT 'generic'::character varying NOT NULL,
    purpose_ref_id character(36),
    paid_at timestamp(6) with time zone,
    expired_at timestamp(6) with time zone,
    raw_callback jsonb,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    provider_reference character varying(120),
    reference character varying(40),
    invoice_data jsonb,
    idempotency_key character varying(100),
    CONSTRAINT ck_payment_transaction_amount CHECK ((amount > (0)::numeric)),
    CONSTRAINT ck_payment_transaction_environment CHECK (((gateway_environment)::text = ANY (ARRAY[('sandbox'::character varying)::text, ('production'::character varying)::text]))),
    CONSTRAINT ck_payment_transaction_status CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('processing'::character varying)::text, ('paid'::character varying)::text, ('failed'::character varying)::text, ('expired'::character varying)::text, ('cancelled'::character varying)::text, ('refunded'::character varying)::text]))),
    CONSTRAINT ck_payment_transactions_gateway CHECK (((gateway_provider)::text = ANY (ARRAY[('xendit'::character varying)::text, ('duitku'::character varying)::text, ('midtrans'::character varying)::text, ('tripay'::character varying)::text, ('sandbox'::character varying)::text, ('sumopod'::character varying)::text])))
);


--
-- Name: payment_webhook_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.payment_webhook_events (
    id character(36) NOT NULL,
    gateway_provider character varying(20) NOT NULL,
    external_transaction_id character varying(120) NOT NULL,
    status character varying(20) NOT NULL,
    payload jsonb,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: platform_admins; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.platform_admins (
    user_id character(36) NOT NULL,
    role character varying(20) DEFAULT 'superadmin'::character varying NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    permissions jsonb DEFAULT '{}'::jsonb NOT NULL,
    CONSTRAINT ck_platform_admins_permissions CHECK ((jsonb_typeof(permissions) = 'object'::text)),
    CONSTRAINT ck_platform_admins_role CHECK (((role)::text = ANY (ARRAY[('super_admin'::character varying)::text, ('admin'::character varying)::text])))
);


--
-- Name: platform_ai_configuration; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: platform_crawlhub_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.platform_crawlhub_config (
    id character varying(36) NOT NULL,
    base_url character varying(500),
    sealed_hmac_secret text,
    sealed_ingest_secret text,
    updated_by character varying(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: platform_gsc_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.platform_gsc_config (
    id text DEFAULT 'default'::text NOT NULL,
    client_id text,
    sealed_client_secret text,
    redirect_uri text,
    updated_by character(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL,
    CONSTRAINT ck_platform_gsc_config_id CHECK ((id = 'default'::text))
);


--
-- Name: platform_oauth_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.platform_oauth_configs (
    id character varying(20) DEFAULT 'google'::character varying NOT NULL,
    provider character varying(20) DEFAULT 'google'::character varying NOT NULL,
    client_id character varying(500),
    sealed_client_secret text,
    redirect_uri character varying(500),
    updated_by character(36),
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT ck_platform_oauth_id CHECK (((id)::text = 'google'::text))
);


--
-- Name: platform_pagespeed_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.platform_pagespeed_config (
    id text NOT NULL,
    sealed_api_key text,
    api_key_hint text,
    updated_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: platform_seo_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.platform_seo_config (
    id text DEFAULT 'default'::text NOT NULL,
    sealed_api_key text,
    api_key_hint character varying(16),
    updated_by character(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL,
    sealed_similarweb_api_key text,
    sealed_smodin_api_key text,
    sealed_serpens_api_key text,
    CONSTRAINT ck_platform_seo_config_id CHECK ((id = 'default'::text))
);


--
-- Name: platform_server_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.platform_server_config (
    id character varying(36) NOT NULL,
    sealed_public_ip text,
    updated_by character varying(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: platform_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.platform_settings (
    key character varying(64) NOT NULL,
    value text DEFAULT ''::text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: platform_turnstile_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.platform_turnstile_config (
    id character(36) NOT NULL,
    site_key character varying(512) NOT NULL,
    sealed_secret_key text NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    updated_by character(36),
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: platform_wa_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.platform_wa_config (
    id text DEFAULT 'default'::text NOT NULL,
    sealed_token text,
    token_hint character varying(16),
    sender_name character varying(120),
    country_code character varying(6) DEFAULT '62'::character varying NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    updated_by character(36),
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT ck_platform_wa_config_id CHECK ((id = 'default'::text))
);


--
-- Name: pr_campaign_scores; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pr_campaign_scores (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    profile_id character(36) NOT NULL,
    profile_version integer NOT NULL,
    weights jsonb NOT NULL,
    status character varying(20) DEFAULT 'not_ready'::character varying NOT NULL,
    score numeric(5,2),
    label character varying(40),
    breakdown jsonb DEFAULT '{}'::jsonb NOT NULL,
    input_snapshot jsonb DEFAULT '{}'::jsonb NOT NULL,
    calculated_by character(36),
    calculated_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_pr_campaign_score_status CHECK (((status)::text = ANY (ARRAY[('not_ready'::character varying)::text, ('calculating'::character varying)::text, ('partial'::character varying)::text, ('ready'::character varying)::text, ('final'::character varying)::text, ('failed'::character varying)::text])))
);


--
-- Name: pr_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pr_config (
    id text DEFAULT 'default'::text NOT NULL,
    rate_per_1k_reach integer DEFAULT 5000 NOT NULL,
    rate_tier1 integer,
    rate_tier2 integer,
    rate_tier3 integer,
    updated_by character(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_pr_config_id CHECK ((id = 'default'::text)),
    CONSTRAINT ck_pr_config_rate CHECK ((rate_per_1k_reach >= 0))
);


--
-- Name: pr_media_rules; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pr_media_rules (
    id character(36) NOT NULL,
    tier_default_quality jsonb DEFAULT '{"tier1": 90, "tier2": 80, "tier3": 70}'::jsonb NOT NULL,
    category_relevance jsonb DEFAULT '{}'::jsonb NOT NULL,
    updated_by character(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: pr_message_analyses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pr_message_analyses (
    id character(36) NOT NULL,
    publication_id character(36) NOT NULL,
    campaign_id character(36),
    key_messages jsonb DEFAULT '[]'::jsonb NOT NULL,
    results jsonb DEFAULT '[]'::jsonb NOT NULL,
    consistency_score numeric(5,2),
    analysis_version character varying(60) NOT NULL,
    status character varying(20) DEFAULT 'completed'::character varying NOT NULL,
    error_message character varying(500),
    analyzed_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_pr_message_analysis_status CHECK (((status)::text = ANY (ARRAY[('queued'::character varying)::text, ('running'::character varying)::text, ('completed'::character varying)::text, ('failed'::character varying)::text])))
);


--
-- Name: pr_metric_results; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pr_metric_results (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    metric_key character varying(60) NOT NULL,
    metric_value numeric NOT NULL,
    source character varying(120),
    calculated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: pr_publication_verifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pr_publication_verifications (
    id character(36) NOT NULL,
    publication_id character(36) NOT NULL,
    campaign_id character(36),
    url character varying(500),
    status character varying(20) DEFAULT 'unavailable'::character varying NOT NULL,
    compliance_score numeric(5,2),
    checks jsonb DEFAULT '[]'::jsonb NOT NULL,
    error_message character varying(500),
    verified_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_pr_verification_status CHECK (((status)::text = ANY (ARRAY[('passed'::character varying)::text, ('failed'::character varying)::text, ('unavailable'::character varying)::text])))
);


--
-- Name: pr_scoring_profiles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pr_scoring_profiles (
    id character(36) NOT NULL,
    version integer NOT NULL,
    name character varying(120) NOT NULL,
    weights jsonb NOT NULL,
    thresholds jsonb DEFAULT '{}'::jsonb NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_pr_scoring_profile_status CHECK (((status)::text = ANY (ARRAY[('draft'::character varying)::text, ('active'::character varying)::text, ('archived'::character varying)::text])))
);


--
-- Name: pr_sentiment_analyses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pr_sentiment_analyses (
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
    CONSTRAINT ck_pr_sentiment_analysis_status CHECK (((status)::text = ANY (ARRAY[('queued'::character varying)::text, ('running'::character varying)::text, ('completed'::character varying)::text, ('failed'::character varying)::text]))),
    CONSTRAINT ck_pr_sentiment_value CHECK (((sentiment IS NULL) OR ((sentiment)::text = ANY (ARRAY[('positive'::character varying)::text, ('neutral'::character varying)::text, ('negative'::character varying)::text]))))
);


--
-- Name: pricing_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.pricing_config (
    id integer DEFAULT 1 NOT NULL,
    markup_percent numeric(5,2) DEFAULT 15 NOT NULL,
    updated_by character(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_pricing_config_markup CHECK ((markup_percent >= (0)::numeric)),
    CONSTRAINT ck_pricing_config_singleton CHECK ((id = 1))
);


--
-- Name: publication_attempts; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: publication_jobs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.publication_jobs (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    document_id character(36) NOT NULL,
    connection_id character(36),
    publication_id character(36),
    status character varying(20) NOT NULL,
    publish_status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    payload_snapshot jsonb DEFAULT '{}'::jsonb NOT NULL,
    remote_post_id character varying(120),
    remote_url character varying(500),
    error_message character varying(500),
    error_code character varying(80),
    attempts integer DEFAULT 0 NOT NULL,
    started_at timestamp(6) with time zone,
    completed_at timestamp(6) with time zone,
    next_retry_at timestamp(6) with time zone,
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    progress integer DEFAULT 0
);


--
-- Name: publication_link_checks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.publication_link_checks (
    publication_id character(36) NOT NULL,
    url character varying(500),
    final_url character varying(500),
    http_status integer,
    state character varying(20) DEFAULT 'no_url'::character varying NOT NULL,
    consecutive_failures integer DEFAULT 0 NOT NULL,
    first_detected_gone_at timestamp(6) with time zone,
    last_checked_at timestamp(6) with time zone,
    error_message character varying(500),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_publication_link_checks_failures CHECK ((consecutive_failures >= 0)),
    CONSTRAINT ck_publication_link_checks_state CHECK (((state)::text = ANY (ARRAY[('live'::character varying)::text, ('gone'::character varying)::text, ('unreachable'::character varying)::text, ('no_url'::character varying)::text])))
);


--
-- Name: publication_order_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.publication_order_seq
    START WITH 1000
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: publication_plan_media; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.publication_plan_media (
    plan_id character varying(36) NOT NULL,
    media_id character varying(36) NOT NULL,
    "position" integer DEFAULT 0 NOT NULL
);


--
-- Name: publication_plan_occurrences; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.publication_plan_occurrences (
    id character varying(36) NOT NULL,
    plan_id character varying(36) NOT NULL,
    media_id character varying(36) NOT NULL,
    scheduled_at timestamp with time zone NOT NULL,
    status character varying(20) DEFAULT 'scheduled'::character varying NOT NULL,
    publication_id character varying(36),
    error_message text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT publication_plan_occurrences_status_check CHECK (((status)::text = ANY (ARRAY[('scheduled'::character varying)::text, ('materialized'::character varying)::text, ('skipped'::character varying)::text, ('failed'::character varying)::text])))
);


--
-- Name: publication_plans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.publication_plans (
    id character varying(36) NOT NULL,
    brand_id character varying(36) NOT NULL,
    name character varying(160) NOT NULL,
    document_id character varying(36) NOT NULL,
    cadence character varying(20) DEFAULT 'daily'::character varying NOT NULL,
    start_at date NOT NULL,
    end_at date,
    time_of_day time without time zone DEFAULT '09:00:00'::time without time zone NOT NULL,
    timezone character varying(64) DEFAULT 'Asia/Jakarta'::character varying NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character varying(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    media_per_day integer DEFAULT 1 NOT NULL,
    date_mode character varying(16) DEFAULT 'consecutive'::character varying NOT NULL,
    custom_dates jsonb,
    CONSTRAINT ck_publication_plans_dates CHECK (((end_at IS NULL) OR (end_at >= start_at))),
    CONSTRAINT publication_plans_cadence_check CHECK (((cadence)::text = 'daily'::text)),
    CONSTRAINT publication_plans_date_mode_check CHECK (((date_mode)::text = ANY (ARRAY[('consecutive'::character varying)::text, ('random'::character varying)::text, ('custom'::character varying)::text]))),
    CONSTRAINT publication_plans_status_check CHECK (((status)::text = ANY (ARRAY[('draft'::character varying)::text, ('active'::character varying)::text, ('paused'::character varying)::text, ('completed'::character varying)::text, ('cancelled'::character varying)::text])))
);


--
-- Name: publication_variant_batches; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.publication_variant_batches (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    mode character varying(20) NOT NULL,
    seed integer NOT NULL,
    status character varying(20) DEFAULT 'queued'::character varying NOT NULL,
    source_document_id character(36),
    source_material text,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    completed_at timestamp(6) with time zone
);


--
-- Name: publication_variants; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.publication_variants (
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
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: publications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.publications (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    document_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    client_price numeric(14,2) DEFAULT 0 NOT NULL,
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
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    auto_replace_disabled boolean DEFAULT false NOT NULL,
    partner_price numeric(14,2),
    markup_percent numeric(5,2),
    partner_draft_edits jsonb,
    client_approved_at timestamp(6) with time zone,
    client_approved_by character(36),
    order_code character varying(40),
    CONSTRAINT ck_publication_publish_status CHECK (((publish_status)::text = ANY (ARRAY[('draft'::character varying)::text, ('publish'::character varying)::text]))),
    CONSTRAINT ck_publication_status CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('waiting_partner'::character varying)::text, ('partner_approved'::character varying)::text, ('partner_rejected'::character varying)::text, ('queued'::character varying)::text, ('processing'::character varying)::text, ('published'::character varying)::text, ('failed'::character varying)::text, ('retrying'::character varying)::text, ('cancelled'::character varying)::text])))
);


--
-- Name: publishing_api_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.publishing_api_keys (
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
    CONSTRAINT ck_publishing_api_key_status CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('revoked'::character varying)::text])))
);


--
-- Name: publishing_callback_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.publishing_callback_logs (
    id character(36) NOT NULL,
    api_key_id character(36),
    partner_id character(36),
    publication_id character(36),
    ip_address character varying(64),
    user_agent character varying(300),
    status_code integer,
    accepted boolean,
    error_message character varying(500),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: quality_evaluations; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: referral_attributions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.referral_attributions (
    id character(36) NOT NULL,
    code character varying(32) NOT NULL,
    referrer_partner_id character(36) NOT NULL,
    referee_user_id character(36),
    ip inet,
    user_agent text,
    notes text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    click_id character(36),
    referer character varying(1000)
);


--
-- Name: referral_clicks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.referral_clicks (
    id character(36) NOT NULL,
    code character varying(32) NOT NULL,
    referrer_partner_id character(36) NOT NULL,
    fingerprint character varying(200) NOT NULL,
    click_count integer DEFAULT 1 NOT NULL,
    first_clicked_at timestamp with time zone DEFAULT now() NOT NULL,
    last_clicked_at timestamp with time zone DEFAULT now() NOT NULL,
    last_ip character varying(64),
    last_user_agent character varying(500)
);


--
-- Name: referral_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.referral_settings (
    key character varying(80) NOT NULL,
    value character varying(200) NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: referred_domains; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.referred_domains (
    id character(36) NOT NULL,
    referrer_partner_id character(36) NOT NULL,
    owner_user_id character(36) NOT NULL,
    media_id character(36),
    domain_name character varying(255) NOT NULL,
    domain_age_months integer,
    is_tld boolean DEFAULT true NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    rejection_reason text,
    cpa_paid boolean DEFAULT false NOT NULL,
    cpa_amount integer,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT referred_domains_status_check CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('approved'::character varying)::text, ('rejected'::character varying)::text])))
);


--
-- Name: registration_guard_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.registration_guard_config (
    id integer DEFAULT 1 NOT NULL,
    scores jsonb NOT NULL,
    thresholds jsonb NOT NULL,
    rate_limits jsonb NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT registration_guard_config_id_check CHECK ((id = 1))
);


--
-- Name: registration_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.registration_history (
    id character(36) NOT NULL,
    user_id character(36),
    registration_source character varying(20) NOT NULL,
    email_hash character varying(64),
    google_sub_hash character varying(64),
    ip_hash character varying(64),
    device_hash character varying(64),
    domain character varying(253),
    registered_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    risk_score integer DEFAULT 0 NOT NULL,
    risk_level character varying(20) DEFAULT 'low'::character varying NOT NULL,
    reason_codes jsonb DEFAULT '[]'::jsonb NOT NULL
);


--
-- Name: revenue_shares; Type: TABLE; Schema: public; Owner: -
--

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
    CONSTRAINT ck_revenue_share_fixed CHECK ((partner_fixed >= (0)::numeric)),
    CONSTRAINT ck_revenue_share_percent CHECK (((partner_percent >= (0)::numeric) AND (partner_percent <= (100)::numeric) AND (tulisio_percent >= (0)::numeric) AND (tulisio_percent <= (100)::numeric))),
    CONSTRAINT ck_revenue_share_status CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('archived'::character varying)::text]))),
    CONSTRAINT ck_revenue_share_type CHECK (((share_type)::text = ANY (ARRAY[('percentage'::character varying)::text, ('fixed'::character varying)::text])))
);


--
-- Name: review_comments; Type: TABLE; Schema: public; Owner: -
--

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
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    attachment_url character varying(500),
    attachment_name character varying(255)
);


--
-- Name: rss_access_logs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.rss_access_logs (
    id character(36) NOT NULL,
    feed_id character(36) NOT NULL,
    ip_address character varying(64),
    user_agent character varying(300),
    status smallint,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: rss_feeds; Type: TABLE; Schema: public; Owner: -
--

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
    CONSTRAINT ck_rss_feed_item_status CHECK (((item_status)::text = ANY (ARRAY[('published'::character varying)::text, ('draft'::character varying)::text]))),
    CONSTRAINT ck_rss_feed_status CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('revoked'::character varying)::text])))
);


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version text NOT NULL,
    applied_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: settlements; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.settlements (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    period_start date NOT NULL,
    period_end date NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    total_amount numeric(14,2) DEFAULT 0 NOT NULL,
    notes text,
    paid_at timestamp(6) with time zone,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    receipt_url character varying(500),
    rejection_reason character varying(1000),
    payment_reference character varying(200),
    payout_method character varying(10) DEFAULT 'bank'::character varying NOT NULL,
    transfer_fee_idr integer DEFAULT 0 NOT NULL,
    net_amount_idr integer DEFAULT 0 NOT NULL,
    invoice_number character varying(40),
    CONSTRAINT ck_settlement_amount CHECK ((total_amount >= (0)::numeric)),
    CONSTRAINT ck_settlement_period CHECK ((period_end >= period_start)),
    CONSTRAINT ck_settlement_status CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('approved'::character varying)::text, ('processing'::character varying)::text, ('paid'::character varying)::text, ('failed'::character varying)::text])))
);


--
-- Name: smtp_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.smtp_settings (
    id character varying(20) DEFAULT 'default'::character varying NOT NULL,
    provider character varying(20) DEFAULT 'smtp'::character varying NOT NULL,
    host character varying(200),
    port integer DEFAULT 587 NOT NULL,
    secure boolean DEFAULT false NOT NULL,
    username character varying(200),
    password_sealed character varying(2000),
    from_email character varying(200) DEFAULT 'no-reply@tauthub.com'::character varying NOT NULL,
    from_name character varying(120) DEFAULT 'Tauthub'::character varying NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    last_test_at timestamp with time zone,
    last_test_ok boolean,
    last_test_message character varying(500),
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by character(36),
    CONSTRAINT ck_smtp_port CHECK (((port IS NULL) OR ((port > 0) AND (port < 65536)))),
    CONSTRAINT ck_smtp_provider CHECK (((provider)::text = 'smtp'::text))
);


--
-- Name: system_metadata; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.system_metadata (
    key character varying(100) NOT NULL,
    value character varying(500) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: taut_agent_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.taut_agent_configs (
    agent character varying(20) NOT NULL,
    base_url text,
    sealed_api_key text,
    model character varying(120),
    temperature numeric(3,2) DEFAULT 0.7,
    max_tokens integer,
    timeout_ms integer DEFAULT 60000,
    system_prompt text,
    enabled boolean DEFAULT false NOT NULL,
    updated_by character(36),
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    rate_limit_per_min integer DEFAULT 3,
    settings jsonb
);


--
-- Name: taut_agent_runs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.taut_agent_runs (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    media_id character(36),
    agent character varying(20) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    model character varying(120),
    token_input integer,
    token_output integer,
    duration_ms integer,
    input jsonb,
    output jsonb,
    error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    finished_at timestamp with time zone,
    input_hash character varying(64),
    CONSTRAINT ck_taut_runs_agent CHECK (((agent)::text = ANY (ARRAY[('profiler'::character varying)::text, ('matchmaker'::character varying)::text, ('writer'::character varying)::text, ('gatekeeper'::character varying)::text]))),
    CONSTRAINT ck_taut_runs_status CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('running'::character varying)::text, ('done'::character varying)::text, ('failed'::character varying)::text, ('skipped'::character varying)::text])))
);


--
-- Name: taut_campaign_files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.taut_campaign_files (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    submission_id character(36),
    original_filename character varying(255) NOT NULL,
    storage_key text NOT NULL,
    mime_type character varying(120),
    size_bytes integer,
    checksum character varying(64),
    parse_status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    extracted_text text,
    parse_error text,
    uploaded_by character(36) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_taut_campaign_files_parse CHECK (((parse_status)::text = ANY (ARRAY[('pending'::character varying)::text, ('parsing'::character varying)::text, ('ready'::character varying)::text, ('failed'::character varying)::text])))
);


--
-- Name: taut_campaign_media; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.taut_campaign_media (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    source character varying(10) DEFAULT 'ai'::character varying NOT NULL,
    price numeric(14,2),
    status character varying(20) DEFAULT 'selected'::character varying NOT NULL,
    article_document_id character(36),
    article_status character varying(20) DEFAULT 'none'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    featured_image_url character varying(500),
    category character varying(120),
    match_reason text,
    ai_score integer,
    match_rank integer,
    regen_count integer DEFAULT 0 NOT NULL,
    CONSTRAINT ck_taut_cm_article_status CHECK (((article_status)::text = ANY (ARRAY[('none'::character varying)::text, ('generating'::character varying)::text, ('draft'::character varying)::text, ('hyperlinked'::character varying)::text, ('safe'::character varying)::text, ('blocked'::character varying)::text, ('revision'::character varying)::text, ('ready'::character varying)::text]))),
    CONSTRAINT ck_taut_cm_source CHECK (((source)::text = ANY (ARRAY[('ai'::character varying)::text, ('manual'::character varying)::text]))),
    CONSTRAINT ck_taut_cm_status CHECK (((status)::text = ANY (ARRAY[('selected'::character varying)::text, ('removed'::character varying)::text, ('replaced'::character varying)::text])))
);


--
-- Name: taut_content_angle_prompt_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.taut_content_angle_prompt_versions (
    id character(36) NOT NULL,
    content_angle_id character(36) NOT NULL,
    version integer NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    model_overrides jsonb,
    change_note character varying(255),
    created_by character(36) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    published_by character(36),
    published_at timestamp with time zone,
    prompt text,
    CONSTRAINT ck_taut_angle_versions_status CHECK (((status)::text = ANY (ARRAY[('draft'::character varying)::text, ('active'::character varying)::text, ('archived'::character varying)::text])))
);


--
-- Name: taut_content_angles; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.taut_content_angles (
    id character(36) NOT NULL,
    channel_id character(36) NOT NULL,
    name character varying(160) NOT NULL,
    slug character varying(100) NOT NULL,
    description text,
    active boolean DEFAULT true NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_by character(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by character(36),
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: taut_gatekeeper_audits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.taut_gatekeeper_audits (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    agent_run_id character(36),
    run_revision integer DEFAULT 1 NOT NULL,
    status character varying(20) NOT NULL,
    checks jsonb DEFAULT '{}'::jsonb NOT NULL,
    overall_verdict character varying(20) NOT NULL,
    created_by character(36),
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: taut_media_channels; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.taut_media_channels (
    id character(36) NOT NULL,
    name character varying(120) NOT NULL,
    slug character varying(80) NOT NULL,
    description text,
    active boolean DEFAULT true NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_by character(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by character(36),
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: taut_profiler_base_prompt_versions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.taut_profiler_base_prompt_versions (
    id character(36) NOT NULL,
    version integer NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    change_note character varying(255),
    created_by character(36) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    published_by character(36),
    published_at timestamp with time zone,
    prompt text,
    CONSTRAINT ck_taut_base_prompt_status CHECK (((status)::text = ANY (ARRAY[('draft'::character varying)::text, ('active'::character varying)::text, ('archived'::character varying)::text])))
);


--
-- Name: taut_profiler_results; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.taut_profiler_results (
    id character(36) NOT NULL,
    submission_id character(36) NOT NULL,
    agent_run_id character(36),
    result_schema_version integer DEFAULT 1 NOT NULL,
    executive_brief jsonb NOT NULL,
    writing_instructions jsonb DEFAULT '[]'::jsonb NOT NULL,
    strategic_notes jsonb DEFAULT '[]'::jsonb NOT NULL,
    compiled_prompt_manifest jsonb DEFAULT '{}'::jsonb NOT NULL,
    model_snapshot jsonb,
    credit_ledger_entry_id character(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: taut_profiler_submission_channels; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.taut_profiler_submission_channels (
    id character(36) NOT NULL,
    submission_id character(36) NOT NULL,
    channel_id character(36) NOT NULL,
    content_angle_id character(36),
    prompt_version_id character(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: taut_profiler_submissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.taut_profiler_submissions (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    revision integer DEFAULT 1 NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    input_schema_version integer DEFAULT 1 NOT NULL,
    objective jsonb DEFAULT '{}'::jsonb NOT NULL,
    business_profile jsonb DEFAULT '{}'::jsonb NOT NULL,
    audience jsonb DEFAULT '{}'::jsonb NOT NULL,
    spokesperson jsonb,
    guardrails jsonb DEFAULT '{"rules": []}'::jsonb NOT NULL,
    backlink jsonb DEFAULT '{}'::jsonb NOT NULL,
    materials jsonb DEFAULT '{}'::jsonb NOT NULL,
    budget jsonb DEFAULT '{}'::jsonb NOT NULL,
    crawl_result jsonb,
    input_hash character(64),
    created_by character(36) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by character(36),
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    channels jsonb DEFAULT '[]'::jsonb NOT NULL,
    spokepersons jsonb DEFAULT '[]'::jsonb NOT NULL,
    CONSTRAINT ck_taut_submissions_status CHECK (((status)::text = ANY (ARRAY[('draft'::character varying)::text, ('analyzing'::character varying)::text, ('completed'::character varying)::text, ('failed'::character varying)::text, ('superseded'::character varying)::text])))
);


--
-- Name: taut_training_samples; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.taut_training_samples (
    id character varying(36) NOT NULL,
    campaign_id character varying(36),
    media_id character varying(36),
    agent character varying(32) NOT NULL,
    status character varying(16) NOT NULL,
    input_text text,
    output_text text,
    model character varying(120),
    token_input integer,
    token_output integer,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: tc_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tc_config (
    id integer DEFAULT 1 NOT NULL,
    rate_idr integer DEFAULT 1000 NOT NULL,
    topup_packages jsonb DEFAULT '[{"tc": 500, "idr": 500000}, {"tc": 1000, "idr": 1000000}, {"tc": 5000, "idr": 5000000}]'::jsonb NOT NULL,
    updated_by character(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    minimum_withdraw_idr integer DEFAULT 10000 NOT NULL,
    withdraw_bank_fee_idr integer DEFAULT 2400 NOT NULL,
    withdraw_ewallet_fee_percent numeric(5,2) DEFAULT 0.7 NOT NULL,
    CONSTRAINT ck_tc_config_id CHECK ((id = 1)),
    CONSTRAINT ck_tc_config_min_withdraw CHECK ((minimum_withdraw_idr > 0)),
    CONSTRAINT ck_tc_config_rate CHECK ((rate_idr > 0))
);


--
-- Name: tracking_events; Type: TABLE; Schema: public; Owner: -
--

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
    CONSTRAINT ck_tracking_event_type CHECK (((event_type)::text = ANY (ARRAY[('article_view'::character varying)::text, ('article_scroll'::character varying)::text, ('outbound_click'::character varying)::text, ('cta_click'::character varying)::text, ('tracking_link_click'::character varying)::text])))
);


--
-- Name: tracking_links; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tracking_links (
    id character(36) NOT NULL,
    code character varying(24) NOT NULL,
    publication_id character(36),
    brand_id character(36) NOT NULL,
    destination_url character varying(2000),
    category character varying(60) DEFAULT 'lainnya'::character varying,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    campaign_id character varying(36),
    kind character varying(20),
    CONSTRAINT ck_tracking_link_status CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('disabled'::character varying)::text])))
);


--
-- Name: transactions; Type: TABLE; Schema: public; Owner: -
--

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
    CONSTRAINT ck_transaction_amount CHECK ((amount <> (0)::numeric)),
    CONSTRAINT ck_transaction_type CHECK (((type)::text = ANY (ARRAY[('publication.revenue'::character varying)::text, ('publication.refund'::character varying)::text, ('settlement.payout'::character varying)::text, ('adjustment'::character varying)::text])))
);


--
-- Name: user_daily_quotas; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_daily_quotas (
    user_id character(36) NOT NULL,
    daily_token_limit integer DEFAULT 0 NOT NULL,
    updated_by character(36) NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL
);


--
-- Name: user_oauth_accounts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_oauth_accounts (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    provider character varying(20) DEFAULT 'google'::character varying NOT NULL,
    provider_account_id character varying(200) NOT NULL,
    email character varying(320),
    display_name character varying(120),
    avatar_url character varying(500),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: user_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_sessions (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    refresh_jti character(36) NOT NULL,
    expires_at timestamp(6) with time zone NOT NULL,
    revoked boolean DEFAULT false NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id character(36) NOT NULL,
    email character varying(320) NOT NULL,
    password_hash character varying(255) NOT NULL,
    display_name character varying(120) NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    approved_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    role character varying(20) DEFAULT 'client'::character varying NOT NULL,
    referred_by_partner_id character(36),
    failed_login_count integer DEFAULT 0 NOT NULL,
    locked_until timestamp with time zone,
    two_factor_secret text,
    two_factor_enabled boolean DEFAULT false NOT NULL,
    two_factor_recovery_codes jsonb,
    registration_source character varying(20) DEFAULT 'public'::character varying NOT NULL,
    writer_generate_quota smallint,
    CONSTRAINT ck_users_role CHECK (((role)::text = ANY (ARRAY[('client'::character varying)::text, ('partner'::character varying)::text, ('admin'::character varying)::text]))),
    CONSTRAINT users_registration_source_check CHECK (((registration_source)::text = ANY (ARRAY[('public'::character varying)::text, ('admin'::character varying)::text, ('google'::character varying)::text])))
);


--
-- Name: voice_profiles; Type: TABLE; Schema: public; Owner: -
--

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


--
-- Name: wa_deliveries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wa_deliveries (
    id character(36) NOT NULL,
    user_id character(36),
    target character varying(30) NOT NULL,
    event character varying(64) NOT NULL,
    rendered_body text NOT NULL,
    status character varying(20) DEFAULT 'queued'::character varying NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    last_error text,
    dedupe_key character varying(200),
    reference_id character varying(120),
    sent_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT wa_deliveries_status_check CHECK (((status)::text = ANY (ARRAY[('queued'::character varying)::text, ('sent'::character varying)::text, ('failed'::character varying)::text])))
);


--
-- Name: wa_templates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.wa_templates (
    event character varying(64) NOT NULL,
    body text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    updated_by character(36),
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: websites; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.websites (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    domain character varying(253) NOT NULL,
    label character varying(120),
    language character varying(20) DEFAULT 'id-ID'::character varying NOT NULL,
    is_primary boolean DEFAULT false NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP(6) NOT NULL
);


--
-- Name: crawler_request_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.crawler_request_log ALTER COLUMN id SET DEFAULT nextval('public.crawler_request_log_id_seq'::regclass);


--
-- Name: affiliate_settings affiliate_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.affiliate_settings
    ADD CONSTRAINT affiliate_settings_pkey PRIMARY KEY (partner_id);


--
-- Name: ai_model_knowledge_chunks ai_model_knowledge_chunks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_model_knowledge_chunks
    ADD CONSTRAINT ai_model_knowledge_chunks_pkey PRIMARY KEY (id);


--
-- Name: ai_model_knowledge ai_model_knowledge_model_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_model_knowledge
    ADD CONSTRAINT ai_model_knowledge_model_id_key UNIQUE (model_id);


--
-- Name: ai_model_knowledge ai_model_knowledge_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_model_knowledge
    ADD CONSTRAINT ai_model_knowledge_pkey PRIMARY KEY (id);


--
-- Name: analytics_daily analytics_daily_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analytics_daily
    ADD CONSTRAINT analytics_daily_pkey PRIMARY KEY (id);


--
-- Name: audit_logs audit_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.audit_logs
    ADD CONSTRAINT audit_logs_pkey PRIMARY KEY (id);


--
-- Name: auth_rate_limits auth_rate_limits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auth_rate_limits
    ADD CONSTRAINT auth_rate_limits_pkey PRIMARY KEY (id);


--
-- Name: auto_replace_configs auto_replace_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auto_replace_configs
    ADD CONSTRAINT auto_replace_configs_pkey PRIMARY KEY (id);


--
-- Name: blogger_oauth_states blogger_oauth_states_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.blogger_oauth_states
    ADD CONSTRAINT blogger_oauth_states_pkey PRIMARY KEY (id);


--
-- Name: brand_ai_quotas brand_ai_quotas_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brand_ai_quotas
    ADD CONSTRAINT brand_ai_quotas_pkey PRIMARY KEY (brand_id);


--
-- Name: brand_memberships brand_memberships_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brand_memberships
    ADD CONSTRAINT brand_memberships_pkey PRIMARY KEY (id);


--
-- Name: brand_pr_standards brand_pr_standards_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brand_pr_standards
    ADD CONSTRAINT brand_pr_standards_pkey PRIMARY KEY (brand_id);


--
-- Name: brand_profiles brand_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brand_profiles
    ADD CONSTRAINT brand_profiles_pkey PRIMARY KEY (brand_id);


--
-- Name: brands brands_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brands
    ADD CONSTRAINT brands_pkey PRIMARY KEY (id);


--
-- Name: bug_reports bug_reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.bug_reports
    ADD CONSTRAINT bug_reports_pkey PRIMARY KEY (id);


--
-- Name: campaign_media_items campaign_media_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaign_media_items
    ADD CONSTRAINT campaign_media_items_pkey PRIMARY KEY (id);


--
-- Name: campaign_report_shares campaign_report_shares_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaign_report_shares
    ADD CONSTRAINT campaign_report_shares_pkey PRIMARY KEY (id);


--
-- Name: campaigns campaigns_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaigns
    ADD CONSTRAINT campaigns_pkey PRIMARY KEY (id);


--
-- Name: chat_learnings chat_learnings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.chat_learnings
    ADD CONSTRAINT chat_learnings_pkey PRIMARY KEY (id);


--
-- Name: cms_connections cms_connections_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cms_connections
    ADD CONSTRAINT cms_connections_pkey PRIMARY KEY (id);


--
-- Name: content_ai_analyses content_ai_analyses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_ai_analyses
    ADD CONSTRAINT content_ai_analyses_pkey PRIMARY KEY (id);


--
-- Name: content_batch_items content_batch_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_batch_items
    ADD CONSTRAINT content_batch_items_pkey PRIMARY KEY (id);


--
-- Name: content_batches content_batches_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_batches
    ADD CONSTRAINT content_batches_pkey PRIMARY KEY (id);


--
-- Name: content_briefs content_briefs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_briefs
    ADD CONSTRAINT content_briefs_pkey PRIMARY KEY (id);


--
-- Name: content_documents content_documents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_documents
    ADD CONSTRAINT content_documents_pkey PRIMARY KEY (id);


--
-- Name: content_intake_types content_intake_types_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_intake_types
    ADD CONSTRAINT content_intake_types_pkey PRIMARY KEY (id);


--
-- Name: content_intakes content_intakes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_intakes
    ADD CONSTRAINT content_intakes_pkey PRIMARY KEY (id);


--
-- Name: content_jobs content_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_jobs
    ADD CONSTRAINT content_jobs_pkey PRIMARY KEY (id);


--
-- Name: content_moderation_audits content_moderation_audits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_moderation_audits
    ADD CONSTRAINT content_moderation_audits_pkey PRIMARY KEY (id);


--
-- Name: content_moderation_results content_moderation_results_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_moderation_results
    ADD CONSTRAINT content_moderation_results_pkey PRIMARY KEY (id);


--
-- Name: content_type_standard_chunks content_type_standard_chunks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_type_standard_chunks
    ADD CONSTRAINT content_type_standard_chunks_pkey PRIMARY KEY (id);


--
-- Name: content_type_standard_versions content_type_standard_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_type_standard_versions
    ADD CONSTRAINT content_type_standard_versions_pkey PRIMARY KEY (id);


--
-- Name: content_type_standards content_type_standards_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_type_standards
    ADD CONSTRAINT content_type_standards_pkey PRIMARY KEY (id);


--
-- Name: content_versions content_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_versions
    ADD CONSTRAINT content_versions_pkey PRIMARY KEY (id);


--
-- Name: crawl_targets crawl_targets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.crawl_targets
    ADD CONSTRAINT crawl_targets_pkey PRIMARY KEY (media_id);


--
-- Name: crawler_credentials crawler_credentials_crawler_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.crawler_credentials
    ADD CONSTRAINT crawler_credentials_crawler_id_key UNIQUE (crawler_id);


--
-- Name: crawler_credentials crawler_credentials_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.crawler_credentials
    ADD CONSTRAINT crawler_credentials_pkey PRIMARY KEY (id);


--
-- Name: crawler_diagnostics crawler_diagnostics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.crawler_diagnostics
    ADD CONSTRAINT crawler_diagnostics_pkey PRIMARY KEY (id);


--
-- Name: crawler_nonces crawler_nonces_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.crawler_nonces
    ADD CONSTRAINT crawler_nonces_pkey PRIMARY KEY (nonce);


--
-- Name: crawler_request_log crawler_request_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.crawler_request_log
    ADD CONSTRAINT crawler_request_log_pkey PRIMARY KEY (id);


--
-- Name: credit_ledger_entries credit_ledger_entries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credit_ledger_entries
    ADD CONSTRAINT credit_ledger_entries_pkey PRIMARY KEY (id);


--
-- Name: credit_topup_reports credit_topup_reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credit_topup_reports
    ADD CONSTRAINT credit_topup_reports_pkey PRIMARY KEY (id);


--
-- Name: credit_transactions credit_transactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credit_transactions
    ADD CONSTRAINT credit_transactions_pkey PRIMARY KEY (id);


--
-- Name: credit_wallets credit_wallets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credit_wallets
    ADD CONSTRAINT credit_wallets_pkey PRIMARY KEY (brand_id);


--
-- Name: editorial_rule_sets editorial_rule_sets_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.editorial_rule_sets
    ADD CONSTRAINT editorial_rule_sets_pkey PRIMARY KEY (brand_id);


--
-- Name: email_deliveries email_deliveries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_deliveries
    ADD CONSTRAINT email_deliveries_pkey PRIMARY KEY (id);


--
-- Name: ga4_configs ga4_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ga4_configs
    ADD CONSTRAINT ga4_configs_pkey PRIMARY KEY (id);


--
-- Name: gsc_connections gsc_connections_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gsc_connections
    ADD CONSTRAINT gsc_connections_pkey PRIMARY KEY (id);


--
-- Name: gsc_oauth_states gsc_oauth_states_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gsc_oauth_states
    ADD CONSTRAINT gsc_oauth_states_pkey PRIMARY KEY (id);


--
-- Name: gsc_properties gsc_properties_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gsc_properties
    ADD CONSTRAINT gsc_properties_pkey PRIMARY KEY (id);


--
-- Name: gsc_snapshots gsc_snapshots_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gsc_snapshots
    ADD CONSTRAINT gsc_snapshots_pkey PRIMARY KEY (id);


--
-- Name: invoices invoices_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT invoices_pkey PRIMARY KEY (id);


--
-- Name: jobs jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_pkey PRIMARY KEY (id);


--
-- Name: knowledge_base_rule_chunks knowledge_base_rule_chunks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_base_rule_chunks
    ADD CONSTRAINT knowledge_base_rule_chunks_pkey PRIMARY KEY (id);


--
-- Name: knowledge_base_rules knowledge_base_rules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_base_rules
    ADD CONSTRAINT knowledge_base_rules_pkey PRIMARY KEY (id);


--
-- Name: knowledge_base_sources knowledge_base_sources_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_base_sources
    ADD CONSTRAINT knowledge_base_sources_pkey PRIMARY KEY (id);


--
-- Name: knowledge_base_versions knowledge_base_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_base_versions
    ADD CONSTRAINT knowledge_base_versions_pkey PRIMARY KEY (id);


--
-- Name: knowledge_bases knowledge_bases_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_bases
    ADD CONSTRAINT knowledge_bases_pkey PRIMARY KEY (id);


--
-- Name: knowledge_chunks knowledge_chunks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_chunks
    ADD CONSTRAINT knowledge_chunks_pkey PRIMARY KEY (id);


--
-- Name: knowledge_collections knowledge_collections_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_collections
    ADD CONSTRAINT knowledge_collections_pkey PRIMARY KEY (id);


--
-- Name: knowledge_crawl_runs knowledge_crawl_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_crawl_runs
    ADD CONSTRAINT knowledge_crawl_runs_pkey PRIMARY KEY (id);


--
-- Name: knowledge_sources knowledge_sources_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_sources
    ADD CONSTRAINT knowledge_sources_pkey PRIMARY KEY (id);


--
-- Name: master_prompt_versions master_prompt_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.master_prompt_versions
    ADD CONSTRAINT master_prompt_versions_pkey PRIMARY KEY (id);


--
-- Name: media_ai_summary_events media_ai_summary_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_ai_summary_events
    ADD CONSTRAINT media_ai_summary_events_pkey PRIMARY KEY (id);


--
-- Name: media_categories media_categories_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_categories
    ADD CONSTRAINT media_categories_pkey PRIMARY KEY (id);


--
-- Name: media_category_links media_category_links_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_category_links
    ADD CONSTRAINT media_category_links_pkey PRIMARY KEY (media_id, category_id);


--
-- Name: media_content_standards media_content_standards_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_content_standards
    ADD CONSTRAINT media_content_standards_pkey PRIMARY KEY (id);


--
-- Name: media_dewan_pers_verifications media_dewan_pers_verifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_dewan_pers_verifications
    ADD CONSTRAINT media_dewan_pers_verifications_pkey PRIMARY KEY (id);


--
-- Name: media_package_items media_package_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_package_items
    ADD CONSTRAINT media_package_items_pkey PRIMARY KEY (package_id, media_id);


--
-- Name: media_packages media_packages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_packages
    ADD CONSTRAINT media_packages_pkey PRIMARY KEY (id);


--
-- Name: media media_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media
    ADD CONSTRAINT media_pkey PRIMARY KEY (id);


--
-- Name: media_price_history media_price_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_price_history
    ADD CONSTRAINT media_price_history_pkey PRIMARY KEY (id);


--
-- Name: media_seo_metrics media_seo_metrics_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_seo_metrics
    ADD CONSTRAINT media_seo_metrics_pkey PRIMARY KEY (media_id);


--
-- Name: media_similarweb_snapshots media_similarweb_snapshots_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_similarweb_snapshots
    ADD CONSTRAINT media_similarweb_snapshots_pkey PRIMARY KEY (id);


--
-- Name: model_usage_events model_usage_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.model_usage_events
    ADD CONSTRAINT model_usage_events_pkey PRIMARY KEY (id);


--
-- Name: notification_preferences notification_preferences_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_preferences
    ADD CONSTRAINT notification_preferences_pkey PRIMARY KEY (user_id);


--
-- Name: notification_webhooks notification_webhooks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_webhooks
    ADD CONSTRAINT notification_webhooks_pkey PRIMARY KEY (id);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- Name: packages packages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.packages
    ADD CONSTRAINT packages_pkey PRIMARY KEY (id);


--
-- Name: partner_documents partner_documents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_documents
    ADD CONSTRAINT partner_documents_pkey PRIMARY KEY (id);


--
-- Name: partner_earnings partner_earnings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_earnings
    ADD CONSTRAINT partner_earnings_pkey PRIMARY KEY (id);


--
-- Name: partner_referral_commissions partner_referral_commissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_referral_commissions
    ADD CONSTRAINT partner_referral_commissions_pkey PRIMARY KEY (id);


--
-- Name: partner_seo_check_history partner_seo_check_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_seo_check_history
    ADD CONSTRAINT partner_seo_check_history_pkey PRIMARY KEY (id);


--
-- Name: partner_service_charges partner_service_charges_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_service_charges
    ADD CONSTRAINT partner_service_charges_pkey PRIMARY KEY (id);


--
-- Name: partner_speed_audits partner_speed_audits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_speed_audits
    ADD CONSTRAINT partner_speed_audits_pkey PRIMARY KEY (id);


--
-- Name: partners partners_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partners
    ADD CONSTRAINT partners_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_token_hash_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_token_hash_key UNIQUE (token_hash);


--
-- Name: payment_webhook_events payment_webhook_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_webhook_events
    ADD CONSTRAINT payment_webhook_events_pkey PRIMARY KEY (id);


--
-- Name: payment_gateway_configs pk_payment_gateway_configs; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_gateway_configs
    ADD CONSTRAINT pk_payment_gateway_configs PRIMARY KEY (id);


--
-- Name: payment_transactions pk_payment_transactions; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_transactions
    ADD CONSTRAINT pk_payment_transactions PRIMARY KEY (id);


--
-- Name: platform_admins platform_admins_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_admins
    ADD CONSTRAINT platform_admins_pkey PRIMARY KEY (user_id);


--
-- Name: platform_ai_configuration platform_ai_configuration_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_ai_configuration
    ADD CONSTRAINT platform_ai_configuration_pkey PRIMARY KEY (id);


--
-- Name: platform_crawlhub_config platform_crawlhub_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_crawlhub_config
    ADD CONSTRAINT platform_crawlhub_config_pkey PRIMARY KEY (id);


--
-- Name: platform_gsc_config platform_gsc_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_gsc_config
    ADD CONSTRAINT platform_gsc_config_pkey PRIMARY KEY (id);


--
-- Name: platform_oauth_configs platform_oauth_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_oauth_configs
    ADD CONSTRAINT platform_oauth_configs_pkey PRIMARY KEY (id);


--
-- Name: platform_pagespeed_config platform_pagespeed_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_pagespeed_config
    ADD CONSTRAINT platform_pagespeed_config_pkey PRIMARY KEY (id);


--
-- Name: platform_seo_config platform_seo_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_seo_config
    ADD CONSTRAINT platform_seo_config_pkey PRIMARY KEY (id);


--
-- Name: platform_server_config platform_server_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_server_config
    ADD CONSTRAINT platform_server_config_pkey PRIMARY KEY (id);


--
-- Name: platform_settings platform_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_settings
    ADD CONSTRAINT platform_settings_pkey PRIMARY KEY (key);


--
-- Name: platform_turnstile_config platform_turnstile_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_turnstile_config
    ADD CONSTRAINT platform_turnstile_config_pkey PRIMARY KEY (id);


--
-- Name: platform_wa_config platform_wa_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_wa_config
    ADD CONSTRAINT platform_wa_config_pkey PRIMARY KEY (id);


--
-- Name: pr_campaign_scores pr_campaign_scores_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_campaign_scores
    ADD CONSTRAINT pr_campaign_scores_pkey PRIMARY KEY (id);


--
-- Name: pr_config pr_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_config
    ADD CONSTRAINT pr_config_pkey PRIMARY KEY (id);


--
-- Name: pr_media_rules pr_media_rules_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_media_rules
    ADD CONSTRAINT pr_media_rules_pkey PRIMARY KEY (id);


--
-- Name: pr_message_analyses pr_message_analyses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_message_analyses
    ADD CONSTRAINT pr_message_analyses_pkey PRIMARY KEY (id);


--
-- Name: pr_metric_results pr_metric_results_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_metric_results
    ADD CONSTRAINT pr_metric_results_pkey PRIMARY KEY (id);


--
-- Name: pr_publication_verifications pr_publication_verifications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_publication_verifications
    ADD CONSTRAINT pr_publication_verifications_pkey PRIMARY KEY (id);


--
-- Name: pr_scoring_profiles pr_scoring_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_scoring_profiles
    ADD CONSTRAINT pr_scoring_profiles_pkey PRIMARY KEY (id);


--
-- Name: pr_sentiment_analyses pr_sentiment_analyses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_sentiment_analyses
    ADD CONSTRAINT pr_sentiment_analyses_pkey PRIMARY KEY (id);


--
-- Name: pricing_config pricing_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pricing_config
    ADD CONSTRAINT pricing_config_pkey PRIMARY KEY (id);


--
-- Name: publication_attempts publication_attempts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_attempts
    ADD CONSTRAINT publication_attempts_pkey PRIMARY KEY (id);


--
-- Name: publication_jobs publication_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_jobs
    ADD CONSTRAINT publication_jobs_pkey PRIMARY KEY (id);


--
-- Name: publication_link_checks publication_link_checks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_link_checks
    ADD CONSTRAINT publication_link_checks_pkey PRIMARY KEY (publication_id);


--
-- Name: publication_plan_media publication_plan_media_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_plan_media
    ADD CONSTRAINT publication_plan_media_pkey PRIMARY KEY (plan_id, media_id);


--
-- Name: publication_plan_occurrences publication_plan_occurrences_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_plan_occurrences
    ADD CONSTRAINT publication_plan_occurrences_pkey PRIMARY KEY (id);


--
-- Name: publication_plan_occurrences publication_plan_occurrences_plan_id_media_id_scheduled_at_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_plan_occurrences
    ADD CONSTRAINT publication_plan_occurrences_plan_id_media_id_scheduled_at_key UNIQUE (plan_id, media_id, scheduled_at);


--
-- Name: publication_plans publication_plans_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_plans
    ADD CONSTRAINT publication_plans_pkey PRIMARY KEY (id);


--
-- Name: publication_variant_batches publication_variant_batches_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_variant_batches
    ADD CONSTRAINT publication_variant_batches_pkey PRIMARY KEY (id);


--
-- Name: publication_variants publication_variants_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_variants
    ADD CONSTRAINT publication_variants_pkey PRIMARY KEY (id);


--
-- Name: publications publications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT publications_pkey PRIMARY KEY (id);


--
-- Name: publishing_api_keys publishing_api_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publishing_api_keys
    ADD CONSTRAINT publishing_api_keys_pkey PRIMARY KEY (id);


--
-- Name: publishing_callback_logs publishing_callback_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publishing_callback_logs
    ADD CONSTRAINT publishing_callback_logs_pkey PRIMARY KEY (id);


--
-- Name: quality_evaluations quality_evaluations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.quality_evaluations
    ADD CONSTRAINT quality_evaluations_pkey PRIMARY KEY (id);


--
-- Name: referral_attributions referral_attributions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.referral_attributions
    ADD CONSTRAINT referral_attributions_pkey PRIMARY KEY (id);


--
-- Name: referral_clicks referral_clicks_code_fingerprint_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.referral_clicks
    ADD CONSTRAINT referral_clicks_code_fingerprint_key UNIQUE (code, fingerprint);


--
-- Name: referral_clicks referral_clicks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.referral_clicks
    ADD CONSTRAINT referral_clicks_pkey PRIMARY KEY (id);


--
-- Name: referral_settings referral_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.referral_settings
    ADD CONSTRAINT referral_settings_pkey PRIMARY KEY (key);


--
-- Name: referred_domains referred_domains_media_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.referred_domains
    ADD CONSTRAINT referred_domains_media_id_key UNIQUE (media_id);


--
-- Name: referred_domains referred_domains_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.referred_domains
    ADD CONSTRAINT referred_domains_pkey PRIMARY KEY (id);


--
-- Name: registration_guard_config registration_guard_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.registration_guard_config
    ADD CONSTRAINT registration_guard_config_pkey PRIMARY KEY (id);


--
-- Name: registration_history registration_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.registration_history
    ADD CONSTRAINT registration_history_pkey PRIMARY KEY (id);


--
-- Name: revenue_shares revenue_shares_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.revenue_shares
    ADD CONSTRAINT revenue_shares_pkey PRIMARY KEY (id);


--
-- Name: review_comments review_comments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.review_comments
    ADD CONSTRAINT review_comments_pkey PRIMARY KEY (id);


--
-- Name: rss_access_logs rss_access_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rss_access_logs
    ADD CONSTRAINT rss_access_logs_pkey PRIMARY KEY (id);


--
-- Name: rss_feeds rss_feeds_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rss_feeds
    ADD CONSTRAINT rss_feeds_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: settlements settlements_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlements
    ADD CONSTRAINT settlements_pkey PRIMARY KEY (id);


--
-- Name: smtp_settings smtp_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smtp_settings
    ADD CONSTRAINT smtp_settings_pkey PRIMARY KEY (id);


--
-- Name: system_metadata system_metadata_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_metadata
    ADD CONSTRAINT system_metadata_pkey PRIMARY KEY (key);


--
-- Name: taut_agent_configs taut_agent_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_agent_configs
    ADD CONSTRAINT taut_agent_configs_pkey PRIMARY KEY (agent);


--
-- Name: taut_agent_runs taut_agent_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_agent_runs
    ADD CONSTRAINT taut_agent_runs_pkey PRIMARY KEY (id);


--
-- Name: taut_campaign_files taut_campaign_files_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_campaign_files
    ADD CONSTRAINT taut_campaign_files_pkey PRIMARY KEY (id);


--
-- Name: taut_campaign_media taut_campaign_media_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_campaign_media
    ADD CONSTRAINT taut_campaign_media_pkey PRIMARY KEY (id);


--
-- Name: taut_content_angle_prompt_versions taut_content_angle_prompt_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_content_angle_prompt_versions
    ADD CONSTRAINT taut_content_angle_prompt_versions_pkey PRIMARY KEY (id);


--
-- Name: taut_content_angles taut_content_angles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_content_angles
    ADD CONSTRAINT taut_content_angles_pkey PRIMARY KEY (id);


--
-- Name: taut_gatekeeper_audits taut_gatekeeper_audits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_gatekeeper_audits
    ADD CONSTRAINT taut_gatekeeper_audits_pkey PRIMARY KEY (id);


--
-- Name: taut_media_channels taut_media_channels_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_media_channels
    ADD CONSTRAINT taut_media_channels_pkey PRIMARY KEY (id);


--
-- Name: taut_profiler_base_prompt_versions taut_profiler_base_prompt_versions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_profiler_base_prompt_versions
    ADD CONSTRAINT taut_profiler_base_prompt_versions_pkey PRIMARY KEY (id);


--
-- Name: taut_profiler_results taut_profiler_results_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_profiler_results
    ADD CONSTRAINT taut_profiler_results_pkey PRIMARY KEY (id);


--
-- Name: taut_profiler_submission_channels taut_profiler_submission_channels_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_profiler_submission_channels
    ADD CONSTRAINT taut_profiler_submission_channels_pkey PRIMARY KEY (id);


--
-- Name: taut_profiler_submissions taut_profiler_submissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_profiler_submissions
    ADD CONSTRAINT taut_profiler_submissions_pkey PRIMARY KEY (id);


--
-- Name: taut_training_samples taut_training_samples_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_training_samples
    ADD CONSTRAINT taut_training_samples_pkey PRIMARY KEY (id);


--
-- Name: tc_config tc_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tc_config
    ADD CONSTRAINT tc_config_pkey PRIMARY KEY (id);


--
-- Name: tracking_events tracking_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tracking_events
    ADD CONSTRAINT tracking_events_pkey PRIMARY KEY (id);


--
-- Name: tracking_links tracking_links_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tracking_links
    ADD CONSTRAINT tracking_links_pkey PRIMARY KEY (id);


--
-- Name: transactions transactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT transactions_pkey PRIMARY KEY (id);


--
-- Name: ai_model_knowledge_chunks uq_ai_model_knowledge_chunks; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_model_knowledge_chunks
    ADD CONSTRAINT uq_ai_model_knowledge_chunks UNIQUE (knowledge_id, section, chunk_index);


--
-- Name: blogger_oauth_states uq_blogger_oauth_states_state; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.blogger_oauth_states
    ADD CONSTRAINT uq_blogger_oauth_states_state UNIQUE (state);


--
-- Name: brand_memberships uq_brand_membership_brand_user; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brand_memberships
    ADD CONSTRAINT uq_brand_membership_brand_user UNIQUE (brand_id, user_id);


--
-- Name: brand_memberships uq_brand_memberships_brand_user; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brand_memberships
    ADD CONSTRAINT uq_brand_memberships_brand_user UNIQUE (brand_id, user_id);


--
-- Name: brands uq_brands_public_id; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brands
    ADD CONSTRAINT uq_brands_public_id UNIQUE (public_id);


--
-- Name: brands uq_brands_slug; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brands
    ADD CONSTRAINT uq_brands_slug UNIQUE (slug);


--
-- Name: campaign_media_items uq_campaign_media; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaign_media_items
    ADD CONSTRAINT uq_campaign_media UNIQUE (campaign_id, media_id);


--
-- Name: campaign_report_shares uq_campaign_report_campaign; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaign_report_shares
    ADD CONSTRAINT uq_campaign_report_campaign UNIQUE (campaign_id);


--
-- Name: campaign_report_shares uq_campaign_report_token; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaign_report_shares
    ADD CONSTRAINT uq_campaign_report_token UNIQUE (token);


--
-- Name: cms_connections uq_cms_connection_brand_name; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cms_connections
    ADD CONSTRAINT uq_cms_connection_brand_name UNIQUE (brand_id, name);


--
-- Name: content_batch_items uq_content_batch_fingerprint; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_batch_items
    ADD CONSTRAINT uq_content_batch_fingerprint UNIQUE (batch_id, fingerprint);


--
-- Name: content_batch_items uq_content_batch_row; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_batch_items
    ADD CONSTRAINT uq_content_batch_row UNIQUE (batch_id, row_number);


--
-- Name: content_documents uq_content_document_job; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_documents
    ADD CONSTRAINT uq_content_document_job UNIQUE (job_id);


--
-- Name: content_intake_types uq_content_intake_type_code; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_intake_types
    ADD CONSTRAINT uq_content_intake_type_code UNIQUE (code);


--
-- Name: content_jobs uq_content_job_idempotency; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_jobs
    ADD CONSTRAINT uq_content_job_idempotency UNIQUE (brand_id, idempotency_key);


--
-- Name: content_type_standards uq_content_type_standards_code; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_type_standards
    ADD CONSTRAINT uq_content_type_standards_code UNIQUE (content_type_code);


--
-- Name: content_type_standard_chunks uq_content_type_std_chunks; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_type_standard_chunks
    ADD CONSTRAINT uq_content_type_std_chunks UNIQUE (version_id, section, chunk_index);


--
-- Name: content_type_standard_versions uq_content_type_std_ver; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_type_standard_versions
    ADD CONSTRAINT uq_content_type_std_ver UNIQUE (standard_id, version);


--
-- Name: content_versions uq_content_versions_document_version; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_versions
    ADD CONSTRAINT uq_content_versions_document_version UNIQUE (document_id, version);


--
-- Name: ga4_configs uq_ga4_config_brand; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ga4_configs
    ADD CONSTRAINT uq_ga4_config_brand UNIQUE (brand_id);


--
-- Name: gsc_oauth_states uq_gsc_oauth_state; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gsc_oauth_states
    ADD CONSTRAINT uq_gsc_oauth_state UNIQUE (state);


--
-- Name: gsc_properties uq_gsc_properties_connection_site; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gsc_properties
    ADD CONSTRAINT uq_gsc_properties_connection_site UNIQUE (connection_id, site_url);


--
-- Name: invoices uq_invoice_number; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT uq_invoice_number UNIQUE (number);


--
-- Name: knowledge_base_rule_chunks uq_kb_rule_chunks_rule_index; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_base_rule_chunks
    ADD CONSTRAINT uq_kb_rule_chunks_rule_index UNIQUE (rule_id, chunk_index);


--
-- Name: knowledge_base_versions uq_knowledge_base_versions_version; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_base_versions
    ADD CONSTRAINT uq_knowledge_base_versions_version UNIQUE (knowledge_base_id, version);


--
-- Name: knowledge_bases uq_knowledge_bases_code; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_bases
    ADD CONSTRAINT uq_knowledge_bases_code UNIQUE (code);


--
-- Name: knowledge_collections uq_knowledge_collection_brand_name; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_collections
    ADD CONSTRAINT uq_knowledge_collection_brand_name UNIQUE (brand_id, name);


--
-- Name: knowledge_chunks uq_knowledge_source_chunk; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_chunks
    ADD CONSTRAINT uq_knowledge_source_chunk UNIQUE (source_id, chunk_index);


--
-- Name: master_prompt_versions uq_master_prompt_brand_version; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.master_prompt_versions
    ADD CONSTRAINT uq_master_prompt_brand_version UNIQUE (brand_id, version);


--
-- Name: media_categories uq_media_categories_normalized; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_categories
    ADD CONSTRAINT uq_media_categories_normalized UNIQUE (normalized);


--
-- Name: media_content_standards uq_media_content_standard_media; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_content_standards
    ADD CONSTRAINT uq_media_content_standard_media UNIQUE (media_id);


--
-- Name: media uq_media_slug; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media
    ADD CONSTRAINT uq_media_slug UNIQUE (slug);


--
-- Name: partner_earnings uq_partner_earnings_publication; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_earnings
    ADD CONSTRAINT uq_partner_earnings_publication UNIQUE (publication_id);


--
-- Name: partners uq_partners_public_id; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partners
    ADD CONSTRAINT uq_partners_public_id UNIQUE (public_id);


--
-- Name: partners uq_partners_user; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partners
    ADD CONSTRAINT uq_partners_user UNIQUE (user_id);


--
-- Name: payment_gateway_configs uq_payment_gateway_provider; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_gateway_configs
    ADD CONSTRAINT uq_payment_gateway_provider UNIQUE (provider);


--
-- Name: payment_webhook_events uq_payment_webhook_event; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.payment_webhook_events
    ADD CONSTRAINT uq_payment_webhook_event UNIQUE (gateway_provider, external_transaction_id, status);


--
-- Name: pr_campaign_scores uq_pr_campaign_profile; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_campaign_scores
    ADD CONSTRAINT uq_pr_campaign_profile UNIQUE (campaign_id, profile_version);


--
-- Name: pr_message_analyses uq_pr_message_analysis_publication; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_message_analyses
    ADD CONSTRAINT uq_pr_message_analysis_publication UNIQUE (publication_id);


--
-- Name: pr_metric_results uq_pr_metric_campaign_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_metric_results
    ADD CONSTRAINT uq_pr_metric_campaign_key UNIQUE (campaign_id, metric_key);


--
-- Name: pr_scoring_profiles uq_pr_scoring_profile_version; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_scoring_profiles
    ADD CONSTRAINT uq_pr_scoring_profile_version UNIQUE (version);


--
-- Name: pr_sentiment_analyses uq_pr_sentiment_analysis_publication; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_sentiment_analyses
    ADD CONSTRAINT uq_pr_sentiment_analysis_publication UNIQUE (publication_id);


--
-- Name: pr_publication_verifications uq_pr_verification_publication; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_publication_verifications
    ADD CONSTRAINT uq_pr_verification_publication UNIQUE (publication_id);


--
-- Name: publication_attempts uq_publication_attempt_number; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_attempts
    ADD CONSTRAINT uq_publication_attempt_number UNIQUE (publication_id, attempt_number);


--
-- Name: publication_jobs uq_publication_document_connection; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_jobs
    ADD CONSTRAINT uq_publication_document_connection UNIQUE (document_id, connection_id);


--
-- Name: publications uq_publication_document_media; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT uq_publication_document_media UNIQUE (document_id, media_id);


--
-- Name: publishing_api_keys uq_publishing_api_key_hash; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publishing_api_keys
    ADD CONSTRAINT uq_publishing_api_key_hash UNIQUE (api_key_hash);


--
-- Name: publishing_api_keys uq_publishing_api_key_partner_label; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publishing_api_keys
    ADD CONSTRAINT uq_publishing_api_key_partner_label UNIQUE (partner_id, label);


--
-- Name: rss_feeds uq_rss_feed_partner_name; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rss_feeds
    ADD CONSTRAINT uq_rss_feed_partner_name UNIQUE (partner_id, name);


--
-- Name: rss_feeds uq_rss_feed_token; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rss_feeds
    ADD CONSTRAINT uq_rss_feed_token UNIQUE (token);


--
-- Name: tracking_links uq_tracking_link_code; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tracking_links
    ADD CONSTRAINT uq_tracking_link_code UNIQUE (code);


--
-- Name: tracking_links uq_tracking_link_publication; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tracking_links
    ADD CONSTRAINT uq_tracking_link_publication UNIQUE (publication_id);


--
-- Name: user_sessions uq_user_sessions_refresh_jti; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT uq_user_sessions_refresh_jti UNIQUE (refresh_jti);


--
-- Name: users uq_users_email; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT uq_users_email UNIQUE (email);


--
-- Name: publication_variants uq_variant_batch_media; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_variants
    ADD CONSTRAINT uq_variant_batch_media UNIQUE (batch_id, media_id);


--
-- Name: websites uq_website_brand_domain; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.websites
    ADD CONSTRAINT uq_website_brand_domain UNIQUE (brand_id, domain);


--
-- Name: user_daily_quotas user_daily_quotas_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_daily_quotas
    ADD CONSTRAINT user_daily_quotas_pkey PRIMARY KEY (user_id);


--
-- Name: user_oauth_accounts user_oauth_accounts_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_oauth_accounts
    ADD CONSTRAINT user_oauth_accounts_pkey PRIMARY KEY (id);


--
-- Name: user_sessions user_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT user_sessions_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: voice_profiles voice_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_profiles
    ADD CONSTRAINT voice_profiles_pkey PRIMARY KEY (brand_id);


--
-- Name: wa_deliveries wa_deliveries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wa_deliveries
    ADD CONSTRAINT wa_deliveries_pkey PRIMARY KEY (id);


--
-- Name: wa_templates wa_templates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wa_templates
    ADD CONSTRAINT wa_templates_pkey PRIMARY KEY (event);


--
-- Name: websites websites_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.websites
    ADD CONSTRAINT websites_pkey PRIMARY KEY (id);


--
-- Name: idx_ai_model_chunks_model; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_model_chunks_model ON public.ai_model_knowledge_chunks USING btree (model_id);


--
-- Name: idx_ai_model_chunks_section; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_model_chunks_section ON public.ai_model_knowledge_chunks USING btree (section);


--
-- Name: idx_ai_model_knowledge_model; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_model_knowledge_model ON public.ai_model_knowledge USING btree (model_id);


--
-- Name: idx_ai_model_knowledge_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_ai_model_knowledge_status ON public.ai_model_knowledge USING btree (status);


--
-- Name: idx_campaigns_brand_archived; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_campaigns_brand_archived ON public.campaigns USING btree (brand_id, archived_at);


--
-- Name: idx_content_type_std_chunks_code; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_type_std_chunks_code ON public.content_type_standard_chunks USING btree (content_type_code);


--
-- Name: idx_content_type_std_chunks_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_content_type_std_chunks_version ON public.content_type_standard_chunks USING btree (version_id);


--
-- Name: idx_credit_ledger_brand; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_credit_ledger_brand ON public.credit_ledger_entries USING btree (brand_id, created_at DESC);


--
-- Name: idx_credit_ledger_ref; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_credit_ledger_ref ON public.credit_ledger_entries USING btree (ref_id, ref_type);


--
-- Name: idx_kb_rule_chunks_rule; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_rule_chunks_rule ON public.knowledge_base_rule_chunks USING btree (rule_id);


--
-- Name: idx_kb_rules_base_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_rules_base_status ON public.knowledge_base_rules USING btree (knowledge_base_id, status);


--
-- Name: idx_kb_rules_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_rules_version ON public.knowledge_base_rules USING btree (version_id);


--
-- Name: idx_kb_sources_base; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_kb_sources_base ON public.knowledge_base_sources USING btree (knowledge_base_id);


--
-- Name: idx_media_domain_recheck; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_media_domain_recheck ON public.media USING btree (domain_verified, domain_last_checked_at);


--
-- Name: idx_moderation_audits_brand_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_moderation_audits_brand_status ON public.content_moderation_audits USING btree (brand_id, status, created_at DESC);


--
-- Name: idx_moderation_audits_content_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_moderation_audits_content_hash ON public.content_moderation_audits USING btree (content_hash);


--
-- Name: idx_moderation_audits_entity; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_moderation_audits_entity ON public.content_moderation_audits USING btree (entity_type, entity_id);


--
-- Name: idx_moderation_audits_review; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_moderation_audits_review ON public.content_moderation_audits USING btree (status, reviewed_at) WHERE ((status)::text = 'review_required'::text);


--
-- Name: idx_moderation_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_moderation_created_at ON public.content_moderation_results USING btree (created_at DESC);


--
-- Name: idx_moderation_hash_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_moderation_hash_version ON public.content_moderation_results USING btree (content_hash, moderation_version);


--
-- Name: idx_notifications_dedupe; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_notifications_dedupe ON public.notifications USING btree (user_id, dedupe_key) WHERE (dedupe_key IS NOT NULL);


--
-- Name: idx_oauth_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_oauth_email ON public.user_oauth_accounts USING btree (email);


--
-- Name: idx_oauth_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_oauth_user ON public.user_oauth_accounts USING btree (user_id);


--
-- Name: idx_pr_verifications_campaign; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_pr_verifications_campaign ON public.pr_publication_verifications USING btree (campaign_id);


--
-- Name: idx_psa_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_psa_created ON public.partner_speed_audits USING btree (partner_id, created_at DESC);


--
-- Name: idx_psa_domain; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_psa_domain ON public.partner_speed_audits USING btree (domain);


--
-- Name: idx_psa_partner_url; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_psa_partner_url ON public.partner_speed_audits USING btree (partner_id, normalized_url);


--
-- Name: idx_taut_gatekeeper_audits_campaign; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_taut_gatekeeper_audits_campaign ON public.taut_gatekeeper_audits USING btree (campaign_id, created_at DESC);


--
-- Name: idx_taut_gatekeeper_audits_media; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_taut_gatekeeper_audits_media ON public.taut_gatekeeper_audits USING btree (media_id, created_at DESC);


--
-- Name: idx_taut_training_agent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_taut_training_agent ON public.taut_training_samples USING btree (agent, created_at DESC);


--
-- Name: idx_taut_training_campaign; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_taut_training_campaign ON public.taut_training_samples USING btree (campaign_id);


--
-- Name: idx_tracking_links_campaign; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tracking_links_campaign ON public.tracking_links USING btree (campaign_id, kind);


--
-- Name: ix_analytics_daily_campaign_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_analytics_daily_campaign_date ON public.analytics_daily USING btree (brand_id, campaign_id, event_date);


--
-- Name: ix_audit_brand_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_audit_brand_created ON public.audit_logs USING btree (brand_id, created_at DESC);


--
-- Name: ix_auth_rate_limits_window; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_auth_rate_limits_window ON public.auth_rate_limits USING btree (window_start);


--
-- Name: ix_blogger_oauth_states_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_blogger_oauth_states_created ON public.blogger_oauth_states USING btree (created_at DESC);


--
-- Name: ix_bug_report_status_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_bug_report_status_created ON public.bug_reports USING btree (status, created_at);


--
-- Name: ix_bug_report_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_bug_report_user ON public.bug_reports USING btree (user_id);


--
-- Name: ix_callback_logs_key_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_callback_logs_key_time ON public.publishing_callback_logs USING btree (api_key_id, created_at DESC);


--
-- Name: ix_callback_logs_pub_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_callback_logs_pub_time ON public.publishing_callback_logs USING btree (publication_id, created_at DESC);


--
-- Name: ix_campaign_brand_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_campaign_brand_status ON public.campaigns USING btree (brand_id, status);


--
-- Name: ix_campaign_media_items_campaign; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_campaign_media_items_campaign ON public.campaign_media_items USING btree (campaign_id);


--
-- Name: ix_campaign_media_items_media; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_campaign_media_items_media ON public.campaign_media_items USING btree (media_id);


--
-- Name: ix_campaign_report_shares_brand; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_campaign_report_shares_brand ON public.campaign_report_shares USING btree (brand_id);


--
-- Name: ix_campaigns_intake; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_campaigns_intake ON public.campaigns USING btree (content_intake_id);


--
-- Name: ix_campaigns_profiler_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_campaigns_profiler_status ON public.campaigns USING btree (brand_id, taut_status) WHERE ((taut_status)::text = ANY (ARRAY[('profiling'::character varying)::text, ('profiled'::character varying)::text]));


--
-- Name: ix_campaigns_selected_media; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_campaigns_selected_media ON public.campaigns USING btree (selected_media_id);


--
-- Name: ix_campaigns_taut_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_campaigns_taut_status ON public.campaigns USING btree (brand_id, taut_status);


--
-- Name: ix_chat_learning_brand_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_chat_learning_brand_created ON public.chat_learnings USING btree (brand_id, created_at);


--
-- Name: ix_chat_learning_conv; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_chat_learning_conv ON public.chat_learnings USING btree (conversation_id, created_at);


--
-- Name: ix_chat_learning_intent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_chat_learning_intent ON public.chat_learnings USING btree (brand_id, intent_tag);


--
-- Name: ix_cms_connection_brand_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_cms_connection_brand_status ON public.cms_connections USING btree (brand_id, status);


--
-- Name: ix_cms_connection_partner_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_cms_connection_partner_status ON public.cms_connections USING btree (partner_id, status);


--
-- Name: ix_content_ai_analyses_campaign; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_content_ai_analyses_campaign ON public.content_ai_analyses USING btree (campaign_id);


--
-- Name: ix_content_ai_analyses_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_content_ai_analyses_hash ON public.content_ai_analyses USING btree (content_hash);


--
-- Name: ix_content_ai_analyses_intake; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_content_ai_analyses_intake ON public.content_ai_analyses USING btree (content_intake_id);


--
-- Name: ix_content_ai_analyses_media; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_content_ai_analyses_media ON public.content_ai_analyses USING btree (media_id);


--
-- Name: ix_content_batch_brand_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_content_batch_brand_created ON public.content_batches USING btree (brand_id, created_at);


--
-- Name: ix_content_batch_item_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_content_batch_item_status ON public.content_batch_items USING btree (batch_id, status);


--
-- Name: ix_content_brief_brand_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_content_brief_brand_created ON public.content_briefs USING btree (brand_id, created_at);


--
-- Name: ix_content_document_brand_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_content_document_brand_status ON public.content_documents USING btree (brand_id, status);


--
-- Name: ix_content_documents_brand_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_content_documents_brand_created ON public.content_documents USING btree (brand_id, created_at DESC);


--
-- Name: ix_content_intakes_brand_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_content_intakes_brand_created ON public.content_intakes USING btree (brand_id, created_at DESC);


--
-- Name: ix_content_job_brand_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_content_job_brand_created ON public.content_jobs USING btree (brand_id, created_at);


--
-- Name: ix_content_jobs_brand_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_content_jobs_brand_status ON public.content_jobs USING btree (brand_id, status, created_at DESC);


--
-- Name: ix_content_versions_document_version; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_content_versions_document_version ON public.content_versions USING btree (document_id, version);


--
-- Name: ix_crawl_targets_crawled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_crawl_targets_crawled ON public.crawl_targets USING btree (last_crawled_at);


--
-- Name: ix_crawler_credentials_partner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_crawler_credentials_partner ON public.crawler_credentials USING btree (partner_id, status);


--
-- Name: ix_crawler_diagnostics_media_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_crawler_diagnostics_media_time ON public.crawler_diagnostics USING btree (media_id, created_at DESC);


--
-- Name: ix_crawler_nonces_expires; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_crawler_nonces_expires ON public.crawler_nonces USING btree (expires_at);


--
-- Name: ix_crawler_request_log; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_crawler_request_log ON public.crawler_request_log USING btree (crawler_id, created_at);


--
-- Name: ix_credit_ledger_brand; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_credit_ledger_brand ON public.credit_ledger_entries USING btree (brand_id, created_at DESC);


--
-- Name: ix_credit_ledger_ref; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_credit_ledger_ref ON public.credit_ledger_entries USING btree (ref_id, ref_type);


--
-- Name: ix_credit_topup_reports_brand; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_credit_topup_reports_brand ON public.credit_topup_reports USING btree (brand_id, created_at DESC);


--
-- Name: ix_credit_topup_reports_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_credit_topup_reports_status ON public.credit_topup_reports USING btree (status, created_at DESC);


--
-- Name: ix_credit_tx_brand_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_credit_tx_brand_time ON public.credit_transactions USING btree (brand_id, created_at DESC);


--
-- Name: ix_credit_tx_publication; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_credit_tx_publication ON public.credit_transactions USING btree (publication_id);


--
-- Name: ix_email_deliveries_dedupe; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_email_deliveries_dedupe ON public.email_deliveries USING btree (template, user_id, dedupe_key, created_at DESC);


--
-- Name: ix_email_deliveries_queue; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_email_deliveries_queue ON public.email_deliveries USING btree (status, next_attempt_at) WHERE ((status)::text = ANY (ARRAY[('queued'::character varying)::text, ('failed'::character varying)::text]));


--
-- Name: ix_email_deliveries_reference; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_email_deliveries_reference ON public.email_deliveries USING btree (reference_id);


--
-- Name: ix_email_deliveries_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_email_deliveries_user ON public.email_deliveries USING btree (user_id, created_at DESC);


--
-- Name: ix_ga4_config_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_ga4_config_status ON public.ga4_configs USING btree (brand_id, enabled);


--
-- Name: ix_gsc_oauth_states_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_gsc_oauth_states_created ON public.gsc_oauth_states USING btree (created_at DESC);


--
-- Name: ix_gsc_snapshots_media_fetched; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_gsc_snapshots_media_fetched ON public.gsc_snapshots USING btree (media_id, fetched_at DESC);


--
-- Name: ix_gsc_snapshots_property_fetched; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_gsc_snapshots_property_fetched ON public.gsc_snapshots USING btree (property_id, fetched_at DESC);


--
-- Name: ix_invoice_brand_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_invoice_brand_status ON public.invoices USING btree (brand_id, status);


--
-- Name: ix_jobs_claim; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_jobs_claim ON public.jobs USING btree (status, run_at);


--
-- Name: ix_jobs_live_publication; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_jobs_live_publication ON public.jobs USING btree (((payload ->> 'publicationId'::text))) WHERE (((kind)::text = 'publication_execution'::text) AND ((status)::text <> ALL (ARRAY[('completed'::character varying)::text, ('failed'::character varying)::text])));


--
-- Name: ix_knowledge_chunk_brand; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_knowledge_chunk_brand ON public.knowledge_chunks USING btree (brand_id);


--
-- Name: ix_mase_media_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_mase_media_time ON public.media_ai_summary_events USING btree (media_id, created_at DESC);


--
-- Name: ix_mase_status_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_mase_status_time ON public.media_ai_summary_events USING btree (status, created_at DESC);


--
-- Name: ix_master_prompt_brand_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_master_prompt_brand_status ON public.master_prompt_versions USING btree (brand_id, status);


--
-- Name: ix_mdpv_cert_no; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_mdpv_cert_no ON public.media_dewan_pers_verifications USING btree (certificate_no, status);


--
-- Name: ix_mdpv_media_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_mdpv_media_status ON public.media_dewan_pers_verifications USING btree (media_id, status);


--
-- Name: ix_mdpv_partner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_mdpv_partner ON public.media_dewan_pers_verifications USING btree (partner_id, submitted_at DESC);


--
-- Name: ix_media_auto_publish; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_media_auto_publish ON public.media USING btree (auto_publish) WHERE (auto_publish = true);


--
-- Name: ix_media_categories_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_media_categories_name ON public.media_categories USING btree (name);


--
-- Name: ix_media_category_links_category; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_media_category_links_category ON public.media_category_links USING btree (category_id);


--
-- Name: ix_media_category_links_media; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_media_category_links_media ON public.media_category_links USING btree (media_id);


--
-- Name: ix_media_domain_last_result; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_media_domain_last_result ON public.media USING btree (domain_last_result, domain_last_error_at DESC);


--
-- Name: ix_media_package_items_media; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_media_package_items_media ON public.media_package_items USING btree (media_id);


--
-- Name: ix_media_packages_source_brand; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_media_packages_source_brand ON public.media_packages USING btree (source, brand_id);


--
-- Name: ix_media_partner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_media_partner ON public.media USING btree (partner_id, status);


--
-- Name: ix_media_price_history_media_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_media_price_history_media_created ON public.media_price_history USING btree (media_id, created_at DESC);


--
-- Name: ix_media_seo_metrics_checked; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_media_seo_metrics_checked ON public.media_seo_metrics USING btree (checked_at DESC);


--
-- Name: ix_media_similarweb_media; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_media_similarweb_media ON public.media_similarweb_snapshots USING btree (media_id, created_at DESC);


--
-- Name: ix_media_stale_health; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_media_stale_health ON public.media USING btree (health_computed_at, id) WHERE ((status)::text = ANY (ARRAY[('active'::character varying)::text, ('draft'::character varying)::text]));


--
-- Name: ix_media_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_media_status ON public.media USING btree (status, name);


--
-- Name: ix_model_usage_brand_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_model_usage_brand_created ON public.model_usage_events USING btree (brand_id, created_at);


--
-- Name: ix_model_usage_provider_model; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_model_usage_provider_model ON public.model_usage_events USING btree (provider, model);


--
-- Name: ix_model_usage_user_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_model_usage_user_date ON public.model_usage_events USING btree (user_id, created_at);


--
-- Name: ix_notification_webhooks_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_notification_webhooks_active ON public.notification_webhooks USING btree (active);


--
-- Name: ix_notifications_publication; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_notifications_publication ON public.notifications USING btree (publication_id);


--
-- Name: ix_notifications_user_read; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_notifications_user_read ON public.notifications USING btree (user_id, read_at, created_at DESC);


--
-- Name: ix_notifications_user_read_expired; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_notifications_user_read_expired ON public.notifications USING btree (user_id, expired_at, read_at, created_at DESC);


--
-- Name: ix_package_brand_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_package_brand_status ON public.packages USING btree (brand_id, status);


--
-- Name: ix_partner_documents_partner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_partner_documents_partner ON public.partner_documents USING btree (partner_id, uploaded_at DESC);


--
-- Name: ix_partner_earnings_kind; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_partner_earnings_kind ON public.partner_earnings USING btree (kind);


--
-- Name: ix_partner_earnings_partner_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_partner_earnings_partner_status ON public.partner_earnings USING btree (partner_id, status, created_at);


--
-- Name: ix_partner_seo_history_partner_domain; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_partner_seo_history_partner_domain ON public.partner_seo_check_history USING btree (partner_id, domain, checked_at DESC);


--
-- Name: ix_partner_seo_history_partner_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_partner_seo_history_partner_time ON public.partner_seo_check_history USING btree (partner_id, checked_at DESC);


--
-- Name: ix_partner_service_charges_partner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_partner_service_charges_partner ON public.partner_service_charges USING btree (partner_id, created_at DESC);


--
-- Name: ix_partner_service_charges_service; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_partner_service_charges_service ON public.partner_service_charges USING btree (service, created_at DESC);


--
-- Name: ix_partners_is_vip; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_partners_is_vip ON public.partners USING btree (is_vip) WHERE (is_vip = true);


--
-- Name: ix_partners_vip_expires; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_partners_vip_expires ON public.partners USING btree (vip_expires_at) WHERE (is_vip = true);


--
-- Name: ix_password_reset_tokens_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_password_reset_tokens_user ON public.password_reset_tokens USING btree (user_id, expires_at DESC);


--
-- Name: ix_payment_transactions_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_payment_transactions_user ON public.payment_transactions USING btree (user_id, created_at DESC);


--
-- Name: ix_plan_occurrences_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_plan_occurrences_due ON public.publication_plan_occurrences USING btree (status, scheduled_at);


--
-- Name: ix_publication_attempt_publication; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publication_attempt_publication ON public.publication_attempts USING btree (publication_id, attempt_number);


--
-- Name: ix_publication_brand_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publication_brand_created ON public.publications USING btree (brand_id, created_at);


--
-- Name: ix_publication_brand_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publication_brand_status ON public.publications USING btree (brand_id, status);


--
-- Name: ix_publication_brand_status_published_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publication_brand_status_published_at ON public.publications USING btree (brand_id, status, published_at DESC);


--
-- Name: ix_publication_campaign; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publication_campaign ON public.publications USING btree (campaign_id);


--
-- Name: ix_publication_jobs_brand_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publication_jobs_brand_created ON public.publication_jobs USING btree (brand_id, created_at);


--
-- Name: ix_publication_jobs_publication; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publication_jobs_publication ON public.publication_jobs USING btree (publication_id);


--
-- Name: ix_publication_link_checks_state_checked; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publication_link_checks_state_checked ON public.publication_link_checks USING btree (state, last_checked_at);


--
-- Name: ix_publication_media_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publication_media_status ON public.publications USING btree (media_id, status);


--
-- Name: ix_publication_plans_brand; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publication_plans_brand ON public.publication_plans USING btree (brand_id, status);


--
-- Name: ix_publication_plans_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publication_plans_due ON public.publication_plans USING btree (status, start_at);


--
-- Name: ix_publication_status_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publication_status_created ON public.publications USING btree (status, created_at);


--
-- Name: ix_publications_replaces; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publications_replaces ON public.publications USING btree (replaces_id) WHERE (replaces_id IS NOT NULL);


--
-- Name: ix_publications_scheduled_due; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publications_scheduled_due ON public.publications USING btree (scheduled_at) WHERE (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('waiting_partner'::character varying)::text, ('partner_approved'::character varying)::text, ('retrying'::character varying)::text])) AND (scheduled_at IS NOT NULL));


--
-- Name: ix_publishing_api_keys_partner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_publishing_api_keys_partner ON public.publishing_api_keys USING btree (partner_id);


--
-- Name: ix_quality_brand_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_quality_brand_created ON public.quality_evaluations USING btree (brand_id, created_at);


--
-- Name: ix_quality_document_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_quality_document_created ON public.quality_evaluations USING btree (document_id, created_at);


--
-- Name: ix_refcomm_earning; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_refcomm_earning ON public.partner_referral_commissions USING btree (earning_id);


--
-- Name: ix_refcomm_referee; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_refcomm_referee ON public.partner_referral_commissions USING btree (referee_partner_id);


--
-- Name: ix_refcomm_referrer_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_refcomm_referrer_status ON public.partner_referral_commissions USING btree (referrer_partner_id, status);


--
-- Name: ix_referral_attributions_click; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_referral_attributions_click ON public.referral_attributions USING btree (click_id);


--
-- Name: ix_referral_attributions_code_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_referral_attributions_code_time ON public.referral_attributions USING btree (code, created_at DESC);


--
-- Name: ix_referral_clicks_code; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_referral_clicks_code ON public.referral_clicks USING btree (code, last_clicked_at DESC);


--
-- Name: ix_referral_clicks_referrer; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_referral_clicks_referrer ON public.referral_clicks USING btree (referrer_partner_id, last_clicked_at DESC);


--
-- Name: ix_referred_domains_media; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_referred_domains_media ON public.referred_domains USING btree (media_id) WHERE (media_id IS NOT NULL);


--
-- Name: ix_referred_domains_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_referred_domains_owner ON public.referred_domains USING btree (owner_user_id);


--
-- Name: ix_referred_domains_referrer; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_referred_domains_referrer ON public.referred_domains USING btree (referrer_partner_id, status);


--
-- Name: ix_registration_history_device_hash_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_registration_history_device_hash_time ON public.registration_history USING btree (device_hash, registered_at DESC);


--
-- Name: ix_registration_history_email_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_registration_history_email_hash ON public.registration_history USING btree (email_hash);


--
-- Name: ix_registration_history_google_sub_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_registration_history_google_sub_hash ON public.registration_history USING btree (google_sub_hash);


--
-- Name: ix_registration_history_ip_hash_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_registration_history_ip_hash_time ON public.registration_history USING btree (ip_hash, registered_at DESC);


--
-- Name: ix_revenue_share_lookup; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_revenue_share_lookup ON public.revenue_shares USING btree (status, media_id, partner_id);


--
-- Name: ix_review_comment_document_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_review_comment_document_created ON public.review_comments USING btree (document_id, created_at);


--
-- Name: ix_rss_access_logs_feed_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_rss_access_logs_feed_time ON public.rss_access_logs USING btree (feed_id, created_at);


--
-- Name: ix_rss_feed_media; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_rss_feed_media ON public.rss_feeds USING btree (media_id);


--
-- Name: ix_rss_feed_partner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_rss_feed_partner ON public.rss_feeds USING btree (partner_id, status);


--
-- Name: ix_settlement_partner_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_settlement_partner_status ON public.settlements USING btree (partner_id, status);


--
-- Name: ix_taut_angles_channel_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_taut_angles_channel_active ON public.taut_content_angles USING btree (channel_id, active, sort_order);


--
-- Name: ix_taut_campaign_files_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_taut_campaign_files_active ON public.taut_campaign_files USING btree (campaign_id) WHERE (deleted_at IS NULL);


--
-- Name: ix_taut_campaign_files_campaign; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_taut_campaign_files_campaign ON public.taut_campaign_files USING btree (campaign_id, created_at DESC);


--
-- Name: ix_taut_campaign_files_checksum; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_taut_campaign_files_checksum ON public.taut_campaign_files USING btree (checksum) WHERE (checksum IS NOT NULL);


--
-- Name: ix_taut_campaign_media_campaign; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_taut_campaign_media_campaign ON public.taut_campaign_media USING btree (campaign_id);


--
-- Name: ix_taut_channels_active_order; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_taut_channels_active_order ON public.taut_media_channels USING btree (active, sort_order);


--
-- Name: ix_taut_profiler_results_submission; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_taut_profiler_results_submission ON public.taut_profiler_results USING btree (submission_id, created_at DESC);


--
-- Name: ix_taut_runs_agent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_taut_runs_agent ON public.taut_agent_runs USING btree (agent, created_at DESC);


--
-- Name: ix_taut_runs_campaign; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_taut_runs_campaign ON public.taut_agent_runs USING btree (campaign_id, created_at DESC);


--
-- Name: ix_taut_runs_input_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_taut_runs_input_hash ON public.taut_agent_runs USING btree (agent, input_hash, status);


--
-- Name: ix_taut_submissions_campaign; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_taut_submissions_campaign ON public.taut_profiler_submissions USING btree (campaign_id, created_at DESC);


--
-- Name: ix_tracking_events_brand_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tracking_events_brand_time ON public.tracking_events USING btree (brand_id, created_at DESC);


--
-- Name: ix_tracking_events_campaign_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tracking_events_campaign_time ON public.tracking_events USING btree (campaign_id, created_at DESC);


--
-- Name: ix_tracking_events_link_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tracking_events_link_time ON public.tracking_events USING btree (link_id, created_at DESC);


--
-- Name: ix_tracking_events_type_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tracking_events_type_time ON public.tracking_events USING btree (event_type, created_at DESC);


--
-- Name: ix_tracking_link_brand; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tracking_link_brand ON public.tracking_links USING btree (brand_id, created_at DESC);


--
-- Name: ix_transaction_brand_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_transaction_brand_time ON public.transactions USING btree (brand_id, created_at DESC);


--
-- Name: ix_transaction_partner_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_transaction_partner_time ON public.transactions USING btree (partner_id, created_at DESC);


--
-- Name: ix_user_sessions_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_sessions_user ON public.user_sessions USING btree (user_id);


--
-- Name: ix_users_referred_by_partner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_users_referred_by_partner ON public.users USING btree (referred_by_partner_id);


--
-- Name: ix_variant_batch; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_variant_batch ON public.publication_variants USING btree (batch_id, created_at);


--
-- Name: ix_variant_batch_brand_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_variant_batch_brand_time ON public.publication_variant_batches USING btree (brand_id, created_at DESC);


--
-- Name: ix_variant_publication; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_variant_publication ON public.publication_variants USING btree (publication_id);


--
-- Name: ix_wa_deliveries_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_wa_deliveries_status ON public.wa_deliveries USING btree (status, created_at);


--
-- Name: ix_wa_deliveries_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_wa_deliveries_user ON public.wa_deliveries USING btree (user_id, created_at DESC);


--
-- Name: uq_analytics_daily; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_analytics_daily ON public.analytics_daily USING btree (brand_id, event_date, event_type, COALESCE(publication_id, ''::bpchar));


--
-- Name: uq_auth_rate_limits_bucket; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_auth_rate_limits_bucket ON public.auth_rate_limits USING btree (bucket);


--
-- Name: uq_auto_replace_brand; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_auto_replace_brand ON public.auto_replace_configs USING btree (brand_id) WHERE (brand_id IS NOT NULL);


--
-- Name: uq_auto_replace_global; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_auto_replace_global ON public.auto_replace_configs USING btree ((true)) WHERE (brand_id IS NULL);


--
-- Name: uq_campaign_order_code; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_campaign_order_code ON public.campaigns USING btree (order_code) WHERE (order_code IS NOT NULL);


--
-- Name: uq_cms_connection_partner_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_cms_connection_partner_name ON public.cms_connections USING btree (partner_id, name) WHERE (partner_id IS NOT NULL);


--
-- Name: uq_content_type_std_ver_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_content_type_std_ver_active ON public.content_type_standard_versions USING btree (standard_id) WHERE ((status)::text = ('active'::character varying)::text);


--
-- Name: uq_crawler_credential_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_crawler_credential_active ON public.crawler_credentials USING btree (media_id) WHERE (status = ANY (ARRAY['active'::text, 'rotating'::text]));


--
-- Name: uq_credit_ledger_idempotency; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_credit_ledger_idempotency ON public.credit_ledger_entries USING btree (idempotency_key);


--
-- Name: uq_gsc_connections_partner_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_gsc_connections_partner_email ON public.gsc_connections USING btree (partner_id, COALESCE(email, ((id)::text)::character varying));


--
-- Name: uq_knowledge_base_versions_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_knowledge_base_versions_active ON public.knowledge_base_versions USING btree (knowledge_base_id) WHERE ((status)::text = ('active'::character varying)::text);


--
-- Name: uq_knowledge_source_url; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_knowledge_source_url ON public.knowledge_sources USING btree (collection_id, source_type, original_filename) WHERE (((source_type)::text = 'url'::text) AND (original_filename IS NOT NULL) AND (deleted_at IS NULL));


--
-- Name: uq_media_packages_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_media_packages_slug ON public.media_packages USING btree (slug);


--
-- Name: uq_notification_webhook_user_url; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_notification_webhook_user_url ON public.notification_webhooks USING btree (user_id, url);


--
-- Name: uq_oauth_provider_account; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_oauth_provider_account ON public.user_oauth_accounts USING btree (provider, provider_account_id);


--
-- Name: uq_partners_referral_code_norm; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_partners_referral_code_norm ON public.partners USING btree (referral_code_normalized) WHERE (referral_code_normalized IS NOT NULL);


--
-- Name: uq_payment_gateway_single_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_payment_gateway_single_active ON public.payment_gateway_configs USING btree ((true)) WHERE (is_active = true);


--
-- Name: uq_payment_transaction_external; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_payment_transaction_external ON public.payment_transactions USING btree (gateway_provider, external_transaction_id) WHERE (external_transaction_id IS NOT NULL);


--
-- Name: uq_payment_transaction_reference; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_payment_transaction_reference ON public.payment_transactions USING btree (reference) WHERE (reference IS NOT NULL);


--
-- Name: uq_payment_transactions_brand_idem; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_payment_transactions_brand_idem ON public.payment_transactions USING btree (brand_id, idempotency_key) WHERE (idempotency_key IS NOT NULL);


--
-- Name: uq_publication_order_code; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_publication_order_code ON public.publications USING btree (order_code) WHERE (order_code IS NOT NULL);


--
-- Name: uq_refcomm_earning_referrer; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_refcomm_earning_referrer ON public.partner_referral_commissions USING btree (earning_id, referrer_partner_id);


--
-- Name: uq_settlement_invoice_number; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_settlement_invoice_number ON public.settlements USING btree (invoice_number);


--
-- Name: uq_taut_angle_versions_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_taut_angle_versions_active ON public.taut_content_angle_prompt_versions USING btree (content_angle_id) WHERE ((status)::text = 'active'::text);


--
-- Name: uq_taut_angle_versions_version; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_taut_angle_versions_version ON public.taut_content_angle_prompt_versions USING btree (content_angle_id, version);


--
-- Name: uq_taut_angles_channel_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_taut_angles_channel_slug ON public.taut_content_angles USING btree (channel_id, slug);


--
-- Name: uq_taut_base_prompt_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_taut_base_prompt_active ON public.taut_profiler_base_prompt_versions USING btree (version) WHERE ((status)::text = 'active'::text);


--
-- Name: uq_taut_base_prompt_version; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_taut_base_prompt_version ON public.taut_profiler_base_prompt_versions USING btree (version);


--
-- Name: uq_taut_campaign_media; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_taut_campaign_media ON public.taut_campaign_media USING btree (campaign_id, media_id);


--
-- Name: uq_taut_channels_slug; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_taut_channels_slug ON public.taut_media_channels USING btree (slug);


--
-- Name: uq_taut_submission_channels; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_taut_submission_channels ON public.taut_profiler_submission_channels USING btree (submission_id, channel_id);


--
-- Name: uq_taut_submissions_campaign_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_taut_submissions_campaign_active ON public.taut_profiler_submissions USING btree (campaign_id) WHERE ((status)::text = 'draft'::text);


--
-- Name: uq_wa_deliveries_dedupe; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_wa_deliveries_dedupe ON public.wa_deliveries USING btree (dedupe_key) WHERE (dedupe_key IS NOT NULL);


--
-- Name: media_package_items trg_client_package_media_limit; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_client_package_media_limit BEFORE INSERT ON public.media_package_items FOR EACH ROW EXECUTE FUNCTION public.enforce_client_package_media_limit();


--
-- Name: affiliate_settings affiliate_settings_partner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.affiliate_settings
    ADD CONSTRAINT affiliate_settings_partner_id_fkey FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: affiliate_settings affiliate_settings_updated_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.affiliate_settings
    ADD CONSTRAINT affiliate_settings_updated_by_fkey FOREIGN KEY (updated_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: ai_model_knowledge_chunks ai_model_knowledge_chunks_knowledge_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_model_knowledge_chunks
    ADD CONSTRAINT ai_model_knowledge_chunks_knowledge_id_fkey FOREIGN KEY (knowledge_id) REFERENCES public.ai_model_knowledge(id) ON DELETE CASCADE;


--
-- Name: brand_pr_standards brand_pr_standards_brand_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brand_pr_standards
    ADD CONSTRAINT brand_pr_standards_brand_id_fkey FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE;


--
-- Name: brand_pr_standards brand_pr_standards_updated_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brand_pr_standards
    ADD CONSTRAINT brand_pr_standards_updated_by_fkey FOREIGN KEY (updated_by) REFERENCES public.users(id);


--
-- Name: content_ai_analyses content_ai_analyses_campaign_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_ai_analyses
    ADD CONSTRAINT content_ai_analyses_campaign_id_fkey FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;


--
-- Name: content_ai_analyses content_ai_analyses_content_intake_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_ai_analyses
    ADD CONSTRAINT content_ai_analyses_content_intake_id_fkey FOREIGN KEY (content_intake_id) REFERENCES public.content_intakes(id) ON DELETE CASCADE;


--
-- Name: content_ai_analyses content_ai_analyses_media_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_ai_analyses
    ADD CONSTRAINT content_ai_analyses_media_id_fkey FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE SET NULL;


--
-- Name: crawler_credentials crawler_credentials_media_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.crawler_credentials
    ADD CONSTRAINT crawler_credentials_media_id_fkey FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: crawler_credentials crawler_credentials_partner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.crawler_credentials
    ADD CONSTRAINT crawler_credentials_partner_id_fkey FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: crawler_diagnostics crawler_diagnostics_credential_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.crawler_diagnostics
    ADD CONSTRAINT crawler_diagnostics_credential_id_fkey FOREIGN KEY (credential_id) REFERENCES public.crawler_credentials(id) ON DELETE SET NULL;


--
-- Name: crawler_diagnostics crawler_diagnostics_media_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.crawler_diagnostics
    ADD CONSTRAINT crawler_diagnostics_media_id_fkey FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: analytics_daily fk_analytics_daily_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analytics_daily
    ADD CONSTRAINT fk_analytics_daily_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: analytics_daily fk_analytics_daily_campaign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analytics_daily
    ADD CONSTRAINT fk_analytics_daily_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;


--
-- Name: analytics_daily fk_analytics_daily_publication; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.analytics_daily
    ADD CONSTRAINT fk_analytics_daily_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;


--
-- Name: auto_replace_configs fk_auto_replace_configs_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auto_replace_configs
    ADD CONSTRAINT fk_auto_replace_configs_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE;


--
-- Name: auto_replace_configs fk_auto_replace_configs_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auto_replace_configs
    ADD CONSTRAINT fk_auto_replace_configs_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: auto_replace_configs fk_auto_replace_configs_updater; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.auto_replace_configs
    ADD CONSTRAINT fk_auto_replace_configs_updater FOREIGN KEY (updated_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: blogger_oauth_states fk_blogger_oauth_states_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.blogger_oauth_states
    ADD CONSTRAINT fk_blogger_oauth_states_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: blogger_oauth_states fk_blogger_oauth_states_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.blogger_oauth_states
    ADD CONSTRAINT fk_blogger_oauth_states_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: brand_ai_quotas fk_brand_ai_quotas_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brand_ai_quotas
    ADD CONSTRAINT fk_brand_ai_quotas_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: brand_memberships fk_brand_memberships_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brand_memberships
    ADD CONSTRAINT fk_brand_memberships_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: brand_memberships fk_brand_memberships_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brand_memberships
    ADD CONSTRAINT fk_brand_memberships_user FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: brand_profiles fk_brand_profiles_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brand_profiles
    ADD CONSTRAINT fk_brand_profiles_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: brands fk_brands_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.brands
    ADD CONSTRAINT fk_brands_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: campaign_media_items fk_campaign_media_items_campaign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaign_media_items
    ADD CONSTRAINT fk_campaign_media_items_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE;


--
-- Name: campaign_media_items fk_campaign_media_items_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaign_media_items
    ADD CONSTRAINT fk_campaign_media_items_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE RESTRICT;


--
-- Name: campaign_media_items fk_campaign_media_items_source_document; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaign_media_items
    ADD CONSTRAINT fk_campaign_media_items_source_document FOREIGN KEY (source_document_id) REFERENCES public.content_documents(id) ON DELETE SET NULL;


--
-- Name: campaign_media_items fk_campaign_media_items_variant_document; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaign_media_items
    ADD CONSTRAINT fk_campaign_media_items_variant_document FOREIGN KEY (variant_document_id) REFERENCES public.content_documents(id) ON DELETE SET NULL;


--
-- Name: campaign_report_shares fk_campaign_report_share_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaign_report_shares
    ADD CONSTRAINT fk_campaign_report_share_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE;


--
-- Name: campaign_report_shares fk_campaign_report_share_campaign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaign_report_shares
    ADD CONSTRAINT fk_campaign_report_share_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE;


--
-- Name: campaign_report_shares fk_campaign_report_share_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaign_report_shares
    ADD CONSTRAINT fk_campaign_report_share_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: campaigns fk_campaigns_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaigns
    ADD CONSTRAINT fk_campaigns_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: campaigns fk_campaigns_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaigns
    ADD CONSTRAINT fk_campaigns_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: campaigns fk_campaigns_selected_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.campaigns
    ADD CONSTRAINT fk_campaigns_selected_media FOREIGN KEY (selected_media_id) REFERENCES public.media(id) ON DELETE SET NULL;


--
-- Name: cms_connections fk_cms_connections_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cms_connections
    ADD CONSTRAINT fk_cms_connections_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id);


--
-- Name: content_briefs fk_content_briefs_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_briefs
    ADD CONSTRAINT fk_content_briefs_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: content_documents fk_content_documents_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_documents
    ADD CONSTRAINT fk_content_documents_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: content_intakes fk_content_intakes_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_intakes
    ADD CONSTRAINT fk_content_intakes_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: content_intakes fk_content_intakes_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_intakes
    ADD CONSTRAINT fk_content_intakes_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: content_type_standard_chunks fk_content_type_std_chunks_version; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_type_standard_chunks
    ADD CONSTRAINT fk_content_type_std_chunks_version FOREIGN KEY (version_id) REFERENCES public.content_type_standard_versions(id) ON DELETE CASCADE;


--
-- Name: content_type_standard_versions fk_content_type_std_versions_standard; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.content_type_standard_versions
    ADD CONSTRAINT fk_content_type_std_versions_standard FOREIGN KEY (standard_id) REFERENCES public.content_type_standards(id) ON DELETE CASCADE;


--
-- Name: knowledge_crawl_runs fk_crawl_runs_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_crawl_runs
    ADD CONSTRAINT fk_crawl_runs_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: knowledge_crawl_runs fk_crawl_runs_collection; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_crawl_runs
    ADD CONSTRAINT fk_crawl_runs_collection FOREIGN KEY (collection_id) REFERENCES public.knowledge_collections(id);


--
-- Name: crawl_targets fk_crawl_targets_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.crawl_targets
    ADD CONSTRAINT fk_crawl_targets_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: credit_ledger_entries fk_credit_ledger_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credit_ledger_entries
    ADD CONSTRAINT fk_credit_ledger_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE;


--
-- Name: credit_transactions fk_credit_tx_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credit_transactions
    ADD CONSTRAINT fk_credit_tx_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: credit_transactions fk_credit_tx_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credit_transactions
    ADD CONSTRAINT fk_credit_tx_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: credit_transactions fk_credit_tx_package; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credit_transactions
    ADD CONSTRAINT fk_credit_tx_package FOREIGN KEY (package_id) REFERENCES public.packages(id) ON DELETE SET NULL;


--
-- Name: credit_transactions fk_credit_tx_publication; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credit_transactions
    ADD CONSTRAINT fk_credit_tx_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE SET NULL;


--
-- Name: credit_wallets fk_credit_wallets_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.credit_wallets
    ADD CONSTRAINT fk_credit_wallets_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE;


--
-- Name: editorial_rule_sets fk_editorial_rule_sets_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.editorial_rule_sets
    ADD CONSTRAINT fk_editorial_rule_sets_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: email_deliveries fk_email_deliveries_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.email_deliveries
    ADD CONSTRAINT fk_email_deliveries_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: ga4_configs fk_ga4_configs_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ga4_configs
    ADD CONSTRAINT fk_ga4_configs_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: ga4_configs fk_ga4_configs_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ga4_configs
    ADD CONSTRAINT fk_ga4_configs_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: gsc_connections fk_gsc_connections_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gsc_connections
    ADD CONSTRAINT fk_gsc_connections_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: gsc_connections fk_gsc_connections_selected_property; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gsc_connections
    ADD CONSTRAINT fk_gsc_connections_selected_property FOREIGN KEY (selected_property_id) REFERENCES public.gsc_properties(id) ON DELETE SET NULL;


--
-- Name: gsc_oauth_states fk_gsc_oauth_states_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gsc_oauth_states
    ADD CONSTRAINT fk_gsc_oauth_states_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: gsc_properties fk_gsc_properties_connection; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gsc_properties
    ADD CONSTRAINT fk_gsc_properties_connection FOREIGN KEY (connection_id) REFERENCES public.gsc_connections(id) ON DELETE CASCADE;


--
-- Name: gsc_snapshots fk_gsc_snapshots_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gsc_snapshots
    ADD CONSTRAINT fk_gsc_snapshots_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE SET NULL;


--
-- Name: gsc_snapshots fk_gsc_snapshots_property; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gsc_snapshots
    ADD CONSTRAINT fk_gsc_snapshots_property FOREIGN KEY (property_id) REFERENCES public.gsc_properties(id) ON DELETE CASCADE;


--
-- Name: invoices fk_invoices_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT fk_invoices_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: invoices fk_invoices_campaign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT fk_invoices_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;


--
-- Name: invoices fk_invoices_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.invoices
    ADD CONSTRAINT fk_invoices_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: knowledge_base_rule_chunks fk_kb_rule_chunks_rule; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_base_rule_chunks
    ADD CONSTRAINT fk_kb_rule_chunks_rule FOREIGN KEY (rule_id) REFERENCES public.knowledge_base_rules(id) ON DELETE CASCADE;


--
-- Name: knowledge_base_rules fk_kb_rules_base; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_base_rules
    ADD CONSTRAINT fk_kb_rules_base FOREIGN KEY (knowledge_base_id) REFERENCES public.knowledge_bases(id) ON DELETE CASCADE;


--
-- Name: knowledge_base_rules fk_kb_rules_version; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_base_rules
    ADD CONSTRAINT fk_kb_rules_version FOREIGN KEY (version_id) REFERENCES public.knowledge_base_versions(id) ON DELETE CASCADE;


--
-- Name: knowledge_base_sources fk_kb_sources_base; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_base_sources
    ADD CONSTRAINT fk_kb_sources_base FOREIGN KEY (knowledge_base_id) REFERENCES public.knowledge_bases(id) ON DELETE CASCADE;


--
-- Name: knowledge_base_versions fk_kb_versions_base; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_base_versions
    ADD CONSTRAINT fk_kb_versions_base FOREIGN KEY (knowledge_base_id) REFERENCES public.knowledge_bases(id) ON DELETE CASCADE;


--
-- Name: knowledge_chunks fk_knowledge_chunks_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_chunks
    ADD CONSTRAINT fk_knowledge_chunks_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: knowledge_chunks fk_knowledge_chunks_collection; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_chunks
    ADD CONSTRAINT fk_knowledge_chunks_collection FOREIGN KEY (collection_id) REFERENCES public.knowledge_collections(id);


--
-- Name: knowledge_chunks fk_knowledge_chunks_source; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_chunks
    ADD CONSTRAINT fk_knowledge_chunks_source FOREIGN KEY (source_id) REFERENCES public.knowledge_sources(id) ON DELETE CASCADE;


--
-- Name: knowledge_collections fk_knowledge_collections_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_collections
    ADD CONSTRAINT fk_knowledge_collections_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE;


--
-- Name: knowledge_sources fk_knowledge_sources_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_sources
    ADD CONSTRAINT fk_knowledge_sources_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: knowledge_sources fk_knowledge_sources_collection; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_sources
    ADD CONSTRAINT fk_knowledge_sources_collection FOREIGN KEY (collection_id) REFERENCES public.knowledge_collections(id);


--
-- Name: knowledge_sources fk_knowledge_sources_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.knowledge_sources
    ADD CONSTRAINT fk_knowledge_sources_creator FOREIGN KEY (created_by) REFERENCES public.users(id);


--
-- Name: master_prompt_versions fk_master_prompt_versions_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.master_prompt_versions
    ADD CONSTRAINT fk_master_prompt_versions_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: master_prompt_versions fk_master_prompt_versions_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.master_prompt_versions
    ADD CONSTRAINT fk_master_prompt_versions_creator FOREIGN KEY (created_by) REFERENCES public.users(id);


--
-- Name: media_category_links fk_mcl_category; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_category_links
    ADD CONSTRAINT fk_mcl_category FOREIGN KEY (category_id) REFERENCES public.media_categories(id) ON DELETE CASCADE;


--
-- Name: media_category_links fk_mcl_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_category_links
    ADD CONSTRAINT fk_mcl_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: media_dewan_pers_verifications fk_mdpv_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_dewan_pers_verifications
    ADD CONSTRAINT fk_mdpv_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: media_dewan_pers_verifications fk_mdpv_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_dewan_pers_verifications
    ADD CONSTRAINT fk_mdpv_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: media_content_standards fk_media_content_standards_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_content_standards
    ADD CONSTRAINT fk_media_content_standards_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: media_content_standards fk_media_content_standards_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_content_standards
    ADD CONSTRAINT fk_media_content_standards_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: media fk_media_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media
    ADD CONSTRAINT fk_media_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: media fk_media_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media
    ADD CONSTRAINT fk_media_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE SET NULL;


--
-- Name: media_price_history fk_media_price_history_actor; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_price_history
    ADD CONSTRAINT fk_media_price_history_actor FOREIGN KEY (actor_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: media_price_history fk_media_price_history_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_price_history
    ADD CONSTRAINT fk_media_price_history_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: media_seo_metrics fk_media_seo_metrics_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_seo_metrics
    ADD CONSTRAINT fk_media_seo_metrics_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: media_similarweb_snapshots fk_media_similarweb_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_similarweb_snapshots
    ADD CONSTRAINT fk_media_similarweb_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: notification_webhooks fk_notification_webhooks_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_webhooks
    ADD CONSTRAINT fk_notification_webhooks_user FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: notifications fk_notifications_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT fk_notifications_user FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: packages fk_packages_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.packages
    ADD CONSTRAINT fk_packages_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: packages fk_packages_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.packages
    ADD CONSTRAINT fk_packages_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: partner_documents fk_partner_documents_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_documents
    ADD CONSTRAINT fk_partner_documents_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: partner_earnings fk_partner_earnings_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_earnings
    ADD CONSTRAINT fk_partner_earnings_media FOREIGN KEY (media_id) REFERENCES public.media(id);


--
-- Name: partner_earnings fk_partner_earnings_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_earnings
    ADD CONSTRAINT fk_partner_earnings_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: partner_earnings fk_partner_earnings_publication; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_earnings
    ADD CONSTRAINT fk_partner_earnings_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;


--
-- Name: partner_earnings fk_partner_earnings_rule; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_earnings
    ADD CONSTRAINT fk_partner_earnings_rule FOREIGN KEY (rule_id) REFERENCES public.revenue_shares(id) ON DELETE SET NULL;


--
-- Name: partner_earnings fk_partner_earnings_settlement; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_earnings
    ADD CONSTRAINT fk_partner_earnings_settlement FOREIGN KEY (settlement_id) REFERENCES public.settlements(id) ON DELETE SET NULL;


--
-- Name: partners fk_partners_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partners
    ADD CONSTRAINT fk_partners_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: platform_admins fk_platform_admins_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_admins
    ADD CONSTRAINT fk_platform_admins_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: platform_ai_configuration fk_platform_ai_config_updated_by; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_ai_configuration
    ADD CONSTRAINT fk_platform_ai_config_updated_by FOREIGN KEY (updated_by) REFERENCES public.users(id);


--
-- Name: pr_campaign_scores fk_pr_campaign_scores_calculator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_campaign_scores
    ADD CONSTRAINT fk_pr_campaign_scores_calculator FOREIGN KEY (calculated_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: pr_campaign_scores fk_pr_campaign_scores_campaign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_campaign_scores
    ADD CONSTRAINT fk_pr_campaign_scores_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE;


--
-- Name: pr_campaign_scores fk_pr_campaign_scores_profile; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_campaign_scores
    ADD CONSTRAINT fk_pr_campaign_scores_profile FOREIGN KEY (profile_id) REFERENCES public.pr_scoring_profiles(id);


--
-- Name: pr_media_rules fk_pr_media_rules_updater; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_media_rules
    ADD CONSTRAINT fk_pr_media_rules_updater FOREIGN KEY (updated_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: pr_message_analyses fk_pr_message_analyses_campaign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_message_analyses
    ADD CONSTRAINT fk_pr_message_analyses_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;


--
-- Name: pr_message_analyses fk_pr_message_analyses_publication; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_message_analyses
    ADD CONSTRAINT fk_pr_message_analyses_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;


--
-- Name: pr_metric_results fk_pr_metric_results_campaign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_metric_results
    ADD CONSTRAINT fk_pr_metric_results_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE;


--
-- Name: pr_scoring_profiles fk_pr_scoring_profiles_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_scoring_profiles
    ADD CONSTRAINT fk_pr_scoring_profiles_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: pr_sentiment_analyses fk_pr_sentiment_analyses_campaign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_sentiment_analyses
    ADD CONSTRAINT fk_pr_sentiment_analyses_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;


--
-- Name: pr_sentiment_analyses fk_pr_sentiment_analyses_publication; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_sentiment_analyses
    ADD CONSTRAINT fk_pr_sentiment_analyses_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;


--
-- Name: pr_publication_verifications fk_pr_verifications_campaign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_publication_verifications
    ADD CONSTRAINT fk_pr_verifications_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;


--
-- Name: pr_publication_verifications fk_pr_verifications_publication; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.pr_publication_verifications
    ADD CONSTRAINT fk_pr_verifications_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;


--
-- Name: publication_attempts fk_publication_attempts_publication; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_attempts
    ADD CONSTRAINT fk_publication_attempts_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;


--
-- Name: publication_jobs fk_publication_jobs_publication; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_jobs
    ADD CONSTRAINT fk_publication_jobs_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE SET NULL;


--
-- Name: publication_link_checks fk_publication_link_checks_publication; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_link_checks
    ADD CONSTRAINT fk_publication_link_checks_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;


--
-- Name: publications fk_publications_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: publications fk_publications_campaign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;


--
-- Name: publications fk_publications_connection; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_connection FOREIGN KEY (connection_id) REFERENCES public.cms_connections(id) ON DELETE SET NULL;


--
-- Name: publications fk_publications_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_creator FOREIGN KEY (created_by) REFERENCES public.users(id);


--
-- Name: publications fk_publications_document; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_document FOREIGN KEY (document_id) REFERENCES public.content_documents(id);


--
-- Name: publications fk_publications_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_media FOREIGN KEY (media_id) REFERENCES public.media(id);


--
-- Name: publications fk_publications_replaces; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_replaces FOREIGN KEY (replaces_id) REFERENCES public.publications(id) ON DELETE SET NULL;


--
-- Name: publications fk_publications_reviewed_by; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_reviewed_by FOREIGN KEY (reviewed_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: publishing_api_keys fk_publishing_api_keys_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publishing_api_keys
    ADD CONSTRAINT fk_publishing_api_keys_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: publishing_api_keys fk_publishing_api_keys_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publishing_api_keys
    ADD CONSTRAINT fk_publishing_api_keys_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: publishing_api_keys fk_publishing_api_keys_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publishing_api_keys
    ADD CONSTRAINT fk_publishing_api_keys_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: publishing_callback_logs fk_publishing_callback_logs_key; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publishing_callback_logs
    ADD CONSTRAINT fk_publishing_callback_logs_key FOREIGN KEY (api_key_id) REFERENCES public.publishing_api_keys(id) ON DELETE SET NULL;


--
-- Name: publishing_callback_logs fk_publishing_callback_logs_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publishing_callback_logs
    ADD CONSTRAINT fk_publishing_callback_logs_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE SET NULL;


--
-- Name: revenue_shares fk_revenue_shares_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.revenue_shares
    ADD CONSTRAINT fk_revenue_shares_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: revenue_shares fk_revenue_shares_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.revenue_shares
    ADD CONSTRAINT fk_revenue_shares_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: revenue_shares fk_revenue_shares_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.revenue_shares
    ADD CONSTRAINT fk_revenue_shares_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: rss_access_logs fk_rss_access_logs_feed; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rss_access_logs
    ADD CONSTRAINT fk_rss_access_logs_feed FOREIGN KEY (feed_id) REFERENCES public.rss_feeds(id) ON DELETE CASCADE;


--
-- Name: rss_feeds fk_rss_feeds_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rss_feeds
    ADD CONSTRAINT fk_rss_feeds_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: rss_feeds fk_rss_feeds_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rss_feeds
    ADD CONSTRAINT fk_rss_feeds_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE SET NULL;


--
-- Name: rss_feeds fk_rss_feeds_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.rss_feeds
    ADD CONSTRAINT fk_rss_feeds_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: settlements fk_settlements_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlements
    ADD CONSTRAINT fk_settlements_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: settlements fk_settlements_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settlements
    ADD CONSTRAINT fk_settlements_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: smtp_settings fk_smtp_settings_updated_by; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smtp_settings
    ADD CONSTRAINT fk_smtp_settings_updated_by FOREIGN KEY (updated_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: taut_campaign_media fk_taut_cm_document; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_campaign_media
    ADD CONSTRAINT fk_taut_cm_document FOREIGN KEY (article_document_id) REFERENCES public.content_documents(id) ON DELETE SET NULL;


--
-- Name: taut_gatekeeper_audits fk_taut_gatekeeper_audits_campaign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_gatekeeper_audits
    ADD CONSTRAINT fk_taut_gatekeeper_audits_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE;


--
-- Name: taut_gatekeeper_audits fk_taut_gatekeeper_audits_media; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_gatekeeper_audits
    ADD CONSTRAINT fk_taut_gatekeeper_audits_media FOREIGN KEY (media_id) REFERENCES public.media(id);


--
-- Name: taut_gatekeeper_audits fk_taut_gatekeeper_audits_run; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_gatekeeper_audits
    ADD CONSTRAINT fk_taut_gatekeeper_audits_run FOREIGN KEY (agent_run_id) REFERENCES public.taut_agent_runs(id) ON DELETE SET NULL;


--
-- Name: tracking_events fk_tracking_events_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tracking_events
    ADD CONSTRAINT fk_tracking_events_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: tracking_events fk_tracking_events_campaign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tracking_events
    ADD CONSTRAINT fk_tracking_events_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;


--
-- Name: tracking_events fk_tracking_events_link; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tracking_events
    ADD CONSTRAINT fk_tracking_events_link FOREIGN KEY (link_id) REFERENCES public.tracking_links(id) ON DELETE SET NULL;


--
-- Name: tracking_events fk_tracking_events_publication; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tracking_events
    ADD CONSTRAINT fk_tracking_events_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE SET NULL;


--
-- Name: tracking_links fk_tracking_links_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tracking_links
    ADD CONSTRAINT fk_tracking_links_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: tracking_links fk_tracking_links_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tracking_links
    ADD CONSTRAINT fk_tracking_links_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: tracking_links fk_tracking_links_publication; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tracking_links
    ADD CONSTRAINT fk_tracking_links_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;


--
-- Name: transactions fk_transactions_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT fk_transactions_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: transactions fk_transactions_campaign; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT fk_transactions_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;


--
-- Name: transactions fk_transactions_creator; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT fk_transactions_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: transactions fk_transactions_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT fk_transactions_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id);


--
-- Name: transactions fk_transactions_publication; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.transactions
    ADD CONSTRAINT fk_transactions_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE SET NULL;


--
-- Name: user_daily_quotas fk_user_daily_quota_updated_by; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_daily_quotas
    ADD CONSTRAINT fk_user_daily_quota_updated_by FOREIGN KEY (updated_by) REFERENCES public.users(id);


--
-- Name: user_daily_quotas fk_user_daily_quota_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_daily_quotas
    ADD CONSTRAINT fk_user_daily_quota_user FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: user_sessions fk_user_sessions_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_sessions
    ADD CONSTRAINT fk_user_sessions_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: users fk_users_referred_by_partner; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT fk_users_referred_by_partner FOREIGN KEY (referred_by_partner_id) REFERENCES public.partners(id) ON DELETE SET NULL;


--
-- Name: voice_profiles fk_voice_profiles_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.voice_profiles
    ADD CONSTRAINT fk_voice_profiles_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: websites fk_websites_brand; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.websites
    ADD CONSTRAINT fk_websites_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id);


--
-- Name: media_ai_summary_events media_ai_summary_events_media_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_ai_summary_events
    ADD CONSTRAINT media_ai_summary_events_media_id_fkey FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: media_package_items media_package_items_media_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_package_items
    ADD CONSTRAINT media_package_items_media_id_fkey FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: media_package_items media_package_items_package_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_package_items
    ADD CONSTRAINT media_package_items_package_id_fkey FOREIGN KEY (package_id) REFERENCES public.media_packages(id) ON DELETE CASCADE;


--
-- Name: media_packages media_packages_brand_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.media_packages
    ADD CONSTRAINT media_packages_brand_id_fkey FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE;


--
-- Name: notification_preferences notification_preferences_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.notification_preferences
    ADD CONSTRAINT notification_preferences_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: partner_referral_commissions partner_referral_commissions_earning_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_referral_commissions
    ADD CONSTRAINT partner_referral_commissions_earning_id_fkey FOREIGN KEY (earning_id) REFERENCES public.partner_earnings(id) ON DELETE CASCADE;


--
-- Name: partner_referral_commissions partner_referral_commissions_referee_partner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_referral_commissions
    ADD CONSTRAINT partner_referral_commissions_referee_partner_id_fkey FOREIGN KEY (referee_partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: partner_referral_commissions partner_referral_commissions_referee_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_referral_commissions
    ADD CONSTRAINT partner_referral_commissions_referee_user_id_fkey FOREIGN KEY (referee_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: partner_referral_commissions partner_referral_commissions_referrer_partner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_referral_commissions
    ADD CONSTRAINT partner_referral_commissions_referrer_partner_id_fkey FOREIGN KEY (referrer_partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: partner_referral_commissions partner_referral_commissions_reversal_of_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_referral_commissions
    ADD CONSTRAINT partner_referral_commissions_reversal_of_fkey FOREIGN KEY (reversal_of) REFERENCES public.partner_referral_commissions(id) ON DELETE SET NULL;


--
-- Name: partner_seo_check_history partner_seo_check_history_partner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_seo_check_history
    ADD CONSTRAINT partner_seo_check_history_partner_id_fkey FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: partner_service_charges partner_service_charges_partner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.partner_service_charges
    ADD CONSTRAINT partner_service_charges_partner_id_fkey FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: password_reset_tokens password_reset_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: platform_wa_config platform_wa_config_updated_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.platform_wa_config
    ADD CONSTRAINT platform_wa_config_updated_by_fkey FOREIGN KEY (updated_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: publication_plan_media publication_plan_media_plan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_plan_media
    ADD CONSTRAINT publication_plan_media_plan_id_fkey FOREIGN KEY (plan_id) REFERENCES public.publication_plans(id) ON DELETE CASCADE;


--
-- Name: publication_plan_occurrences publication_plan_occurrences_plan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.publication_plan_occurrences
    ADD CONSTRAINT publication_plan_occurrences_plan_id_fkey FOREIGN KEY (plan_id) REFERENCES public.publication_plans(id) ON DELETE CASCADE;


--
-- Name: referral_attributions referral_attributions_click_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.referral_attributions
    ADD CONSTRAINT referral_attributions_click_id_fkey FOREIGN KEY (click_id) REFERENCES public.referral_clicks(id) ON DELETE SET NULL;


--
-- Name: referral_attributions referral_attributions_referee_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.referral_attributions
    ADD CONSTRAINT referral_attributions_referee_user_id_fkey FOREIGN KEY (referee_user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: referral_attributions referral_attributions_referrer_partner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.referral_attributions
    ADD CONSTRAINT referral_attributions_referrer_partner_id_fkey FOREIGN KEY (referrer_partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: referral_clicks referral_clicks_referrer_partner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.referral_clicks
    ADD CONSTRAINT referral_clicks_referrer_partner_id_fkey FOREIGN KEY (referrer_partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: referred_domains referred_domains_media_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.referred_domains
    ADD CONSTRAINT referred_domains_media_id_fkey FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: referred_domains referred_domains_owner_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.referred_domains
    ADD CONSTRAINT referred_domains_owner_user_id_fkey FOREIGN KEY (owner_user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: referred_domains referred_domains_referrer_partner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.referred_domains
    ADD CONSTRAINT referred_domains_referrer_partner_id_fkey FOREIGN KEY (referrer_partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;


--
-- Name: registration_history registration_history_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.registration_history
    ADD CONSTRAINT registration_history_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: taut_agent_runs taut_agent_runs_campaign_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_agent_runs
    ADD CONSTRAINT taut_agent_runs_campaign_id_fkey FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE;


--
-- Name: taut_campaign_files taut_campaign_files_campaign_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_campaign_files
    ADD CONSTRAINT taut_campaign_files_campaign_id_fkey FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE;


--
-- Name: taut_campaign_files taut_campaign_files_submission_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_campaign_files
    ADD CONSTRAINT taut_campaign_files_submission_id_fkey FOREIGN KEY (submission_id) REFERENCES public.taut_profiler_submissions(id) ON DELETE SET NULL;


--
-- Name: taut_campaign_media taut_campaign_media_campaign_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_campaign_media
    ADD CONSTRAINT taut_campaign_media_campaign_id_fkey FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE;


--
-- Name: taut_campaign_media taut_campaign_media_media_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_campaign_media
    ADD CONSTRAINT taut_campaign_media_media_id_fkey FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;


--
-- Name: taut_content_angle_prompt_versions taut_content_angle_prompt_versions_content_angle_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_content_angle_prompt_versions
    ADD CONSTRAINT taut_content_angle_prompt_versions_content_angle_id_fkey FOREIGN KEY (content_angle_id) REFERENCES public.taut_content_angles(id) ON DELETE CASCADE;


--
-- Name: taut_content_angles taut_content_angles_channel_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_content_angles
    ADD CONSTRAINT taut_content_angles_channel_id_fkey FOREIGN KEY (channel_id) REFERENCES public.taut_media_channels(id) ON DELETE CASCADE;


--
-- Name: taut_profiler_results taut_profiler_results_submission_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_profiler_results
    ADD CONSTRAINT taut_profiler_results_submission_id_fkey FOREIGN KEY (submission_id) REFERENCES public.taut_profiler_submissions(id) ON DELETE CASCADE;


--
-- Name: taut_profiler_submission_channels taut_profiler_submission_channels_channel_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_profiler_submission_channels
    ADD CONSTRAINT taut_profiler_submission_channels_channel_id_fkey FOREIGN KEY (channel_id) REFERENCES public.taut_media_channels(id) ON DELETE CASCADE;


--
-- Name: taut_profiler_submission_channels taut_profiler_submission_channels_content_angle_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_profiler_submission_channels
    ADD CONSTRAINT taut_profiler_submission_channels_content_angle_id_fkey FOREIGN KEY (content_angle_id) REFERENCES public.taut_content_angles(id) ON DELETE SET NULL;


--
-- Name: taut_profiler_submission_channels taut_profiler_submission_channels_prompt_version_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_profiler_submission_channels
    ADD CONSTRAINT taut_profiler_submission_channels_prompt_version_id_fkey FOREIGN KEY (prompt_version_id) REFERENCES public.taut_content_angle_prompt_versions(id) ON DELETE SET NULL;


--
-- Name: taut_profiler_submission_channels taut_profiler_submission_channels_submission_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_profiler_submission_channels
    ADD CONSTRAINT taut_profiler_submission_channels_submission_id_fkey FOREIGN KEY (submission_id) REFERENCES public.taut_profiler_submissions(id) ON DELETE CASCADE;


--
-- Name: taut_profiler_submissions taut_profiler_submissions_campaign_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.taut_profiler_submissions
    ADD CONSTRAINT taut_profiler_submissions_campaign_id_fkey FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE;


--
-- Name: wa_deliveries wa_deliveries_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wa_deliveries
    ADD CONSTRAINT wa_deliveries_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: wa_templates wa_templates_updated_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.wa_templates
    ADD CONSTRAINT wa_templates_updated_by_fkey FOREIGN KEY (updated_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- PostgreSQL database dump complete
--


