package brandflow

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Antrean penulisan Dapur Produksi (Taut Pintar): artikel massal, cover massal,
// dan artikel turunan. Aturan ringan:
//   - satu job per artikel; job yang selesai mengantre job berikutnya milik batch
//     yang sama, jadi antrean umum (dipakai semua tenant) tetap bisa menyelipkan
//     job lain di antaranya dan tidak ada job yang menahan antrean;
//   - satu batch aktif per brand, maksimal 30 artikel per batch;
//   - kredit baru dipotong setelah tiap artikel berhasil (aturan tagih yang ada).

const (
	writerBatchMaxItems   = 30
	writerBatchRecent     = 30 * time.Minute
	writerBatchStaleAfter = 2 * time.Hour
	writerItemJobKind     = "taut_writer_item"
)

type writerBatchOptions struct {
	Topic         string `json:"topic"`
	Gaya          string `json:"gaya"`
	BriefID       string `json:"brief_id"`
	SourceMediaID string `json:"source_media_id"`
	SourceSlot    int    `json:"source_slot"`
}

// TautWriterBatchCreate: POST .../taut/campaigns/{campaignId}/writer-batches
func (d *Deps) TautWriterBatchCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	ctx := r.Context()
	campaignID := chiParam(r, "campaignId")
	if _, err := d.loadTautCampaignRow(ctx, campaignID, brandRow.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	kind, _ := strField(body, "kind")
	kind = strings.TrimSpace(kind)
	if kind != "article" && kind != "cover" && kind != "derived" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Jenis antrean tidak dikenal.")
		return
	}
	opts := writerBatchOptions{}
	opts.Topic, _ = strField(body, "topic")
	opts.Topic = truncateString(strings.TrimSpace(opts.Topic), 4000)
	opts.Gaya, _ = strField(body, "gaya")
	opts.Gaya = strings.ToLower(strings.TrimSpace(opts.Gaya))
	opts.BriefID, _ = strField(body, "brief_id")
	opts.BriefID = strings.TrimSpace(opts.BriefID)
	opts.SourceMediaID, _ = strField(body, "source_media_id")
	opts.SourceMediaID = strings.TrimSpace(opts.SourceMediaID)
	if n, ok := body["source_slot"].(float64); ok && n >= 1 {
		opts.SourceSlot = int(n)
	}
	if opts.SourceSlot < 1 {
		opts.SourceSlot = 1
	}
	if kind == "article" {
		if opts.Gaya != "proper" {
			opts.Gaya = "sederhana"
		}
		if opts.Gaya == "proper" && opts.BriefID == "" {
			httpx.Error(w, http.StatusUnprocessableEntity, "Gaya Yang Proper butuh brief. Pilih brief dulu.")
			return
		}
	}
	if kind == "derived" {
		if opts.SourceMediaID == "" {
			httpx.Error(w, http.StatusUnprocessableEntity, "Pilih artikel sumber dulu.")
			return
		}
		src, qerr := d.queryRow(ctx,
			`SELECT article_document_id FROM taut_campaign_media
			 WHERE campaign_id = $1 AND media_id = $2 AND slot = $3 AND status = 'selected' LIMIT 1`,
			campaignID, opts.SourceMediaID, opts.SourceSlot)
		if qerr != nil {
			d.writeErr(w, qerr)
			return
		}
		if src == nil || !src.has("article_document_id") {
			httpx.Error(w, http.StatusUnprocessableEntity, "Artikel sumber belum ada isinya.")
			return
		}
	}

	rawItems, _ := body["items"].([]any)
	d.startWriterBatch(w, r, brandRow.ID, user.ID, campaignID, kind, opts, rawItems)
}

