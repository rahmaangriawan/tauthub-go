-- 236 — Mitra bisa memulai obrolan ("Hubungi Klien").
--
-- Sebelumnya sesi chat selalu dimulai klien. Mitra kini bisa membuka obrolan
-- per publikasi dengan alasan terstruktur: konfirmasi, revisi, atau butuh
-- kutipan (khusus media mainstream). Alasan disimpan agar klien melihat
-- konteksnya langsung di header thread; isi lengkap tetap ada di pesan pertama.
--
-- Sesi tetap 3×24 jam dan dihitung sejak pesan PERTAMA dari pihak mana pun.

ALTER TABLE public.chat_threads
  ADD COLUMN IF NOT EXISTS topic varchar(20),
  ADD COLUMN IF NOT EXISTS topic_note text;

DO $$
BEGIN
  ALTER TABLE public.chat_threads
    ADD CONSTRAINT ck_chat_threads_topic
    CHECK (topic IS NULL OR topic IN ('confirm', 'revision', 'quote'));
EXCEPTION
  WHEN duplicate_object THEN NULL;
END $$;
