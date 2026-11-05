package notify

// Registry template WhatsApp per AUDIENS.
//
// Sebelumnya template WA hanya dikenali per event, padahal event publikasi
// dikirim ke dua penerima berbeda (mitra pemilik media dan klien pemilik
// artikel) dengan tautan portal yang berbeda. Tanpa pemisahan ini, menyunting
// template untuk mitra ikut mengubah pesan yang diterima klien.
//
// Paket ini adalah satu sumber kebenaran untuk teks bawaan, label, dan tautan
// pratinjau. Domain platform menggabungkan peta di sini ke halaman WA Gateway,
// dan pengirim (notify.EnqueueUserWA) memakai teks yang sama.

// Nilai kolom wa_templates.audience.
const (
	AudiencePartner = "partner"
	AudienceClient  = "client"
	AudienceAdmin   = "admin"
)

// Audiences: urutan tampil di panel admin.
var Audiences = []string{AudiencePartner, AudienceClient, AudienceAdmin}

// AudienceLabel: nama audiens untuk tab di panel admin.
var AudienceLabel = map[string]string{
	AudiencePartner: "Mitra",
	AudienceClient:  "Klien",
	AudienceAdmin:   "Admin",
}

// WADefaultTemplates: teks bawaan per audiens → event.
var WADefaultTemplates = map[string]map[string]string{
	AudiencePartner: {
		"publication.partner_approved": "Halo {nama}, publikasi \"{judul}\" telah DISETUJUI mitra dan masuk antrean terbit.\n\nDetail: {link}",
		"publication.partner_rejected": "Halo {nama}, publikasi \"{judul}\" ditolak.\n\nAlasan: {status}\nDetail: {link}",
		"publication.published":        "Halo {nama}, artikel \"{judul}\" sudah TERBIT di {media}.\n\nLihat: {link}",
		"publication.failed":           "Halo {nama}, publikasi \"{judul}\" gagal diproses.\n\nKeterangan: {status}\nDetail: {link}",
		"media.approved":               "Halo {nama}, media Anda telah disetujui dan siap menerima order.\n\nDetail: {link}",
		"media.rejected":               "Halo {nama}, pengajuan media Anda ditolak.\n\nCatatan: {status}\nDetail: {link}",
		"withdrawal.requested":         "Halo {nama}, permintaan pencairan {jumlah} sudah masuk dan menunggu diproses.\n\nDetail: {link}",
		"withdrawal.processing":        "Halo {nama}, pencairan {jumlah} sedang diproses.\n\nDetail: {link}",
		"withdrawal.paid":              "Halo {nama}, pencairan {jumlah} sudah DIBAYAR.\n\nDetail: {link}",
		"withdrawal.rejected":          "Halo {nama}, pencairan {jumlah} ditolak.\n\nAlasan: {status}\nDetail: {link}",
		"referral.cpa.credited":        "Halo {nama}, fee referral {jumlah} sudah dikreditkan untuk domain {judul}.\n\nDetail: {link}",
		"ticket.replied":               "Halo {nama}, tiket dukungan \"{judul}\" dibalas admin.\n\nBalasan: {status}\nDetail: {link}",
		"ticket.closed":                "Halo {nama}, tiket dukungan \"{judul}\" sudah ditandai selesai.\n\nDetail: {link}",
		"security.login":               "Halo {nama}, kami mendeteksi bahwa Anda berhasil login dengan perangkat berikut :\n\nIP Address: {ip}\nTanggal dan Waktu: {waktu}\nWeb Browser: {browser}\nPlatform: {platform}\n\nJika ini adalah Anda, abaikan pesan ini, jika bukan Anda silakan hubungi kami di\n\nWA : care@tauthub.com\nEmail : +62 851-2466-5015",
	},
	AudienceClient: {
		"publication.partner_approved": "Halo {nama}, media \"{judul}\" sudah menyetujui artikel Anda dan sedang diantrekan untuk terbit.\n\nDetail: {link}",
		"publication.partner_rejected": "Halo {nama}, media menolak artikel \"{judul}\".\n\nAlasan: {status}\nDetail: {link}",
		"publication.published":        "Halo {nama}, artikel \"{judul}\" sudah TERBIT di {media} dan tautannya aktif.\n\nLihat: {link}",
		"publication.failed":           "Halo {nama}, artikel \"{judul}\" gagal diterbitkan.\n\nKeterangan: {status}\nDetail: {link}",
		"security.login":               "Halo {nama}, kami mendeteksi bahwa Anda berhasil login dengan perangkat berikut :\n\nIP Address: {ip}\nTanggal dan Waktu: {waktu}\nWeb Browser: {browser}\nPlatform: {platform}\n\nJika ini adalah Anda, abaikan pesan ini, jika bukan Anda silakan hubungi kami di\n\nWA : care@tauthub.com\nEmail : +62 851-2466-5015",
	},
	AudienceAdmin: {},
}

