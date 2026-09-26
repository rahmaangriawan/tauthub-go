-- Settlement payout receipt (bukti transfer) yang diunggah admin.
-- Kolom ini sudah dipakai kode (route admin & mitra, _settlement.ts) tapi belum
-- pernah ada di schema/DB — tambahkan idempoten tanpa menyentuh data existing.

ALTER TABLE ONLY public.settlements
  ADD COLUMN IF NOT EXISTS receipt_url character varying(500);
