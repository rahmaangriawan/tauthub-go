package partner

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	dbpkg "github.com/rahmaangriawan/tauthub-go/internal/db"
)

// seller_center.go: GET /v1/partner/seller-center, satu respons untuk halaman
// Pusat Penjual: daftar tugas berprioritas, pipeline order, penghasilan, dan
// kartu kesehatan tiap media.
//
// Semua angka dihitung di server dari data yang sudah ada (publications,
// media, partner_earnings, publication_link_checks, link_guardian_claims) dan
// memakai definisi yang sama dengan halaman lain: alasan media tersembunyi
// dari partnerCatalogHiddenReason, tenggat SLA 3x24 jam dari scheduler, dan
// batas penghasilan dari partnerFinanceSummary. Yang belum ada datanya
// ditampilkan kosong ("Belum ada riwayat"), bukan 0 atau perkiraan.

// sellerWIB: zona waktu tampilan harian (tanpa bergantung pada tzdata server).
var sellerWIB = time.FixedZone("WIB", 7*3600)

const sellerSLAHours = 72

type sellerTodo struct {
	Kind     string  `json:"kind"`
	Severity string  `json:"severity"` // urgent | warning | info
	Title    string  `json:"title"`
	Detail   string  `json:"detail"`
	Route    string  `json:"route"` // bagian setelah /mitra/{partnerId}/
	Query    string  `json:"query,omitempty"`
	Deadline *string `json:"deadline,omitempty"`
	MediaID  string  `json:"media_id,omitempty"`
}

// "waiting" = bukan tugas mitra, hanya menunggu pihak lain (mis. tinjauan tim Tauthub); selalu terakhir.
var sellerSeverityRank = map[string]int{"urgent": 0, "warning": 1, "info": 2, "waiting": 3}

// sellerAvgPublishHours: rata-rata jam tayang dari health_breakdown (jsonb).
func sellerAvgPublishHours(raw any) (float64, bool) {
	var m map[string]any
	switch v := raw.(type) {
	case map[string]any:
		m = v
	case string:
		_ = json.Unmarshal([]byte(v), &m)
	case []byte:
		_ = json.Unmarshal(v, &m)
	}
	if m == nil {
		return 0, false
	}
	f, ok := m["avg_publish_hours"].(float64)
	return f, ok
}

// sellerRemaining: sisa waktu dalam kalimat awam.
func sellerRemaining(d time.Duration) string {
	if d <= 0 {
		return "sudah lewat"
	}
	if d < time.Hour {
		return itoa(int(d.Minutes())+1) + " menit lagi"
	}
	if d < 48*time.Hour {
		h := int(d.Hours())
		return itoa(h) + " jam lagi"
	}
	return itoa(int(d.Hours()/24)) + " hari lagi"
}

type sellerInactiveText struct{ suffix, detail, severity, query string }

// sellerInactiveReason: kalimat untuk media yang belum aktif di katalog. Slug
// status mesin (pending_review, archived) tidak pernah sampai ke layar mitra.
func sellerInactiveReason(status string) sellerInactiveText {
	switch status {
	case "pending_review", "pending", "menunggu_review":
		return sellerInactiveText{"menunggu tinjauan tim Tauthub", "Media baru ditinjau tim Tauthub dulu sebelum tampil di katalog.", "waiting", "status=pending_review"}
	case "archived":
		return sellerInactiveText{"diarsipkan", "Media yang diarsipkan tidak tampil di katalog.", "info", "status=archived"}
	case "rejected":
		return sellerInactiveText{"ditolak tim Tauthub", "Perbaiki data media sesuai catatan review, lalu ajukan lagi.", "warning", "status=rejected"}
	}
	return sellerInactiveText{"belum aktif di katalog", "Media ini belum aktif, jadi klien belum bisa memesan.", "info", ""}
}

