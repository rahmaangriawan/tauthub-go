-- Migration 071 - Media Pricing (NETT/NEGO) + GSC Kalkulator
--
-- Additif & backward compatible: tidak drop/hapus data existing.
-- Semua kolom baru punya DEFAULT atau NULL — media lama tetap valid.
--
-- Fitur:
--   * pricing_type  (nett|nego) + price_status (menunggu_review|disetujui|negosiasi|ditolak|final)
--     + negotiated_price (harga hasil negosiasi admin)
--   * media_price_history — riwayat perubahan harga/pengajuan
--   * gsc_connections / gsc_properties / gsc_snapshots — Kalkulator Media (GSC)
--
-- Idempotent: ADD COLUMN IF NOT EXISTS + CREATE TABLE IF NOT EXISTS + DO block untuk CK.
-- Apply: bun scripts/db/migrate-additive.mjs  (dipakai deploy-actions.sh 2/5)

-- ---------------------------------------------------------------------------
-- media: pricing columns
-- ---------------------------------------------------------------------------
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS pricing_type character varying(10) NOT NULL DEFAULT 'nett';
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS price_status character varying(20) NOT NULL DEFAULT 'menunggu_review';
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS negotiated_price numeric(14,2);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'ck_media_pricing_type' AND conrelid = 'public.media'::regclass
  ) THEN
    ALTER TABLE public.media ADD CONSTRAINT ck_media_pricing_type
      CHECK (pricing_type IN ('nett', 'nego'));
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'ck_media_price_status' AND conrelid = 'public.media'::regclass
  ) THEN
    ALTER TABLE public.media ADD CONSTRAINT ck_media_price_status
      CHECK (price_status IN ('menunggu_review', 'disetujui', 'negosiasi', 'ditolak', 'final'));
  END IF;
END $$;

-- ---------------------------------------------------------------------------
-- media_price_history
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.media_price_history (
  id                character(36) NOT NULL,
  media_id          character(36) NOT NULL,
  actor_id          character(36),
  from_price        numeric(14,2),
  to_price          numeric(14,2),
  from_pricing_type character varying(10),
  to_pricing_type   character varying(10),
  from_price_status character varying(20),
  to_price_status   character varying(20),
  note              character varying(1000),
  created_at        timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT media_price_history_pkey PRIMARY KEY (id),
  CONSTRAINT fk_media_price_history_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE,
  CONSTRAINT fk_media_price_history_actor FOREIGN KEY (actor_id) REFERENCES public.users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS ix_media_price_history_media_created
  ON public.media_price_history USING btree (media_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- GSC Kalkulator Media
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.gsc_connections (
  id                    character(36) NOT NULL,
  partner_id            character(36) NOT NULL,
  provider              character varying(20) NOT NULL DEFAULT 'google',
  email                 character varying(320),
  sealed_refresh_token  text,
  scope                 character varying(500),
  status                character varying(20) NOT NULL DEFAULT 'pending',
  created_at            timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at            timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT gsc_connections_pkey PRIMARY KEY (id),
  CONSTRAINT uq_gsc_connections_partner UNIQUE (partner_id),
  CONSTRAINT fk_gsc_connections_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE,
  CONSTRAINT ck_gsc_connections_status CHECK (status IN ('pending','active','revoked','error'))
);

CREATE TABLE IF NOT EXISTS public.gsc_properties (
  id                character(36) NOT NULL,
  connection_id     character(36) NOT NULL,
  site_url          character varying(500) NOT NULL,
  permission_level  character varying(20),
  created_at        timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT gsc_properties_pkey PRIMARY KEY (id),
  CONSTRAINT uq_gsc_properties_connection_site UNIQUE (connection_id, site_url),
  CONSTRAINT fk_gsc_properties_connection FOREIGN KEY (connection_id) REFERENCES public.gsc_connections(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS public.gsc_snapshots (
  id            character(36) NOT NULL,
  property_id   character(36) NOT NULL,
  media_id      character(36),
  period_days   integer NOT NULL,
  clicks        integer NOT NULL DEFAULT 0,
  impressions   integer NOT NULL DEFAULT 0,
  ctr           numeric(6,4) NOT NULL DEFAULT 0,
  position      numeric(6,2) NOT NULL DEFAULT 0,
  fetched_at    timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT gsc_snapshots_pkey PRIMARY KEY (id),
  CONSTRAINT fk_gsc_snapshots_property FOREIGN KEY (property_id) REFERENCES public.gsc_properties(id) ON DELETE CASCADE,
  CONSTRAINT fk_gsc_snapshots_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE SET NULL,
  CONSTRAINT ck_gsc_snapshots_period CHECK (period_days IN (7, 28, 30, 90))
);

CREATE INDEX IF NOT EXISTS ix_gsc_snapshots_property_fetched
  ON public.gsc_snapshots USING btree (property_id, fetched_at DESC);
CREATE INDEX IF NOT EXISTS ix_gsc_snapshots_media_fetched
  ON public.gsc_snapshots USING btree (media_id, fetched_at DESC);
