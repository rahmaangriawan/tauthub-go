package clientread

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// rowMap: padanan RowDataPacket app lama untuk query yang memilih banyak kolom.
// Dipakai handler yang butuh membaca field dinamis (mis. agregat + join) tanpa
// menulis struct khusus per query.
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

func (m rowMap) strOr(key, fallback string) string {
	if s, ok := m.str(key); ok {
		return s
	}
	return fallback
}

func (m rowMap) num(key string) (float64, bool) {
	v := m.raw(key)
	if v == nil {
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

// timePtr: kolom timestamp → ISO string milidetik atau nil (parity JSON Date).
func (m rowMap) timePtr(key string) any {
	v := m.raw(key)
	if v == nil {
		return nil
	}
	if t, ok := v.(time.Time); ok {
		return t.Truncate(time.Millisecond).UTC().Format("2006-01-02T15:04:05.000Z07:00")
	}
	return nil
}

// queryRows: jalankan query dan kembalikan baris sebagai rowMap.
func (d *Deps) queryRows(ctx context.Context, sql string, args ...any) ([]rowMap, error) {
	rows, err := d.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fields := rows.FieldDescriptions()
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = string(f.Name)
	}
	out := []rowMap{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		item := make(rowMap, len(names))
		for i, name := range names {
			item[name] = normalizeRowValue(values[i])
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// queryRow: satu baris; nil bila tidak ada.
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

// normalizeRowValue: pgtype/[]byte → tipe yang siap diserialkan JSON.
func normalizeRowValue(v any) any {
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
			out[i] = normalizeRowValue(item)
		}
		return out
	default:
		return v
	}
}
