package brandflow

import (
	"strings"
	"testing"
)

// Regresi: mapHTMLTextNodes wajib mempertahankan tag. Versi lama memakai
// Regexp.Split yang membuang tag (beda dengan String.split di JS) sehingga
// seluruh markup artikel hilang setiap enforceTautBacklinkPolicy jalan.
func TestMapHTMLTextNodesPreservesTags(t *testing.T) {
	in := `<p>Epoxy <strong>lantai</strong> kian <a href="https://x.test/t/abc">dipakai</a> luas.</p>`
	got := mapHTMLTextNodes(in, strings.ToUpper)
	want := `<p>EPOXY <strong>LANTAI</strong> KIAN <a href="https://x.test/t/abc">dipakai</a> LUAS.</p>`
	if got != want {
		t.Fatalf("tag hilang/berubah:\n got: %s\nwant: %s", got, want)
	}
	if same := mapHTMLTextNodes(in, func(s string) string { return s }); same != in {
		t.Fatalf("identitas harus mengembalikan html utuh:\n got: %s\nwant: %s", same, in)
	}
}

// Regresi: artikel Writer yang menyebut anchor utama wajib menyimpan <p>
// utuh dan backlink primary ditanam — bukan teks polos tanpa tag.
func TestFinalizeTautArticleHTMLKeepsParagraphsAndPlantsPrimaryLink(t *testing.T) {
	in := "<p>Permintaan jasa epoxy lantai meningkat di kawasan industri Bandung.</p>" +
		"<p>Bahan resin dicampur sesuai takaran, lalu dituang di atas permukaan beton yang sudah rata.</p>"
	links := []tautBacklinkSpec{
		{URL: "https://klien.test/layanan", Anchor: "jasa epoxy lantai", Mandatory: true},
	}
	got := finalizeTautArticleHTML(in, "mainstream", true, links)
	if !strings.Contains(got, "<p>") {
		t.Fatalf("paragraf hilang: %s", got)
	}
	if strings.Count(got, "<p>") != 2 {
		t.Fatalf("jumlah <p> berubah: %s", got)
	}
	if !strings.Contains(got, `<a href="https://klien.test/layanan"`) {
		t.Fatalf("backlink primary tidak ditanam: %s", got)
	}
	if !strings.Contains(got, ">jasa epoxy lantai</a>") {
		t.Fatalf("anchor primary tidak terbungkus link: %s", got)
	}
}

// Konsep Writer lama: mainstream tanpa heading (h1-h4 diturunkan jadi
// paragraf), blogger tetap ber-heading dan boleh punya daftar + kutipan.
func TestFinalizeTautArticleHTMLStructurePerMediaType(t *testing.T) {
	t.Run("mainstream demotes headings", func(t *testing.T) {
		in := "<p>Lead menjawab siapa apa kapan di mana mengapa bagaimana.</p><h2>Subjudul</h2><p>Isi paragraf kedua.</p>"
		got := finalizeTautArticleHTML(in, "mainstream", false, nil)
		if !strings.Contains(got, "<p>Subjudul</p>") {
			t.Fatalf("h2 tidak diturunkan jadi paragraf: %s", got)
		}
		if strings.Contains(got, "<h2") {
			t.Fatalf("mainstream tidak boleh menyimpan heading: %s", got)
		}
	})

	t.Run("blogger keeps headings, lists and quotes", func(t *testing.T) {
		in := "<p>Jenis Epoxy Lantai</p><p>Ada beberapa jenis yang umum dipakai.</p>" +
			"<ul><li>Epoxy trowel</li><li>Epoxy self-leveling</li></ul>" +
			"<blockquote><p>Menurut kontraktor, lapisan ini tahan lima tahun.</p></blockquote>"
		got := finalizeTautArticleHTML(in, "blogger", false, nil)
		for _, want := range []string{"<h2>Jenis Epoxy Lantai</h2>", "<ul>", "<li>Epoxy trowel</li>", "<blockquote>"} {
			if !strings.Contains(got, want) {
				t.Fatalf("blogger kehilangan %s: %s", want, got)
			}
		}
	})
}

