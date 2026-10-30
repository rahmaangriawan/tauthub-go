package summary

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
)

// RegisterJobs mendaftarkan processor job ringkasan brief ke dispatcher
// bersama. Dipanggil dari jobwire.Register.
func RegisterJobs(pool *pgxpool.Pool, cfg *config.Config) {
	jobs.Register(JobKind, func(ctx context.Context, job jobs.JobRow) error {
		profileID, _ := job.Payload["profileId"].(string)
		brandID, _ := job.Payload["brandId"].(string)
		kindRaw, _ := job.Payload["kind"].(string)
		force, _ := job.Payload["force"].(bool)
		kind, ok := ParseKind(kindRaw)
		if profileID == "" || !ok {
			return fmt.Errorf("payload brief_summary tidak lengkap")
		}
		// Brief sudah dihapus: tidak ada yang perlu dikerjakan. Brand diambil
		// dari baris profil (sumber kebenaran) — payload job tidak dipercaya
		// untuk menentukan tenant, dan ketidakcocokan ditolak.
		//
		// btrim wajib: profiler_profiles.brand_id bertipe character(36) (bpchar),
		// sehingga driver mengembalikan nilai yang dipad spasi sampai 36 karakter
		// ("cza87b" + 30 spasi). Tanpa btrim, perbandingan dengan public_id di
		// payload selalu gagal dan setiap job ringkasan ditolak sebagai
		// "brand tidak cocok", membuat ringkasan tidak pernah bisa diperbarui.
		var profileBrand string
		if err := pool.QueryRow(ctx,
			`SELECT btrim(brand_id::text) FROM profiler_profiles WHERE id = $1 AND deleted_at IS NULL`, profileID).
			Scan(&profileBrand); err != nil || profileBrand == "" {
			return nil
		}
		if brandID != "" && strings.TrimSpace(brandID) != profileBrand {
			// Tandai baris ringkasan gagal, bukan cuma mengembalikan error job.
			// Tanpa ini statusnya tertinggal 'queued' dan panel klien berputar
			// "Sedang dibuat…" sampai sapuan UnstickStale 5 menit kemudian,
			// tanpa pernah menjelaskan apa yang salah.
			msg := "payload brand tidak cocok dengan profil brief"
			_ = setStatus(ctx, pool, profileID, kind, "failed", msg)
			return fmt.Errorf("%s", msg)
		}
		return Generate(ctx, pool, cfg, profileID, profileBrand, kind, force)
	})
}
