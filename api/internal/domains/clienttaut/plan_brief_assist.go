package clienttaut

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/domains/brief/summary"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// PlanBriefAssist: POST /v1/brands/{brandId}/taut/plans/brief-assist
// Klien menulis brief singkat; AI merapikannya menjadi brief yang cukup untuk
// menyusun rencana. Memakai koneksi AI Racik Agent (sudah dikonfigurasi admin)
// dengan prompt tetap di sini. Tidak menyimpan apa pun; klien meninjau hasilnya.

const planBriefAssistPrompt = `Anda membantu klien menulis brief untuk rencana artikel media (Bahasa Indonesia).
Dari catatan singkat klien, tulis brief yang lengkap dan jelas dalam satu paragraf pendek, maksimal 120 kata:
- apa yang dijual atau dipromosikan, dan manfaat utamanya,
- untuk siapa (target pembaca),
- pesan utama yang ingin disampaikan,
- sudut pandang atau topik yang bisa diangkat di beberapa artikel.
Aturan keras: jangan menambah fakta, angka, harga, klaim, nama perusahaan, atau testimoni yang tidak ada di catatan klien. Bila ada bagian yang tidak diketahui, jangan mengarang; cukup tidak disebut.
Keluarkan hanya teks brief, tanpa judul, tanpa tanda kutip, tanpa penjelasan.`

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
	}, false)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "Asisten AI sedang tidak bisa dihubungi. Coba lagi, atau tulis brief manual.")
		return
	}
	out := strings.Trim(strings.TrimSpace(result.Content), "\"")
	if out == "" {
		httpx.Error(w, http.StatusBadGateway, "Asisten AI belum memberi hasil. Coba lagi.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"brief": out})
}
