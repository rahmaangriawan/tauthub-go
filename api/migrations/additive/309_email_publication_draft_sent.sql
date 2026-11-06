-- 309 — email untuk artikel yang terkirim ke media sebagai draf.
--
-- Event baru publication.draft_sent dipisah dari publication.published supaya
-- klien tidak menerima email "sudah terbit" untuk artikel yang masih draf.
-- enabled_at diisi now() agar hanya notifikasi baru yang masuk antrean email.

INSERT INTO public.email_event_channels (event, enabled, enabled_at) VALUES
    ('publication.draft_sent', true, now())
ON CONFLICT (event) DO NOTHING;
