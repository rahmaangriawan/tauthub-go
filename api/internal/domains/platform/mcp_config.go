// Endpoint konfigurasi MCP admin (service key, bot Telegram, allowlist,
// kill switch tulis, batas penyesuaian saldo). Super admin only — kolom
// berisi secret berharga. Setelah PUT, pg_notify('mcp_config_changed')
// sehingga mcpconfig.Store di cmd/api (Authenticator.ServiceKeyProvider)
// dan cmd/mcp me-reload tanpa restart.
package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

// McpConfigGet: GET /v1/platform/mcp-config
// Tidak mengembalikan nilai asli secret — hanya hint (4 karakter terakhir)
// dan flag "service_key_set" / "telegram_bot_token_set" supaya UI dapat
// membedakan "belum di-set" vs "sudah tersedia, isi tersembunyi".
func (d *Deps) McpConfigGet(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	row, err := d.queryRow(r.Context(), `
		SELECT COALESCE(sealed_service_key, '') AS sealed_service_key,
		       COALESCE(service_key_hint, '')  AS service_key_hint,
		       COALESCE(sealed_telegram_bot_token, '') AS sealed_tg,
		       COALESCE(telegram_bot_hint, '') AS telegram_bot_hint,
		       COALESCE(telegram_chat_ids, '{}'::bigint[]) AS telegram_chat_ids,
		       writes_enabled, max_credit_adjust_tc, updated_at
		FROM mcp_config WHERE id = 'default' LIMIT 1`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeJSON(w, http.StatusOK, mcpDefaultConfigJSON())
		return
	}
	nr := ptRow(ptNormalizeMap(row))
	writeJSON(w, http.StatusOK, map[string]any{
		"service_key_set":       strings.TrimSpace(nr.strOr("sealed_service_key", "")) != "",
		"service_key_hint":      nr.strOr("service_key_hint", ""),
		"telegram_bot_token_set": strings.TrimSpace(nr.strOr("sealed_tg", "")) != "",
		"telegram_bot_hint":     nr.strOr("telegram_bot_hint", ""),
		"telegram_chat_ids":     nr["telegram_chat_ids"],
		"writes_enabled":        nr.boolOr("writes_enabled", true),
		"max_credit_adjust_tc":  nr.intOr("max_credit_adjust_tc", 50_000_000),
		"updated_at":            nr.timePtr("updated_at"),
	})
}

func mcpDefaultConfigJSON() map[string]any {
	return map[string]any{
		"service_key_set": false, "service_key_hint": "",
		"telegram_bot_token_set": false, "telegram_bot_hint": "",
		"telegram_chat_ids": []int64{},
		"writes_enabled":    true, "max_credit_adjust_tc": 50_000_000,
	}
}

