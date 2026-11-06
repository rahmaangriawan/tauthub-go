package email

import (
	"context"
	"html"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminNoticeOptions: satu pemberitahuan email untuk semua admin aktif.
type AdminNoticeOptions struct {
	// Template nama jenis email di log (mis. "admin-publication-draft").
	Template    string
	Event       string
	ReferenceID string
	Subject     string
	// Paragraphs teks polos; di-escape sebelum dimasukkan ke HTML.
	Paragraphs []string
	Link       string
	CTALabel   string
}

// EnqueueAdminNotice mengantrekan email ke setiap user admin aktif yang punya
// email. Dedupe per (template, referensi, admin) supaya percobaan ulang tidak
// mengirim dua kali. Mengembalikan jumlah email yang masuk antrean.
func EnqueueAdminNotice(ctx context.Context, pool *pgxpool.Pool, opts AdminNoticeOptions) int {
	if pool == nil || strings.TrimSpace(opts.Subject) == "" {
		return 0
	}
	rows, err := pool.Query(ctx,
		"SELECT id, email, COALESCE(display_name, '') FROM users WHERE role = 'admin' AND is_active = TRUE AND COALESCE(email, '') <> ''")
	if err != nil {
		return 0
	}
	type admin struct{ id, email, name string }
	var admins []admin
	for rows.Next() {
		var a admin
		if err := rows.Scan(&a.id, &a.email, &a.name); err == nil {
			admins = append(admins, a)
		}
	}
	rows.Close()
	if len(admins) == 0 {
		return 0
	}

	style := LoadStyle(ctx, pool)
	sent := 0
	for _, a := range admins {
		name := strings.TrimSpace(a.name)
		if name == "" {
			name = "Admin"
		}
		parts := []string{pg("Halo " + html.EscapeString(name) + ",")}
		for _, p := range opts.Paragraphs {
			parts = append(parts, pg(html.EscapeString(p)))
		}
		if strings.TrimSpace(opts.Link) != "" && strings.TrimSpace(opts.CTALabel) != "" {
			parts = append(parts, `<p style="margin:16px 0 0;"><a href="`+html.EscapeString(Absolutize(opts.Link))+`" style="color:#2b67ec;font-weight:600;text-decoration:none;">`+html.EscapeString(opts.CTALabel)+`</a></p>`)
		}
		rendered := PageStyled(style, opts.Subject, opts.Subject, strings.Join(parts, ""))
		userID := a.id
		ref := opts.ReferenceID
		dedupe := opts.Template + ":" + opts.ReferenceID + ":" + a.id
		if _, _, err := EnqueueEmail(ctx, pool, EnqueueOptions{
			Template:       opts.Template,
			RecipientEmail: a.email,
			UserID:         &userID,
			ReferenceID:    &ref,
			Event:          opts.Event,
			Subject:        rendered.Subject,
			HTML:           rendered.HTML,
			DedupeKey:      &dedupe,
		}); err == nil {
			sent++
		}
	}
	return sent
}
