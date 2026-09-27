package brandflow

import (
	"context"
	"fmt"

	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
)

// RegisterJobs: daftarkan seluruh processor job milik paket brandflow ke
// dispatcher bersama. Dipanggil sekali dari cmd/api setelah Deps punya Pool.
func (d *Deps) RegisterJobs() {
	jobs.Register("publication_execution", func(ctx context.Context, job jobs.JobRow) error {
		pubID, _ := job.Payload["publicationId"].(string)
		if pubID == "" {
			return fmt.Errorf("payload publicationId kosong")
		}
		return d.PublicationExecutionJob(ctx, pubID, job.Attempts, job.MaxAttempts)
	})
	jobs.Register("publication_variant_generation", func(ctx context.Context, job jobs.JobRow) error {
		return d.PublicationVariantGenerationJob(ctx, job.Payload)
	})
	jobs.Register("publication_link_check", func(ctx context.Context, job jobs.JobRow) error {
		pubID, _ := job.Payload["publicationId"].(string)
		if pubID == "" {
			return fmt.Errorf("payload publicationId kosong")
		}
		return d.PublicationLinkCheckJob(ctx, pubID)
	})
}
