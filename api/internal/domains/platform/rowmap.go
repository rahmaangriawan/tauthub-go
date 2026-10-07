package platform

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"github.com/rahmaangriawan/tauthub-go/internal/adminshared"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

// rowMap: padanan RowDataPacket app lama. Query yang memilih banyak kolom
// dikembalikan sebagai peta nama kolom → nilai supaya handler bisa membaca
// field dinamis tanpa struct khusus per query.
//
// `d.queryRows`/`d.queryRow` (types.go) sudah mengembalikan map; method di sini
// memberi akses bertipe + konversi null/value yang sama dengan app lama.
type rowMap map[string]any

func (m rowMap) raw(key string) any {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok {
		return nil
	}
	return v
}

func (m rowMap) has(key string) bool {
	v := m.raw(key)
	return v != nil
}

func (m rowMap) str(key string) (string, bool) {
	v := m.raw(key)
	if v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	case []byte:
		return string(t), true
	}
	return "", false
}

// strOr: parity `row.x ?? fallback`.
func (m rowMap) strOr(key, fallback string) string {
	if s, ok := m.str(key); ok {
		return s
	}
	return fallback
}

func (m rowMap) strPtr(key string) *string {
	if s, ok := m.str(key); ok {
		return &s
	}
	return nil
}

func (m rowMap) num(key string) (float64, bool) {
	v := m.raw(key)
	if v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int64:
		return float64(t), true
	case int32:
		return float64(t), true
	// Kolom smallint (da, pa, spam_score, ai_trust_score, credit_cost, …)
	// dikembalikan pgx sebagai int16 — tanpa cabang ini semua pembacaan
	// lewat rowMap menghasilkan nil/0 di JSON.
	case int16:
		return float64(t), true
	case int8:
		return float64(t), true
	case int:
		return float64(t), true
	case pgtype.Numeric:
		if !t.Valid {
			return 0, false
		}
		f8, err := t.Float64Value()
		if err != nil || !f8.Valid {
			return 0, false
		}
		return f8.Float64, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

// numOr: parity `Number(row.x) || 0`.
func (m rowMap) numOr(key string) float64 {
	f, _ := m.num(key)
	return f
}

func (m rowMap) numPtr(key string) any {
	f, ok := m.num(key)
	if !ok {
		return nil
	}
	return f
}

func (m rowMap) intOr(key string, fallback int) int {
	f, ok := m.num(key)
	if !ok {
		return fallback
	}
	return int(f)
}

func (m rowMap) intPtr(key string) any {
	f, ok := m.num(key)
	if !ok {
		return nil
	}
	return int(f)
}

func (m rowMap) boolOr(key string, fallback bool) bool {
	v := m.raw(key)
	if v == nil {
		return fallback
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return fallback
}

// timePtr: kolom timestamp → ISO milidetik atau nil (parity JSON Date).
func (m rowMap) timePtr(key string) any {
	v := m.raw(key)
	if v == nil {
		return nil
	}
	if t, ok := v.(time.Time); ok {
		return isoMilli(t)
	}
	return nil
}

// jsonArr: kolom JSON array → []string (parity parseJsonArray).
func (m rowMap) jsonArr(key string) []string {
	v := m.raw(key)
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return []string{}
		}
		if strings.HasPrefix(s, "[") {
			if arr, ok := decodeStringArray(s); ok {
				return arr
			}
		}
		return []string{s}
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	}
	return []string{}
}

// jsonField: nilai kolom JSONB apa adanya (map/slice) untuk diteruskan ke JSON.
func (m rowMap) jsonField(key string) any {
	v := m.raw(key)
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok {
		var parsed any
		if err := json.Unmarshal([]byte(s), &parsed); err == nil {
			return parsed
		}
		return s
	}
	return v
}

// sealV3ForEntity: seal v3 (t3:) per-entitas — parity sealSecretV3 app lama.
func (d *Deps) sealV3ForEntity(plain, entityID string) (string, error) {
	return crypto.SealV3(plain, d.Cfg.Secret, entityID)
}

// unsealV3ForEntity: buka sealed v3 per-entitas.
func (d *Deps) unsealV3ForEntity(sealed, entityID string) (string, error) {
	return crypto.UnsealV3(sealed, d.Cfg.Secret, entityID)
}

// getenv: pembaca env sederhana (dipakai modul settings sebagai fallback).
var getenv = func(key string) string { return os.Getenv(key) }

// ---------- helper config admin (misc_admin_config.go) ----------

var (
	reIPv4OrIPv6 = regexp.MustCompile(`^[0-9a-fA-F:.]{3,45}$`)
	reDomainOnly = regexp.MustCompile(`^[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

// isAbsURL: validasi URL absolut (parity z.string().url()).
func isAbsURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

// sealT2Entity: seal v2 per-entitas memakai CMS secret (parity sealSecretValue).
func (d *Deps) sealT2Entity(plain, entityID string) (string, error) {
	secret := d.Cfg.CmsEncryptionSecret
	if secret == "" {
		secret = d.Cfg.Secret
	}
	_ = entityID
	return crypto.Seal(plain, secret)
}

// aiRequireObject: body harus objek JSON (parity helper requireObject di app lama).
func aiRequireObject(r *http.Request) (map[string]any, error) {
	var parsed any
	if err := json.NewDecoder(r.Body).Decode(&parsed); err != nil {
		return nil, errUnprocessable("Invalid input: expected object")
	}
	obj, ok := parsed.(map[string]any)
	if !ok {
		return nil, errUnprocessable("Invalid input: expected object, received " + mopsJSONType(parsed))
	}
	return obj, nil
}

// ---------- helper endpoint publik (§7) ----------

// requireUser: user terautentikasi dari konteks; 401 bila tidak ada.
func requireUser(w http.ResponseWriter, r *http.Request) *httpx.AuthUser {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return nil
	}
	return user
}

func sha256HexPlatform(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func hmacSHA256HexPlatform(key []byte, message string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func constantTimeEqualHexPlatform(a, b string) bool {
	ab, err1 := hex.DecodeString(a)
	bb, err2 := hex.DecodeString(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return subtle.ConstantTimeCompare(ab, bb) == 1
}

func strconvParseInt(v string) (int, error) { return strconv.Atoi(strings.TrimSpace(v)) }

func marshalJSONAny(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func adminsharedErrTooManyWith(msg string) error {
	return adminshared.Err(http.StatusTooManyRequests, msg)
}

func slogWarn(msg string, err error) { slog.Warn(msg, "err", err) }