// startWriterBatch: inti pembuatan antrean, dipakai pembuatan baru dan "Coba
// lagi". Memvalidasi tiap artikel, aturan satu antrean aktif per brand, batas 30
// artikel, dan saldo di muka, lalu menyimpan antrean dan mengantre job pertama.
func (d *Deps) startWriterBatch(w http.ResponseWriter, r *http.Request, brandID, userID, campaignID, kind string, opts writerBatchOptions, rawItems []any) {
	ctx := r.Context()
	type target struct {
		MediaID string
		Slot    int
	}
	targets := []target{}
	seen := map[string]struct{}{}
	skipped := 0
	for _, raw := range rawItems {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		mediaID, _ := m["media_id"].(string)
		mediaID = strings.TrimSpace(mediaID)
		slot := 1
		if n, ok := m["slot"].(float64); ok && n >= 1 {
			slot = int(n)
		}
		key := mediaID + ":" + itoa(slot)
		if mediaID == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		if kind == "derived" && mediaID == opts.SourceMediaID && slot == opts.SourceSlot {
			skipped++
			continue
		}
		row, qerr := d.queryRow(ctx,
			`SELECT article_document_id FROM taut_campaign_media
			 WHERE campaign_id = $1 AND media_id = $2 AND slot = $3 AND status = 'selected' LIMIT 1`,
			campaignID, mediaID, slot)
		if qerr != nil {
			d.writeErr(w, qerr)
			return
		}
		if row == nil {
			skipped++
			continue
		}
		hasDoc := row.has("article_document_id")
		if (kind == "cover") != hasDoc { // cover butuh artikel; artikel dan turunan butuh baris kosong
			skipped++
			continue
		}
		if _, locked, lerr := d.ArticleScheduleLock(ctx, campaignID, mediaID, slot); lerr != nil {
			d.writeErr(w, lerr)
			return
		} else if locked {
			skipped++
			continue
		}
		targets = append(targets, target{MediaID: mediaID, Slot: slot})
	}
	if len(targets) == 0 {
		httpx.Error(w, http.StatusConflict, "Tidak ada artikel yang bisa diproses. Periksa pilihan Anda.")
		return
	}
	if len(targets) > writerBatchMaxItems {
		httpx.Error(w, http.StatusUnprocessableEntity,
			"Paling banyak "+itoa(writerBatchMaxItems)+" artikel dalam satu antrean. Kurangi pilihan Anda.")
		return
	}

	// Satu antrean aktif per brand. Antrean yang tidak bergerak lebih dari 2 jam
	// dianggap macet dan tidak menghalangi antrean baru.
	if active, aerr := d.queryRow(ctx,
		`SELECT id FROM taut_writer_batches
		 WHERE brand_id = $1 AND status IN ('queued', 'running') AND updated_at > NOW() - INTERVAL '2 hours' LIMIT 1`,
		brandID); aerr != nil {
		d.writeErr(w, aerr)
		return
	} else if active != nil {
		httpx.Error(w, http.StatusConflict, "Masih ada antrean penulisan yang berjalan. Tunggu sampai selesai.")
		return
	}

	// Cek saldo di muka untuk seluruh batch. Kredit tetap dipotong per artikel
	// setelah berhasil, jadi ini hanya mencegah antrean yang pasti gagal di tengah.
	unit := d.writerCreditCost(ctx)
	switch kind {
	case "derived":
		unit = d.derivativeCreditCost(ctx)
	case "cover":
		unit = d.effectiveImageCost(ctx)
	}
	need := unit * len(targets)
	if remaining, _ := d.imageBalance(ctx, brandID); unit > 0 && remaining < need {
		httpx.Error(w, http.StatusPaymentRequired,
			"Saldo kredit tidak cukup: butuh "+itoa(need)+", tersedia "+itoa(remaining)+". Silakan top up terlebih dahulu.")
		return
	}

	optsJSON, _ := json.Marshal(opts)
	batchID := uuid.NewString()
	if err := d.exec(ctx,
		`INSERT INTO taut_writer_batches (id, campaign_id, brand_id, user_id, kind, status, total, options)
		 VALUES ($1, $2, $3, $4, $5, 'queued', $6, $7::jsonb)`,
		batchID, campaignID, brandID, userID, kind, len(targets), string(optsJSON)); err != nil {
		d.writeErr(w, err)
		return
	}
	firstItemID := ""
	for i, t := range targets {
		itemID := uuid.NewString()
		if i == 0 {
			firstItemID = itemID
		}
		if err := d.exec(ctx,
			`INSERT INTO taut_writer_batch_items (id, batch_id, media_id, slot, position) VALUES ($1, $2, $3, $4, $5)`,
			itemID, batchID, t.MediaID, t.Slot, i); err != nil {
			d.writeErr(w, err)
			return
		}
	}
	if _, err := d.enqueueJobAt(ctx, writerItemJobKind,
		map[string]any{"batchId": batchID, "itemId": firstItemID}, nil, 1); err != nil {
		_ = d.exec(ctx, "UPDATE taut_writer_batches SET status = 'failed', updated_at = NOW() WHERE id = $1", batchID)
		d.writeErr(w, err)
		return
	}
	writeNoStore(w, http.StatusOK, map[string]any{
		"id": batchID, "kind": kind, "status": "queued", "total": len(targets), "skipped": skipped,
		"unit_credits": unit, "estimated_credits": need,
	})
}

