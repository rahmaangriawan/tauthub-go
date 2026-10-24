package brief

import (
	"context"
	"fmt"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/domains/brief/summary"
)

// executeInput: satu giliran percakapan.
type executeInput struct {
	BrandID   string
	ProfileID string
	UserID    string
	Message   string
	UserName  string
	// OnReply != nil → coba jalur streaming.
	OnReply func(string)
	// OnStatus != nil → kabari klien soal progres singkat sebelum balasan
	// mengalir (mis. "Membaca tautan …") selagi AI menyiapkan giliran.
	OnStatus func(string)
}

// turnResult: yang dikembalikan ke frontend setelah satu giliran.
type turnResult struct {
	Output   *EngineOutput
	Accepted []FieldUpdate
	Rejected []string
	Phase    Phase
	Filled   int
	Total    int
	Streamed bool
}

func (t *turnResult) response() map[string]any {
	out := t.Output
	return map[string]any{
		"reply":                  out.Reply,
		"field_updates":          t.Accepted,
		"suggested_values":       out.SuggestedValues,
		"asked_field":            out.AskedField,
		"ready_for_confirmation": out.ReadyForConfirmation,
		"phase":                  string(t.Phase),
		"progress":               map[string]any{"filled": t.Filled, "total": t.Total},
		"streamed":               t.Streamed,
	}
}

// executeTurn menjalankan satu giliran penuh: simpan pesan user → rakit konteks
// → panggil model → validasi → tulis nilai → simpan balasan → catat run.
//
// Urutannya penting: pesan user disimpan LEBIH DULU supaya ia ikut jadi pesan
// terakhir di riwayat, dan blok state yang menempel padanya menggambarkan
// kondisi sebelum pesan itu diekstrak.
func (d *Deps) executeTurn(inCtx context.Context, in executeInput) (result *turnResult, err error) {
	// Buku besar giliran (pesan, nilai field, run, fase) memakai konteks yang
	// LEPAS dari siklus hidup request HTTP. Alasannya nyata: kalau koneksi
	// browser batal di tengah streaming (reload/HMR/navigasi), konteks request
	// ikut mati — dan tanpa pemisahan ini, tulisan "tutup run" ikut dibatalkan
	// secara sunyi sehingga run tercekik selamanya di status running.
	ctx := context.WithoutCancel(inCtx)

	cfg, err := d.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.configured() {
		return nil, errNotConfigured
	}

	if err := d.insertMessage(ctx, in.ProfileID, in.BrandID, "user", in.Message, in.UserID, nil); err != nil {
		return nil, err
	}

	// Tautan di pesan user dibaca lebih dulu, jadi isinya sudah masuk daftar
	// sumber saat konteks giliran ini dirakit. Kegagalan tidak membatalkan
	// giliran — AI tetap menjawab (tanpa bahan halaman itu).
	d.absorbMessageLinks(ctx, in.ProfileID, in.UserID, in.Message, in.OnStatus)

	t, err := d.loadTurn(ctx, in.ProfileID, in.BrandID, in.UserName)
	if err != nil {
		return nil, err
	}

	runID := d.startRun(ctx, in.BrandID, in.ProfileID, cfg.Model, in.Message)
	defer func() {
		// Pengaman terakhir: panic apa pun di jalur ini tidak boleh menyisakan
		// run menggantung tanpa status.
		if r := recover(); r != nil {
			d.finishRun(ctx, runID, "failed", 0, 0, 0, "", fmt.Sprintf("panic: %v", r))
			panic(r)
		}
	}()

	// Panggilan model tetap diikat konteks request + timeout konfigurasi:
	// tombol "Hentikan" dan timeout tetap bisa memutuskannya. Yang diselamatkan
	// dari pembatalan hanya pembukuan.
	run, err := d.runEngine(inCtx, t, in.OnReply)
	if err != nil {
		d.finishRun(ctx, runID, "failed", 0, 0, 0, "", err.Error())
		return nil, err
	}

	accepted, rejected := ValidateUpdates(t.Fields, t.States, run.Output.FieldUpdates)
	if err := d.applyUpdates(ctx, in.ProfileID, in.UserID, t.Fields, t.States, accepted); err != nil {
		d.finishRun(ctx, runID, "failed", run.Result.InputTokens, run.Result.OutputTokens,
			run.Result.DurationMS, "", err.Error())
		return nil, err
	}
	// Data entitas berubah → ringkasan Writer perlu dibuat ulang (async).
	if len(accepted) > 0 {
		_ = summary.Queue(ctx, d.Pool, in.ProfileID, in.BrandID, summary.KindEntity)
	}

	reply := strings.TrimSpace(run.Output.Reply)
	if reply == "" {
		reply = "Maaf, aku belum bisa merumuskan balasan. Bisa ulangi dengan kalimat lain?"
	}
	meta := map[string]any{"ready_for_confirmation": run.Output.ReadyForConfirmation}
	if run.Output.AskedField != nil && strings.TrimSpace(*run.Output.AskedField) != "" {
		meta["asked_field"] = *run.Output.AskedField
	}
	if err := d.insertMessage(ctx, in.ProfileID, in.BrandID, "assistant", reply, in.UserID, meta); err != nil {
		d.finishRun(ctx, runID, "failed", run.Result.InputTokens, run.Result.OutputTokens,
			run.Result.DurationMS, "", err.Error())
		return nil, err
	}

	// Ringkasan bergulir: dicatat per giliran (best-effort) supaya pesan yang
	// keluar dari jendela riwayat tetap terwakili di konteks giliran berikutnya.
	d.appendChatSummary(ctx, in.ProfileID, in.Message, accepted, run.Output)

	d.finishRun(ctx, runID, "done", run.Result.InputTokens, run.Result.OutputTokens,
		run.Result.DurationMS, reply, "")

	result = &turnResult{
		Output: run.Output, Accepted: accepted, Rejected: rejected,
		Streamed: run.Streamed,
	}
	if states, err := d.loadStates(ctx, in.ProfileID); err == nil {
		result.Filled, result.Total = Progress(t.Fields, states)
	}
	if phase, err := d.currentPhase(ctx, in.ProfileID, t.Fields); err == nil {
		result.Phase = phase
	} else {
		result.Phase = t.Phase
	}
	return result, nil
}

// currentPhase menghitung fase terkini dari state tersimpan. Dipakai setelah
// giliran selesai supaya frontend tahu fase berikutnya tanpa menghitung sendiri.
func (d *Deps) currentPhase(ctx context.Context, profileID string, fields []Field) (Phase, error) {
	states, err := d.loadStates(ctx, profileID)
	if err != nil {
		return "", err
	}
	sources, materialAt := d.loadSources(ctx, profileID)
	lastUserAt, hasUser := d.lastUserAt(ctx, profileID)
	awaiting, awaitingAt, _ := d.lastAssistantMeta(ctx, profileID)
	filled, total := Progress(fields, states)
	return DeterminePhase(PhaseInput{
		HasUserMessage:           hasUser,
		HasMaterial:              len(sources) > 0,
		MaterialAt:               materialAt,
		LastUserAt:               lastUserAt,
		AssistantAwaitingConfirm: awaiting,
		AwaitingConfirmAt:        awaitingAt,
		RequiredFilled:           filled,
		RequiredTotal:            total,
	}), nil
}
