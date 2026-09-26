-- 110 -- Restore 3 media pending_review yang terhapus (Elektropedia, Purworejo Media, sutoro).
-- Data diambil dari backup VPS 2026-08-31 02:00 (ai_writer_20260831_0200.sql).
-- Idempoten: hanya insert bila media belum ada & partner pemilik masih ada.

BEGIN;

INSERT INTO public.media (id, name, slug, category, description, website_url, price, credit_cost, status, integration_type, partner_id, publish_defaults, notes, created_by, created_at, updated_at, traffic_estimate, health_score, health_breakdown, health_computed_at, adapter_url, media_tier, media_quality_score, media_quality_notes, dewan_pers_verified, dewan_pers_note, dewan_pers_cert_no, dewan_pers_doc_key, publication_mode, rejection_reason, reviewed_by, reviewed_at, pricing_type, price_status, negotiated_price, custom_web_endpoint_url, custom_web_api_key_sealed, custom_web_status, custom_web_last_test_at, custom_web_last_test_message, blogger_credentials_sealed, blogger_blog_id, blogger_blog_name, blogger_status, blogger_last_test_at, blogger_last_test_message, media_type, allow_hyperlink)
SELECT 'b8773253-1dcc-45c1-87f5-162a142c0a15','Elektropedia','elektropedia','teknologi',NULL,'https://www.elektro.web.id','100000.00','115','pending_review','blogger','f0beca5b-d8a6-4571-b7f7-0eda320c12a9',NULL,NULL,'4d9e4f64-12fe-48b1-b653-552d9aa17745','2026-08-30 17:01:05.539412+07','2026-08-30 17:01:05.539412+07','100000',NULL,NULL,NULL,NULL,'tier3',NULL,NULL,'f',NULL,NULL,NULL,'draft',NULL,NULL,NULL,'nego','menunggu_review',NULL,NULL,NULL,'disconnected',NULL,NULL,NULL,NULL,NULL,'disconnected',NULL,NULL,'blogger','f'
WHERE EXISTS (SELECT 1 FROM public.partners WHERE id = 'f0beca5b-d8a6-4571-b7f7-0eda320c12a9')
  AND NOT EXISTS (SELECT 1 FROM public.media WHERE id = 'b8773253-1dcc-45c1-87f5-162a142c0a15')
ON CONFLICT (id) DO NOTHING;

INSERT INTO public.media (id, name, slug, category, description, website_url, price, credit_cost, status, integration_type, partner_id, publish_defaults, notes, created_by, created_at, updated_at, traffic_estimate, health_score, health_breakdown, health_computed_at, adapter_url, media_tier, media_quality_score, media_quality_notes, dewan_pers_verified, dewan_pers_note, dewan_pers_cert_no, dewan_pers_doc_key, publication_mode, rejection_reason, reviewed_by, reviewed_at, pricing_type, price_status, negotiated_price, custom_web_endpoint_url, custom_web_api_key_sealed, custom_web_status, custom_web_last_test_at, custom_web_last_test_message, blogger_credentials_sealed, blogger_blog_id, blogger_blog_name, blogger_status, blogger_last_test_at, blogger_last_test_message, media_type, allow_hyperlink)
SELECT '827ce0d0-ca3c-4f64-9514-becb84cf334f','Purworejo Media','purworejo-media','teknologi',NULL,'https://www.purworejomedia.com','100000.00','115','pending_review','blogger','f0beca5b-d8a6-4571-b7f7-0eda320c12a9',NULL,NULL,'4d9e4f64-12fe-48b1-b653-552d9aa17745','2026-08-30 17:00:14.561023+07','2026-08-30 17:00:14.561023+07','500',NULL,NULL,NULL,NULL,'tier3',NULL,NULL,'f',NULL,NULL,NULL,'draft',NULL,NULL,NULL,'nego','menunggu_review',NULL,NULL,NULL,'disconnected',NULL,NULL,NULL,NULL,NULL,'disconnected',NULL,NULL,'blogger','f'
WHERE EXISTS (SELECT 1 FROM public.partners WHERE id = 'f0beca5b-d8a6-4571-b7f7-0eda320c12a9')
  AND NOT EXISTS (SELECT 1 FROM public.media WHERE id = '827ce0d0-ca3c-4f64-9514-becb84cf334f')
ON CONFLICT (id) DO NOTHING;

INSERT INTO public.media (id, name, slug, category, description, website_url, price, credit_cost, status, integration_type, partner_id, publish_defaults, notes, created_by, created_at, updated_at, traffic_estimate, health_score, health_breakdown, health_computed_at, adapter_url, media_tier, media_quality_score, media_quality_notes, dewan_pers_verified, dewan_pers_note, dewan_pers_cert_no, dewan_pers_doc_key, publication_mode, rejection_reason, reviewed_by, reviewed_at, pricing_type, price_status, negotiated_price, custom_web_endpoint_url, custom_web_api_key_sealed, custom_web_status, custom_web_last_test_at, custom_web_last_test_message, blogger_credentials_sealed, blogger_blog_id, blogger_blog_name, blogger_status, blogger_last_test_at, blogger_last_test_message, media_type, allow_hyperlink)
SELECT 'b65f2dfe-c80e-4799-915d-9d04cadeede6','sutoro','sutoro','Umum',NULL,'https://sutoro.web.id','50000.00','58','pending_review','wordpress','95e53690-1feb-4446-ad77-4695057f298c',NULL,NULL,'6f4fecfc-f68d-4b6c-8476-8a329288fa3c','2026-08-25 19:28:45.911111+07','2026-08-25 19:28:45.911111+07','0',NULL,NULL,NULL,NULL,'tier3',NULL,NULL,'f',NULL,NULL,NULL,'publish',NULL,NULL,NULL,'nett','menunggu_review',NULL,NULL,NULL,'disconnected',NULL,NULL,NULL,NULL,NULL,'disconnected',NULL,NULL,'blogger','t'
WHERE EXISTS (SELECT 1 FROM public.partners WHERE id = '95e53690-1feb-4446-ad77-4695057f298c')
  AND NOT EXISTS (SELECT 1 FROM public.media WHERE id = 'b65f2dfe-c80e-4799-915d-9d04cadeede6')
ON CONFLICT (id) DO NOTHING;

COMMIT;
