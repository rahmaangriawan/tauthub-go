package profiler

// uploadquota_test.go — test logika kuota & siklus hidup unggahan (fungsi
// murni, tanpa DB/HTTP). Perilaku handler (status HTTP, advisory lock,
// penghapusan fisik) diverifikasi pemilik aplikasi secara manual sesuai
// aturan repo.

import (
	"net/http"
	"testing"
	"time"
)

func TestUploadLimitsCheckFileTooBig(t *testing.T) {
	lim := LoadUploadLimits()
	lim.MaxFileBytes = 10 << 20
	v := lim.Check(UploadUsage{}, 11<<20, true)
	if v == nil || v.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("berkas >10MB harus 413, dapat %+v", v)
	}
}

func TestUploadLimitsCheckConversationCaps(t *testing.T) {
	lim := LoadUploadLimits()
	lim.MaxConversationFiles = 5
	lim.MaxConversationBytes = 25 << 20

	// Jumlah berkas percakapan penuh → 429.
	v := lim.Check(UploadUsage{ConversationFiles: 5}, 1<<20, true)
	if v == nil || v.Status != http.StatusTooManyRequests {
		t.Fatalf("kuota jumlah percakapan harus 429, dapat %+v", v)
	}
	// Total ukuran percakapan terlampaui → 413.
	v = lim.Check(UploadUsage{ConversationBytes: 25 << 20}, 1, true)
	if v == nil || v.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("kuota ukuran percakapan harus 413, dapat %+v", v)
	}
	// Jalur library (tab Media) tidak memakan kuota percakapan.
	if v := lim.Check(UploadUsage{ConversationFiles: 5, ConversationBytes: 25 << 20}, 1<<20, false); v != nil {
		t.Fatalf("library harus lolos kuota percakapan, dapat %+v", v)
	}
}

func TestUploadLimitsCheckDailyAndActive(t *testing.T) {
	lim := LoadUploadLimits()
	lim.MaxDailyFiles = 30
	lim.MaxDailyBytes = 100 << 20
	lim.MaxActiveTempBytes = 100 << 20

	v := lim.Check(UploadUsage{DailyFiles: 30}, 1<<20, false)
	if v == nil || v.Status != http.StatusTooManyRequests {
		t.Fatalf("kuota jumlah harian harus 429, dapat %+v", v)
	}
	v = lim.Check(UploadUsage{DailyBytes: 100 << 20}, 1, false)
	if v == nil || v.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("kuota ukuran harian harus 413, dapat %+v", v)
	}
	v = lim.Check(UploadUsage{ActiveTempBytes: 100 << 20}, 1, false)
	if v == nil || v.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("kuota aktif per-user harus 413, dapat %+v", v)
	}
	// Semua masih di bawah batas → lolos.
	if v := lim.Check(UploadUsage{DailyFiles: 29, DailyBytes: 50 << 20, ActiveTempBytes: 50 << 20}, 1<<20, true); v != nil {
		t.Fatalf("pemakaian wajar harus lolos, dapat %+v", v)
	}
}

func TestSniffFormatOK(t *testing.T) {
	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}
	cases := []struct {
		name  string
		buf   []byte
		mime  string
		valid bool
	}{
		{"pdf sah", []byte("%PDF-1.7 ..."), "application/pdf", true},
		{"pdf palsu (HTML)", []byte("<html>burukan</html>"), "application/pdf", false},
		{"docx = zip PK", append([]byte{'P', 'K', 3, 4}, make([]byte, 64)...), docxMime, true},
		{"docx palsu", []byte("%PDF-bukan-zip"), docxMime, false},
		{"png sah", pngHeader, "image/png", true},
		{"png palsu (teks)", []byte("bukan gambar sama sekali"), "image/png", false},
		{"gif sah", []byte("GIF89a......"), "image/gif", true},
		{"webp sah", append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 32)...), "image/webp", true},
		{"teks tanpa magic", []byte("halo dunia"), "text/plain", true},
		{"csv tanpa magic", []byte("a,b\n1,2"), "text/csv", true},
	}
	for _, c := range cases {
		if got := sniffFormatOK(c.buf, c.mime); got != c.valid {
			t.Errorf("%s: sniffFormatOK=%v, mau %v", c.name, got, c.valid)
		}
	}
}

