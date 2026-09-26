-- Migration: 068_withdraw_config
-- Saldo minimal penarikan (withdraw) Mitra yang DAPAT DIATUR Admin.
--
-- Sebelumnya nilai ini di-hardcode Rp 10.000 di kode. Sekarang disimpan di
-- tc_config (tabel singleton id=1 yang sudah ada — dipakai juga untuk rate
-- TC & paket top-up), sehingga Admin dapat mengubahnya dari dashboard tanpa
-- deploy. Backward compatible: default 10000 = nilai existing.
--
-- Additive & idempotent: ADD COLUMN IF NOT EXISTS + DO block untuk CHECK.
-- Tidak menghapus/mengubah data existing.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/068_withdraw_config.sql

ALTER TABLE public.tc_config
    ADD COLUMN IF NOT EXISTS minimum_withdraw_idr integer NOT NULL DEFAULT 10000;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_tc_config_min_withdraw'
          AND conrelid = 'public.tc_config'::regclass
    ) THEN
        ALTER TABLE public.tc_config
            ADD CONSTRAINT ck_tc_config_min_withdraw
            CHECK (minimum_withdraw_idr > 0);
    END IF;
END $$;
