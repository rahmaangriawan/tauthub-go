package platform

import "testing"

// pending_changes tiba sebagai string JSON dari pgx (normalizeValue), bukan
// array. Tanpa parsing, tabel admin menganggap tidak ada perubahan dan selalu
// menulis "Pengajuan baru — tanpa perubahan data." — termasuk saat mitra
// mengubah masa berlaku tautan.
func TestMPJSONValueParsesArray(t *testing.T) {
	raw := `[{"field":"link_duration_months","label":"Masa Berlaku Tautan","from":"12 bulan","to":"Permanen"}]`
	got := mpJSONValue(raw)
	arr, ok := got.([]any)
	if !ok {
		t.Fatalf("harus jadi array, dapat %T", got)
	}
	if len(arr) != 1 {
		t.Fatalf("panjang array salah: %d", len(arr))
	}
	item, ok := arr[0].(map[string]any)
	if !ok {
		t.Fatalf("item harus objek, dapat %T", arr[0])
	}
	if item["label"] != "Masa Berlaku Tautan" || item["to"] != "Permanen" {
		t.Fatalf("isi perubahan salah: %#v", item)
	}
}

func TestMPJSONValueHandlesNilAndEmpty(t *testing.T) {
	if got := mpJSONValue(nil); got != nil {
		t.Fatalf("nil harus tetap nil, dapat %#v", got)
	}
	if got := mpJSONValue(""); got != nil {
		t.Fatalf("string kosong harus nil, dapat %#v", got)
	}
	if got := mpJSONValue("bukan json"); got != nil {
		t.Fatalf("json rusak harus nil, dapat %#v", got)
	}
}
