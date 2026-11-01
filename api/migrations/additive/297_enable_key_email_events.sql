-- 297 — aktifkan email untuk event operasional yang wajib sampai ke mitra.
--
-- Tiket dukungan dan penarikan dana sebelumnya hanya punya notifikasi in-app
-- (kebijakan registry Email=ModeOff). Pemilik produk meminta keduanya juga
-- dikirim lewat email, dan pembatalan otomatis karena SLA perlu memberi tahu
-- klien tanpa menunggu mereka membuka aplikasi.
--
-- enabled_at diisi now() supaya notifikasi lama tidak ikut terkirim; hanya
-- notifikasi baru (dibuat setelah kanal aktif) yang masuk antrean email.
-- ON CONFLICT DO NOTHING menjaga pilihan admin bila baris sudah ada.

INSERT INTO public.email_event_channels (event, enabled, enabled_at) VALUES
    ('ticket.created', true, now()),
    ('ticket.replied', true, now()),
    ('ticket.closed', true, now()),
    ('withdrawal.requested', true, now()),
    ('withdrawal.processing', true, now()),
    ('withdrawal.paid', true, now()),
    ('withdrawal.rejected', true, now()),
    ('publication.sla_cancelled', true, now())
ON CONFLICT (event) DO NOTHING;
