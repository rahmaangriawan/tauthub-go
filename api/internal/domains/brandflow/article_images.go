package brandflow

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/imageconvert"
)

// article_images.go: pustaka gambar hasil AI per artikel Taut Pintar
// (endpoint /v1/brands/{brandId}/taut/campaigns/{campaignId}/images).
//
// Isolasi: setiap baris terikat (brand, campaign, media, article). Handler list
// memverifikasi brand + campaign + media SEBELUM membaca, jadi pustaka satu
// artikel tidak pernah bocor ke artikel lain.

// effectiveImageCost: harga per gambar yang ditampilkan & ditagih. Selalu
// mengembalikan tarif terkonfigurasi (tidak lagi 0 saat provider gambar belum
// siap) supaya label biaya di UI konsisten dengan tagihan sebenarnya.
func (d *Deps) effectiveImageCost(ctx context.Context) int {
	rt, err := ai.LoadAIRuntime(ctx, d.Pool)
	if err != nil || rt == nil {
		return ai.ImageCreditCostEnv()
	}
	return ai.ImageCreditCost(rt)
}

// imageScopeKey: kunci isolasi stabil (tanpa perlu join berlapis).
func imageScopeKey(brandID, campaignID, mediaID, articleID string) string {
	return brandID + ":" + campaignID + ":" + mediaID + ":" + articleID
}

