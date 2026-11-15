-- 330 — Antrean penulisan Dapur Produksi (Taut Pintar): batch artikel, cover,
-- dan artikel turunan "Pakai artikel untuk media lain". Satu job per artikel; job yang
-- selesai mengantre job berikutnya, jadi antrean umum tidak tertahan oleh batch
-- besar. Kolom turunan di taut_campaign_media menyimpan asal artikel, skor
-- kemiripan, dan teks rujukan fakta untuk Gatekeeper. Tipe notifikasi baru
-- writer.batch_done. Additive dan idempoten.

BEGIN;

CREATE TABLE IF NOT EXISTS public.taut_writer_batches (
  id          character(36) PRIMARY KEY,
  campaign_id character(36) NOT NULL,
  brand_id    character(36) NOT NULL,
  user_id     character(36) NOT NULL,
  kind        character varying(16) NOT NULL,
  status      character varying(16) NOT NULL DEFAULT 'queued',
  total       integer NOT NULL,
  done        integer NOT NULL DEFAULT 0,
  failed      integer NOT NULL DEFAULT 0,
  options     jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ck_taut_writer_batches_kind CHECK (kind IN ('article', 'cover', 'derived')),
  CONSTRAINT ck_taut_writer_batches_status CHECK (status IN ('queued', 'running', 'done', 'failed'))
);
CREATE INDEX IF NOT EXISTS ix_taut_writer_batches_campaign ON public.taut_writer_batches (campaign_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_taut_writer_batches_brand_active ON public.taut_writer_batches (brand_id) WHERE status IN ('queued', 'running');

CREATE TABLE IF NOT EXISTS public.taut_writer_batch_items (
  id            character(36) PRIMARY KEY,
  batch_id      character(36) NOT NULL REFERENCES public.taut_writer_batches(id) ON DELETE CASCADE,
  media_id      character(36) NOT NULL,
  slot          integer NOT NULL DEFAULT 1,
  position      integer NOT NULL DEFAULT 0,
  status        character varying(16) NOT NULL DEFAULT 'queued',
  error_message text,
  similarity    numeric(4,3),
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ck_taut_writer_batch_items_status CHECK (status IN ('queued', 'running', 'done', 'failed'))
);
CREATE INDEX IF NOT EXISTS ix_taut_writer_batch_items_batch ON public.taut_writer_batch_items (batch_id, position);

ALTER TABLE public.taut_campaign_media ADD COLUMN IF NOT EXISTS derived_from_media_id character(36);
ALTER TABLE public.taut_campaign_media ADD COLUMN IF NOT EXISTS derived_from_slot integer;
ALTER TABLE public.taut_campaign_media ADD COLUMN IF NOT EXISTS derived_similarity numeric(4,3);
ALTER TABLE public.taut_campaign_media ADD COLUMN IF NOT EXISTS derived_source_text text;

ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type
  CHECK (((type)::text = ANY ((ARRAY[
    'announcement.new', 'billing.balance_forfeited', 'billing.balance_forfeit_warning',
    'billing.balance_low', 'billing.topup_needed', 'changelog.new', 'chat.message',
    'media.approved', 'media.crawl_blocked', 'media.dewan_pers.approved',
    'media.dewan_pers.rejected', 'media.dewan_pers.submitted', 'media.domain_down',
    'media.integration_missing',
    'media.negotiated', 'media.rejected', 'media.resubmitted', 'media.submitted',
    'partner.guarantee_reversed', 'partner.wa_required',
    'pr.score_updated', 'pr.verification_completed',
    'publication.auto_replaced', 'publication.cancelled_by_admin',
    'publication.client_approved', 'publication.failed',
    'publication.guarantee_update', 'publication.guarantee_triggered',
    'publication.late_approval', 'publication.link_lost',
    'publication.link_replacement_requested', 'publication.partner_approved',
    'publication.partner_rejected', 'publication.published', 'publication.reminder',
    'publication.revision_accepted', 'publication.revision_rejected',
    'publication.revision_requested', 'publication.sla_cancelled',
    'publication.sla_extended', 'publication.url_requested',
    'publication.waiting_partner', 'publication.waiting_long', 'writer.batch_done',
    'referral.commission.credited', 'referral.commission.reversed',
    'referral.cpa.credited', 'referral.domain.approved', 'referral.domain.rejected',
    'report.monthly',
    'secure_direct.approved', 'secure_direct.requested',
    'seller.order_expiring', 'seller.traffic_drop',
    'ticket.closed', 'ticket.created', 'ticket.replied',
    'variant_batch.failed', 'vip.disabled', 'vip.enabled',
    'withdrawal.paid', 'withdrawal.processing', 'withdrawal.rejected',
    'withdrawal.requested'
  ])::text[])));

COMMIT;
