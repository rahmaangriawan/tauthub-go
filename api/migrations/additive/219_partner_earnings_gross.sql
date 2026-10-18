-- Penghasilan mitra kini disimpan dua angka: gross_amount (angka yang
-- diajukan sebelum potongan platform) dan amount (pendapatan bersih yang
-- diterima mitra setelah potongan 5%). Kolom amount tetap yang dipakai saldo
-- dan penarikan, jadi angka keuangan otomatis mengikuti nilai bersih.
ALTER TABLE public.partner_earnings ADD COLUMN IF NOT EXISTS gross_amount numeric(14,2);

-- Baris lama belum punya gross: anggap amount lama = gross.
UPDATE public.partner_earnings SET gross_amount = amount WHERE gross_amount IS NULL;

-- Potong 5% HANYA untuk earning publikasi lama yang belum dibayar. Baris yang
-- sudah masuk settlement (dibayar) dibiarkan utuh sebagai riwayat, dan jenis
-- lain (referral/CPA/komisi VIP) tidak dipotong. Idempoten: setelah dipotong
-- gross <> amount sehingga syarat gross_amount = amount tidak terpenuhi lagi.
UPDATE public.partner_earnings
SET amount = ROUND(gross_amount * 0.95, 2)
WHERE settlement_id IS NULL
  AND kind = 'publication'
  AND gross_amount = amount
  AND amount > 0;
