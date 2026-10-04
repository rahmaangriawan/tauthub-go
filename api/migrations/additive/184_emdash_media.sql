-- 184_emdash_media.sql
-- integration_type 'emdash' sudah diizinkan di API (partner/partner.go, media
-- platform) dan dropdown admin, tetapi constraint lama (081_blogger_media.sql)
-- belum memuatnya sehingga baris media EmDash tidak bisa disimpan sama sekali.
ALTER TABLE ONLY public.media DROP CONSTRAINT IF EXISTS ck_media_integration_type;
ALTER TABLE ONLY public.media ADD CONSTRAINT ck_media_integration_type
CHECK (integration_type IN ('manual','wordpress','rss','webhook','tulisio_api','custom_web','blogger','emdash'));
