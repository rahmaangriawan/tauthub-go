package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool: deskripsi dan handler satu tool MCP. InputSchema berbentuk JSON
// Schema (dialek draft-07) — Hermes dan Claude memakainya untuk validasi
// argumen dan menampilkan form.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handle      func(ctx context.Context, s *Server, client *AgentClient, args json.RawMessage) (any, error)
}

// ---------- helper respons tool ----------

// textResult: membalas tool_call dengan konten teks JSON terstruktur.
// text berisi versi pretty untuk LLM; structured berisi payload mentah.
func textResult(structured any) (any, error) {
	buf, err := json.MarshalIndent(structured, "", "  ")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(buf)},
		},
		"structuredContent": structured,
	}, nil
}

// decodeArgs: unmarshal JSON argumen tool ke struct target. Bila kosong,
// target tidak disentuh.
func decodeArgs(raw json.RawMessage, target any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("argumen invalid: %w", err)
	}
	return nil
}
