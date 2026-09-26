-- Dashboard hot-path indexes (read-only optimization; no data changes).
--
-- Queries yang dipercepat:
--   * audit_logs brand_id + created_at DESC LIMIT 8  (dipanggil tiap /dashboard
--     dan /overview — sebelumnya full scan karena hanya ada PK).
--   * content_jobs brand_id + status IN (...) — filter status sekarang pakai
--     index range scan, bukan filter setelah index scan.
--   * content_documents brand_id + created_at (count artikel bulanan).
--   * publications brand_id + status + published_at (jendela laporan).
--
-- Semua index idempoten (IF NOT EXISTS) dan murni menambah struktur — tidak
-- menghapus/mengubah data apa pun.

CREATE INDEX IF NOT EXISTS ix_audit_brand_created
  ON public.audit_logs (brand_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_content_jobs_brand_status
  ON public.content_jobs (brand_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_content_documents_brand_created
  ON public.content_documents (brand_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_publication_brand_status_published_at
  ON public.publications (brand_id, status, published_at DESC);
