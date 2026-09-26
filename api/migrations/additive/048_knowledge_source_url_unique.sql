-- Anti-duplicate knowledge sources: one ACTIVE URL per collection.
--
-- The legacy constraint `uq_knowledge_source_collection_filename`
-- (collection_id, original_filename) blocks re-importing a URL after its
-- soft-delete (deleted_at IS NOT NULL) — the tombstone still holds the key.
-- Replace it with a partial unique index that only applies to live rows, so:
--   * concurrent crawl retries cannot import the same page twice, and
--   * a soft-deleted source can be re-imported (restore) without a conflict.
ALTER TABLE ONLY public.knowledge_sources
  DROP CONSTRAINT IF EXISTS uq_knowledge_source_collection_filename;

CREATE UNIQUE INDEX IF NOT EXISTS uq_knowledge_source_url
  ON public.knowledge_sources (collection_id, source_type, original_filename)
  WHERE source_type = 'url' AND original_filename IS NOT NULL AND deleted_at IS NULL;
