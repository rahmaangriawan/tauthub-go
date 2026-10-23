package summary

import (
	"context"
	"fmt"

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
		// Brief sudah dihapus: tidak ada yang perlu dikerjakan.
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT true FROM profiler_profiles WHERE id = $1 AND deleted_at IS NULL`, profileID).
			Scan(&exists); err != nil || !exists {
			return nil
		}
		return Generate(ctx, pool, cfg, profileID, brandID, kind, force)
	})
}
