package brief

// actions_meta.go: bagian murni dari alur aksi di chatbot, dipisahkan dari
// handler HTTP supaya bisa diuji tanpa database.
//
// Dua hal yang diuji di sini adalah tempat paling rawan salah:
//   - menandai aksi terpakai harus idempoten dan tidak merusak meta lain;
//   - pesan yang dikirim ke frontend harus membawa aksi, tapi tidak membocorkan
//     isi meta internal lain.

import (
	"encoding/json"
	"time"
)

// markActionUsed menandai kind aksi sebagai sudah dipakai di dalam meta pesan.
//
// Mengembalikan meta baru dan flag already=true kalau kind itu memang sudah ada.
// Pemanggil memakai flag itu untuk membalas tanpa menulis ulang baris.
//
// Kenapa tidak memakai map yang sama: meta mentah hasil json.Unmarshal bisa
// berisi []any, sedangkan nilai baru kita tulis sebagai []any juga. Fungsi ini
// menormalkan keduanya supaya tipe tidak bercampur setelah beberapa klik.
func markActionUsed(meta map[string]any, kind string) (map[string]any, bool) {
	if meta == nil {
		meta = map[string]any{}
	}
	used := actionUsedKinds(meta)
	for _, k := range used {
		if k == kind {
			return meta, true
		}
	}
	used = append(used, kind)
	meta["actions_used"] = used
	return meta, false
}

// actionUsedKinds membaca daftar kind yang sudah dipakai dari meta.
//
// Nilai di meta bisa datang dari tiga bentuk: []any (hasil unmarshal JSON),
// []string (hasil tulis di proses yang sama), atau tidak ada sama sekali.
// Semuanya dinormalkan ke []string.
func actionUsedKinds(meta map[string]any) []string {
	raw, ok := meta["actions_used"]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		out := make([]string, len(v))
		copy(out, v)
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, isStr := item.(string); isStr {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// actionUsedIn melaporkan apakah satu kind sudah dipakai. Dipakai frontend
// lewat respons; di backend dipakai untuk memastikan idempotensi.
func actionUsedIn(meta map[string]any, kind string) bool {
	for _, k := range actionUsedKinds(meta) {
		if k == kind {
			return true
		}
	}
	return false
}

// assistantMessageRow membentuk satu baris pesan untuk respons GET chat.
//
// Aksi hanya dikirim untuk pesan assistant yang punya meta, karena pesan user
// tidak pernah punya aksi dan mengirimnya hanya menambah payload.
func assistantMessageRow(m message) map[string]any {
	row := map[string]any{
		"id": m.ID, "role": m.Role, "content": m.Content,
		"created_at": messageCreatedAt(m.CreatedAt),
	}
	if m.Role != "assistant" || len(m.Meta) == 0 {
		return row
	}
	if actions, ok := m.Meta["actions"]; ok {
		row["actions"] = actions
	}
	// Selalu kirim array (bukan null) supaya frontend tidak perlu menebak:
	// pesan tanpa aksi terpakai tetap punya bentuk yang sama.
	row["actions_used"] = actionUsedKinds(m.Meta)
	return row
}

// encodeMeta menyiapkan meta untuk kolom jsonb. Nil kosong dikembalikan sebagai
// nil supaya kolom tetap NULL, bukan objek kosong.
func encodeMeta(meta map[string]any) (any, error) {
	if len(meta) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

// messageCreatedAt memusatkan format waktu pesan supaya GET dan POST konsisten.
func messageCreatedAt(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}
