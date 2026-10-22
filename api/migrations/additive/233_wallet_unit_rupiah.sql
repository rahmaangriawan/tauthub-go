-- 233 — Satuan saldo dari TC ke rupiah penuh (1 unit = Rp1).
--
-- Sistem lama menghitung kredit TC dengan kurs 1 TC = Rp1.000 (tc_config).
-- Semua tampilan sekarang memakai nominal rupiah, jadi seluruh nilai TC yang
-- tersimpan (saldo, ledger, klaim topup, kredit per media, tarif penulisan &
-- gambar AI) dikonversi ×rate supaya basis datanya = rupiah, lalu rate
-- diset 1 supaya perhitungan lama (creditFromClientPrice, dst.) tetap benar
-- tanpa perubahan logika.
--
-- Konsisten dengan pilihan user: saldo lama dibawa ×1000 (1.000 TC → 1.000.000.
--
DO $$
DECLARE
  v_rate bigint;
BEGIN
  SELECT COALESCE(MAX(rate_idr), 1000) INTO v_rate
    FROM tc_config WHERE rate_idr IS NOT NULL;
  IF v_rate < 1 THEN
    v_rate := 1000;
  END IF;

  -- 1. Perlebar kolom saldo & ledger ke bigint (nilai ×1000 bisa melewati
  --    batas integer 32-bit bila saldo sudah besar).
  ALTER TABLE public.credit_wallets
    ALTER COLUMN available_tc TYPE bigint,
    ALTER COLUMN reserved_tc TYPE bigint,
    ALTER COLUMN captured_tc TYPE bigint;
  ALTER TABLE public.credit_ledger_entries
    ALTER COLUMN amount_tc TYPE bigint,
    ALTER COLUMN balance_after_tc TYPE bigint;
  ALTER TABLE public.credit_topup_reports
    ALTER COLUMN claimed_tc TYPE bigint;

  -- 2. Konversi ×rate semua nilai kredit.
  IF v_rate <> 1 THEN
    UPDATE public.credit_wallets
       SET available_tc = available_tc * v_rate,
           reserved_tc  = reserved_tc  * v_rate,
           captured_tc  = captured_tc  * v_rate;
    UPDATE public.credit_ledger_entries
       SET amount_tc         = amount_tc * v_rate,
           balance_after_tc  = balance_after_tc * v_rate;
    UPDATE public.credit_topup_reports
       SET claimed_tc = claimed_tc * v_rate;
    UPDATE public.media
       SET credit_cost = credit_cost * v_rate
     WHERE credit_cost > 0;
    UPDATE public.platform_settings
       SET value = (NULLIF(value, '')::numeric * v_rate)::int::text
     WHERE key = 'writer_credit_cost' AND value IS NOT NULL
       AND value ~ '^(\d+)(\.\d+)?$';
    UPDATE public.platform_ai_configuration
       SET image_credit_cost = image_credit_cost * v_rate
     WHERE image_credit_cost > 0;
    -- tc_config.rate_idr: simpan kurs lama lalu set 1? Baris pencatatan lama
    -- menyimpan rate lama sendiri di ledger, jadi aman direset.
  END IF;

  UPDATE public.tc_config SET rate_idr = 1 WHERE id = 1;
END $$;

COMMENT ON COLUMN public.tc_config.rate_idr IS
  'Nominal rupiah per unit saldo TC. Setelah satuan dipindah ke rupiah penuh, nilai konversi = 1.';
