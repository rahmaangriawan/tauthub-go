package platform

import (
	"context"
	"log/slog"
	"strings"
)

// Pemicu otomatis pemeriksaan DA/AI Trust dan trafik SimilarWeb untuk satu media.
//
// Sebelum ini tiap titik panggil memutuskan sendiri: media baru hanya memicu DA
// (trafik menunggu sweep), media yang website_url-nya diganti tidak memicu
// apa pun, dan pemulihan dari arsip tidak memicu apa pun. Akibatnya angka DA
// milik domain lama tetap tampil setelah domain diganti, dan kolom trafik
// kosong berhari-hari.
//
// Satu pintu di sini menyatukan keputusan tersebut sekaligus menambahkan satu
// pengaman yang sebelumnya tidak ada: JANGAN meng-enqueue job bila kredensial
// penyedianya belum lengkap. Tanpa pengaman itu, setiap siklus scheduler (60
// detik) membuat job yang pasti gagal — pernah tercatat 184 baris `failed`
// hanya karena kredensial provider trafik lama kosong, dan tiap percobaan
// tetap memakan kuota RapidAPI.

// mpCheckKind: jenis pemeriksaan yang bisa dipicu.
type mpCheckKind int

const (
	mpCheckSeo     mpCheckKind = 1 << iota // DA + PA + AI Trust (job media_seo_refresh)
	mpCheckTraffic                         // SimilarWeb (job media_similarweb_refresh)
)

const mpCheckAll = mpCheckSeo | mpCheckTraffic

// mpProviderReadiness: hasil pemeriksaan kelengkapan kredensial penyedia.
type mpProviderReadiness struct {
	Seo     bool
	Traffic bool
	// Reason berisi sebab pertama yang membuat pemeriksaan tidak bisa jalan,
	// dipakai untuk log dan untuk chip status di UI admin.
	SeoReason     string
	TrafficReason string
}

// mpCheckProviderReadiness: baca konfigurasi penyedia sekali, tanpa membuka
// secret (cukup tahu kolomnya terisi — membuka secret untuk sekadar cek
// keberadaan hanya menambah kerja dan risiko).
//
// Penting: "kolom terisi" BUKAN berarti "bisa dipakai". Penolakan provider
// DA/PA (dicatat mpRecordDAError di last_da_error) dicatat untuk UI, tetapi
// hanya berlaku untuk provider DA; provider trafik dinilai dari key-nya saja.
func (d *Deps) mpCheckProviderReadiness(ctx context.Context) mpProviderReadiness {
	var out mpProviderReadiness
	row, err := d.queryRow(ctx, `
		SELECT COALESCE(sealed_api_key, '')            AS seo_key,
		       COALESCE(sealed_similarweb_api_key, '') AS rapid_key
		FROM platform_seo_config WHERE id = 'default' LIMIT 1`)
	if err != nil || row == nil {
		out.SeoReason = "Konfigurasi SEO platform belum ada."
		out.TrafficReason = "Konfigurasi SEO platform belum ada."
		return out
	}

	// DA/PA memakai kolom sealed_api_key (RapidAPI Domain Metrics). Bila
	// penyedia DA sedang "dijeda" karena error berulang (kuota/langganan),
	// pemeriksaan tetap boleh jalan untuk sinyal AI, jadi tidak dianggap
	// blocker — tapi dicatat supaya UI bisa menjelaskan.
	out.Seo = strings.TrimSpace(row.strOr("seo_key", "")) != ""
	if !out.Seo {
		out.SeoReason = "RapidAPI SEO key belum diisi."
	}

	// Provider trafik (Similar Web API, belchiorarkad) cukup satu key RapidAPI.
	//
	// last_da_error SENGAJA tidak dibaca di sini: kolom itu hanya diisi
	// provider DA/PA (listing RapidAPI lain). Dulu penolakan provider DA ikut
	// menghentikan cek trafik, padahal dua provider itu langganannya terpisah.
	// Kegagalan trafik per media sudah diredam oleh cooldown 24 jam di sweep.
	if strings.TrimSpace(row.strOr("rapid_key", "")) == "" {
		out.TrafficReason = "Key RapidAPI SimilarWeb belum diisi."
	} else {
		out.Traffic = true
	}
	return out
}

