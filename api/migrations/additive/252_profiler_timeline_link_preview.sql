-- 252 — Pratinjau tautan (Open Graph) pada post Timeline.
--
-- Saat menempel tautan di composer, server mengambil metadata OG sekali lalu
-- menyimpannya di baris post. Feed tidak perlu fetch ulang ke situs pihak
-- ketiga setiap kali dibuka (hemat latensi + tidak membocorkan IP pembaca).
--
-- Semua kolom nullable: post lama tetap valid, post tanpa tautan tidak terpengaruh.

ALTER TABLE public.profiler_timeline_posts
    ADD COLUMN IF NOT EXISTS link_url text,
    ADD COLUMN IF NOT EXISTS link_title text,
    ADD COLUMN IF NOT EXISTS link_description text,
    ADD COLUMN IF NOT EXISTS link_image text,
    ADD COLUMN IF NOT EXISTS link_site text;
