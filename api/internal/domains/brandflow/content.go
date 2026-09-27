package brandflow

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	authpkg "github.com/rahmaangriawan/tauthub-go/internal/auth"
	contentengine "github.com/rahmaangriawan/tauthub-go/internal/content"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// content.go: port app/v1/brands/[brandId]/content-intakes/**/route.ts
// (ai-fill, auto-key-messages, recommend) + documents/[documentId]/**
// (PATCH metadata, publish, standard-check).

// ---------- intake row + JSON ----------

type bsIntakeRow struct {
	ID              string
	BrandID         string
	Mode            string
	ContentTypeCode *string
	Topic           *string
	Answers         map[string]any
	ArticleText     *string
	ArticleFilename *string
	KeyMessages     []string
	ReadinessScore  *int
	ReadinessChecks []byte
	Recommendation  []byte
	Status          string
	JobID           *string
	CreatedBy       string
	CreatedAt       any
	UpdatedAt       any
}

const bsIntakeColumns = `id, brand_id, mode, content_type_code, topic, answers, article_text, article_filename,
	key_messages, readiness_score, readiness_checks, recommendation, status, job_id, created_by, created_at, updated_at`

// bsLoadIntakeRow: parity intakeDbRow — error "Intake not found" bila tidak ada.
func (d *Deps) bsLoadIntakeRow(ctx context.Context, intakeID string) (bsIntakeRow, error) {
	row, err := d.queryRow(ctx, "SELECT "+bsIntakeColumns+" FROM content_intakes WHERE id = $1 LIMIT 1", intakeID)
	if err != nil {
		return bsIntakeRow{}, err
	}
	if row == nil {
		return bsIntakeRow{}, errors.New("Intake not found")
	}
	var out bsIntakeRow
	out.ID = row.strOr("id", "")
	out.BrandID = row.strOr("brand_id", "")
	out.Mode = row.strOr("mode", "")
	out.ContentTypeCode = row.strPtr("content_type_code")
	out.Topic = row.strPtr("topic")
	if answers, ok := row.jsonField("answers").(map[string]any); ok {
		out.Answers = answers
	}
	if out.Answers == nil {
		out.Answers = map[string]any{}
	}
	out.ArticleText = row.strPtr("article_text")
	out.ArticleFilename = row.strPtr("article_filename")
	out.KeyMessages = jsonStringList(row.jsonField("key_messages"))
	out.ReadinessScore = row.numPtrInt("readiness_score")
	out.ReadinessChecks = bsJSONBytes(row.raw("readiness_checks"))
	out.Recommendation = bsJSONBytes(row.raw("recommendation"))
	out.Status = row.strOr("status", "")
	out.JobID = row.strPtr("job_id")
	out.CreatedBy = row.strOr("created_by", "")
	out.CreatedAt = row.timePtr("created_at")
	out.UpdatedAt = row.timePtr("updated_at")
	return out, nil
}

// bsIntakeJSON: parity intakeJson (_intake.ts).
func bsIntakeJSON(row bsIntakeRow) map[string]any {
	answers := row.Answers
	if answers == nil {
		answers = map[string]any{}
	}
	keyMessages := row.KeyMessages
	if keyMessages == nil {
		keyMessages = []string{}
	}
	return map[string]any{
		"id": row.ID, "brand_id": row.BrandID, "mode": row.Mode,
		"content_type_code": row.ContentTypeCode, "topic": row.Topic,
		"answers": answers, "article_text": row.ArticleText,
		"article_filename": row.ArticleFilename, "readiness_score": row.ReadinessScore,
		"readiness_checks": bsJSONRaw(row.ReadinessChecks, []any{}),
		"recommendation":   bsJSONRaw(row.Recommendation, map[string]any{}),
		"status":           row.Status, "job_id": row.JobID, "key_messages": keyMessages,
		"created_by": row.CreatedBy, "created_at": row.CreatedAt, "updated_at": row.UpdatedAt,
	}
}

// bsJSONBytes: nilai kolom jsonb apa pun → bytes (untuk bsJSONRaw).
func bsJSONBytes(v any) []byte {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return t
	case string:
		return []byte(t)
	case map[string]any, []any:
		b, err := json.Marshal(t)
		if err != nil {
			return nil
		}
		return b
	}
	return nil
}

// numPtrInt: kolom integer nullable → *int (nil bila kolom NULL).
func (m bsRow) numPtrInt(key string) *int {
	if !m.has(key) {
		return nil
	}
	v := m.intOr(key, 0)
	return &v
}

// ---------- prompt intake AI fill ----------

// bsCompanyKeys: field perusahaan/penyelenggara tidak boleh diisi otomatis.
var bsCompanyKeys = map[string]bool{
	"company": true, "organizer": true, "penyelenggara": true, "perusahaan": true,
	"nama_perusahaan": true, "nama_organisasi": true, "organisasi": true, "institution": true,
}

// bsEmptyAnswerValue: true bila nilai jawaban dianggap kosong.
func bsEmptyAnswerValue(value any) bool {
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	if arr, ok := value.([]any); ok {
		return len(arr) == 0
	}
	return value == nil
}

// bsStringifyAnswerValue: nilai jawaban → teks (parity closure app lama).
func bsStringifyAnswerValue(value any) string {
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	if arr, ok := value.([]any); ok {
		parts := []string{}
		for _, item := range arr {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		return strings.Join(parts, ", ")
	}
	return ""
}

// bsParseJSONObject: parse JSON object (toleran pagar ```json), parity
// `JSON.parse(raw.replace(/^```(?:json)?\s*/i,"").replace(/\s*```$/i,""))`.
func bsParseJSONObject(raw string) (map[string]any, bool) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(bsStripAIFence(raw))), &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

