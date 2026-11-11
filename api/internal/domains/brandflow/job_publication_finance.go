package brandflow

import "context"

// PublicationFinanceJob: melengkapi pembukuan publikasi yang ditandai terbit
// lewat jalur manual mitra (POST /partner/publications/{id}/url). Jalur itu
// hanya mengubah status dan menagih kredit; tanggal terbit, transaksi, dan
// penghasilan mitra dikerjakan di sini dengan rumus yang sama dengan jalur
// otomatis (peRecordFinance, yang menolak pencatatan ganda per publikasi).
//
// published_at memakai waktu percobaan terbit bila ada, supaya data lama yang
// di-backfill tidak bertanggal hari ini.
func (d *Deps) PublicationFinanceJob(ctx context.Context, publicationID string) error {
	publication, err := d.queryRow(ctx, "SELECT * FROM publications WHERE id = $1 LIMIT 1", publicationID)
	if err != nil {
		return err
	}
	if publication == nil || publication.strOr("status", "") != "published" {
		return nil
	}
	if err := d.exec(ctx, `
		UPDATE publications
		   SET published_at = COALESCE(
		         (SELECT MAX(completed_at) FROM publication_attempts
		           WHERE publication_id = $1 AND status = 'published'), NOW())
		 WHERE id = $1 AND published_at IS NULL`, publicationID); err != nil {
		return err
	}
	media, err := d.queryRow(ctx, "SELECT * FROM media WHERE id = $1 LIMIT 1", publication.strOr("media_id", ""))
	if err != nil {
		return err
	}
	if media == nil {
		return nil
	}
	d.peRecordFinance(ctx, publication, media)
	return nil
}
