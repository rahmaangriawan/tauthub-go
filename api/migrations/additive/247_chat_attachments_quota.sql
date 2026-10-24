-- 247 — Ledger unggahan chatbot (kuota + siklus hidup berkas sementara).
--
-- chat_attachments = buku besar SEMUA unggahan chatbot Taut Brief (dokumen &
-- gambar). Dipakai untuk:
--   1) penegakan kuota: per percakapan (profil), per user per hari, dan total
--      berkas aktif per user — dicek dalam satu transaksi dengan advisory lock
--      sehingga aman terhadap unggahan bersamaan;
--   2) siklus hidup berkas fisik: processing → processed → delete_pending →
--      deleted. Berkas fisik di UPLOAD_DIR bersifat SEMENTARA; teks hasil
--      ekstraksi dokumen TETAP di profiler_documents.extracted_text sehingga
--      memori AI tidak hilang saat berkas dibersihkan.
-- Isi berkas biner TIDAK disimpan di PostgreSQL — hanya metadata.
--
-- Pemetaan istilah: "conversation" pada kebijakan kuota = profiler_profiles
-- (satu brief = satu percakapan chat).

CREATE TABLE IF NOT EXISTS public.chat_attachments (
    id character(36) NOT NULL PRIMARY KEY,
    user_id character(36) NOT NULL,
    profile_id character(36) NOT NULL,
    kind character varying(10) NOT NULL,
    source character varying(10) NOT NULL DEFAULT 'chat',
    storage_key character varying(255) NOT NULL,
    original_filename character varying(255) NOT NULL,
    mime_type character varying(120) NOT NULL,
    size_bytes bigint NOT NULL,
    status character varying(20) NOT NULL DEFAULT 'processing',
    parse_error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    processed_at timestamp with time zone,
    expires_at timestamp with time zone,
    deleted_at timestamp with time zone,
    CONSTRAINT ck_chat_attachments_kind CHECK (kind IN ('document', 'image')),
    CONSTRAINT ck_chat_attachments_source CHECK (source IN ('chat', 'library')),
    CONSTRAINT ck_chat_attachments_status CHECK (
        status IN ('uploaded', 'processing', 'processed', 'delete_pending', 'deleted', 'failed')
    )
);

-- Satu berkas fisik = satu baris ledger (storage_key adalah nama berkas internal
-- yang dihasilkan server, bukan nama asli user).
CREATE UNIQUE INDEX IF NOT EXISTS chat_attachments_storage_uidx
    ON public.chat_attachments (storage_key);

-- Kuota percakapan (aktif = belum deleted).
CREATE INDEX IF NOT EXISTS chat_attachments_profile_idx
    ON public.chat_attachments (profile_id)
    WHERE status <> 'deleted';

-- Kuota harian per user.
CREATE INDEX IF NOT EXISTS chat_attachments_user_day_idx
    ON public.chat_attachments (user_id, created_at);

-- Sapuan cleanup: baris yang siap kadaluarsa / menunggu penghapusan.
CREATE INDEX IF NOT EXISTS chat_attachments_expiry_idx
    ON public.chat_attachments (status, expires_at)
    WHERE status IN ('processed', 'failed', 'delete_pending');

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_attachments_profile_fk') THEN
        ALTER TABLE public.chat_attachments
            ADD CONSTRAINT chat_attachments_profile_fk
            FOREIGN KEY (profile_id) REFERENCES public.profiler_profiles (id) ON DELETE CASCADE;
    END IF;
    -- Tanpa FK ke users: baris lama bisa memiliki created_by NULL (dipetakan ke
    -- zero-UUID saat backfill) dan users memakai char(36) warisan.
END $$;

-- Penanda berkas fisik dokumen sudah dibersihkan. Baris profiler_documents
-- TETAP dipertahankan (teks ekstraksi = memori AI); tab Media menampilkan
-- status "berkas dibersihkan" alih-alih tautan unduhan mati.
ALTER TABLE public.profiler_documents
    ADD COLUMN IF NOT EXISTS file_deleted_at timestamp with time zone;

-- Backfill: unggahan lama masuk ledger supaya kuota & cleanup mencakupnya.
-- Retensi dihitung DARI MIGRASI (bukan created_at) agar file lama tidak
-- langsung tersapu pada siklus pertama setelah deploy.
INSERT INTO public.chat_attachments
    (id, user_id, profile_id, kind, source, storage_key, original_filename,
     mime_type, size_bytes, status, created_at, processed_at, expires_at)
SELECT
    d.id,
    COALESCE(d.created_by, '00000000-0000-0000-0000-000000000000'),
    d.profile_id,
    'document',
    'library',
    d.storage_key,
    d.original_filename,
    COALESCE(d.mime_type, 'application/octet-stream'),
    COALESCE(d.size_bytes, 0),
    'processed',
    d.created_at,
    d.created_at,
    now() + interval '24 hours'
FROM public.profiler_documents d
WHERE d.deleted_at IS NULL
ON CONFLICT (storage_key) DO NOTHING;

INSERT INTO public.chat_attachments
    (id, user_id, profile_id, kind, source, storage_key, original_filename,
     mime_type, size_bytes, status, created_at, processed_at, expires_at)
SELECT
    i.id,
    COALESCE(i.created_by, '00000000-0000-0000-0000-000000000000'),
    i.profile_id,
    'image',
    'library',
    i.storage_key,
    COALESCE(i.file_name, i.storage_key),
    COALESCE(i.mime_type, 'application/octet-stream'),
    COALESCE(i.size_bytes, 0),
    'processed',
    i.created_at,
    i.created_at,
    now() + interval '720 hours'
FROM public.profiler_images i
WHERE i.deleted_at IS NULL
ON CONFLICT (storage_key) DO NOTHING;