func bsStringifyAny(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return bsNumberToString(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func bsNumberToString(f float64) string {
	if f == float64(int64(f)) {
		return itoa(int(f))
	}
	b, _ := json.Marshal(f)
	return string(b)
}

// ---------- content-intakes/{intakeId}/ai-fill ----------

// ContentIntakeAIFill: POST /v1/brands/{brandId}/content-intakes/{intakeId}/ai-fill
func (d *Deps) ContentIntakeAIFill(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	// Parity route lama: `brand(brandId, ...)` hanya cek membership, lalu
	// `row.brand_id !== brandId` dibandingkan dengan param path APA ADANYA
	// (bukan id kanonik) — dipertahankan supaya perilaku identik.
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	brandID := bfBrand.ID
	intakeID := chiParam(r, "intakeId")
	row, err := d.bsLoadIntakeRow(r.Context(), intakeID)
	if err != nil {
		// parity `throw new Error("Intake not found")` → errorResponse 500.
		d.writeErr(w, err)
		return
	}
	if row.BrandID != brandID {
		d.writeErr(w, errors.New("Intake not found"))
		return
	}

	body := decodeJSON(r)
	// bodySchema: topic trim 3..2000; type_code trim max 50 optional;
	// answers record optional; key_messages array(trim max500) max20 optional.
	topic, err := bsRequiredTopic(body, "topic", 3, 2000)
	if err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Masukkan topik/judul naskah terlebih dahulu.")
		return
	}
	typeCode := ""
	if raw, ok := body["type_code"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr {
			httpx.Error(w, http.StatusUnprocessableEntity, "Masukkan topik/judul naskah terlebih dahulu.")
			return
		}
		if len([]rune(s)) > 50 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Masukkan topik/judul naskah terlebih dahulu.")
			return
		}
		typeCode = strings.TrimSpace(s)
	}
	answersOverride := map[string]any{}
	if raw, ok := body["answers"]; ok && raw != nil {
		m, isMap := raw.(map[string]any)
		if !isMap {
			httpx.Error(w, http.StatusUnprocessableEntity, "Masukkan topik/judul naskah terlebih dahulu.")
			return
		}
		answersOverride = m
	}
	keyMessagesInput, okKeyMessages := bsOptionalStringArray(body, "key_messages", 500, 20)
	if !okKeyMessages {
		httpx.Error(w, http.StatusUnprocessableEntity, "Masukkan topik/judul naskah terlebih dahulu.")
		return
	}

	// Jenis konten: pilihan aktif di form menang, DB sebagai fallback.
	effectiveTypeCode := typeCode
	if effectiveTypeCode == "" && row.ContentTypeCode != nil {
		effectiveTypeCode = *row.ContentTypeCode
	}
	var intakeType *contentengine.IntakeType
	if effectiveTypeCode != "" {
		intakeType = contentengine.LoadIntakeType(r.Context(), d.Pool, effectiveTypeCode)
	}
	if intakeType == nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Pilih jenis naskah terlebih dahulu.")
		return
	}
	questions := intakeType.Questions

	existingAnswers := map[string]any{}
	for k, v := range row.Answers {
		existingAnswers[k] = v
	}
	for k, v := range answersOverride {
		existingAnswers[k] = v
	}
	emptyQuestions := []contentengine.IntakeQuestion{}
	for _, q := range questions {
		if bsEmptyAnswerValue(existingAnswers[q.Key]) {
			emptyQuestions = append(emptyQuestions, q)
		}
	}

	existingKeyMessages := []string{}
	if len(keyMessagesInput) > 0 {
		for _, m := range keyMessagesInput {
			if strings.TrimSpace(m) != "" {
				existingKeyMessages = append(existingKeyMessages, m)
			}
		}
	} else {
		for _, m := range row.KeyMessages {
			if strings.TrimSpace(m) != "" {
				existingKeyMessages = append(existingKeyMessages, m)
			}
		}
	}
	needKeyMessages := len(existingKeyMessages) == 0

	if len(emptyQuestions) == 0 && !needKeyMessages {
		writeJSON(w, http.StatusOK, map[string]any{
			"answers": existingAnswers, "filled": []string{}, "skipped": 0,
		})
		return
	}

	// TC: Isi Otomatis (intake + key messages) — 2 TC sekali generate.
	if err := d.adjustCredit(r.Context(), brandID, -2, "Isi Otomatis AI — intake + key messages", user.ID, ""); err != nil {
		var ic *insufficientCreditsError
		if asInsufficient(err, &ic) {
			writeJSON(w, http.StatusPaymentRequired, map[string]any{
				"detail": "Saldo TC tidak cukup. Butuh 2 TC, tersedia " + itoa(ic.Available) + " TC. Silakan Top Up.",
				"code":   "insufficient_credits", "needed": 2, "available": ic.Available,
			})
			return
		}
		d.writeErr(w, err)
		return
	}

	brandRow, _ := d.queryRow(r.Context(),
		"SELECT name, industry, client_name FROM brands WHERE id = $1 LIMIT 1", brandID)
	campaignRow, _ := d.queryRow(r.Context(),
		"SELECT c.name AS campaign_name, c.description AS campaign_description FROM campaigns c WHERE c.content_intake_id = $1 AND c.status <> 'cancelled' LIMIT 1",
		intakeID)

	// Perusahaan/penyelenggara TIDAK boleh diisi otomatis.
	filtered := []contentengine.IntakeQuestion{}
	for _, q := range emptyQuestions {
		if !bsCompanyKeys[strings.ToLower(q.Key)] {
			filtered = append(filtered, q)
		}
	}
	emptyQuestions = filtered

	brandName := ""
	brandIndustry := ""
	brandClientName := ""
	if brandRow != nil {
		brandName = brandRow.strOr("name", "")
		brandIndustry = brandRow.strOr("industry", "")
		brandClientName = brandRow.strOr("client_name", "")
	}
	campaignName := ""
	campaignDescription := ""
	if campaignRow != nil {
		campaignName = campaignRow.strOr("campaign_name", "")
		campaignDescription = campaignRow.strOr("campaign_description", "")
	}

	filledContextParts := []string{}
	for _, q := range questions {
		text := bsStringifyAnswerValue(existingAnswers[q.Key])
		if text != "" {
			filledContextParts = append(filledContextParts, "- "+q.Label+": "+text)
		}
	}
	filledContext := strings.Join(filledContextParts, "\n")

	keyMessagesText := ""
	if len(keyMessagesInput) > 0 {
		parts := []string{}
		for _, m := range keyMessagesInput {
			if strings.TrimSpace(m) != "" {
				parts = append(parts, m)
			}
		}
		keyMessagesText = strings.Join(parts, "; ")
	} else if len(row.KeyMessages) > 0 {
		parts := []string{}
		for _, m := range row.KeyMessages {
			if strings.TrimSpace(m) != "" {
				parts = append(parts, m)
			}
		}
		keyMessagesText = strings.Join(parts, "; ")
	}

	fieldParts := []string{}
	for _, q := range emptyQuestions {
		required := "opsional"
		if q.Required {
			required = "wajib"
		}
		lines := []string{"- " + q.Key + " | " + q.Label + " | " + q.Type + " | " + required}
		if q.Helper != nil && *q.Helper != "" {
			lines = append(lines, "  helper: "+*q.Helper)
		}
		if q.Placeholder != nil && *q.Placeholder != "" {
			lines = append(lines, "  placeholder: "+*q.Placeholder)
		}
		if len(q.Options) > 0 {
			lines = append(lines, "  pilihan: "+strings.Join(q.Options, " / "))
		}
		fieldParts = append(fieldParts, strings.Join(lines, "\n"))
	}
	fieldList := strings.Join(fieldParts, "\n")

	brandLine := "Brand: (tidak diketahui)"
	if brandName != "" {
		brandLine = "Brand: " + brandName
		if brandIndustry != "" {
			brandLine += " · industri " + brandIndustry
		}
		if brandClientName != "" {
			brandLine += "\nKlien brand: " + truncateString(brandClientName, 500)
		}
	} else if brandIndustry != "" {
		brandLine = "Brand: (tidak diketahui) · industri " + brandIndustry
	}
	typeLine := "Jenis konten: " + intakeType.Name
	if intakeType.Description != nil && *intakeType.Description != "" {
		typeLine += " — " + *intakeType.Description
	}
	promptLines := []string{
		"Anda asisten pengisian brief konten profesional. Tugas: menganalisis topik yang diberikan klien lalu mengisi field brief yang MASIH KOSONG untuk jenis konten yang dipilih.",
		"",
		"== KONTEKS ==",
		brandLine,
	}
	if campaignRow != nil {
		line := "Kampanye tujuan: " + campaignName
		if campaignDescription != "" {
			line += " — " + truncateString(campaignDescription, 500)
		}
		promptLines = append(promptLines, line, "Semua isian harus selaras dengan tujuan kampanye ini.")
	}
	promptLines = append(promptLines, typeLine, "Topik dari klien: "+topic)
	if keyMessagesText != "" {
		promptLines = append(promptLines, "Pesan kunci wajib tercakup: "+keyMessagesText)
	}
	if filledContext != "" {
		promptLines = append(promptLines, "", "== FAKTA DARI USER (jangan diubah/dikarang ulang, gunakan apa adanya) ==", filledContext)
	}
	promptLines = append(promptLines,
		"",
		"== SCHEMA FIELD (satu-satunya field yang boleh diisi — jangan buat key baru) ==",
		fieldList,
		"",
		"== ATURAN ==",
		"1. Analisis topik klien, lalu isi field yang kosong dengan informasi yang DAPAT disimpulkan langsung dari topik.",
		"2. Untuk fakta yang TIDAK tersedia di topik (misal: tanggal, lokasi, nama pembicara, biaya, jumlah peserta) balas null. JANGAN mengarang fakta spesifik.",
		"3. Input manual klien (fakta user di atas) tidak boleh ditimpa atau diubah.",
		"4. Balas HANYA JSON object: key = kode field persis seperti di schema, value = isi atau null.",
		"5. Jangan mengisi field Perusahaan/penyelenggara (key company/penyelenggara/perusahaan/organisasi/organizer/institution) — balas null untuk field itu.",
	)

	result, genErr := ai.GenerateChat(r.Context(), d.Pool, ai.GenerateChatOptions{
		Prompt: strings.Join(promptLines, "\n"), Tier: "default",
		TaskType: "intake_ai_fill", BrandID: &brandID, UserID: &user.ID,
	})
	if genErr != nil {
		d.writeErr(w, genErr)
		return
	}

	merged := map[string]any{}
	for k, v := range existingAnswers {
		merged[k] = v
	}
	filled := []string{}
	json, ok := bsParseJSONObject(result.Content)
	if !ok {
		httpx.Error(w, http.StatusBadGateway, "AI tidak menghasilkan jawaban yang valid. Coba lagi.")
		return
	}
	for _, q := range emptyQuestions {
		value, has := json[q.Key]
		if !has || value == nil {
			// Fallback: AI kadang memakai label Indonesia sebagai key.
			value, has = json[q.Label]
		}
		if !has || value == nil {
			// Sparse match: cari entry yang key-nya memuat kode/label field.
			lowerKey := strings.ToLower(q.Key)
			lowerLabel := strings.ToLower(strings.TrimSpace(q.Label))
			for k, v := range json {
				norm := strings.ToLower(strings.TrimSpace(k))
				if strings.Contains(norm, lowerKey) || (lowerLabel != "" && norm == lowerLabel) {
					value = v
					has = true
					break
				}
			}
		}
		if !has || value == nil {
			continue
		}
		normalized := value
		if s, isStr := value.(string); isStr {
			normalized = strings.TrimSpace(s)
			if normalized == "" {
				continue
			}
		}
		merged[q.Key] = normalized
		filled = append(filled, q.Key)
	}

	if len(filled) == 0 && !needKeyMessages {
		httpx.Error(w, http.StatusBadGateway, "AI tidak mengenali field yang tersedia. Coba lagi dengan topik lebih spesifik.")
		return
	}

	// Sekalian generate key messages (masih dalam 1 charge 2 TC).
	generatedKeyMessages := []string{}
	if needKeyMessages {
		kmLines := []string{
			"Anda adalah editor konten profesional. Tugas: menyusun 3-5 key messages (pesan kunci) untuk artikel berdasarkan topik dan bahan berikut.",
			"",
		}
		if brandName != "" {
			kmLines = append(kmLines, "Brand: "+brandName)
		} else {
			kmLines = append(kmLines, "Brand: (tidak diketahui)")
		}
		kmLines = append(kmLines, "Topik: "+topic)
		if filledContext != "" {
			kmLines = append(kmLines, "", "Bahan:", filledContext)
		}
		kmLines = append(kmLines, "", "Balas HANYA JSON array of strings, contoh: [\"Pesan kunci 1\", \"Pesan kunci 2\"]")
		if kmResult, kmErr := ai.GenerateChat(r.Context(), d.Pool, ai.GenerateChatOptions{
			Prompt: strings.Join(kmLines, "\n"), Tier: "default",
			TaskType: "intake_key_messages", BrandID: &brandID, UserID: &user.ID,
		}); kmErr == nil {
			generatedKeyMessages = bsParseStringArrayLenient(kmResult.Content, 5)
			if generatedKeyMessages == nil {
				generatedKeyMessages = []string{}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"answers": merged, "filled": filled, "key_messages": generatedKeyMessages,
	})
}

