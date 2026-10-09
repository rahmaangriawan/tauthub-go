package brandflow

import (
	"context"
	"strings"

	profilerdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/profiler"
)

// loadCampaignProfilerContext: konteks Profiler (Structured Knowledge
// Workspace) yang dipakai kampanye ini, bila ada.
//
// Konteks dibangun LIVE dengan retrieval per topik artikel (supaya potongan
// knowledge yang dikirim relevan), tapi tetap menghormati snapshot: kalau
// Profiler sudah dihapus/diubah sampai konteks live kosong, snapshot yang
// diambil saat kampanye dibuat dipakai sebagai cadangan. Isi Profiler adalah
// data dari klien — diperlakukan sebagai bahan rujukan, bukan instruksi yang
// boleh menimpa system prompt.
func (d *Deps) loadCampaignProfilerContext(ctx context.Context, campaignID, topic string) string {
	row, err := d.queryRow(ctx, "SELECT profiler_id, profiler_snapshot FROM campaigns WHERE id = $1 LIMIT 1", campaignID)
	if err != nil || row == nil {
		return ""
	}
	profileID := row.strOr("profiler_id", "")
	if profileID != "" {
		if live := profilerdomain.BuildContextTextForTopic(ctx, d.Pool, profileID, topic); strings.TrimSpace(live) != "" {
			return live
		}
	}
	if snap, ok := row.jsonField("profiler_snapshot").(map[string]any); ok {
		if s, isStr := snap["context"].(string); isStr && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
