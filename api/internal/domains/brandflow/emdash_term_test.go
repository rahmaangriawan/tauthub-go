package brandflow

import (
	"reflect"
	"testing"
)

// emdash_term_test.go: logika murni resolusi term publikasi — dedupe nama
// berulang dalam satu panggilan, urutan keluaran, dan kesetaraannya dengan
// resolusi per-term penuh. Semua lewat server tiruan; tidak ada HTTP nyata
// (jalur peFetchWPJSON dibungkus SafeClient yang memblokir loopback, jadi
// sengaja tidak dilewati dari sini).

// peTermServerUji: mitra term deterministik. Nama yang sudah terlihat
// mengembalikan id-nya; nama yang belum ada "dibuat" dengan id berurutan.
// hideNew meniru perilaku wp/v2 asli (hide_empty=true): term yang baru dibuat
// belum terlihat oleh ?search= sampai punya post, dan create ulang atas nama
// yang sama ditolak mitra (HTTP 400 term_exists → dilewati).
type peTermServerUji struct {
	ids     map[string]int
	hidden  map[string]int
	berikut int
	create  bool
	hideNew bool
	panggil int
}

func peTermServerUjiBaru(ada map[string]int, create, hideNew bool) *peTermServerUji {
	ids := make(map[string]int, len(ada))
	for name, id := range ada {
		ids[name] = id
	}
	return &peTermServerUji{ids: ids, hidden: map[string]int{}, berikut: 100, create: create, hideNew: hideNew}
}

func (s *peTermServerUji) resolve(name string) int {
	s.panggil++
	if id, ok := s.ids[name]; ok && id > 0 {
		return id
	}
	if !s.create {
		return 0
	}
	if _, sudah := s.hidden[name]; sudah {
		return 0 // create ulang ditolak mitra (term_exists)
	}
	s.berikut++
	if s.hideNew {
		s.hidden[name] = s.berikut
	} else {
		s.ids[name] = s.berikut
	}
	return s.berikut
}

// peResolvePerTermPenuh: perilaku lama — tiap kemunculan nama di-resolve
// independen (search ulang, create ulang bila perlu).
func peResolvePerTermPenuh(s *peTermServerUji, names []string) []int {
	out := []int{}
	for _, name := range names {
		if id := s.resolve(name); id > 0 {
			out = append(out, id)
		}
	}
	return out
}

func TestPeTermDedupeSlots(t *testing.T) {
	slots := peTermDedupeSlots([]string{"Berita", "Teknologi", "Berita", "Gaya Hidup", "Berita"})
	want := []int{0, 1, 0, 3, 0}
	if !reflect.DeepEqual(slots, want) {
		t.Fatalf("slots = %v, mau %v", slots, want)
	}
}

func TestPeTermDedupeSlotsVarianNamaTetapTerpisah(t *testing.T) {
	// Huruf besar/kecil dan spasi menghasilkan query `?search=` berbeda di
	// mitra, jadi tidak boleh digabung walau setelah normalisasi terlihat sama.
	slots := peTermDedupeSlots([]string{"Berita", "berita", "Berita ", "berita"})
	want := []int{0, 1, 2, 1}
	if !reflect.DeepEqual(slots, want) {
		t.Fatalf("slots = %v, mau %v", slots, want)
	}
	if got := peTermDedupeSlots(nil); len(got) != 0 {
		t.Fatalf("nama kosong harus menghasilkan slot kosong, dapat %v", got)
	}
}

func TestPePlanTermResolutionUrarutanDanLewati(t *testing.T) {
	// create=false: hanya nama terdaftar yang menghasilkan id; kemunculan
	// ulang menyalin id pertamanya; nama absen hilang dari keluaran.
	server := peTermServerUjiBaru(map[string]int{"Berita": 5, "Teknologi": 9}, false, false)
	got := pePlanTermResolution(
		[]string{"Berita", "Hilang", "Teknologi", "Berita", "Hilang"},
		server.resolve,
	)
	want := []int{5, 9, 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hasil = %v, mau %v", got, want)
	}
	if server.panggil != 3 {
		t.Fatalf("resolve dipanggil %d kali, mau 3 (sekali per nama unik)", server.panggil)
	}
}

func TestPePlanTermResolutionSetaraPerTermPenuh(t *testing.T) {
	kasus := []struct {
		nama   []string
		ada    map[string]int
		create bool
	}{
		{[]string{"Berita", "Teknologi"}, map[string]int{"Berita": 5, "Teknologi": 9}, true},
		{[]string{"Berita", "Teknologi", "Berita", "Baru", "Berita", "Baru"}, map[string]int{"Berita": 5, "Teknologi": 9}, true},
		{[]string{"Berita", "Teknologi", "Berita", "Hilang", "Hilang"}, map[string]int{"Berita": 5, "Teknologi": 9}, false},
		{[]string{"Sendirian"}, map[string]int{}, true},
		{nil, map[string]int{"Berita": 5}, true},
	}
	for i, kc := range kasus {
		// Jalur baru: nama unik di-resolve sekali.
		baru := peTermServerUjiBaru(kc.ada, kc.create, false)
		got := pePlanTermResolution(kc.nama, baru.resolve)
		// Jalur lama: tiap kemunculan di-resolve independen terhadap mitra
		// yang sama (deterministik).
		lama := peTermServerUjiBaru(kc.ada, kc.create, false)
		want := peResolvePerTermPenuh(lama, kc.nama)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("kasus %d: hasil %v ≠ per-term penuh %v (nama %v, create %v)", i, got, want, kc.nama, kc.create)
		}
	}
}

func TestPePlanTermResolutionPayloadMitraSamaPadaHideEmpty(t *testing.T) {
	// Mitra wp/v2 asli menyembunyikan term baru (hide_empty=true): pencarian
	// kedua atas nama yang sama tidak menemukan term yang baru dibuat, lalu
	// create ulangnya ditolak (400). Output mentah bisa beda satu duplikat id,
	// tetapi builder payload menjalankan bsDedupeInts sebelum dikirim —
	// payload ke mitra harus tetap identik.
	kasus := []struct {
		nama   []string
		ada    map[string]int
		create bool
	}{
		{[]string{"Baru", "Baru"}, map[string]int{}, true},
		{[]string{"Lama", "Baru", "Baru", "Lama"}, map[string]int{"Lama": 7}, true},
		{[]string{"Hilang", "Hilang"}, map[string]int{}, false},
	}
	for i, kc := range kasus {
		baru := peTermServerUjiBaru(kc.ada, kc.create, true)
		got := pePlanTermResolution(kc.nama, baru.resolve)
		lama := peTermServerUjiBaru(kc.ada, kc.create, true)
		want := peResolvePerTermPenuh(lama, kc.nama)
		if !reflect.DeepEqual(bsDedupeInts(got), bsDedupeInts(want)) {
			t.Fatalf("kasus %d: payload terdedupe %v ≠ %v (nama %v)", i, bsDedupeInts(got), bsDedupeInts(want), kc.nama)
		}
	}
}
