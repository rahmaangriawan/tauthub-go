package notify

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Satu pintu keputusan untuk semua kanal berbayar.
//
// Pemanggil tidak perlu tahu urutan pemeriksaannya: cukup bertanya "boleh
// kirim email untuk event ini ke user ini?" lalu menghormati jawabannya.
// Semua ambang ada di blok const di bawah, jadi menyesuaikan biaya cukup
// mengubah satu tempat.

const (
	// EmailReadWindow: kalau notifikasi in-app untuk event yang sama sudah
	// dibaca dalam 24 jam, email dibatalkan. User yang aktif di aplikasi tidak
	// perlu diberi tahu dua kali dengan biaya dua kali.
	EmailReadWindow = 24 * time.Hour

	// EmailDedupeWindow: jangan kirim email kedua untuk event yang sama dalam
	// 24 jam, walau baris notifikasinya berbeda.
	EmailDedupeWindow = 24 * time.Hour

	// EmailHourlyLimit: batas email per user per jam. Sama dengan batas lama
	// di paket email, tetapi sekarang berlaku lintas proses.
	EmailHourlyLimit = 20

	// WAReadWindow: jendela gate baca untuk WhatsApp. Lebih pendek dari email
	// karena WA dipakai justru saat user kemungkinan tidak membuka aplikasi.
	WAReadWindow = 6 * time.Hour

	// WADedupeWindow: WA jauh lebih mahal per pesan, jadi dedupe-nya lebih
	// panjang. Satu event, satu pesan, dalam 72 jam.
	WADedupeWindow = 72 * time.Hour

	// WAHourlyLimit: batas WA per user per jam. Sengaja kecil; kanal ini hanya
	// dipakai untuk hal yang mendesak.
	WAHourlyLimit = 5

	// QuotaWindow: bucket kuota. Satu jam, sejajar dengan batas lama.
	QuotaWindow = time.Hour
)

// Nilai Reason pada Decision. Dipakai untuk log dan audit, jadi jangan diubah
// tanpa memperbarui dokumentasi di README.
const (
	ReasonAllowed     = "allowed"
	ReasonOff         = "off"
	ReasonDigest      = "digest"
	ReasonAlreadyRead = "already-read"
	ReasonDuplicate   = "duplicate"
	ReasonQuota       = "quota"
	ReasonNoContact   = "no-contact"
	ReasonError       = "error"
)

// Request adalah pertanyaan satu kanal untuk satu event.
type Request struct {
	// UserID penerima. Wajib.
	UserID string
	// Type tipe notifikasi, sama dengan nilai di tabel notifications.
	Type string
	// DedupeKey identitas event. Untuk event yang bisa berulang pada entitas
	// berbeda, sertakan id entitas terkecil yang membedakannya.
	DedupeKey string
	// Channel "email" atau "wa".
	Channel string
}

// Decision adalah jawaban ShouldSend.
type Decision struct {
	// Send true berarti pemanggil boleh mengirim sekarang.
	Send bool
	// Reason salah satu konstanta Reason* di atas.
	Reason string
}

