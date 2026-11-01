package email

// Penyimpanan template email yang bisa diedit admin + render ke kerangka gaya.

import (
	"context"
	"errors"
	"html"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// StoredTemplate satu baris email_templates.
type StoredTemplate struct {
	Audience  string
	Event     string
	Subject   string
	BodyHTML  string
	Enabled   bool
	UpdatedAt time.Time
}

// LoadTemplate mengambil template tersimpan; nil bila belum ada.
func LoadTemplate(ctx context.Context, pool *pgxpool.Pool, audience, event string) (*StoredTemplate, error) {
	if pool == nil {
		return nil, nil
	}
	row := pool.QueryRow(ctx, `
		SELECT audience, event, subject, body_html, enabled, updated_at
		FROM email_templates WHERE audience = $1 AND event = $2 LIMIT 1`, audience, event)
	var t StoredTemplate
	if err := row.Scan(&t.Audience, &t.Event, &t.Subject, &t.BodyHTML, &t.Enabled, &t.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

// SaveTemplate menyimpan (upsert) template hasil edit admin. Body disanitasi di
// sini juga, bukan hanya di klien.
func SaveTemplate(ctx context.Context, pool *pgxpool.Pool, t StoredTemplate, updatedBy string) error {
	body, _ := security.SanitizeArticleHTML(t.BodyHTML)
	if pool == nil {
		return nil
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO email_templates (audience, event, subject, body_html, enabled, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::uuid, NOW())
		ON CONFLICT (audience, event) DO UPDATE SET
		  subject = EXCLUDED.subject,
		  body_html = EXCLUDED.body_html,
		  enabled = EXCLUDED.enabled,
		  updated_by = EXCLUDED.updated_by,
		  updated_at = NOW()`,
		t.Audience, t.Event, t.Subject, body, t.Enabled, updatedBy)
	return err
}

// ResetTemplate menghapus override sehingga kembali ke bawaan katalog.
func ResetTemplate(ctx context.Context, pool *pgxpool.Pool, audience, event string) error {
	if pool == nil {
		return nil
	}
	_, err := pool.Exec(ctx,
		"DELETE FROM email_templates WHERE audience = $1 AND event = $2", audience, event)
	return err
}

// SubstituteVariables mengganti {nama}, {judul}, dst. Nilai di-escape karena
// body berupa HTML. Variabel yang tidak dikenal dibiarkan apa adanya.
func SubstituteVariables(text string, data map[string]string) string {
	out := text
	for k, v := range data {
		out = strings.ReplaceAll(out, "{"+k+"}", html.EscapeString(v))
	}
	return out
}

// RenderStored merender template tersimpan menjadi email lengkap.
//
// Return ok=false berarti pemanggil harus memakai jalur bawaannya (hardcoded).
// Data variabel: nama, judul, isi, status, jumlah, link, brand.
func RenderStored(ctx context.Context, pool *pgxpool.Pool, style Style, audience, event string, data map[string]string) (subject, htmlBody string, ok bool) {
	stored, err := LoadTemplate(ctx, pool, audience, event)
	if err != nil || stored == nil || !stored.Enabled {
		return "", "", false
	}
	if data == nil {
		data = map[string]string{}
	}
	if _, has := data["brand"]; !has {
		data["brand"] = style.brand()
	}
	subject = strings.TrimSpace(SubstituteVariables(stored.Subject, data))
	if subject == "" {
		return "", "", false
	}
	if strings.TrimSpace(data["link"]) != "" {
		data["link"] = Absolutize(data["link"])
	}
	bodyHTML := SubstituteVariables(stored.BodyHTML, data)
	bodyHTML, _ = security.SanitizeArticleHTML(bodyHTML)
	rendered := PageStyled(style, subject, subject, bodyHTML)
	return rendered.Subject, rendered.HTML, true
}

// RenderEvent merender template tersimpan bila ada, atau body bawaan katalog
// bila admin belum mengubahnya. Pengirim langsung (media review, tiket,
// pengingat) memakai ini supaya copy yang terlihat di panel Template juga
// benar-benar yang dikirim, tanpa menunggu admin menyimpan tiap event.
func RenderEvent(ctx context.Context, pool *pgxpool.Pool, style Style, audience, event string, data map[string]string) (subject, htmlBody string, ok bool) {
	if s, h, ok := RenderStored(ctx, pool, style, audience, event, data); ok {
		return s, h, true
	}
	entry, found := CatalogEvent(audience, event)
	if !found {
		return "", "", false
	}
	if data == nil {
		data = map[string]string{}
	}
	if _, has := data["brand"]; !has {
		data["brand"] = style.brand()
	}
	subject = strings.TrimSpace(SubstituteVariables(entry.Subject, data))
	if subject == "" {
		return "", "", false
	}
	if strings.TrimSpace(data["link"]) != "" {
		data["link"] = Absolutize(data["link"])
	}
	bodyHTML := SubstituteVariables(entry.BodyHTML, data)
	bodyHTML, _ = security.SanitizeArticleHTML(bodyHTML)
	rendered := PageStyled(style, subject, subject, bodyHTML)
	return rendered.Subject, rendered.HTML, true
}

// RenderTemplateHTML merender template secara sinkron untuk pratinjau admin.
// Tidak menyentuh database.
func RenderTemplateHTML(style Style, subject, bodyHTML string, data map[string]string) Rendered {
	if data == nil {
		data = map[string]string{}
	}
	if _, has := data["brand"]; !has {
		data["brand"] = style.brand()
	}
	subject = strings.TrimSpace(SubstituteVariables(subject, data))
	if subject == "" {
		subject = "Pratinjau email"
	}
	if strings.TrimSpace(data["link"]) != "" {
		data["link"] = Absolutize(data["link"])
	}
	body := SubstituteVariables(bodyHTML, data)
	body, _ = security.SanitizeArticleHTML(body)
	return PageStyled(style, subject, subject, body)
}

// ---------- kanal email per event ----------

// ChannelState status saklar email satu event.
type ChannelState struct {
	Event     string
	Enabled   bool
	EnabledAt time.Time
}

// LoadChannels memuat seluruh saklar (dipakai cache notify).
func LoadChannels(ctx context.Context, pool *pgxpool.Pool) (map[string]ChannelState, error) {
	out := map[string]ChannelState{}
	if pool == nil {
		return out, nil
	}
	rows, err := pool.Query(ctx, "SELECT event, enabled, COALESCE(enabled_at, updated_at) FROM email_event_channels")
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var c ChannelState
		if err := rows.Scan(&c.Event, &c.Enabled, &c.EnabledAt); err != nil {
			return out, err
		}
		out[c.Event] = c
	}
	return out, rows.Err()
}

// SetChannel menyalakan/mematikan email satu event. Saat dinyalakan,
// enabled_at diperbarui agar notifikasi lama tidak ikut terkirim.
func SetChannel(ctx context.Context, pool *pgxpool.Pool, event string, enabled bool, updatedBy string) error {
	if pool == nil {
		return nil
	}
	var enabledAt any
	if enabled {
		enabledAt = time.Now()
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO email_event_channels (event, enabled, enabled_at, updated_by, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, NOW())
		ON CONFLICT (event) DO UPDATE SET
		  enabled = EXCLUDED.enabled,
		  enabled_at = CASE WHEN EXCLUDED.enabled THEN EXCLUDED.enabled_at ELSE NULL END,
		  updated_by = EXCLUDED.updated_by,
		  updated_at = NOW()`,
		event, enabled, enabledAt, updatedBy)
	return err
}
