package clienttaut

// support_shared.go: helper bersama yang dipakai lintas modul Taut klien —
// port kecil dari pola app lama yang belum terdefinisi saat modul dipecah.

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// pgxTx: alias transaksi pgx (dipakai db.WithPoolTx).
type pgxTx = pgx.Tx

// asErr: pembungkus errors.As (parity pola app lama yang ringkas).
func asErr(err error, target any) bool { return errors.As(err, target) }

// rePromptConfigErr: pesan error konfigurasi prompt/agent yang dipetakan ke 422
// (parity pengecekan string di route lama).
var rePromptConfigErr = regexp.MustCompile(`(?i)(prompt|angle|channel|konfigurasi)`)

// strconvFormat: format angka dengan presisi tertentu.
func strconvFormat(f float64, prec int) string {
	return strconv.FormatFloat(f, 'f', prec, 64)
}

// loadTautCampaignRow: ambil campaign milik brand (404 bila bukan miliknya).
func (d *Deps) loadTautCampaignRow(ctx context.Context, campaignID, brandID string) (rowMap, error) {
	row, err := d.queryRow(ctx, `
		SELECT * FROM campaigns WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL LIMIT 1`,
		campaignID, brandID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Kampanye tidak ditemukan")
	}
	return row, nil
}

// requireCampaignInBrand: 404 bila campaign tidak ada atau bukan milik brand.
// queryRow mengembalikan (nil, nil) saat tidak ada baris, jadi hasilnya wajib
// dicek, bukan hanya err.
func (d *Deps) requireCampaignInBrand(ctx context.Context, campaignID, brandID string) error {
	row, err := d.queryRow(ctx,
		"SELECT id FROM campaigns WHERE id = $1 AND brand_id = $2 AND deleted_at IS NULL LIMIT 1", campaignID, brandID)
	if err != nil {
		return err
	}
	if row == nil {
		return errNotFound("Kampanye tidak ditemukan")
	}
	return nil
}

// decodeJSONBody: body JSON → map (kosong bila gagal) — dipakai handler yang
// menerima objek opsional.
func decodeJSONBody(raw []byte) map[string]any {
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

var _ = httpx.Error
