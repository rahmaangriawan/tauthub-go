-- 076 — Media Dewan Pers verification workflow (per media/domain)
--
-- Mengubah verifikasi Dewan Pers dari boolean yang ditulis mitra sendiri
-- menjadi workflow review: mitra mengajukan nomor sertifikat (claim) ->
-- pending -> Admin approve/reject -> verified/rejected/conflict.
--
-- Prinsip:
--   MITRA = mengajukan claim
--   ADMIN = melakukan verification
--   MEDIA/DOMAIN = pemilik/scope sertifikat
--
-- Additive & idempotent: CREATE TABLE IF NOT EXISTS + index IF NOT EXISTS.
-- Kolom lama media.dewan_pers_* TIDAK disentuh (tetap dipakai sebagai hasil
-- akhir yang di-sync dari tabel ini, agar tampilan existing tetap jalan).

CREATE TABLE IF NOT EXISTS public.media_dewan_pers_verifications (
  id                character(36) NOT NULL PRIMARY KEY,
  media_id          character(36) NOT NULL,
  partner_id        character(36) NOT NULL,           -- pemilik saat submit (boundary ownership)
  certificate_no    character varying(200) NOT NULL,
  status            character varying(20) NOT NULL DEFAULT 'pending'
                    CONSTRAINT ck_mdpv_status CHECK (status IN ('pending','verified','rejected','conflict','superseded')),
  rejection_reason  text,                              -- wajib saat rejected
  conflict_with     character(36),                     -- media_id lain yang mengklaim nomor sama (opsional)
  submitted_at      timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  reviewed_by       character(36),
  reviewed_at       timestamp(6) with time zone,
  created_at        timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at        timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_mdpv_media   FOREIGN KEY (media_id)   REFERENCES public.media(id)   ON DELETE CASCADE,
  CONSTRAINT fk_mdpv_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS ix_mdpv_media_status
  ON public.media_dewan_pers_verifications USING btree (media_id, status);
CREATE INDEX IF NOT EXISTS ix_mdpv_cert_no
  ON public.media_dewan_pers_verifications USING btree (certificate_no, status);
CREATE INDEX IF NOT EXISTS ix_mdpv_partner
  ON public.media_dewan_pers_verifications USING btree (partner_id, submitted_at DESC);
