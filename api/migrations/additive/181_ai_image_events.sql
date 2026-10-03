-- 181_ai_image_events.sql
-- Log & monitoring generate gambar AI (halaman admin Taut Pintar > AI Gambar).
--
-- GenerateImage tidak pernah hard-fail: bila provider belum lengkap atau
-- panggilannya gagal, ia mengembalikan placeholder lokal dan alasannya hanya
-- masuk log server (slog). Akibatnya admin tidak tahu KENAPA gambar jadi
-- placeholder. Tabel ini menyimpan setiap percobaan (sukses, fallback
-- placeholder, gagal keras) beserta sebabnya supaya bisa dipantau dari UI.
--
-- Append-only; tidak ada relasi keras ke article_images (image_id/url dicatat
-- sebagai referensi lunak). Idempoten: aman dijalankan berulang.

CREATE TABLE IF NOT EXISTS public.ai_image_events (
    id text PRIMARY KEY DEFAULT gen_random_uuid()::text,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    event_kind text NOT NULL DEFAULT 'generation',
    status text NOT NULL,
    provider text,
    model text,
    base_url text,
    error_detail text,
    prompt text,
    role text,
    brand_id character(36),
    campaign_id character(36),
    media_id character(36),
    article_id character(36),
    requested_by character(36),
    width integer NOT NULL DEFAULT 0,
    height integer NOT NULL DEFAULT 0,
    credits_charged integer NOT NULL DEFAULT 0,
    elapsed_ms integer NOT NULL DEFAULT 0,
    image_id text,
    image_url text,
    CONSTRAINT ck_ai_image_events_kind CHECK (event_kind IN ('generation', 'connection_test')),
    CONSTRAINT ck_ai_image_events_status CHECK (status IN ('succeeded', 'fallback', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_ai_image_events_created ON public.ai_image_events (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_image_events_status ON public.ai_image_events (status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_image_events_kind ON public.ai_image_events (event_kind, created_at DESC);
