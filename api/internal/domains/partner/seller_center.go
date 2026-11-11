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

var sellerSeverityRank = map[string]int{"urgent": 0, "warning": 1, "info": 2}

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

	peerPool := d.loadPeerPool(ctx)
	historyRows, _ := d.queryRows(ctx, `
		SELECT h.media_id, h.from_price::float8 AS from_price, h.to_price::float8 AS to_price, h.created_at
		FROM media_price_history h
		WHERE h.media_id IN (SELECT id FROM media WHERE partner_id = $1)
		ORDER BY h.created_at DESC
		LIMIT 200`, partnerID)
	lastPriceChange := map[string]map[string]any{}
	for _, h := range historyRows {
		id := h.strOr("media_id", "")
		if _, seen := lastPriceChange[id]; seen {
			continue
		}
		lastPriceChange[id] = map[string]any{
			"from": h.raw("from_price"), "to": h.raw("to_price"), "at": h.timePtr("created_at"),
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

		card := map[string]any{
			"id":                    id,
			"name":                  name,
			"website_url":           row.raw("website_url"),
			"status":                row.raw("status"),
			"media_type":            row.strOr("media_type", "mainstream"),
			"integration_type":      row.raw("integration_type"),
			"has_connection":        row.boolOr("has_connection", false),
			"catalog_visible":       isVisible,
			"catalog_hidden_reason": reason,
			"domain_verified":       verified,
			"dewan_pers_verified":   row.boolOr("dewan_pers_verified", false),
			"domain_health":         domainHealth[id],
			"price":                 row.numOr("price"),
			"link_rel":              existingLinkRel(row),
			"link_duration_months":  row.intOr("link_duration_months", 12),
			"seo_da":                row.raw("seo_da"),
			"seo_pa":                row.raw("seo_pa"),
			"seo_spam_score":        row.raw("seo_spam_score"),
			"sw_visits":             row.numPtr("sw_visits"),
			"rating_avg":            row.numPtr("rating_avg"),
			"rating_count":          row.intOr("rating_count", 0),
			"articles_7d":           row.numPtr("articles_7d"),
			"crawl_last_status":     row.raw("last_status"),
			"total_terbit":          totalTerbit,
			"pending_publications":  row.intOr("pending_publications", 0),
			// Tanpa riwayat order, skor kesehatan belum berarti apa pun.
			"has_history":     totalTerbit > 0,
			"health_score":    row.raw("health_score"),
			"avg_publish_hrs": nil,
		}
		if hasHours && totalTerbit > 0 {
			card["avg_publish_hrs"] = hours
		}
		// Pembanding harga sebaya (nil bila belum ada harga atau pembanding kurang).
		self := peerMedia{ID: id, Category: row.strOr("category", ""), MediaType: row.strOr("media_type", "mainstream"), Price: row.numOr("price")}
		if v, ok := row.num("seo_da"); ok {
			self.DA = &v
		}
		card["price_benchmark"] = benchmarkPrice(self, peerPool)
		card["last_price_change"] = lastPriceChange[id]
		media = append(media, card)

		switch reasonStr {
		case "not_connected":
			todos = append(todos, sellerTodo{Kind: "media_hidden", Severity: "warning", MediaID: id, Route: "integrations",
				Title:  name + " belum tersambung, jadi belum tampil di katalog",
				Detail: "Sambungkan situs Anda di Integrasi supaya klien bisa memesan."})
		case "no_price":
			todos = append(todos, sellerTodo{Kind: "media_hidden", Severity: "warning", MediaID: id, Route: "media",
				Title:  name + " belum punya harga",
				Detail: "Media tanpa harga tidak tampil di katalog klien."})
		case "partner_hidden":
			todos = append(todos, sellerTodo{Kind: "media_hidden", Severity: "info", MediaID: id, Route: "media",
				Title:  name + " Anda sembunyikan dari katalog",
				Detail: "Tampilkan lagi dari Daftar Media bila Anda siap menerima order."})
		case "suspended":
			todos = append(todos, sellerTodo{Kind: "media_hidden", Severity: "warning", MediaID: id, Route: "dukungan",
				Title:  name + " ditangguhkan",
				Detail: "Hubungi tim Tauthub lewat Dukungan untuk tahu alasannya."})
		case "inactive":
			todos = append(todos, sellerTodo{Kind: "media_hidden", Severity: "info", MediaID: id, Route: "media",
				Title:  name + " belum aktif di katalog",
				Detail: "Status media: " + row.strOr("status", "-") + ". Media baru ditinjau tim Tauthub dulu."})
		}

		if active && domainHealth[id] == "unreachable" {
			todos = append(todos, sellerTodo{Kind: "domain_down", Severity: "warning", MediaID: id, Route: "link-monitor",
				Title:  name + " tidak bisa diakses",
				Detail: "Situs yang sering mati membuat klien ragu memesan, dan artikel yang sudah terbit tidak terpantau."})
		}
		if active && crawlStatus != "" && crawlStatus != "ok" {
			todos = append(todos, sellerTodo{Kind: "crawler", Severity: "info", MediaID: id, Route: "crawler",
				Title:  "Crawler Tauthub gagal membaca " + name,
				Detail: "Tanpa bacaan ini, jumlah artikel mingguan Anda tidak tampil di katalog. Cek Akses Crawler."})
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
				Detail: "Di " + row.strOr("media_name", "media") + ", " + sellerRemaining(remaining) + ". Lewat tenggat, pesanan dibatalkan otomatis dan tidak ada penghasilan."})
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
			Title:  "Isi nomor WhatsApp untuk notifikasi order",
			Detail: "Order baru dan pesanan yang hampir kedaluwarsa bisa sampai ke WhatsApp Anda."})
	}
	finance := d.partnerFinanceSummary(ctx, partnerID)
	if partner.BankAccountNumber == nil || strings.TrimSpace(*partner.BankAccountNumber) == "" {
		sev := "info"
		if finance.SettledEarnings > 0 || finance.LifetimePublication > 0 {
			sev = "warning"
		}
		todos = append(todos, sellerTodo{Kind: "payout", Severity: sev, Route: "profile",
			Title:  "Isi rekening atau e-wallet untuk menarik penghasilan",
			Detail: "Tanpa data ini penghasilan Anda tidak bisa dicairkan."})
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