// ---------- content-intakes/{intakeId}/auto-key-messages ----------

// ContentIntakeAutoKeyMessages: POST /v1/brands/{brandId}/content-intakes/{intakeId}/auto-key-messages
func (d *Deps) ContentIntakeAutoKeyMessages(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	brandID := bfBrand.ID
	intakeID := chiParam(r, "intakeId")
	row, err := d.bsLoadIntakeRow(r.Context(), intakeID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row.BrandID != brandID {
		d.writeErr(w, errors.New("Intake not found"))
		return
	}

	body := decodeJSON(r)
	// bodySchema: topic trim 3..2000; answers record optional; article_text max
	// 40000 optional; media_id max 36 optional; media_type enum optional;
	// existing_key_messages array(trim max500) max20 optional.
	topic, err := bsRequiredTopic(body, "topic", 3, 2000)
	if err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Masukkan topik/judul naskah terlebih dahulu.")
		return
	}
	answers := map[string]any{}
	if raw, ok := body["answers"]; ok && raw != nil {
		m, isMap := raw.(map[string]any)
		if !isMap {
			httpx.Error(w, http.StatusUnprocessableEntity, "Masukkan topik/judul naskah terlebih dahulu.")
			return
		}
		answers = m
	}
	articleText := ""
	if raw, ok := body["article_text"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr || len([]rune(s)) > 40000 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Masukkan topik/judul naskah terlebih dahulu.")
			return
		}
		articleText = s
	}
	mediaID := ""
	if raw, ok := body["media_id"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr || len([]rune(s)) > 36 {
			httpx.Error(w, http.StatusUnprocessableEntity, "Masukkan topik/judul naskah terlebih dahulu.")
			return
		}
		mediaID = s
	}
	mediaType := ""
	if raw, ok := body["media_type"]; ok && raw != nil {
		s, isStr := raw.(string)
		if !isStr || (s != "blogger" && s != "mainstream") {
			httpx.Error(w, http.StatusUnprocessableEntity, "Masukkan topik/judul naskah terlebih dahulu.")
			return
		}
		mediaType = s
	}
	existingInput, okExisting := bsOptionalStringArray(body, "existing_key_messages", 500, 20)
	if !okExisting {
		httpx.Error(w, http.StatusUnprocessableEntity, "Masukkan topik/judul naskah terlebih dahulu.")
		return
	}

	var mediaTypePtr *string
	if mediaType != "" {
		mediaTypePtr = &mediaType
	}
	standardSource, resolvedMediaName := d.bsResolveStandardSource(r.Context(), mediaID, mediaTypePtr)
	mediaName := resolvedMediaName
	if mediaID != "" && mediaName == "" {
		if mediaRow, qErr := d.queryRow(r.Context(), "SELECT name FROM media WHERE id = $1 LIMIT 1", mediaID); qErr == nil && mediaRow != nil {
			mediaName = mediaRow.strOr("name", "")
		}
	}
	brandRow, _ := d.queryRow(r.Context(), "SELECT name, industry FROM brands WHERE id = $1 LIMIT 1", brandID)

	existingMessages := []string{}
	if len(existingInput) > 0 {
		for _, m := range existingInput {
			if strings.TrimSpace(m) != "" {
				existingMessages = append(existingMessages, m)
			}
		}
	} else {
		for _, m := range row.KeyMessages {
			if strings.TrimSpace(m) != "" {
				existingMessages = append(existingMessages, m)
			}
		}
	}

	answerLines := ""
	keys := sortedKeys(answers)
	parts := []string{}
	for _, k := range keys {
		v := answers[k]
		s, isStr := v.(string)
		if !isStr || strings.TrimSpace(s) == "" {
			continue
		}
		parts = append(parts, "- "+k+": "+truncateString(s, 300))
	}
	answerLines = strings.Join(parts, "\n")

	standardLine := "Standard editorial global"
	switch standardSource {
	case "media_specific":
		name := mediaName
		if name == "" {
			name = "Media"
		}
		standardLine = "Standard khusus media: " + name
	case "mainstream_fallback":
		standardLine = "Standard Jurnalistik Mainstream (fallback)"
	case "blogger_fallback":
		standardLine = "Standard Blog / Evergreen (fallback)"
	}

	brandName := ""
	brandIndustry := ""
	if brandRow != nil {
		brandName = brandRow.strOr("name", "")
		brandIndustry = brandRow.strOr("industry", "")
	}
	promptLines := []string{
		"Anda adalah editor konten profesional di platform Tauthub. Tugas: menyusun key messages (pesan kunci) yang harus tersampaikan dalam sebuah artikel.",
		"",
		"== KONTEKS ==",
	}
	if brandName != "" {
		line := "Brand: " + brandName
		if brandIndustry != "" {
			line += " · industri " + brandIndustry
		}
		promptLines = append(promptLines, line)
	} else {
		promptLines = append(promptLines, "Brand: (tidak diketahui)")
	}
	promptLines = append(promptLines, "Topik: "+topic)
	if mediaName != "" {
		line := "Media target: " + mediaName
		if mediaType == "blogger" {
			line += " (Media Blogger)"
		} else if mediaType == "mainstream" {
			line += " (Media Mainstream)"
		}
		promptLines = append(promptLines, line)
	} else if mediaType != "" {
		if mediaType == "blogger" {
			promptLines = append(promptLines, "Media type: Media Blogger")
		} else {
			promptLines = append(promptLines, "Media type: Media Mainstream")
		}
	}
	promptLines = append(promptLines, "Standard editorial: "+standardLine)
	if answerLines != "" {
		promptLines = append(promptLines, "", "== BAHAN / JAWABAN INTAKE ==", answerLines)
	}
	if articleText != "" {
		promptLines = append(promptLines, "", "== TEKS ARTIKEL (ringkas) ==", truncateString(articleText, 3000))
	}
	if len(existingMessages) > 0 {
		msgs := make([]string, 0, len(existingMessages))
		for _, m := range existingMessages {
			msgs = append(msgs, "- "+m)
		}
		promptLines = append(promptLines, "", "== KEY MESSAGES EXISTING (pertahankan bila masih relevan) ==", strings.Join(msgs, "\n"))
	}
	promptLines = append(promptLines,
		"",
		"== OUTPUT ==",
		"Buat 3-5 key messages yang harus tersampaikan dalam artikel ini. Setiap key message:",
		"1. Satu kalimat yang jelas dan ringkas.",
		"2. Relevan dengan audiens media target.",
		"3. Selaras dengan tujuan konten dan bahan yang diberikan.",
		"4. Bukan klaim promosi tanpa atribusi.",
		"5. Tidak mengarang fakta yang tidak ada di bahan.",
		"",
		"Balas HANYA JSON array of strings, contoh: [\"Pesan kunci 1\", \"Pesan kunci 2\"]",
	)

	result, genErr := ai.GenerateChat(r.Context(), d.Pool, ai.GenerateChatOptions{
		Prompt: strings.Join(promptLines, "\n"), Tier: "default",
		TaskType: "intake_key_messages", BrandID: &brandID, UserID: &user.ID,
	})
	if genErr != nil {
		d.writeErr(w, genErr)
		return
	}
	messages := bsParseStringArrayLenient(result.Content, 5)
	if len(messages) == 0 {
		httpx.Error(w, http.StatusBadGateway, "AI tidak menghasilkan key messages yang valid. Coba lagi.")
		return
	}

	// Simpan langsung ke intake (merge, maks 20, unik).
	existingRow, _ := d.queryRow(r.Context(), "SELECT key_messages FROM content_intakes WHERE id = $1 LIMIT 1", intakeID)
	existingArr := []string{}
	if existingRow != nil {
		existingArr = jsonStringList(existingRow.jsonField("key_messages"))
	}
	merged := bsDedupeStrings(append(append([]string{}, existingArr...), messages...))
	if len(merged) > 20 {
		merged = merged[:20]
	}
	_ = d.exec(r.Context(),
		"UPDATE content_intakes SET key_messages = $1::jsonb, updated_at = NOW() WHERE id = $2",
		marshalJSONArr(merged), intakeID)

	writeJSON(w, http.StatusOK, map[string]any{"key_messages": messages})
}

