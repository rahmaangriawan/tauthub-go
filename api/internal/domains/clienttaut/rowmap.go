package clienttaut

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// rowMap: padanan RowDataPacket app lama — map nama kolom → nilai.
// Salinan pola clientread/partner (tipe unexported tidak bisa diimpor lintas
// paket), dengan normalisasi nilai pgtype/[]byte agar JSON identik.
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
	v, ok := m[key]
	return ok && v != nil
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
	default:
		return fmt.Sprint(v), true
	}
}

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
	// int16: kolom smallint (da, pa, spam_score, ai_trust_score) — tanpa cabang
	// ini nilainya terbaca 0/null (DA media 0 di daftar) padahal terisi di DB.
	case int16:
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
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

func (m rowMap) numOr(key string) float64 {
	f, _ := m.num(key)
	return f
}

func (m rowMap) intOr(key string, fallback int) int {
	f, ok := m.num(key)
	if !ok {
		return fallback
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

// timePtr: kolom timestamp → ISO milidetik atau nil (parity JSON.stringify(Date)).
func (m rowMap) timePtr(key string) any {
	v := m.raw(key)
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case time.Time:
		return isoMilli(t)
	case *time.Time:
		if t == nil {
			return nil
		}
		return isoMilli(*t)
	}
	return nil
}

// timeStr: kolom timestamp → ISO milidetik, atau "" bila NULL.
func (m rowMap) timeStr(key string) string {
	v := m.timePtr(key)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ---------- query helpers (padanan rows()/run()) ----------

func (d *Deps) queryRows(ctx context.Context, sql string, args ...any) ([]rowMap, error) {
	rows, err := d.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fds := rows.FieldDescriptions()
	names := make([]string, len(fds))
	for i, fd := range fds {
		names[i] = string(fd.Name)
	}
	out := []rowMap{}
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		m := make(rowMap, len(names))
		for i, name := range names {
			m[name] = normalizeValue(vals[i])
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// queryRow: satu baris; nil bila tidak ada (parity `const [row] = await rows()`).
func (d *Deps) queryRow(ctx context.Context, sql string, args ...any) (rowMap, error) {
	list, err := d.queryRows(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return list[0], nil
}

func (d *Deps) exec(ctx context.Context, sql string, args ...any) error {
	_, err := d.Pool.Exec(ctx, sql, args...)
	return err
}

func normalizeValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(t)
	case pgtype.Numeric:
		if !t.Valid {
			return nil
		}
		f8, err := t.Float64Value()
		if err != nil || !f8.Valid {
			return nil
		}
		return f8.Float64
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = normalizeValue(item)
		}
		return out
	case time.Time:
		return t
	default:
		return v
	}
}

// isoMilli: serialisasi waktu parity JSON.stringify(Date).
func isoMilli(t time.Time) string {
	return t.Truncate(time.Millisecond).UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

// ---------- JSON ----------

// jsonField: kolom jsonb/text → nilai apa adanya (map/slice/string/nil).
// Parity parseJson app lama: jsonb sudah ter-parse pgx; text masih string JSON.
func jsonField(m rowMap, key string) any {
	v := m.raw(key)
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case string:
		trimmed := strings.TrimSpace(t)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			var parsed any
			if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
				return parsed
			}
			return nil
		}
		// pg mengembalikan jsonb primitif (mis. objective: "conversion") sebagai string.
		return t
	default:
		return v
	}
}

// jsonString: nilai apa pun → JSON string (parity JSON.stringify).
func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

// jsonIndent2: JSON.stringify(x, null, 2).
func jsonIndent2(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "null"
	}
	return string(b)
}

// decodeBody: body JSON → map; body tidak valid/kosong → map kosong.
func decodeBody(r *http.Request) map[string]any {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body == nil {
		return map[string]any{}
	}
	return body
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func strSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		if list, ok := v.([]string); ok {
			return list
		}
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// pqTextArray: literalisasi text[] untuk parameter pgx.
func pqTextArray(items []string) any {
	if len(items) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, item := range items {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		b.WriteString(strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(item))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String()
}

// placeholders: daftar `$n` mulai dari `start`.
func placeholders(start, count int) string {
	parts := make([]string, count)
	for i := 0; i < count; i++ {
		parts[i] = "$" + strconv.Itoa(start+i)
	}
	return strings.Join(parts, ", ")
}

func inPlaceholders(start, count int) string { return placeholders(start, count) }
