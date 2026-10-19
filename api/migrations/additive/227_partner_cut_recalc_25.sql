-- 227_partner_cut_recalc_25.sql
-- Koreksi tarif potongan platform untuk penghasilan mitra: 25% (mitra menerima
-- 75% dari gross), bukan 5% seperti perhitungan awal.
--
-- Berlaku surut untuk SEMUA baris kind='publication' (order lama) yang punya
-- gross_amount. Earning lain (referral, CPA, transaction_commission) TIDAK
-- disentuh karena bukan objek potongan platform.
--
-- Catatan: settlement yang sudah dibuat menyimpan total_amount sendiri; baris
-- yang sudah terhubung ke settlement tetap dikoreksi di ledger, tetapi total
-- settlement historis tidak diubah (sudah menjadi dokumen pembayaran).
UPDATE public.partner_earnings
SET amount = round(gross_amount * 0.75, 2)
WHERE kind = 'publication'
  AND gross_amount IS NOT NULL
  AND gross_amount > 0
  AND amount IS DISTINCT FROM round(gross_amount * 0.75, 2);