// TautWriterBatchRetry: POST .../writer-batches/{batchId}/retry
// Antrean baru berisi hanya artikel yang gagal pada antrean lama, dengan jenis
// dan opsi yang sama. Aturan antrean (satu aktif per brand, saldo di muka)
// berlaku lagi lewat startWriterBatch.
func (d *Deps) TautWriterBatchRetry(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	ctx := r.Context()
	campaignID := chiParam(r, "campaignId")
	old, err := d.queryRow(ctx,
		`SELECT id, kind, status, options FROM taut_writer_batches WHERE id = $1 AND campaign_id = $2 AND brand_id = $3 LIMIT 1`,
		chiParam(r, "batchId"), campaignID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if old == nil {
		httpx.Error(w, http.StatusNotFound, "Antrean tidak ditemukan.")
		return
	}
	if st := old.strOr("status", ""); st == "queued" || st == "running" {
		httpx.Error(w, http.StatusConflict, "Antrean ini masih berjalan.")
		return
	}
	var opts writerBatchOptions
	switch v := old.raw("options").(type) {
	case []byte:
		_ = json.Unmarshal(v, &opts)
	case string:
		_ = json.Unmarshal([]byte(v), &opts)
	case map[string]any:
		if raw, merr := json.Marshal(v); merr == nil {
			_ = json.Unmarshal(raw, &opts)
		}
	}
	failed, err := d.queryRows(ctx,
		`SELECT media_id, slot FROM taut_writer_batch_items WHERE batch_id = $1 AND status = 'failed' ORDER BY position`,
		old.strOr("id", ""))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if len(failed) == 0 {
		httpx.Error(w, http.StatusConflict, "Tidak ada artikel yang gagal pada antrean ini.")
		return
	}
	rawItems := make([]any, 0, len(failed))
	for _, it := range failed {
		rawItems = append(rawItems, map[string]any{"media_id": it.strOr("media_id", ""), "slot": float64(it.intOr("slot", 1))})
	}
	d.startWriterBatch(w, r, brandRow.ID, user.ID, campaignID, old.strOr("kind", ""), opts, rawItems)
}

// TautWriterBatchActive: GET .../taut/campaigns/{campaignId}/writer-batches/active
// Antrean terbaru kampanye ini bila masih berjalan atau baru selesai (30 menit),
// beserta status tiap artikel. null bila tidak ada.
func (d *Deps) TautWriterBatchActive(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	ctx := r.Context()
	campaignID := chiParam(r, "campaignId")
	batch, err := d.queryRow(ctx,
		`SELECT id, kind, status, total, done, failed, created_at, updated_at FROM taut_writer_batches
		 WHERE campaign_id = $1 AND brand_id = $2
		   AND (status IN ('queued', 'running') OR updated_at > NOW() - make_interval(secs => $3))
		 ORDER BY created_at DESC LIMIT 1`,
		campaignID, brandRow.ID, writerBatchRecent.Seconds())
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if batch == nil {
		writeNoStore(w, http.StatusOK, map[string]any{"batch": nil})
		return
	}
	rows, err := d.queryRows(ctx,
		`SELECT media_id, slot, status, error_message, similarity FROM taut_writer_batch_items
		 WHERE batch_id = $1 ORDER BY position`, batch.strOr("id", ""))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, it := range rows {
		items = append(items, map[string]any{
			"media_id": it.strOr("media_id", ""), "slot": it.intOr("slot", 1), "status": it.strOr("status", ""),
			"error_message": it.raw("error_message"), "similarity": it.raw("similarity"),
		})
	}
	writeNoStore(w, http.StatusOK, map[string]any{"batch": map[string]any{
		"id": batch.strOr("id", ""), "kind": batch.strOr("kind", ""), "status": batch.strOr("status", ""),
		"total": batch.intOr("total", 0), "done": batch.intOr("done", 0), "failed": batch.intOr("failed", 0),
		"items": items,
	}})
}

// TautWriterItemJob: processor job "taut_writer_item". Mengerjakan SATU artikel
// batch, mencatat hasilnya, lalu mengantre artikel berikutnya atau menutup batch.
// Selalu mengembalikan nil: kegagalan dicatat per artikel dan bisa diulang dari
// layar, jadi tidak ada retry otomatis (menghindari penulisan ganda).
func (d *Deps) TautWriterItemJob(ctx context.Context, payload map[string]any) error {
	batchID := asString(payload["batchId"])
	itemID := asString(payload["itemId"])
	if batchID == "" || itemID == "" {
		return nil
	}
	bg := context.WithoutCancel(ctx)

	batch, err := d.queryRow(bg,
		`SELECT id, campaign_id, brand_id, user_id, kind, options FROM taut_writer_batches WHERE id = $1 LIMIT 1`, batchID)
	if err != nil || batch == nil {
		return nil
	}
	item, err := d.queryRow(bg,
		`SELECT id, media_id, slot, status, updated_at FROM taut_writer_batch_items WHERE id = $1 AND batch_id = $2 LIMIT 1`,
		itemID, batchID)
	if err != nil || item == nil {
		return nil
	}
	status := item.strOr("status", "")
	resumable := status == "queued"
	if status == "running" {
		// Job yang sempat tertahan (worker mati) dilanjutkan setelah 10 menit.
		if t := rowTime(item, "updated_at"); t != nil && time.Since(*t) > 10*time.Minute {
			resumable = true
		}
	}
	if !resumable {
		d.advanceWriterBatch(bg, batchID)
		return nil
	}

	campaignID := batch.strOr("campaign_id", "")
	brandID := batch.strOr("brand_id", "")
	userID := batch.strOr("user_id", "")
	kind := batch.strOr("kind", "")
	mediaID := item.strOr("media_id", "")
	slot := item.intOr("slot", 1)
	var opts writerBatchOptions
	if raw, ok := batch.raw("options").([]byte); ok {
		_ = json.Unmarshal(raw, &opts)
	} else if s, ok := batch.raw("options").(string); ok {
		_ = json.Unmarshal([]byte(s), &opts)
	} else if m, ok := batch.raw("options").(map[string]any); ok {
		if raw, merr := json.Marshal(m); merr == nil {
			_ = json.Unmarshal(raw, &opts)
		}
	}

	_ = d.exec(bg, "UPDATE taut_writer_batch_items SET status = 'running', updated_at = NOW() WHERE id = $1", itemID)
	_ = d.exec(bg, "UPDATE taut_writer_batches SET status = 'running', updated_at = NOW() WHERE id = $1", batchID)

	var runErr error
	switch kind {
	case "article":
		runErr = d.runBatchArticle(ctx, campaignID, brandID, userID, mediaID, slot, opts)
	case "derived":
		runErr = d.runBatchDerived(ctx, campaignID, brandID, userID, mediaID, slot, opts)
	case "cover":
		runErr = d.runBatchCover(ctx, campaignID, brandID, userID, mediaID, slot)
	default:
		runErr = errUnprocessable("Jenis antrean tidak dikenal.")
	}

	if runErr != nil {
		slog.Warn("[writer-batch] artikel gagal", "batch", batchID, "media", mediaID, "kind", kind, "err", truncateString(runErr.Error(), 300))
		_ = d.exec(bg,
			"UPDATE taut_writer_batch_items SET status = 'failed', error_message = $2, updated_at = NOW() WHERE id = $1",
			itemID, writerBatchClientError(runErr))
		_ = d.exec(bg, "UPDATE taut_writer_batches SET failed = failed + 1, updated_at = NOW() WHERE id = $1", batchID)
	} else {
		var sim any
		if kind == "derived" {
			if row, _ := d.queryRow(bg,
				"SELECT derived_similarity FROM taut_campaign_media WHERE campaign_id = $1 AND media_id = $2 AND slot = $3 LIMIT 1",
				campaignID, mediaID, slot); row != nil {
				sim = row.raw("derived_similarity")
			}
		}
		_ = d.exec(bg,
			"UPDATE taut_writer_batch_items SET status = 'done', similarity = $2, updated_at = NOW() WHERE id = $1", itemID, sim)
		_ = d.exec(bg, "UPDATE taut_writer_batches SET done = done + 1, updated_at = NOW() WHERE id = $1", batchID)
	}
	d.advanceWriterBatch(bg, batchID)
	return nil
}

// writerBatchClientError: kalimat biasa untuk klien; penyebab teknis hanya di log.
func writerBatchClientError(err error) string {
	var ic *insufficientCreditsError
	if asInsufficient(err, &ic) {
		return "Saldo kredit tidak cukup."
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "jatah generate"):
		return "Batas jatah generate akun tercapai."
	case strings.Contains(msg, "brief"):
		return "Brief belum siap. Periksa ringkasan brief, lalu coba lagi."
	}
	return "Belum bisa dibuat. Coba lagi dari baris ini."
}

