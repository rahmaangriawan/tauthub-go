package email

// Dispatcher email untuk event yang hanya punya notifikasi in-app
// (katalog dengan Sender == SenderGeneric).
//
// Event yang sudah punya pengirim sendiri (media review, tiket, pengingat)
// TIDAK lewat sini — itu urusan pengirim masing-masing lewat ShouldSend.
// Dispatcher hanya mengirim notifikasi yang dibuat SETELAH kanal dinyalakan
// (enabled_at), jadi menyalakan event lama tidak membanjiri pengguna dengan
// email backlog.

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DispatchNotificationEmails mengantrekan email untuk notifikasi generic yang
// kanalnya aktif. Mengembalikan jumlah email yang masuk antrean.
func DispatchNotificationEmails(ctx context.Context, pool *pgxpool.Pool, limit int) (int, error) {
	if pool == nil {
		return 0, nil
	}
	if limit < 1 {
		limit = 1
	}
	rows, err := pool.Query(ctx, `
		SELECT n.id, n.user_id, n.type, n.title, COALESCE(n.body, ''), COALESCE(n.link, ''),
		       u.role, COALESCE(u.display_name, ''), COALESCE(u.email, ''),
		       COALESCE(p.email_partner, true), COALESCE(p.email_campaign, true)
		FROM notifications n
		JOIN users u ON u.id = n.user_id AND u.is_active
		JOIN email_event_channels c ON c.event = n.type AND c.enabled
		LEFT JOIN notification_preferences p ON p.user_id = n.user_id
		WHERE n.read_at IS NULL
		  AND n.expired_at IS NULL
		  AND c.enabled_at IS NOT NULL
		  AND n.created_at >= c.enabled_at
		  AND n.created_at > NOW() - INTERVAL '24 hours'
		  AND u.role IN ('client', 'partner')
		  AND NOT EXISTS (
		    SELECT 1 FROM email_deliveries d WHERE d.dedupe_key = 'notif:' || n.id
		  )
		ORDER BY n.created_at ASC
		LIMIT $1`, limit*4)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type candidate struct {
		notifID, userID, event, title, body, link, role, name, email string
		prefPartner, prefCampaign                                    bool
	}
	list := make([]candidate, 0, limit)
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.notifID, &c.userID, &c.event, &c.title, &c.body, &c.link,
			&c.role, &c.name, &c.email, &c.prefPartner, &c.prefCampaign); err != nil {
			return 0, err
		}
		list = append(list, c)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	style := LoadStyle(ctx, pool)
	sent := 0
	for _, c := range list {
		if sent >= limit {
			break
		}
		if !IsGenericEvent(c.event) {
			continue
		}
		audience := AudienceForRole(c.role)
		if audience == "" {
			continue
		}
		// Preferensi email: mitra memakai email_partner, klien email_campaign.
		if audience == AudiencePartner && !c.prefPartner {
			continue
		}
		if audience == AudienceClient && !c.prefCampaign {
			continue
		}
		if strings.TrimSpace(c.email) == "" {
			continue
		}

		data := map[string]string{
			"nama":   c.name,
			"judul":  c.title,
			"isi":    c.body,
			"link":   c.link,
			"brand":  style.brand(),
			"status": "",
			"jumlah": "",
		}
		subject, htmlBody, ok := RenderEvent(ctx, pool, style, audience, c.event, data)
		if !ok {
			continue
		}

		dedupe := "notif:" + c.notifID
		userID := c.userID
		notifID := c.notifID
		event := c.event
		if _, _, err := EnqueueEmail(ctx, pool, EnqueueOptions{
			Template:       "email-template:" + event,
			RecipientEmail: c.email,
			UserID:         &userID,
			ReferenceID:    &notifID,
			Event:          event,
			Subject:        subject,
			HTML:           htmlBody,
			DedupeKey:      &dedupe,
		}); err != nil {
			continue
		}
		sent++
	}
	return sent, nil
}
