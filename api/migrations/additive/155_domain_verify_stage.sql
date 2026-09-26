-- 155: Tahap kegagalan verifikasi domain (diagnostik berjenjang).
--
-- Migrasi 154 menyimpan SEBAB ("unreachable") dan PESAN ("ECONNREFUSED: ..."),
-- tapi admin masih harus menebak bagian mana yang gagal. Kolom di sini
-- menyimpan tahapnya secara eksplisit + jejak langkah lengkapnya, sehingga
-- halaman Diagnostik Domain bisa menunjuk "gagal di DNS", "gagal di TLS",
-- "server menolak (HTTP 403)", atau "halaman terbaca, kode tidak ada".
--
-- Nilai domain_last_stage:
--   no_website | dns | connect | http | bot_challenge | token | ok

ALTER TABLE public.media
    ADD COLUMN IF NOT EXISTS domain_last_stage character varying(20),
    ADD COLUMN IF NOT EXISTS domain_last_trace text;

ALTER TABLE ONLY public.media DROP CONSTRAINT IF EXISTS ck_media_domain_last_stage;
ALTER TABLE ONLY public.media ADD CONSTRAINT ck_media_domain_last_stage
    CHECK (domain_last_stage IS NULL OR domain_last_stage IN ('no_website','dns','connect','http','bot_challenge','token','ok'));
