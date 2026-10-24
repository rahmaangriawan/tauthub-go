-- 251 — Timeline brief brand (post gambar + caption + hashtag, retensi 3 bulan).
--
-- Pemilik brand mencatat kabar terbaru (promo, kunjungan, peluncuran) supaya
-- AI Writer punya bahan SEGAR — makin baru informasinya, makin unik artikelnya.
--
-- RETENSI: post kedaluwarsa 3 bulan (permintaan pemilik). Baris dihapus oleh
-- worker upload-cleanup; gambar fisiknya ikut lewat ledger chat_attachments
-- (status delete_pending) sehingga tidak ada file yatim. Gambar timeline
-- memakai kolom profiler_images yang sama (kuota & dedupe berlaku).
--
-- Batas: satu gambar per post (video tidak didukung), caption maks 2000
-- karakter, maks 10 hashtag per post.

CREATE TABLE IF NOT EXISTS public.profiler_timeline_posts (
    id character(36) NOT NULL PRIMARY KEY,
    profile_id character(36) NOT NULL,
    body text NOT NULL,
    hashtags text[] NOT NULL DEFAULT '{}',
    image_id character(36),
    image_url text,
    -- Tanggal kejadian (opsional): promo bertanggal, kunjungan, dsb. NULL =
    -- gunakan created_at sebagai acuannya.
    occurred_on date,
    created_by character(36),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);

CREATE INDEX IF NOT EXISTS profiler_timeline_profile_idx
    ON public.profiler_timeline_posts (profile_id, created_at DESC)
    WHERE deleted_at IS NULL;

-- Sapuan retensi 3 bulan.
CREATE INDEX IF NOT EXISTS profiler_timeline_expiry_idx
    ON public.profiler_timeline_posts (created_at)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS profiler_timeline_hashtags_idx
    ON public.profiler_timeline_posts USING GIN (hashtags);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'profiler_timeline_profile_fk') THEN
        ALTER TABLE public.profiler_timeline_posts
            ADD CONSTRAINT profiler_timeline_profile_fk
            FOREIGN KEY (profile_id) REFERENCES public.profiler_profiles (id) ON DELETE CASCADE;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'profiler_timeline_image_fk') THEN
        ALTER TABLE public.profiler_timeline_posts
            ADD CONSTRAINT profiler_timeline_image_fk
            FOREIGN KEY (image_id) REFERENCES public.profiler_images (id) ON DELETE SET NULL;
    END IF;
END $$;
