package brandflow

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/moderation"
)

// moderation_bridge.go: jembatan tipis ke internal/moderation. Dipisah supaya
// pemanggil di paket ini memakai tipe lokal dan tidak mengimpor paket moderation
// langsung di banyak tempat.

type moderationInput = moderation.Input
type moderationResult = moderation.Result

func moderateContent(ctx context.Context, pool *pgxpool.Pool, in moderationInput) moderationResult {
	return moderation.ModerateContent(ctx, pool, in)
}

func moderationClientSafeMessage(status string) string {
	return moderation.ClientSafeMessage(status)
}

func recordModerationAudit(ctx context.Context, pool *pgxpool.Pool, entityType, entityID string, result moderationResult, brandID *string) {
	moderation.RecordAudit(ctx, pool, entityType, entityID, result, brandID)
}

// ---------- standard check (port lib/standard/service.ts runStandardCheck) ----------

var promotionalTerms = []string{
	"diskon", "discount", "promo", "promosi", "gratis", "beli sekarang", "order sekarang",
	"dapatkan sekarang", "harga spesial", "terbatas", "stok terbatas", "klik di sini", "klik disini",
	"murah", "best price", "hubungi kami", "segera daftar", "bursa", "bundling", "cashback",
}

func countPromotional(text string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, term := range promotionalTerms {
		from := 0
		for {
			idx := strings.Index(lower[from:], term)
			if idx < 0 {
				break
			}
			count++
			from += idx + len(term)
		}
	}
	return count
}

func countWords(text string) int {
	cleaned := reHTMLTag.ReplaceAllString(text, " ")
	cleaned = strings.NewReplacer("#", " ", "*", " ", "_", " ", "`", " ", "~", " ", ">", " ", "-", " ").Replace(cleaned)
	cleaned = strings.TrimSpace(reWhitespace.ReplaceAllString(cleaned, " "))
	if cleaned == "" {
		return 0
	}
	return len(strings.Fields(cleaned))
}

// countQuotes: jumlah pasangan tanda kutip.
func countQuotes(text string) int {
	n := 0
	for _, r := range text {
		switch r {
		case '"', '\u201c', '\u201d', '\u00ab', '\u00bb', '\u201e':
			n++
		}
	}
	return n / 2
}

var toleranceThresholds = map[string][2]int{
	"none":   {1, 1},
	"low":    {1, 4},
	"medium": {3, 7},
	"high":   {6, 11},
}

