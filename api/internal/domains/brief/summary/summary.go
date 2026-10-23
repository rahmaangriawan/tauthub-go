// Package summary menghasilkan ringkasan entitas & file untuk satu brief
// (profiler_profiles) dan menyimpannya di profiler_brief_summaries.
//
// Writer mode "Yang Proper" hanya membaca ringkasan ini sebagai memori brand,
// bukan lagi seluruh isi profil, sehingga prompt jauh lebih pendek. Dua agent
// sengaja dipisah karena bahan dan tujuannya berbeda:
//
//   - brief_entity_summary: identitas & fakta kanonik dari profil + blok.
//   - brief_file_summary: isi dokumen/tautan/gambar yang diunggah ke brief.
//
// Ringkasan diperbarui lewat job `brief_summary`; perubahan data brief hanya
// menandai status `queued` lalu menjadwalkan job (debounce 20 detik).
package summary

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

type Kind string

const (
	KindEntity Kind = "entity"
	KindFile   Kind = "file"
)

const (
	AgentEntity = "brief_entity_summary"
	AgentFile   = "brief_file_summary"
	// AgentRacik menyusun instruksi topik dari anchor di Pengelola Naskah.
	// Bukan ringkasan brief, tetapi memakai transport + prompt berversi yang
	// sama supaya bisa diatur admin tanpa deploy.
	AgentRacik = "racik_instruksi"
	// AgentMatchmaker mengubah brief kampanye bebas menjadi kriteria media
	// terstruktur untuk katalog. Sama seperti Racik: transport + prompt
	// berversi yang sama, bukan ringkasan brief.
	AgentMatchmaker = "media_matchmaker"
)

const JobKind = "brief_summary"

// Batas bahan yang dikirim ke model. Cukup untuk konteks brief normal, tetapi
// tetap membatasi biaya bila klien mengunggah banyak dokumen.
const (
	entitySourceCap = 12000
	fileSourceCap   = 24000
	docTextCap      = 4000
	urlTextCap      = 3000
	blockTextCap    = 1500
	maxDocs         = 5
	maxURLs         = 5
)

// AgentName: nama agent penyimpan config/prompt untuk sebuah kind.
func AgentName(kind Kind) string {
	if kind == KindFile {
		return AgentFile
	}
	return AgentEntity
}

func ParseKind(raw string) (Kind, bool) {
	switch strings.TrimSpace(raw) {
	case string(KindEntity):
		return KindEntity, true
	case string(KindFile):
		return KindFile, true
	}
	return "", false
}

func ValidAgent(agent string) bool {
	return agent == AgentEntity || agent == AgentFile || agent == AgentRacik || agent == AgentMatchmaker
}

// KindOfAgent: kebalikan AgentName.
func KindOfAgent(agent string) (Kind, bool) {
	switch agent {
	case AgentEntity:
		return KindEntity, true
	case AgentFile:
		return KindFile, true
	}
	return "", false
}

// Summary: satu baris profiler_brief_summaries.
type Summary struct {
	ProfileID   string     `json:"profile_id"`
	Kind        Kind       `json:"kind"`
	Status      string     `json:"status"`
	Content     string     `json:"content"`
	SourceChars int        `json:"source_chars"`
	SourceHash  string     `json:"source_hash"`
	Model       string     `json:"model"`
	TokenInput  int        `json:"token_input"`
	TokenOutput int        `json:"token_output"`
	Error       string     `json:"error"`
	GeneratedAt *time.Time `json:"generated_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Source: bahan mentah yang dikirim ke model + sidik jari untuk mendeteksi
// perubahan sejak ringkasan terakhir dibuat.
type Source struct {
	Text  string
	Chars int
	Hash  string
}

func newSource(text string) Source {
	text = strings.TrimSpace(text)
	sum := sha256.Sum256([]byte(text))
	return Source{Text: text, Chars: len([]rune(text)), Hash: hex.EncodeToString(sum[:])}
}

// clip: potong ke max rune tanpa memotong karakter multibyte.
func clip(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
