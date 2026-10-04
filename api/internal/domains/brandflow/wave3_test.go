package brandflow

import (
	"strings"
	"testing"
)

// Regresi gelombang 3: penyusun multi-row INSERT content_batch_items. Nilai,
// urutan, dan penomoran placeholder harus sama dengan loop INSERT lama:
// per baris (id baru, brand_id, batch_id, row_number=i+1, fingerprint,
// brief_payload::jsonb, status literal 'pending').
func TestBuildBatchItemInsert(t *testing.T) {
	parsed := []batchBriefRow{
		{Topic: "Topik A", Raw: map[string]any{"topic": "Topik A", "target_words": 500.0}},
		{Topic: "Topik B", Raw: map[string]any{"topic": "Topik B"}},
		{Topic: "Topik C", Raw: map[string]any{"topic": "Topik C"}},
	}
	fingerprints := []string{"fp-a", "fp-b", "fp-c"}
	sql, args := buildBatchItemInsert(parsed, "brand-1", "batch-1", fingerprints)

	wantPrefix := "INSERT INTO content_batch_items (id, brand_id, batch_id, row_number, fingerprint, brief_payload, status) VALUES "
	if !strings.HasPrefix(sql, wantPrefix) {
		t.Fatalf("header SQL berubah:\n got: %s", sql)
	}
	wantTuples := "($1, $2, $3, $4, $5, $6::jsonb, 'pending'), ($7, $8, $9, $10, $11, $12::jsonb, 'pending'), ($13, $14, $15, $16, $17, $18::jsonb, 'pending')"
	if strings.TrimPrefix(sql, wantPrefix) != wantTuples {
		t.Fatalf("daftar VALUES salah:\n got: %s\nwant: %s", strings.TrimPrefix(sql, wantPrefix), wantTuples)
	}
	if len(args) != 18 {
		t.Fatalf("jumlah argumen = %d, want 18", len(args))
	}
	for i, brief := range parsed {
		base := i * 6
		id, ok := args[base].(string)
		if !ok || len(id) != 36 {
			t.Fatalf("baris %d: id bukan uuid: %v", i, args[base])
		}
		if i > 0 {
			if prev, _ := args[base-6].(string); prev == id {
				t.Fatalf("id baris %d sama dengan baris sebelumnya", i)
			}
		}
		if args[base+1] != "brand-1" || args[base+2] != "batch-1" {
			t.Fatalf("baris %d: brand/batch salah: %v", i, args[base+1:base+3])
		}
		if args[base+3] != i+1 {
			t.Fatalf("baris %d: row_number = %v, want %d", i, args[base+3], i+1)
		}
		if args[base+4] != fingerprints[i] {
			t.Fatalf("baris %d: fingerprint = %v, want %s", i, args[base+4], fingerprints[i])
		}
		if args[base+5] != marshalJSON(brief.Raw) {
			t.Fatalf("baris %d: brief_payload = %v, want %s", i, args[base+5], marshalJSON(brief.Raw))
		}
	}
}

// Baris tunggal harus menghasilkan bentuk statement yang persis sama dengan
// INSERT lama (satu baris pun tetap satu statement).
func TestBuildBatchItemInsertSingleRow(t *testing.T) {
	sql, args := buildBatchItemInsert(
		[]batchBriefRow{{Topic: "Topik", Raw: map[string]any{}}}, "b", "bt", []string{"fp"})
	if !strings.Contains(sql, "($1, $2, $3, $4, $5, $6::jsonb, 'pending')") {
		t.Fatalf("tuple satu baris salah: %s", sql)
	}
	if len(args) != 6 || args[3] != 1 {
		t.Fatalf("argumen satu baris salah: %v", args)
	}
}