// runStandardCheck: port runStandardCheck — 7 pengecek deterministik berbobot.
func runStandardCheck(s mediaStandard, articleText string, mediaName any) standardCheckResult {
	checks := []standardCheck{}
	failed := []string{}
	warnings := []string{}
	text := articleText

	words := countWords(text)
	if s.MinWords > 0 && words < s.MinWords {
		checks = append(checks, standardCheck{"length", "Panjang artikel", "fail", itoa(words) + " kata — kurang dari minimum " + itoa(s.MinWords)})
		failed = append(failed, "Panjang artikel ("+itoa(words)+" kata < "+itoa(s.MinWords)+")")
	} else if s.MaxWords > 0 && words > s.MaxWords {
		checks = append(checks, standardCheck{"length", "Panjang artikel", "fail", itoa(words) + " kata — melebihi maksimum " + itoa(s.MaxWords)})
		failed = append(failed, "Panjang artikel ("+itoa(words)+" kata > "+itoa(s.MaxWords)+")")
	} else {
		checks = append(checks, standardCheck{"length", "Panjang artikel", "ok", itoa(words) + " kata (batas " + itoa(s.MinWords) + "-" + itoa(s.MaxWords) + ")"})
	}

	promo := countPromotional(text)
	thresholds, ok := toleranceThresholds[s.HardSellingTol]
	if !ok {
		thresholds = toleranceThresholds["low"]
	}
	if promo >= thresholds[1] {
		checks = append(checks, standardCheck{"hard_selling", "Konten promosi berlebihan", "fail", itoa(promo) + " istilah promosi ditemukan (toleransi " + s.HardSellingTol + ")"})
		failed = append(failed, "Konten promosi ("+itoa(promo)+" istilah)")
	} else if promo >= thresholds[0] {
		checks = append(checks, standardCheck{"hard_selling", "Konten promosi", "warn", itoa(promo) + " istilah promosi — kurangi agar sesuai gaya " + s.JournalisticStyle})
		warnings = append(warnings, "Konten promosi ("+itoa(promo)+" istilah)")
	} else {
		checks = append(checks, standardCheck{"hard_selling", "Konten promosi", "ok", itoa(promo) + " istilah promosi (toleransi " + s.HardSellingTol + ")"})
	}

	quotes := countQuotes(text)
	quoteThreshold := s.MinQuotes
	if s.QuoteRequired {
		quoteThreshold = s.MinQuotes
		if quoteThreshold < 1 {
			quoteThreshold = 1
		}
	}
	if quoteThreshold > 0 && quotes < quoteThreshold {
		checks = append(checks, standardCheck{"quotes", "Kutipan narasumber", "fail", "Ditemukan " + itoa(quotes) + " kutipan — butuh minimal " + itoa(quoteThreshold)})
		failed = append(failed, "Kutipan ("+itoa(quotes)+" < "+itoa(quoteThreshold)+")")
	} else if quoteThreshold > 0 {
		checks = append(checks, standardCheck{"quotes", "Kutipan narasumber", "ok", itoa(quotes) + " kutipan"})
	} else if s.QuoteRequired {
		checks = append(checks, standardCheck{"quotes", "Kutipan narasumber", "fail", "Belum ada kutipan"})
		failed = append(failed, "Kutipan narasumber")
	} else {
		checks = append(checks, standardCheck{"quotes", "Kutipan narasumber", "warn", "Tidak wajib, tapi memperkuat artikel"})
		warnings = append(warnings, "Kutipan narasumber belum ada")
	}

	lowerText := strings.ToLower(text)
	forbiddenHits := []string{}
	for _, term := range s.ForbiddenContent {
		if term != "" && strings.Contains(lowerText, strings.ToLower(term)) {
			forbiddenHits = append(forbiddenHits, term)
		}
	}
	if len(forbiddenHits) > 0 {
		checks = append(checks, standardCheck{"forbidden", "Konten terlarang", "fail", "Ditemukan: " + strings.Join(capStrings(forbiddenHits, 5), ", ")})
		failed = append(failed, "Konten terlarang ("+strings.Join(forbiddenHits, ", ")+")")
	} else {
		checks = append(checks, standardCheck{"forbidden", "Konten terlarang", "ok", "Bersih"})
	}

	if len(s.RequiredInfo) > 0 {
		missing := []string{}
		for _, term := range s.RequiredInfo {
			if term != "" && !strings.Contains(lowerText, strings.ToLower(term)) {
				missing = append(missing, term)
			}
		}
		if len(missing) == 0 {
			checks = append(checks, standardCheck{"required_info", "Informasi wajib", "ok", "Semua tercakup"})
		} else {
			checks = append(checks, standardCheck{"required_info", "Informasi wajib", "warn", "Belum ada: " + strings.Join(capStrings(missing, 5), ", ")})
			warnings = append(warnings, "Informasi wajib belum ada ("+strings.Join(missing, ", ")+")")
		}
	} else {
		checks = append(checks, standardCheck{"required_info", "Informasi wajib", "ok", "Tidak ada ketentuan"})
	}

	hasImage := reImageInText.MatchString(text)
	if s.ImageRequired && !hasImage {
		checks = append(checks, standardCheck{"image", "Gambar pendukung", "warn", "Belum ada gambar — media memintanya"})
		warnings = append(warnings, "Gambar pendukung belum ada")
	} else if s.ImageRequired {
		checks = append(checks, standardCheck{"image", "Gambar pendukung", "ok", "Ada"})
	} else {
		checks = append(checks, standardCheck{"image", "Gambar pendukung", "ok", "Tidak wajib"})
	}

	if s.CategoryRequired {
		checks = append(checks, standardCheck{"category", "Kategori media", "warn", "Pastikan kategori dipilih saat mengirim ke media"})
		warnings = append(warnings, "Kategori media perlu dipilih")
	} else {
		checks = append(checks, standardCheck{"category", "Kategori media", "ok", "Tidak wajib"})
	}

	weights := map[string]int{"length": 20, "hard_selling": 20, "quotes": 15, "forbidden": 25, "required_info": 15, "image": 5, "category": 5}
	achieved := 0
	for _, c := range checks {
		if c.Status == "ok" {
			achieved += weights[c.Key]
		}
	}
	status := "pass"
	if len(failed) > 0 {
		status = "fail"
	} else if len(warnings) > 0 {
		status = "warning"
	}
	return standardCheckResult{
		MediaID: s.MediaID, MediaName: mediaName, Status: status, Score: achieved,
		Checks: checks, Failed: failed, Warnings: warnings,
	}
}
