package summary

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

const briefConfigEntityID = "taut_brief_config"

// AgentConfig: koneksi & parameter model untuk satu agent ringkasan.
//
// Kolom yang kosong di taut_brief_agent_configs mewarisi taut_brief_config,
// sehingga fitur langsung jalan setelah migrasi tanpa admin mengisi API key
// ulang. Agent tetap bisa punya model/key sendiri (mis. model murah).
type AgentConfig struct {
	Agent       string
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   *int
	TimeoutMS   int
	Enabled     bool
	Inherited   bool
}

func (c *AgentConfig) Configured() bool {
	return c != nil && c.Enabled &&
		strings.TrimSpace(c.BaseURL) != "" &&
		strings.TrimSpace(c.APIKey) != "" &&
		strings.TrimSpace(c.Model) != ""
}

func agentEntityID(agent string) string { return "taut_brief_agent_" + agent }

// LoadAgentConfig membaca config agent; bagian kosong diisi dari config Taut
// Brief. Tidak pernah error untuk tabel yang belum ada — dianggap belum
// dikonfigurasi.
func LoadAgentConfig(ctx context.Context, pool *pgxpool.Pool, appSecret, agent string) *AgentConfig {
	cfg := &AgentConfig{Agent: agent, Temperature: 0.2, TimeoutMS: 120000, Enabled: true}

	var (
		baseURL, sealed, model *string
		temperature            *float64
		maxTokens              *int
		timeoutMS              *int
		enabled                *bool
	)
	err := pool.QueryRow(ctx, `
		SELECT base_url, sealed_api_key, model, temperature, max_tokens, timeout_ms, enabled
		FROM taut_brief_agent_configs WHERE agent = $1`, agent).
		Scan(&baseURL, &sealed, &model, &temperature, &maxTokens, &timeoutMS, &enabled)
	if err == nil {
		cfg.BaseURL = strings.TrimRight(deref(baseURL), "/")
		cfg.Model = deref(model)
		cfg.MaxTokens = maxTokens
		if temperature != nil {
			cfg.Temperature = *temperature
		}
		if timeoutMS != nil && *timeoutMS > 0 {
			cfg.TimeoutMS = *timeoutMS
		}
		if enabled != nil {
			cfg.Enabled = *enabled
		}
		if sealed != nil && *sealed != "" {
			if plain, uErr := crypto.UnsealV3(*sealed, appSecret, agentEntityID(agent)); uErr == nil {
				cfg.APIKey = plain
			}
		}
	}

	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.Model == "" {
		ownBase := cfg.BaseURL != ""
		var (
			bBase, bSealed, bModel *string
		)
		if err := pool.QueryRow(ctx,
			`SELECT base_url, sealed_api_key, model FROM taut_brief_config WHERE id = 'default'`).
			Scan(&bBase, &bSealed, &bModel); err == nil {
			briefBase := strings.TrimRight(deref(bBase), "/")
			inherited := false
			if cfg.BaseURL == "" {
				cfg.BaseURL = briefBase
				inherited = true
			}
			if cfg.Model == "" {
				cfg.Model = deref(bModel)
				inherited = true
			}
			// Kunci hanya diwarisi bila koneksinya memang koneksi Brief. Agent
			// yang punya base URL sendiri wajib punya kunci sendiri — jangan
			// pernah mengirim kunci Brief ke host lain yang dipilih admin.
			if cfg.APIKey == "" && !ownBase && cfg.BaseURL == briefBase && bSealed != nil && *bSealed != "" {
				if plain, uErr := crypto.UnsealV3(*bSealed, appSecret, briefConfigEntityID); uErr == nil {
					cfg.APIKey = plain
					inherited = true
				}
			}
			cfg.Inherited = inherited
		}
	}
	return cfg
}

// LoadActivePrompt: prompt aktif terbaru untuk agent. "" bila belum ada.
func LoadActivePrompt(ctx context.Context, pool *pgxpool.Pool, agent string) string {
	var prompt *string
	if err := pool.QueryRow(ctx, `
		SELECT prompt FROM taut_brief_agent_prompt_versions
		WHERE agent = $1 AND status = 'active'
		ORDER BY version DESC LIMIT 1`, agent).Scan(&prompt); err != nil {
		return ""
	}
	return deref(prompt)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}
