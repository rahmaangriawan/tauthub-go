package ai

import (
	"strings"
	"testing"
)

func TestJSONStringFieldStreamSingleChunk(t *testing.T) {
	s := NewJSONStringFieldStream("reply")
	got := s.Write(`{"reply":"Halo, aku bantu susun brief.","field_updates":[]}`)
	if got != "Halo, aku bantu susun brief." {
		t.Fatalf("isi reply salah: %q", got)
	}
	if !s.Done() {
		t.Fatalf("Done() harus true setelah string penutup terlihat")
	}
	if extra := s.Write("sisa"); extra != "" {
		t.Fatalf("Write setelah selesai harus kosong, dapat %q", extra)
	}
}

// Potongan bisa jatuh di tengah kata, di tengah escape, bahkan di tengah
// digit hex unicode — rangkaian delta harus sama dengan isi penuhnya.
func TestJSONStringFieldStreamChunkBoundaries(t *testing.T) {
	full := `{"reply":"Kutangkap: brand \"Tauthub\" bergerak di PR.\nBaris dua.\tTab.","asked_field":null}`
	want := "Kutangkap: brand \"Tauthub\" bergerak di PR.\nBaris dua.\tTab."

	for _, size := range []int{1, 2, 3, 7, 13, 64} {
		s := NewJSONStringFieldStream("reply")
		var out strings.Builder
		for i := 0; i < len(full); i += size {
			end := i + size
			if end > len(full) {
				end = len(full)
			}
			out.WriteString(s.Write(full[i:end]))
		}
		if out.String() != want {
			t.Fatalf("ukuran potongan %d: dapat %q, mau %q", size, out.String(), want)
		}
		if !s.Done() {
			t.Fatalf("ukuran potongan %d: Done() harus true", size)
		}
	}
}

func TestJSONStringFieldStreamUnicodeEscapes(t *testing.T) {
	// \u00e9 = é, \uD83D\uDE00 = 😀 (pasangan surrogate harus digabung).
	full := `{"reply":"caf\u00e9 \uD83D\uDE00 siap","field_updates":[]}`
	s := NewJSONStringFieldStream("reply")
	var out strings.Builder
	for i := 0; i < len(full); i += 4 {
		end := i + 4
		if end > len(full) {
			end = len(full)
		}
		out.WriteString(s.Write(full[i:end]))
	}
	if out.String() != "café 😀 siap" {
		t.Fatalf("decode unicode salah: %q", out.String())
	}
}

func TestJSONStringFieldStreamPartialInput(t *testing.T) {
	s := NewJSONStringFieldStream("reply")
	if got := s.Write(`{"reply":"belum seles`); got != "belum seles" {
		t.Fatalf("bagian parsial harus terpancar, dapat %q", got)
	}
	if s.Done() {
		t.Fatalf("Done() harus false selama string penutup belum ada")
	}
	if got := s.Write(`ai juga"`); got != "ai juga" {
		t.Fatalf("lanjutan harus terpancar, dapat %q", got)
	}
	if !s.Done() {
		t.Fatalf("Done() harus true setelah penutup tiba")
	}
}

// Pemotongan di tengah escape tidak boleh memancarkan karakter setengah jadi.
func TestJSONStringFieldStreamEscapeSplitAcrossChunks(t *testing.T) {
	s := NewJSONStringFieldStream("reply")
	if got := s.Write(`{"reply":"baris satu\`); got != "baris satu" {
		t.Fatalf("backslash menggantung tidak boleh terpancar: %q", got)
	}
	if got := s.Write(`nbaris dua"`); got != "\nbaris dua" {
		t.Fatalf("escape lanjutan salah: %q", got)
	}
}

func TestJSONStringFieldStreamWaitsForField(t *testing.T) {
	s := NewJSONStringFieldStream("reply")
	if got := s.Write(`{"field_updates":[]`); got != "" {
		t.Fatalf("belum ada key reply, harus kosong, dapat %q", got)
	}
	if got := s.Write(`,"reply":"baru muncul"}`); got != "baru muncul" {
		t.Fatalf("harus mulai memancar saat key muncul: %q", got)
	}
}

func TestJSONStringFieldStreamNonStringValue(t *testing.T) {
	s := NewJSONStringFieldStream("reply")
	if got := s.Write(`{"reply":null,"field_updates":[]}`); got != "" {
		t.Fatalf("nilai bukan string harus diabaikan, dapat %q", got)
	}
	if s.Done() {
		t.Fatalf("nilai non-string tidak boleh dianggap selesai")
	}
}
