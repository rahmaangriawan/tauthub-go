package brandflow

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func parseFloat(raw string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(raw), 64)
}

// isoNow: parity new Date().toISOString().
func isoNow() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// parseIntSafe: strconv.Atoi tanpa fallback.
func parseIntSafe(raw string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(raw))
}

// timeAgo: waktu N detik lalu (parity new Date(Date.now() - n*86400000)).
func timeAgo(seconds int) time.Time {
	return time.Now().Add(-time.Duration(seconds) * time.Second)
}

// isHTTPURL: validasi url http(s) sederhana (parity z.string().url()).
func isHTTPURL(raw string) bool {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
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
	return host != "" && !strings.HasPrefix(host, ":")
}

// round2: pembulatan 2 desimal parity app lama.
func round2(v float64) float64 {
	if v < 0 {
		return float64(int(v*100-0.5)) / 100
	}
	return float64(int(v*100+0.5)) / 100
}

// trackingAlphabet: parity CODE_ALPHABET lib/tracking/service.ts.
const trackingAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// randomTrackingCode: kode 8 karakter acak (parity randomTrackingCode).
func randomTrackingCode() string {
	b := make([]byte, 8)
	for i := range b {
		b[i] = trackingAlphabet[randInt(len(trackingAlphabet))]
	}
	return string(b)
}

// helpers.go: util kecil lintas handler + state machine publikasi (port
// lib/publication/state.ts) + createPublicationRecord (port _publication.ts).

// ---------- util ----------

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func capStrings(list []string, n int) []string {
	if len(list) <= n {
		return list
	}
	return list[:n]
}

// jsonStringList: nilai apa pun → []string (array JSON, []any, string JSON, koma).
func jsonStringList(v any) []string {
	switch t := v.(type) {
	case nil:
		return []string{}
	case []string:
		out := []string{}
		for _, s := range t {
			if strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case string:
		trimmed := strings.TrimSpace(t)
		if trimmed == "" {
			return []string{}
		}
		if strings.HasPrefix(trimmed, "[") {
			var arr []any
			if json.Unmarshal([]byte(trimmed), &arr) == nil {
				return jsonStringList(arr)
			}
		}
		out := []string{}
		for _, part := range strings.Split(trimmed, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return []string{}
}

func writeNoStore(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, payload)
}

// stringOrEmpty: nilai apa pun → string ("" bila nil/bukan string).
func stringOrEmpty(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ptrOrNil: string kosong → nil pointer, else pointer.
func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	v := s
	return &v
}

// anyOrNil: string kosong → nil (untuk parameter SQL nullable).
func anyOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ---------- state machine (port lib/publication/state.ts) ----------

var publicationTransitions = map[string][]string{
	"pending":          {"queued", "waiting_partner", "cancelled"},
	"waiting_partner":  {"partner_approved", "partner_rejected", "cancelled"},
	"partner_approved": {"queued"},
	"partner_rejected": {"cancelled"},
	"queued":           {"processing", "cancelled"},
	"processing":       {"published", "failed", "retrying", "cancelled"},
	"published":        {},
	"failed":           {"queued", "retrying", "cancelled"},
	"retrying":         {"queued", "cancelled"},
	"cancelled":        {},
}

var publicationStatuses = []string{
	"pending", "waiting_partner", "partner_approved", "partner_rejected",
	"queued", "processing", "published", "failed", "retrying", "cancelled",
}

func validPublicationStatus(s string) bool {
	for _, v := range publicationStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// assertPublicationTransition: Conflict bila transisi tidak diizinkan.
func assertPublicationTransition(current, next string) error {
	if current == next {
		return nil
	}
	for _, allowed := range publicationTransitions[current] {
		if allowed == next {
			return nil
		}
	}
	return errConflict("Publication status cannot transition from " + current + " to " + next)
}

// transitionPublication: UPDATE atomik dengan guard `WHERE status = current`.
func (d *Deps) transitionPublication(ctx context.Context, publicationID, to string) error {
	row, err := d.queryRow(ctx, "SELECT status FROM publications WHERE id = $1 LIMIT 1", publicationID)
	if err != nil {
		return err
	}
	if row == nil {
		return errNotFound("Publication not found")
	}
	current := row.strOr("status", "")
	if err := assertPublicationTransition(current, to); err != nil {
		return err
	}
	return d.exec(ctx,
		"UPDATE publications SET status = $1, updated_at = NOW() WHERE id = $2 AND status = $3",
		to, publicationID, current)
}

// transitionPublicationWith: varian dengan kolom tambahan.
func (d *Deps) transitionPublicationWith(ctx context.Context, publicationID, to string, extra map[string]any) error {
	row, err := d.queryRow(ctx, "SELECT status FROM publications WHERE id = $1 LIMIT 1", publicationID)
	if err != nil {
		return err
	}
	if row == nil {
		return errNotFound("Publication not found")
	}
	current := row.strOr("status", "")
	if err := assertPublicationTransition(current, to); err != nil {
		return err
	}
	sets := []string{"status = $1"}
	args := []any{to}
	i := 2
	for _, col := range sortedKeys(extra) {
		sets = append(sets, col+" = $"+itoa(i))
		args = append(args, extra[col])
		i++
	}
	args = append(args, publicationID, current)
	sql := "UPDATE publications SET " + strings.Join(sets, ", ") + ", updated_at = NOW() WHERE id = $" + itoa(i) + " AND status = $" + itoa(i+1)
	return d.exec(ctx, sql, args...)
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
