package brandflow

import "testing"

// Uji logika pemeriksaan tautan DI DALAM artikel. Bagian ini yang menentukan
// status "Tautan Dihapus", jadi salah di sini berarti menuduh media mencabut
// tautan tanpa dasar. Semua kasus di bawah memakai HTML apa adanya, tanpa
// jaringan.

func TestExtractPageLinks(t *testing.T) {
	html := `<p>Lihat <a href="https://contoh.id/artikel" class="x">Jasa Artikel SEO</a> ya.</p>
		<a href='https://lain.id/promo' target="_blank">Promo <strong>Spesial</strong></a>
		<a href="/relatif">relatif</a>
		<a name="anchor">tanpa href</a>
		<a href="https://contoh.id/artikel#bagian">fragmen</a>`

	links := extractPageLinks(html)
	if len(links) != 3 {
		t.Fatalf("harus 3 tautan absolut, dapat %d: %+v", len(links), links)
	}
	if links[0].URL != "https://contoh.id/artikel" || links[0].Anchor != "Jasa Artikel SEO" {
		t.Errorf("tautan pertama salah: %+v", links[0])
	}
	if links[1].URL != "https://lain.id/promo" || links[1].Anchor != "Promo Spesial" {
		t.Errorf("anchor harus dibersihkan dari tag: %+v", links[1])
	}
	if links[2].Anchor != "fragmen" {
		t.Errorf("tautan berfragmen harus tetap terbaca: %+v", links[2])
	}
}

func TestExpectedLinksFromDocument(t *testing.T) {
	doc := `<a href="https://klien.id/produk">Produk</a> <a href="https://klien.id/produk">Produk lagi</a>
		<a href="https://klien.id/layanan/">Layanan</a>`
	links := expectedLinksFromDocument(doc)
	if len(links) != 2 {
		t.Fatalf("tautan kembar harus disatukan, dapat %d", len(links))
	}
	if links[0].Anchor != "Produk" || links[1].Anchor != "Layanan" {
		t.Errorf("urutan anchor tidak sesuai: %+v", links)
	}
}

func TestCompareArticleLinks(t *testing.T) {
	expected := []pageLink{
		{URL: "https://klien.id/produk", Anchor: "Jasa Artikel SEO"},
		{URL: "https://klien.id/layanan/", Anchor: "Layanan Kami"},
	}
	cases := []struct {
		name        string
		found       []pageLink
		wantState   string
		wantAnchor  string
		wantFoundAt int
	}{
		{
			name: "semua tautan masih ada",
			found: []pageLink{
				{URL: "https://klien.id/produk", Anchor: "Jasa Artikel SEO"},
				{URL: "https://klien.id/layanan/", Anchor: "Layanan Kami"},
				{URL: "https://media.id/lain", Anchor: "lain"},
			},
			wantState: "present", wantAnchor: "Jasa Artikel SEO", wantFoundAt: 3,
		},
		{
			name: "tautan kedua dicabut",
			found: []pageLink{
				{URL: "https://klien.id/produk", Anchor: "Jasa Artikel SEO"},
			},
			wantState: "removed", wantAnchor: "Layanan Kami", wantFoundAt: 1,
		},
		{
			name:      "halaman terbaca tapi tanpa tautan",
			found:     nil,
			wantState: "unknown", wantAnchor: "Jasa Artikel SEO", wantFoundAt: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state, anchor, foundCount := compareArticleLinks(expected, tc.found)
			if state != tc.wantState {
				t.Errorf("state = %q, mau %q", state, tc.wantState)
			}
			if anchor != tc.wantAnchor {
				t.Errorf("anchor = %q, mau %q", anchor, tc.wantAnchor)
			}
			if foundCount != tc.wantFoundAt {
				t.Errorf("jumlah ditemukan = %d, mau %d", foundCount, tc.wantFoundAt)
			}
		})
	}
}

func TestCompareArticleLinksToleranKosmetik(t *testing.T) {
	expected := []pageLink{{URL: "https://Klien.id/Produk/", Anchor: "Produk"}}
	// Media menulis ulang skema/trailing slash: tetap dianggap ada.
	found := []pageLink{{URL: "http://klien.id/Produk", Anchor: "Produk"}}
	if state, _, _ := compareArticleLinks(expected, found); state != "present" {
		t.Errorf("perbedaan kosmetik tidak boleh dianggap tautan hilang, dapat %q", state)
	}
}

func TestCompareArticleLinksTanpaAcuan(t *testing.T) {
	if state, _, _ := compareArticleLinks(nil, []pageLink{{URL: "https://a.id"}}); state != "unknown" {
		t.Errorf("dokumen tanpa tautan acuan harus 'unknown', dapat %q", state)
	}
}
