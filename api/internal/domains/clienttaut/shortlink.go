package clienttaut

import (
	"context"
	"crypto/rand"
	"strings"

	"github.com/google/uuid"
)

// Port lib/tracking/short-link.ts.

const (
	shortLinkCodeLength = 5
	shortLinkDomainKey  = "short_link_domain"
	codeAlphabet        = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

type writerShortLink struct {
	Code           string
	DestinationURL *string
}

// randomCode5: kode acak 5 char (A-Z a-z 0-9).
func randomCode5() string {
	buf := make([]byte, shortLinkCodeLength)
	if _, err := rand.Read(buf); err != nil {
		for i := range buf {
			buf[i] = 0
		}
	}
	var b strings.Builder
	for _, v := range buf {
		b.WriteByte(codeAlphabet[int(v)%len(codeAlphabet)])
	}
	return b.String()
}

// getShortLinkBase: domain short link dari platform_settings; "" = pakai domain utama.
func (d *Deps) getShortLinkBase(ctx context.Context) string {
	row, err := d.queryRow(ctx, "SELECT value FROM platform_settings WHERE key = $1 LIMIT 1", shortLinkDomainKey)
	if err != nil || row == nil {
		return ""
	}
	return strings.TrimSpace(row.strOr("value", ""))
}

// upsertWriterShortLink: upsert idempoten per (campaign, kind). URL berubah →
// destination di-update, KODE TETAP.
func (d *Deps) upsertWriterShortLink(ctx context.Context, brandID, campaignID, kind, destinationURL, userID string) (writerShortLink, error) {
	existing, err := d.queryRow(ctx,
		"SELECT id, code FROM tracking_links WHERE campaign_id = $1 AND kind = $2 LIMIT 1", campaignID, kind)
	if err != nil {
		return writerShortLink{}, err
	}
	if existing != nil {
		if err := d.exec(ctx,
			"UPDATE tracking_links SET destination_url = $1, status = 'active', updated_at = NOW() WHERE id = $2",
			destinationURL, existing.strOr("id", "")); err != nil {
			return writerShortLink{}, err
		}
		return writerShortLink{Code: existing.strOr("code", ""), DestinationURL: &destinationURL}, nil
	}

	for attempt := 0; attempt < 8; attempt++ {
		code := randomCode5()
		taken, err := d.queryRow(ctx, "SELECT id FROM tracking_links WHERE code = $1 LIMIT 1", code)
		if err != nil {
			return writerShortLink{}, err
		}
		if taken != nil {
			continue
		}
		createdBy := any(nil)
		if userID != "" {
			createdBy = userID
		}
		err = d.exec(ctx,
			"INSERT INTO tracking_links (id, code, brand_id, campaign_id, kind, destination_url, status, created_by) VALUES ($1, $2, $3, $4, $5, $6, 'active', $7)",
			uuid.NewString(), code, brandID, campaignID, kind, destinationURL, createdBy)
		if err == nil {
			return writerShortLink{Code: code, DestinationURL: &destinationURL}, nil
		}
		// Race collision pada UNIQUE(code) → coba kode lain.
	}
	return writerShortLink{}, errStatus(500, "Gagal menghasilkan kode short link unik.")
}

// getWriterShortLinks: pasangan link writer (primary/backup) untuk campaign.
func (d *Deps) getWriterShortLinks(ctx context.Context, campaignID string) (primary, backup *writerShortLink, err error) {
	rows, err := d.queryRows(ctx,
		"SELECT code, kind, destination_url FROM tracking_links WHERE campaign_id = $1 AND kind IN ('writer_primary','writer_backup')", campaignID)
	if err != nil {
		return nil, nil, err
	}
	pick := func(kind string) *writerShortLink {
		for _, r := range rows {
			if r.strOr("kind", "") != kind {
				continue
			}
			var dest *string
			if r.has("destination_url") {
				s := r.strOr("destination_url", "")
				dest = &s
			}
			return &writerShortLink{Code: r.strOr("code", ""), DestinationURL: dest}
		}
		return nil
	}
	return pick("writer_primary"), pick("writer_backup"), nil
}

// isTrackingCode: true bila kode short link tercatat di tracking_links.
func (d *Deps) isTrackingCode(ctx context.Context, code string) bool {
	c := strings.TrimSpace(code)
	if c == "" || len(c) > 16 {
		return false
	}
	row, err := d.queryRow(ctx, "SELECT id FROM tracking_links WHERE code = $1 LIMIT 1", c)
	return err == nil && row != nil
}

// extractTrackingCode: "https://x/t/abc" → "abc", "abc" → "abc", "https://x/y" → "".
func extractTrackingCode(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	if !strings.Contains(v, "://") && !strings.HasPrefix(v, "/") {
		return strings.SplitN(strings.SplitN(v, "?", 2)[0], "#", 2)[0]
	}
	idx := strings.Index(v, "://")
	rest := v[idx+3:]
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return ""
	}
	path := rest[slash:]
	if !strings.HasPrefix(path, "/t/") {
		return ""
	}
	code := strings.TrimPrefix(path, "/t/")
	code = strings.SplitN(code, "?", 2)[0]
	code = strings.SplitN(code, "/", 2)[0]
	return code
}
