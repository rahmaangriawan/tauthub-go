-- 139 — Tracking/monitoring Kesimpulan Website per media.
-- Satu baris event per kejadian: queued (masuk antrean), success (AI berhasil),
-- failed (gagal — dicatat untuk monitoring admin). pages_tried mencatat URL
-- yang ditarik beserta hasilnya (tracking "sudah tracking di web mana").

BEGIN;

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

COMMIT;