// advanceWriterBatch: antre artikel berikutnya yang masih menunggu, atau tutup
// batch dan kirim notifikasi bila semuanya sudah diproses.
func (d *Deps) advanceWriterBatch(ctx context.Context, batchID string) {
	next, err := d.queryRow(ctx,
		`SELECT id FROM taut_writer_batch_items WHERE batch_id = $1 AND status = 'queued' ORDER BY position LIMIT 1`, batchID)
	if err != nil {
		return
	}
	if next != nil {
		if _, eerr := d.enqueueJobAt(ctx, writerItemJobKind,
			map[string]any{"batchId": batchID, "itemId": next.strOr("id", "")}, nil, 1); eerr != nil {
			slog.Error("[writer-batch] gagal mengantre artikel berikutnya", "batch", batchID, "err", eerr)
			_ = d.exec(ctx, `UPDATE taut_writer_batch_items SET status = 'failed', error_message = 'Antrean terhenti. Coba lagi.' WHERE batch_id = $1 AND status = 'queued'`, batchID)
			d.finishWriterBatch(ctx, batchID)
		}
		return
	}
	d.finishWriterBatch(ctx, batchID)
}

func (d *Deps) finishWriterBatch(ctx context.Context, batchID string) {
	batch, err := d.queryRow(ctx,
		`SELECT campaign_id, brand_id, user_id, kind, total, done, failed FROM taut_writer_batches WHERE id = $1 LIMIT 1`, batchID)
	if err != nil || batch == nil {
		return
	}
	done := batch.intOr("done", 0)
	failed := batch.intOr("failed", 0)
	final := "done"
	if done == 0 && failed > 0 {
		final = "failed"
	}
	tag, uerr := d.Pool.Exec(ctx,
		"UPDATE taut_writer_batches SET status = $2, updated_at = NOW() WHERE id = $1 AND status IN ('queued', 'running')", batchID, final)
	if uerr != nil || tag.RowsAffected() == 0 {
		return // sudah ditutup (jangan kirim notifikasi dua kali)
	}
	title := "Penulisan artikel selesai"
	switch batch.strOr("kind", "") {
	case "cover":
		title = "Pembuatan cover selesai"
	case "derived":
		title = "Artikel untuk media lain selesai"
	}
	body := itoa(done) + " selesai"
	if failed > 0 {
		body += ", " + itoa(failed) + " belum berhasil"
	}
	body += "."
	brandPublic := batch.strOr("brand_id", "")
	if row, _ := d.queryRow(ctx, "SELECT public_id FROM brands WHERE id = $1 LIMIT 1", batch.strOr("brand_id", "")); row != nil {
		if p := row.strOr("public_id", ""); p != "" {
			brandPublic = p
		}
	}
	link := "/klien/" + brandPublic + "/taut-pintar/" + batch.strOr("campaign_id", "") + "/writer"
	d.createNotification(ctx, batch.strOr("user_id", ""), "writer.batch_done", title, body, &link, nil)
}

