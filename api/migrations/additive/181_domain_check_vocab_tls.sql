-- 181: Tambah kosakata "tls_error" (result) dan "tls" (stage) ke
-- diagnostik domain media.
--
-- 180 sudah menambahkan blocked_by_waf, tetapi masih melewatkan nilai yang
-- dipancarkan security.VerifyDomainOwnership untuk kegagalan TLS: Result
-- "tls_error" + Stage "tls". UPDATE diagnostik ditolak DB (SQLSTATE 23514,
-- ck_media_domain_last_result / ck_media_domain_last_stage) dan errornya
-- ditelan, sehingga domain_last_checked_at tidak maju → sweep domain_verify
-- meng-enqueue ulang media yang sama tiap siklus scheduler (loop 360 job/hari
-- untuk satu media di production yang HTTPS-nya mati permanen).
--
-- Additive: DROP + ADD constraint bernama sama, idempoten bila dijalankan ulang
-- oleh runner migrasi.

ALTER TABLE ONLY public.media DROP CONSTRAINT IF EXISTS ck_media_domain_last_stage;
ALTER TABLE ONLY public.media ADD CONSTRAINT ck_media_domain_last_stage
    CHECK (domain_last_stage IS NULL OR domain_last_stage IN
        ('no_website','dns','connect','fetch','http','waf','bot_challenge','tls','token','txt','ok'));

ALTER TABLE ONLY public.media DROP CONSTRAINT IF EXISTS ck_media_domain_last_result;
ALTER TABLE ONLY public.media ADD CONSTRAINT ck_media_domain_last_result
    CHECK (domain_last_result IS NULL OR domain_last_result IN
        ('verified','token_not_found','token_mismatch','token_missing','unreachable','blocked_by_waf','tls_error'));
