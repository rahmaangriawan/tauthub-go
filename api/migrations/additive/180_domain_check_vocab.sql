-- 180: Samakan kosakata diagnostik domain dengan nilai yang benar-benar
-- dipancarkan security.VerifyDomainOwnership.
--
-- Constraint dari migrasi 154/155 hanya menerima sebagian nilai, sehingga UPDATE
-- diagnostik ditolak DB untuk stage fetch/waf/txt dan result token_missing/
-- blocked_by_waf. Error tulisnya ditelan (_ = d.exec), jadi kegagalan tidak
-- pernah tercatat: domain_last_checked_at tidak maju, sweep scheduler
-- (schEnqueueDomainRechecks, ambang 7 hari) meng-enqueue media yang sama tiap
-- siklus, dan notifikasi admin "Domain mitra tidak terjangkau" terkirim
-- berulang dengan angka gagal yang beku.
--
-- Nilai stage  : no_website | dns | connect | fetch | http | waf | bot_challenge | token | txt | ok
-- Nilai result : verified | token_not_found | token_mismatch | token_missing | unreachable | blocked_by_waf

ALTER TABLE ONLY public.media DROP CONSTRAINT IF EXISTS ck_media_domain_last_stage;
ALTER TABLE ONLY public.media ADD CONSTRAINT ck_media_domain_last_stage
    CHECK (domain_last_stage IS NULL OR domain_last_stage IN
        ('no_website','dns','connect','fetch','http','waf','bot_challenge','token','txt','ok'));

ALTER TABLE ONLY public.media DROP CONSTRAINT IF EXISTS ck_media_domain_last_result;
ALTER TABLE ONLY public.media ADD CONSTRAINT ck_media_domain_last_result
    CHECK (domain_last_result IS NULL OR domain_last_result IN
        ('verified','token_not_found','token_mismatch','token_missing','unreachable','blocked_by_waf'));
