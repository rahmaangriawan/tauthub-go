-- 100 — Taut Pintar: edit artikel per media (gambar + kategori).
-- Doc: docs/Flow Taut Pintar/taut-pintar-master-flow.md — "Edit: Judul, Gambar, Kategori".
ALTER TABLE public.taut_campaign_media ADD COLUMN IF NOT EXISTS featured_image_url character varying(500);
ALTER TABLE public.taut_campaign_media ADD COLUMN IF NOT EXISTS category character varying(120);