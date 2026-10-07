package content

import "testing"

func TestNormalizeArticleHTML(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "div per baris ala contenteditable",
			in:   `Bekerja di depan komputer mudah lelah.<div><br></div><div>Salah satu kebiasaan sederhana.</div>`,
			want: "<p>Bekerja di depan komputer mudah lelah.</p>\n<p>Salah satu kebiasaan sederhana.</p>",
		},
		{
			name: "blok kosong dibuang",
			in:   `<div><br></div><div>&nbsp;</div><p></p><div>ada isi</div>`,
			want: `<p>ada isi</p>`,
		},
		{
			name: "div bersarang",
			in:   `<div><div>teks dalam</div></div>`,
			want: `<p>teks dalam</p>`,
		},
		{
			name: "div dengan atribut",
			in:   `<div class="x" style="color:red">teks</div>`,
			want: `<p>teks</p>`,
		},
		{
			name: "heading dan daftar dibiarkan",
			in:   `<h2>Judul</h2><ul><li>satu</li><li>dua</li></ul>`,
			want: "<h2>Judul</h2>\n<ul><li>satu</li><li>dua</li></ul>",
		},
		{
			name: "tag inline tetap di dalam paragraf",
			in:   `<div>Halo <strong>dunia</strong> dan <a href="https://x.id/">tautan</a></div>`,
			want: `<p>Halo <strong>dunia</strong> dan <a href="https://x.id/">tautan</a></p>`,
		},
		{
			name: "blok berisi gambar tidak dibuang",
			in:   `<div><img src="/uploads/a.webp" alt="a"></div>`,
			want: `<p><img src="/uploads/a.webp" alt="a"></p>`,
		},
		{
			name: "entitas dipertahankan",
			in:   `<div>At&amp;T &lt;tag&gt;</div>`,
			want: `<p>At&amp;T &lt;tag&gt;</p>`,
		},
		{
			name: "teks telanjang jadi paragraf",
			in:   `Halo dunia`,
			want: `<p>Halo dunia</p>`,
		},
		{
			name: "teks polos dipecah per baris kosong",
			in:   "Baris satu\n\nBaris dua",
			want: "<p>Baris satu</p>\n<p>Baris dua</p>",
		},
		{
			name: "atribut berisi kurung sudut tidak memecah tag",
			in:   `<p title="a > b">isi</p>`,
			want: `<p title="a > b">isi</p>`,
		},
		{
			name: "paragraf kosong dari editor ikut hilang",
			in:   `<p>satu</p><p><br></p><p>dua</p>`,
			want: "<p>satu</p>\n<p>dua</p>",
		},
		{
			name: "input kosong",
			in:   ``,
			want: ``,
		},
		{
			name: "input hanya spasi tidak diubah",
			in:   "   ",
			want: "   ",
		},
	}
	for _, tc := range cases {
		got := NormalizeArticleHTML(tc.in)
		if got != tc.want {
			t.Errorf("%s:\n  masuk  %q\n  dapat %q\n  mau   %q", tc.name, tc.in, got, tc.want)
		}
	}
}

// Normalizer dipanggil di beberapa titik penyimpanan sekaligus saat render
// publikasi, jadi hasilnya harus stabil saat dijalankan berulang.
func TestNormalizeArticleHTMLIdempoten(t *testing.T) {
	inputs := []string{
		`Bekerja di depan komputer mudah lelah.<div><br></div><div>Salah satu kebiasaan sederhana.</div>`,
		`<h2>Judul</h2><ul><li>satu</li><li>dua</li></ul>`,
		`<p>satu</p><p>dua</p>`,
		`<div><img src="/uploads/a.webp"></div>`,
		"Baris satu\n\nBaris dua",
		`<blockquote><p>kutipan</p></blockquote>`,
	}
	for _, in := range inputs {
		once := NormalizeArticleHTML(in)
		twice := NormalizeArticleHTML(once)
		if once != twice {
			t.Errorf("tidak idempoten:\n  masuk    %q\n  sekali   %q\n  dua kali %q", in, once, twice)
		}
	}
}

func TestNormalizeArticleHTMLMenjagaKutipanBerparagraf(t *testing.T) {
	in := `<blockquote><p>kutipan penting</p></blockquote><div>lanjutan</div>`
	want := "<blockquote><p>kutipan penting</p></blockquote>\n<p>lanjutan</p>"
	if got := NormalizeArticleHTML(in); got != want {
		t.Fatalf("dapat %q, mau %q", got, want)
	}
}
