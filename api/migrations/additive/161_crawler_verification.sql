-- 161: Tauthub Crawler Verification — kredensial per-domain untuk akses crawl.
--
-- Masalah lama: crawler diizinkan lewat whitelist IP VPS, yang memaksa mitra
-- memahami WAF dan tidak bisa di-revoke per domain. Gantinya tiap domain media
-- mitra mendapat kredensial sendiri (crawler_id + secret) yang ditandatangani
-- (HMAC-SHA256) per request dan divalidasi di Cloudflare Worker milik mitra.
--
-- Secret TIDAK disimpan plaintext: kolom secret_* menyimpan ciphertext
-- AES-256-GCM + nonce + tag, kunci master dari env (CRAWLER_MASTER_KEY).
-- secret_fingerprint = 8 byte pertama SHA-256(secret) — cukup untuk mengecek
-- "secret ini milik domain mana" tanpa pernah membuka secret-nya.
--
-- Nonce satu-pakai (anti-replay) di kolom crawler_nonces; request log untuk
-- rate limit; diagnosis blokir per percobaan di crawler_diagnostics (hanya
-- fingerprint body, bukan seluruh isi halaman).
--
-- Aditif: tabel baru semua.

BEGIN;

CREATE TABLE IF NOT EXISTS public.crawler_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- media.id / partners.id bertipe character(36) (UUID disimpan sebagai teks)
    -- di seluruh schema ini — FK wajib menyamai tipe kolom yang direferensikan.
    media_id character(36) NOT NULL REFERENCES public.media(id) ON DELETE CASCADE,
    partner_id character(36) NOT NULL REFERENCES public.partners(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    crawler_id TEXT NOT NULL UNIQUE,             -- "site_" + nanoid(10)
    secret_enc BYTEA NOT NULL,
    secret_nonce BYTEA NOT NULL,
    secret_tag BYTEA NOT NULL,
    secret_fingerprint CHAR(16) NOT NULL,
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'rotating', 'disabled', 'revoked')),
    allowed_hosts TEXT[] NOT NULL DEFAULT '{}',
    allowed_paths TEXT[] NOT NULL DEFAULT ARRAY['/*'],
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rotated_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    last_successful_crawl TIMESTAMPTZ,
    last_failed_crawl TIMESTAMPTZ,
    fail_count INT NOT NULL DEFAULT 0
);

-- Satu kredensial aktif per media (rotasi memakai status 'rotating' sementara).
CREATE UNIQUE INDEX IF NOT EXISTS uq_crawler_credential_active
    ON public.crawler_credentials (media_id)
    WHERE status IN ('active', 'rotating');

CREATE INDEX IF NOT EXISTS ix_crawler_credentials_partner
    ON public.crawler_credentials (partner_id, status);

-- Anti-replay: setiap nonce hanya sah sekali dalam jendela expired.
CREATE TABLE IF NOT EXISTS public.crawler_nonces (
    nonce CHAR(32) PRIMARY KEY,                  -- hex 16 byte
    crawler_id TEXT NOT NULL,
    seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS ix_crawler_nonces_expires
    ON public.crawler_nonces (expires_at);

-- Log request untuk rate limit per domain (di-prune berkala).
CREATE TABLE IF NOT EXISTS public.crawler_request_log (
    id BIGSERIAL PRIMARY KEY,
    crawler_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS ix_crawler_request_log
    ON public.crawler_request_log (crawler_id, created_at);

-- Diagnosis blokir: tiap percobaan crawl dicatat verdict-nya supaya dashboard
-- bisa menjelaskan ke mitra apa yang terjadi (WAF? challenge? 403 origin?).
CREATE TABLE IF NOT EXISTS public.crawler_diagnostics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    credential_id UUID REFERENCES public.crawler_credentials(id) ON DELETE SET NULL,
    media_id character(36) NOT NULL REFERENCES public.media(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    url TEXT NOT NULL,
    http_status INT,
    cf_ray TEXT,
    cf_mitigated TEXT,
    server_header TEXT,
    body_fingerprint CHAR(64),                   -- SHA-256 dari 4KB pertama body
    verdict TEXT NOT NULL,                       -- ok | expired | invalid_signature | replay | rate_limited | blocked_cloudflare | blocked_waf | blocked_origin | error
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS ix_crawler_diagnostics_media_time
    ON public.crawler_diagnostics (media_id, created_at DESC);

COMMIT;