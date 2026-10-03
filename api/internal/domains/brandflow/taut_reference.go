package brandflow

import (
	"context"
	"strings"
)

// taut_reference.go: bahan fakta kanonik untuk The Writer DAN Gatekeeper AI
// Overview Checker.
//
// Akar masalah yang diperbaiki: Writer menulis dari brief Profiler + materials
// klien (editorial_standard_snapshot) + topik klien + kutipan narasumber,
// sedangkan Checker hanya menerima brief Profiler. Akibatnya klaim yang sah
// (nama, angka, kutipan dari materials/topik) ditandai "tanpa dasar" dan
// menurunkan skor entitas + E-E-A-T. Keduanya kini memakai fungsi ini.

// tautReferenceMaterial: rangkai bahan fakta untuk satu media. hasFacts=false
// berarti benar-benar tidak ada rujukan: Writer harus menulis generik (tanpa
// nama/angka karangan) dan Checker memperlakukan bahan sebagai kosong.
func (d *Deps) tautReferenceMaterial(ctx context.Context, campaign bsRow, campaignID, mediaID string) (string, bool, error) {
	brief, err := d.loadTautBriefContext(ctx, campaignID)
	if err != nil {
		return "", false, err
	}
	materials := asMapValue(campaign.jsonField("editorial_standard_snapshot"))
	topic := d.loadTautWriterTopic(ctx, campaignID, mediaID)
	spokes := spokespersonListValue(campaign.jsonField("spokepersons"))
	text, hasFacts := buildTautReferenceMaterial(brief, materials, topic, spokes)
	return text, hasFacts, nil
}

// buildTautReferenceMaterial: bagian murni (tanpa DB) supaya bisa diuji.
func buildTautReferenceMaterial(brief string, materials map[string]any, topic string, spokes []spokespersonValue) (string, bool) {
	sections := []string{}
	if s := strings.TrimSpace(brief); s != "" {
		sections = append(sections, "[BRIEF PROFILER]\n"+s)
	}
	if materialHasContent(materials) {
		sections = append(sections, "[MATERI DARI KLIEN]\n"+jsonStringify(materials))
	}
	if s := strings.TrimSpace(topic); s != "" {
		sections = append(sections, "[KONTEKS/TOPIK DARI KLIEN]\n"+s)
	}
	if len(spokes) > 0 {
		lines := make([]string, 0, len(spokes))
		for i, sp := range spokes {
			line := itoa(i+1) + ". " + sp.Name
			if sp.Role != "" {
				line += ", " + sp.Role
			}
			line += " — kutipan: \"" + sp.Quote + "\""
			lines = append(lines, line)
		}
		sections = append(sections, "[NARASUMBER (kutipan resmi)]\n"+strings.Join(lines, "\n"))
	}
	if len(sections) == 0 {
		return "", false
	}
	return strings.Join(sections, "\n\n"), true
}

// materialHasContent: true bila materials klien memuat minimal satu nilai
// non-kosong (brief teks, key_messages, references, assets). Mencegah map yang
// ada tapi seluruh nilainya kosong dianggap sebagai bahan rujukan.
func materialHasContent(m map[string]any) bool {
	if len(m) == 0 {
		return false
	}
	for _, v := range m {
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				return true
			}
		case []any:
			for _, item := range t {
				if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
					return true
				}
			}
		case []string:
			for _, s := range t {
				if strings.TrimSpace(s) != "" {
					return true
				}
			}
		case map[string]any:
			if materialHasContent(t) {
				return true
			}
		}
	}
	return false
}

// loadTautWriterTopic: topik/konteks klien yang tersimpan untuk media ini
// (kolom taut_campaign_media.writer_topic). "" bila belum ada atau kolom belum
// dimigrasi — error dianggap kosong supaya generate/audit tidak ikut gagal.
func (d *Deps) loadTautWriterTopic(ctx context.Context, campaignID, mediaID string) string {
	row, err := d.queryRow(ctx,
		"SELECT writer_topic FROM taut_campaign_media WHERE campaign_id = $1 AND media_id = $2 LIMIT 1",
		campaignID, mediaID)
	if err != nil || row == nil {
		return ""
	}
	return strings.TrimSpace(row.strOr("writer_topic", ""))
}
