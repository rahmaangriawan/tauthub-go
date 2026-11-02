package partner

import (
	"context"
	"time"

	dbpkg "github.com/rahmaangriawan/tauthub-go/internal/db"
)

// partnerEarningHold: masa tunggu pendapatan mitra sebelum bisa ditarik (H+2).
const partnerEarningHold = dbpkg.PartnerEarningHoldHours * time.Hour

// releaseDuePartnerEarnings: rilis pendapatan yang sudah lewat masa tunggu.
// Pendapatan baru berstatus 'pending'; setelah created_at lewat masa tunggu
// dan belum masuk pengajuan, statusnya menjadi 'settled' = saldo yang bisa
// ditarik. Idempoten, jadi aman dipanggil di setiap pembacaan saldo mitra.
func (d *Deps) releaseDuePartnerEarnings(ctx context.Context, partnerID string) {
	_ = d.exec(ctx,
		`UPDATE partner_earnings SET status = 'settled'
		 WHERE partner_id = $1 AND status = 'pending' AND settlement_id IS NULL
		   AND created_at <= `+dbpkg.PartnerEarningHoldCutoffSQL, partnerID)
}
