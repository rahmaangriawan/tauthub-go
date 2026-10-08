package platform

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// Port app/uploads/[key]/route.ts — serve berkas unggahan dari disk.
//
// WAJIB LOGIN: cookie `refresh_token` diverifikasi sebelum berkas dilayani.
// Dokumen di sini sensitif (bukti transfer, sertifikat, sampul artikel) dan
// tidak boleh diunduh siapa pun yang menebak URL. Nama berkas dibatasi
// (`filepath.Base` + whitelist ekstensi) supaya tidak ada path traversal.
var uploadMIME = map[string]string{
	".pdf":  "application/pdf",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".gif":  "image/gif",
	".avif": "image/avif",
	".txt":  "text/plain; charset=utf-8",
}

// uploadServeDirs: direktori unggahan, parity `UPLOAD_DIR` lalu
// `LEGACY_UPLOAD_DIR` (public/uploads) app lama.
func uploadServeDirs() []string {
	primary := strings.TrimSpace(os.Getenv("UPLOAD_DIR"))
	if primary == "" {
		primary = filepath.Join(".data", "uploads")
	}
	legacy := strings.TrimSpace(os.Getenv("LEGACY_UPLOAD_DIR"))
	if legacy == "" {
		legacy = filepath.Join("public", "uploads")
	}
	return []string{primary, legacy}
}

// UploadsServe: GET /uploads/{key}
func (d *Deps) UploadsServe(w http.ResponseWriter, r *http.Request) {
	key := chiParam(r, "key")

	// Dua jalur akses: sesi login (berkas privat) atau akses publik untuk
	// gambar artikel yang sudah terbit di situs media. Tanpa keduanya, berkas
	// tetap tertutup.
	public := false
	if !d.uploadSessionOK(r) {
		signed := security.VerifyUploadKey(d.Cfg.Secret, key, r.URL.Query().Get(security.UploadSignatureParam))
		if !signed && !d.uploadKeyPublished(r.Context(), key) {
			httpx.Error(w, http.StatusUnauthorized, "Login diperlukan untuk mengakses dokumen ini.")
			return
		}
		public = true
	}

	// Batasi laju pembacaan berkas per IP (jalur publik lebih longgar karena
	// gambarnya memang dibaca pengunjung situs media).
	if public {
		if !d.pubRateLimit(w, r, "uploads-public", 3000) {
			return
		}
	} else if !d.pubRateLimit(w, r, "uploads", 600) {
		return
	}

	name := filepath.Base(key)
	// `filepath.Base("..")` = ".." dan `Base("a/../b")` = "b" — tolak yang
	// tersisa sebagai nama relatif apa pun supaya tidak keluar dari direktori.
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		httpx.Error(w, http.StatusBadRequest, "File tidak valid.")
		return
	}
	mime, ok := uploadMIME[strings.ToLower(filepath.Ext(name))]
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "File tidak valid.")
		return
	}

	var data []byte
	for _, dir := range uploadServeDirs() {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			data = body
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			continue
		}
	}
	if data == nil {
		httpx.Error(w, http.StatusNotFound, "File tidak ditemukan.")
		return
	}

	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", `inline; filename="`+name+`"`)
	if public {
		// Nama berkas sudah unik dan isinya tidak berubah, jadi aman di-cache
		// lama oleh browser maupun cache perantara (Blogger, CDN).
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		// Privat: konten hanya untuk sesi login, jangan disimpan cache bersama.
		w.Header().Set("Cache-Control", "private, max-age=3600")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// uploadSessionOK: cookie refresh_token sah, sesinya masih hidup, dan user
// masih aktif. Dipakai jalur unduh privat di atas.
func (d *Deps) uploadSessionOK(r *http.Request) bool {
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		return false
	}
	claims, err := auth.VerifyRefreshToken(d.Cfg.Secret, d.Cfg.AppName, cookie.Value)
	if err != nil {
		return false
	}
	// Sesi wajib masih hidup & user masih aktif — refresh token yang sudah
	// di-revoke (logout, reset password, suspend) tidak lagi boleh mengunduh.
	if claims.SID != "" {
		var revoked bool
		var expiresAt time.Time
		if err := d.Pool.QueryRow(r.Context(),
			"SELECT revoked, expires_at FROM user_sessions WHERE refresh_jti = $1 LIMIT 1",
			claims.SID,
		).Scan(&revoked, &expiresAt); err != nil || revoked || !expiresAt.After(time.Now()) {
			return false
		}
	}
	var active bool
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT is_active FROM users WHERE id = $1 AND is_active = TRUE AND approved_at IS NOT NULL LIMIT 1",
		claims.Sub,
	).Scan(&active); err != nil {
		return false
	}
	return true
}

// uploadPublicTTL: masa simpan hasil pengecekan di memori.
const uploadPublicTTL = 60 * time.Second

var (
	uploadPublicMu    sync.Mutex
	uploadPublicCache = map[string]uploadPublicEntry{}
)

type uploadPublicEntry struct {
	published bool
	expiresAt time.Time
}

// uploadKeyPublished: berkas sudah dipakai sebagai gambar utama publikasi yang
// sudah terbit. Artikel yang sudah tayang di Blogger menautkan /uploads tanpa
// tanda tangan (fitur tanda tangan baru dipakai publikasi berikutnya), jadi
// berkas seperti ini harus tetap bisa dibaca pengunjung. Hasilnya di-cache
// singkat supaya tidak query tiap permintaan gambar.
func (d *Deps) uploadKeyPublished(ctx context.Context, key string) bool {
	if key == "" {
		return false
	}
	now := time.Now()
	uploadPublicMu.Lock()
	entry, ok := uploadPublicCache[key]
	uploadPublicMu.Unlock()
	if ok && entry.expiresAt.After(now) {
		return entry.published
	}

	var published bool
	if err := d.Pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM publications WHERE status = 'published' AND featured_image_url LIKE '%/uploads/' || $1 || '%')",
		key,
	).Scan(&published); err != nil {
		return false
	}

	uploadPublicMu.Lock()
	uploadPublicCache[key] = uploadPublicEntry{published: published, expiresAt: now.Add(uploadPublicTTL)}
	uploadPublicMu.Unlock()
	return published
}
