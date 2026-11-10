// Package push: langganan Web Push per pengguna dan pengiriman antrean
// push_deliveries. Antrean diisi otomatis oleh trigger DB saat baris
// `notifications` dibuat, jadi 21 titik pembuat notifikasi tidak disentuh.
package push

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
	"github.com/rahmaangriawan/tauthub-go/internal/webpush"
)

type Deps struct {
	Pool *pgxpool.Pool
	Cfg  *config.Config

	client *webpush.Client
}

// New membangun Deps; client nil bila VAPID belum dikonfigurasi (fitur nonaktif,
// tidak fail-fast agar API tetap jalan tanpa push).
func New(pool *pgxpool.Pool, cfg *config.Config) *Deps {
	d := &Deps{Pool: pool, Cfg: cfg}
	if cfg.VAPIDPrivateKey == "" {
		slog.Warn("[push] VAPID_PRIVATE_KEY kosong — Web Push nonaktif")
		return d
	}
	// Endpoint langganan diisi pengguna: pakai SafeClient (tolak IP privat/loopback
	// saat dial) dan jangan ikuti redirect.
	hc := security.SafeClient(15 * time.Second)
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	c, err := webpush.NewClient(cfg.VAPIDPrivateKey, cfg.VAPIDSubject, hc)
	if err != nil {
		slog.Error("[push] kunci VAPID tidak valid — Web Push nonaktif", "err", err)
		return d
	}
	d.client = c
	return d
}

func (d *Deps) enabled() bool { return d.client != nil }

// Enabled: true bila VAPID terkonfigurasi sehingga antrean benar-benar dikirim.
func (d *Deps) Enabled() bool { return d.enabled() }

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// Config: GET /v1/push/config — kunci publik VAPID + status fitur + apakah
// pengguna ini sudah punya langganan.
func (d *Deps) Config(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if !d.enabled() {
		writeJSON(w, http.StatusOK, map[string]any{"enabled": false, "public_key": "", "subscriptions": 0})
		return
	}
	var count int
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT COUNT(*) FROM push_subscriptions WHERE user_id = $1", user.ID).Scan(&count)
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":       true,
		"public_key":    d.client.PublicKey(),
		"subscriptions": count,
	})
}

type subscribeBody struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Subscribe: POST /v1/push/subscriptions — simpan/perbarui langganan perangkat
// ini. Endpoint unik global: kalau sudah ada milik user lain (perangkat
// bersama, ganti akun), dipindahkan ke user yang login sekarang.
func (d *Deps) Subscribe(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if !d.enabled() {
		httpx.Error(w, http.StatusServiceUnavailable, "Web Push belum dikonfigurasi.")
		return
	}
	var body subscribeBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Body tidak valid")
		return
	}
	endpoint := strings.TrimSpace(body.Endpoint)
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || len(endpoint) > 2048 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Endpoint langganan tidak valid")
		return
	}
	p256dh := strings.TrimSpace(body.Keys.P256dh)
	auth := strings.TrimSpace(body.Keys.Auth)
	if len(p256dh) < 80 || len(p256dh) > 120 || len(auth) < 16 || len(auth) > 32 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Kunci langganan tidak valid")
		return
	}
	ua := r.UserAgent()
	if len(ua) > 255 {
		ua = ua[:255]
	}
	var id string
	err = d.Pool.QueryRow(r.Context(), `
		INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, user_agent, last_used_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (endpoint) DO UPDATE
		  SET user_id = EXCLUDED.user_id, p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth,
		      user_agent = EXCLUDED.user_agent, last_used_at = NOW()
		RETURNING id`, user.ID, endpoint, p256dh, auth, ua).Scan(&id)
	if err != nil {
		slog.Error("[push] simpan langganan", "err", err)
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
}

// Unsubscribe: DELETE /v1/push/subscriptions — hapus langganan by endpoint
// (hanya milik user login). Idempoten.
func (d *Deps) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil || strings.TrimSpace(body.Endpoint) == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Endpoint wajib diisi")
		return
	}
	tag, err := d.Pool.Exec(r.Context(),
		"DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2", user.ID, strings.TrimSpace(body.Endpoint))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "removed": tag.RowsAffected()})
}

