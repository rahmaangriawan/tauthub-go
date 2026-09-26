-- 046_campaign_media_items.sql
-- Linear campaign flow: per-media article versions, thumbnails, validation.

CREATE TABLE public.campaign_media_items (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    source_document_id character(36),
    variant_document_id character(36),
    variant_type character varying(20) DEFAULT 'original' NOT NULL,
    angle_label character varying(200),
    thumbnail_url character varying(500),
    tc_cost integer DEFAULT 0 NOT NULL,
    status character varying(30) DEFAULT 'pending' NOT NULL,
    validation jsonb DEFAULT '{}' NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,

    CONSTRAINT campaign_media_items_pkey PRIMARY KEY (id),
    CONSTRAINT uq_campaign_media UNIQUE (campaign_id, media_id),
    CONSTRAINT ck_campaign_media_variant_type CHECK (variant_type IN ('original','paraphrase','angle')),
    CONSTRAINT ck_campaign_media_status CHECK (status IN ('pending','article_ready','needs_rewrite','rewriting','angle_select','ready','processing','published','failed','cancelled'))
);

ALTER TABLE public.campaign_media_items OWNER TO ai_writer;

CREATE INDEX ix_campaign_media_items_campaign ON public.campaign_media_items (campaign_id);
CREATE INDEX ix_campaign_media_items_media ON public.campaign_media_items (media_id);

ALTER TABLE public.campaign_media_items
    ADD CONSTRAINT fk_campaign_media_items_campaign
    FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE;

ALTER TABLE public.campaign_media_items
    ADD CONSTRAINT fk_campaign_media_items_media
    FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE RESTRICT;

ALTER TABLE public.campaign_media_items
    ADD CONSTRAINT fk_campaign_media_items_source_document
    FOREIGN KEY (source_document_id) REFERENCES public.content_documents(id) ON DELETE SET NULL;

ALTER TABLE public.campaign_media_items
    ADD CONSTRAINT fk_campaign_media_items_variant_document
    FOREIGN KEY (variant_document_id) REFERENCES public.content_documents(id) ON DELETE SET NULL;

-- Add content_intake_id to campaigns for linear flow linking.
ALTER TABLE public.campaigns ADD COLUMN content_intake_id character(36);
ALTER TABLE public.campaigns ADD COLUMN campaign_flow_status character varying(30) DEFAULT 'intake' NOT NULL;
ALTER TABLE public.campaigns ADD CONSTRAINT ck_campaign_flow_status CHECK (campaign_flow_status IN ('intake','content','media_selection','final_review','checkout','distributing','completed','cancelled'));

CREATE INDEX ix_campaigns_intake ON public.campaigns (content_intake_id);