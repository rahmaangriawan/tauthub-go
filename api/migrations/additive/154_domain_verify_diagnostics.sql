-- 154: Diagnostik verifikasi domain.
--
-- Sebelumnya penyebab kegagalan cek domain hanya hidup di console produksi dan
-- sekali tampil di layar mitra, jadi admin tidak bisa menelusuri kenapa sebuah
-- media gagal verifikasi ("situs tak terjangkau" vs "kode salah" vs "diblokir
-- WAF" tampak sama). Dua kolom ini menyimpan sebabnya supaya bisa dibaca kapan
-- saja dari halaman admin.
--
-- domain_health sengaja TIDAK diperluas: nilainya dibatasi CHECK constraint
-- (migrasi 148) dan dipakai Pelacak 404. Sebab yang lebih rinci (mis. token
-- ketemu tapi nilainya beda) masuk ke domain_last_result.

ALTER TABLE public.media
    ADD COLUMN IF NOT EXISTS domain_last_error text,
    ADD COLUMN IF NOT EXISTS domain_last_error_at timestamptz,
    ADD COLUMN IF NOT EXISTS domain_last_result character varying(20);

-- domain_last_result: sebab yang dilaporkan ke UI.
--   verified | token_not_found | token_mismatch | unreachable
ALTER TABLE ONLY public.media DROP CONSTRAINT IF EXISTS ck_media_domain_last_result;
ALTER TABLE ONLY public.media ADD CONSTRAINT ck_media_domain_last_result
    CHECK (domain_last_result IS NULL OR domain_last_result IN ('verified','token_not_found','token_mismatch','unreachable'));

-- Admin menyaring "domain yang perlu ditindak": gagal terakhir kali dicek.
CREATE INDEX IF NOT EXISTS ix_media_domain_last_result
    ON public.media (domain_last_result, domain_last_error_at DESC);
