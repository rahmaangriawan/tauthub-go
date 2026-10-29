package clienttaut

import (
	"strings"
	"testing"
)

// Regresi gelombang 3: gatekeeper memuat dokumen + versi artikel semua item
// dalam satu query. Nilai default untuk baris yang hilang harus sama dengan
// baca per dokumen: dokumen tidak ada = title ""/versi 1, versi tidak ada =
// isi ""/atribusi "".
func TestMapGatekeeperDocRowsDefaults(t *testing.T) {
	rows := []rowMap{
		{"document_id": "doc-1", "title": "Judul A", "current_version": int64(3), "content": "<p>isi</p>", "attribution": "ai_generated"},
		// Padanan dokumen tidak ada: semua kolom dari doc/cv bernilai NULL.
		{"document_id": "doc-2", "title": nil, "current_version": nil, "content": nil, "attribution": nil},
		{"document_id": "doc-3", "title": "Judul C", "current_version": int64(7)},
	}
	got := mapGatekeeperDocRows(rows)
	if len(got) != 3 {
		t.Fatalf("jumlah entri = %d, want 3", len(got))
	}
	a := got["doc-1"]
	if a.title != "Judul A" || a.currentVersion != 3 || a.content != "<p>isi</p>" || a.attribution != "ai_generated" {
		t.Fatalf("doc-1 salah: %+v", a)
	}
	b := got["doc-2"]
	if b.title != "" || b.currentVersion != 1 || b.content != "" || b.attribution != "" {
		t.Fatalf("dokumen hilang harus memakai default lama: %+v", b)
	}
	c := got["doc-3"]
	if c.title != "Judul C" || c.currentVersion != 7 || c.content != "" || c.attribution != "" {
		t.Fatalf("doc-3 (versi tidak ada) salah: %+v", c)
	}
}

// Regresi gelombang 3: dokumen yang dipakai beberapa item harus terhitung,
// karena item kedua dan seterusnya wajib baca ulang langsung (versi bisa
// dinaikkan item sebelumnya di tengah loop).
func TestGatekeeperDocUsageCountsDuplicates(t *testing.T) {
	items := []rowMap{
		{"article_document_id": "doc-1"},
		{"article_document_id": "doc-2"},
		{"article_document_id": "doc-1"},
	}
	ids, counts := gatekeeperDocUsage(items)
	if len(ids) != 3 || ids[0] != "doc-1" || ids[1] != "doc-2" || ids[2] != "doc-1" {
		t.Fatalf("urutan id salah: %v", ids)
	}
	if counts["doc-1"] != 2 || counts["doc-2"] != 1 {
		t.Fatalf("hitungan salah: %v", counts)
	}
}

