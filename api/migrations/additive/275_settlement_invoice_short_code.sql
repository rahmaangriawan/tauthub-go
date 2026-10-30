-- 275 — Nomor invoice penarikan jadi 6 karakter.
--
-- Nomor lama dibuat dengan format "INV/<uuid mitra>/<urut>", sehingga di
-- halaman Invoice & Penarikan tampil sebagai deretan UUID yang panjang.
-- Nomor pengganti diturunkan dari id baris (md5) supaya stabil: menjalankan
-- migrasi ini dua kali menghasilkan nilai yang sama (idempoten).
--
-- Batasan: baris yang kode barunya kebetulan bentrok dengan nomor lain
-- dibiarkan memakai nomor lama (bukan diganti acak) — unique index
-- uq_settlement_invoice_number tetap utuh.
--
-- Catatan: invoice yang TERLANJUR dikirim ke mitra dengan nomor lama tidak
-- bisa ditarik kembali; migrasi ini menyelaraskan nomor di aplikasi.
UPDATE settlements s
SET invoice_number = u.code
FROM (
  SELECT id, upper(substr(md5(id::text || '|tauthub-invoice'), 1, 6)) AS code
  FROM settlements
  WHERE invoice_number LIKE 'INV/%'
) u
WHERE s.id = u.id
  AND NOT EXISTS (
    SELECT 1 FROM settlements x WHERE x.invoice_number = u.code AND x.id <> s.id
  );
