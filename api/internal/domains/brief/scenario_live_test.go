//go:build manual

package brief

// Uji skenario percakapan ke MODEL SUNGGUHAN (bukan mock). Gated dengan build
// tag supaya tidak ikut `go test ./...` di CI — tiap giliran adalah panggilan
// berbayar ke provider.
//
// Jalankan:
//
//	cd api && go test -tags manual ./internal/domains/brief/ -run TestLiveSkenario -v
//
// Butuh DB lokal + taut_brief_config yang sudah aktif (base_url, key, model).

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/rahmaangriawan/tauthub-go/internal/config"
)

func newLiveDeps(t *testing.T) (*Deps, context.Context) {
	t.Helper()
	// `go test` berjalan di direktori paket, sedangkan .env ada di akar api/ —
	// muat dari beberapa kandidat supaya tes bisa dijalankan tanpa ekspor env manual.
	for _, candidate := range []string{"../../../.env", "../../.env", ".env"} {
		if _, statErr := os.Stat(candidate); statErr == nil {
			_ = godotenv.Load(candidate)
			break
		}
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DBDSN())
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(pool.Close)
	return &Deps{Pool: pool, Cfg: cfg}, ctx
}

// liveTurn: satu giliran terhadap state buatan (tidak menyentuh Profiler siapa pun).
func liveTurn(d *Deps, ctx context.Context, asked string, states map[string]FieldState, values map[string]any, msg string) (*EngineOutput, []FieldUpdate, []string) {
	cfg, _ := d.loadConfig(ctx)
	prompt, _ := d.loadActivePrompt(ctx)
	fields, _, _ := d.loadActiveFieldSet(ctx)

	tr := &turn{
		Config: cfg, Prompt: prompt, Fields: fields,
		States: states, Values: values, AskedField: asked,
		History:     []message{{Role: "user", Content: msg}},
		ProfileName: "(uji skenario)", UserName: "Penguji",
	}
	tr.Phase = DeterminePhase(PhaseInput{
		HasUserMessage: true,
		RequiredTotal:  totalRequired(fields),
	})

	run, err := d.runEngine(ctx, tr, nil)
	if err != nil {
		return nil, nil, []string{"ERROR: " + err.Error()}
	}
	accepted, rejected := ValidateUpdates(fields, states, run.Output.FieldUpdates)
	return run.Output, accepted, rejected
}

func acceptedValue(updates []FieldUpdate, key string) (string, bool) {
	for _, u := range updates {
		if u.Key == key {
			return u.Value, true
		}
	}
	return "", false
}

func TestLiveSkenario(t *testing.T) {
	d, ctx := newLiveDeps(t)
	cfg, err := d.loadConfig(ctx)
	if err != nil || !cfg.configured() {
		t.Skip("taut_brief_config belum aktif — isi dulu di /admin/taut-brief")
	}

	// A. Jawaban satu kata menempel ke pertanyaan terakhir (kasus "rabiyuk").
	t.Run("A_jawaban_satu_kata", func(t *testing.T) {
		out, updates, rejected := liveTurn(d, ctx, "canonical_name",
			map[string]FieldState{}, map[string]any{}, "rabiyuk")
		if out == nil {
			t.Fatalf("gagal memanggil model: %v", rejected)
		}
		t.Logf("reply: %s", strings.TrimSpace(out.Reply))
		val, ok := acceptedValue(updates, "canonical_name")
		if !ok {
			t.Fatalf("HARAPAN GAGAL: canonical_name tidak terisi dari jawaban satu kata 'rabiyuk' (rejected=%v)", rejected)
		}
		if !strings.Contains(strings.ToLower(val), "rabiyuk") {
			t.Fatalf("nilai canonical_name tidak memuat 'rabiyuk': %q", val)
		}
		if strings.Contains(strings.ToLower(out.Reply), "typo") || strings.Contains(strings.ToLower(out.Reply), "kepotong") {
			t.Errorf("balasan masih menuduh typo: %s", out.Reply)
		}
	})

	// B. Dua informasi dalam satu pesan: keduanya diekstrak, tidak ditanya ulang.
	t.Run("B_dua_info_sekaligus", func(t *testing.T) {
		out, updates, rejected := liveTurn(d, ctx, "canonical_name",
			map[string]FieldState{}, map[string]any{},
			"brand-ku Rabiyuk, kami jual kopi susu literan buat anak kos")
		if out == nil {
			t.Fatalf("gagal memanggil model: %v", rejected)
		}
		t.Logf("reply: %s", strings.TrimSpace(out.Reply))
		keys := make([]string, 0, len(updates))
		for _, u := range updates {
			keys = append(keys, u.Key)
		}
		t.Logf("field terisi: %v", keys)
		if _, ok := acceptedValue(updates, "canonical_name"); !ok {
			t.Errorf("canonical_name tidak terisi (rejected=%v)", rejected)
		}
		// Satu pesan berisi dua informasi harus menghasilkan minimal dua field —
		// nama field persisnya bebas (produk bisa jatuh ke products/audience).
		if len(updates) < 2 {
			t.Errorf("hanya %d field terisi dari satu pesan berisi dua informasi (rejected=%v)", len(updates), rejected)
		}
	})

	// C. Koreksi di tengah jalan memperbarui nilai lama.
	t.Run("C_koreksi_nilai", func(t *testing.T) {
		states := map[string]FieldState{
			"canonical_name": {Status: StatusConfirmed, Source: SourceUser},
		}
		values := map[string]any{"canonical_name": "rabiyuk"}
		out, updates, rejected := liveTurn(d, ctx, "", states, values,
			"eh salah, nama brand-nya Rabiyuk.id")
		if out == nil {
			t.Fatalf("gagal memanggil model: %v", rejected)
		}
		t.Logf("reply: %s", strings.TrimSpace(out.Reply))
		val, ok := acceptedValue(updates, "canonical_name")
		if !ok {
			t.Fatalf("koreksi nama tidak disimpan (rejected=%v)", rejected)
		}
		if !strings.Contains(strings.ToLower(val), "rabiyuk.id") {
			t.Fatalf("nilai setelah koreksi tidak sesuai: %q", val)
		}
	})
}