// Regresi gelombang 3: penulisan hasil matchmaker. Cabang existing vs baru
// tetap dari snapshot existingIDs; nilai kolom dan urutan argumen INSERT
// harus sama dengan loop satu-query-per-media.
func TestPlanMatchmakerWrites(t *testing.T) {
	pool := []candidateLine{
		{MediaID: "m1", Name: "Media Satu"},
		{MediaID: "m2", Name: "Media Dua"},
		{MediaID: "m3", Name: "Media Tiga"},
	}
	ranked := []rankedItem{
		{MediaID: "m1", Alasan: "alasan bagus", Skor: 90},
		{MediaID: "m3", Alasan: "", Skor: 70},
	}
	out, plan := planMatchmakerWrites("camp-1", pool, ranked, map[string]bool{"m1": true})

	if len(out) != 3 {
		t.Fatalf("jumlah out = %d, want 3", len(out))
	}
	if out[0].Status != "selected" || out[0].Score != 90 {
		t.Fatalf("m1 harus terpilih skor 90: %+v", out[0])
	}
	if out[1].Status != "removed" || out[1].Score != nil {
		t.Fatalf("m2 tidak masuk ranked harus removed tanpa skor: %+v", out[1])
	}
	if out[2].Status != "selected" || out[2].Score != 70 {
		t.Fatalf("m3 harus terpilih skor 70: %+v", out[2])
	}

	if len(plan.updateRows) != 1 {
		t.Fatalf("baris update = %d, want 1", len(plan.updateRows))
	}
	upd := plan.updateRows[0]
	if upd["media_id"] != "m1" || upd["status"] != "selected" || upd["ai_score"] != 90 || upd["match_rank"] != 1 {
		t.Fatalf("payload update m1 salah: %v", upd)
	}
	if upd["match_reason"] != "alasan bagus" {
		t.Fatalf("match_reason m1 salah: %v", upd["match_reason"])
	}

	wantSQL := "INSERT INTO taut_campaign_media (id, campaign_id, media_id, slot, source, status, match_reason, ai_score, match_rank) " +
		"VALUES ($1, $2, $3, 1, $4, $5, $6, $7), ($8, $9, $10, 1, $11, $12, $13, $14)"
	if plan.insertSQL != wantSQL {
		t.Fatalf("SQL insert salah:\n got: %s\nwant: %s", plan.insertSQL, wantSQL)
	}
	if len(plan.insertArgs) != 14 {
		t.Fatalf("jumlah argumen insert = %d, want 14", len(plan.insertArgs))
	}
	id1, id1OK := plan.insertArgs[0].(string)
	id2, id2OK := plan.insertArgs[7].(string)
	if !id1OK || !id2OK || id1 == id2 || len(id1) != 36 || len(id2) != 36 {
		t.Fatalf("id insert harus uuid baru berbeda per baris: %q %q", plan.insertArgs[0], plan.insertArgs[7])
	}
	if plan.insertArgs[1] != "camp-1" || plan.insertArgs[2] != "m2" || plan.insertArgs[3] != "removed" {
		t.Fatalf("argumen insert m2 salah: %v", plan.insertArgs[1:4])
	}
	if plan.insertArgs[4] != "Media Dua: kurang relevan dengan brief kampanye." || plan.insertArgs[5] != nil || plan.insertArgs[6] != nil {
		t.Fatalf("reason/score/rank m2 salah: %v", plan.insertArgs[4:7])
	}
	if plan.insertArgs[8] != "camp-1" || plan.insertArgs[9] != "m3" || plan.insertArgs[10] != "selected" {
		t.Fatalf("argumen insert m3 salah: %v", plan.insertArgs[8:11])
	}
	if plan.insertArgs[11] != "Media Tiga: kurang relevan dengan brief kampanye." || plan.insertArgs[12] != 70 || plan.insertArgs[13] != 2 {
		t.Fatalf("reason/score/rank m3 salah: %v", plan.insertArgs[11:14])
	}
}

// Pool kosong: tidak boleh ada statement yang tersusun.
func TestPlanMatchmakerWritesEmpty(t *testing.T) {
	out, plan := planMatchmakerWrites("camp-1", nil, nil, map[string]bool{})
	if len(out) != 0 || len(plan.updateRows) != 0 || len(plan.insertArgs) != 0 || plan.insertSQL != "" {
		t.Fatalf("pool kosong harus menghasilkan rencana kosong: %+v", plan)
	}
}

// SQL insert memakai kolom yang sama dengan statement lama.
func TestPlanMatchmakerWritesSQLShape(t *testing.T) {
	_, plan := planMatchmakerWrites("camp-1", []candidateLine{{MediaID: "m1", Name: "M1"}}, nil, map[string]bool{})
	if !strings.HasPrefix(plan.insertSQL, "INSERT INTO taut_campaign_media (id, campaign_id, media_id, slot, source, status, match_reason, ai_score, match_rank)") {
		t.Fatalf("kolom insert berubah: %s", plan.insertSQL)
	}
	if !strings.Contains(plan.insertSQL, "($1, $2, $3, 1, $4, $5, $6, $7)") {
		t.Fatalf("placeholder per baris salah: %s", plan.insertSQL)
	}
}
