package profiler

// uploadquota.go — kuota & siklus hidup unggahan chatbot Taut Brief.
//
// Berkas fisik di UPLOAD_DIR bersifat SEMENTARA:
//   dokumen → teksnya diekstrak ke profiler_documents.extracted_text (dibaca
//             AI dari DB, bukan dari file), berkas fisik dibuang setelah
//             retensi (default 24 jam setelah diproses);
//   gambar  → berkasnya memang asetnya (tab Media / GEO), retensi default
//             lebih panjang (30 hari) supaya pustaka media tidak mengosong
//             diri sendiri — operator bisa mengubahnya via env.
//
// Semua batas dikonfigurasi lewat env (lihat docs/upload-quota.md). Kuota
// dicek di endpoint unggah dalam SATU transaksi dengan advisory lock per
// user, lalu baris ledger status 'processing' di-insert sebagai reservasi —
// unggahan bersamaan dari user yang sama tidak bisa lolos bersamaan.

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// UploadLimits: batas kuota + siklus hidup. Angka default mengikuti kebijakan
// produk (spesifikasi kuota upload chatbot).
type UploadLimits struct {
	MaxFileBytes         int64         // ukuran maksimum satu berkas
	MaxConversationFiles int           // jumlah berkas aktif per percakapan (jalur chat)
	MaxConversationBytes int64         // total ukuran aktif per percakapan (jalur chat)
	MaxDailyFiles        int           // jumlah berkas per user per hari (semua jalur)
	MaxDailyBytes        int64         // total ukuran per user per hari (semua jalur)
	MaxActiveTempBytes   int64         // total berkas aktif (belum deleted) per user
	Retention            time.Duration // dokumen: umur berkas setelah diproses
	ImageRetention       time.Duration // gambar: umur berkas setelah diproses
	MaxUnprocessedAge    time.Duration // umur maksimum berkas tertinggal uploaded/processing
	CleanupInterval      time.Duration // interval worker cleanup
	OrphanGrace          time.Duration // grace period sebelum berkas tanpa metadata dianggap yatim
}

const (
	docxMime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	// zeroUserID: pengganti created_by NULL pada backfill ledger.
	zeroUserID = "00000000-0000-0000-0000-000000000000"
)

func LoadUploadLimits() UploadLimits {
	return UploadLimits{
		MaxFileBytes:         uploadEnvBytes("UPLOAD_MAX_FILE_MB", 10),
		MaxConversationFiles: uploadEnvInt("UPLOAD_MAX_PER_CONVERSATION_FILES", 5),
		MaxConversationBytes: uploadEnvBytes("UPLOAD_MAX_PER_CONVERSATION_MB", 25),
		MaxDailyFiles:        uploadEnvInt("UPLOAD_MAX_DAILY_FILES", 30),
		MaxDailyBytes:        uploadEnvBytes("UPLOAD_MAX_DAILY_MB", 100),
		MaxActiveTempBytes:   uploadEnvBytes("UPLOAD_MAX_ACTIVE_TEMP_MB", 100),
		Retention:            uploadEnvDuration("UPLOAD_RETENTION_HOURS", 24*time.Hour),
		ImageRetention:       uploadEnvDuration("UPLOAD_IMAGE_RETENTION_HOURS", 30*24*time.Hour),
		MaxUnprocessedAge:    uploadEnvDuration("UPLOAD_MAX_UNPROCESSED_HOURS", 6*time.Hour),
		CleanupInterval:      uploadEnvDurationMinutes("UPLOAD_CLEANUP_INTERVAL_MINUTES", 15*time.Minute),
		OrphanGrace:          uploadEnvDurationMinutes("UPLOAD_ORPHAN_GRACE_MINUTES", 60*time.Minute),
	}
}

func uploadEnvInt(key string, def int) int {
	if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func uploadEnvBytes(key string, defMB int64) int64 {
	if n := uploadEnvInt(key, 0); n > 0 {
		return int64(n) << 20
	}
	return defMB << 20
}

// uploadEnvDuration: env dalam satuan jam (nilai pecahan diperbolehkan).
func uploadEnvDuration(key string, def time.Duration) time.Duration {
	if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
		if h, err := strconv.ParseFloat(raw, 64); err == nil && h > 0 {
			return time.Duration(h * float64(time.Hour))
		}
	}
	return def
}

func uploadEnvDurationMinutes(key string, def time.Duration) time.Duration {
	if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
		if m, err := strconv.ParseFloat(raw, 64); err == nil && m > 0 {
			return time.Duration(m * float64(time.Minute))
		}
	}
	return def
}