// ---------- content-intakes/{intakeId}/recommend ----------

// ContentIntakeRecommend: POST /v1/brands/{brandId}/content-intakes/{intakeId}/recommend
func (d *Deps) ContentIntakeRecommend(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	brandID := bfBrand.ID
	intakeID := chiParam(r, "intakeId")
	row, err := d.bsLoadIntakeRow(r.Context(), intakeID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row.BrandID != brandID {
		d.writeErr(w, errors.New("Intake not found"))
		return
	}

	var intakeType *contentengine.IntakeType
	if row.ContentTypeCode != nil && *row.ContentTypeCode != "" {
		intakeType = contentengine.LoadIntakeType(r.Context(), d.Pool, *row.ContentTypeCode)
	}
	readiness := contentengine.ComputeReadiness(intakeType, row.Mode, row.Answers, row.Topic, row.ArticleText)

	brandRow, _ := d.queryRow(r.Context(), "SELECT name, industry FROM brands WHERE id = $1 LIMIT 1", brandID)
	brandName := "Brand"
	brandIndustry := ""
	if brandRow != nil {
		brandName = brandRow.strOr("name", "Brand")
		brandIndustry = brandRow.strOr("industry", "")
	}

	recommendation := d.bsGenerateRecommendation(r.Context(), bsRecommendationInput{
		BrandName: brandName, BrandIndustry: brandIndustry, Type: intakeType,
		Topic: derefPtr(row.Topic), Answers: row.Answers, Readiness: readiness, UserID: user.ID,
	})
	_ = d.exec(r.Context(),
		"UPDATE content_intakes SET recommendation = $1::jsonb, updated_at = NOW() WHERE id = $2",
		marshalJSONMap(recommendation), intakeID)

	out := bsIntakeJSON(row)
	out["recommendation"] = recommendation
	out["readiness"] = readiness
	writeJSON(w, http.StatusOK, out)
}

// bsRecommendation: parity interface Recommendation (lib/intake/recommend.ts).
type bsRecommendation struct {
	Angle             string   `json:"angle"`
	TopicSuggestion   string   `json:"topic_suggestion"`
	Structure         []string `json:"structure"`
	MissingInfo       []string `json:"missing_info"`
	FollowUpQuestions []string `json:"follow_up_questions"`
	Notes             string   `json:"notes"`
}

type bsRecommendationInput struct {
	BrandName     string
	BrandIndustry string
	Type          *contentengine.IntakeType
	Topic         string
	Answers       map[string]any
	Readiness     contentengine.ReadinessResult
	UserID        string
}

var bsAngleByType = map[string]string{
	"press_release":     "Fokus pada nilai berita: dampak bagi pembaca/industri, bukan sekadar pengumuman.",
	"event":             "Fokus pada manfaat bagi calon peserta dan momen utama acara.",
	"csr":               "Fokus pada dampak sosial yang terukur dan pihak yang terbantu.",
	"product_launch":    "Fokus pada masalah yang dipecahkan produk dan keunggulan kompetitifnya.",
	"personal_branding": "Fokus pada kisah perjalanan dan nilai yang dibawa tokoh.",
}

var bsStructureByType = map[string][]string{
	"press_release": {
		"Lead 5W+1H (siapa, apa, kapan, di mana, mengapa)",
		"Detail utama & konteks", "Kutipan narasumber", "Data/fakta pendukung",
		"Tentang perusahaan (boilerplate)", "Kontak media / CTA",
	},
	"event": {
		"Pembuka: apa dan kapan acara", "Agenda / rangkaian acara",
		"Narasumber / pembicara", "Cara ikut (lokasi/link)", "Info tambahan & CTA",
	},
	"csr": {
		"Pembuka: program dan perusahaan", "Apa yang dilakukan",
		"Dampak & penerima manfaat", "Data pendukung", "Kutipan", "Penutup CTA",
	},
	"product_launch": {
		"Pembuka: produk diluncurkan", "Fitur / keunggulan utama",
		"Harga & ketersediaan", "Kutipan perusahaan", "Data / riset", "CTA pembelian",
	},
	"personal_branding": {
		"Pembuka profil tokoh", "Perjalanan / pencapaian", "Visi & dampak",
		"Kutipan tokoh", "Kontak / CTA",
	},
}

var bsFollowUpByType = map[string][]string{
	"press_release": {
		"Siapa narasumber utama yang bisa dikutip?",
		"Apakah ada angka/statistik pendukung?",
		"Apakah ada tanggal pelaksanaan/pengumuman resmi?",
		"Apakah ada kutipan resmi dari pihak terkait?",
	},
	"event": {
		"Kapan dan di mana acara berlangsung?",
		"Siapa pembicara/penyelenggara kunci?",
		"Apakah ada agenda atau susunan acara?",
		"Bagaimana cara peserta mendaftar?",
	},
	"csr": {
		"Siapa penerima manfaat program?",
		"Berapa nilai atau skala programnya?",
		"Apakah ada kutipan dari perusahaan atau penerima manfaat?",
		"Kapan dan di mana kegiatan berlangsung?",
	},
	"product_launch": {
		"Kapan produk resmi diluncurkan?",
		"Apa fitur utama yang membedakan dari kompetitor?",
		"Berapa harga dan di mana bisa didapat?",
		"Apakah ada data riset atau testimoni?",
	},
	"personal_branding": {
		"Apa pencapaian terbesar tokoh sejauh ini?",
		"Apakah ada penghargaan atau angka yang bisa dikutip?",
		"Apa visi/misi yang ingin disampaikan?",
		"Apakah ada kutipan langsung dari tokoh?",
	},
}

// bsDeterministicRecommendation: parity deterministicRecommendation.
func bsDeterministicRecommendation(in bsRecommendationInput) bsRecommendation {
	code := ""
	typeName := "Artikel"
	if in.Type != nil {
		code = in.Type.Code
		typeName = in.Type.Name
	}
	provided := []string{}
	keys := sortedKeys(in.Answers)
	for _, k := range keys {
		text := bsStringifyAny(in.Answers[k])
		if text != "" {
			provided = append(provided, k+": "+text)
		}
	}
	missingLabels := in.Readiness.Missing
	if len(missingLabels) == 0 {
		missingLabels = []string{"Data pendukung", "Kutipan narasumber"}
	}
	angle, okAngle := bsAngleByType[code]
	if !okAngle {
		angle = "Fokus pada informasi yang paling bernilai bagi pembaca, dengan struktur yang jelas dan fakta yang bisa diverifikasi."
	}
	topicSuggestion := "Ide topik " + strings.ToLower(typeName) + " untuk brand Anda — mulai dari satu momen/kejadian yang paling ingin diberitakan"
	if len(strings.TrimSpace(in.Topic)) > 2 {
		topicSuggestion = strings.TrimSpace(in.Topic)
	}
	structure, okStructure := bsStructureByType[code]
	if !okStructure {
		structure = []string{"Pembuka yang menarik", "Informasi utama", "Detail pendukung", "Kutipan", "Penutup CTA"}
	}
	followUp, okFollowUp := bsFollowUpByType[code]
	if !okFollowUp {
		followUp = []string{"Apa momen utama yang ingin diberitakan?", "Siapa pihak yang terlibat?", "Apakah ada data atau kutipan?"}
	}
	notes := "Belum ada bahan yang terisi — rekomendasi disusun dari jenis konten yang dipilih."
	if len(provided) > 0 {
		notes = "Bahan yang sudah tersedia: " + strings.Join(provided, "; ")
	}
	return bsRecommendation{
		Angle: angle, TopicSuggestion: topicSuggestion, Structure: structure,
		MissingInfo: missingLabels, FollowUpQuestions: followUp, Notes: notes,
	}
}

// bsGenerateRecommendation: parity generateRecommendation (AI + fallback
// deterministik).
func (d *Deps) bsGenerateRecommendation(ctx context.Context, in bsRecommendationInput) bsRecommendation {
	deterministic := bsDeterministicRecommendation(in)
	runtime, err := ai.LoadAIRuntime(ctx, d.Pool)
	if err != nil || runtime == nil || runtime.BaseURL == "" || runtime.APIKey == "" {
		return deterministic
	}
	typeName := "Artikel"
	if in.Type != nil {
		typeName = in.Type.Name
	}
	filledParts := []string{}
	keys := sortedKeys(in.Answers)
	for _, k := range keys {
		text := bsStringifyAny(in.Answers[k])
		if text != "" {
			filledParts = append(filledParts, "- "+k+": "+text)
		}
	}
	filled := strings.Join(filledParts, "\n")
	if filled == "" {
		filled = "(belum ada)"
	}
	industry := ""
	if in.BrandIndustry != "" {
		industry = " (industri: " + in.BrandIndustry + ")"
	}
	topicText := strings.TrimSpace(in.Topic)
	if topicText == "" {
		topicText = "(belum ada)"
	}
	missing := strings.Join(in.Readiness.Missing, ", ")
	if missing == "" {
		missing = "tidak ada"
	}
	prompt := "Anda adalah content strategist Tulisio. Buat rekomendasi konten untuk klien.\n" +
		"Brand: " + in.BrandName + industry + "\n" +
		"Jenis konten: " + typeName + "\n" +
		"Topik (jika ada): " + topicText + "\n" +
		"Bahan yang sudah diberikan:\n" + filled + "\n" +
		"Informasi yang masih kurang: " + missing + "\n\n" +
		"Balas HANYA JSON (tanpa markdown) dengan bentuk:\n" +
		"{\n" +
		"  \"angle\": \"sudut pemberitaan yang kuat dalam 1-2 kalimat\",\n" +
		"  \"topic_suggestion\": \"usulan topik/judul kerja\",\n" +
		"  \"structure\": [\"bagian 1\", \"bagian 2\", \"...\"],\n" +
		"  \"missing_info\": [\"info yang perlu dilengkapi\"],\n" +
		"  \"follow_up_questions\": [\"pertanyaan lanjutan untuk user\"],\n" +
		"  \"notes\": \"catatan singkat\"\n" +
		"}"
	result, genErr := ai.GenerateChat(ctx, d.Pool, ai.GenerateChatOptions{
		Prompt: prompt, Tier: "default", TaskType: "content_recommendation",
		BrandID: nil, UserID: &in.UserID,
	})
	if genErr != nil {
		return deterministic
	}
	parsed, ok := bsReadJSONBlock(result.Content)
	if !ok {
		return deterministic
	}
	angle, hasAngle := parsed["angle"].(string)
	if !hasAngle {
		return deterministic
	}
	out := bsRecommendation{}
	out.Angle = truncateString(angle, 1000)
	if s, ok := parsed["topic_suggestion"].(string); ok {
		out.TopicSuggestion = truncateString(s, 300)
	} else {
		out.TopicSuggestion = deterministic.TopicSuggestion
	}
	out.Structure = bsAiStringList(parsed["structure"], deterministic.Structure, 300, 12)
	out.MissingInfo = bsAiStringList(parsed["missing_info"], deterministic.MissingInfo, 200, 12)
	out.FollowUpQuestions = bsAiStringList(parsed["follow_up_questions"], deterministic.FollowUpQuestions, 300, 8)
	if s, ok := parsed["notes"].(string); ok {
		out.Notes = truncateString(s, 1000)
	} else {
		out.Notes = deterministic.Notes
	}
	return out
}

// bsAiStringList: array nilai → []string terpotong, atau fallback.
func bsAiStringList(v any, fallback []string, maxLen, cap int) []string {
	arr, ok := v.([]any)
	if !ok {
		return fallback
	}
	out := []string{}
	for _, item := range arr {
		if len(out) >= cap {
			break
		}
		out = append(out, truncateString(bsStringifyAny(item), maxLen))
	}
	return out
}

// bsReadJSONBlock: paritas readJsonBlock — blok { ... } terluar.
func bsReadJSONBlock(content string) (map[string]any, bool) {
	trimmed := strings.TrimSpace(content)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(trimmed[start:end+1]), &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

// ---------- documents ----------

// ContentDocumentPatch: PATCH /v1/brands/{brandId}/documents/{documentId}
func (d *Deps) ContentDocumentPatch(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	// Parity route lama: scopedDocument memakai param path brandId apa adanya.
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.contentErr(w, err)
		return
	}
	brandID := bfBrand.ID
	documentID := chiParam(r, "documentId")
	body := decodeJSON(r)
	// metadataSchema: title 1..300 wajib; slug max300 nullable optional;
	// meta_title max255 nullable optional; meta_description max500 nullable optional.
	title, err := bsZodStringRaw(body, "title", 1, 300, false, false)
	if err != nil {
		d.contentErr(w, err)
		return
	}
	slug, err := bsNullableString(body, "slug", 300)
	if err != nil {
		d.contentErr(w, err)
		return
	}
	metaTitle, err := bsNullableString(body, "meta_title", 255)
	if err != nil {
		d.contentErr(w, err)
		return
	}
	metaDescription, err := bsNullableString(body, "meta_description", 500)
	if err != nil {
		d.contentErr(w, err)
		return
	}
	document, err := d.scopedDocument(r.Context(), brandID, documentID)
	if err != nil {
		d.contentErr(w, err)
		return
	}

	if title != document.strOr("title", "") {
		current, err := d.queryRow(r.Context(),
			"SELECT content FROM content_versions WHERE document_id = $1 AND version = $2 LIMIT 1",
			documentID, document.intOr("current_version", 1))
		if err != nil {
			d.contentErr(w, err)
			return
		}
		if current == nil {
			d.contentErr(w, errConflict("Current content version is missing"))
			return
		}
		heading := "<h1>" + strings.TrimSpace(title) + "</h1>"
		html := current.strOr("content", "")
		updated := heading + html
		if bsArticleTitle(html) != nil {
			updated = reH1Lazy.ReplaceAllString(html, heading)
		}
		nextVersion := document.intOr("current_version", 1) + 1
		if err := d.exec(r.Context(),
			"INSERT INTO content_versions (id, document_id, version, content, content_format, attribution, change_note, created_by) VALUES ($1, $2, $3, $4, 'html', 'human_edited', 'Updated article title', $5)",
			uuid.NewString(), documentID, nextVersion, updated, user.ID); err != nil {
			d.contentErr(w, err)
			return
		}
		if err := d.exec(r.Context(),
			"UPDATE content_documents SET current_version = $1, status = 'draft', approved_by = NULL, approved_at = NULL WHERE id = $2",
			nextVersion, documentID); err != nil {
			d.contentErr(w, err)
			return
		}
	}

	if err := d.exec(r.Context(),
		"UPDATE content_documents SET title = $1, slug = $2, meta_title = $3, meta_description = $4 WHERE id = $5",
		title, slug, metaTitle, metaDescription, documentID); err != nil {
		d.contentErr(w, err)
		return
	}
	d.audit(r.Context(), brandID, user.ID, "content.document.metadata_updated", "content_document", documentID)
	out, err := d.bsDocumentJSON(r.Context(), brandID, documentID)
	if err != nil {
		d.contentErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// bsDocumentJSON: parity documentJson (_content.ts) — dokumen + versi +
// komentar + konteks job.
func (d *Deps) bsDocumentJSON(ctx context.Context, brandID, documentID string) (map[string]any, error) {
	document, err := d.scopedDocument(ctx, brandID, documentID)
	if err != nil {
		return nil, err
	}
	versions, err := d.queryRows(ctx,
		"SELECT id, version, content AS html, attribution, change_note, created_by, created_at FROM content_versions WHERE document_id = $1 ORDER BY version DESC",
		documentID)
	if err != nil {
		return nil, err
	}
	comments, err := d.queryRows(ctx,
		"SELECT id, version, body, selection_quote, line_start, line_end, resolved, created_by, created_at FROM review_comments WHERE document_id = $1 ORDER BY created_at",
		documentID)
	if err != nil {
		return nil, err
	}
	job, _ := d.queryRow(ctx,
		"SELECT source_snapshot, evaluation FROM content_jobs WHERE id = $1 LIMIT 1", document.strOr("job_id", ""))

	currentVersion := document.intOr("current_version", 1)
	var current any
	versionMaps := make([]map[string]any, 0, len(versions))
	for _, v := range versions {
		item := map[string]any{
			"id": v.raw("id"), "version": v.intOr("version", 0), "html": v.raw("html"),
			"attribution": v.raw("attribution"), "change_note": v.raw("change_note"),
			"created_by": v.raw("created_by"), "created_at": v.timePtr("created_at"),
		}
		versionMaps = append(versionMaps, item)
		if v.intOr("version", 0) == currentVersion {
			current = item
		}
	}
	if current == nil {
		return nil, errConflict("Document version is missing")
	}
	commentMaps := make([]map[string]any, 0, len(comments))
	for _, c := range comments {
		commentMaps = append(commentMaps, map[string]any{
			"id": c.raw("id"), "version": c.raw("version"), "body": c.raw("body"),
			"selection_quote": c.raw("selection_quote"), "line_start": c.raw("line_start"),
			"line_end": c.raw("line_end"), "resolved": c.boolOr("resolved", false),
			"created_by": c.raw("created_by"), "created_at": c.timePtr("created_at"),
		})
	}
	var sources any = []any{}
	var evaluation any = map[string]any{}
	if job != nil {
		sources = job.jsonField("source_snapshot")
		if sources == nil {
			sources = []any{}
		}
		evaluation = job.jsonField("evaluation")
		if evaluation == nil {
			evaluation = map[string]any{}
		}
	}
	return map[string]any{
		"id": document.raw("id"), "job_id": document.raw("job_id"), "status": document.raw("status"),
		"title": document.raw("title"), "slug": document.raw("slug"),
		"meta_title": document.raw("meta_title"), "meta_description": document.raw("meta_description"),
		"assigned_reviewer_id": document.raw("assigned_reviewer_id"),
		"current_version":      document.raw("current_version"),
		"approved_by":          document.raw("approved_by"), "approved_at": document.timePtr("approved_at"),
		"updated_at": document.timePtr("updated_at"),
		"content":    current, "versions": versionMaps, "comments": commentMaps,
		"sources": sources, "evaluation": evaluation,
	}, nil
}

// ContentDocumentStandardCheck: POST /v1/brands/{brandId}/documents/{documentId}/standard-check
func (d *Deps) ContentDocumentStandardCheck(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	brandID := bfBrand.ID
	documentID := chiParam(r, "documentId")
	if _, err := d.scopedDocument(r.Context(), brandID, documentID); err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	// schema: media_ids array(string min1) min1 max50.
	rawMedia, hasMedia := body["media_ids"]
	if !hasMedia || rawMedia == nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid input: expected array, received undefined")
		return
	}
	arr, isArr := rawMedia.([]any)
	if !isArr {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid input: expected array, received "+bsZodTypeName(rawMedia))
		return
	}
	if len(arr) < 1 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Too small: expected array to have >=1 items")
		return
	}
	if len(arr) > 50 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Too big: expected array to have <=50 items")
		return
	}
	mediaIDs := make([]string, 0, len(arr))
	for _, item := range arr {
		s, isStr := item.(string)
		if !isStr || s == "" {
			httpx.Error(w, http.StatusUnprocessableEntity, "Invalid input: expected string, received "+bsZodTypeName(item))
			return
		}
		mediaIDs = append(mediaIDs, s)
	}

	results, err := d.finalStandardCheckMany(r.Context(), brandID, documentID, mediaIDs)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	anyFail := false
	anyWarning := false
	out := make([]map[string]any, 0, len(results))
	for _, res := range results {
		if res.Status == "fail" {
			anyFail = true
		}
		if res.Status == "warning" {
			anyWarning = true
		}
		out = append(out, bsStandardResultJSON(res))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"document_id": documentID, "checked": len(results), "results": out,
		"any_fail": anyFail, "any_warning": anyWarning,
	})
}

