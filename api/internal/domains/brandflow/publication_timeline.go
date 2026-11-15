package brandflow

import (
	"context"
	"sort"
	"strconv"
	"time"
)

// Timeline kejadian per artikel untuk klien. Tidak ada tabel riwayat status,
// jadi urutannya disusun dari data yang sudah ada: baris publikasi, percobaan
// terbit, notifikasi yang pernah dikirim, dan hasil cek link terakhir. Semua
// kalimat netral: tidak menyebut integrasi, koneksi, URL, atau domain media.

// clientWaitingLongAfter: batas "menunggu lama". Sama dengan sweeper pengingat
// klien (schSweepClientWaiting), supaya lencana di detail dan notifikasinya
// muncul pada saat yang sama.
const clientWaitingLongAfter = 48 * time.Hour

type timelineEvent struct {
	At     time.Time
	Key    string
	Label  string
	Detail string
	Tone   string // ok | warn | bad | info
}

// Notifikasi publikasi yang layak jadi baris timeline. publication.published
// dan publication.failed sengaja tidak ada: sudah diwakili published_at dan
// percobaan terbit.
var timelineNotificationEvents = map[string]struct{ Label, Tone string }{
	"publication.waiting_partner":    {"Menunggu persetujuan media", "info"},
	"publication.partner_approved":   {"Disetujui media", "ok"},
	"publication.partner_rejected":   {"Ditolak media", "bad"},
	"publication.revision_requested": {"Revisi diminta", "warn"},
	"publication.revision_accepted":  {"Revisi diterima", "ok"},
	"publication.revision_rejected":  {"Revisi ditolak", "warn"},
	"publication.client_approved":    {"Anda menyetujui draf", "ok"},
	"publication.waiting_long":       {"Masih menunggu media", "warn"},
	"publication.sla_cancelled":      {"Dibatalkan otomatis karena media tidak merespons", "bad"},
	"publication.cancelled_by_admin": {"Dibatalkan oleh tim Tauthub", "bad"},
	"publication.link_lost":          {"Artikel tidak bisa diakses", "bad"},
	"publication.auto_replaced":      {"Diganti otomatis", "info"},
	"publication.guarantee_triggered": {"Garansi dipakai", "info"},
}

// publicationTimeline menyusun daftar kejadian terurut naik menurut waktu.
func (d *Deps) publicationTimeline(ctx context.Context, publicationID string, attempts []bsRow) []map[string]any {
	events := []timelineEvent{}

	if pub, err := d.queryRow(ctx,
		"SELECT status, created_at, scheduled_at, published_at FROM publications WHERE id = $1 LIMIT 1", publicationID); err == nil && pub != nil {
		if t := rowTime(pub, "created_at"); t != nil {
			events = append(events, timelineEvent{At: *t, Key: "created", Label: "Pesanan dibuat", Tone: "info"})
		}
		if t := rowTime(pub, "scheduled_at"); t != nil {
			events = append(events, timelineEvent{At: *t, Key: "scheduled", Label: "Dijadwalkan terbit", Tone: "info"})
		}
		if t := rowTime(pub, "published_at"); t != nil {
			events = append(events, timelineEvent{At: *t, Key: "published", Label: "Artikel terbit", Tone: "ok"})
		}
	}

	if rows, err := d.queryRows(ctx,
		"SELECT type, MIN(created_at) AS at FROM notifications WHERE publication_id = $1 GROUP BY type", publicationID); err == nil {
		for _, row := range rows {
			spec, ok := timelineNotificationEvents[row.strOr("type", "")]
			if !ok {
				continue
			}
			if t := rowTime(row, "at"); t != nil {
				events = append(events, timelineEvent{At: *t, Key: row.strOr("type", ""), Label: spec.Label, Tone: spec.Tone})
			}
		}
	}

	for _, a := range attempts {
		status := a.strOr("status", "")
		if status != "failed" && status != "retrying" {
			continue
		}
		at := rowTime(a, "completed_at")
		if at == nil {
			at = rowTime(a, "started_at")
		}
		if at == nil {
			continue
		}
		label := "Percobaan terbit ke-" + strconv.Itoa(a.intOr("attempt_number", 0)) + " belum berhasil"
		tone := "warn"
		if status == "failed" {
			label = "Artikel belum bisa diterbitkan"
			tone = "bad"
		}
		events = append(events, timelineEvent{
			At: *at, Key: "attempt_" + strconv.Itoa(a.intOr("attempt_number", 0)),
			Label: label, Detail: clientFailureText(a.strOr("error_code", "")), Tone: tone,
		})
	}

	if lc, err := d.queryRow(ctx,
		"SELECT state, last_checked_at FROM publication_link_checks WHERE publication_id = $1 LIMIT 1", publicationID); err == nil && lc != nil {
		if t := rowTime(lc, "last_checked_at"); t != nil {
			label, tone := "Link dicek: artikel bisa diakses", "ok"
			switch lc.strOr("state", "") {
			case "gone", "removed", "not_found":
				label, tone = "Link dicek: artikel tidak ditemukan", "bad"
			case "unreachable", "error":
				label, tone = "Link dicek: belum bisa dijangkau", "warn"
			}
			events = append(events, timelineEvent{At: *t, Key: "link_check", Label: label, Tone: tone})
		}
	}

	sort.SliceStable(events, func(i, j int) bool { return events[i].At.Before(events[j].At) })
	out := make([]map[string]any, 0, len(events))
	for _, e := range events {
		item := map[string]any{"key": e.Key, "label": e.Label, "tone": e.Tone, "at": e.At}
		if e.Detail != "" {
			item["detail"] = e.Detail
		}
		out = append(out, item)
	}
	return out
}

// publicationWaitingLong: artikel masih menunggu media lebih lama dari batas,
// dihitung dari titik mulai SLA (jadwal terbit bila ada, kalau tidak saat dibuat).
func publicationWaitingLong(status string, since *time.Time) bool {
	return status == "waiting_partner" && since != nil && time.Since(*since) >= clientWaitingLongAfter
}

// rowTime membaca kolom waktu apa adanya (bukan string ISO seperti timePtr).
func rowTime(m bsRow, key string) *time.Time {
	switch t := m.raw(key).(type) {
	case time.Time:
		return &t
	case *time.Time:
		return t
	}
	return nil
}
