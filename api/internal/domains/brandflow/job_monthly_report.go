package brandflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/email"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
)

type mrTopMedia struct {
	Name  string
	Count int
}

// schSweepMonthlyReport: enqueue laporan bulanan untuk brand yang punya
// kampanye bulan lalu dan belum dikirim. Hanya dijalankan tanggal 1-2
// (memberi waktu satu hari retry kalau tanggal 1 gagal).
func (d *Deps) schSweepMonthlyReport(ctx context.Context, limit int) int {
	if !d.schClaimDaily(ctx, "monthly_report") {
		return 0
	}
	if time.Now().Day() > 2 {
		return 0
	}

	month := time.Now().AddDate(0, -1, 0).Format("2006-01")

	rows, err := d.Pool.Query(ctx, `
		SELECT DISTINCT c.brand_id
		FROM campaigns c
		WHERE c.status <> 'cancelled'
		  AND c.created_at >= date_trunc('month', NOW()) - INTERVAL '1 month'
		  AND c.created_at < date_trunc('month', NOW())
		  AND NOT EXISTS (
		    SELECT 1 FROM monthly_report_runs r
		    WHERE r.brand_id = c.brand_id AND r.month = $1
		  )
		LIMIT $2`, month, limit)
	if err != nil {
		slog.Error("[scheduler] query brand laporan bulanan gagal", "err", err)
		d.schReleaseDaily(ctx, "monthly_report")
		return 0
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var brandID string
		if err := rows.Scan(&brandID); err != nil {
			continue
		}
		payload, _ := json.Marshal(map[string]string{
			"brandId": brandID,
			"month":   month,
		})
		if _, err := d.Pool.Exec(ctx,
			`INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at)
			 VALUES (gen_random_uuid(), 'monthly_report_email', $1::jsonb, 'pending', 0, 3, NOW())`,
			string(payload)); err != nil {
			slog.Error("[scheduler] enqueue laporan bulanan gagal", "brand", brandID, "err", err)
			continue
		}
		count++
	}
	if count > 0 {
		slog.Info("[scheduler] laporan bulanan diantrekan", "jumlah", count, "bulan", month)
	}
	return count
}

