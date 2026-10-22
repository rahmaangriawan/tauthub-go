-- 182: Izinkan mode custom jumlah kata (format custom_{min}_{max}) pada tabel media.
--
-- Memperlebar kolom media.word_count_mode dari varchar(20) ke varchar(40)
-- dan memperbarui CHECK constraint agar menerima format custom_{min}_{max}
-- selain opsi baku (sesuai_permintaan, kata_300_700, kata_800_1500).

ALTER TABLE ONLY public.media ALTER COLUMN word_count_mode TYPE character varying(40);
ALTER TABLE ONLY public.media DROP CONSTRAINT IF EXISTS ck_media_word_count_mode;
ALTER TABLE ONLY public.media ADD CONSTRAINT ck_media_word_count_mode
    CHECK (word_count_mode IN ('sesuai_permintaan', 'kata_300_700', 'kata_800_1500') OR word_count_mode ~ '^custom_[0-9]+_[0-9]+$');
