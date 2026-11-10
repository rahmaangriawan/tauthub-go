-- 310 — Earning affiliate (CPA, komisi referral, komisi transaksi VVIP) bisa tercatat.
--
-- partner_earnings.publication_id dulu UNIQUE dan berkunci asing ke publications.
-- Itu hanya cocok untuk earning bertipe 'publication'. Tiga tipe lain ikut
-- memakai kolom yang sama:
--   cpa                     publication_id = 'cpa:<referred_domain_id>'  (bukan id publikasi, FK menolak)
--   referral                publication_id = <id earning sumber>         (bukan id publikasi, FK menolak)
--   transaction_commission  publication_id = <id publikasi klien>        (bentrok UNIQUE dengan earning publikasi)
-- Akibatnya insert gagal atau dilewati diam-diam. Kunci unik sekarang
-- (publication_id, kind), dan FK dilepas karena nilai pengenal tipe lain bukan
-- id publikasi. Pembersihan baris earning mengikuti partner_id, bukan cascade.
-- Aditif dan idempoten.

BEGIN;

ALTER TABLE public.partner_earnings DROP CONSTRAINT IF EXISTS fk_partner_earnings_publication;
ALTER TABLE public.partner_earnings DROP CONSTRAINT IF EXISTS uq_partner_earnings_publication;

CREATE UNIQUE INDEX IF NOT EXISTS uq_partner_earnings_publication_kind
    ON public.partner_earnings (publication_id, kind);

COMMIT;
