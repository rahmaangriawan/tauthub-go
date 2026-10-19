package chat

import (
	"strings"
	"testing"
)

// TestMaskChatMessage membuktikan kontak & keyword obfuscated tersensor,
// sementara URL artikel (draft/backlink) tetap utuh.
func TestMaskChatMessage(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		wantFlagged bool
		wantContain string // potongan asli yang harus tetap ada
		wantMasked  []string
	}{
		{
			name: "telepon format strip", in: "Hubungi saya di 0812-3456-7890 ya",
			wantFlagged: true, wantMasked: []string{"0812-3456-7890"},
		},
		{
			name: "telepon format spasi", in: "nomornya 0812 3456 7890",
			wantFlagged: true, wantMasked: []string{"0812 3456 7890"},
		},
		{
			name: "telepon format koma (obfuscation)", in: "wa 0812,3456,7890",
			wantFlagged: true, wantMasked: []string{"0812,3456,7890"},
		},
		{
			name: "telepon +62", in: "bisa hub ke +62 812 3456 7890",
			wantFlagged: true, wantMasked: []string{"+62 812 3456 7890"},
		},
		{
			name: "telepon zero-width", in: "0812\u200b3456\u200b7890",
			wantFlagged: true, wantMasked: []string{"0812\u200b3456\u200b7890"},
		},
		{
			name: "email standar", in: "kirim ke nama@domain.com",
			wantFlagged: true, wantMasked: []string{"nama@domain.com"},
		},
		{
			name: "email obfuscated", in: "email saya nama [at] domain [dot] com",
			wantFlagged: true, wantMasked: []string{"nama [at] domain [dot] com"},
		},
		{
			name: "keyword whatsapp leet", in: "chat via w4ts4pp saja",
			wantFlagged: true, wantMasked: []string{"w4ts4pp"},
		},
		{
			name: "keyword whatsapp berspasi", in: "hub w h a t s a p p ya",
			wantFlagged: true, wantMasked: []string{"w h a t s a p p"},
		},
		{
			name: "keyword telegram leet", in: "pindah ke t3l3gr4m",
			wantFlagged: true, wantMasked: []string{"t3l3gr4m"},
		},
		{
			name: "keyword bypass", in: "boleh byp4ss komisi?",
			wantFlagged: true, wantMasked: []string{"byp4ss"},
		},
		{
			name: "keyword no rek", in: "kirim ke n0 rek saya",
			wantFlagged: true, wantMasked: []string{"n0 rek"},
		},
		{
			name: "keyword wa berdiri sendiri", in: "lanjut di wa aja",
			wantFlagged: true, wantMasked: []string{"wa"},
		},
		{
			name: "link messenger", in: "chat di wa.me/6281234567890",
			wantFlagged: true, wantMasked: []string{"wa.me/6281234567890"},
		},
		{
			name: "URL artikel dengan skema", in: "draft ada di https://domain.com/artikel-1",
			wantFlagged: false, wantContain: "https://domain.com/artikel-1",
		},
		{
			name: "URL artikel tanpa skema", in: "backlink di namadomain.com/artikel-1",
			wantFlagged: false, wantContain: "namadomain.com/artikel-1",
		},
		{
			name: "URL berisi kata terlarang", in: "baca https://domain.com/artikel-whatsapp",
			wantFlagged: false, wantContain: "https://domain.com/artikel-whatsapp",
		},
		{
			name: "URL berisi angka mirip telepon", in: "lihat https://domain.com/081234567890",
			wantFlagged: false, wantContain: "https://domain.com/081234567890",
		},
		{
			name: "kata biasa tidak kena", in: "waktu dan kawan sudah telepon rumah",
			wantFlagged: false, wantContain: "waktu dan kawan sudah telepon rumah",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, flagged := MaskChatMessage(tc.in)
			if flagged != tc.wantFlagged {
				t.Fatalf("flagged = %v, ingin %v (out=%q)", flagged, tc.wantFlagged, got)
			}
			if tc.wantContain != "" && !strings.Contains(got, tc.wantContain) {
				t.Fatalf("hasil %q tidak memuat %q", got, tc.wantContain)
			}
			for _, leaked := range tc.wantMasked {
				if strings.Contains(got, leaked) {
					t.Fatalf("kontak %q masih bocor di %q", leaked, got)
				}
			}
			if tc.wantFlagged && !strings.Contains(got, MaskedContact) {
				t.Fatalf("placeholder tidak ada di %q", got)
			}
			if !tc.wantFlagged && strings.Contains(got, MaskedContact) {
				t.Fatalf("pesan bersih malah disensor: %q", got)
			}
		})
	}
}