// Regresi: model menanam URL yang sama lebih dari sekali (anchor diulang atau
// "di-divariasikan") → pasca-proses wajib menyisakan TEPAT SATU link per URL.
func TestEnforceTautBacklinkPolicyKeepsSingleLinkPerURL(t *testing.T) {
	in := "<p>Solusi epoxy lantai dari <a href=\"https://klien.test/layanan\">jasa epoxy lantai</a> populer.</p>" +
		"<p>Bagi pemula, <a href=\"https://klien.test/layanan\">layanan epoxy profesional</a> jadi pilihan utama.</p>"
	links := []tautBacklinkSpec{{URL: "https://klien.test/layanan", Anchor: "jasa epoxy lantai", Mandatory: true}}
	got := enforceTautBacklinkPolicy(in, links)
	if n := strings.Count(got, "https://klien.test/layanan"); n != 1 {
		t.Fatalf("URL yang sama muncul %dx, harus 1: %s", n, got)
	}
	if !strings.Contains(got, ">jasa epoxy lantai</a>") {
		t.Fatalf("link yang tersisa tidak memakai anchor spesifikasi: %s", got)
	}
}

// Regresi: link utama sudah ditanam model dengan href yang ter-escape (&amp;)
// atau diberi trailing slash — enforcement tidak boleh salah anggap link
// hilang lalu menyisipkan link kedua.
func TestEnforceTautBacklinkPolicyDetectsEscapedHrefAndTrailingSlash(t *testing.T) {
	links := []tautBacklinkSpec{{URL: "https://klien.test/layanan?id=7&ref=x", Anchor: "jasa epoxy lantai", Mandatory: true}}
	for _, href := range []string{
		"https://klien.test/layanan?id=7&amp;ref=x",
		"https://klien.test/layanan?id=7&ref=x/",
	} {
		in := "<p>Konsultasikan kebutuhan Anda pada dasbor tim Bumi Cafe.</p><p>Lihat <a href=\"" + href + "\">jasa epoxy lantai</a> untuk detail.</p>"
		got := enforceTautBacklinkPolicy(in, links)
		if n := strings.Count(got, "<a "); n != 1 {
			t.Fatalf("href %s: link jadi %d, harus tetap 1: %s", href, n, got)
		}
	}
}

// Regresi kasus nyata: model menanam URL yang sama dua kali dengan dua anchor
// yang hanya beda kapitalisasi ("Bukalapak" vs "bukalapak"). Sisakan SATU, dan
// yang tersisa harus persis seperti anchor yang diketik klien.
func TestEnforceTautBacklinkPolicyKeepExactCaseAnchor(t *testing.T) {
	in := "<p>PT <a href=\"https://bukalapak.com\">Bukalapak</a>.com Tbk resmi menyetujui pengangkatan direksi baru.</p>" +
		"<p>Stabilitas tata kelola ini memantapkan PT <a href=\"https://bukalapak.com\">bukalapak</a> Tbk mengejar profitabilitas.</p>"
	links := []tautBacklinkSpec{{URL: "https://bukalapak.com", Anchor: "bukalapak", Mandatory: true}}
	got := enforceTautBacklinkPolicy(in, links)
	if n := strings.Count(got, "https://bukalapak.com"); n != 1 {
		t.Fatalf("URL sama masih muncul %dx, harus 1: %s", n, got)
	}
	if !strings.Contains(got, ">bukalapak</a>") {
		t.Fatalf("anchor tersisa bukan persis seperti input klien: %s", got)
	}
	if strings.Contains(got, ">Bukalapak</a>") {
		t.Fatalf("anchor variasi kapital masih tersisa: %s", got)
	}
}