// bsStandardResultJSON: bentuk StandardCheckResult TS (checks[].detail).
func bsStandardResultJSON(res standardCheckResult) map[string]any {
	checks := make([]map[string]any, 0, len(res.Checks))
	for _, c := range res.Checks {
		item := map[string]any{
			"key": c.Key, "label": c.Label, "status": c.Status,
		}
		if c.Message != "" {
			item["detail"] = c.Message
		}
		checks = append(checks, item)
	}
	return map[string]any{
		"media_id": res.MediaID, "media_name": res.MediaName, "status": res.Status,
		"score": res.Score, "checks": checks, "failed": res.Failed, "warnings": res.Warnings,
	}
}

// ---------- publish ----------

// cmsPublishSchema: parity publishSchema (_cms.ts).
type cmsPublishInput struct {
	ConnectionID  string
	PublishStatus string
	Excerpt       any
	Categories    []any
	Tags          []any
}

// ContentDocumentPublish: POST /v1/brands/{brandId}/documents/{documentId}/publish
func (d *Deps) ContentDocumentPublish(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	bfBrand, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.cmsError(w, err)
		return
	}
	brandID := bfBrand.ID
	if err := authCheckWriteRateOrErr(r, d); err != nil {
		d.cmsError(w, err)
		return
	}
	documentID := chiParam(r, "documentId")
	document, err := d.scopedDocument(r.Context(), brandID, documentID)
	if err != nil {
		d.cmsError(w, err)
		return
	}
	if document.strOr("status", "") != "approved" {
		httpx.Error(w, http.StatusConflict, "Only approved content can be published")
		return
	}
	body := decodeJSON(r)
	value, err := bsParsePublishSchema(body)
	if err != nil {
		d.cmsError(w, err)
		return
	}
	connection, err := d.bsScopedConnection(r.Context(), brandID, value.ConnectionID)
	if err != nil {
		d.cmsError(w, err)
		return
	}
	if connection.strOr("status", "") != "active" {
		httpx.Error(w, http.StatusConflict, "CMS connection is not active")
		return
	}
	snapshot := map[string]any{
		"publish_status": value.PublishStatus, "excerpt": value.Excerpt,
		"categories": value.Categories, "tags": value.Tags,
	}
	existing, err := d.queryRow(r.Context(),
		"SELECT id, status FROM publication_jobs WHERE document_id = $1 AND connection_id = $2 LIMIT 1",
		documentID, connection.strOr("id", ""))
	if err != nil {
		d.cmsError(w, err)
		return
	}
	if existing != nil {
		if existing.strOr("status", "") == "failed" {
			if err := d.exec(r.Context(),
				"UPDATE publication_jobs SET status = 'queued', error_message = NULL, payload_snapshot = $1::jsonb WHERE id = $2",
				marshalJSONMap(snapshot), existing.strOr("id", "")); err != nil {
				d.cmsError(w, err)
				return
			}
			d.bsPublishNow(r.Context(), existing.strOr("id", ""), documentID, connection, document)
		}
		payload, err := d.bsPublicationJSON(r.Context(), existing.strOr("id", ""))
		if err != nil {
			d.cmsError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, payload)
		return
	}
	jobID := uuid.NewString()
	if err := d.exec(r.Context(),
		"INSERT INTO publication_jobs (id, brand_id, document_id, connection_id, status, publish_status, payload_snapshot, created_by) VALUES ($1, $2, $3, $4, 'queued', $5, $6::jsonb, $7)",
		jobID, brandID, documentID, connection.strOr("id", ""), value.PublishStatus,
		marshalJSONMap(snapshot), user.ID); err != nil {
		d.cmsError(w, err)
		return
	}
	d.audit(r.Context(), brandID, user.ID, "content.publication.queued", "content_document", documentID)
	d.bsPublishNow(r.Context(), jobID, documentID, connection, document)
	payload, err := d.bsPublicationJSON(r.Context(), jobID)
	if err != nil {
		d.cmsError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, payload)
}

