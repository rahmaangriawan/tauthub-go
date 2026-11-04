package brandflow

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

// Platform website media (media.website_platform): wordpress | blogspot | custom.
// Dideteksi dari halaman depan situs, terpisah dari integration_type (cara
// artikel dikirim). Kolom di-reset NULL oleh trigger saat website_url berubah,
// jadi sweep ini sekaligus backfill dan deteksi ulang.
const (
	sitePlatformRecheckAfter = 6 * time.Hour
	sitePlatformFetchTimeout = 15 * time.Second
	sitePlatformUserAgent    = "Mozilla/5.0 (compatible; TauthubBot/1.0; +https://tauthub.com)"
)

// detectSitePlatform: "" bila halaman tak bisa dibaca (jangan ditebak).
func detectSitePlatform(ctx context.Context, websiteURL string) string {
	u := strings.TrimSpace(websiteURL)
	if u == "" {
		return ""
	}
	if !strings.Contains(u, "://") {
		u = "https://" + u
	}
	status, body, err := bsHTTPDo(ctx, "GET", u, map[string]string{"User-Agent": sitePlatformUserAgent, "Accept": "text/html"}, "", sitePlatformFetchTimeout)
	if err != nil || status < 200 || status >= 400 {
		return ""
	}
	return classifySitePlatform(body, func(path string) (int, string) {
		st, b, e := bsHTTPDo(ctx, "GET", strings.TrimRight(u, "/")+path, map[string]string{"User-Agent": sitePlatformUserAgent}, "", sitePlatformFetchTimeout)
		if e != nil {
			return 0, ""
		}
		return st, b
	})
}

// classifySitePlatform: Blogger lebih dulu karena tema Blogger kadang memuat
// satu referensi wp-content (mis. gambar/embed), sedangkan penanda Blogger
// (generator, b:version, blogger.com/static) tidak muncul di situs WordPress.
func classifySitePlatform(html string, probe func(path string) (int, string)) string {
	low := strings.ToLower(html)
	if strings.Contains(low, "content='blogger'") || strings.Contains(low, `content="blogger"`) ||
		strings.Contains(low, "b:version") || strings.Contains(low, "blogger.com/static") ||
		strings.Contains(low, "blogger.com/feeds") || strings.Contains(low, "www.blogger.com/") ||
		strings.Contains(low, ".blogspot.com") {
		return "blogspot"
	}
	if strings.Contains(low, `content="wordpress`) || strings.Contains(low, "content='wordpress") ||
		strings.Contains(low, "/wp-content/") || strings.Contains(low, "/wp-includes/") {
		return "wordpress"
	}
	if probe != nil {
		if st, b := probe("/wp-json/"); st == 200 && strings.Contains(b, `"namespaces"`) {
			return "wordpress"
		}
	}
	return "custom"
}

// schSweepSitePlatforms: mendeteksi platform media yang belum diketahui.
func (d *Deps) schSweepSitePlatforms(ctx context.Context, limit int) int {
	if limit < 1 {
		return 0
	}
	rows, err := d.queryRows(ctx, `
		SELECT id, website_url FROM media
		 WHERE website_platform IS NULL AND COALESCE(TRIM(website_url), '') <> ''
		   AND (website_platform_checked_at IS NULL OR website_platform_checked_at < NOW() - make_interval(secs => $1))
		 ORDER BY website_platform_checked_at NULLS FIRST, created_at
		 LIMIT $2`, sitePlatformRecheckAfter.Seconds(), limit)
	if err != nil {
		slog.Error("[scheduler] sweep platform website gagal", "err", err)
		return 0
	}
	detected := 0
	for _, r := range rows {
		id := r.strOr("id", "")
		platform := detectSitePlatform(ctx, r.strOr("website_url", ""))
		var execErr error
		if platform == "" {
			execErr = d.exec(ctx, `UPDATE media SET website_platform_checked_at = NOW() WHERE id = $1`, id)
		} else {
			execErr = d.exec(ctx, `UPDATE media SET website_platform = $2, website_platform_checked_at = NOW() WHERE id = $1`, id, platform)
			detected++
		}
		if execErr != nil {
			slog.Error("[scheduler] simpan platform website gagal", "media", id, "err", execErr)
		}
	}
	if detected > 0 {
		slog.Info("[scheduler] platform website terdeteksi", "jumlah", detected)
	}
	return detected
}
