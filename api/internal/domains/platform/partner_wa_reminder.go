package platform

// Pengingat WhatsApp untuk mitra: dipakai aksi "Kirim pengingat isi WA" di
// /admin/partners. Mitra dengan lebih dari 1 media yang belum mengisi nomor WA
// (notification_preferences.wa_number + wa_consent) dikirimi notifikasi lonceng
// + email. Idempoten: maksimal sekali per 30 hari per mitra.

import (
	"net/http"
	"os"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/email"
)

const partnerWAThreshold = 1

// AdminPartnerWAReminder: POST /v1/platform/partners/{partnerId}/wa-reminder
func (d *Deps) AdminPartnerWAReminder(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	partnerID := chiParam(r, "partnerId")
	row, err := d.queryRow(r.Context(), `
		SELECT p.id, p.name, p.public_id, p.user_id, u.email,
		       (SELECT COUNT(*) FROM media m WHERE m.partner_id = p.id AND m.status <> 'hidden') AS media_count,
		       np.wa_number, np.wa_consent
		FROM partners p
		JOIN users u ON u.id = p.user_id
		LEFT JOIN notification_preferences np ON np.user_id = p.user_id
		WHERE p.id = $1 LIMIT 1`, partnerID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Partner not found"))
		return
	}

	waNumber := strings.TrimSpace(row.strOr("wa_number", ""))
	waConsent := row.boolOr("wa_consent", false)
	if waNumber != "" && waConsent {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "sent": false,
			"detail": "Mitra ini sudah mengisi nomor WhatsApp dan menyetujui dihubungi.",
		})
		return
	}

	partnerUserID := row.strOr("user_id", "")
	publicID := row.strOr("public_id", "")
	partnerName := row.strOr("name", "")
	if partnerName == "" {
		partnerName = row.strOr("email", "Mitra")
	}
	mediaCount := row.intOr("media_count", 0)

	// Anti-ulang: notifikasi dengan dedupe_key sama dalam 30 hari terakhir.
	dedupeKey := "partner.wa_required:" + partnerID
	dup, dupErr := d.queryRow(r.Context(), `
		SELECT 1 AS found FROM notifications
		WHERE user_id = $1 AND dedupe_key = $2 AND created_at > NOW() - INTERVAL '30 days'
		LIMIT 1`, partnerUserID, dedupeKey)
	if dupErr != nil {
		writeErr(w, d, dupErr)
		return
	}
	if dup != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "sent": false,
			"detail": "Pengingat sudah dikirim dalam 30 hari terakhir — tunggu sebelum mengirim lagi.",
		})
		return
	}

	title := "Lengkapi nomor WhatsApp Anda"
	body := "Media Anda sudah " + ptItoa(mediaCount) + " dan pesanan bisa masuk kapan saja. " +
		"Mohon isi nomor WhatsApp dan centang izin dihubungi di Profil → Notifikasi, " +
		"supaya tim Tauthub bisa mengonfirmasi pesanan lebih cepat."
	link := "/mitra"
	if publicID != "" {
		link = "/mitra/" + publicID + "/profile"
	}

	notified := false
	if partnerUserID != "" {
		if err := d.exec(r.Context(), `
			INSERT INTO notifications (id, user_id, type, title, body, link, dedupe_key)
			VALUES (gen_random_uuid(), $1, 'partner.wa_required', $2, $3, $4, $5)`,
			partnerUserID, title, body, link, dedupeKey); err != nil {
			d.logWarn("notifikasi wa_required ke mitra gagal", partnerID, err)
		} else {
			notified = true
		}
	}

	emailed := false
	recipientEmail := row.strOr("email", "")
	if recipientEmail != "" {
		appURL := strings.TrimRight(os.Getenv("NEXT_PUBLIC_APP_URL"), "/")
		if appURL == "" {
			appURL = "https://app.tauthub.com"
		}
		cta := appURL + link
		html := "<p>Halo " + partnerName + ",</p>" +
			"<p>Media Anda sudah <strong>" + ptItoa(mediaCount) + "</strong> di Tauthub, dan pesanan dari klien bisa masuk kapan saja. " +
			"Agar tim kami bisa mengonfirmasi pesanan lebih cepat (dan menghindari pesanan yang gagal diproses), mohon lengkapi:</p>" +
			"<ul><li>nomor WhatsApp aktif, dan</li><li>centang izin dihubungi lewat WhatsApp.</li></ul>" +
			`<p><a href="` + cta + `">Buka Profil → Notifikasi</a></p>` +
			"<p>Terima kasih.</p>"
		if _, _, err := email.EnqueueEmail(r.Context(), d.Pool, email.EnqueueOptions{
			Template:       "partner-wa-required",
			RecipientEmail: recipientEmail,
			UserID:         mpStrPtr(partnerUserID),
			ReferenceID:    mpStrPtr(partnerID),
			Event:          "partner-wa-required",
			Subject:        "Lengkapi nomor WhatsApp Anda di Tauthub",
			HTML:           html,
			DedupeKey:      mpStrPtr(dedupeKey),
		}); err != nil {
			d.logWarn("email wa_required ke mitra gagal", partnerID, err)
		} else {
			emailed = true
		}
	}

	d.audit(r.Context(), user.ID, "partner.wa_reminder_sent", "partner", partnerID)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "sent": true, "notified": notified, "emailed": emailed,
		"detail": "Pengingat dikirim ke " + recipientEmail + ".",
	})
}
