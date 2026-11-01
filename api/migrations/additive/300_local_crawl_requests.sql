-- 300 — Local Retrieval Worker: antrean penarikan halaman lewat IP ISP rumahan.
--
-- Latar: WAF banyak media mitra memblokir IP datacenter (VPS Tencent kena 403
-- di semua path, termasuk sitemap/feed — terverifikasi), sementara IP ISP
-- Indonesia diterima. Saat crawl VPS gagal total (semua halaman ditolak),
-- job membuat request di tabel ini; komputer lokal milik admin (worker Bun)
-- polling endpoint claim ber-token, menarik halamannya lewat ISP rumah, lalu
-- mengirim teks hasil ekstraksi. AI tetap berjalan di VPS dengan prompt yang
-- sama seperti job biasa.
--
-- Siklus status: pending → claimed → done | failed.
-- UNIQUE(media_id): satu media satu request aktif; upsert saat trigger lagi.

CREATE TABLE IF NOT EXISTS public.local_crawl_requests (
    id            character(36) NOT NULL,
    media_id      character(36) NOT NULL REFERENCES public.media (id) ON DELETE CASCADE,
    media_name    character varying(160),
    website_url   character varying(500),
    status        character varying(16) NOT NULL DEFAULT 'pending',
    result_text   text,
    result_pages  jsonb NOT NULL DEFAULT '[]'::jsonb,
    error_message character varying(1000),
    claimed_at    timestamp with time zone,
    done_at       timestamp with time zone,
    created_at    timestamp with time zone NOT NULL DEFAULT now(),
    updated_at    timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT local_crawl_requests_pkey PRIMARY KEY (id),
    CONSTRAINT uq_local_crawl_requests_media UNIQUE (media_id),
    CONSTRAINT ck_local_crawl_requests_status
      CHECK (((status)::text = ANY ((ARRAY[
        'pending'::character varying, 'claimed'::character varying,
        'done'::character varying, 'failed'::character varying
      ])::text[])))
);

CREATE INDEX IF NOT EXISTS ix_lcr_status_time
  ON public.local_crawl_requests (status, created_at)
  WHERE status IN ('pending', 'claimed');
