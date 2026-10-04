package brandflow

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// Benchmark jalur rewrite HTML publikasi. Fungsi-fungsi ini berjalan sekali per
// job dengan dokumen besar (ratusan KB) dan tag <img> sampai peMaxInlineUploads,
// jadi dipakai sebagai tolok ukur optimasi sekaligus sumber profil PGO.

var (
	benchSinkStr  string
	benchSinkBool bool
	benchSinkInt  int
)

func benchArticleHTML(images int) string {
	var b strings.Builder
	b.WriteString("<p>Paragraf pembuka artikel tentang epoxy lantai industri.</p>\n")
	for i := 0; i < images; i++ {
		fmt.Fprintf(&b, "<p>Bagian %d sebelum gambar.</p>", i)
		fmt.Fprintf(&b, `<img src="/uploads/gambar-%d.webp" alt="Gambar %d" class="size-full">`, i, i)
		b.WriteString("<p>Penjelasan lanjutan setelah gambar.</p>\n")
	}
	return b.String()
}

func BenchmarkPERewriteImages(b *testing.B) {
	html := benchArticleHTML(20)
	identity := func(tag string) (string, bool, string) { return tag, false, "" }
	b.ReportAllocs()
	b.SetBytes(int64(len(html)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, failures := peRewriteImages(html, identity)
		if len(failures) != 0 {
			b.Fatalf("kegagalan tak terduga: %v", failures)
		}
		benchSinkStr = out
	}
}

func BenchmarkPartialContentHTML(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`{"title":"Epoxy","content_html":"`)
	for i := 0; i < 2000; i++ {
		sb.WriteString("<p>Baris konten artikel yang cukup panjang untuk mewakili beban nyata.</p>")
	}
	sb.WriteString(`","meta_description":"x"}`)
	acc := sb.String()
	b.ReportAllocs()
	b.SetBytes(int64(len(acc)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchSinkStr, benchSinkBool = partialContentHTML(acc)
	}
}

func BenchmarkBSGroupID(b *testing.B) {
	vals := []int64{0, 5, 25, 2500, 1234567, 9876543210, -42000000}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range vals {
			benchSinkStr = bsGroupID(v)
		}
	}
}

// TestBSGroupIDParity mengunci keluaran bsGroupID sebelum dan sesudah optimasi.
func TestBSGroupIDParity(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{5, "5"},
		{25, "25"},
		{250, "250"},
		{2500, "2.500"},
		{1000000, "1.000.000"},
		{1234567, "1.234.567"},
		{9876543210, "9.876.543.210"},
		{-42000000, "-42.000.000"},
		{9223372036854775807, "9.223.372.036.854.775.807"},
	}
	for _, c := range cases {
		if got := bsGroupID(c.in); got != c.want {
			t.Errorf("bsGroupID(%d) = %q, mau %q", c.in, got, c.want)
		}
	}
}

// TestPEInlineImageNameSuffix mengunci format "%08x" (huruf kecil, padding nol,
// minimal 8 digit) yang dipakai sebagai sufiks nama berkas gambar badan artikel.
func TestPEInlineImageNameSuffix(t *testing.T) {
	srcs := []string{"", "a", "/uploads/abc.webp", strings.Repeat("panjang-", 40)}
	for _, src := range srcs {
		want := fmt.Sprintf("%08x", fnv32(src))
		got := peInlineImageName("Judul Artikel Uji", src)
		if !strings.HasSuffix(got, want) {
			t.Errorf("sufiks peInlineImageName(%q) = %q, mau berakhiran %q", src, got, want)
		}
	}
}

// TestRuneCountMatchesRuneSliceLen: utf8.RuneCountInString harus sama persis
// dengan len([]rune(s)) — termasuk untuk byte UTF-8 tak valid (masing-masing
// dihitung 1). Ini dasar keamanan penggantian len([]rune(s)).
func TestRuneCountMatchesRuneSliceLen(t *testing.T) {
	cases := []string{
		"",
		"ascii biasa",
		"é à ü ñ",
		"emoji 🚀 dan aksen",
		"campur \xff\xfe byte rusak",
		"\xc3",         // urutan UTF-8 terpotong
		"\xed\xa0\x80", // surrogate tak valid
		strings.Repeat("é🚀x", 1000),
	}
	for _, s := range cases {
		want := len([]rune(s))
		got := utf8.RuneCountInString(s)
		if got != want {
			t.Errorf("RuneCount(%q) = %d, len([]rune) = %d", s, got, want)
		}
	}
}

// BenchmarkWriterStream: pola streaming writer — akumulasi delta lalu ekstrak
// content_html. Versi lama memindai ulang seluruh akumulasi per delta (O(n²)),
// versi baru membawa kursor (O(n)).
func BenchmarkWriterStream(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`{"title":"Epoxy","content_html":"`)
	body := "<p>Baris konten artikel yang cukup panjang untuk mewakili beban nyata.</p>"
	for i := 0; i < 2000; i++ {
		sb.WriteString(body)
	}
	sb.WriteString(`","meta_description":"x"}`)
	acc := sb.String()
	var deltas []string
	size := (len(acc) + 99) / 100
	for i := 0; i < len(acc); i += size {
		end := i + size
		if end > len(acc) {
			end = len(acc)
		}
		deltas = append(deltas, acc[i:end])
	}

	b.Run("pindai-ulang (lama)", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var acc2 strings.Builder
			sent := 0
			for _, d := range deltas {
				acc2.WriteString(d)
				visible, _ := partialContentHTML(acc2.String())
				if len(visible) > sent {
					sent = len(visible)
				}
			}
			benchSinkInt = sent
		}
	})
	b.Run("inkremental (baru)", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := newPEStreamHTML()
			total := 0
			for _, d := range deltas {
				if tail, _ := stream.Write(d); tail != "" {
					total += len(tail)
				}
			}
			benchSinkInt = total
		}
	})
}