// monthlyReportJob: handler job `monthly_report_email`. Mengirim email
// ringkasan bulanan ke admin brand: jumlah artikel terbit, total belanja,
// saldo, dan media terbaik.
func (d *Deps) monthlyReportJob(ctx context.Context, job jobs.JobRow) error {
	brandID, _ := job.Payload["brandId"].(string)
	month, _ := job.Payload["month"].(string)
	if brandID == "" || month == "" {
		return fmt.Errorf("payload brandId/month kosong")
	}

	// Idempotency: INSERT ON CONFLICT DO NOTHING.
	tag, err := d.Pool.Exec(ctx,
		`INSERT INTO monthly_report_runs (brand_id, month) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		brandID, month)
	if err != nil {
		return fmt.Errorf("insert monthly_report_runs: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil
	}

	// Ambil nama brand dan email admin.
	var brandName, adminEmail, adminUserID string
	err = d.Pool.QueryRow(ctx, `
		SELECT b.name,
		       COALESCE(u.email, ''),
		       COALESCE(u.id, '')
		FROM brands b
		LEFT JOIN brand_members bm ON bm.brand_id = b.id AND bm.role = 'owner'
		LEFT JOIN users u ON u.id = bm.user_id
		WHERE b.id = $1
		LIMIT 1`, brandID).Scan(&brandName, &adminEmail, &adminUserID)
	if err != nil || adminEmail == "" {
		slog.Warn("[monthly-report] brand tanpa admin/email, lewati", "brand", brandID)
		return nil
	}

	// Cek opt-out.
	var optout bool
	_ = d.Pool.QueryRow(ctx,
		`SELECT COALESCE(monthly_report_optout, false) FROM notification_preferences WHERE user_id = $1`,
		adminUserID).Scan(&optout)
	if optout {
		return nil
	}

	// Agregat bulan ini.
	var pubCount int
	var totalSpent float64
	_ = d.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM publications p
		WHERE p.brand_id = $1
		  AND p.status = 'published'
		  AND p.published_at >= $2::date
		  AND p.published_at < ($2::date + INTERVAL '1 month')`,
		brandID, month+"-01").Scan(&pubCount)

	_ = d.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(ABS(amount)), 0)
		FROM credit_ledger_entries
		WHERE brand_id = $1
		  AND (entry_type = 'capture' OR (entry_type = 'adjustment' AND amount < 0))
		  AND created_at >= $2::date
		  AND created_at < ($2::date + INTERVAL '1 month')`,
		brandID, month+"-01").Scan(&totalSpent)

	var balance float64
	_ = d.Pool.QueryRow(ctx,
		`SELECT COALESCE(balance, 0) FROM credit_wallets WHERE brand_id = $1`, brandID).Scan(&balance)

	// Top 3 media.
	topRows, err := d.Pool.Query(ctx, `
		SELECT m.name, COUNT(*)::int AS cnt
		FROM publications p
		JOIN media m ON m.id = p.media_id
		WHERE p.brand_id = $1
		  AND p.status = 'published'
		  AND p.published_at >= $2::date
		  AND p.published_at < ($2::date + INTERVAL '1 month')
		GROUP BY m.name
		ORDER BY cnt DESC
		LIMIT 3`, brandID, month+"-01")
	var topMediaList []mrTopMedia
	if err == nil {
		defer topRows.Close()
		for topRows.Next() {
			var tm mrTopMedia
			if topRows.Scan(&tm.Name, &tm.Count) == nil {
				topMediaList = append(topMediaList, tm)
			}
		}
	}

	// Render email body.
	htmlBody := mrBuildHTML(brandName, month, pubCount, totalSpent, balance, topMediaList)

	recipientID := adminUserID
	refID := "monthly-report-" + brandID + "-" + month
	_, _, err = email.SendTransactionalEmail(ctx, d.Pool, email.SendOptions{
		Template:        "report.monthly",
		To:              adminEmail,
		RecipientUserID: &recipientID,
		ReferenceID:     &refID,
		Event:           "report.monthly",
		Audience:        email.AudienceClient,
		Data: map[string]string{
			"nama":  brandName,
			"brand": brandName,
			"isi":   htmlBody,
			"link":  "/klien/" + brandID,
		},
	})
	if err != nil {
		return fmt.Errorf("kirim email laporan bulanan: %w", err)
	}
	return nil
}

func mrBuildHTML(brandName, month string, pubCount int, totalSpent, balance float64, topMedia []mrTopMedia) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf(
		`<p style="margin:0 0 12px;line-height:1.6;">Berikut ringkasan aktivitas <b>%s</b> selama bulan <b>%s</b>:</p>`, brandName, month))

	b.WriteString(`<table style="width:100%;border-collapse:collapse;margin:12px 0;">`)
	mrRow(&b, "Artikel terbit", fmt.Sprintf("%d", pubCount))
	mrRow(&b, "Total belanja", fmt.Sprintf("Rp %s", mrFormatRupiah(totalSpent)))
	mrRow(&b, "Saldo saat ini", fmt.Sprintf("Rp %s", mrFormatRupiah(balance)))
	b.WriteString(`</table>`)

	if len(topMedia) > 0 {
		b.WriteString(`<p style="margin:16px 0 8px;font-weight:600;">Media terbaik:</p>`)
		b.WriteString(`<ol style="margin:0;padding-left:20px;">`)
		for _, tm := range topMedia {
			b.WriteString(fmt.Sprintf(`<li style="margin:4px 0;">%s (%d artikel)</li>`, tm.Name, tm.Count))
		}
		b.WriteString(`</ol>`)
	}
	return b.String()
}

func mrRow(b *strings.Builder, label, value string) {
	b.WriteString(fmt.Sprintf(
		`<tr><td style="padding:6px 0;border-bottom:1px solid #eee;">%s</td>`+
			`<td style="padding:6px 0;border-bottom:1px solid #eee;text-align:right;font-weight:600;">%s</td></tr>`,
		label, value))
}

func mrFormatRupiah(amount float64) string {
	intVal := int64(amount)
	if intVal == 0 {
		return "0"
	}
	s := fmt.Sprintf("%d", intVal)
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, '.')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