// bsParsePublishSchema: validasi publishSchema.
func bsParsePublishSchema(body map[string]any) (cmsPublishInput, error) {
	var out cmsPublishInput
	raw, ok := body["connection_id"]
	if !ok || raw == nil {
		return out, errUnprocessable("Invalid input: expected string, received undefined")
	}
	s, isStr := raw.(string)
	if !isStr {
		return out, errUnprocessable("Invalid input: expected string, received " + bsZodTypeName(raw))
	}
	if len(s) < 1 {
		return out, errUnprocessable("Too small: expected string to have >=1 characters")
	}
	out.ConnectionID = s
	out.PublishStatus = "draft"
	if raw, ok := body["publish_status"]; ok {
		// default hanya untuk undefined; null tetap ditolak zod.
		ps, isStr := raw.(string)
		if !isStr || (ps != "draft" && ps != "publish") {
			return out, errUnprocessable("Invalid option: expected one of \"draft\"|\"publish\"")
		}
		out.PublishStatus = ps
	}
	if raw, ok := body["excerpt"]; ok {
		if raw == nil {
			out.Excerpt = nil
		} else {
			es, isStr := raw.(string)
			if !isStr {
				return out, errUnprocessable("Invalid input: expected string, received " + bsZodTypeName(raw))
			}
			if len([]rune(es)) > 500 {
				return out, errUnprocessable("Too big: expected string to have <=500 characters")
			}
			out.Excerpt = es
		}
	}
	out.Categories = []any{}
	if raw, ok := body["categories"]; ok {
		arr, isArr := raw.([]any)
		if !isArr {
			return out, errUnprocessable("Invalid input: expected array, received " + bsZodTypeName(raw))
		}
		if len(arr) > 20 {
			return out, errUnprocessable("Too big: expected array to have <=20 items")
		}
		for _, item := range arr {
			if _, isNum := item.(float64); !isNum {
				return out, errUnprocessable("Invalid input: expected number, received " + bsZodTypeName(item))
			}
		}
		out.Categories = arr
	}
	out.Tags = []any{}
	if raw, ok := body["tags"]; ok {
		arr, isArr := raw.([]any)
		if !isArr {
			return out, errUnprocessable("Invalid input: expected array, received " + bsZodTypeName(raw))
		}
		if len(arr) > 50 {
			return out, errUnprocessable("Too big: expected array to have <=50 items")
		}
		for _, item := range arr {
			if _, isNum := item.(float64); !isNum {
				return out, errUnprocessable("Invalid input: expected number, received " + bsZodTypeName(item))
			}
		}
		out.Tags = arr
	}
	return out, nil
}

