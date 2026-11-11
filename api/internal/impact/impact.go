// Package impact: hitungan klik dari tautan pendek kampanye untuk laporan dampak.
//
// Satu sumber untuk laporan klien (brandflow) dan berita acara publik
// (clientread), supaya angka di dua tempat itu selalu sama.
//
// Yang dihitung, dan batasnya:
//   - Hanya event `tracking_link_click` (klik tautan pendek /t/{code}).
//   - Klik dari bot, pemindai, dan pratinjau tautan (WhatsApp, Slack, Facebook,
//     dst.) dibuang. Pengalihan /t/{code} sengaja tidak menyaring supaya tetap
//     cepat; penyaringan terjadi di sini, saat laporan dibaca.
//   - Satu pengunjung (session_id, atau alamat IP bila tak ada sesi) dihitung
//     sekali per hari dan per kampanye, karena pengalihan 301 bisa di-cache dan
//     refresh tidak boleh menggandakan angka.
//   - Event tanpa campaign_id (tautan publikasi lama, event dari skrip) tidak
//     ikut total kampanye.
//
// Tidak ada data negara, dan klik tidak bisa dipecah per media: tautan Writer
// berlaku per kampanye.
package impact

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MaxEvents: batas baris yang dibaca per laporan. Bila tercapai, Stats.Truncated
// menyala dan tampilan harus menyebutnya.
const MaxEvents = 20000

// wib: hari dihitung menurut waktu Indonesia barat.
var wib = time.FixedZone("WIB", 7*3600)

type Event struct {
	CreatedAt  time.Time
	SessionID  string
	IP         string
	UserAgent  string
	Referrer   string
	CampaignID string
	LinkKind   string
}

type Day struct {
	Day    string `json:"day"`
	Clicks int    `json:"clicks"`
}

type Stats struct {
	// Raw: semua event klik di jangka waktu; Bots: dibuang sebagai bot atau
	// pratinjau; Counted: klik yang dihitung setelah dedupe; Visitors: pengunjung
	// berbeda.
	Raw       int            `json:"raw_events"`
	Bots      int            `json:"bots_filtered"`
	Counted   int            `json:"clicks"`
	Visitors  int            `json:"visitors"`
	Truncated bool           `json:"truncated"`
	Daily     []Day          `json:"daily"`
	Devices   map[string]int `json:"devices"`
	Referrers map[string]int `json:"referrers"`
	ByKind    map[string]int `json:"by_kind"`
}

var botMarkers = []string{
	"bot", "crawler", "spider", "slurp", "preview", "facebookexternalhit", "whatsapp",
	"telegram", "slack", "discord", "skype", "embedly", "curl/", "wget", "python-requests",
	"go-http-client", "okhttp", "headless", "lighthouse", "pingdom", "uptime", "monitor",
	"scanner", "validator", "feedfetcher", "mediapartners", "bingpreview", "yahoo! slurp",
}

// IsBot: user agent kosong atau penanda bot dan pratinjau tautan yang umum.
func IsBot(ua string) bool {
	s := strings.ToLower(strings.TrimSpace(ua))
	if s == "" {
		return true
	}
	for _, m := range botMarkers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

func device(ua string) string {
	s := strings.ToLower(ua)
	switch {
	case strings.Contains(s, "ipad") || strings.Contains(s, "tablet"):
		return "tablet"
	case strings.Contains(s, "mobi") || strings.Contains(s, "android") || strings.Contains(s, "iphone"):
		return "mobile"
	}
	return "desktop"
}

func referrerGroup(ref string) string {
	r := strings.ToLower(ref)
	if r == "" {
		return "Direct"
	}
	for _, h := range []string{"google", "bing", "yahoo", "duckduckgo", "yandex", "baidu", "ecosia"} {
		if strings.Contains(r, h) {
			return "Search"
		}
	}
	for _, h := range []string{"facebook", "instagram", "twitter", "x.com", "t.co", "linkedin", "tiktok", "whatsapp", "wa.me", "youtube", "pinterest", "reddit", "telegram"} {
		if strings.Contains(r, h) {
			return "Social"
		}
	}
	return "Referral"
}

// Aggregate: hitung statistik untuk `days` hari terakhir (termasuk hari ini).
func Aggregate(events []Event, days int, now time.Time) Stats {
	if days < 1 {
		days = 30
	}
	st := Stats{
		Devices: map[string]int{}, Referrers: map[string]int{}, ByKind: map[string]int{},
		Daily: make([]Day, 0, days),
	}
	perDay := make(map[string]int, days)
	seen := make(map[string]bool, len(events))
	visitors := make(map[string]bool, len(events))
	for i, e := range events {
		st.Raw++
		if IsBot(e.UserAgent) {
			st.Bots++
			continue
		}
		day := e.CreatedAt.In(wib).Format("2006-01-02")
		who := e.SessionID
		if who == "" {
			who = e.IP
		}
		if who == "" {
			who = "event-" + itoa(i) // tanpa identitas: tiap event dihitung sendiri
		}
		key := who + "|" + day + "|" + e.CampaignID
		if seen[key] {
			continue
		}
		seen[key] = true
		visitors[who] = true
		st.Counted++
		perDay[day]++
		st.Devices[device(e.UserAgent)]++
		st.Referrers[referrerGroup(e.Referrer)]++
		if e.LinkKind != "" {
			st.ByKind[e.LinkKind]++
		}
	}
	st.Visitors = len(visitors)
	for i := days - 1; i >= 0; i-- {
		d := now.In(wib).AddDate(0, 0, -i).Format("2006-01-02")
		st.Daily = append(st.Daily, Day{Day: d, Clicks: perDay[d]})
	}
	return st
}

// Load: baca event klik sebuah brand sejak `since`, opsional untuk satu kampanye.
// Mengembalikan true bila hasil terpotong di MaxEvents.
func Load(ctx context.Context, pool *pgxpool.Pool, brandID, campaignID string, since time.Time) ([]Event, bool, error) {
	args := []any{brandID, since}
	where := ""
	if campaignID != "" {
		args = append(args, campaignID)
		where = " AND e.campaign_id = $3"
	}
	rows, err := pool.Query(ctx, `
		SELECT e.created_at, COALESCE(e.session_id::text, ''), COALESCE(e.ip_address::text, ''),
		       COALESCE(e.user_agent, ''), COALESCE(e.referrer, ''), COALESCE(e.campaign_id::text, ''),
		       COALESCE(l.kind, '')
		FROM tracking_events e
		LEFT JOIN tracking_links l ON l.id = e.link_id
		WHERE e.event_type = 'tracking_link_click' AND e.brand_id = $1 AND e.created_at >= $2`+where+`
		ORDER BY e.created_at DESC
		LIMIT `+itoa(MaxEvents+1), args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := make([]Event, 0, 256)
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.CreatedAt, &e.SessionID, &e.IP, &e.UserAgent, &e.Referrer, &e.CampaignID, &e.LinkKind); err != nil {
			return nil, false, err
		}
		e.CampaignID = strings.TrimSpace(e.CampaignID)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	truncated := len(out) > MaxEvents
	if truncated {
		out = out[:MaxEvents]
	}
	return out, truncated, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
