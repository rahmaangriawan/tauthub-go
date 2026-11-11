package mcpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/mcpconfig"
)

// Server: HTTP server MCP Streamable HTTP. Satu endpoint POST /mcp menerima
// request JSON-RPC 2.0. GET /mcp mengembalikan 405 (kita tidak memakai SSE
// server-initiated untuk saat ini — semua respons sinkron).
//
// Protokol yang diimplementasikan (subset minimum yang dibutuhkan Hermes dan
// Claude Code):
//   - initialize            → capabilities, serverInfo
//   - notifications/initialized (notifikasi, tanpa respons)
//   - tools/list            → daftar tool
//   - tools/call            → eksekusi tool, mengembalikan content[]
//   - ping                  → {}
//
// Autentikasi klien: Bearer token di header Authorization, dicocokkan dengan
// token_hash di tabel agent_clients (SHA-256 hex). Baca memakai
// read_as_user_id sebagai actAsUser di API.
type Server struct {
	Cfg       *config.Config
	Pool      *pgxpool.Pool
	MCPConfig *mcpconfig.Store // sumber service key, token bot, allowlist, dst.
	API       *APIClient
	Telegram  *Telegram // nil bila bot token kosong

	tools map[string]*Tool
	mu    sync.RWMutex

	// clientCache memetakan token hash → agent_clients row (TTL 60s). Ini
	// mengurangi roundtrip DB untuk permintaan beruntun dari Hermes.
	clientCache    map[string]cachedClient
	clientCacheTTL time.Duration
	clientMu       sync.Mutex
}

type cachedClient struct {
	client    *AgentClient
	expiresAt time.Time
}

// AgentClient: baris agent_clients yang sudah tervalidasi.
type AgentClient struct {
	ID            string
	Name          string
	ReadAsUserID  string
	LastUsedAt    *time.Time
}

func New(cfg *config.Config, pool *pgxpool.Pool, store *mcpconfig.Store) *Server {
	s := &Server{
		Cfg:            cfg,
		Pool:           pool,
		MCPConfig:      store,
		API:            newAPIClientFromStore(cfg.APIInternalURL, store),
		tools:          map[string]*Tool{},
		clientCache:    map[string]cachedClient{},
		clientCacheTTL: 60 * time.Second,
	}
	s.registerReadTools()
	s.registerWriteTools()
	return s
}

// newAPIClientFromStore: APIClient yang mengambil service key terbaru per
// request (bukan di-freeze saat boot), supaya rotasi berlaku di sini pula.
func newAPIClientFromStore(baseURL string, store *mcpconfig.Store) *APIClient {
	c := NewAPIClient(baseURL, "")
	c.KeyProvider = store.ServiceKey
	return c
}

// AttachTelegram menyiapkan bot Persetujuan. Dipanggil cmd/mcp setelah
// server dibuat; nil bila token kosong.
func (s *Server) AttachTelegram(t *Telegram) {
	s.Telegram = t
}

// Register menambahkan tool ke registry. Dipanggil saat boot server.
func (s *Server) Register(t *Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tools[t.Name]; exists {
		slog.Warn("[mcp] tool terdaftar ganda, ditimpa", "name", t.Name)
	}
	s.tools[t.Name] = t
}

// Handler: http.Handler untuk endpoint /mcp.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handleMCP)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	return mux
}

// ---------- autentikasi klien ----------