func (d *Deps) runBatchArticle(ctx context.Context, campaignID, brandID, userID, mediaID string, slot int, o writerBatchOptions) error {
	opts := tautWriterOptions{Topic: o.Topic, Gaya: o.Gaya}
	if o.Gaya == "proper" {
		opts.Proper = &tautProperInput{BriefID: o.BriefID}
	}
	_, err := d.generateTautWriterArticle(ctx, campaignID, brandID, userID, mediaID, slot, opts)
	return err
}

func (d *Deps) runBatchDerived(ctx context.Context, campaignID, brandID, userID, mediaID string, slot int, o writerBatchOptions) error {
	src, srcRow, err := d.loadDerivedSource(ctx, campaignID, o.SourceMediaID, o.SourceSlot)
	if err != nil {
		return err
	}
	srcTopic := d.loadTautWriterTopic(ctx, campaignID, o.SourceMediaID, o.SourceSlot)
	if strings.TrimSpace(o.Topic) != "" {
		srcTopic = o.Topic
	}
	name := d.mediaNameFor(ctx, mediaID)
	opts := tautWriterOptions{
		Topic:           srcTopic,
		Derived:         src,
		ChargeRefPrefix: derivedChargePrefix,
		ChargeCredits:   d.derivativeCreditCost(ctx),
		ChargeReason:    "Taut Pintar — artikel turunan untuk " + name,
	}
	// Gaya dan brief mengikuti artikel asal.
	if stored := d.loadTautProperStored(ctx, srcRow.strOr("id", "")); stored != nil {
		opts.Gaya = "proper"
		opts.Proper = stored
	} else {
		opts.Gaya = "sederhana"
	}
	// Sudut mengikuti artikel asal bila jenis medianya sama; sudut berbeda jenis
	// bisa ditolak aturan media tujuan, jadi dibiarkan dipilih otomatis.
	if angle := srcRow.strOr("angle_slug", ""); angle != "" {
		if dst, _ := d.queryRow(ctx, "SELECT media_type FROM media WHERE id = $1 LIMIT 1", mediaID); dst != nil &&
			dst.strOr("media_type", "") == srcRow.strOr("media_type", "") {
			opts.AngleSlug = &angle
		}
	}
	_, err = d.generateTautWriterArticle(ctx, campaignID, brandID, userID, mediaID, slot, opts)
	return err
}