// Test: POST /v1/push/test — kirim push uji ke semua perangkat user login,
// tanpa membuat baris notifications (supaya lonceng tidak ikut terisi).
func (d *Deps) Test(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if !d.enabled() {
		httpx.Error(w, http.StatusServiceUnavailable, "Web Push belum dikonfigurasi.")
		return
	}
	rows, err := d.Pool.Query(r.Context(),
		"SELECT id, endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = $1", user.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, httpx.Scrub500(d.Cfg.AppEnv, err.Error()))
		return
	}
	defer rows.Close()
	type sub struct {
		id string
		s  webpush.Subscription
	}
	var subs []sub
	for rows.Next() {
		var x sub
		if err := rows.Scan(&x.id, &x.s.Endpoint, &x.s.P256dh, &x.s.Auth); err == nil {
			subs = append(subs, x)
		}
	}
	payload, _ := json.Marshal(map[string]any{
		"title": "Notifikasi perangkat aktif",
		"body":  "Tauthub akan mengabari Anda di perangkat ini.",
		"url":   "/",
		"tag":   "push-test",
	})
	sent, gone := 0, 0
	for _, x := range subs {
		res, err := d.client.Send(r.Context(), x.s, payload, webpush.Options{TTL: 300, Urgency: "high"})
		switch {
		case err != nil:
			slog.Warn("[push] uji kirim gagal", "err", err)
		case res.Gone:
			gone++
			_, _ = d.Pool.Exec(r.Context(), "DELETE FROM push_subscriptions WHERE id = $1", x.id)
		case res.StatusCode >= 200 && res.StatusCode < 300:
			sent++
		default:
			slog.Warn("[push] uji kirim ditolak", "status", res.StatusCode, "body", res.Body)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sent": sent, "removed": gone, "devices": len(subs)})
}

// ---------- antrean pengiriman (dipakai worker) ----------

type delivery struct {
	ID           string
	Attempts     int
	MaxAttempts  int
	SubID        string
	Sub          webpush.Subscription
	NotifType    string
	Title        string
	Body         string
	Link         string
	NotifCreated time.Time
}

// claimBatch: ambil hingga limit antrean yang jatuh tempo (FOR UPDATE SKIP
// LOCKED) dan tandai running — aman dijalankan oleh beberapa worker sekaligus.
func (d *Deps) claimBatch(ctx context.Context, workerID string, limit int) ([]delivery, error) {
	rows, err := d.Pool.Query(ctx, `
		WITH picked AS (
		  SELECT id FROM push_deliveries
		  WHERE status IN ('pending','failed') AND run_at <= NOW()
		  ORDER BY run_at
		  LIMIT $2
		  FOR UPDATE SKIP LOCKED
		), upd AS (
		  UPDATE push_deliveries pd SET status = 'running', locked_at = NOW(), locked_by = $1,
		         attempts = pd.attempts + 1, updated_at = NOW()
		  WHERE pd.id IN (SELECT id FROM picked)
		  RETURNING pd.id, pd.attempts, pd.max_attempts, pd.subscription_id, pd.notification_id
		)
		SELECT u.id, u.attempts, u.max_attempts, s.id, s.endpoint, s.p256dh, s.auth,
		       n.type, n.title, COALESCE(n.body, ''), COALESCE(n.link, ''), n.created_at
		FROM upd u
		JOIN push_subscriptions s ON s.id = u.subscription_id
		JOIN notifications n ON n.id = u.notification_id`, workerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []delivery
	for rows.Next() {
		var x delivery
		if err := rows.Scan(&x.ID, &x.Attempts, &x.MaxAttempts, &x.SubID, &x.Sub.Endpoint, &x.Sub.P256dh, &x.Sub.Auth,
			&x.NotifType, &x.Title, &x.Body, &x.Link, &x.NotifCreated); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (d *Deps) markDone(ctx context.Context, id string) {
	_, _ = d.Pool.Exec(ctx,
		"UPDATE push_deliveries SET status='sent', locked_at=NULL, locked_by=NULL, updated_at=NOW() WHERE id=$1", id)
	_, _ = d.Pool.Exec(ctx,
		"UPDATE push_subscriptions SET last_used_at = NOW() WHERE id = (SELECT subscription_id FROM push_deliveries WHERE id=$1)", id)
}

func (d *Deps) markSkipped(ctx context.Context, id, reason string) {
	_, _ = d.Pool.Exec(ctx,
		"UPDATE push_deliveries SET status='skipped', last_error=$2, locked_at=NULL, locked_by=NULL, updated_at=NOW() WHERE id=$1", id, reason)
}

func (d *Deps) markFailed(ctx context.Context, x delivery, msg string, retryAfter time.Duration) {
	status := "pending"
	if x.Attempts >= x.MaxAttempts {
		status = "failed_final"
	}
	if len(msg) > 500 {
		msg = msg[:500]
	}
	wait := time.Duration(1<<uint(min(x.Attempts, 5))) * 15 * time.Second
	if retryAfter > wait {
		wait = retryAfter
	}
	_, _ = d.Pool.Exec(ctx,
		"UPDATE push_deliveries SET status=$1, last_error=$2, run_at=$3, locked_at=NULL, locked_by=NULL, updated_at=NOW() WHERE id=$4",
		status, msg, time.Now().Add(wait), x.ID)
}

// dropSubscription: endpoint sudah mati (404/410) → hapus langganan; antrean
// lain untuk langganan itu ikut hilang lewat ON DELETE CASCADE.
func (d *Deps) dropSubscription(ctx context.Context, subID string) {
	_, _ = d.Pool.Exec(ctx, "DELETE FROM push_subscriptions WHERE id=$1", subID)
}

// ProcessBatch: kirim satu batch antrean dengan konkurensi terbatas.
// Mengembalikan jumlah antrean yang diklaim (0 = kosong).
func (d *Deps) ProcessBatch(ctx context.Context, workerID string, batch int) (int, error) {
	if !d.enabled() {
		return 0, nil
	}
	items, err := d.claimBatch(ctx, workerID, batch)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}
	sem := make(chan struct{}, d.Cfg.PushWorkers)
	done := make(chan struct{})
	for _, x := range items {
		sem <- struct{}{}
		go func(x delivery) {
			defer func() { <-sem; done <- struct{}{} }()
			d.deliverOne(ctx, x)
		}(x)
	}
	for range items {
		<-done
	}
	return len(items), nil
}

func (d *Deps) deliverOne(ctx context.Context, x delivery) {
	// Notifikasi basi (mis. antrean menumpuk lama) tidak layak didorong ke layar.
	if time.Since(x.NotifCreated) > 48*time.Hour {
		d.markSkipped(ctx, x.ID, "notifikasi lebih dari 48 jam")
		return
	}
	body := x.Body
	if len(body) > 180 {
		body = body[:177] + "…"
	}
	payload, _ := json.Marshal(map[string]any{
		"title": x.Title,
		"body":  body,
		"url":   x.Link,
		"tag":   x.NotifType,
	})
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	res, err := d.client.Send(cctx, x.Sub, payload, webpush.Options{TTL: 12 * 3600, Urgency: urgencyFor(x.NotifType)})
	switch {
	case err != nil:
		d.markFailed(ctx, x, err.Error(), 0)
	case res.Gone:
		d.dropSubscription(ctx, x.SubID)
	case res.StatusCode >= 200 && res.StatusCode < 300:
		d.markDone(ctx, x.ID)
	case res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500:
		d.markFailed(ctx, x, "push server "+http.StatusText(res.StatusCode), res.RetryAfter)
	default:
		// 400/401/403: payload/VAPID ditolak — tidak akan sembuh dengan retry.
		d.markSkipped(ctx, x.ID, "ditolak "+http.StatusText(res.StatusCode)+": "+res.Body)
	}
}

// urgencyFor: pengumuman & pembayaran mendesak; sisanya normal supaya OS
// mobile boleh menunda saat hemat daya.
func urgencyFor(notifType string) string {
	switch {
	case strings.HasPrefix(notifType, "withdrawal."), strings.HasPrefix(notifType, "announcement."),
		notifType == "publication.revision_requested", notifType == "publication.failed":
		return "high"
	}
	return "normal"
}
