package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Klien kecil OpenAI-compatible untuk Taut Brief, TERPISAH dari jalur
// taut_agent_configs. Dipakai karena Taut Brief punya tabel konfigurasi sendiri
// (taut_brief_config) dan butuh dua hal yang tidak ada di salinan callAgentLLM
// lain: pemancaran sebagian jawaban (onDelta) sekaligus JSON mode, plus
// ekstraktor nilai "reply" dari JSON yang belum selesai.
//
// Tiga salinan callAgentLLM yang sudah ada (platform, brandflow, clienttaut)
// sengaja tidak diubah supaya perubahan fitur ini tidak melebar; menyatukannya
// adalah pekerjaan tersendiri.

// ChatClient: satu konfigurasi panggilan.
type ChatClient struct {
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   *int
	Timeout     time.Duration

	// HTTPClient opsional; dipakai test untuk menyuntikkan transport palsu.
	HTTPClient *http.Client
}

type ChatMessage struct {
	Role    string
	Content string
}

type ChatResult struct {
	Content      string
	Model        string
	InputTokens  int
	OutputTokens int
	DurationMS   int64
	Streamed     bool
}

func (c *ChatClient) Configured() bool {
	return strings.TrimSpace(c.BaseURL) != "" &&
		strings.TrimSpace(c.APIKey) != "" &&
		strings.TrimSpace(c.Model) != ""
}

func (c *ChatClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (c *ChatClient) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 60 * time.Second
}

func (c *ChatClient) endpoint() string {
	return strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
}

func (c *ChatClient) payloadMessages(msgs []ChatMessage, jsonMode bool) map[string]any {
	items := make([]map[string]string, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, map[string]string{"role": m.Role, "content": m.Content})
	}
	body := map[string]any{
		"model":       c.Model,
		"temperature": c.Temperature,
		"messages":    items,
	}
	if c.MaxTokens != nil {
		body["max_tokens"] = *c.MaxTokens
	}
	if jsonMode {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	return body
}

func (c *ChatClient) newRequest(ctx context.Context, body map[string]any) (*http.Request, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	return req, nil
}

// CompleteMessages: satu panggilan tanpa streaming.
func (c *ChatClient) CompleteMessages(ctx context.Context, msgs []ChatMessage, jsonMode bool) (*ChatResult, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("Taut Brief belum dikonfigurasi (base URL, API key, atau model kosong).")
	}
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()
	req, err := c.newRequest(reqCtx, c.payloadMessages(msgs, jsonMode))
	if err != nil {
		return nil, fmt.Errorf("brief LLM: %w", err)
	}
	started := time.Now()
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("brief LLM: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Error any `json:"error"`
	}
	_ = json.Unmarshal(raw, &parsed)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("brief LLM: %s", httpErrorDetail(resp.StatusCode, parsed.Error))
	}
	content := ""
	if len(parsed.Choices) > 0 {
		content = parsed.Choices[0].Message.Content
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("brief LLM: respons kosong.")
	}
	return &ChatResult{
		Content: content, Model: c.Model,
		InputTokens: parsed.Usage.PromptTokens, OutputTokens: parsed.Usage.CompletionTokens,
		DurationMS: time.Since(started).Milliseconds(),
	}, nil
}

// CompleteStreamMessages: panggilan dengan stream:true. Bila gateway membalas
// JSON biasa (mengabaikan stream), seluruh isi dikirim lewat onDelta sekali —
// perilaku yang sama dengan callAgentLLMCommon di brandflow.
//
// Delta yang dibaca hanya choices[].delta.content. Model reasoning yang
// mengirim delta.reasoning / reasoning_content tidak ikut dipancarkan, sesuai
// kebijakan: proses berpikir internal tidak ditampilkan ke user.
func (c *ChatClient) CompleteStreamMessages(ctx context.Context, msgs []ChatMessage, jsonMode bool, onDelta func(string)) (*ChatResult, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("Taut Brief belum dikonfigurasi (base URL, API key, atau model kosong).")
	}
	body := c.payloadMessages(msgs, jsonMode)
	body["stream"] = true
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()
	req, err := c.newRequest(reqCtx, body)
	if err != nil {
		return nil, fmt.Errorf("brief LLM: %w", err)
	}
	started := time.Now()
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("brief LLM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var parsed struct {
			Error any `json:"error"`
		}
		_ = json.Unmarshal(raw, &parsed)
		return nil, fmt.Errorf("brief LLM: %s", httpErrorDetail(resp.StatusCode, parsed.Error))
	}

	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		var parsed struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		_ = json.Unmarshal(raw, &parsed)
		content := ""
		if len(parsed.Choices) > 0 {
			content = parsed.Choices[0].Message.Content
		}
		if strings.TrimSpace(content) == "" {
			return nil, fmt.Errorf("brief LLM: respons kosong.")
		}
		if onDelta != nil {
			onDelta(content)
		}
		return &ChatResult{
			Content: content, Model: c.Model,
			InputTokens: parsed.Usage.PromptTokens, OutputTokens: parsed.Usage.CompletionTokens,
			DurationMS: time.Since(started).Milliseconds(),
		}, nil
	}

	var acc strings.Builder
	usageIn, usageOut := 0, 0
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		if chunk.Usage.PromptTokens > 0 {
			usageIn = chunk.Usage.PromptTokens
		}
		if chunk.Usage.CompletionTokens > 0 {
			usageOut = chunk.Usage.CompletionTokens
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			delta := chunk.Choices[0].Delta.Content
			acc.WriteString(delta)
			if onDelta != nil {
				onDelta(delta)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("brief LLM: stream terputus: %w", err)
	}
	content := acc.String()
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("brief LLM: respons kosong.")
	}
	return &ChatResult{
		Content: content, Model: c.Model,
		InputTokens: usageIn, OutputTokens: usageOut,
		DurationMS: time.Since(started).Milliseconds(), Streamed: true,
	}, nil
}

