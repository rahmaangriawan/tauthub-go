package platform

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
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
	// Login wajib — verifikasi cookie refresh, sama seperti app lama.
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		httpx.Error(w, http.StatusUnauthorized, "Login diperlukan untuk mengakses dokumen ini.")
		return
	}
	if _, err := auth.VerifyRefreshToken(d.Cfg.Secret, d.Cfg.AppName, cookie.Value); err != nil {
		httpx.Error(w, http.StatusUnauthorized, "Login diperlukan untuk mengakses dokumen ini.")
		return
	}

	// Sudah login: batasi laju pembacaan berkas per IP.
	if !d.pubRateLimit(w, r, "uploads", 600) {
		return
	}

	key := chiParam(r, "key")
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
	// Privat: konten hanya untuk sesi login, jangan disimpan cache bersama.
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
