# 03 — Database: `tauthub-go` (PostgreSQL 16)

> **Aturan utama: schema TIDAK dirombak.** Backend Go mengonsumsi database ini apa adanya.
> DB ini sudah terpisah dari DB lama `ai_writer` dan berisi dump produksi per 2026-09-26.
> Baseline repo baru = `schema.pg.sql` (109 tabel dasar) + **172 file migration** `apps/web/src/db/migrations/` (001–168, additive-only). Hasil akhirnya di DB live: **159 tabel** (+ `schema_migrations` sebagai buku besar migration).

---

## 1. Inventaris Tabel per Domain (159)

### 1.1 Identity, Auth & Audit — 10
`users` · `user_sessions` · `user_oauth_accounts` · `password_reset_tokens` · `platform_admins` · `registration_guard_config` · `registration_history` · `auth_rate_limits` · `audit_logs` · `bug_reports`

### 1.2 Brands & Workspace — 12
`brands` · `brand_memberships` · `brand_profiles` · `brand_ai_quotas` · `brand_pr_standards` · `voice_profiles` · `editorial_rule_sets` · `master_prompt_versions` · `websites` · `content_type_standards` · `content_type_standard_versions` · `content_type_standard_chunks`

### 1.3 Knowledge & RAG — 11
`knowledge_collections` · `knowledge_sources` · `knowledge_chunks` · `knowledge_crawl_runs` · `knowledge_bases` · `knowledge_base_rules` · `knowledge_base_rule_chunks` · `knowledge_base_sources` · `knowledge_base_versions` · `ai_model_knowledge` · `ai_model_knowledge_chunks`

### 1.4 Crawler — 5
`crawl_targets` · `crawler_credentials` · `crawler_diagnostics` · `crawler_nonces` · `crawler_request_log`

### 1.5 Content Pipeline — 14
`content_briefs` · `content_intakes` · `content_intake_types` · `content_jobs` · `content_documents` · `content_versions` · `content_batches` · `content_batch_items` · `review_comments` · `quality_evaluations` · `content_ai_analyses` · `content_moderation_audits` · `content_moderation_results` · `chat_learnings`

### 1.6 Media — 12
`media` · `media_categories` · `media_category_links` · `media_packages` · `media_package_items` · `media_price_history` · `media_content_standards` · `media_seo_metrics` · `media_similarweb_snapshots` · `media_ai_summary_events` · `media_dewan_pers_verifications` · `auto_replace_configs`

### 1.7 Publications & Publishing — 13
`publications` · `publication_jobs` · `publication_attempts` · `publication_variants` · `publication_variant_batches` · `publication_plans` · `publication_plan_media` · `publication_plan_occurrences` · `publication_link_checks` · `cms_connections` · `publishing_api_keys` · `publishing_callback_logs` · `blogger_oauth_states`

### 1.8 Campaigns — 3
`campaigns` · `campaign_media_items` · `campaign_report_shares`

### 1.9 Credits & Finance — 11
`credit_wallets` · `credit_ledger_entries` · `credit_transactions` · `credit_topup_reports` · `tc_config` · `invoices` · `transactions` · `settlements` · `revenue_shares` · `packages` · `pricing_config`

### 1.10 Payments — 3
`payment_gateway_configs` · `payment_transactions` · `payment_webhook_events`

### 1.11 Partners — 7
`partners` · `partner_documents` · `partner_earnings` · `partner_referral_commissions` · `partner_seo_check_history` · `partner_service_charges` · `partner_speed_audits`

### 1.12 Referral & Affiliate — 5
`referral_attributions` · `referral_clicks` · `referral_settings` · `referred_domains` · `affiliate_settings`

### 1.13 PR Intelligence — 8
`pr_config` · `pr_media_rules` · `pr_scoring_profiles` · `pr_campaign_scores` · `pr_message_analyses` · `pr_sentiment_analyses` · `pr_metric_results` · `pr_publication_verifications`

### 1.14 Tracking & Analytics — 3
`tracking_links` · `tracking_events` · `analytics_daily`

### 1.15 SEO / GSC / GA4 — 7
`ga4_configs` · `gsc_connections` · `gsc_oauth_states` · `gsc_properties` · `gsc_snapshots` · `platform_gsc_config` · `platform_seo_config`

### 1.16 Platform Config & Settings — 9
`platform_ai_configuration` · `platform_crawlhub_config` · `platform_oauth_configs` · `platform_pagespeed_config` · `platform_server_config` · `platform_settings` · `platform_turnstile_config` · `platform_wa_config` · `smtp_settings`

### 1.17 Taut Pintar — 13
`taut_agent_configs` · `taut_agent_runs` · `taut_campaign_files` · `taut_campaign_media` · `taut_content_angles` · `taut_content_angle_prompt_versions` · `taut_gatekeeper_audits` · `taut_media_channels` · `taut_profiler_base_prompt_versions` · `taut_profiler_results` · `taut_profiler_submission_channels` · `taut_profiler_submissions` · `taut_training_samples`

