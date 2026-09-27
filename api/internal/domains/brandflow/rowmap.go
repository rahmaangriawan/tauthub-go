package brandflow

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// rowmap.go: pola rowMap (padanan RowDataPacket app lama) — map nama kolom →
// nilai, dengan normalisasi nilai pgtype/[]byte agar JSON identik.
// Salinan pola clienttaut/partner dengan nama tipe sendiri (bsRow) karena tipe
// unexported tidak bisa diimpor lintas paket.

type bsRow map[string]any

func (m bsRow) raw(key string) any {
	if m == nil {
		return nil
	}
	return m[key]
}

func (m bsRow) has(key string) bool {
	v, ok := m[key]
	return ok && v != nil
}

func (m bsRow) str(key string) (string, bool) {
	v := m.raw(key)
	switch t := v.(type) {
	case string:
		return t, true
	case []byte:
		return string(t), true
	}
	return "", false
}

func (m bsRow) strOr(key, fallback string) string {
	if s, ok := m.str(key); ok {
		return s
	}
	return fallback
}

func (m bsRow) strPtr(key string) *string {
	if s, ok := m.str(key); ok {
		return &s
	}
	return nil
}

func (m bsRow) num(key string) (float64, bool) {
	v := m.raw(key)
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
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
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

func (m bsRow) numOr(key string) float64 {
	f, _ := m.num(key)
	return f
}

func (m bsRow) numPtr(key string) any {
	f, ok := m.num(key)
	if !ok {
		return nil
	}
	return f
}

func (m bsRow) intOr(key string, fallback int) int {
	f, ok := m.num(key)
	if !ok {
		return fallback
	}
	return int(f)
}

func (m bsRow) boolOr(key string, fallback bool) bool {
	if b, ok := m.raw(key).(bool); ok {
		return b
	}
	return fallback
}

func (m bsRow) timePtr(key string) any {
	v := m.raw(key)
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

func (m bsRow) timeStr(key string) string {
	if s, ok := m.timePtr(key).(string); ok {
		return s
	}
	return ""
}

// jsonField: kolom jsonb/text → nilai apa adanya (parity parseJson app lama).
func (m bsRow) jsonField(key string) any {
	v := m.raw(key)
	if s, ok := v.(string); ok {
		trimmed := strings.TrimSpace(s)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			var parsed any
			if json.Unmarshal([]byte(trimmed), &parsed) == nil {
				return parsed
			}
			return nil
		}
		return s
	}
	return v
}

// ---------- query helpers (padanan rows()/run()) ----------

func (d *Deps) queryRows(ctx context.Context, sql string, args ...any) ([]bsRow, error) {
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
	out := []bsRow{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		item := make(bsRow, len(names))
		for i, name := range names {
			item[name] = normalizeValue(values[i])
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// queryRow: satu baris; nil bila tidak ada (parity `const [row] = await rows()`).
func (d *Deps) queryRow(ctx context.Context, sql string, args ...any) (bsRow, error) {
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

// execRows: seperti exec tapi mengembalikan jumlah baris terpengaruh.
func (d *Deps) execRows(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := d.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
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

// placeholders: daftar `$n` mulai dari `start`.
func placeholders(start, count int) string {
	parts := make([]string, count)
	for i := 0; i < count; i++ {
		parts[i] = "$" + itoa(start+i)
	}
	return strings.Join(parts, ", ")
}

// ---------- helper bersama yang dipakai modul brandflow ----------

// clampInt: batas nilai int (parity clampInt di paket lain).
func clampInt(raw string, def, min, max int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

// itoa: pembungkus strconv.Itoa untuk menyusun placeholder $n.
func itoa(n int) string { return strconv.Itoa(n) }

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

// insufficientCreditsError: parity InsufficientCreditsError lib/credits/wallet.ts
// (422 dengan rincian saldo dibutuhkan vs tersedia).
type insufficientCreditsError struct {
	Needed    int
	Available int
}

func (e *insufficientCreditsError) Error() string {
	return "Kredit tidak cukup: butuh " + itoa(e.Needed) + ", tersedia " + itoa(e.Available)
}

// errStatus: error dengan status eksplisit (parity helper status di app lama).
func errStatus(status int, msg string) error {
	return &bfStatusError{Status: status, Message: msg}
}

// parseISO: tanggal ISO → time.Time; false bila tidak valid (parity Date.parse).
func parseISO(v string) (time.Time, bool) {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", trimmed); err == nil {
		return t, true
	}
	return time.Time{}, false
}
