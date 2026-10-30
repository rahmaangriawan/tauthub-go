-- 260 — WA Gateway: tujuan grup untuk notifikasi admin.
--
-- Selain fan-out ke nomor pribadi admin (notification_preferences), admin bisa
-- mengisi ID GRUP WA Fonnte. Bila terisi, notifikasi admin (pengajuan media,
-- penarikan dana, tiket dukungan) dikirim SATU pesan ke grup — bukan ke nomor
-- pribadi masing-masing admin. Kosong = perilaku lama (per admin).
ALTER TABLE public.platform_wa_config
  ADD COLUMN IF NOT EXISTS admin_group_target character varying(64);
