-- 109 -- Referral code jadi kode acak 5 karakter (a-z0-9, tanpa username pemilik).
-- Backfill semua referral_code existing agar seragam format baru.

BEGIN;

DO $$
DECLARE
  r RECORD;
  new_code TEXT;
  attempts INT;
BEGIN
  FOR r IN
    SELECT id FROM public.partners
    WHERE referral_code IS NOT NULL OR referral_code_normalized IS NOT NULL
  LOOP
    attempts := 0;
    LOOP
      attempts := attempts + 1;
      new_code := substring(md5(random()::text) FROM 1 FOR 5);
      BEGIN
        UPDATE public.partners
        SET referral_code = new_code, referral_code_normalized = lower(new_code)
        WHERE id = r.id;
        EXIT;
      EXCEPTION WHEN unique_violation THEN
        IF attempts > 25 THEN EXIT; END IF;
      END;
    END LOOP;
  END LOOP;
END $$;

COMMIT;
