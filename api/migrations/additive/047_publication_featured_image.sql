-- 047_publication_featured_image.sql
-- Per-publikasi featured image/thumbnail (overrides media.publish_defaults).

ALTER TABLE public.publications ADD COLUMN featured_image_url character varying(500);