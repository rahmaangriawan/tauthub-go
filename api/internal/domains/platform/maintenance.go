package platform

import (
	"net/http"
	"strconv"
	"strings"

	brandflow "github.com/rahmaangriawan/tauthub-go/internal/domains/brandflow"
)

// maintenance.go: perawatan database tingkat platform.

// MaintenanceRetention: POST /v1/platform/maintenance/retention (super admin).
// Menjalankan pemangkasan retensi sekarang. Body opsional mengoverride ambang
// hari (field-nya satu-satu, lihat brandflow.RetentionConfig) — tanpa body
// berarti memakai ambang default yang dipakai jadwal harian. Respons memuat
// jumlah baris yang dihapus per tabel + total yang diarsipkan.
func (d *Deps) MaintenanceRetention(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	cfg := brandflow.DefaultRetentionConfig()
	body := decodeJSON(r)
	applyRetentionOverrides(&cfg, body)

	rep, err := brandflow.RunRetention(r.Context(), d.Pool, cfg)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "maintenance.retention", "retention",
		"deleted:"+strconv.FormatInt(rep.Deleted, 10)+", archived:"+strconv.FormatInt(rep.Archived, 10))
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "report": rep, "config": cfg})
}

// applyRetentionOverrides: terapkan override ambang dari body admin. Nilai di
// luar rentang wajar diabaikan (bukan error) supaya salah ketik tidak bisa
// membuat retensi jadi agresif.
func applyRetentionOverrides(cfg *brandflow.RetentionConfig, body map[string]any) {
	day := func(dst *int, key string, min, max int) {
		if n, ok := intField(body, key); ok && n >= min && n <= max {
			*dst = n
		}
	}
	day(&cfg.AgentRunsDays, "agent_runs_days", 1, 365)
	day(&cfg.TrainingSamplesDays, "training_samples_days", 1, 365)
	day(&cfg.UserSessionDays, "user_session_days", 1, 365)
	day(&cfg.JobsDoneDays, "jobs_done_days", 1, 365)
	day(&cfg.AuthRateLimitDays, "auth_rate_limit_days", 1, 365)
	day(&cfg.NotificationDays, "notification_days", 1, 3650)
	day(&cfg.EmailDeliveryDays, "email_delivery_days", 1, 3650)
	day(&cfg.TrackingEventDays, "tracking_event_days", 1, 3650)
	day(&cfg.AuditLogDays, "audit_log_days", 30, 3650)
	day(&cfg.ContentVersionKeep, "content_version_keep", 1, 100)
	if s, ok := strField(body, "archive_dir"); ok && strings.TrimSpace(s) != "" {
		cfg.ArchiveDir = strings.TrimSpace(s)
	} else if ok {
		// eksplisit kosong = tanpa arsip (hard delete)
		cfg.ArchiveDir = ""
	}
	if n, ok := intField(body, "batch_size"); ok && n >= 50 && n <= 5000 {
		cfg.BatchSize = n
	}
}
