-- Migration: 035_payment_reference
-- Human-readable payment reference on payment_transactions, invoice-style:
--   PAY-202608-00042
-- (PAY + year-month + 5-digit sequence). Generated at insert time from a
-- dedicated sequence (race-free, unlike a COUNT-based number). Existing rows
-- are backfilled from created_at + the same sequence.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/035_payment_reference.sql

BEGIN;

CREATE SEQUENCE IF NOT EXISTS public.payment_transaction_ref_seq START 1000;

ALTER TABLE public.payment_transactions ADD COLUMN IF NOT EXISTS reference VARCHAR(40);

-- Backfill historical rows (chronological order → stable numbering).
WITH ordered AS (
  SELECT id, created_at,
         row_number() OVER (ORDER BY created_at, id) AS n
  FROM payment_transactions
  WHERE reference IS NULL
)
UPDATE payment_transactions t
SET reference = 'PAY-' || to_char(o.created_at, 'YYYYMM') || '-' || lpad((1000 + o.n)::text, 5, '0')
FROM ordered o
WHERE t.id = o.id;

-- Advance the sequence past the backfill (3-arg setval → next nextval returns max+1).
SELECT setval('public.payment_transaction_ref_seq',
              (SELECT COALESCE(MAX(right(reference, 5)::int), 1000) FROM payment_transactions WHERE reference ~ '^PAY-'),
              TRUE) ;

CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_transaction_reference
  ON public.payment_transactions (reference) WHERE reference IS NOT NULL;

COMMIT;