// UploadUsage: pemakaian terkini yang dihimpun dari ledger (dihitung di dalam
// transaksi ber-advisory-lock sebelum reservasi).
type UploadUsage struct {
	ConversationFiles int
	ConversationBytes int64
	DailyFiles        int
	DailyBytes        int64
	ActiveTempBytes   int64
}

// QuotaViolation: pelanggaran kuota → status HTTP 413 (basis ukuran) atau 429
// (basis jumlah/laju).
type QuotaViolation struct {
	Status  int
	Message string
}

func (q *QuotaViolation) Error() string { return q.Message }

// mb: pembantu format pesan.
func mb(n int64) string { return strconv.FormatInt(n>>20, 10) + " MB" }

// Check: putuskan apakah unggahan `fileSize` bytes masih boleh, dengan
// pemakaian `u`. viaChat=false (unggah dari tab Media/library) tidak
// menghabiskan kuota percakapan — batas harian & aktif tetap berlaku.
func (l UploadLimits) Check(u UploadUsage, fileSize int64, viaChat bool) *QuotaViolation {
	if fileSize > l.MaxFileBytes {
		return &QuotaViolation{
			Status:  http.StatusRequestEntityTooLarge,
			Message: fmt.Sprintf("Berkas %s melebihi batas %s per berkas.", mb(fileSize), mb(l.MaxFileBytes)),
		}
	}
	if viaChat {
		if u.ConversationFiles+1 > l.MaxConversationFiles {
			return &QuotaViolation{
				Status:  http.StatusTooManyRequests,
				Message: fmt.Sprintf("Kuota percakapan terlampaui: maksimal %d berkas per brief. Hapus lampiran lama atau lanjutkan di brief baru.", l.MaxConversationFiles),
			}
		}
		if u.ConversationBytes+fileSize > l.MaxConversationBytes {
			return &QuotaViolation{
				Status:  http.StatusRequestEntityTooLarge,
				Message: fmt.Sprintf("Total lampiran brief akan melebihi %s. Hapus lampiran lama dulu.", mb(l.MaxConversationBytes)),
			}
		}
	}
	if u.DailyFiles+1 > l.MaxDailyFiles {
		return &QuotaViolation{
			Status:  http.StatusTooManyRequests,
			Message: fmt.Sprintf("Kuota harian terlampaui: maksimal %d berkas per hari.", l.MaxDailyFiles),
		}
	}
	if u.DailyBytes+fileSize > l.MaxDailyBytes {
		return &QuotaViolation{
			Status:  http.StatusRequestEntityTooLarge,
			Message: fmt.Sprintf("Kuota harian terlampaui: maksimal total %s per hari.", mb(l.MaxDailyBytes)),
		}
	}
	if u.ActiveTempBytes+fileSize > l.MaxActiveTempBytes {
		return &QuotaViolation{
			Status:  http.StatusRequestEntityTooLarge,
			Message: fmt.Sprintf("Total unggahan aktifmu melebihi %s. Tunggu pembersihan otomatis atau hapus lampiran lama.", mb(l.MaxActiveTempBytes)),
		}
	}
	return nil
}

// sniffFormatOK: cocokkan isi berkas (magic number) dengan MIME yang
// diklaimnya. Teks (txt/md/csv) tidak punya magic number → diterima.
// Mencegah PDF palsu berisi skrip, dsb.
func sniffFormatOK(buf []byte, mime string) bool {
	switch mime {
	case "application/pdf":
		return bytes.HasPrefix(buf, []byte("%PDF"))
	case docxMime:
		// DOCX = arsip zip: selalu diawali tanda tangan "PK".
		return len(buf) > 3 && buf[0] == 'P' && buf[1] == 'K' && (buf[2] == 3 || buf[2] == 5 || buf[2] == 7)
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return http.DetectContentType(buf) == mime
	}
	return true
}

// expiryFor: kapan berkas fisik boleh dibersihkan setelah diproses.
func expiryFor(kind string, lim UploadLimits, now time.Time) time.Time {
	if kind == "image" {
		return now.Add(lim.ImageRetention)
	}
	return now.Add(lim.Retention)
}

// chatFileKeyRe: pola nama berkas internal yang dihasilkan server untuk
// unggahan profiler/chat (uuid + ekstensi pendek). Dipakai sapuan yatim agar
// hanya berkas buatan sistem yang kandidat dihapus — nama bebas user tidak
// pernah jadi nama path.
var chatFileKeyRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.[a-z0-9]{1,5}$`)

func looksLikeManagedUpload(name string) bool {
	return chatFileKeyRe.MatchString(strings.ToLower(name))
}

// quotaViolationOf: ekstrak QuotaViolation dari error (nil bila bukan).
func quotaViolationOf(err error) *QuotaViolation {
	var v *QuotaViolation
	if errors.As(err, &v) {
		return v
	}
	return nil
}
