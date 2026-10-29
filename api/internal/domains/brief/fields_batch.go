package brief

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Batas panjang per field. Cukup longgar untuk deskripsi dan daftar fakta,
// tetapi menolak kiriman yang jelas bukan isian form (mis. badan JSON besar
// yang ditempel ke satu field).
const (
	maxNameLen   = 200
	maxShortLen  = 1000
	maxLongLen   = 8000
	maxBatchSize = 40
)

type batchField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// BriefFieldsBatchPost menyimpan beberapa field Profil Brand dalam satu
// permintaan. Berbeda dari BriefFieldsPost (satu field per panggilan, dipakai
// alur chat), endpoint ini memvalidasi seluruh isian dulu lalu menulisnya
// dalam satu transaksi: tidak ada simpan separuh kalau satu field ditolak.
// Jalur ini deterministik dan tidak memanggil LLM.
func (d *Deps) BriefFieldsBatchPost(w http.ResponseWriter, r *http.Request) {
	userID, brandID, profileID, ok := d.accessOrFail(w, r, "writer")
	if !ok {
		return
	}

	var body struct {
		Fields []batchField `json:"fields"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Format isian tidak valid.")
		return
	}
	if len(body.Fields) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Tidak ada isian yang dikirim.")
		return
	}
	if len(body.Fields) > maxBatchSize {
		httpx.Error(w, http.StatusUnprocessableEntity, "Terlalu banyak field dalam satu permintaan.")
		return
	}

	fields, fieldVersion, err := d.loadActiveFieldSet(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	states, err := d.loadStates(r.Context(), profileID)
	if err != nil {
		writeErr(w, err)
		return
	}

	updates := make([]FieldUpdate, 0, len(body.Fields))
	for _, f := range body.Fields {
		updates = append(updates, FieldUpdate{
			Key:    strings.TrimSpace(f.Key),
			Value:  strings.TrimSpace(f.Value),
			Status: StatusConfirmed,
			Source: SourceUser,
		})
	}
	if msg := validateBatchValues(updates); msg != "" {
		httpx.Error(w, http.StatusUnprocessableEntity, msg)
		return
	}

	accepted, rejected := ValidateUpdates(fields, states, updates)
	if len(rejected) > 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Isian belum bisa disimpan: "+strings.Join(rejected, "; "))
		return
	}
	if len(accepted) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Tidak ada isian yang bisa disimpan.")
		return
	}

	if err := d.applyUpdatesAtomic(r.Context(), profileID, userID, fieldVersion, fields, accepted); err != nil {
		writeErr(w, err)
		return
	}
	d.auditBriefWrite(r.Context(), brandID, userID, "brief.fields_batch_saved", profileID, map[string]any{
		"saved": len(accepted), "keys": fieldUpdateKeys(accepted),
	})

	states, _ = d.loadStates(r.Context(), profileID)
	filled, total := Progress(fields, states)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"saved":    len(accepted),
		"progress": map[string]any{"filled": filled, "total": total},
	})
}

// validateBatchValues memeriksa bentuk isian yang tidak tercakup ValidateUpdates
// (yang hanya tahu tipe widget). Pesan error ditulis untuk dibaca pengguna.
func validateBatchValues(updates []FieldUpdate) string {
	byKey := make(map[string]string, len(updates))
	for _, u := range updates {
		byKey[u.Key] = u.Value
	}

	if strings.TrimSpace(byKey["canonical_name"]) == "" {
		return "Nama resmi brand wajib diisi."
	}
	if strings.TrimSpace(byKey["entity_category"]) == "" {
		return "Kategori brand wajib dipilih."
	}

	for _, u := range updates {
		limit := maxLongLen
		switch u.Key {
		case "canonical_name":
			limit = maxNameLen
		case "entity_category", "preferred_terms", "forbidden_words":
			limit = maxShortLen
		}
		if utf8.RuneCountInString(u.Value) > limit {
			return "Isian " + u.Key + " terlalu panjang."
		}
	}

	if links := byKey["canonical_links"]; links != "" {
		for _, raw := range splitURLList(links) {
			if !validHTTPURL(raw) {
				return "Kanal resmi harus berupa URL yang diawali http:// atau https://: " + raw
			}
		}
	}
	return ""
}

// splitURLList: daftar URL dipisah baris baru atau titik koma, sesuai petunjuk
// di form. Koma tidak dipakai sebagai pemisah karena bisa muncul di query.
func splitURLList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func validHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return u.Host != ""
}