// mpEnqueueMediaChecks: picu pemeriksaan otomatis untuk satu media.
//
// Guard yang berlaku untuk kedua jenis:
//   - media harus ada, berstatus 'active', dan punya website_url (job untuk
//     media tanpa domain pasti selesai tanpa hasil);
//   - tidak ada job sejenis yang masih pending/running (dedupe antrean);
//   - kredensial penyedia tersedia (lihat mpCheckProviderReadiness).
//
// reason hanya untuk log/audit supaya jejaknya jelas saat menelusuri kenapa
// sebuah media masuk antrean.
func (d *Deps) mpEnqueueMediaChecks(ctx context.Context, mediaID string, kinds mpCheckKind, reason string) {
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" || kinds == 0 {
		return
	}

	var websiteURL, status string
	err := d.Pool.QueryRow(ctx,
		"SELECT COALESCE(website_url, ''), COALESCE(status, '') FROM media WHERE id = $1",
		mediaID).Scan(&websiteURL, &status)
	if err != nil || strings.TrimSpace(websiteURL) == "" || status != "active" {
		return
	}

	readiness := d.mpCheckProviderReadiness(ctx)

	if kinds&mpCheckSeo != 0 {
		if !readiness.Seo {
			slog.Warn("[media] cek DA dilewati: penyedia belum siap",
				"media", mediaID, "reason", reason, "sebab", readiness.SeoReason)
		} else if !d.mpHasActiveJob(ctx, "media_seo_refresh", mediaID) {
			d.mpEnqueueJobBestEffort(ctx, "media_seo_refresh", map[string]any{"mediaId": mediaID}, 2)
		}
	}

	if kinds&mpCheckTraffic != 0 {
		if !readiness.Traffic {
			slog.Warn("[media] cek trafik dilewati: penyedia belum siap",
				"media", mediaID, "reason", reason, "sebab", readiness.TrafficReason)
		} else if !d.mpHasActiveJob(ctx, "media_similarweb_refresh", mediaID) {
			d.mpEnqueueJobBestEffort(ctx, "media_similarweb_refresh", map[string]any{"mediaId": mediaID}, 2)
		}
	}
}

// mpHasActiveJob: true bila sudah ada job sejenis yang pending/running untuk
// media ini. Dipakai supaya satu media tidak masuk antrean berkali-kali ketika
// beberapa pemicu (create, approve, edit) berjalan berdekatan.
func (d *Deps) mpHasActiveJob(ctx context.Context, kind, mediaID string) bool {
	var exists bool
	err := d.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM jobs
			WHERE kind = $1 AND status IN ('pending','running')
			  AND payload->>'mediaId' = $2
		)`, kind, mediaID).Scan(&exists)
	return err == nil && exists
}

// mpArchiveSeoMetricsForDomainChange: pindahkan metrik lama ke riwayat ketika
// domain sebuah media diganti.
//
// Tanpa ini, media_seo_metrics (satu baris per media) tetap menyimpan DA/PA/AI
// Trust milik domain LAMA dan menyajikannya seolah milik domain baru — sekaligus
// menahan pemeriksaan otomatis karena baris dianggap sudah berskor. Riwayatnya
// tidak dibuang: disalin ke media_seo_metric_history lebih dulu.
func (d *Deps) mpArchiveSeoMetricsForDomainChange(ctx context.Context, mediaID, oldDomain, newDomain string) {
	if strings.TrimSpace(mediaID) == "" {
		return
	}
	if strings.EqualFold(strings.TrimSpace(oldDomain), strings.TrimSpace(newDomain)) {
		return
	}
	// Hanya bertindak bila ada baris yang benar-benar perlu diarsipkan.
	if err := d.exec(ctx, `
		INSERT INTO media_seo_metric_history
		  (media_id, domain, da, pa, spam_score, ai_trust_score, ai_citation,
		   ai_citation_checked_at, source, checked_at, reason)
		SELECT media_id, COALESCE(NULLIF(domain, ''), $2), da, pa, spam_score,
		       ai_trust_score, ai_citation, ai_citation_checked_at, source,
		       checked_at, 'domain_changed'
		FROM media_seo_metrics WHERE media_id = $1`, mediaID, strings.TrimSpace(oldDomain)); err != nil {
		// Gagal mengarsipkan tidak boleh menghalangi perubahan domain: lebih baik
		// kehilangan riwayat satu baris daripada menampilkan DA domain lama.
		d.logWarn("arsip metrik SEO saat ganti domain gagal", mediaID, err)
		return
	}
	if err := d.exec(ctx, "DELETE FROM media_seo_metrics WHERE media_id = $1", mediaID); err != nil {
		d.logWarn("hapus metrik SEO lama saat ganti domain gagal", mediaID, err)
	}
}

// mpResetTrafficForDomainChange: kosongkan angka trafik lama saat domain
// berubah, supaya kolom trafik tidak menampilkan kunjungan domain lama sambil
// menunggu pemeriksaan baru. Snapshot lama TIDAK dihapus — riwayat trafik
// tersimpan di media_similarweb_snapshots dan tetap bisa dibaca.
func (d *Deps) mpResetTrafficForDomainChange(ctx context.Context, mediaID string) {
	if err := d.exec(ctx,
		"UPDATE media SET traffic_estimate = 0, updated_at = NOW() WHERE id = $1", mediaID); err != nil {
		d.logWarn("reset trafik saat ganti domain gagal", mediaID, err)
	}
}

// mpSimilarWebReady: dipakai endpoint status supaya UI bisa membedakan
// "tidak ada yang perlu dicek" dari "pemeriksaan berhenti karena kredensial
// kurang".
func (d *Deps) mpSimilarWebReady(ctx context.Context) (bool, string) {
	r := d.mpCheckProviderReadiness(ctx)
	return r.Traffic, r.TrafficReason
}
