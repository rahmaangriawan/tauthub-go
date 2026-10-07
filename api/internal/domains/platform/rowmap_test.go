package platform

import "testing"

// Kolom smallint (da, pa, spam_score, ai_trust_score, credit_cost, …) dikirim
// pgx sebagai int16; tanpa cabang itu semua pembacaan lewat rowMap (numOr,
// numPtr, intOr) mengembalikan 0/nil sehingga JSON admin kehilangan angkanya.
func TestRowMapNumHandlesDriverIntegerWidths(t *testing.T) {
	cases := []struct {
		name string
		val  any
		want float64
	}{
		{"int16 (smallint)", int16(19), 19},
		{"int8", int8(7), 7},
		{"int32 (integer)", int32(345), 345},
		{"int64 (bigint)", int64(575), 575},
		{"int", int(230), 230},
		{"float32 (real)", float32(12.5), 12.5},
		{"float64", float64(65.9), 65.9},
		{"numeric string", "81", 81},
	}
	for _, tc := range cases {
		m := rowMap{"v": tc.val}
		got, ok := m.num("v")
		if !ok || got != tc.want {
			t.Errorf("%s: num() = (%v, %v), ingin (%v, true)", tc.name, got, ok, tc.want)
		}
	}
}

// Nilai kosong tetap harus menghasilkan 0/nil, bukan error.
func TestRowMapNumNullStaysAbsent(t *testing.T) {
	m := rowMap{"v": nil}
	if got, ok := m.num("v"); ok || got != 0 {
		t.Errorf("nil: num() = (%v, %v), ingin (0, false)", got, ok)
	}
	if numPtr := m.numPtr("v"); numPtr != nil {
		t.Errorf("nil: numPtr() = %v, ingin nil", numPtr)
	}
}
