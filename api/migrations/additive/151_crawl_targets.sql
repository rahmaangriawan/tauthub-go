-- 151 — Crawling otomatis media mitra: salinan lokal hasil crawl Crawlhub.
-- Aditif: tabel baru, tanpa DROP/ubah data.
--
-- Alasannya: sebelumnya seluruh status crawl hanya hidup di service Crawlhub
-- (crawl.tauthub.com), jadi modal media memanggil service itu live setiap kali
-- dibuka dan tidak ada monitoring/antrean di sisi Tauthub. Tabel ini adalah
-- salinan hasil terakhir per media — ditulis oleh job crawl_target_sync.
--
-- Tabel konfigurasi service sendiri ada di migrasi 150 (platform_crawlhub_config).

CREATE TABLE IF NOT EXISTS public.crawl_targets (
    media_id             character(36) NOT NULL,
    website_url          character varying(500),
    active               boolean NOT NULL DEFAULT TRUE,
    -- Hasil crawl terakhir dari Crawlhub.
    articles_7d          integer,
    confidence           character varying(50),
    source               character varying(100),
    last_crawled_at      timestamp(6) with time zone,
    last_status          character varying(50),
    consecutive_failures integer NOT NULL DEFAULT 0,
    last_error           character varying(500),
    created_at           timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at           timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_crawl_targets_failures CHECK (consecutive_failures >= 0)
);

ALTER TABLE ONLY public.crawl_targets
    DROP CONSTRAINT IF EXISTS crawl_targets_pkey;
ALTER TABLE ONLY public.crawl_targets
    ADD CONSTRAINT crawl_targets_pkey PRIMARY KEY (media_id);

ALTER TABLE ONLY public.crawl_targets
    DROP CONSTRAINT IF EXISTS fk_crawl_targets_media;
ALTER TABLE ONLY public.crawl_targets
    ADD CONSTRAINT fk_crawl_targets_media
    FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE;

-- Sweep re-crawl kadaluarsa memfilter last_crawled_at.
CREATE INDEX IF NOT EXISTS ix_crawl_targets_crawled
    ON public.crawl_targets (last_crawled_at);
