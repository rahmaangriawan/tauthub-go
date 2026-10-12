package brandflow

// taut_angle_context.go: jembatan "angle berita" → mesin The Writer.
//
// Angle berita (taut_content_angles) sebelumnya hanya sampai ke prompt sebagai
// instruksi gaya (channel instruction / prompt versi aktif). Berkas ini menambah
// jalur data: nama + deskripsi + kolom "Sumber Informasi" (migrasi 196, diisi
// admin di Prompt Studio) dirangkai menjadi blok [ANGLE BERITA] yang dipakai
// Writer sebagai bahan fakta DAN ikut ke bahan rujukan Gatekeeper AI Overview —
// sehingga klaim yang bersumber dari angle tidak dinilai "tanpa dasar".

import (
	"context"
	"strings"
)

// tautAngleContext: detail angle berita untuk satu tipe media.
type tautAngleContext struct {
	Channel     string
	Slug        string
	Name        string
	Description string
	SourceNotes string
	Prompt      string
}

// writerAgentName: subagent The Writer untuk tipe media ini (migrasi 195).
// Tipe selain blogger dianggap mainstream — sama dengan default mesin writer.
func writerAgentName(mediaType string) string {
	if strings.EqualFold(strings.TrimSpace(mediaType), "blogger") {
		return "writer_blogger"
	}
	return "writer_mainstream"
}

// loadTautAngleContext: ambil angle yang berlaku untuk tipe media ini beserta
// kolom Sumber Informasi-nya. nil = tidak ada angle (kampanye tanpa pilihan
// angle / angle sudah dihapus).
func (d *Deps) loadTautAngleContext(ctx context.Context, campaignID, mediaType string) *tautAngleContext {
	slug := d.tautChosenAngleSlug(ctx, campaignID, mediaType)
	if slug == "" {
		return nil
	}
	angle, _ := d.loadTautAngleBySlug(ctx, slug, mediaType, false)
	return angle
}

// Explicit and saved Writer choices must stay usable; never replace them with a
// different Profiler angle when an admin disables an angle or its prompt.
func (d *Deps) resolveTautWriterAngle(ctx context.Context, campaignID, mediaType, savedSlug string, requestedSlug *string) (*tautAngleContext, bool, error) {
	slug := strings.TrimSpace(savedSlug)
	override := requestedSlug != nil || slug != ""
	if !override {
		return d.loadTautAngleContext(ctx, campaignID, mediaType), false, nil
	}
	if requestedSlug != nil {
		slug = strings.TrimSpace(*requestedSlug)
	}
	if slug == "" || len(slug) > 200 {
		return nil, true, errStatus(422, "Pilih sudut konten yang valid.")
	}
	angle, err := d.loadTautAngleBySlug(ctx, slug, mediaType, true)
	if err != nil {
		return nil, true, err
	}
	if angle == nil || angle.Prompt == "" {
		return nil, true, errStatus(422, "Sudut konten tidak tersedia untuk kanal media ini atau belum memiliki prompt aktif.")
	}
	return angle, true, nil
}

func (d *Deps) loadTautAngleBySlug(ctx context.Context, slug, mediaType string, activeOnly bool) (*tautAngleContext, error) {
	row, err := d.queryRow(ctx, `
		SELECT a.slug, a.name,
		       COALESCE(a.description, '') AS description,
		       COALESCE(a.source_notes, '') AS source_notes,
		       COALESCE((SELECT v.prompt FROM taut_content_angle_prompt_versions v
		                  WHERE v.content_angle_id = a.id AND v.status = 'active'
		                  ORDER BY v.version DESC LIMIT 1), '') AS prompt
		FROM taut_content_angles a
		JOIN taut_media_channels c ON c.id = a.channel_id
		WHERE a.slug = $1 AND c.slug = $2
		  AND (NOT $3::boolean OR (a.active = TRUE AND c.active = TRUE)) LIMIT 1`, slug, mediaType, activeOnly)
	if err != nil || row == nil {
		return nil, err
	}
	return &tautAngleContext{
		Channel:     mediaType,
		Slug:        row.strOr("slug", slug),
		Name:        row.strOr("name", slug),
		Description: row.strOr("description", ""),
		SourceNotes: row.strOr("source_notes", ""),
		Prompt:      strings.TrimSpace(row.strOr("prompt", "")),
	}, nil
}

// tautAngleBriefBlock: rangkai blok teks untuk prompt. Nilai kedua (hasSources)
// menandai apakah blok memuat Sumber Informasi dari admin — hanya itu yang
// dihitung sebagai "bahan fakta" (mencegah model mengarang di luar sumber).
func tautAngleBriefBlock(a *tautAngleContext) (string, bool) {
	if a == nil {
		return "", false
	}
	hasSources := strings.TrimSpace(a.SourceNotes) != ""
	var b strings.Builder
	b.WriteString("[ANGLE BERITA — " + a.Channel + "/" + a.Name + "]")
	if a.Description != "" {
		b.WriteString("\nDeskripsi angle: " + a.Description)
	}
	if hasSources {
		b.WriteString("\nSumber informasi angle (bahan fakta dari admin — pakai yang relevan, jangan menambah fakta di luar ini):\n" +
			truncateString(a.SourceNotes, 4000))
	}
	if a.Prompt != "" {
		b.WriteString("\nInstruksi gaya angle:\n" + truncateString(a.Prompt, 1500))
	}
	return strings.TrimSpace(b.String()), hasSources
}