func httpErrorDetail(status int, providerErr any) string {
	detail := fmt.Sprintf("HTTP %d", status)
	if providerErr == nil {
		return detail
	}
	if b, err := json.Marshal(providerErr); err == nil {
		msg := string(b)
		if len(msg) > 300 {
			msg = msg[:300]
		}
		detail = msg
	}
	return detail
}

// ---------- ekstraksi nilai string JSON secara bertahap ----------

// JSONStringFieldStream memancarkan isi sebuah field string tingkat atas
// seiring potongan JSON datang, tanpa menunggu dokumen selesai. Dipakai untuk
// menampilkan "reply" saat model masih menulis, sementara field_updates baru
// dibaca setelah JSON utuh.
//
// Cara kerja: teks mentah dikumpulkan, lalu isi field dipindai ulang dan hanya
// bagian yang belum pernah dipancarkan yang dikembalikan. Penyidikan ulang ini
// O(n) per potongan dengan n kecil (balasan chat), jadi jauh lebih sederhana
// daripada mesin status yang harus tahan potongan di tengah escape.
//
// Kontrak: field target ada di tingkat atas. Sesuai kontrak Brief Engine,
// "reply" adalah key pertama, sehingga pencarian kunci dari awal dokumen aman.
type JSONStringFieldStream struct {
	key     string
	raw     strings.Builder
	decoded string
	done    bool
}

func NewJSONStringFieldStream(field string) *JSONStringFieldStream {
	return &JSONStringFieldStream{key: field}
}

// Write memproses potongan baru dan mengembalikan teks yang sudah ter-decode
// pada potongan ini (bisa kosong). Setelah string penutup ditemukan, Write
// berikutnya tidak menghasilkan apa-apa.
func (s *JSONStringFieldStream) Write(chunk string) string {
	if s.done {
		return ""
	}
	s.raw.WriteString(chunk)
	value, complete := scanTopLevelJSONString(s.raw.String(), s.key)
	if complete {
		s.done = true
	}
	if len(value) <= len(s.decoded) {
		return ""
	}
	delta := value[len(s.decoded):]
	s.decoded = value
	return delta
}

// Done melaporkan apakah string penutup sudah terlihat.
func (s *JSONStringFieldStream) Done() bool { return s.done }

// scanTopLevelJSONString mengembalikan nilai string field dan apakah string
// penutup sudah ditemukan. Selama input masih terpotong, ia mengembalikan
// bagian yang sudah bisa didecode dengan complete=false.
func scanTopLevelJSONString(raw, key string) (string, bool) {
	needle := `"` + key + `"`
	idx := strings.Index(raw, needle)
	if idx < 0 {
		return "", false
	}
	i := idx + len(needle)
	i = skipJSONSpace(raw, i)
	if i >= len(raw) {
		return "", false
	}
	if raw[i] != ':' {
		return "", false
	}
	i = skipJSONSpace(raw, i+1)
	if i >= len(raw) || raw[i] != '"' {
		return "", false
	}
	i++

	var b strings.Builder
decode:
	for i < len(raw) {
		c := raw[i]
		if c == '"' {
			return b.String(), true
		}
		if c != '\\' {
			b.WriteByte(c)
			i++
			continue
		}
		if i+1 >= len(raw) {
			break // escape belum lengkap; tunggu potongan berikutnya
		}
		switch raw[i+1] {
		case '"':
			b.WriteByte('"')
			i += 2
		case '\\':
			b.WriteByte('\\')
			i += 2
		case '/':
			b.WriteByte('/')
			i += 2
		case 'b':
			b.WriteByte('\b')
			i += 2
		case 'f':
			b.WriteByte('\f')
			i += 2
		case 'n':
			b.WriteByte('\n')
			i += 2
		case 'r':
			b.WriteByte('\r')
			i += 2
		case 't':
			b.WriteByte('\t')
			i += 2
		case 'u':
			if i+6 > len(raw) {
				break decode // 4 digit hex belum lengkap
			}
			r, ok := decodeHex4(raw[i+2 : i+6])
			if !ok {
				return b.String(), false
			}
			// Pasangan surrogate (\uD83D\uDE00) harus digabung agar emoji utuh.
			if r >= 0xD800 && r <= 0xDBFF {
				if i+12 > len(raw) {
					break decode
				}
				if raw[i+6] == '\\' && raw[i+7] == 'u' {
					if low, okLow := decodeHex4(raw[i+8 : i+12]); okLow && low >= 0xDC00 && low <= 0xDFFF {
						b.WriteRune(rune(0x10000 + (r-0xD800)<<10 + (low - 0xDC00)))
						i += 12
						continue
					}
				}
			}
			b.WriteRune(r)
			i += 6
		default:
			b.WriteByte(raw[i+1])
			i += 2
		}
	}
	return b.String(), false
}

func skipJSONSpace(raw string, i int) int {
	for i < len(raw) {
		switch raw[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

func decodeHex4(s string) (rune, bool) {
	var n rune
	for i := 0; i < 4; i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			n = n<<4 | rune(c-'0')
		case c >= 'a' && c <= 'f':
			n = n<<4 | rune(c-'a'+10)
		case c >= 'A' && c <= 'F':
			n = n<<4 | rune(c-'A'+10)
		default:
			return 0, false
		}
	}
	return n, true
}