func (d *Deps) SellerCenter(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	partnerID := partner.ID
	now := time.Now()

	// ---- media ----
	mediaRows, err := d.queryRows(ctx,
		mediaListSelect+" WHERE m.partner_id = $1 AND m.status <> 'hidden' ORDER BY m.created_at DESC LIMIT 200", partnerID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	// Kolom yang tidak ada di mediaListSelect.
	healthRows, _ := d.queryRows(ctx, "SELECT id, domain_health FROM media WHERE partner_id = $1", partnerID)
	domainHealth := make(map[string]string, len(healthRows))
	for _, h := range healthRows {
		domainHealth[h.strOr("id", "")] = h.strOr("domain_health", "")
	}

	// Minat klien: berapa kali media dipilih ke kampanye dalam 30 hari terakhir,
	// oleh berapa brand, dan berapa yang lanjut jadi pesanan atau dibatalkan.
	// Hanya hitungan: nama brand tidak pernah dikirim ke mitra. Bookmark dan
	// jumlah tayang di katalog tidak tercatat di server, jadi tidak ditampilkan.
	interestRows, _ := d.queryRows(ctx, `
		SELECT cmi.media_id,
		       COUNT(*) FILTER (WHERE cmi.created_at > NOW() - INTERVAL '30 days')::int AS picked,
		       COUNT(DISTINCT c.brand_id) FILTER (WHERE cmi.created_at > NOW() - INTERVAL '30 days')::int AS brands,
		       COUNT(*) FILTER (WHERE cmi.created_at > NOW() - INTERVAL '30 days'
		                          AND EXISTS (SELECT 1 FROM publications p
		                                      WHERE p.campaign_id = cmi.campaign_id AND p.media_id = cmi.media_id
		                                        AND p.status <> 'cancelled'))::int AS ordered,
		       COUNT(*) FILTER (WHERE cmi.created_at > NOW() - INTERVAL '30 days' AND cmi.status = 'cancelled')::int AS cancelled,
		       COUNT(*)::int AS picked_total
		FROM campaign_media_items cmi
		JOIN campaigns c ON c.id = cmi.campaign_id
		JOIN media m ON m.id = cmi.media_id
		WHERE m.partner_id = $1
		GROUP BY cmi.media_id`, partnerID)
	interestByMedia := make(map[string]map[string]any, len(interestRows))
	for _, ir := range interestRows {
		interestByMedia[ir.strOr("media_id", "")] = map[string]any{
			"picked": ir.intOr("picked", 0), "brands": ir.intOr("brands", 0),
			"ordered": ir.intOr("ordered", 0), "cancelled": ir.intOr("cancelled", 0),
			"picked_total": ir.intOr("picked_total", 0),
		}
	}

	media := make([]map[string]any, 0, len(mediaRows))
	todos := make([]sellerTodo, 0, 16)
	visible, hidden, unverified, noDewanPers := 0, 0, 0, 0
	var hoursSum float64
	hoursN := 0

	for _, row := range mediaRows {
		id := row.strOr("id", "")
		name := row.strOr("name", "Media")
		reason := partnerCatalogHiddenReason(row)
		reasonStr, _ := reason.(string)
		isVisible := row.boolOr("catalog_visible", false)
		if isVisible {
			visible++
		} else {
			hidden++
		}
		active := row.strOr("status", "") == "active"
		verified := row.boolOr("domain_verified", false)
		if active && !verified {
			unverified++
		}
		if active && !row.boolOr("dewan_pers_verified", false) {
			noDewanPers++
		}
		totalTerbit := row.intOr("total_terbit", 0)
		hours, hasHours := sellerAvgPublishHours(row.raw("health_breakdown"))
		if hasHours && totalTerbit > 0 {
			hoursSum += hours
			hoursN++
		}
		crawlStatus := row.strOr("last_status", "")

		interest := interestByMedia[id]
		if interest == nil {
			interest = map[string]any{"picked": 0, "brands": 0, "ordered": 0, "cancelled": 0, "picked_total": 0}
		}
		card := map[string]any{
			"id":                    id,
			"name":                  name,
			"website_url":           row.raw("website_url"),
			"status":                row.raw("status"),
			"catalog_visible":       isVisible,
			"catalog_hidden_reason": reason,
			"interest":              interest,
		}
		media = append(media, card)

		switch reasonStr {
		case "not_connected":
			todos = append(todos, sellerTodo{Kind: "media_hidden", Severity: "warning", MediaID: id, Route: "integrations",
				Title:  name + " belum tersambung, jadi belum tampil di katalog",
				Detail: "Sambungkan situs di Integrasi agar klien bisa memesan."})
		case "no_price":
			todos = append(todos, sellerTodo{Kind: "media_hidden", Severity: "warning", MediaID: id, Route: "media",
				Title:  name + " belum punya harga",
				Detail: "Media tanpa harga tidak tampil di katalog klien."})
		case "partner_hidden":
			todos = append(todos, sellerTodo{Kind: "media_hidden", Severity: "info", MediaID: id, Route: "media",
				Title:  name + " Anda sembunyikan dari katalog",
				Detail: "Tampilkan lagi dari Daftar Media saat Anda siap menerima pesanan."})
		case "suspended":
			todos = append(todos, sellerTodo{Kind: "media_hidden", Severity: "warning", MediaID: id, Route: "dukungan",
				Title:  name + " ditangguhkan",
				Detail: "Tanyakan alasannya lewat Dukungan."})
		case "inactive":
			inactiveReason := sellerInactiveReason(row.strOr("status", ""))
			todos = append(todos, sellerTodo{Kind: "media_hidden", Severity: inactiveReason.severity, MediaID: id, Route: "media", Query: inactiveReason.query,
				Title:  name + " " + inactiveReason.suffix,
				Detail: inactiveReason.detail})
		}

		if active && domainHealth[id] == "unreachable" {
			todos = append(todos, sellerTodo{Kind: "domain_down", Severity: "warning", MediaID: id, Route: "link-monitor",
				Title:  name + " tidak bisa diakses",
				Detail: "Situs yang sering mati membuat klien ragu memesan, dan artikel terbit tidak bisa dipantau."})
		}
		if active && crawlStatus != "" && crawlStatus != "ok" {
			todos = append(todos, sellerTodo{Kind: "crawler", Severity: "info", MediaID: id, Route: "crawler",
				Title:  "Crawler Tauthub gagal membaca " + name,
				Detail: "Tanpa itu, jumlah artikel mingguan tidak tampil di katalog. Cek Akses Crawler."})
		}
	}

	// ---- order menunggu persetujuan (tenggat SLA 3x24 jam) ----
	waiting, _ := d.queryRows(ctx, `
		SELECT p.id, d.title AS document_title, m.name AS media_name,
		       COALESCE(p.scheduled_at, p.created_at) + make_interval(hours => $2) AS sla_deadline
		FROM publications p
		JOIN media m ON m.id = p.media_id
		LEFT JOIN content_documents d ON d.id = p.document_id
		WHERE m.partner_id = $1 AND p.status = 'waiting_partner'
		ORDER BY sla_deadline ASC
		LIMIT 50`, partnerID, sellerSLAHours)
	waitingList := make([]map[string]any, 0, len(waiting))
	expiring24 := 0
	for i, row := range waiting {
		var deadline time.Time
		if v, ok := row.raw("sla_deadline").(time.Time); ok {
			deadline = v
		}
		remaining := deadline.Sub(now)
		if remaining < 24*time.Hour {
			expiring24++
		}
		title := row.strOr("document_title", "Artikel")
		var dl *string
		if !deadline.IsZero() {
			s := deadline.UTC().Format("2006-01-02T15:04:05.000Z")
			dl = &s
		}
		waitingList = append(waitingList, map[string]any{
			"id": row.raw("id"), "document_title": title, "media_name": row.raw("media_name"),
			"deadline": dl, "remaining": sellerRemaining(remaining),
		})
		if i < 5 {
			sev := "warning"
			if remaining < 24*time.Hour {
				sev = "urgent"
			}
			todos = append(todos, sellerTodo{Kind: "order_expiring", Severity: sev, Route: "publications",
				Query: "pub=" + row.strOr("id", ""), Deadline: dl,
				Title:  "Setujui pesanan \"" + title + "\"",
				Detail: "Di " + row.strOr("media_name", "media") + ", " + sellerRemaining(remaining) + ". Lewat batas itu, pesanan dibatalkan otomatis dan Anda tidak dapat penghasilan."})
		}
	}

	// ---- garansi tautan ----
	var openClaims, goneLinks int
	_ = d.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM link_guardian_claims g
		JOIN publications p ON p.id = g.publication_id
		JOIN media m ON m.id = p.media_id
		WHERE m.partner_id = $1 AND g.status IN ('submitted', 'checking', 'contacted')
		  AND COALESCE(g.client_choice, '') = ''`, partnerID).Scan(&openClaims)
	_ = d.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM publication_link_checks lc
		JOIN publications p ON p.id = lc.publication_id
		JOIN media m ON m.id = p.media_id
		WHERE m.partner_id = $1 AND p.status = 'published' AND lc.state = 'gone'
		  AND `+dbpkg.LinkWarrantySQL("p", "m"), partnerID).Scan(&goneLinks)
	if openClaims > 0 {
		todos = append(todos, sellerTodo{Kind: "link_claim", Severity: "urgent", Route: "link-monitor",
			Title:  itoa(openClaims) + " artikel perlu diperbaiki tautannya",
			Detail: "Klien bisa memilih artikel pengganti atau kredit kembali, dan penghasilan artikel itu dibatalkan. Pulihkan tautannya secepatnya."})
	} else if goneLinks > 0 {
		todos = append(todos, sellerTodo{Kind: "link_gone", Severity: "warning", Route: "link-monitor",
			Title:  itoa(goneLinks) + " artikel terbit tidak ditemukan lagi",
			Detail: "Pulihkan sebelum klien mengajukan klaim garansi."})
	}

	// ---- profil: WhatsApp dan rekening ----
	var waNumber string
	var waConsent bool
	_ = d.Pool.QueryRow(ctx,
		"SELECT COALESCE(wa_number, ''), COALESCE(wa_consent, FALSE) FROM notification_preferences WHERE user_id = $1 LIMIT 1",
		user.ID).Scan(&waNumber, &waConsent)
	if strings.TrimSpace(waNumber) == "" || !waConsent {
		sev := "info"
		if len(mediaRows) > waMediaThreshold {
			sev = "warning"
		}
		todos = append(todos, sellerTodo{Kind: "whatsapp", Severity: sev, Route: "profile",
			Title:  "Isi nomor WhatsApp untuk notifikasi pesanan",
			Detail: "Pesanan baru dan pesanan yang hampir kedaluwarsa bisa sampai ke WhatsApp Anda."})
	}
	finance := d.partnerFinanceSummary(ctx, partnerID)
	if partner.BankAccountNumber == nil || strings.TrimSpace(*partner.BankAccountNumber) == "" {
		sev := "info"
		if finance.SettledEarnings > 0 || finance.LifetimePublication > 0 {
			sev = "warning"
		}
		todos = append(todos, sellerTodo{Kind: "payout", Severity: sev, Route: "profile",
			Title:  "Isi rekening atau e-wallet untuk menarik penghasilan",
			Detail: "Tanpa data ini penghasilan tidak bisa dicairkan."})
	}
	if unverified > 0 {
		todos = append(todos, sellerTodo{Kind: "domain_unverified", Severity: "info", Route: "media",
			Title:  itoa(unverified) + " media belum terverifikasi domainnya",
			Detail: "Hanya media terverifikasi yang masuk garansi tayang dan lebih dipercaya klien."})
	}
	if noDewanPers > 0 && len(mediaRows) > 0 {
		todos = append(todos, sellerTodo{Kind: "dewan_pers", Severity: "info", Route: "media",
			Title:  itoa(noDewanPers) + " media belum terverifikasi Dewan Pers",
			Detail: "Verifikasi Dewan Pers menambah kepercayaan klien, terutama instansi dan perusahaan."})
	}

	sort.SliceStable(todos, func(i, j int) bool {
		return sellerSeverityRank[todos[i].Severity] < sellerSeverityRank[todos[j].Severity]
	})

	// ---- pipeline ----
	statusRows, _ := d.queryRows(ctx, `
		SELECT p.status, COUNT(*)::int AS count
		FROM publications p JOIN media m ON m.id = p.media_id
		WHERE m.partner_id = $1 GROUP BY p.status`, partnerID)
	counts := map[string]int{}
	for _, s := range statusRows {
		counts[s.strOr("status", "")] = s.intOr("count", 0)
	}
	handled := 0
	for _, st := range []string{"partner_approved", "partner_rejected", "queued", "processing", "published", "failed", "retrying"} {
		handled += counts[st]
	}
	var rejectionRate any
	if handled > 0 {
		rejectionRate = float64(counts["partner_rejected"]) / float64(handled)
	}
	var unresponsive30 int
	_ = d.Pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM publications p JOIN media m ON m.id = p.media_id
		WHERE m.partner_id = $1 AND p.status = 'cancelled'
		  AND p.cancel_reason LIKE 'Dibatalkan otomatis: media tidak merespons%'
		  AND p.updated_at > NOW() - INTERVAL '30 days'`, partnerID).Scan(&unresponsive30)
	var avgHours any
	if hoursN > 0 {
		avgHours = hoursSum / float64(hoursN)
	}

	// ---- penghasilan: ringkasan + deret 30 hari ----
	seriesRows, _ := d.queryRows(ctx, `
		SELECT to_char(created_at AT TIME ZONE 'Asia/Jakarta', 'YYYY-MM-DD') AS day,
		       COALESCE(SUM(amount), 0)::float8 AS amount, COUNT(*)::int AS orders
		FROM partner_earnings
		WHERE partner_id = $1 AND kind = 'publication' AND status <> 'voided'
		  AND created_at >= NOW() - INTERVAL '30 days'
		GROUP BY 1`, partnerID)
	byDay := make(map[string]rowMap, len(seriesRows))
	for _, s := range seriesRows {
		byDay[s.strOr("day", "")] = s
	}
	series := make([]map[string]any, 0, 30)
	var last30 float64
	var orders30 int
	for i := 29; i >= 0; i-- {
		day := now.In(sellerWIB).AddDate(0, 0, -i).Format("2006-01-02")
		amount, n := 0.0, 0
		if s, ok := byDay[day]; ok {
			amount, n = s.numOr("amount"), s.intOr("orders", 0)
		}
		last30 += amount
		orders30 += n
		series = append(series, map[string]any{"day": day, "amount": amount, "orders": n})
	}

	w.Header().Set("Cache-Control", "private, max-age=10")
	writeJSON(w, http.StatusOK, map[string]any{
		"generated_at": now.UTC().Format("2006-01-02T15:04:05.000Z"),
		"summary": map[string]any{
			"media_count":   len(mediaRows),
			"visible_count": visible,
			"hidden_count":  hidden,
			"todo_count":    len(todos),
		},
		"todos": todos,
		"pipeline": map[string]any{
			"counts":                 counts,
			"waiting":                waitingList,
			"waiting_total":          counts["waiting_partner"],
			"expiring_24h":           expiring24,
			"rejection_rate":         rejectionRate,
			"avg_publish_hours":      avgHours,
			"unresponsive_cancelled": unresponsive30,
			"sla_hours":              sellerSLAHours,
		},
		"earnings": map[string]any{
			"pending":              finance.PendingEarnings,
			"available":            finance.SettledEarnings,
			"paid":                 finance.PaidSettlements,
			"lifetime_publication": finance.LifetimePublication,
			"hold_hours":           dbpkg.PartnerEarningHoldHours,
			"last_30_days":         last30,
			"orders_30_days":       orders30,
			"series":               series,
		},
		"media": media,
	})
}
