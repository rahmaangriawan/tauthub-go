-- 255 — Nomor invoice penarikan mitra lebih panjang dari kolomnya.
--
-- Format nomor dari server: "INV/<uuid mitra>/<urutan>" = 4 + 36 + 1 + 3 = 44
-- karakter, sedangkan kolom settlements.invoice_number hanya varchar(40).
-- INSERT settlement gagal dengan "value too long for type character varying(40)",
-- jadi mitra tidak pernah bisa mengajukan penarikan (POST /v1/partner/settlements
-- balas 500; di produksi pesannya ter-scrub jadi "Terjadi kesalahan internal").
--
-- Format nomor TIDAK diubah — nomor yang sudah tampil di invoice mitra tetap
-- sama bentuknya. Kolomnya yang dilebarkan (64 karakter: cukup untuk urutan
-- sampai 23 digit).
--
-- Additive: hanya memperlebar tipe kolom, tidak menghapus/mengubah data.
-- Idempoten: menjalankan ALTER yang sama pada kolom varchar(64) tidak berefek.

ALTER TABLE public.settlements
    ALTER COLUMN invoice_number TYPE character varying(64);
