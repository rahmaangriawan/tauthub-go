-- Partner verification documents (Dewan Pers / legalitas media).
--
-- Mitra mengunggah dokumen verifikasi (sertifikat Dewan Pers, SIUP/NIB, dll).
-- File disimpan di public/uploads/ (storage_key = nama file), baris ini hanya
-- metadata + status review admin. Data existing tidak diubah — tabel baru.

CREATE TABLE IF NOT EXISTS public.partner_documents (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    name character varying(255) NOT NULL,
    type character varying(10) DEFAULT 'pdf'::character varying NOT NULL,
    size_bytes integer DEFAULT 0 NOT NULL,
    storage_key character varying(500) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    note character varying(500),
    uploaded_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    reviewed_at timestamp(6) with time zone,
    CONSTRAINT partner_documents_pkey PRIMARY KEY (id),
    CONSTRAINT ck_partner_documents_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'verified'::character varying, 'revision'::character varying])::text[])))
);

ALTER TABLE ONLY public.partner_documents
    ADD CONSTRAINT fk_partner_documents_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS ix_partner_documents_partner
    ON public.partner_documents (partner_id, uploaded_at DESC);
