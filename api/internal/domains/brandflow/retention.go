package brandflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// retention.go: pangkas data yang tumbuh tanpa batas supaya database tidak
// membengkak. Dibagi dua kategori:
//
//   - AMAN (log/cache internal, tidak terlihat klien) → dihapus langsung.
//   - BERISIKO (terlihat user / dipakai analitik & kepatuhan) → retensi lebih
//     panjang, dan SEBELUM dihapus barisnya diarsipkan ke file JSONL supaya
//     masih bisa dibaca kalau benar-benar dibutuhkan.
//
// Ambang waktunya hidup di RetentionConfig: DefaultRetentionConfig() dipakai
// jadwal harian, dan endpoint admin POST /v1/platform/maintenance/retention
// bisa mengirim override + menjalankan manual.

type RetentionConfig struct {
	// AMAN — log/cache internal.
	AgentRunsDays       int // riwayat panggilan AI per agent (input+output penuh)
	TrainingSamplesDays int // salinan input/output AI untuk latihan
	UserSessionDays     int // sesi login kedaluwarsa/lama
	JobsDoneDays        int // job antrean selesai/gagal
	AuthRateLimitDays   int // bucket rate-limit kadaluarsa

	// BERISIKO — dipakai user / analitik / kepatuhan.
	NotificationDays   int // notifikasi yang SUDAH DIBACA
	EmailDeliveryDays  int // log email (termasuk html-nya)
	TrackingEventDays  int // klik/open (sumber laporan)
	AuditLogDays       int // jejak audit
	ContentVersionKeep int // jumlah versi artikel yang disimpan per dokumen

	// Umum
	BatchSize  int    // baris per batch (jaga lock & memori tetap pendek)
	ArchiveDir string // folder arsip JSONL; kosong = hapus tanpa arsip
}

// DefaultRetentionConfig: ambang yang dipakai jadwal harian.
func DefaultRetentionConfig() RetentionConfig {
	return RetentionConfig{
		AgentRunsDays:       30,
		TrainingSamplesDays: 30,
		UserSessionDays:     30,
		JobsDoneDays:        14,
		AuthRateLimitDays:   7,
		NotificationDays:    90,
		EmailDeliveryDays:   90,
		TrackingEventDays:   180,
		AuditLogDays:        365,
		ContentVersionKeep:  10,
		BatchSize:           500,
		ArchiveDir:          defaultArchiveDir(),
	}
}

// defaultArchiveDir: folder arsip JSONL. Di VPS hidup di .data/archive —
// dikecualikan dari rsync --delete, jadi tidak ikut terhapus saat deploy.
func defaultArchiveDir() string {
	if v := strings.TrimSpace(os.Getenv("RETENTION_ARCHIVE_DIR")); v != "" {
		return v
	}
	return "./.data/archive"
}

// RetentionReport: jumlah baris per tabel (dihapus) + total arsip.
type RetentionReport struct {
	AgentRuns       int64 `json:"taut_agent_runs"`
	TrainingSamples int64 `json:"taut_training_samples"`
	UserSessions    int64 `json:"user_sessions"`
	Jobs            int64 `json:"jobs"`
	AuthRateLimits  int64 `json:"auth_rate_limits"`
	Notifications   int64 `json:"notifications"`
	EmailDeliveries int64 `json:"email_deliveries"`
	TrackingEvents  int64 `json:"tracking_events"`
	AuditLogs       int64 `json:"audit_logs"`
	ContentVersions int64 `json:"content_versions"`
	Deleted         int64 `json:"deleted"`
	Archived        int64 `json:"archived"`
	DurationMS      int64 `json:"duration_ms"`
}

