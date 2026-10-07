package platform

import (
	"net/http"

	"github.com/google/uuid"
)

// Kanal media Taut (taut_media_channels) + angle per kanal.

// TautChannelsList: GET /v1/platform/taut/channels
func (d *Deps) TautChannelsList(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found"))
		return
	}
	rows, err := d.queryRows(r.Context(),
		`SELECT c.id, c.name, c.slug, c.description, c.active, c.sort_order,
		        (SELECT COUNT(*)::int FROM taut_content_angles a WHERE a.channel_id = c.id AND a.active) AS angle_count
		 FROM taut_media_channels c
		 ORDER BY c.sort_order, c.name`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": ptRows(rows)})
}

// TautChannelCreate: POST /v1/platform/taut/channels
func (d *Deps) TautChannelCreate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found"))
		return
	}
	body := decodeJSON(r)
	name, _ := strField(body, "name")
	slug, _ := strField(body, "slug")
	description, _ := strField(body, "description")
	active, hasActive := boolField(body, "active")
	sortOrder, hasSort := intField(body, "sort_order")
	if name == "" || len(name) > 120 || len(slug) > 80 || !reTautChannelSlug.MatchString(slug) {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	if slug == "" || len(description) > 1000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	if !hasActive {
		active = true
	}
	if !hasSort {
		sortOrder = 0
	}
	if err := d.exec(r.Context(),
		"INSERT INTO taut_media_channels (id, name, slug, description, active, sort_order, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		uuid.NewString(), name, slug, description, active, sortOrder, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TautChannelUpdate: PATCH /v1/platform/taut/channels/{channelId}
func (d *Deps) TautChannelUpdate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	channelID := chiParam(r, "channelId")
	body := decodeJSON(r)
	name, hasName := strField(body, "name")
	description, hasDescription := strField(body, "description")
	active, hasActive := boolField(body, "active")
	sortOrder, hasSort := intField(body, "sort_order")
	if hasName && (name == "" || len(name) > 120) {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	if hasDescription && len(description) > 1000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	row, err := d.queryRow(r.Context(), "SELECT id FROM taut_media_channels WHERE id = $1 LIMIT 1", channelID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Channel tidak ditemukan."))
		return
	}
	if err := d.exec(r.Context(),
		`UPDATE taut_media_channels SET name = COALESCE($1, name), description = COALESCE($2, description),
		        active = COALESCE($3, active), sort_order = COALESCE($4, sort_order), updated_by = $5, updated_at = NOW()
		 WHERE id = $6`,
		ptNullableArgStr(name, hasName), ptNullableArgStr(description, hasDescription),
		ptNullableArgBool(active, hasActive), ptNullableArgInt(sortOrder, hasSort), user.ID, channelID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "taut.channel_updated", "taut_media_channel", channelID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TautChannelDelete: DELETE /v1/platform/taut/channels/{channelId}
func (d *Deps) TautChannelDelete(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	channelID := chiParam(r, "channelId")
	row, err := d.queryRow(r.Context(), "SELECT id FROM taut_media_channels WHERE id = $1 LIMIT 1", channelID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Channel tidak ditemukan."))
		return
	}
	if err := d.exec(r.Context(), "DELETE FROM taut_media_channels WHERE id = $1", channelID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "taut.channel_deleted", "taut_media_channel", channelID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// TautChannelAngles: GET /v1/platform/taut/channels/{channelId}/angles
func (d *Deps) TautChannelAngles(w http.ResponseWriter, r *http.Request) {
	_, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	channelID := chiParam(r, "channelId")
	rows, err := d.queryRows(r.Context(),
		`SELECT a.id, a.channel_id, a.name, a.slug, a.description, a.source_notes, a.active, a.sort_order,
		        v.id AS active_version_id, v.version AS active_version
		 FROM taut_content_angles a
		 LEFT JOIN taut_content_angle_prompt_versions v
		   ON v.content_angle_id = a.id AND v.status = 'active'
		 WHERE a.channel_id = $1
		 ORDER BY a.sort_order, a.name`, channelID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": ptRows(rows)})
}

// TautAngleCreate: POST /v1/platform/taut/channels/{channelId}/angles
func (d *Deps) TautAngleCreate(w http.ResponseWriter, r *http.Request) {
	user, ctx, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	if !ctx.IsSuperAdmin() {
		writeErr(w, d, errNotFound("Resource not found."))
		return
	}
	channelID := chiParam(r, "channelId")
	channel, err := d.queryRow(r.Context(), "SELECT id FROM taut_media_channels WHERE id = $1 LIMIT 1", channelID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if channel == nil {
		writeErr(w, d, errNotFound("Channel tidak ditemukan."))
		return
	}
	body := decodeJSON(r)
	name, _ := strField(body, "name")
	slug, _ := strField(body, "slug")
	description, _ := strField(body, "description")
	sourceNotes, _ := strField(body, "source_notes")
	active, hasActive := boolField(body, "active")
	sortOrder, hasSort := intField(body, "sort_order")
	if name == "" || len(name) > 160 {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	if slug == "" || len(slug) > 100 || !reTautAngleSlug.MatchString(slug) {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	if len(description) > 2000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	if len(sourceNotes) > 4000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid."))
		return
	}
	if !hasActive {
		active = true
	}
	if !hasSort {
		sortOrder = 0
	}
	angleID := uuid.NewString()
	if err := d.exec(r.Context(),
		"INSERT INTO taut_content_angles (id, channel_id, name, slug, description, source_notes, active, sort_order, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		angleID, channelID, name, slug, description, sourceNotes, active, sortOrder, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": angleID})
}
