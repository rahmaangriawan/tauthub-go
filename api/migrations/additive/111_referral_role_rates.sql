-- 111 -- Komisi referral per role pendaftar: mitra 1% (100 bps), klien 5% (500 bps).
-- Dua baris tambahan di referral_settings; komisi mitra tetap memakai key lama.

INSERT INTO public.referral_settings (key, value) VALUES
  ('mitra_commission_rate_bps', '100'),
  ('klien_commission_rate_bps', '500')
ON CONFLICT (key) DO NOTHING;
