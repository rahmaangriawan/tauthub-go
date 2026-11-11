package clienttaut

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/domains/brief/summary"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// PlanBriefAssist: POST /v1/brands/{brandId}/taut/plans/brief-assist
// Klien menulis brief singkat; AI merapikannya menjadi brief yang cukup untuk
// menyusun rencana. Memakai koneksi AI Racik Agent (sudah dikonfigurasi admin)
// dengan prompt tetap di sini. Tidak menyimpan apa pun; klien meninjau hasilnya.

const planBriefAssistPrompt = `Anda membantu klien menyusun rencana artikel media (Bahasa Indonesia).
Dari catatan singkat klien, keluarkan JSON saja dengan kunci:
- "brief": string, satu paragraf maksimal 120 kata: apa yang dijual dan manfaat utamanya, untuk siapa, pesan utama, dan sudut topik untuk beberapa artikel.
- "category": string kategori media yang paling cocok (mis. "Ekonomi", "Gaya Hidup", "Teknologi", "Kesehatan", "Otomotif", "Properti", "Pendidikan", "Umum") atau "" bila tidak jelas.
- "weeks": angka bulat 1 sampai 8, jumlah minggu. Pakai angka yang disebut klien; bila tidak disebut, sarankan 4.
- "per_week": angka bulat 1 sampai 3, artikel per minggu. Pakai angka yang disebut; bila tidak disebut, sarankan 1.
- "budget_idr": angka rupiah total anggaran HANYA bila klien menyebut nominal uang secara eksplisit; bila tidak, null.
Aturan keras: jangan menambah fakta, angka, harga, klaim, nama perusahaan, atau testimoni yang tidak ada di catatan. Jangan mengarang anggaran.`

const planBriefAssistMaxInput = 1200

func (d *Deps) PlanBriefAssist(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	if _, err := d.brandID(r, user.ID, "writer"); err != nil {
		d.writeErr(w, err)
		return
	}
	var body struct {
		Brief string `json:"brief"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	note := strings.TrimSpace(body.Brief)
	if note == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Tulis dulu sedikit catatan brief, lalu AI akan merapikannya.")
		return
	}
	if len([]rune(note)) > planBriefAssistMaxInput {
		httpx.Error(w, http.StatusUnprocessableEntity, "Catatan terlalu panjang. Persingkat sampai 1200 karakter.")
		return
	}

	secret := ""
	if d.Cfg != nil {
		secret = d.Cfg.CmsEncryptionSecret
	}
	agentCfg := summary.LoadAgentConfig(r.Context(), d.Pool, secret, summary.AgentRacik)
	if !agentCfg.Configured() {
		httpx.Error(w, http.StatusUnprocessableEntity, "Asisten AI belum dikonfigurasi. Hubungi admin untuk mengisi koneksi AI.")
		return
	}
	client := &ai.ChatClient{
		BaseURL:     agentCfg.BaseURL,
		APIKey:      agentCfg.APIKey,
		Model:       agentCfg.Model,
		Temperature: 0.4,
		MaxTokens:   intPtr(600),
		Timeout:     time.Duration(agentCfg.TimeoutMS) * time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	result, err := client.CompleteMessages(ctx, []ai.ChatMessage{
		{Role: "system", Content: planBriefAssistPrompt},
		{Role: "user", Content: "Catatan klien:\n" + note},
	}, true)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "Asisten AI sedang tidak bisa dihubungi. Coba lagi, atau tulis brief manual.")
		return
	}
	var parsed struct {
		Brief     string `json:"brief"`
		Category  string `json:"category"`
		Weeks     int    `json:"weeks"`
		PerWeek   int    `json:"per_week"`
		BudgetIDR *int   `json:"budget_idr"`
	}
	raw := strings.TrimSpace(result.Content)
	if i := strings.Index(raw, "{"); i >= 0 {
		raw = raw[i:]
	}
	if j := strings.LastIndex(raw, "}"); j >= 0 {
		raw = raw[:j+1]
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil || strings.TrimSpace(parsed.Brief) == "" {
		httpx.Error(w, http.StatusBadGateway, "Asisten AI belum memberi hasil yang bisa dipakai. Coba lagi.")
		return
	}
	weeks := parsed.Weeks
	if weeks < 1 || weeks > planMaxWeeks {
		weeks = 4
	}
	perWeek := parsed.PerWeek
	if perWeek < 1 || perWeek > planMaxPerWeek {
		perWeek = 1
	}
	category := strings.TrimSpace(parsed.Category)
	if utf8.RuneCountInString(category) > 120 {
		category = ""
	}

	budget, source := 0, "note"
	if parsed.BudgetIDR != nil && *parsed.BudgetIDR >= 50000 {
		budget = roundUpIDR10k(*parsed.BudgetIDR)
	} else {
		// Tidak ada nominal di catatan: estimasi dari rata-rata biaya media yang
		// bisa dipesan di kategori itu, dikali jumlah artikel. Hanya saran; klien
		// bisa mengubahnya.
		rate := d.getTcConfig(r.Context()).RateIDR
		if rate < 1 {
			rate = 1000
		}
		cands, cErr := d.planCandidates(r.Context(), category)
		if cErr == nil && len(cands) > 0 {
			sumTC := 0
			for _, c := range cands {
				sumTC += c.TC
			}
			avgTC := float64(sumTC) / float64(len(cands))
			budget = roundUpIDR10k(int(avgTC * float64(rate) * float64(weeks*perWeek)))
			source = "estimate"
		}
	}
	brief := strings.Trim(strings.TrimSpace(parsed.Brief), "\"")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"brief": brief, "category": category, "weeks": weeks, "per_week": perWeek,
		"budget_idr": budget, "budget_source": source,
	})
}

func roundUpIDR10k(v int) int {
	if v <= 0 {
		return 0
	}
	return ((v + 9999) / 10000) * 10000
}
