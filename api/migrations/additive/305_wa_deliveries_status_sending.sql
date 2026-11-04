-- 305 — Perbaiki antrean WhatsApp yang tidak pernah mengirim.
--
-- Migrasi 265/266 dan mjPickDueWA (platform/job_messaging.go) mengklaim baris
-- dengan status 'sending', tetapi CHECK wa_deliveries_status_check dari 165
-- hanya mengizinkan queued/sent/failed. Akibatnya setiap klaim ditolak database
-- (SQLSTATE 23514) dan job send_wa gagal terus: tidak ada WA terkirim sama
-- sekali (ribuan job send_wa failed sejak 1 Nov 2026). Additive.

BEGIN;

ALTER TABLE public.wa_deliveries DROP CONSTRAINT IF EXISTS wa_deliveries_status_check;
ALTER TABLE public.wa_deliveries
  ADD CONSTRAINT wa_deliveries_status_check
  CHECK (((status)::text = ANY (ARRAY['queued'::text, 'sending'::text, 'sent'::text, 'failed'::text])));

-- Pesan yang tertahan lebih dari 3 jam akibat bug di atas sudah basi (mis. kabar
-- "media disetujui" berhari-hari lalu); jangan dikirim tiba-tiba setelah perbaikan.
UPDATE public.wa_deliveries
   SET status = 'failed',
       last_error = 'Kedaluwarsa: tertahan sebelum perbaikan antrean WA (migrasi 305)'
 WHERE status = 'queued'
   AND created_at < now() - interval '3 hours';

COMMIT;
