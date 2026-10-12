ALTER TABLE public.taut_campaign_media
  ADD COLUMN IF NOT EXISTS angle_slug text;

COMMENT ON COLUMN public.taut_campaign_media.angle_slug IS
  'Writer content angle slug for this placement, resolved against the target media channel. NULL preserves the legacy Profiler angle fallback.';
