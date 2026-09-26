-- 156: Jumlah kata per media (setelan mitra).
--
-- Mitra memilih panjang artikel yang dihasilkan untuk media-nya; the writer
-- memakai nilai ini sebagai aturan panjang saat membuat artikel. Tiga pilihan:
--   sesuai_permintaan → tidak dibatasi; aturan lama per tipe media tetap berlaku
--                       (mainstream min 300, blogger 500-5000)
--   kata_300_700      → artikel 300-700 kata
--   kata_800_1500     → artikel 800-1500 kata
--
-- Kolom BARU di media, bukan menumpang media_content_standards.min_words /
-- max_words: tabel itu milik admin untuk standar editorial + cek standar, dan
-- kolomnya NOT NULL default 400/1500 sehingga keadaan "sesuai permintaan" tidak
-- bisa diwakili di sana.
--
-- Aditif: kolom baru + constraint. Tidak destructive, tidak mengubah kolom lama.
-- DEFAULT 'sesuai_permintaan' -> seluruh media yang sudah ada otomatis masuk
-- opsi default, jadi perilaku generasi artikel yang berjalan tidak berubah.

ALTER TABLE public.media
    ADD COLUMN IF NOT EXISTS word_count_mode character varying(20) NOT NULL DEFAULT 'sesuai_permintaan';

ALTER TABLE ONLY public.media DROP CONSTRAINT IF EXISTS ck_media_word_count_mode;
ALTER TABLE ONLY public.media ADD CONSTRAINT ck_media_word_count_mode
    CHECK (word_count_mode IN ('sesuai_permintaan', 'kata_300_700', 'kata_800_1500'));