// verifyTautImageScope: kampanye harus milik brand; media harus ada di kampanye.
func (d *Deps) verifyTautImageScope(ctx context.Context, brandID, campaignID, mediaID string) error {
	campaign, err := d.queryRow(ctx,
		"SELECT id FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandID)
	if err != nil {
		return err
	}
	if campaign == nil {
		return errNotFound("Campaign not found")
	}
	item, err := d.queryRow(ctx,
		"SELECT id FROM taut_campaign_media WHERE campaign_id = $1 AND media_id = $2 LIMIT 1", campaignID, mediaID)
	if err != nil {
		return err
	}
	if item == nil {
		return errNotFound("Media tidak termasuk dalam kampanye ini.")
	}
	return nil
}

// imageBalance: saldo efektif brand. Paket legacy dipakai bila ada; kalau tidak,
// saldo dompet TC (top-up).
func (d *Deps) imageBalance(ctx context.Context, brandID string) (int, bool) {
	balance := d.brandCreditBalance(ctx, brandID)
	if balance.PackageCount > 0 {
		return balance.Remaining, true
	}
	wallet := d.getWallet(ctx, brandID)
	if wallet == nil {
		return 0, false
	}
	return wallet.AvailableTC, false
}

// chargeImageCredit: potong kredit satu gambar. Paket legacy (FIFO) bila brand
// punya paket aktif; selain itu dompet TC. Kembalikan sisa saldo setelah potong.
func (d *Deps) chargeImageCredit(ctx context.Context, brandID, refID string, tc int, reason, userID string) (int, error) {
	if tc <= 0 {
		remaining, _ := d.imageBalance(ctx, brandID)
		return remaining, nil
	}
	balance := d.brandCreditBalance(ctx, brandID)
	if balance.PackageCount > 0 {
		return d.chargePackageCredits(ctx, brandID, tc, reason, userID)
	}
	if err := d.chargeCredit(ctx, brandID, refID, tc, reason, userID); err != nil {
		return 0, err
	}
	wallet := d.getWallet(ctx, brandID)
	if wallet == nil {
		return 0, nil
	}
	return wallet.AvailableTC, nil
}

// chargePackageCredits: potong kredit paket FIFO + catat credit_transactions
// dengan reason kustom (port reservePackageCredits tanpa publication_id).
func (d *Deps) chargePackageCredits(ctx context.Context, brandID string, amount int, reason, userID string) (int, error) {
	packages, err := d.queryRows(ctx,
		"SELECT id, credit_total, credit_used FROM packages WHERE brand_id = $1 AND status = 'active' ORDER BY created_at ASC", brandID)
	if err != nil {
		return 0, err
	}
	available := 0
	for _, pkg := range packages {
		available += pkg.intOr("credit_total", 0) - pkg.intOr("credit_used", 0)
	}
	if available < amount {
		return available, &insufficientCreditsError{Needed: amount, Available: available}
	}
	needed := amount
	remainingAfter := available
	err = d.withTx(ctx, func(tx pgx.Tx) error {
		for _, pkg := range packages {
			if needed <= 0 {
				break
			}
			pkgRemaining := pkg.intOr("credit_total", 0) - pkg.intOr("credit_used", 0)
			take := needed
			if pkgRemaining < take {
				take = pkgRemaining
			}
			if take <= 0 {
				continue
			}
			if _, err := tx.Exec(ctx,
				"UPDATE packages SET credit_used = credit_used + $1, updated_at = NOW() WHERE id = $2",
				take, pkg.strOr("id", "")); err != nil {
				return err
			}
			remainingAfter -= take
			// refID dicatat pada reason tidak tersedia di skema credit_transactions;
			// id transaksi sudah unik sehingga audit tetap bisa dilacak.
			if _, err := tx.Exec(ctx,
				"INSERT INTO credit_transactions (id, brand_id, package_id, publication_id, amount, reason, balance_after, created_by) VALUES ($1, $2, $3, NULL, $4, $5, $6, $7)",
				uuid.NewString(), brandID, pkg.strOr("id", ""), -take, reason, remainingAfter, userID); err != nil {
				return err
			}
			needed -= take
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return remainingAfter, nil
}

// storeGeneratedImage: konversi ke WebP (bila memungkinkan) lalu tulis ke
// UPLOAD_DIR; kembalikan URL publik /uploads/<file>.
func (d *Deps) storeGeneratedImage(result *ai.ImageResult) (string, error) {
	payload := result.Data
	mime := result.MimeType
	if mime == "" {
		mime = "image/png"
	}
	ext := imageExtForMime(mime)
	if ext != "webp" {
		if converted, ok := imageconvert.ConvertToWebP(result.Data, mime); ok {
			payload = converted
			ext = "webp"
		}
	}
	key := uuid.NewString() + "." + ext
	if err := d.saveUpload(key, payload); err != nil {
		return "", err
	}
	return "/uploads/" + key, nil
}

func imageExtForMime(mime string) string {
	switch mime {
	case "image/jpeg":
		return "jpg"
	case "image/webp":
		return "webp"
	case "image/gif":
		return "gif"
	default:
		return "png"
	}
}

func articleImageJSON(row bsRow) map[string]any {
	return map[string]any{
		"id":           row.strOr("id", ""),
		"url":          row.strOr("url", ""),
		"provider":     row.strOr("provider", ""),
		"model":        row.strOr("model", ""),
		"prompt":       row.strOr("prompt", ""),
		"heading":      row.raw("heading"),
		"role":         row.strOr("role", "inline"),
		"width":        row.intOr("width", 0),
		"height":       row.intOr("height", 0),
		"cost_credits": row.intOr("cost_credits", 0),
		"created_at":   row.timePtr("created_at"),
	}
}

// ArticleImagesList: GET /v1/brands/{brandId}/taut/campaigns/{campaignId}/images
func (d *Deps) ArticleImagesList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	mediaID := strings.TrimSpace(r.URL.Query().Get("media_id"))
	articleID := strings.TrimSpace(r.URL.Query().Get("article_id"))
	if mediaID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "media_id wajib diisi.")
		return
	}
	if err := d.verifyTautImageScope(r.Context(), brandRow.ID, campaignID, mediaID); err != nil {
		d.writeErr(w, err)
		return
	}

	clause := "WHERE brand_id = $1 AND campaign_id = $2 AND media_id = $3"
	args := []any{brandRow.ID, campaignID, mediaID}
	if articleID != "" {
		args = append(args, articleID)
		clause += " AND article_id = $" + itoa(len(args))
	}
	rows, err := d.queryRows(r.Context(),
		"SELECT id, url, provider, model, prompt, heading, role, width, height, cost_credits, created_at FROM article_images "+
			clause+" ORDER BY created_at DESC LIMIT 200", args...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	images := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		images = append(images, articleImageJSON(row))
	}
	remaining, hasPackages := d.imageBalance(r.Context(), brandRow.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"images":            images,
		"scope_key":         imageScopeKey(brandRow.ID, campaignID, mediaID, articleID),
		"cost_credits":      d.effectiveImageCost(r.Context()),
		"balance_remaining": remaining,
		"has_packages":      hasPackages,
		// Kurs aktif supaya klien melihat biaya & saldo dalam rupiah.
		"rate_idr": d.getTcConfig(r.Context()).RateIDR,
	})
}

