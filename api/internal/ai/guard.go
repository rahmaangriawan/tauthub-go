// guard.go: cek keamanan konten (content safety) + kesiapan AI Overview dalam
// SATU panggilan LLM. Keduanya membaca teks artikel yang sama dengan model yang
// sama, jadi menggabungkannya menghemat satu round-trip (15-40 dtk per artikel
// pada model flash). Bila respons gabungan tidak valid, pemanggil fallback ke
// dua panggilan terpisah (perilaku lama) — kualitas cek tidak dikorbankan.
package ai

import (
	"fmt"
	"strings"
)

// GuardSystemPrompt: aturan keamanan konten (dari domain, supaya daftar
// kategori/kebijakan tetap satu sumber) + rubrik AI Overview + SATU skema
// keluaran JSON gabungan.
func GuardSystemPrompt(safetyRules string) string {
	return strings.TrimSpace(safetyRules) + "\n\n" + aiOverviewRules + `

TUGAS GABUNGAN: kerjakan DUA penilaian dalam satu jawaban — (A) klasifikasi keamanan konten dan (B) penilaian kesiapan AI Overview di atas.
Balas HANYA JSON valid, tanpa markdown/penjelasan lain:
{"safety":{"allowed":true|false,"riskLevel":"low"|"medium"|"high","categories":["..."],"reasons":["... maks 3, bahasa Indonesia, aman dibaca pengguna"],"confidence":0.0},"overview":{"indikator":[` + AIOverviewIndicatorSchemaFragment() + `],"catatan_revisi":[],"entitas_tanpa_dasar":[]}}
Aturan: isi KEDUA bagian. safety.allowed=true HANYA untuk risiko rendah; medium = meragukan (allowed=false, review manual); high = blokir.`
}

// GuardUserPrompt: satu prompt pengguna untuk kedua tugas — judul/tipe media/
// jumlah kata, aturan kelulusan, bahan rujukan, tautan, teks polos (safety), dan
// HTML artikel (struktur untuk rubrik).
func GuardUserPrompt(title, mediaType, contentHTML, text, brief string, links []string, wordCount int, st AIOverviewSettings) string {
	typeName := "mainstream (berita, tanpa heading)"
	if mediaType == "blogger" {
		typeName = "blogger (artikel evergreen, memakai heading)"
	}
	maxHTML := st.MaxChars
	if maxHTML <= 0 {
		maxHTML = aiOverviewDefaultMaxChar
	}
	html := contentHTML
	if len(html) > maxHTML {
		html = html[:maxHTML]
	}
	briefText := strings.TrimSpace(brief)
	if briefText == "" {
		briefText = "(KOSONG — tidak ada brief/bahan rujukan untuk generate artikel ini)"
	} else if len([]rune(briefText)) > aiOverviewDefaultBriefChars {
		briefText = string([]rune(briefText)[:aiOverviewDefaultBriefChars])
	}
	body := text
	if len(body) > 6000 {
		body = body[:6000]
	}
	linkBlock := "(tidak ada tautan)"
	if len(links) > 0 {
		limit := len(links)
		if limit > 20 {
			limit = 20
		}
		linkBlock = strings.Join(links[:limit], "\n")
	}
	return strings.Join([]string{
		"Judul: " + title,
		"Tipe media: " + typeName,
		"Jumlah kata: " + fmt.Sprintf("%d", wordCount),
		fmt.Sprintf("Aturan kelulusan AI Overview: %s. Bila tidak memenuhi, sertakan catatan_revisi (maks 3) yang sangat spesifik.", AIOverviewPassRule(st)),
		"BAHAN/BRIEF (rujukan fakta yang boleh dipakai artikel):\n" + briefText,
		"Konten (teks polos):\n" + body,
		"Links:\n" + linkBlock,
		"Isi (HTML):\n" + html,
	}, "\n\n")
}

// ParseGuardResponse: ambil sub-objek "overview" dan "safety" dari respons
// gabungan. ok=false bila salah satu bagian hilang/tidak berbentuk objek —
// pemanggil harus fallback ke dua panggilan terpisah.
func ParseGuardResponse(llmContent string) (overviewFields map[string]any, safetyFields map[string]any, ok bool) {
	parsed := firstJSONObject(llmContent)
	if parsed == nil {
		return nil, nil, false
	}
	overviewFields, _ = parsed["overview"].(map[string]any)
	safetyFields, _ = parsed["safety"].(map[string]any)
	if overviewFields == nil || safetyFields == nil {
		return nil, nil, false
	}
	return overviewFields, safetyFields, true
}