### 1.18 Jobs & Infra — 5
`jobs` · `model_usage_events` · `user_daily_quotas` · `system_metadata` · `schema_migrations`

### 1.19 Notifications & Comms — 6
`notifications` · `notification_preferences` · `notification_webhooks` · `email_deliveries` · `wa_deliveries` · `wa_templates`

### 1.20 RSS — 2
`rss_feeds` · `rss_access_logs`

**Total: 159 tabel** (10+12+11+5+14+12+13+3+11+3+7+5+8+3+7+9+13+5+6+2)

---

## 2. Pola Akses Kritis (wajib dipertahankan saat port)

| Pola | Di mana | Catatan porting |
|---|---|---|
| `FOR UPDATE SKIP LOCKED` | claim job (`jobs`), materialisasi `publication_plan_occurrences` | jantung concurrency; salin SQL persis |
| `ON CONFLICT ... DO UPDATE` | upsert metrics, auth_rate_limits, dedupe | banyak; sqlc harus enable `:upsert` biasa |
| `payload->>'publicationId'` | dedupe job scheduler | index khusus dibuat migration 117 — jangan hilang |
| `DISTINCT ON` | dedup kueri laporan/media | valid di sqlc |
| `RETURNING id/...` | insert di semua CRUD | sumber `insertId` shim |
| `INTERVAL '30 days'` interpolasi string | email dedupe window | R11: salin persis, uji window |
| JSONB (`payload`, `validation`, `entitlements`, …) | snapshot job, publish defaults, konfig | Go: `[]byte` → `json.Unmarshal` ke struct eksplisit |
| Tenancy scoping `brand_id` / `partner_id` | hampir semua tabel domain | lintas tenant → 404 |
| Soft delete (`deleted_at`) | brands, campaigns, knowledge_sources | filter wajib ikut tersalin |
| Kolom `enc_key_ver` | cms_connections, media, payment gateways | menandakan sealed secret v3 (lihat `01 §8`) |
| `public_id` (short id, fungsi `_random_short_id()`) | brands, campaigns | function DB tetap ada; jangan generate di aplikasi |

## 3. Konvensi Data

- ID: `character(36)` (UUID string) di sebagian besar tabel; beberapa tabel crawler pakai `uuid` native + `gen_random_uuid()`. sqlc: gunakan `string`/`pgtype.UUID` sesuai kolom — cek per tabel.
- Timestamp: `timestamptz` semua. Uang: `numeric(14,2)`. Skor: `smallint`/`integer` dengan CHECK constraint — **CHECK constraint jangan di-drop** saat port (documented di schema).
- Enum dilakukan via CHECK constraint (bukan enum type) — salin nilai validnya ke validasi Go juga.

## 4. Strategi Migration di Repo Baru

1. `api/migrations/` berisi **salinan utuh** `schema.pg.sql` + 172 migration SQL lama (baseline historis).
2. Untuk DB `tauthub-go` yang sudah live: repo baru TIDAK menjalankan baseline — hanya mencatat versi terakhir (`168`) ke `schema_migrations`.
3. Migration baru: **additive-only** (CREATE TABLE/ADD COLUMN/CREATE INDEX), satu file satu transaksi, tercatat di `schema_migrations`. Runner Go meniru `migrate-additive.mjs`: toleran error duplicate-object (`42P07, 42701, 42710, 42704, 23505`) dan tetap mencatatnya applied.
4. Dilarang: `DROP COLUMN/TABLE`, rename kolom, ubah tipe kolom eksisting. Kebutuhan baru = kolom/tabel baru.
5. DB dev lokal: database `tauthub-go` di Postgres.app (lihat `.env` root repo lama: host 127.0.0.1, user `ai_writer`).

## 5. Aturan Kompatibilitas Data (jangan dilanggar)

- [ ] Jangan rename/drop kolom atau tabel mana pun (app lama masih hidup saat parallel run).
- [ ] Jangan menulis nilai `seal` format lain selain v3 (`t3:`) — app lama harus tetap bisa unseal.
- [ ] Jangan mengubah semantik status/existing CHECK constraint (mis. `publications.status`, `campaign_media_items.status`, `media.status`).
- [ ] Jangan menulis `public_id` dari aplikasi — pakai default DB.
- [ ] `password_hash`, `secret` ter-seal, TOTP secret: baca-saja dari sudut pandang schema (hanya ditulis lewat alur auth/kredensial yang sama).

## 6. Operasional

- Backup: port `scripts/production/backup/backup-hourly.sh` (pg_dump) ke systemd timer di VPS baru.
- Diagnostik: `diagnostics.yml` workflow lama ssh+psql → port jadi skrip `make diag` di repo baru (deploy sudah tidak pakai GitHub Actions deploy.yml — file itu sudah dihapus).
- Seed dev: port `seed-dev.mjs` (admin `wantek@gmail.com`, argon2 params sama) → `api/cmd/seed` atau skrip Go.