// ArticleImageGenerate: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/images/generate
// Body: { media_id, article_id?, heading?, prompt, role? }
func (d *Deps) ArticleImageGenerate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	body := decodeJSON(r)

	mediaID, _ := strField(body, "media_id")
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "media_id wajib diisi.")
		return
	}
	if err := d.verifyTautImageScope(r.Context(), brandRow.ID, campaignID, mediaID); err != nil {
		d.writeErr(w, err)
		return
	}
	articleID, _ := strField(body, "article_id")
	articleID = strings.TrimSpace(articleID)
	heading, _ := strField(body, "heading")
	heading = truncateString(strings.TrimSpace(heading), 300)
	prompt, _ := strField(body, "prompt")
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = heading
	}
	if prompt == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Prompt gambar wajib diisi.")
		return
	}
	prompt = truncateString(prompt, 2000)
	role, _ := strField(body, "role")
	role = strings.ToLower(strings.TrimSpace(role))
	if role != "cover" {
		role = "inline"
	}

	cost := d.effectiveImageCost(r.Context())
	remaining, _ := d.imageBalance(r.Context(), brandRow.ID)
	// Cek saldo SEBELUM generate supaya tidak ada gambar terbuat tanpa biaya
	// yang bisa dibayar.
	if cost > 0 && remaining < cost {
		d.writeErr(w, errConflict("Saldo kredit tidak cukup untuk generate gambar: butuh "+itoa(cost)+
			", tersedia "+itoa(remaining)+". Silakan top up terlebih dahulu."))
		return
	}

	started := time.Now()
	result, err := ai.GenerateImage(r.Context(), d.Pool, ai.GenerateImageOptions{
		Prompt: prompt, Role: role, Heading: heading,
		BrandID: &brandRow.ID, UserID: &user.ID,
	})
	if err != nil {
		ai.RecordImageEvent(r.Context(), d.Pool, ai.ImageEvent{
			Kind: "generation", Status: "failed", ErrorDetail: err.Error(),
			Prompt: prompt, Role: role,
			BrandID: brandRow.ID, CampaignID: campaignID, MediaID: mediaID, ArticleID: articleID,
			RequestedBy: user.ID, ElapsedMS: int(time.Since(started).Milliseconds()),
		})
		d.writeErr(w, err)
		return
	}
	url, err := d.storeGeneratedImage(result)
	if err != nil {
		ai.RecordImageEvent(r.Context(), d.Pool, ai.ImageEvent{
			Kind: "generation", Status: "failed",
			Provider: result.Provider, Model: result.Model, BaseURL: result.BaseURL,
			ErrorDetail: "gagal menyimpan gambar: " + err.Error(), Prompt: prompt, Role: role,
			BrandID: brandRow.ID, CampaignID: campaignID, MediaID: mediaID, ArticleID: articleID,
			RequestedBy: user.ID, ElapsedMS: int(time.Since(started).Milliseconds()),
		})
		d.writeErr(w, err)
		return
	}

	// Baru potong kredit setelah gambar berhasil dibuat & disimpan — generate
	// yang gagal tidak meninggalkan tagihan. Biaya berlaku untuk semua gambar
	// (termasuk placeholder) sesuai tarif per generate.
	charged := 0
	balanceAfter := remaining
	if cost > 0 {
		refID := "ai-image:" + campaignID + ":" + mediaID + ":" + uuid.NewString()
		bal, chargeErr := d.chargeImageCredit(r.Context(), brandRow.ID, refID, cost, "ai.image.generate", user.ID)
		if chargeErr != nil {
			d.writeErr(w, chargeErr)
			return
		}
		charged = cost
		balanceAfter = bal
	} else {
		balanceAfter, _ = d.imageBalance(r.Context(), brandRow.ID)
	}

	id := uuid.NewString()
	scopeKey := imageScopeKey(brandRow.ID, campaignID, mediaID, articleID)
	if err := d.exec(r.Context(),
		`INSERT INTO article_images (id, brand_id, campaign_id, media_id, article_id, scope_key, url, provider, model, prompt, heading, role, width, height, cost_credits, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		id, brandRow.ID, campaignID, mediaID, nullIfEmpty(articleID), scopeKey, url,
		result.Provider, result.Model, prompt, nullIfEmpty(heading), role,
		result.Width, result.Height, charged, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}

	// Log monitoring: sukses atau fallback placeholder (beserta sebabnya) supaya
	// admin bisa melihat "apa & kenapa" tanpa membuka log server.
	logStatus, logDetail := "succeeded", ""
	if result.Placeholder {
		logStatus, logDetail = "fallback", result.FallbackReason
	}
	ai.RecordImageEvent(r.Context(), d.Pool, ai.ImageEvent{
		Kind: "generation", Status: logStatus,
		Provider: result.Provider, Model: result.Model, BaseURL: result.BaseURL,
		ErrorDetail: logDetail, Prompt: prompt, Role: role,
		BrandID: brandRow.ID, CampaignID: campaignID, MediaID: mediaID, ArticleID: articleID,
		RequestedBy: user.ID, Width: result.Width, Height: result.Height,
		CreditsCharged: charged, ElapsedMS: result.ElapsedMS,
		ImageID: id, ImageURL: url,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"id":                id,
		"url":               url,
		"provider":          result.Provider,
		"model":             result.Model,
		"width":             result.Width,
		"height":            result.Height,
		"cost_credits":      charged,
		"balance_remaining": balanceAfter,
		"rate_idr":          d.getTcConfig(r.Context()).RateIDR,
		"placeholder":       result.Placeholder,
		"mime":              result.MimeType,
	})
}

// ArticleImageDelete: DELETE /v1/brands/{brandId}/taut/campaigns/{campaignId}/images/{imageId}
func (d *Deps) ArticleImageDelete(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	imageID := chiParam(r, "imageId")
	row, err := d.queryRow(r.Context(),
		"SELECT url FROM article_images WHERE id = $1 AND brand_id = $2 AND campaign_id = $3 LIMIT 1",
		imageID, brandRow.ID, campaignID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		d.writeErr(w, errNotFound("Gambar tidak ditemukan."))
		return
	}
	if err := d.exec(r.Context(),
		"DELETE FROM article_images WHERE id = $1 AND brand_id = $2 AND campaign_id = $3",
		imageID, brandRow.ID, campaignID); err != nil {
		d.writeErr(w, err)
		return
	}
	// Hapus berkas fisik best-effort (kegagalan tidak menggagalkan respons).
	if name := strings.TrimPrefix(row.strOr("url", ""), "/uploads/"); name != "" && !strings.Contains(name, "/") {
		_ = os.Remove(d.uploadDir() + string(os.PathSeparator) + name)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ArticleImageUpload: POST /v1/brands/{brandId}/taut/campaigns/{campaignId}/images/upload
// Unggah gambar manual (drag-and-drop, paste, browse) langsung ke pustaka artikel.
// Gambar otomatis dinormalisasi ke rasio 16:9 murni (crop tengah) & batas 1920x1280.
func (d *Deps) ArticleImageUpload(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	campaignID := chiParam(r, "campaignId")
	file, filename, size, contentType, err := formFile(r, "file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "File gambar tidak ditemukan.")
		return
	}
	defer file.Close()

	mediaID := strings.TrimSpace(r.FormValue("media_id"))
	if mediaID == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "media_id wajib diisi.")
		return
	}
	if err := d.verifyTautImageScope(r.Context(), brandRow.ID, campaignID, mediaID); err != nil {
		d.writeErr(w, err)
		return
	}

	mime := strings.ToLower(contentType)
	if !strings.HasPrefix(mime, "image/") && mime != "application/octet-stream" {
		httpx.Error(w, http.StatusUnprocessableEntity, "File harus berupa gambar (JPG, PNG, WebP).")
		return
	}
	if size > 15*1024*1024 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Maksimal ukuran gambar 15 MB.")
		return
	}

	buffer, err := readAllLimited(file, 15*1024*1024)
	if err != nil {
		d.writeErr(w, err)
		return
	}

	normData, wPx, hPx, normErr := ai.NormalizeTo16x9(buffer)
	var finalBytes []byte
	var finalMime string
	var finalWidth, finalHeight int
	if normErr == nil {
		finalBytes = normData
		finalMime = "image/png"
		finalWidth, finalHeight = wPx, hPx
	} else {
		finalBytes = buffer
		finalMime = mime
		finalWidth, finalHeight = 0, 0
	}

	storedURL, storeErr := d.storeGeneratedImage(&ai.ImageResult{
		Data:     finalBytes,
		MimeType: finalMime,
		Width:    finalWidth,
		Height:   finalHeight,
	})
	if storeErr != nil {
		d.writeErr(w, storeErr)
		return
	}

	articleID := strings.TrimSpace(r.FormValue("article_id"))
	role := strings.ToLower(strings.TrimSpace(r.FormValue("role")))
	if role != "cover" {
		role = "inline"
	}
	prompt := strings.TrimSpace(filename)
	if prompt == "" {
		prompt = "Unggahan manual"
	}

	id := uuid.NewString()
	scopeKey := imageScopeKey(brandRow.ID, campaignID, mediaID, articleID)
	if err := d.exec(r.Context(),
		`INSERT INTO article_images (id, brand_id, campaign_id, media_id, article_id, scope_key, url, provider, model, prompt, heading, role, width, height, cost_credits, created_by)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		id, brandRow.ID, campaignID, mediaID, nullIfEmpty(articleID), scopeKey, storedURL,
		"upload", "manual_upload", prompt, nil, role,
		finalWidth, finalHeight, 0, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":           id,
		"url":          storedURL,
		"provider":     "upload",
		"model":        "manual_upload",
		"prompt":       prompt,
		"heading":      nil,
		"role":         role,
		"width":        finalWidth,
		"height":       finalHeight,
		"cost_credits": 0,
		"created_at":   time.Now().UTC().Format(time.RFC3339),
	})
}
