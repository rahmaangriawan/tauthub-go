-- 232 — Nominal rupiah di ledger & laporan topup memakai bigint.
--
-- Batas penyesuaian manual adalah 10.000.000 TC. Dengan rate bawaan Rp1.000
-- per TC, nominalnya Rp10.000.000.000 — melewati batas integer 32-bit
-- (2.147.483.647), sehingga topup besar yang lolos validasi API gagal saat
-- INSERT dan muncul sebagai error server. Memperlebar tipe tidak mengubah
-- isi data lama.
ALTER TABLE public.credit_ledger_entries
  ALTER COLUMN amount_idr TYPE bigint;

ALTER TABLE public.credit_topup_reports
  ALTER COLUMN amount_idr TYPE bigint;