// WAPreviewLinks: contoh tautan pratinjau per audiens → event.
var WAPreviewLinks = map[string]map[string]string{
	AudiencePartner: {
		"publication.partner_approved": "https://app.tauthub.com/mitra/mitra-contoh/publications?pub=pub-contoh",
		"publication.partner_rejected": "https://app.tauthub.com/mitra/mitra-contoh/publications?pub=pub-contoh",
		"publication.published":        "https://app.tauthub.com/mitra/mitra-contoh/publications?pub=pub-contoh",
		"publication.failed":           "https://app.tauthub.com/mitra/mitra-contoh/publications?pub=pub-contoh",
		"media.approved":               "https://app.tauthub.com/mitra/mitra-contoh/media",
		"media.rejected":               "https://app.tauthub.com/mitra/mitra-contoh/media",
		"withdrawal.requested":         "https://app.tauthub.com/mitra/mitra-contoh/settlements",
		"withdrawal.processing":        "https://app.tauthub.com/mitra/mitra-contoh/settlements",
		"withdrawal.paid":              "https://app.tauthub.com/mitra/mitra-contoh/settlements",
		"withdrawal.rejected":          "https://app.tauthub.com/mitra/mitra-contoh/settlements",
		"referral.cpa.credited":        "https://app.tauthub.com/mitra/mitra-contoh/referral",
		"ticket.replied":               "https://app.tauthub.com/mitra/mitra-contoh/dukungan/tiket-contoh",
		"ticket.closed":                "https://app.tauthub.com/mitra/mitra-contoh/dukungan/tiket-contoh",
	},
	AudienceClient: {
		"publication.partner_approved": "https://app.tauthub.com/klien/brand-contoh/publications?pub=pub-contoh",
		"publication.partner_rejected": "https://app.tauthub.com/klien/brand-contoh/publications?pub=pub-contoh",
		"publication.published":        "https://app.tauthub.com/klien/brand-contoh/publications?pub=pub-contoh",
		"publication.failed":           "https://app.tauthub.com/klien/brand-contoh/publications?pub=pub-contoh",
	},
	AudienceAdmin: {},
}

// WALabels: nama event yang dibaca admin di panel, per audiens.
var WALabels = map[string]map[string]string{
	AudiencePartner: {
		"publication.partner_approved": "Publikasi disetujui mitra",
		"publication.partner_rejected": "Publikasi ditolak mitra",
		"publication.published":        "Artikel terbit",
		"publication.failed":           "Publikasi gagal",
		"media.approved":               "Media disetujui",
		"media.rejected":               "Media ditolak",
		"withdrawal.requested":         "Pencairan diajukan",
		"withdrawal.processing":        "Pencairan diproses",
		"withdrawal.paid":              "Pencairan dibayar",
		"withdrawal.rejected":          "Pencairan ditolak",
		"referral.cpa.credited":        "Fee referral (CPA) masuk",
		"ticket.replied":               "Tiket dukungan dibalas",
		"ticket.closed":                "Tiket dukungan selesai",
		"security.login":               "Notifikasi login",
	},
	AudienceClient: {
		"publication.partner_approved": "Artikel disetujui media",
		"publication.partner_rejected": "Artikel ditolak media",
		"publication.published":        "Artikel terbit",
		"publication.failed":           "Publikasi gagal",
		"security.login":               "Notifikasi login",
	},
	AudienceAdmin: {
		AdminEventMediaSubmitted:      "Admin · Pengajuan media/domain",
		AdminEventWithdrawalRequested: "Admin · Penarikan dana",
		AdminEventTicketCreated:       "Admin · Tiket dukungan baru",
	},
}

// init: event admin berasal dari wa_admins.go supaya pengirim dan panel admin
// memakai teks bawaan yang sama.
func init() {
	for event, body := range AdminWADefaultTemplates {
		WADefaultTemplates[AudienceAdmin][event] = body
	}
	for event, link := range AdminWAPreviewLinks {
		WAPreviewLinks[AudienceAdmin][event] = link
	}
}

// WADefaultTemplate: teks bawaan untuk (audiens, event). Kosong bila event itu
// tidak dipetakan ke audiens tersebut.
func WADefaultTemplate(audience, event string) string {
	if m, ok := WADefaultTemplates[audience]; ok {
		return m[event]
	}
	return ""
}

// WAKnownEvent: true bila event dikenal untuk audiens itu.
func WAKnownEvent(audience, event string) bool {
	return WADefaultTemplate(audience, event) != ""
}

// WAPreviewLink: tautan contoh untuk pratinjau.
func WAPreviewLink(audience, event string) string {
	if m, ok := WAPreviewLinks[audience]; ok {
		return m[event]
	}
	return ""
}

// WALabel: nama event untuk tampilan; jatuh ke nilai event itu sendiri.
func WALabel(audience, event string) string {
	if m, ok := WALabels[audience]; ok {
		if label := m[event]; label != "" {
			return label
		}
	}
	return event
}

// NormalizeAudience: nilai audiens yang sah, default mitra.
func NormalizeAudience(audience string) string {
	switch audience {
	case AudienceClient:
		return AudienceClient
	case AudienceAdmin:
		return AudienceAdmin
	default:
		return AudiencePartner
	}
}