// ShouldSend memutuskan apakah satu kanal berbayar boleh dipakai.
//
// Urutan pemeriksaan, dari yang paling murah:
//  1. mode kanal di registry (off/digest langsung menolak)
//  2. kelengkapan kontak penerima
//  3. gate baca in-app
//  4. dedupe per event pada kanal itu
//  5. kuota per jam
//
// Setiap error database menghasilkan penolakan, bukan pengiriman. Arah itu
// disengaja: query yang rusak tidak boleh berubah menjadi tagihan.
func ShouldSend(ctx context.Context, pool *pgxpool.Pool, req Request) Decision {
	policy := PolicyFor(req.Type)

	mode := policy.Email
	readWindow := EmailReadWindow
	dedupeWindow := EmailDedupeWindow
	limit := EmailHourlyLimit
	switch req.Channel {
	case "email":
		mode = policy.Email
	case "wa":
		mode = policy.WA
		readWindow = WAReadWindow
		dedupeWindow = WADedupeWindow
		limit = WAHourlyLimit
	default:
		return Decision{Send: false, Reason: ReasonOff}
	}

	switch mode {
	case ModeOff:
		// Admin bisa menyalakan email per event dari panel Template. Override
		// hanya berlaku untuk email; WA tetap mengikuti registry. Gate baca,
		// dedupe, kuota, dan preferensi user tetap berlaku di bawah.
		if req.Channel == "email" && EmailChannelOverride(ctx, pool, req.Type) {
			mode = ModeImmediate
			break
		}
		return Decision{Send: false, Reason: ReasonOff}
	case ModeDigest:
		return Decision{Send: false, Reason: ReasonDigest}
	}

	if pool == nil {
		return Decision{Send: false, Reason: ReasonError}
	}

	hasContact, err := hasContactChannel(ctx, pool, req.UserID, req.Channel)
	if err != nil {
		slogWarn("cek kontak gagal", req, err)
		return Decision{Send: false, Reason: ReasonError}
	}
	if !hasContact {
		return Decision{Send: false, Reason: ReasonNoContact}
	}

	read, err := SuppressedByRead(ctx, pool, req.UserID, req.Type, req.DedupeKey, readWindow)
	if err != nil {
		slogWarn("gate baca gagal", req, err)
		return Decision{Send: false, Reason: ReasonError}
	}
	if read {
		return Decision{Send: false, Reason: ReasonAlreadyRead}
	}

	dup, err := AlreadySentRecently(ctx, pool, req.Channel, req.DedupeKey, dedupeWindow)
	if err != nil {
		slogWarn("cek duplikat gagal", req, err)
		return Decision{Send: false, Reason: ReasonError}
	}
	if dup {
		return Decision{Send: false, Reason: ReasonDuplicate}
	}

	allowed, err := TakeQuota(ctx, pool, req.UserID, req.Channel, limit, QuotaWindow)
	if err != nil {
		slogWarn("ambil kuota gagal", req, err)
		return Decision{Send: false, Reason: ReasonError}
	}
	if !allowed {
		return Decision{Send: false, Reason: ReasonQuota}
	}

	return Decision{Send: true, Reason: ReasonAllowed}
}

// slogWarn menyeragamkan log kegagalan pemeriksaan di jalur ini. Tanpa user id
// saja, supaya alamat email dan nomor WA tidak pernah masuk log.
func slogWarn(msg string, req Request, err error) {
	slog.Warn("[notify] "+msg,
		"channel", req.Channel,
		"user_id", req.UserID,
		"type", req.Type,
		"err", err,
	)
}

// hasContactChannel memastikan penerima memang bisa dihubungi lewat kanal ini.
// Mengirim ke alamat atau nomor kosong tetap dihitung sebagai percobaan
// berbayar oleh provider, jadi lebih baik ditolak sebelum enqueue.
func hasContactChannel(ctx context.Context, pool *pgxpool.Pool, userID, channel string) (bool, error) {
	switch channel {
	case "email":
		var email string
		err := pool.QueryRow(ctx,
			`SELECT COALESCE(email, '') FROM users WHERE id = $1`, userID).Scan(&email)
		if err != nil {
			if isNoRows(err) {
				return false, nil
			}
			return false, err
		}
		return email != "", nil
	case "wa":
		// WA butuh nomor DAN persetujuan. Tanpa consent, pesan tidak boleh
		// dikirim walau nomornya ada.
		var number string
		var consent bool
		err := pool.QueryRow(ctx, `
			SELECT COALESCE(wa_number, ''), COALESCE(wa_consent, false)
			FROM notification_preferences WHERE user_id = $1`, userID).Scan(&number, &consent)
		if err != nil {
			if isNoRows(err) {
				return false, nil
			}
			return false, err
		}
		return number != "" && consent, nil
	}
	return false, nil
}
