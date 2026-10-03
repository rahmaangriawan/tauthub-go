package clienttaut

import "testing"

// Regresi: judul artikel hidup di field judul terpisah — blok <h1> dari model
// wajib diturunkan jadi <h2>, bukan dibiarkan apa adanya di dalam isi.
func TestTransformSegmentsDemotesH1(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "h1 di awal isi",
			in:   `<h1>Judul Dokumen</h1><p>Paragraf pertama isi.</p>`,
			want: `<h2>Judul Dokumen</h2><p>Paragraf pertama isi.</p>`,
		},
		{
			name: "h1 dengan atribut (atribut ikut dibuang)",
			in:   `<h1 class="judul">Judul Dokumen</h1><p>Isi.</p>`,
			want: `<h2>Judul Dokumen</h2><p>Isi.</p>`,
		},
		{
			name: "tanpa h1 tidak berubah",
			in:   `<h2>Subjudul</h2><p>Isi.</p>`,
			want: `<h2>Subjudul</h2><p>Isi.</p>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeHeadings(tc.in); got != tc.want {
				t.Fatalf("H1 tidak diturunkan dengan benar:\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

// demoteH1ToH2: penjaga deterministik untuk hasil tulisan ulang gatekeeper —
// idempoten dan hanya menyentuh H1.
func TestDemoteH1ToH2(t *testing.T) {
	if got := demoteH1ToH2(`<h1>A</h1><h2>B</h2>`); got != `<h2>A</h2><h2>B</h2>` {
		t.Fatalf("H1 gagal diturunkan: %s", got)
	}
	if got := demoteH1ToH2(`<h2>B</h2>`); got != `<h2>B</h2>` {
		t.Fatalf("html tanpa H1 harus utuh: %s", got)
	}
	if got := demoteH1ToH2(""); got != "" {
		t.Fatalf("html kosong harus utuh: %s", got)
	}
}
