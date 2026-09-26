-- 143_ga4_api_secret_seal.sql
-- Seal GA4 Measurement Protocol api_secret (audit: satu-satunya credential
-- yang tersimpan plaintext). Additive: kolom plaintext lama DIPERTAHANKAN agar
-- nilai existing tetap terbaca selama transisi; tulisan baru mengisi kolom
-- sealed. Kolom lama di-drop di migration pembersihan berikutnya setelah
-- semua row di-rotate (simpan ulang config).
ALTER TABLE public.ga4_configs ADD COLUMN IF NOT EXISTS sealed_api_secret text;
