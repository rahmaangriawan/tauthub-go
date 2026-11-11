-- 314 — Bukti tayang dan garansi tautan.
--
-- 1. publication_link_snapshots: riwayat kondisi tautan per publikasi (rel,
--    robots, status HTTP). Sebelumnya publication_link_checks hanya menyimpan
--    hasil TERAKHIR, jadi tidak ada pembanding untuk "kondisi saat tayang".
--    Satu baris kind='publish' per publikasi (baseline), baris kind='check'
--    ditulis bila kondisi berubah atau sudah seminggu tanpa catatan.
-- 2. publication_link_checks: status rel/noindex dan hitungan pelanggaran
--    berturut-turut (garansi baru jalan setelah dua pengecekan berturut-turut).
-- 3. link_guardian_claims: klaim otomatis, pilihan klien (artikel pengganti
--    atau kredit), dan jenis pelanggaran baru (noindex, rel_changed).
-- 4. Tipe notifikasi baru, ditambahkan ke constraint yang berlaku (definisi
--    terakhir di 299) tanpa menghapus nilai lama.
--
-- Additive dan idempoten.

BEGIN;

CREATE TABLE IF NOT EXISTS public.publication_link_snapshots (
  id              bigserial PRIMARY KEY,
  publication_id  character(36) NOT NULL REFERENCES public.publications (id) ON DELETE CASCADE,
  kind            text NOT NULL,
  href            text,
  anchor          text,
  rel             text,
  robots_meta     text,
  x_robots_tag    text,
  http_status     integer,
  link_state      text,
  in_article_state text,
  rel_state       text,
  noindex_state   text,
  checked_at      timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT publication_link_snapshots_kind_check CHECK (kind IN ('publish', 'check'))
);

CREATE INDEX IF NOT EXISTS publication_link_snapshots_pub_idx
  ON public.publication_link_snapshots (publication_id, checked_at DESC);

-- Satu baseline per publikasi.
CREATE UNIQUE INDEX IF NOT EXISTS publication_link_snapshots_publish_key
  ON public.publication_link_snapshots (publication_id) WHERE kind = 'publish';

ALTER TABLE public.publication_link_checks ADD COLUMN IF NOT EXISTS rel_value text;
ALTER TABLE public.publication_link_checks ADD COLUMN IF NOT EXISTS rel_state text;
ALTER TABLE public.publication_link_checks ADD COLUMN IF NOT EXISTS robots_value text;
ALTER TABLE public.publication_link_checks ADD COLUMN IF NOT EXISTS noindex_state text;
ALTER TABLE public.publication_link_checks ADD COLUMN IF NOT EXISTS violation_streak integer NOT NULL DEFAULT 0;
ALTER TABLE public.publication_link_checks ADD COLUMN IF NOT EXISTS violation_kind text;

ALTER TABLE public.link_guardian_claims ADD COLUMN IF NOT EXISTS auto_detected boolean NOT NULL DEFAULT FALSE;
ALTER TABLE public.link_guardian_claims ADD COLUMN IF NOT EXISTS client_choice text;
ALTER TABLE public.link_guardian_claims ADD COLUMN IF NOT EXISTS client_chosen_at timestamptz;
ALTER TABLE public.link_guardian_claims ADD COLUMN IF NOT EXISTS replacement_publication_id character(36);

ALTER TABLE public.link_guardian_claims DROP CONSTRAINT IF EXISTS link_guardian_claims_detection_check;
ALTER TABLE public.link_guardian_claims ADD CONSTRAINT link_guardian_claims_detection_check
  CHECK (detection_state IS NULL OR detection_state IN ('gone', 'link_removed', 'noindex', 'rel_changed'));

ALTER TABLE public.link_guardian_claims DROP CONSTRAINT IF EXISTS link_guardian_claims_choice_check;
ALTER TABLE public.link_guardian_claims ADD CONSTRAINT link_guardian_claims_choice_check
  CHECK (client_choice IS NULL OR client_choice IN ('credit', 'replacement'));

-- status 'replaced': klien memilih artikel pengganti dan penggantinya sudah dibuat.
ALTER TABLE public.link_guardian_claims DROP CONSTRAINT IF EXISTS link_guardian_claims_status_check;
ALTER TABLE public.link_guardian_claims ADD CONSTRAINT link_guardian_claims_status_check
  CHECK (status IN ('submitted', 'checking', 'contacted', 'refunded', 'rejected', 'replaced'));

ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type
  CHECK (((type)::text = ANY ((ARRAY[
    'announcement.new', 'billing.balance_forfeited', 'billing.balance_forfeit_warning',
    'billing.topup_needed', 'changelog.new', 'chat.message',
    'media.approved', 'media.crawl_blocked', 'media.dewan_pers.approved',
    'media.dewan_pers.rejected', 'media.dewan_pers.submitted', 'media.domain_down',
    'media.integration_missing',
    'media.negotiated', 'media.rejected', 'media.resubmitted', 'media.submitted',
    'partner.guarantee_reversed',
    'partner.wa_required',
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
    'publication.waiting_partner',
    'referral.commission.credited', 'referral.commission.reversed',
    'referral.cpa.credited', 'referral.domain.approved', 'referral.domain.rejected',
    'secure_direct.approved', 'secure_direct.requested',
    'ticket.closed', 'ticket.created', 'ticket.replied',
    'variant_batch.failed', 'vip.disabled', 'vip.enabled',
    'withdrawal.paid', 'withdrawal.processing', 'withdrawal.rejected',
    'withdrawal.requested'
  ])::text[])));

COMMIT;
