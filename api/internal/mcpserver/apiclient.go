// Package mcpserver menjalankan MCP (Model Context Protocol) untuk admin
// dashboard Tauthub. Server ini dipanggil Hermes Agent (Nous Research) dan
// Claude Code; keduanya memakai transport Streamable HTTP dengan Bearer token
// per-klien yang dicocokkan dengan tabel agent_clients.
//
// Pola arsitekturnya: paket ini TIDAK menyentuh database bisnis langsung. Ia
// berbicara ke /v1/platform/* via loopback, membawa MCP_SERVICE_KEY sebagai
// Bearer dan X-Act-As-User sebagai identitas admin yang bersangkutan. Dengan
// begitu seluruh adminGuard, izin, rate limit, dan audit tetap berlaku.
package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// APIClient: wrapper HTTP ke API internal. Mode baca memakai read_as_user_id
// dari agent_clients; mode tulis memakai actAsUser admin yang menyetujui.
type APIClient struct {
	BaseURL     string // mis. http://127.0.0.1:8080
	ServiceKey  string // service key statis; kosong bila dinamis via KeyProvider
	KeyProvider func() string
	HTTP        *http.Client
}

func NewAPIClient(baseURL, serviceKey string) *APIClient {
	return &APIClient{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		ServiceKey: serviceKey,
		HTTP:       &http.Client{Timeout: 60 * time.Second},
	}
}

// currentKey: pilih KeyProvider bila ada, jatuh ke ServiceKey.
func (c *APIClient) currentKey() string {
	if c.KeyProvider != nil {
		if k := c.KeyProvider(); k != "" {
			return k
		}
	}
	return c.ServiceKey
}

// Call: memanggil endpoint API dengan jalur service-key. actAsUser WAJIB diisi
// (user_id admin — biasanya agent_clients.read_as_user_id untuk baca, atau
// user id admin yang menekan "Setujui" untuk tulis). agentCtx opsional, hanya
// untuk tulis: menandai aksi di audit_logs.detail.
func (c *APIClient) Call(ctx context.Context, method, path string, query url.Values, body any, actAsUser string, agentCtx *AgentHeader) (*APIResponse, error) {
	if actAsUser == "" {
		return nil, fmt.Errorf("actAsUser kosong")
	}
	u := c.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return nil, err
	}
	key := c.currentKey()
	if key == "" {
		return nil, fmt.Errorf("service key MCP belum di-set — buka /admin/konfigurasi/mcp untuk mengisi")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("X-Act-As-User", actAsUser)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if agentCtx != nil {
		if agentCtx.ClientID != "" {
			req.Header.Set("X-Agent-Client-ID", agentCtx.ClientID)
		}
		if agentCtx.ActionID != "" {
			req.Header.Set("X-Agent-Action-ID", agentCtx.ActionID)
		}
		if agentCtx.ActionCode != "" {
			req.Header.Set("X-Agent-Action-Code", agentCtx.ActionCode)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	return &APIResponse{Status: resp.StatusCode, Raw: raw}, nil
}

// AgentHeader: konteks agent yang dikirim bersama request tulis supaya audit
// mencatat sumbernya.
type AgentHeader struct {
	ClientID   string
	ActionID   string
	ActionCode string
}

// APIResponse: respons mentah dari API internal.
type APIResponse struct {
	Status int
	Raw    []byte
}

// JSON: parse body sebagai map (atau tipe lain) dan kembalikan error bila
// status non-2xx.
func (r *APIResponse) JSON(target any) error {
	if r.Status < 200 || r.Status >= 300 {
		// Coba ekstrak pesan error dari envelope API.
		var env struct {
			Error  string `json:"error"`
			Detail string `json:"detail"`
		}
		_ = json.Unmarshal(r.Raw, &env)
		msg := env.Error
		if msg == "" {
			msg = env.Detail
		}
		if msg == "" {
			msg = strings.TrimSpace(string(r.Raw))
		}
		if len(msg) > 400 {
			msg = msg[:400] + "…"
		}
		return &APIError{Status: r.Status, Message: msg}
	}
	if target == nil {
		return nil
	}
	return json.Unmarshal(r.Raw, target)
}

// APIError: pembungkus kesalahan non-2xx supaya tool bisa menanggapi dengan
// pesan yang jelas, bukan panic "unexpected body".
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api %d: %s", e.Status, e.Message)
}
