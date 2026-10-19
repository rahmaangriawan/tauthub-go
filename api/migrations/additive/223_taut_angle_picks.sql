-- 223_taut_angle_picks.sql
-- Popularitas angle artikel: satu baris per (kampanye, media, angle) yang
-- pernah dipilih klien di modal writer. Unik per penempatan media supaya
-- klik berulang / ganti-ganti angle tidak menggelembungkan hitungan.
CREATE TABLE IF NOT EXISTS public.taut_angle_picks (
    id          character(36) PRIMARY KEY,
    angle_id    character(36) NOT NULL REFERENCES public.taut_content_angles(id) ON DELETE CASCADE,
    angle_slug  varchar(100) NOT NULL,
    brand_id    character(36),
    campaign_id character(36),
    media_id    character(36),
    user_id     character(36),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS taut_angle_picks_angle_idx
    ON public.taut_angle_picks (angle_id);

CREATE UNIQUE INDEX IF NOT EXISTS taut_angle_picks_placement_uq
    ON public.taut_angle_picks (campaign_id, media_id, angle_id)
    WHERE campaign_id IS NOT NULL AND media_id IS NOT NULL;

-- Backfill dari penempatan yang sudah punya angle tersimpan, supaya daftar
-- populer tidak kosong saat fitur baru dibuka.
INSERT INTO public.taut_angle_picks (id, angle_id, angle_slug, brand_id, campaign_id, media_id, created_at)
SELECT gen_random_uuid()::text, a.id, a.slug, c.brand_id, tcm.campaign_id, tcm.media_id, tcm.created_at
FROM public.taut_campaign_media tcm
JOIN public.taut_content_angles a ON a.slug = tcm.angle_slug
JOIN public.campaigns c ON c.id = tcm.campaign_id
WHERE tcm.angle_slug IS NOT NULL AND btrim(tcm.angle_slug) <> ''
ON CONFLICT DO NOTHING;