// RunRetention: jalankan pemangkasan semua tabel sesuai cfg. Idempoten —
// baris yang tidak memenuhi ambang tidak tersentuh.
func RunRetention(ctx context.Context, pool *pgxpool.Pool, cfg RetentionConfig) (*RetentionReport, error) {
	started := time.Now()
	if cfg.BatchSize < 1 {
		cfg.BatchSize = 500
	}
	rep := &RetentionReport{}

	// ---------- AMAN (hapus langsung, per batch supaya lock pendek) ----------
	safe := []struct {
		table string
		cond  string
		days  int
		dest  *int64
	}{
		{"taut_agent_runs", "created_at < NOW() - ($1 * INTERVAL '1 day')", cfg.AgentRunsDays, &rep.AgentRuns},
		{"taut_training_samples", "created_at < NOW() - ($1 * INTERVAL '1 day')", cfg.TrainingSamplesDays, &rep.TrainingSamples},
		{"user_sessions", "(expires_at < NOW() OR created_at < NOW() - ($1 * INTERVAL '1 day'))", cfg.UserSessionDays, &rep.UserSessions},
		{"jobs", "(status IN ('completed','failed') AND updated_at < NOW() - ($1 * INTERVAL '1 day'))", cfg.JobsDoneDays, &rep.Jobs},
		{"auth_rate_limits", "window_start < NOW() - ($1 * INTERVAL '1 day')", cfg.AuthRateLimitDays, &rep.AuthRateLimits},
	}
	for _, s := range safe {
		n, err := retentionPurge(ctx, pool, s.table, s.cond, s.days, cfg.BatchSize)
		if err != nil {
			return rep, err
		}
		*s.dest = n
		rep.Deleted += n
	}

	// ---------- BERISIKO (arsip JSONL dulu, lalu hapus) ----------
	risky := []struct {
		table string
		cond  string
		days  int
		dest  *int64
	}{
		{"notifications", "read_at IS NOT NULL AND read_at < NOW() - ($1 * INTERVAL '1 day') AND id > $2", cfg.NotificationDays, &rep.Notifications},
		{"email_deliveries", "created_at < NOW() - ($1 * INTERVAL '1 day') AND id > $2", cfg.EmailDeliveryDays, &rep.EmailDeliveries},
		{"tracking_events", "created_at < NOW() - ($1 * INTERVAL '1 day') AND id > $2", cfg.TrackingEventDays, &rep.TrackingEvents},
		{"audit_logs", "created_at < NOW() - ($1 * INTERVAL '1 day') AND id > $2", cfg.AuditLogDays, &rep.AuditLogs},
		// Versi artikel: simpan ContentVersionKeep versi terakhir per dokumen.
		{"content_versions", "id IN (SELECT id FROM (SELECT id, row_number() OVER (PARTITION BY document_id ORDER BY version DESC) rn FROM content_versions) s WHERE s.rn > $1) AND id > $2", cfg.ContentVersionKeep, &rep.ContentVersions},
	}
	for _, s := range risky {
		arch, del, err := retentionArchiveAndPurge(ctx, pool, s.table, s.cond, s.days, cfg)
		if err != nil {
			return rep, err
		}
		*s.dest = del
		rep.Deleted += del
		rep.Archived += arch
	}

	rep.DurationMS = time.Since(started).Milliseconds()
	return rep, nil
}

// retentionPurge: hapus baris yang memenuhi kondisi, per batch (ctid) supaya
// lock dan memori tetap pendek. Kondisi memakai $1 = ambang hari.
func retentionPurge(ctx context.Context, pool *pgxpool.Pool, table, cond string, days, batch int) (int64, error) {
	var total int64
	for {
		tag, err := pool.Exec(ctx, fmt.Sprintf(
			`DELETE FROM %s WHERE ctid IN (SELECT ctid FROM %s WHERE %s LIMIT %d)`,
			table, table, cond, batch), days)
		if err != nil {
			return total, err
		}
		n := tag.RowsAffected()
		total += n
		if n < int64(batch) {
			return total, nil
		}
	}
}

// retentionArchiveAndPurge: salin baris yang akan dihapus ke file JSONL
// (.data/archive/<tabel>/<tanggal>.jsonl) per batch, lalu hapus berdasarkan id.
// Kondisi memakai $1 = ambang hari dan $2 = id terakhir (untuk paging).
func retentionArchiveAndPurge(ctx context.Context, pool *pgxpool.Pool, table, cond string, days int, cfg RetentionConfig) (int64, int64, error) {
	var archived, deleted int64
	var w *os.File
	if strings.TrimSpace(cfg.ArchiveDir) != "" {
		dir := filepath.Join(cfg.ArchiveDir, table)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 0, 0, err
		}
		f, err := os.OpenFile(filepath.Join(dir, time.Now().UTC().Format("2006-01-02")+".jsonl"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return 0, 0, err
		}
		defer f.Close()
		w = f
	}

	var lastID string
	for {
		rows, err := pool.Query(ctx, fmt.Sprintf(
			`SELECT t.id::text, row_to_json(t) FROM %s t WHERE %s ORDER BY t.id LIMIT %d`,
			table, cond, cfg.BatchSize),
			append([]any{days}, lastID)...)
		if err != nil {
			return archived, deleted, err
		}
		var ids []string
		for rows.Next() {
			var id string
			var raw []byte
			if err := rows.Scan(&id, &raw); err != nil {
				rows.Close()
				return archived, deleted, err
			}
			if w != nil {
				if _, err := w.Write(append(raw, '\n')); err != nil {
					rows.Close()
					return archived, deleted, err
				}
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return archived, deleted, err
		}
		if len(ids) == 0 {
			return archived, deleted, nil
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = ANY($1)`, table), ids); err != nil {
			return archived, deleted, err
		}
		archived += int64(len(ids))
		deleted += int64(len(ids))
		lastID = ids[len(ids)-1]
		if len(ids) < cfg.BatchSize {
			return archived, deleted, nil
		}
	}
}
