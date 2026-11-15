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

// TautWriterReferenceMaterial shares the Writer's saved-angle context with the
// legacy /finish Gatekeeper route without duplicating angle resolution.
func (d *Deps) TautWriterReferenceMaterial(ctx context.Context, campaignID, brandID, mediaID string) (string, error) {
	campaign, err := d.loadTautCampaignRow(ctx, campaignID, brandID)
	if err != nil {
		return "", err
	}
	text, _, err := d.tautReferenceMaterial(ctx, campaign, campaignID, mediaID, 1)
	return text, err
}

// tautReferenceMaterial: rangkai bahan fakta untuk satu media. hasFacts=false
// berarti benar-benar tidak ada rujukan: Writer harus menulis generik (tanpa
// nama/angka karangan) dan Checker memperlakukan bahan sebagai kosong.
func (d *Deps) tautReferenceMaterial(ctx context.Context, campaign bsRow, campaignID, mediaID string, slot int) (string, bool, error) {
	if slot < 1 {
		slot = 1
	}
	item, err := d.queryRow(ctx, `
		SELECT tcm.angle_slug, m.media_type FROM taut_campaign_media tcm
		JOIN media m ON m.id = tcm.media_id
		WHERE tcm.campaign_id = $1 AND tcm.media_id = $2 AND tcm.slot = $3 LIMIT 1`, campaignID, mediaID, slot)
	if err != nil {
		return "", false, err
	}
	if item == nil {
		return "", false, errNotFound("Media tidak ditemukan di kampanye ini.")
	}
	mediaType := resolveTautMediaType(item.strOr("media_type", ""), campaign.strOr("media_type", ""))
	angle, angleOverride, err := d.resolveTautWriterAngle(ctx, campaignID, mediaType, item.strOr("angle_slug", ""), nil)
	if err != nil {
		return "", false, err
	}
	brief, err := d.loadTautBriefContext(ctx, campaignID, angleOverride)
	if err != nil {
		return "", false, err
	}
	materials := asMapValue(campaign.jsonField("editorial_standard_snapshot"))
	topic := d.loadTautWriterTopic(ctx, campaignID, mediaID, slot)
	spokes := spokespersonListValue(campaign.jsonField("spokepersons"))
	angleBlock, angleHasSources := tautAngleBriefBlock(angle)
	text, hasFacts := buildTautReferenceMaterial(brief, materials, topic, spokes, angleBlock)
	if angleHasSources {
		hasFacts = true
	}
	// Mode Proper tersimpan per media: Checker memverifikasi klaim ke blok yang
	// sama dengan [DATA FAKTA TERKUNCI]/[RINGKASAN ENTITAS BRIEF] milik Writer.
	// Best-effort: bila ringkasan belum ada, QC tetap jalan tanpa blok itu.
	if proper := d.loadTautProperStored(ctx, item.strOr("id", "")); proper != nil {
		briefCtx, _ := d.loadTautProperBriefContext(ctx, campaign.strOr("brand_id", ""), proper.BriefID)
		if block := buildTautProperBlock(proper, briefCtx); block != "" {
			text = strings.TrimSpace(text + "\n\n" + block)
			hasFacts = true
		}
	}
	// Artikel turunan: teks artikel asal ikut jadi bahan, supaya Checker tidak
	// menandai fakta yang memang berasal dari sana sebagai klaim tanpa dasar.
	if src, _ := d.queryRow(ctx,
		"SELECT derived_source_text FROM taut_campaign_media WHERE campaign_id = $1 AND media_id = $2 AND slot = $3 LIMIT 1",
		campaignID, mediaID, slot); src != nil {
		if t := strings.TrimSpace(src.strOr("derived_source_text", "")); t != "" {
			text = strings.TrimSpace(text + "\n\n[RUJUKAN FAKTA DARI ARTIKEL MEDIA LAIN]\n" + t)
			hasFacts = true
		}
	}
	return text, hasFacts, nil
}

func resolveTautMediaType(mediaType, campaignType string) string {
	if mt := strings.TrimSpace(mediaType); mt != "" {
		return mt
	}
	if mt := strings.TrimSpace(campaignType); mt != "" {
		return mt
	}
	return "mainstream"
}

// buildTautReferenceMaterial: bagian murni (tanpa DB) supaya bisa diuji.
func buildTautReferenceMaterial(brief string, materials map[string]any, topic string, spokes []spokespersonValue, angleBlock string) (string, bool) {
	sections := []string{}
	if s := strings.TrimSpace(angleBlock); s != "" {
		sections = append(sections, "[ANGLE BERITA (sumber informasi)]\n"+s)
	}
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
func (d *Deps) loadTautWriterTopic(ctx context.Context, campaignID, mediaID string, slot int) string {
	if slot < 1 {
		slot = 1
	}
	row, err := d.queryRow(ctx,
		"SELECT writer_topic FROM taut_campaign_media WHERE campaign_id = $1 AND media_id = $2 AND slot = $3 LIMIT 1",
		campaignID, mediaID, slot)
	if err != nil || row == nil {
		return ""
	}
	return strings.TrimSpace(row.strOr("writer_topic", ""))
}
