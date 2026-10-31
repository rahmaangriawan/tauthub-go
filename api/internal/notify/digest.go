package notify

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/email"
)

// Digest harian untuk notifikasi tier 3.
//
// Tanpa digest, setiap event tier 3 yang kebetulan dipetakan ke email akan
// menjadi satu email berbayar. Dengan digest, semua event tier 3 yang belum
// dibaca dikumpulkan jadi satu email per user per hari. Kalau user sudah
// membaca semuanya di aplikasi, tidak ada email yang keluar sama sekali.

// DigestItem satu baris di dalam email ringkasan.
type DigestItem struct {
	Title string
	Body  string
	Link  string
}

// BuildDigestBody merangkai judul dan isi email ringkasan.
//
// Isi daftar sengaja HTML sederhana (tanpa CSS eksternal), lalu dibungkus
// kerangka brand bersama supaya email ringkasan tetap memakai logo dan footer
// Tauthub seperti email transaksional lain.
func BuildDigestBody(items []DigestItem) (subject, htmlBody string) {
	if len(items) == 0 {
		return "", ""
	}
	subject = fmt.Sprintf("Ringkasan notifikasi Tauthub (%d)", len(items))
	var b strings.Builder
	b.WriteString(`<p style="margin:0 0 16px">Berikut kabar yang belum Anda buka di Tauthub.</p>`)
	b.WriteString(`<ul style="margin:0;padding-left:18px">`)
	for _, it := range items {
		b.WriteString(`<li style="margin:0 0 12px">`)
		if it.Link != "" {
			b.WriteString(`<a href="` + html.EscapeString(email.Absolutize(it.Link)) + `" style="color:#2b67ec;font-weight:600;text-decoration:none">` +
				html.EscapeString(it.Title) + `</a>`)
		} else {
			b.WriteString(`<strong>` + html.EscapeString(it.Title) + `</strong>`)
		}
		if it.Body != "" {
			b.WriteString(`<div style="color:#667085">` + html.EscapeString(it.Body) + `</div>`)
		}
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ul>`)
	return subject, email.Page(subject, "Kabar terbaru yang menunggu Anda di Tauthub.", b.String()).HTML
}

// EnqueueDigestJobs menaruh satu job pengiriman digest untuk setiap user yang
// punya notifikasi tier 3 belum dibaca.
//
// Fungsi ini hanya meng-enqueue, tidak mengirim. Pemanggil (siklus scheduler)
// yang menentukan kapan dijalankan, misalnya sekali sehari.
//
// Kuota tidak dipesan di sini: enqueue belum tentu jadi email kalau job gagal.
// Yang dicek hanya apakah user sudah penuh di jam berjalan, supaya job tidak
// menumpuk untuk user yang jelas akan ditolak.
func EnqueueDigestJobs(ctx context.Context, pool *pgxpool.Pool, now time.Time, minAge time.Duration, maxUsers int) (int, error) {
	if pool == nil {
		return 0, nil
	}
	if maxUsers < 1 {
		maxUsers = 1
	}
	// Ambil kandidat: user dengan notifikasi tier 3 yang belum dibaca dan
	// sudah berumur lebih dari minAge. Daftar tipe tier 3 diambil dari
	// registry supaya menambah event baru tidak perlu menyunting query ini.
	types := TierTypes(TierInfo)
	if len(types) == 0 {
		return 0, nil
	}

	rows, err := pool.Query(ctx, `
		SELECT n.user_id, count(*)::int
		FROM notifications n
		JOIN users u ON u.id = n.user_id AND u.is_active
		LEFT JOIN notification_preferences p ON p.user_id = n.user_id
		WHERE n.read_at IS NULL
		  AND n.expired_at IS NULL
		  AND n.type = ANY($1)
		  AND n.created_at < $2
		  AND COALESCE(p.email_campaign, true)
		  AND COALESCE(p.email_partner, true)
		  AND NOT EXISTS (
		    SELECT 1 FROM jobs j
		    WHERE j.kind = 'send_email'
		      AND j.status IN ('pending', 'running')
		      AND j.payload->>'digest' = 'true'
		      AND j.payload->>'userId' = n.user_id
		  )
		GROUP BY n.user_id
		ORDER BY count(*) DESC
		LIMIT $3`, types, now.Add(-minAge), maxUsers)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	candidates := make([]string, 0, 32)
	for rows.Next() {
		var userID string
		var count int
		if err := rows.Scan(&userID, &count); err != nil {
			return 0, err
		}
		candidates = append(candidates, userID)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	queued := 0
	for _, userID := range candidates {
		used, err := QuotaUsed(ctx, pool, userID, "email", QuotaWindow)
		if err != nil {
			// Kuota yang tidak terbaca bukan alasan menggagalkan seluruh
			// putaran; user ini dilewati dan dicoba lagi besok.
			continue
		}
		if used >= EmailHourlyLimit {
			continue
		}

		// Alamat email diambil di sini, bukan di job: baris email_deliveries
		// mewajibkan recipient_email, dan job send_email hanya menguras tabel
		// itu. Job tanpa baris delivery tidak akan mengirim apa pun.
		var recipient string
		if err := pool.QueryRow(ctx,
			`SELECT COALESCE(email, '') FROM users WHERE id = $1 AND is_active`, userID).Scan(&recipient); err != nil {
			continue
		}
		if recipient == "" {
			continue
		}

		items, err := loadDigestItems(ctx, pool, userID, types, now.Add(-minAge))
		if err != nil || len(items) == 0 {
			continue
		}
		subject, htmlBody := BuildDigestBody(items)
		if subject == "" {
			continue
		}

		// Dedupe per hari: satu email ringkasan per user per hari. Kunci
		// memakai tanggal supaya besok boleh kirim lagi.
		dedupeKey := "digest:" + now.Format("2006-01-02")
		uid := userID
		if _, _, err := email.EnqueueEmail(ctx, pool, email.EnqueueOptions{
			Template:       "notify-digest",
			RecipientEmail: recipient,
			UserID:         &uid,
			Event:          "notify.digest",
			Subject:        subject,
			HTML:           htmlBody,
			DedupeKey:      &dedupeKey,
		}); err != nil {
			continue
		}
		queued++
	}
	return queued, nil
}

// loadDigestItems mengambil notifikasi tier 3 yang belum dibaca untuk satu
// user, terbaru lebih dulu, dibatasi jumlahnya supaya email tidak memanjang.
func loadDigestItems(ctx context.Context, pool *pgxpool.Pool, userID string, types []string, before time.Time) ([]DigestItem, error) {
	rows, err := pool.Query(ctx, `
		SELECT title, COALESCE(body, ''), COALESCE(link, '')
		FROM notifications
		WHERE user_id = $1
		  AND read_at IS NULL
		  AND expired_at IS NULL
		  AND type = ANY($2)
		  AND created_at < $3
		ORDER BY created_at DESC
		LIMIT 20`, userID, types, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DigestItem, 0, 20)
	for rows.Next() {
		var it DigestItem
		if err := rows.Scan(&it.Title, &it.Body, &it.Link); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// TierTypes mengembalikan daftar tipe notifikasi yang terdaftar pada satu tier,
// diurutkan agar hasil query stabil dan mudah dibaca di log.
func TierTypes(tier int) []string {
	out := make([]string, 0, len(policies))
	for t, p := range policies {
		if p.Tier == tier {
			out = append(out, t)
		}
	}
	// Urutan peta Go acak; sort supaya query plan dan log tidak berubah-ubah.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
