package brandflow

import (
	"context"
	"log/slog"

	dbpkg "github.com/rahmaangriawan/tauthub-go/internal/db"
)

// Media non-manual tanpa koneksi aktif disembunyikan dari katalog klien
// (db.MediaIntegrationReadySQL). Mitranya diberi tahu sekali per media lewat
// notifikasi in-app, dengan tautan langsung ke halaman Integrasi. Dedupe
// lewat dedupe_key supaya scheduler yang berjalan tiap menit tidak mengulang.

const mediaIntegrationMissingDedupe = "media.integration_missing:"

// schSweepMediaIntegrationMissing: mengembalikan jumlah notifikasi terkirim.
func (d *Deps) schSweepMediaIntegrationMissing(ctx context.Context, limit int) int {
	if limit < 1 {
		return 0
	}
	rows, err := d.queryRows(ctx, `
		SELECT m.id, m.name, m.website_url, m.integration_type,
		       prt.user_id AS partner_user_id, prt.public_id AS partner_public_id
		FROM media m
		JOIN partners prt ON prt.id = m.partner_id
		WHERE m.status = 'active' AND m.suspended_at IS NULL
		  AND m.integration_type <> 'manual'
		  AND prt.user_id IS NOT NULL
		  AND NOT `+dbpkg.MediaIntegrationReadySQL("m")+`
		  AND NOT EXISTS (
		    SELECT 1 FROM notifications n
		    WHERE n.user_id = prt.user_id AND n.dedupe_key = $1 || m.id::text
		  )
		ORDER BY m.created_at
		LIMIT $2`, mediaIntegrationMissingDedupe, limit)
	if err != nil {
		slog.Error("[scheduler] sweep media belum tersambung gagal", "err", err)
		return 0
	}

	sent := 0
	for _, row := range rows {
		mediaID := row.strOr("id", "")
		userID := row.strOr("partner_user_id", "")
		if mediaID == "" || userID == "" {
			continue
		}
		label := mediaLabelOf(row.strOr("name", ""), row.strOr("website_url", ""))
		platform, anchor := mediaIntegrationPlatform(row.strOr("integration_type", ""))
		link := "/mitra/integrations#" + anchor
		if pid := row.strOr("partner_public_id", ""); pid != "" {
			link = "/mitra/" + pid + "/integrations#" + anchor
		}
		title := "Media disembunyikan dari katalog: " + label
		body := label + " belum tersambung ke " + platform + ", jadi untuk sementara tidak tampil di katalog klien dan tidak bisa dipesan. " +
			"Hubungkan situs Anda di menu Integrasi; begitu tersambung, media otomatis tampil lagi."
		if err := d.exec(ctx, `
			INSERT INTO notifications (id, user_id, type, title, body, link, dedupe_key)
			VALUES (gen_random_uuid(), $1, 'media.integration_missing', $2, $3, $4, $5)`,
			userID, title, body, link, mediaIntegrationMissingDedupe+mediaID); err != nil {
			slog.Warn("[scheduler] notifikasi media belum tersambung gagal", "media", mediaID, "err", err)
			continue
		}
		sent++
	}
	return sent
}

// mediaIntegrationPlatform: nama platform untuk teks notifikasi dan anchor tab
// di halaman Integrasi mitra.
func mediaIntegrationPlatform(integrationType string) (name, anchor string) {
	switch integrationType {
	case "blogger":
		return "Blogspot", "blogger"
	case "custom_web":
		return "Custom Web", "custom"
	case "webhook":
		return "webhook", "wordpress"
	case "tulisio_api":
		return "Tulisio API", "custom"
	default:
		return "WordPress", "wordpress"
	}
}