// bsPublicationJSON: parity publicationJson (_cms.ts).
func (d *Deps) bsPublicationJSON(ctx context.Context, jobID string) (map[string]any, error) {
	row, err := d.queryRow(ctx,
		"SELECT id, document_id, connection_id, status, publish_status, payload_snapshot, remote_post_id, remote_url, error_message, attempts, created_at, updated_at FROM publication_jobs WHERE id = $1 LIMIT 1",
		jobID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Publication job not found")
	}
	out := bsRowJSON(row)
	payload := row.jsonField("payload_snapshot")
	if payload == nil {
		payload = map[string]any{}
	}
	out["payload_snapshot"] = payload
	return out, nil
}

// bsPublishNow: port publishNow (executor.ts). Mengirim dokumen ke CMS
// (WordPress REST atau webhook bertanda tangan) lalu menandai job completed;
// kegagalan → status failed + error_message (dipotong 500).
func (d *Deps) bsPublishNow(ctx context.Context, jobID, documentID string, connection, document bsRow) {
	version, err := d.queryRow(ctx,
		"SELECT content, content_format FROM content_versions WHERE document_id = $1 AND version = $2 LIMIT 1",
		documentID, document.intOr("current_version", 1))
	if err != nil {
		d.bsFailPublicationJob(ctx, jobID, err.Error())
		return
	}
	if version == nil {
		d.bsFailPublicationJob(ctx, jobID, "Current content version is missing")
		return
	}
	job, err := d.queryRow(ctx,
		"SELECT id, remote_post_id, payload_snapshot FROM publication_jobs WHERE id = $1 LIMIT 1", jobID)
	if err != nil || job == nil {
		return
	}
	sealed, err := d.queryRow(ctx,
		"SELECT id, sealed_secret, enc_key_ver FROM cms_connections WHERE id = $1 LIMIT 1", connection.strOr("id", ""))
	if err != nil {
		d.bsFailPublicationJob(ctx, jobID, err.Error())
		return
	}
	if sealed == nil {
		d.bsFailPublicationJob(ctx, jobID, "CMS connection is not active")
		return
	}
	secret, secretErr := d.bsConnectionSecret(sealed)
	if secretErr != nil {
		d.bsFailPublicationJob(ctx, jobID, secretErr.Error())
		return
	}
	snapshot, _ := job.jsonField("payload_snapshot").(map[string]any)
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	content := version.strOr("content", "")
	rendered := content
	if strings.ToLower(version.strOr("content_format", "")) == "markdown" {
		rendered = bsPublishMarkdown(content)
	}
	rendered = bsStripH1(rendered)

	var publishErr error
	if connection.strOr("provider", "wordpress") == "webhook" {
		publishErr = d.bsPublishToWebhook(ctx, jobID, connection, document, snapshot, secret, content, rendered)
	} else {
		publishErr = d.bsPublishToWordpress(ctx, jobID, connection, job, document, snapshot, secret, rendered)
	}
	if publishErr != nil {
		d.bsFailPublicationJob(ctx, jobID, publishErr.Error())
	}
}

// bsFailPublicationJob: parity catch publishNow → status failed.
func (d *Deps) bsFailPublicationJob(ctx context.Context, jobID, message string) {
	_ = d.exec(ctx,
		"UPDATE publication_jobs SET status = 'failed', error_message = $1 WHERE id = $2",
		truncateString(message, 500), jobID)
}

// bsPublishToWebhook: parity publishToWebhook (satu attempt; worker memegang retry).
func (d *Deps) bsPublishToWebhook(ctx context.Context, jobID string, connection, document bsRow, snapshot map[string]any, secret, content, html string) error {
	eventID := uuid.NewString()
	excerpt := any(nil)
	if v, ok := snapshot["excerpt"].(string); ok {
		excerpt = v
	} else if v, ok := document.raw("meta_description").(string); ok && v != "" {
		excerpt = v
	}
	primaryKeyword := document.raw("primary_keyword")
	keywords := []any{}
	if arr, ok := snapshot["secondary_keywords"].([]any); ok {
		keywords = arr
	}
	snapshotStrings := func(key string) []any {
		out := []any{}
		if arr, ok := snapshot[key].([]any); ok {
			for _, item := range arr {
				out = append(out, bsStringifyAny(item))
			}
		}
		return out
	}
	var author any
	if n, ok := snapshot["author_id"].(float64); ok && n == float64(int64(n)) && n > 0 {
		author = n
	}
	var featured any
	if s, ok := snapshot["featured_image_url"].(string); ok {
		featured = s
	}
	customFields := map[string]any{}
	if m, ok := snapshot["custom_fields"].(map[string]any); ok {
		customFields = m
	}
	slug := document.strOr("slug", "")
	if slug == "" {
		slug = bsSlugify(document.strOr("title", ""))
	}
	payload := map[string]any{
		"event": "content.delivered", "event_id": eventID, "timestamp": isoNow(),
		"document": map[string]any{
			"id": document.raw("id"), "title": document.raw("title"), "slug": slug,
			"language": document.raw("language"), "content_markdown": content, "content_html": html,
			"excerpt": excerpt, "primary_keyword": primaryKeyword, "secondary_keywords": keywords,
			"tags": snapshotStrings("tags"), "categories": snapshotStrings("categories"),
			"author": author, "featured_image_url": featured, "custom_fields": customFields,
			"publish_status": "draft",
			"generated_at":   bsFirstNonNil(document.raw("updated_at"), document.raw("created_at")),
		},
	}
	raw, _ := json.Marshal(payload)
	if err := d.exec(ctx, "UPDATE publication_jobs SET status = 'running', attempts = attempts + 1 WHERE id = $1", jobID); err != nil {
		return err
	}
	status, text, doErr := bsHTTPDo(ctx, http.MethodPost, connection.strOr("base_url", ""),
		bsEventHeaders(secret, "content.delivered", eventID, string(raw)), string(raw), 30*time.Second)
	if doErr != nil {
		return doErr
	}
	if status < 200 || status >= 300 {
		return errors.New("Webhook rejected payload (status " + itoa(status) + "): " + truncateString(text, 400))
	}
	var result struct {
		OK       *bool  `json:"ok"`
		RemoteID any    `json:"remote_id"`
		Message  string `json:"message"`
	}
	_ = json.Unmarshal([]byte(text), &result)
	if result.OK == nil || !*result.OK {
		msg := "Webhook did not respond with ok:true"
		if result.Message != "" {
			msg += " — " + result.Message
		}
		return errors.New(msg)
	}
	var remoteID any
	if result.RemoteID != nil {
		remoteID = bsStringifyAny(result.RemoteID)
	}
	return d.exec(ctx,
		"UPDATE publication_jobs SET remote_post_id = COALESCE($1, remote_post_id), remote_url = $2, status = 'completed', progress = 100, error_message = NULL WHERE id = $3",
		remoteID, nil, jobID)
}

