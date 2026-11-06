package db

import "strings"

// Kesiapan integrasi media: satu definisi untuk katalog klien, pembuatan
// order, daftar media mitra, dan admin, supaya keempatnya tidak pernah
// berbeda pendapat soal media mana yang bisa dipesan.
//
// Media `manual` tidak butuh koneksi (mitra memasang sendiri). Tipe lain
// butuh koneksi aktif: kolom status untuk custom_web/blogger, adapter_url
// untuk tulisio_api, dan baris cms_connections (host tanpa www sama dengan
// host website_url) untuk wordpress/webhook.

// MediaIntegrationConnectedSQL: ekspresi boolean "koneksi aktif ada" untuk
// tabel media dengan alias tertentu. Manual selalu FALSE.
func MediaIntegrationConnectedSQL(alias string) string {
	return strings.ReplaceAll(`CASE
         WHEN M.integration_type = 'manual' THEN FALSE
         WHEN M.integration_type = 'custom_web' THEN COALESCE(M.custom_web_status, '') = 'connected'
         WHEN M.integration_type = 'blogger' THEN COALESCE(M.blogger_status, '') = 'connected'
         WHEN M.integration_type = 'tulisio_api' THEN COALESCE(NULLIF(TRIM(M.adapter_url), ''), '') <> ''
         ELSE EXISTS (
           SELECT 1 FROM cms_connections mic
           WHERE mic.partner_id = M.partner_id
             AND mic.provider = M.integration_type
             AND mic.status = 'active'
             AND M.website_url IS NOT NULL
             AND regexp_replace(lower(split_part(split_part(mic.base_url, '://', 2), '/', 1)), '^www\.', '')
               = regexp_replace(lower(split_part(split_part(M.website_url, '://', 2), '/', 1)), '^www\.', '')
         )
       END`, "M.", alias+".")
}

// MediaIntegrationReadySQL: media boleh dipesan dari sisi integrasi, yaitu
// manual, atau tipe lain yang koneksinya aktif.
func MediaIntegrationReadySQL(alias string) string {
	return "(" + alias + ".integration_type = 'manual' OR (" + MediaIntegrationConnectedSQL(alias) + "))"
}

// CMSProviderForIntegration: nilai cms_connections.provider yang dipakai untuk
// tipe integrasi media. Semua tipe di luar webhook dan emdash memakai koneksi
// WordPress.
func CMSProviderForIntegration(integrationType string) string {
	switch integrationType {
	case "webhook", "emdash":
		return integrationType
	}
	return "wordpress"
}

// MediaCatalogVisibleSQL: media tampil di katalog klien. Syaratnya sama
// dengan WHERE katalog clientread: aktif, tidak ditangguhkan, berharga,
// mitra aktif, tidak disembunyikan mitranya, dan siap dari sisi integrasi.
func MediaCatalogVisibleSQL(alias string) string {
	return "(" + alias + ".status = 'active' AND " + alias + ".suspended_at IS NULL AND COALESCE(" + alias + ".price, 0) > 0" +
		" AND NOT " + alias + ".partner_hidden" +
		" AND EXISTS (SELECT 1 FROM partners mcp WHERE mcp.id = " + alias + ".partner_id AND mcp.status = 'active')" +
		" AND " + MediaIntegrationReadySQL(alias) + ")"
}

// MediaHiddenByPartnerMessage: pesan 409 saat klien memesan media yang sedang
// disembunyikan mitranya (URL lama / API tetap dijaga, sama seperti
// MediaNotReadyMessage).
const MediaHiddenByPartnerMessage = "Media ini sedang disembunyikan mitranya dari katalog, jadi belum bisa dipesan. Silakan pilih media lain."

// MediaNotReadyMessage: pesan 409 saat klien memesan media yang belum
// tersambung (katalog menyembunyikannya, tapi URL lama / API tetap dijaga).
const MediaNotReadyMessage = "Media ini belum tersambung ke sistem publikasi, jadi belum bisa dipesan. Silakan pilih media lain."

// CMSConnectionForMediaSQL: koneksi aktif untuk snapshot order. Utamakan yang
// terikat website_id = media ($3); kalau koneksi disimpan tanpa website_id,
// cocokkan host base_url dengan host website_url media ($4), seperti worker.
// Argumen: $1 partner_id, $2 provider, $3 media_id, $4 website_url.
const CMSConnectionForMediaSQL = `SELECT c.id FROM cms_connections c
 WHERE c.partner_id = $1 AND c.provider = $2 AND c.status = 'active'
   AND (c.website_id = $3
        OR regexp_replace(lower(split_part(split_part(c.base_url, '://', 2), '/', 1)), '^www\.', '')
           = regexp_replace(lower(split_part(split_part($4::text, '://', 2), '/', 1)), '^www\.', ''))
 ORDER BY (c.website_id = $3) DESC NULLS LAST, c.created_at DESC LIMIT 1`
