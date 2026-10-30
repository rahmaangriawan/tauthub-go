package brandflow

import (
	"net/http/httptest"
	"testing"
)

// Satu domain bisa punya beberapa artikel (slot). Pemindaian QC untuk satu
// artikel tidak boleh menyapu artikel lain di domain yang sama.
func TestFilterScopeBySlots(t *testing.T) {
	rows := []bsRow{
		{"media_id": "m1", "slot": int64(1)},
		{"media_id": "m1", "slot": int64(2)},
		{"media_id": "m2", "slot": int64(1)},
	}

	t.Run("slot kosong berarti semua", func(t *testing.T) {
		got := filterScopeBySlots(rows, nil)
		if len(got) != 3 {
			t.Fatalf("ingin 3 baris, dapat %d", len(got))
		}
	})

	t.Run("hanya slot yang diminta", func(t *testing.T) {
		got := filterScopeBySlots(rows, []int{2})
		if len(got) != 1 {
			t.Fatalf("ingin 1 baris, dapat %d", len(got))
		}
		if got[0].intOr("slot", 0) != 2 || got[0].strOr("media_id", "") != "m1" {
			t.Fatalf("baris yang tersisa salah: %#v", got[0])
		}
	})

	t.Run("slot 1 di semua media", func(t *testing.T) {
		got := filterScopeBySlots(rows, []int{1})
		if len(got) != 2 {
			t.Fatalf("ingin 2 baris, dapat %d", len(got))
		}
	})

	t.Run("slot tidak ada di kampanye", func(t *testing.T) {
		got := filterScopeBySlots(rows, []int{3})
		if len(got) != 0 {
			t.Fatalf("ingin 0 baris, dapat %d", len(got))
		}
	})

	t.Run("beberapa slot sekaligus", func(t *testing.T) {
		got := filterScopeBySlots(rows, []int{1, 2})
		if len(got) != 3 {
			t.Fatalf("ingin 3 baris, dapat %d", len(got))
		}
	})
}

// ?slot=N mengalamatkan artikel ke-N pada satu media. Nilai tidak sah harus
// jatuh ke artikel pertama, bukan diteruskan apa adanya ke query.
func TestSlotParam(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"", 1},
		{"1", 1},
		{"2", 2},
		{"7", 7},
		{"0", 1},
		{"-3", 1},
		{"abc", 1},
		{"1001", 1},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/?slot="+c.raw, nil)
		if got := slotParam(r); got != c.want {
			t.Errorf("slot=%q → %d, ingin %d", c.raw, got, c.want)
		}
	}
}