// McpConfigPut: PUT /v1/platform/mcp-config. Semua field opsional — hanya
// yang dikirim yang di-update. Untuk mengosongkan service key / token,
// kirim string kosong secara eksplisit.
func (d *Deps) McpConfigPut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)

	patches := []string{}
	args := []any{}
	next := func(v any) string {
		args = append(args, v)
		return "$" + ptItoa(len(args))
	}

	if raw, present := body["service_key"]; present {
		s := ""
		if raw != nil {
			sv, isStr := raw.(string)
			if !isStr {
				writeErr(w, d, errUnprocessable("service_key harus string"))
				return
			}
			s = strings.TrimSpace(sv)
		}
		if s == "" {
			patches = append(patches, "sealed_service_key = NULL", "service_key_hint = NULL")
		} else {
			if len(s) < 32 {
				writeErr(w, d, errUnprocessable("service_key minimal 32 karakter"))
				return
			}
			sealed, err := mcpSeal(d, s)
			if err != nil {
				writeErr(w, d, err)
				return
			}
			patches = append(patches, "sealed_service_key = "+next(sealed), "service_key_hint = "+next(lastFour(s)))
		}
	}

	if raw, present := body["telegram_bot_token"]; present {
		s := ""
		if raw != nil {
			sv, isStr := raw.(string)
			if !isStr {
				writeErr(w, d, errUnprocessable("telegram_bot_token harus string"))
				return
			}
			s = strings.TrimSpace(sv)
		}
		if s == "" {
			patches = append(patches, "sealed_telegram_bot_token = NULL", "telegram_bot_hint = NULL")
		} else {
			sealed, err := mcpSeal(d, s)
			if err != nil {
				writeErr(w, d, err)
				return
			}
			patches = append(patches, "sealed_telegram_bot_token = "+next(sealed), "telegram_bot_hint = "+next(lastFour(s)))
		}
	}

	if raw, present := body["telegram_chat_ids"]; present {
		ids, err := mcpParseChatIDs(raw)
		if err != nil {
			writeErr(w, d, errUnprocessable(err.Error()))
			return
		}
		patches = append(patches, "telegram_chat_ids = "+next(ids))
	}

	if raw, present := body["writes_enabled"]; present {
		b, isBool := raw.(bool)
		if !isBool {
			writeErr(w, d, errUnprocessable("writes_enabled harus boolean"))
			return
		}
		patches = append(patches, "writes_enabled = "+next(b))
	}

	if raw, present := body["max_credit_adjust_tc"]; present {
		var v int64
		switch n := raw.(type) {
		case float64:
			v = int64(n)
		case int:
			v = int64(n)
		case int64:
			v = n
		case string:
			parsed, perr := mcpParseInt64(n)
			if perr != nil {
				writeErr(w, d, errUnprocessable("max_credit_adjust_tc harus angka"))
				return
			}
			v = parsed
		default:
			writeErr(w, d, errUnprocessable("max_credit_adjust_tc harus angka"))
			return
		}
		if v < 0 || v > 1_000_000_000_000 {
			writeErr(w, d, errUnprocessable("max_credit_adjust_tc di luar jangkauan yang masuk akal"))
			return
		}
		patches = append(patches, "max_credit_adjust_tc = "+next(v))
	}

	if len(patches) == 0 {
		writeErr(w, d, errUnprocessable("Tidak ada field yang diubah."))
		return
	}
	patches = append(patches, "updated_by = "+next(user.ID), "updated_at = NOW()")

	sql := "UPDATE mcp_config SET " + strings.Join(patches, ", ") + " WHERE id = 'default'"
	if err := d.exec(r.Context(), sql, args...); err != nil {
		writeErr(w, d, err)
		return
	}
	// Broadcast reload ke semua proses yang LISTEN (cmd/api dan cmd/mcp).
	if _, err := d.Pool.Exec(r.Context(), "SELECT pg_notify('mcp_config_changed', '')"); err != nil {
		// Jangan gagalkan permintaan; perubahan toh akan terlihat setelah
		// polling / restart. Admin UI melaporkan sukses update.
		d.auditWith(r.Context(), user.ID, "platform.mcp_config.notify_failed", "mcp_config", "default",
			map[string]any{"err": err.Error()})
	}
	d.audit(r.Context(), user.ID, "platform.mcp_config.updated", "mcp_config", "default")

	// Balikkan representasi aman (sama dengan GET).
	d.McpConfigGet(w, r)
}

// ---------- helper ----------

func mcpSeal(d *Deps, plain string) (string, error) {
	secret := d.Cfg.CmsEncryptionSecret
	if secret == "" {
		secret = d.Cfg.Secret
	}
	sealed, err := crypto.Seal(plain, secret)
	if err != nil {
		return "", errors.New("gagal menyegel nilai")
	}
	return sealed, nil
}

func lastFour(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("•", len(s))
	}
	return "••••" + s[len(s)-4:]
}

func mcpParseChatIDs(raw any) ([]int64, error) {
	arr, ok := raw.([]any)
	if !ok {
		return nil, errors.New("telegram_chat_ids harus array")
	}
	out := make([]int64, 0, len(arr))
	for _, item := range arr {
		switch v := item.(type) {
		case float64:
			if v != float64(int64(v)) {
				return nil, errors.New("telegram_chat_ids: entri harus integer")
			}
			out = append(out, int64(v))
		case string:
			n, err := mcpParseInt64(v)
			if err != nil {
				return nil, errors.New("telegram_chat_ids: entri harus integer")
			}
			out = append(out, n)
		default:
			return nil, errors.New("telegram_chat_ids: entri harus integer")
		}
	}
	return out, nil
}

func mcpParseInt64(s string) (int64, error) {
	// strconv.ParseInt via json.Unmarshal bila format valid.
	var v int64
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &v); err != nil {
		return 0, err
	}
	return v, nil
}

// McpConfigNotifyReload dipanggil startup cmd/api untuk sekali-sinyal
// supaya proses lain refresh (dipakai setelah migrasi/restart, bukan jalur
// umum).
func (d *Deps) McpConfigNotifyReload(ctx context.Context) {
	_, _ = d.Pool.Exec(ctx, "SELECT pg_notify('mcp_config_changed', '')")
}
