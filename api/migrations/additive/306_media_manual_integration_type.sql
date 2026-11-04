-- 306 — Media mode Manual (publication_mode='draft') tidak punya kanal
-- integrasi. Form mitra lama selalu mengirim integration_type bawaan
-- 'wordpress' walau mitra memilih Manual, sehingga di admin/klien media
-- Blogspot/lainnya terbaca "wordpress". Rapikan: media Manual yang tidak
-- punya bukti integrasi nyata (koneksi WordPress aktif, blog Blogger
-- terpilih, endpoint Custom Web) diset 'manual'. Additive, idempoten.

BEGIN;

UPDATE public.media m
   SET integration_type = 'manual'
 WHERE m.publication_mode = 'draft'
   AND (
        (m.integration_type = 'wordpress'
         AND NOT EXISTS (SELECT 1 FROM public.cms_connections c WHERE c.website_id = m.id AND c.status = 'active'))
     OR (m.integration_type = 'blogger' AND m.blogger_blog_id IS NULL)
     OR (m.integration_type = 'custom_web' AND COALESCE(NULLIF(TRIM(m.custom_web_endpoint_url), ''), '') = '')
   );

COMMIT;