// runBatchCover membuat cover dari judul artikel dan memasangnya di baris artikel.
func (d *Deps) runBatchCover(ctx context.Context, campaignID, brandID, userID, mediaID string, slot int) error {
	item, err := d.queryRow(ctx,
		`SELECT id, article_document_id FROM taut_campaign_media
		 WHERE campaign_id = $1 AND media_id = $2 AND slot = $3 AND status = 'selected' LIMIT 1`,
		campaignID, mediaID, slot)
	if err != nil {
		return err
	}
	if item == nil || !item.has("article_document_id") {
		return errUnprocessable("Artikel belum ada.")
	}
	doc, err := d.queryRow(ctx, "SELECT title FROM content_documents WHERE id = $1 LIMIT 1", item.strOr("article_document_id", ""))
	if err != nil || doc == nil {
		return errUnprocessable("Artikel belum ada.")
	}
	title := truncateString(strings.TrimSpace(doc.strOr("title", "")), 300)
	if title == "" {
		return errUnprocessable("Judul artikel kosong.")
	}
	prompt := "Gambar sampul untuk artikel berjudul \"" + title + "\". Ilustrasi editorial yang bersih, relevan dengan topik, tanpa teks di dalam gambar."

	cost := d.effectiveImageCost(ctx)
	if remaining, _ := d.imageBalance(ctx, brandID); cost > 0 && remaining < cost {
		return &insufficientCreditsError{Needed: cost, Available: remaining}
	}
	started := time.Now()
	result, err := ai.GenerateImage(ctx, d.Pool, ai.GenerateImageOptions{
		Prompt: prompt, Role: "cover", Heading: title, BrandID: &brandID, UserID: &userID,
	})
	if err != nil {
		return err
	}
	url, err := d.storeGeneratedImage(result, title)
	if err != nil {
		return err
	}
	charged := 0
	if cost > 0 {
		refID := "ai-image:" + campaignID + ":" + mediaID + ":" + uuid.NewString()
		if _, cerr := d.chargeImageCredit(ctx, brandID, refID, cost, "ai.image.generate", userID); cerr != nil {
			return cerr
		}
		charged = cost
	}
	articleID := item.strOr("article_document_id", "")
	if err := d.exec(ctx,
		`INSERT INTO article_images (id, brand_id, campaign_id, media_id, article_id, scope_key, url, provider, model, prompt, heading, role, width, height, cost_credits, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'cover', $12, $13, $14, $15)`,
		uuid.NewString(), brandID, campaignID, mediaID, nullIfEmpty(articleID), imageScopeKey(brandID, campaignID, mediaID, articleID), url,
		result.Provider, result.Model, prompt, nullIfEmpty(title), result.Width, result.Height, charged, userID); err != nil {
		return err
	}
	if err := d.exec(ctx, "UPDATE taut_campaign_media SET featured_image_url = $1 WHERE id = $2", url, item.strOr("id", "")); err != nil {
		return err
	}
	ai.RecordImageEvent(ctx, d.Pool, ai.ImageEvent{
		Kind: "generation", Status: "succeeded", Provider: result.Provider, Model: result.Model, BaseURL: result.BaseURL,
		Prompt: prompt, Role: "cover", BrandID: brandID, CampaignID: campaignID, MediaID: mediaID, ArticleID: articleID,
		RequestedBy: userID, Width: result.Width, Height: result.Height, CreditsCharged: charged,
		ElapsedMS: int(time.Since(started).Milliseconds()), ImageURL: url,
	})
	return nil
}
