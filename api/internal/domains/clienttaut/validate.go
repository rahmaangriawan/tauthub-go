package clienttaut

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

// Validasi parity zod v4 (pesan error default English + pesan kustom Indonesia
// dari skema di app lama). errorResponse lama memakai `error.issues[0].message`,
// jadi hanya issue PERTAMA yang muncul — urutan pemeriksaan di sini mengikuti
// urutan field di skema zod masing-masing route.

func zodInvalidType(expected, received string) *tautError {
	return errUnprocessable("Invalid input: expected " + expected + ", received " + received)
}

func zodTooSmallString(min int) *tautError {
	return errUnprocessable("Too small: expected string to have >=" + itoa(min) + " characters")
}

func zodTooBigString(max int) *tautError {
	return errUnprocessable("Too big: expected string to have <=" + itoa(max) + " characters")
}

func zodTooBigArray(max int) *tautError {
	return errUnprocessable("Too big: expected array to have <=" + itoa(max) + " items")
}

func zodTooSmallArray(min int) *tautError {
	return errUnprocessable("Too small: expected array to have >=" + itoa(min) + " items")
}

func zodTooSmallNumber(min string) *tautError {
	return errUnprocessable("Too small: expected number to be >=" + min)
}

func zodTooSmallNumberExclusive(min string) *tautError {
	return errUnprocessable("Too small: expected number to be >" + min)
}

func zodTooBigNumber(max string) *tautError {
	return errUnprocessable("Too big: expected number to be <=" + max)
}

func zodInvalidOption(values ...string) *tautError {
	if len(values) == 1 {
		return errUnprocessable("Invalid input: expected " + jsonStringPrimitive(values[0]))
	}
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = jsonStringPrimitive(v)
	}
	return errUnprocessable("Invalid option: expected one of " + strings.Join(quoted, "|"))
}

func zodInvalidURL() *tautError  { return errUnprocessable("Invalid URL") }
func zodInvalidUUID() *tautError { return errUnprocessable("Invalid UUID") }
func zodInvalidDatetime() *tautError {
	return errUnprocessable("Invalid ISO datetime")
}
func zodInvalidDate() *tautError {
	return errUnprocessable("Invalid input: expected date, received Date")
}

func jsonStringPrimitive(v string) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `"` + v + `"`
	}
	return string(b)
}

// receivedType: parity parsedType zod (typeof JS + null/array).
func receivedType(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64, int, int64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		_ = t
		return "object"
	}
}

// ---------- regex validasi ----------

var (
	reUUID      = regexp.MustCompile(`^([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}|00000000-0000-0000-0000-000000000000|ffffffff-ffff-ffff-ffff-ffffffffffff)$`)
	reDatetime  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`)
	reSlug      = regexp.MustCompile(`^[0-9a-z]+(-[0-9a-z]+)*$`)
	reURLScheme = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)
)

// isZodURL: parity z.string().url() — menerima skema apa pun yang dikenali URL
// JS (bukan hanya http/https).
func isZodURL(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" || strings.ContainsAny(s, " \t\n") {
		return false
	}
	if !reURLScheme.MatchString(s) {
		return false
	}
	rest := s[strings.Index(s, "://")+3:]
	if rest == "" {
		return false
	}
	host := rest
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		host = rest[:i]
	}
	if host == "" {
		return false
	}
	// Host tanpa titik masih valid di JS URL (mis. http://localhost) selama
	// bukan string kosong; tolak hanya karakter ilegal yang jelas.
	return !strings.ContainsAny(host, "<>\"'`{}|\\^")
}

// ---------- field helpers ----------

func fieldStr(body map[string]any, key string) (string, bool, error) {
	v, ok := body[key]
	if !ok || v == nil {
		return "", false, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", true, zodInvalidType("string", receivedType(v))
	}
	return s, true, nil
}

func fieldNum(body map[string]any, key string) (float64, bool, error) {
	v, ok := body[key]
	if !ok || v == nil {
		return 0, false, nil
	}
	switch t := v.(type) {
	case float64:
		return t, true, nil
	case int:
		return float64(t), true, nil
	case int64:
		return float64(t), true, nil
	}
	return 0, true, zodInvalidType("number", receivedType(v))
}

func fieldBool(body map[string]any, key string) (bool, bool, error) {
	v, ok := body[key]
	if !ok || v == nil {
		return false, false, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, true, zodInvalidType("boolean", receivedType(v))
	}
	return b, true, nil
}

func fieldArray(body map[string]any, key string) ([]any, bool, error) {
	v, ok := body[key]
	if !ok || v == nil {
		return nil, false, nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, true, zodInvalidType("array", receivedType(v))
	}
	return arr, true, nil
}

func fieldMap(body map[string]any, key string) (map[string]any, bool, error) {
	v, ok := body[key]
	if !ok || v == nil {
		return nil, false, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, true, zodInvalidType("object", receivedType(v))
	}
	return m, true, nil
}

// validateTrimmedString: parity z.string().trim().min(n).max(m) dengan pesan kustom.
func validateTrimmedString(raw string, min, max int, customMin, customMax string) (string, error) {
	s := strings.TrimSpace(raw)
	if len([]rune(s)) < min {
		if customMin != "" {
			return "", errUnprocessable(customMin)
		}
		return "", zodTooSmallString(min)
	}
	if len([]rune(s)) > max {
		if customMax != "" {
			return "", errUnprocessable(customMax)
		}
		return "", zodTooBigString(max)
	}
	return s, nil
}

// validateUUIDArray: array string UUID dengan batas max (0 = tanpa batas).
func validateUUIDArray(arr []any, max int) ([]string, error) {
	if max > 0 && len(arr) > max {
		return nil, zodTooBigArray(max)
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, zodInvalidType("string", receivedType(item))
		}
		if !reUUID.MatchString(s) {
			return nil, zodInvalidUUID()
		}
		out = append(out, s)
	}
	return out, nil
}

// validateStringArray: array string dengan min/max panjang tiap elemen.
func validateStringArray(arr []any, maxItems, minLen, maxLen int) ([]string, error) {
	if maxItems > 0 && len(arr) > maxItems {
		return nil, zodTooBigArray(maxItems)
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, zodInvalidType("string", receivedType(item))
		}
		trimmed := strings.TrimSpace(s)
		if minLen > 0 && len([]rune(trimmed)) < minLen {
			return nil, zodTooSmallString(minLen)
		}
		if maxLen > 0 && len([]rune(trimmed)) > maxLen {
			return nil, zodTooBigString(maxLen)
		}
		out = append(out, trimmed)
	}
	return out, nil
}

// validateEnum: nilai tunggal harus salah satu opsi.
func validateEnum(value string, options ...string) (string, error) {
	for _, o := range options {
		if o == value {
			return value, nil
		}
	}
	return "", zodInvalidOption(options...)
}

// parseDatetimeOrNil: parity z.string().datetime().nullable().
func parseDatetimeOrNil(v any) (*string, error) {
	if v == nil {
		return nil, nil
	}
	s, ok := v.(string)
	if !ok {
		return nil, zodInvalidType("string", receivedType(v))
	}
	if !reDatetime.MatchString(s) {
		return nil, zodInvalidDatetime()
	}
	return &s, nil
}

// requireJSONObject: body harus objek JSON (parity schema.parse(await request.json().catch(() => null))).
func requireJSONObject(r *http.Request) (map[string]any, error) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body == nil {
		return nil, zodInvalidType("object", "null")
	}
	return body, nil
}