// bsPublishToWordpress: parity publishToWordpress (reconciliation slug + POST).
func (d *Deps) bsPublishToWordpress(ctx context.Context, jobID string, connection, job, document bsRow, snapshot map[string]any, secret, html string) error {
	slug := document.strOr("slug", "")
	if slug == "" {
		slug = bsSlugify(document.strOr("title", ""))
	}
	status := "draft"
	if snapshot["publish_status"] == "publish" {
		status = "publish"
	}
	payload := map[string]any{
		"title": document.raw("title"), "slug": slug, "status": status, "content": html,
	}
	excerpt, hasExcerpt := snapshot["excerpt"].(string)
	metaDesc, hasMeta := document.raw("meta_description").(string)
	if (hasExcerpt && excerpt != "") || (hasMeta && metaDesc != "") {
		if hasExcerpt && excerpt != "" {
			payload["excerpt"] = excerpt
		} else {
			payload["excerpt"] = metaDesc
		}
	}
	resolved, _ := snapshot["resolved"].(map[string]any)
	numbers := func(value any) []int {
		arr, ok := value.([]any)
		if !ok {
			return []int{}
		}
		out := []int{}
		for _, item := range arr {
			if n, isNum := item.(float64); isNum && n == float64(int64(n)) && n > 0 {
				out = append(out, int(n))
			}
		}
		return out
	}
	resolvedNumbers := func(key string) []int {
		if resolved == nil {
			return []int{}
		}
		return numbers(resolved[key])
	}
	categories := bsDedupeInts(append(resolvedNumbers("categories"), numbers(snapshot["categories"])...))
	tags := bsDedupeInts(append(resolvedNumbers("tags"), numbers(snapshot["tags"])...))
	if len(categories) > 0 {
		payload["categories"] = categories
	}
	if len(tags) > 0 {
		payload["tags"] = tags
	}
	authorID := 0
	if n, ok := snapshot["author_id"].(float64); ok {
		authorID = int(n)
	}
	if resolved != nil {
		if n, ok := resolved["author_id"].(float64); ok && authorID == 0 {
			authorID = int(n)
		}
	}
	if authorID > 0 {
		payload["author"] = authorID
	}
	if resolved != nil {
		if n, ok := resolved["featured_media"].(float64); ok && n == float64(int64(n)) && n > 0 {
			payload["featured_media"] = int(n)
		}
	}
	if fields, ok := snapshot["custom_fields"].(map[string]any); ok && len(fields) > 0 {
		if acf, hasACF := fields["acf"]; hasACF {
			payload["acf"] = acf
		}
		meta := map[string]any{}
		for k, v := range fields {
			if k != "acf" {
				meta[k] = v
			}
		}
		if len(meta) > 0 {
			payload["meta"] = meta
		}
	}
	basic := base64.StdEncoding.EncodeToString([]byte(connection.strOr("username", "") + ":" + secret))
	headers := map[string]string{"Content-Type": "application/json", "Authorization": "Basic " + basic}

	remotePostID := job.strOr("remote_post_id", "")
	knownRemoteURL := ""
	if remotePostID == "" {
		lookup := bsWPURL(connection.strOr("base_url", ""),
			"wp/v2/posts?slug="+urlQueryEscape(bsStringifyAny(payload["slug"]))+"&status=any&per_page=1")
		status, text, err := bsHTTPDo(ctx, http.MethodGet, lookup, headers, "", 15*time.Second)
		if err == nil && status >= 200 && status < 300 {
			var matches []struct {
				ID   any `json:"id"`
				Link any `json:"link"`
			}
			if json.Unmarshal([]byte(text), &matches) == nil && len(matches) > 0 && matches[0].ID != nil {
				remotePostID = bsStringifyAny(matches[0].ID)
				if matches[0].Link != nil {
					knownRemoteURL = bsStringifyAny(matches[0].Link)
				}
			}
		}
	}
	target := bsWPURL(connection.strOr("base_url", ""), "wp/v2/posts")
	if remotePostID != "" {
		target = bsWPURL(connection.strOr("base_url", ""), "wp/v2/posts/"+remotePostID)
	} else {
		headers["X-AI-Writer-Idempotency-Key"] = jobID
	}
	if err := d.exec(ctx, "UPDATE publication_jobs SET status = 'running', attempts = attempts + 1, progress = 70 WHERE id = $1", jobID); err != nil {
		return err
	}
	raw, _ := json.Marshal(payload)
	httpStatus, text, err := bsHTTPDo(ctx, http.MethodPost, target, headers, string(raw), 30*time.Second)
	if err != nil {
		return err
	}
	if httpStatus < 200 || httpStatus >= 300 {
		return errors.New("WordPress returned " + itoa(httpStatus) + ": " + truncateString(text, 400))
	}
	var data struct {
		ID   any `json:"id"`
		Link any `json:"link"`
		GUID *struct {
			Rendered any `json:"rendered"`
		} `json:"guid"`
	}
	_ = json.Unmarshal([]byte(text), &data)
	finalRemoteID := remotePostID
	if data.ID != nil {
		finalRemoteID = bsStringifyAny(data.ID)
	}
	finalRemoteURL := knownRemoteURL
	if data.Link != nil {
		finalRemoteURL = bsStringifyAny(data.Link)
	} else if data.GUID != nil && data.GUID.Rendered != nil {
		finalRemoteURL = bsStringifyAny(data.GUID.Rendered)
	}
	if err := d.exec(ctx,
		"UPDATE publication_jobs SET remote_post_id = COALESCE($1, remote_post_id), remote_url = $2, status = 'completed', progress = 100, error_message = NULL WHERE id = $3",
		anyOrNil(finalRemoteID), finalRemoteURL, jobID); err != nil {
		return err
	}
	if finalRemoteURL != "" {
		if publicationID := job.strOr("publication_id", ""); publicationID != "" {
			_ = d.exec(ctx,
				"UPDATE publications SET published_url = COALESCE(published_url, $1), updated_at = NOW() WHERE id = $2",
				finalRemoteURL, publicationID)
		}
	}
	return nil
}

func bsDedupeInts(items []int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func bsFirstNonNil(values ...any) any {
	for _, v := range values {
		if v != nil {
			if t, isTime := v.(time.Time); isTime {
				return isoMilli(t)
			}
			return v
		}
	}
	return nil
}

// ---------- util ----------

// bsRequiredTopic: field topik wajib (trim 3..2000).
func bsRequiredTopic(body map[string]any, key string, min, max int) (string, error) {
	value, err := bsZodString(body, key, min, max, false, false)
	if err != nil {
		return "", err
	}
	return value, nil
}

// bsOptionalStringArray: array string optional (trim, batas panjang & jumlah);
// (nil, true) bila field absen.
func bsOptionalStringArray(body map[string]any, key string, maxLen, maxItems int) ([]string, bool) {
	raw, ok := body[key]
	if !ok || raw == nil {
		return nil, true
	}
	arr, isArr := raw.([]any)
	if !isArr {
		return nil, false
	}
	if len(arr) > maxItems {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, isStr := item.(string)
		if !isStr {
			return nil, false
		}
		if len([]rune(s)) > maxLen {
			return nil, false
		}
		out = append(out, strings.TrimSpace(s))
	}
	return out, true
}

// bsNullableString: field string nullable optional dengan batas panjang;
// absen/null → nil.
func bsNullableString(body map[string]any, key string, max int) (any, error) {
	raw, ok := body[key]
	if !ok || raw == nil {
		return nil, nil
	}
	s, isStr := raw.(string)
	if !isStr {
		return nil, errUnprocessable("Invalid input: expected string, received " + bsZodTypeName(raw))
	}
	if len([]rune(s)) > max {
		return nil, errUnprocessable("Too big: expected string to have <=" + itoa(max) + " characters")
	}
	return s, nil
}

// contentErr: port contentError (_content.ts) — NotFound 404, Conflict 409,
// sisanya lewat pemetaan error umum.
func (d *Deps) contentErr(w http.ResponseWriter, err error) {
	d.writeErr(w, err)
}

// authCheckWriteRateOrErr: port checkWriteRate (bucket write:publish:{userId}).
func authCheckWriteRateOrErr(r *http.Request, d *Deps) error {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		return errStatus(http.StatusUnauthorized, "Invalid or expired access token")
	}
	return authpkg.CheckWriteRate(r.Context(), d.Pool, "write:publish:"+user.ID, d.Cfg.WriteRateLimitPerMinute)
}

// urlQueryEscape: shim url.QueryEscape.
func urlQueryEscape(s string) string { return url.QueryEscape(s) }