func TestExpiryFor(t *testing.T) {
	lim := LoadUploadLimits()
	lim.Retention = 24 * time.Hour
	lim.ImageRetention = 30 * 24 * time.Hour
	now := time.Now()
	if d := expiryFor("document", lim, now).Sub(now); d != 24*time.Hour {
		t.Errorf("retensi dokumen = %v, mau 24 jam", d)
	}
	if d := expiryFor("image", lim, now).Sub(now); d != 30*24*time.Hour {
		t.Errorf("retensi gambar = %v, mau 720 jam", d)
	}
}

func TestLooksLikeManagedUpload(t *testing.T) {
	valid := []string{
		"0f79a5c2-1b3e-4c5d-9f8a-7e6d5c4b3a21.pdf",
		"0F79A5C2-1B3E-4C5D-9F8A-7E6D5C4B3A21.PNG",
	}
	invalid := []string{
		"laporan final.pdf",
		"../../etc/passwd",
		"/etc/passwd",
		"0f79a5c2-1b3e-4c5d-9f8a-7e6d5c4b3a21",
		"0f79a5c2-1b3e-4c5d-9f8a-7e6d5c4b3a21.exe.bat",
		"habis..pdf",
	}
	for _, n := range valid {
		if !looksLikeManagedUpload(n) {
			t.Errorf("%q harus dianggap berkas terkelola", n)
		}
	}
	for _, n := range invalid {
		if looksLikeManagedUpload(n) {
			t.Errorf("%q tidak boleh dianggap berkas terkelola", n)
		}
	}
}

func TestLoadUploadLimitsDefaults(t *testing.T) {
	for _, k := range []string{
		"UPLOAD_MAX_FILE_MB", "UPLOAD_MAX_PER_CONVERSATION_FILES", "UPLOAD_MAX_PER_CONVERSATION_MB",
		"UPLOAD_MAX_DAILY_FILES", "UPLOAD_MAX_DAILY_MB", "UPLOAD_MAX_ACTIVE_TEMP_MB",
		"UPLOAD_RETENTION_HOURS", "UPLOAD_IMAGE_RETENTION_HOURS", "UPLOAD_MAX_UNPROCESSED_HOURS",
		"UPLOAD_CLEANUP_INTERVAL_MINUTES", "UPLOAD_ORPHAN_GRACE_MINUTES",
	} {
		t.Setenv(k, "")
	}
	lim := LoadUploadLimits()
	if lim.MaxFileBytes != 10<<20 || lim.MaxConversationFiles != 5 || lim.MaxConversationBytes != 25<<20 {
		t.Errorf("default salah: %+v", lim)
	}
	if lim.MaxDailyFiles != 30 || lim.MaxDailyBytes != 100<<20 || lim.MaxActiveTempBytes != 100<<20 {
		t.Errorf("default harian/aktif salah: %+v", lim)
	}
	if lim.Retention != 24*time.Hour || lim.ImageRetention != 720*time.Hour || lim.MaxUnprocessedAge != 6*time.Hour {
		t.Errorf("default retensi salah: %+v", lim)
	}
	if lim.CleanupInterval != 15*time.Minute || lim.OrphanGrace != time.Hour {
		t.Errorf("default interval/grace salah: %+v", lim)
	}
}

func TestLoadUploadLimitsEnvOverride(t *testing.T) {
	t.Setenv("UPLOAD_MAX_FILE_MB", "2")
	t.Setenv("UPLOAD_RETENTION_HOURS", "1.5")
	lim := LoadUploadLimits()
	if lim.MaxFileBytes != 2<<20 {
		t.Errorf("override MB tidak jalan: %v", lim.MaxFileBytes)
	}
	if lim.Retention != 90*time.Minute {
		t.Errorf("override jam pecahan tidak jalan: %v", lim.Retention)
	}
}

func TestQuotaViolationIsError(t *testing.T) {
	lim := LoadUploadLimits()
	lim.MaxDailyFiles = 1
	v := lim.Check(UploadUsage{DailyFiles: 1}, 1, false)
	if v == nil {
		t.Fatal("harus ada pelanggaran")
	}
	if v.Error() != v.Message || quotaViolationOf(v) != v {
		t.Errorf("QuotaViolation harus memenuhi interface error")
	}
	if quotaViolationOf(errString("x")) != nil {
		t.Errorf("error biasa tidak boleh dianggap pelanggaran kuota")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
