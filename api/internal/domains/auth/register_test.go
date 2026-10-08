package authdomain

import "testing"

// Pesan penolakan registrasi lintas peran menyebut peran yang sudah terdaftar;
// salah label membuat pengguna bingung harus masuk ke halaman mana.
func TestRoleLabelID(t *testing.T) {
	cases := map[string]string{
		"partner": "mitra",
		"client":  "klien",
		"admin":   "akun Tauthub",
		"":        "akun Tauthub",
	}
	for role, want := range cases {
		if got := roleLabelID(role); got != want {
			t.Errorf("roleLabelID(%q) = %q, mau %q", role, got, want)
		}
	}
}
