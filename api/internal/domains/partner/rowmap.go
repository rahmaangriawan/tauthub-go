package partner

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// rowMap: padanan RowDataPacket app lama — map nama kolom → nilai.
// Port yang menentukan: route lama menulis `SELECT p.*` lalu membaca field
// apa pun dari hasilnya. Peta ini menjaga properti itu tanpa perlu
// struct per-query, dengan konversi pgtype (numeric/jsonb/array) ke tipe
// JS-equivalent supaya serialisasi JSON identik.
type rowMap map[string]any

func (m rowMap) str(key string) (string, bool) {
	v, ok := m[key]
	if !ok || v == nil {
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

// strOr: nilai string dengan fallback saat NULL (parity `row.x ?? "y"`).
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

func (m rowMap) has(key string) bool {
	v, ok := m[key]
	return ok && v != nil
}

func (m rowMap) num(key string) (float64, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
	case int64:
		return float64(t), true
	case int32:
		return float64(t), true
	case int:
		return float64(t), true
	case pgtype.Numeric:
		f, err := numericToFloat(t)
		if err != nil {
			return 0, false
		}
		return f, true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

// numOr: parity `Number(row.x) || 0` — NULL/non-numerik → 0.
func (m rowMap) numOr(key string) float64 {
	f, _ := m.num(key)
	return f
}

// numPtr: parity `row.x != null ? Number(row.x) : null`.
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
	v, ok := m[key]
	if !ok || v == nil {
		return fallback
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return fallback
}

func (m rowMap) boolPtr(key string) any {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return nil
}

// timePtr: kolom timestamp/date → ISO string atau nil (parity JSON Date).
func (m rowMap) timePtr(key string) any {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	if t, ok := v.(time.Time); ok {
		return isoMilli(t)
	}
	return nil
}

// raw: nilai apa adanya (jsonb sudah jadi map/slice, array jadi []any).
func (m rowMap) raw(key string) any {
	v, ok := m[key]
	if !ok {
		return nil
	}
	return v
}

func (m rowMap) parseJSONArray(key string) []string {
	v, ok := m[key]
	if !ok || v == nil {
		return []string{}
	}
	switch t := v.(type) {
	case string:
		return parseJSONArrayString(t)
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

// parseJSONArrayString: parity parseJsonArray — JSON array → []string;
// string legacy non-JSON → satu elemen.
func parseJSONArrayString(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return []string{}
	}
	if strings.HasPrefix(s, "[") {
		var arr []any
		if err := json.Unmarshal([]byte(s), &arr); err == nil {
			out := make([]string, 0, len(arr))
			for _, item := range arr {
				if str, ok := item.(string); ok && str != "" {
					out = append(out, str)
				}
			}
			return out
		}
	}
	return []string{s}
}

// ---------- query helpers (padanan rows()/run() app lama) ----------

// queryRows: jalankan query dan kembalikan seluruh baris sebagai rowMap.
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
	rows, err := d.queryRows(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (d *Deps) exec(ctx context.Context, sql string, args ...any) error {
	_, err := d.Pool.Exec(ctx, sql, args...)
	return err
}

// execRows: jalankan lalu kembalikan jumlah baris terpengaruh (parity run().rowCount).
func (d *Deps) execRows(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := d.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// normalizeValue: pgtype/[]byte → tipe yang gampang diserialkan JSON.
func normalizeValue(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(t)
	case pgtype.Numeric:
		f, err := numericToFloat(t)
		if err != nil {
			return nil
		}
		return f
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = normalizeValue(item)
		}
		return out
	default:
		return v
	}
}

func numericToFloat(n pgtype.Numeric) (float64, error) {
	if !n.Valid {
		return 0, fmt.Errorf("numeric null")
	}
	f8, err := n.Float64Value()
	if err != nil {
		return 0, err
	}
	if !f8.Valid {
		return 0, fmt.Errorf("numeric tidak valid")
	}
	return f8.Float64, nil
}
