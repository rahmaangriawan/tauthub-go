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
