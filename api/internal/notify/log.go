package notify

import (
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// isNoRows menyembunyikan detail driver dari pemanggil di paket ini.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// LogDecision mencatat satu keputusan kanal dengan bentuk yang sama di semua
// titik pemasangan, supaya "kenapa email ini tidak terkirim" bisa dijawab dari
// log tanpa menebak titik mana yang menolak.
//
// Hanya user id yang dicatat. Alamat email, nomor WA, dan isi pesan tidak ikut
// ke log karena itu data pribadi dan tidak diperlukan untuk menelusuri biaya.
func LogDecision(decision Decision, req Request) {
	slog.Debug("[notify] keputusan kanal",
		"channel", req.Channel,
		"user_id", req.UserID,
		"type", req.Type,
		"send", decision.Send,
		"reason", decision.Reason,
	)
}
