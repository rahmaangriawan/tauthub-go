-- 102 — Taut Pintar: policy hyperlink deterministik per media.
-- Gatekeeper hanya menanam hyperlink pada media yang mengizinkan.
-- Default: blogger izinkan (review/perbandingan), mainstream tidak. Admin bisa toggle kolom ini nanti.
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS allow_hyperlink boolean NOT NULL DEFAULT FALSE;
UPDATE public.media SET allow_hyperlink = TRUE WHERE media_type = 'blogger' AND allow_hyperlink IS NOT TRUE;