func (s *Server) authenticate(r *http.Request) (*AgentClient, error) {
	raw := r.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		return nil, errors.New("missing bearer")
	}
	token := strings.TrimSpace(raw[len("Bearer "):])
	if token == "" {
		return nil, errors.New("empty token")
	}
	sum := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(sum[:])

	s.clientMu.Lock()
	if hit, ok := s.clientCache[hashHex]; ok && hit.expiresAt.After(time.Now()) {
		client := hit.client
		s.clientMu.Unlock()
		return client, nil
	}
	s.clientMu.Unlock()

	var ac AgentClient
	err := s.Pool.QueryRow(r.Context(),
		`SELECT id, name, read_as_user_id FROM agent_clients
		 WHERE token_hash = $1 AND revoked_at IS NULL LIMIT 1`, hashHex,
	).Scan(&ac.ID, &ac.Name, &ac.ReadAsUserID)
	if err != nil {
		return nil, errors.New("unknown token")
	}
	// Catat last_used_at best-effort.
	go func(id string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = s.Pool.Exec(ctx, `UPDATE agent_clients SET last_used_at = NOW() WHERE id = $1`, id)
	}(ac.ID)

	s.clientMu.Lock()
	s.clientCache[hashHex] = cachedClient{client: &ac, expiresAt: time.Now().Add(s.clientCacheTTL)}
	s.clientMu.Unlock()
	return &ac, nil
}

// ---------- JSON-RPC ----------

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		// GET /mcp dipakai beberapa klien untuk membuka saluran SSE; kita
		// kembalikan 405 supaya mereka jatuh ke mode POST-only.
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	client, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCError(w, nil, -32700, "parse error", nil)
		return
	}
	if req.JSONRPC != "2.0" {
		writeRPCError(w, req.ID, -32600, "jsonrpc must be 2.0", nil)
		return
	}

	// Notifikasi (tanpa id) tidak butuh respons.
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"

	ctx := r.Context()
	switch req.Method {
	case "initialize":
		writeRPCResult(w, req.ID, s.handleInitialize(req.Params))
	case "notifications/initialized":
		// Tidak ada balasan.
		w.WriteHeader(http.StatusAccepted)
	case "ping":
		if isNotification {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeRPCResult(w, req.ID, map[string]any{})
	case "tools/list":
		writeRPCResult(w, req.ID, s.handleToolsList())
	case "tools/call":
		result, rpcErr := s.handleToolsCall(ctx, client, req.Params)
		if rpcErr != nil {
			writeRPCError(w, req.ID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
			return
		}
		writeRPCResult(w, req.ID, result)
	default:
		if isNotification {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeRPCError(w, req.ID, -32601, "method not found: "+req.Method, nil)
	}
}

func writeRPCResult(w http.ResponseWriter, id json.RawMessage, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func writeRPCError(w http.ResponseWriter, id json.RawMessage, code int, message string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors tetap 200 di body
	_ = json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0", ID: id,
		Error: &jsonRPCError{Code: code, Message: message, Data: data},
	})
}

// ---------- initialize ----------

func (s *Server) handleInitialize(_ json.RawMessage) any {
	return map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities": map[string]any{
			"tools": map[string]any{"listChanged": false},
		},
		"serverInfo": map[string]any{
			"name":    "tauthub-admin",
			"version": s.Cfg.Version,
		},
	}
}

// ---------- tools/list ----------

func (s *Server) handleToolsList() any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]map[string]any, 0, len(s.tools))
	for _, t := range s.tools {
		out = append(out, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		})
	}
	return map[string]any{"tools": out}
}

// ---------- tools/call ----------

type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) handleToolsCall(ctx context.Context, client *AgentClient, raw json.RawMessage) (any, *jsonRPCError) {
	var p toolsCallParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: "invalid params"}
	}
	s.mu.RLock()
	tool, ok := s.tools[p.Name]
	s.mu.RUnlock()
	if !ok {
		return nil, &jsonRPCError{Code: -32601, Message: "unknown tool: " + p.Name}
	}
	args := p.Arguments
	if len(args) == 0 {
		args = []byte(`{}`)
	}
	result, err := tool.Handle(ctx, s, client, args)
	if err != nil {
		slog.Warn("[mcp] tool error", "tool", p.Name, "err", err)
		// Tool error dilaporkan sebagai isError di content, bukan error RPC.
		// Dengan begitu LLM bisa melihat pesan dan memperbaiki pemanggilan.
		return map[string]any{
			"isError": true,
			"content": []map[string]any{
				{"type": "text", "text": fmt.Sprintf("Tool %s gagal: %s", p.Name, err.Error())},
			},
		}, nil
	}
	return result, nil
}
