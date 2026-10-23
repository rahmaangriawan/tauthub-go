package summary

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/config"
)

// Generate menjalankan satu agent lalu menyimpan hasilnya. Status selalu
// ditinggalkan dalam keadaan terminal (`ready`/`idle`/`failed`) supaya UI tidak
// berputar selamanya bila model bermasalah.
//
// force=false dipakai job otomatis: bila bahan brief tidak berubah sejak
// ringkasan terakhir, model TIDAK dipanggil sama sekali (hemat token). force=true
// dipakai tombol manual supaya admin tetap bisa memaksa ringkasan baru setelah
// mengganti prompt/model.
func Generate(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, profileID, brandID string, kind Kind, force bool) error {
	agent := AgentName(kind)
	agentCfg := LoadAgentConfig(ctx, pool, cfg.CmsEncryptionSecret, agent)

	// Bahan dihitung lebih dulu — inilah yang menentukan perlu-tidaknya
	// memanggil model.
	src, err := BuildSource(ctx, pool, profileID, kind)
	if err != nil {
		_ = setStatus(ctx, pool, profileID, kind, "failed", err.Error())
		return err
	}
	storedStatus, storedHash, storedContent := loadStored(ctx, pool, profileID, kind)

	if src.Text == "" {
		// Tidak ada bahan: jangan buang token untuk meminta model menulis
		// "tidak ada data". Ringkasan lama (bila ada) ikut dikosongkan.
		_, err := pool.Exec(ctx, `
			UPDATE profiler_brief_summaries
			SET status = 'idle', content = NULL, source_chars = 0, source_hash = $3,
			    error = NULL, generated_at = NULL, updated_at = now()
			WHERE profile_id = $1 AND kind = $2`,
			profileID, string(kind), src.Hash)
		return err
	}
	if !force && storedStatus == "ready" && storedHash == src.Hash && strings.TrimSpace(storedContent) != "" {
		// Bahan identik dengan yang sudah diringkas — cukup kembalikan status.
		return setStatus(ctx, pool, profileID, kind, "ready", "")
	}

	if err := setStatus(ctx, pool, profileID, kind, "running", ""); err != nil {
		return err
	}

	fail := func(err error) error {
		_ = setStatus(ctx, pool, profileID, kind, "failed", err.Error())
		logRun(ctx, pool, brandID, profileID, agent, agentCfg.Model, "", 0, 0, 0, err)
		return err
	}

	if !agentCfg.Enabled {
		return fail(fmt.Errorf("agent %s dinonaktifkan di admin", agent))
	}
	if !agentCfg.Configured() {
		return fail(errors.New("agent ringkasan belum dikonfigurasi (base URL, API key, atau model kosong)"))
	}
	prompt := LoadActivePrompt(ctx, pool, agent)
	if strings.TrimSpace(prompt) == "" {
		return fail(fmt.Errorf("prompt agent %s belum diatur", agent))
	}

	material := src.Text
	if material == "" {
		material = "(tidak ada bahan pada brief ini)"
	}

	client := &ai.ChatClient{
		BaseURL:     agentCfg.BaseURL,
		APIKey:      agentCfg.APIKey,
		Model:       agentCfg.Model,
		Temperature: agentCfg.Temperature,
		MaxTokens:   agentCfg.MaxTokens,
		Timeout:     time.Duration(agentCfg.TimeoutMS) * time.Millisecond,
	}
	started := time.Now()
	res, err := client.CompleteMessages(ctx, []ai.ChatMessage{
		{Role: "system", Content: prompt},
		{Role: "user", Content: "Bahan brief yang harus diringkas:\n\n" + material},
	}, false)
	if err != nil {
		return fail(err)
	}
	content := strings.TrimSpace(res.Content)
	if content == "" {
		return fail(errors.New("model mengembalikan ringkasan kosong"))
	}

	if _, err := pool.Exec(ctx, `
		UPDATE profiler_brief_summaries
		SET status = 'ready', content = $3, source_chars = $4, source_hash = $5,
		    model = $6, token_input = $7, token_output = $8, error = NULL,
		    generated_at = now(), updated_at = now()
		WHERE profile_id = $1 AND kind = $2`,
		profileID, string(kind), content, src.Chars, src.Hash,
		agentCfg.Model, res.InputTokens, res.OutputTokens); err != nil {
		return err
	}

	logRun(ctx, pool, brandID, profileID, agent, agentCfg.Model,
		src.Text, res.InputTokens, res.OutputTokens, int(time.Since(started).Milliseconds()), nil)
	return nil
}

// loadStored: keadaan ringkasan terakhir (kosong bila belum pernah dibuat).
func loadStored(ctx context.Context, pool *pgxpool.Pool, profileID string, kind Kind) (status, hash, content string) {
	_ = pool.QueryRow(ctx, `
		SELECT status, COALESCE(source_hash, ''), COALESCE(content, '')
		FROM profiler_brief_summaries WHERE profile_id = $1 AND kind = $2`,
		profileID, string(kind)).Scan(&status, &hash, &content)
	return status, hash, content
}

func setStatus(ctx context.Context, pool *pgxpool.Pool, profileID string, kind Kind, status, errMsg string) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO profiler_brief_summaries (profile_id, kind, status, error, updated_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), now())
		ON CONFLICT (profile_id, kind) DO UPDATE
		SET status = EXCLUDED.status, error = EXCLUDED.error, updated_at = now()`,
		profileID, string(kind), status, errMsg)
	return err
}

// logRun mencatat pemakaian ke taut_brief_runs dengan nama agent, sehingga
// ikut tampil di tab Log admin bersama percakapan Copilot.
func logRun(ctx context.Context, pool *pgxpool.Pool, brandID, profileID, agent, model, input string, tokIn, tokOut, durationMS int, runErr error) {
	if strings.TrimSpace(brandID) == "" {
		return
	}
	status := "done"
	errText := ""
	if runErr != nil {
		status = "failed"
		errText = runErr.Error()
	}
	_, _ = pool.Exec(ctx, `
		INSERT INTO taut_brief_runs
		  (id, brand_id, profile_id, agent, status, model, token_input, token_output,
		   duration_ms, error, input_preview, output_preview, created_at, finished_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''), $11, $12, now(), now())`,
		uuid.NewString(), brandID, profileID, agent, status, model,
		nullInt(tokIn), nullInt(tokOut), durationMS, errText,
		clip(input, 2000), clip(errText, 2000))
}

func nullInt(v int) any {
	if v <= 0 {
		return nil
	}
	return v
}
