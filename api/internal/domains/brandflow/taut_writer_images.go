package brandflow

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
)

// taut_writer_images.go: gambar isi otomatis untuk The Writer.
//
// Alur: bila klien meminta gambar/foto/ilustrasi di dalam artikel (lewat
// topik/konteks), Writer menaruh penanda [[IMAGE: deskripsi]] di content_html.
// Setelah artikel jadi, tiap penanda diganti gambar AI yang benar-benar dibuat
// dan biayanya dipotong per gambar. Gambar utama (cover) TIDAK lewat sini.

// tautImageMarkerRe: penanda gambar inline dari Writer.
var tautImageMarkerRe = regexp.MustCompile(`(?i)\[\[\s*image\s*:\s*([^\]\n]{0,300})\]\]`)

// tautImageInstructionRe: deteksi deterministik bahwa klien memang meminta
// gambar. Backstop supaya gambar tidak dibuat hanya karena model menulis
// penanda tanpa diminta.
var tautImageInstructionRe = regexp.MustCompile(`(?i)\b(gambar|image|images|foto|ilustrasi|infografis)\b`)

// tautMaxInlineImages: batas aman jumlah gambar isi per artikel (biaya + waktu).
const tautMaxInlineImages = 5

// tautImageNegationRe: kata sanggahan. Klien yang menulis "tanpa gambar",
// "jangan pakai gambar", atau "tidak perlu ilustrasi" TIDAK dianggap meminta
// gambar — dulu selalu terdeteksi sehingga muncul estimasi biaya + tagihan
// gambar padahal klien justru menolaknya.
var tautImageNegationRe = regexp.MustCompile(`(?i)\b(tanpa|jangan|tidak|tdk|tak|gak|nggak|hindari|no|without)\b`)

// tautClauseSplitRe: pecah teks per klausa (tanda baca/ganti baris) supaya
// negasi dinilai lokal, tidak menular ke seluruh teks. "tanpa gambar utama,
// tapi tambahkan 2 gambar isi" tetap terdeteksi benar.
var tautClauseSplitRe = regexp.MustCompile(`[.,;!?()\[\]{}"\r\n]+`)

// userRequestedInlineImages: true bila minimal satu klausa menyebut gambar
// TANPA kata sanggahan di klausa yang sama.
func userRequestedInlineImages(topic string) bool {
	for _, clause := range tautClauseSplitRe.Split(topic, -1) {
		if !tautImageInstructionRe.MatchString(clause) {
			continue
		}
		if tautImageNegationRe.MatchString(clause) {
			continue
		}
		return true
	}
	return false
}

// resolveTautInlineImages: ganti setiap penanda [[IMAGE: ...]] dengan gambar AI
// hasil generate. Gambar yang gagal dibuat atau saldo tidak cukup: penandanya
// DIHAPUS (bukan diisi gambar placeholder) supaya artikel tidak berisi gambar
// palsu. Mengembalikan html baru, jumlah gambar jadi, total kredit terpotong,
// dan catatan kegagalan untuk log.
func (d *Deps) resolveTautInlineImages(ctx context.Context, brandID, campaignID, mediaID, userID, html, contextHint string) (string, int, int, []string) {
	if !strings.Contains(strings.ToLower(html), "[[image") {
		return html, 0, 0, nil
	}
	cost := d.effectiveImageCost(ctx)
	var (
		generated int
		totalCost int
		warnings  []string
	)
	out := tautImageMarkerRe.ReplaceAllStringFunc(html, func(match string) string {
		sub := tautImageMarkerRe.FindStringSubmatch(match)
		brief := ""
		if len(sub) > 1 {
			brief = strings.TrimSpace(sub[1])
		}
		if brief == "" {
			return ""
		}
		if generated >= tautMaxInlineImages {
			warnings = append(warnings, "batas "+itoa(tautMaxInlineImages)+" gambar isi per artikel tercapai — penanda berikutnya dilewati")
			return ""
		}
		if remaining, _ := d.imageBalance(ctx, brandID); cost > 0 && remaining < cost {
			warnings = append(warnings, "saldo kredit tidak cukup — gambar \""+brief+"\" dilewati")
			return ""
		}
		prompt := brief
		if strings.TrimSpace(contextHint) != "" {
			prompt = contextHint + " — " + brief
		}
		prompt = truncateString(prompt, 2000)
		started := time.Now()
		res, err := ai.GenerateImage(ctx, d.Pool, ai.GenerateImageOptions{
			Prompt: prompt, Role: "inline", Heading: brief,
			BrandID: &brandID, UserID: &userID,
		})
		if err != nil {
			warnings = append(warnings, "gagal membuat gambar \""+brief+"\": "+err.Error())
			ai.RecordImageEvent(ctx, d.Pool, ai.ImageEvent{
				Kind: "generation", Status: "failed", ErrorDetail: err.Error(), Prompt: prompt, Role: "inline",
				BrandID: brandID, CampaignID: campaignID, MediaID: mediaID, RequestedBy: userID,
				ElapsedMS: int(time.Since(started).Milliseconds()),
			})
			return ""
		}
		url, err := d.storeGeneratedImage(res, firstWords(brief, 4))
		if err != nil {
			warnings = append(warnings, "gagal menyimpan gambar \""+brief+"\": "+err.Error())
			return ""
		}
		charged := 0
		if cost > 0 {
			refID := "ai-image:" + campaignID + ":" + mediaID + ":" + uuid.NewString()
			if _, cErr := d.chargeImageCredit(ctx, brandID, refID, cost, "ai.image.generate", userID); cErr != nil {
				// Gambar tetap dipakai; kegagalan potong kredit dicatat supaya bisa ditelusuri.
				warnings = append(warnings, "gagal memotong kredit gambar \""+brief+"\": "+cErr.Error())
			} else {
				charged = cost
			}
		}
		if err := d.exec(ctx,
			`INSERT INTO article_images (id, brand_id, campaign_id, media_id, article_id, scope_key, url, provider, model, prompt, heading, role, width, height, cost_credits, created_by)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
			uuid.NewString(), brandID, campaignID, mediaID, nil, imageScopeKey(brandID, campaignID, mediaID, ""), url,
			res.Provider, res.Model, prompt, brief, "inline", res.Width, res.Height, charged, userID); err != nil {
			warnings = append(warnings, "gagal mencatat pustaka gambar \""+brief+"\": "+err.Error())
		}
		logStatus, logDetail := "succeeded", ""
		if res.Placeholder {
			logStatus, logDetail = "fallback", res.FallbackReason
		}
		ai.RecordImageEvent(ctx, d.Pool, ai.ImageEvent{
			Kind: "generation", Status: logStatus,
			Provider: res.Provider, Model: res.Model, BaseURL: res.BaseURL,
			ErrorDetail: logDetail, Prompt: prompt, Role: "inline",
			BrandID: brandID, CampaignID: campaignID, MediaID: mediaID, RequestedBy: userID,
			Width: res.Width, Height: res.Height, CreditsCharged: charged, ElapsedMS: res.ElapsedMS,
		})
		generated++
		totalCost += charged
		alt := strings.ReplaceAll(strings.ReplaceAll(brief, `"`, ""), "<", "")
		return `<img src="` + url + `" alt="` + alt + `">`
	})
	return out, generated, totalCost, warnings
}
