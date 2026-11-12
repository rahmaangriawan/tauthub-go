package email

import "strings"

// Katalog event email yang bisa diatur admin.
//
// Setiap event punya audiens (client/partner) dan mode pengiriman:
//   - auth   : dikirim oleh alur akun (reset password, undangan). Copy-nya
//              selalu bisa diganti; kanal tidak pernah dimatikan.
//   - direct : sudah punya pengirim sendiri (media review, tiket, pengingat).
//              Mengaktifkan kanal cukup lewat override policy, pengirim lama
//              yang mengirim.
//   - generic: hanya punya notifikasi in-app. Dispatcher email
//              (DispatchNotificationEmails) yang mengirim bila kanal aktif.
//
// Event yang sama boleh muncul di dua audiens dengan copy berbeda, misalnya
// email reset password untuk klien dan mitra. Keduanya disimpan terpisah.
//
// Variabel yang tersedia di body: {nama}, {judul}, {isi}, {status},
// {jumlah}, {link}, {brand}. Untuk event generic, {judul} berisi judul
// notifikasi in-app dan {isi} berisi isinya, jadi copy generic memakai {isi}
// untuk detail konkret (nama media, nominal, alasan) dan tidak menaruh
// {judul} di subjek.

type SenderMode string

const (
	SenderAuth    SenderMode = "auth"
	SenderDirect  SenderMode = "direct"
	SenderGeneric SenderMode = "generic"
)

const (
	AudienceClient  = "client"
	AudiencePartner = "partner"
)

// TemplateEvent satu entri katalog.
type TemplateEvent struct {
	Audience  string
	Event     string
	Label     string
	Subject   string
	BodyHTML  string
	Variables []string
	Sender    SenderMode
}

var catalogVariables = []string{"{nama}", "{judul}", "{isi}", "{status}", "{jumlah}", "{link}", "{brand}"}

// Variabel khusus event keamanan: kode OTP pendaftaran dan detail perangkat
// login. Dipakai entri security.email_verification dan security.login.
var securityVariables = append(append([]string{}, catalogVariables...), "{kode}", "{ip}", "{waktu}", "{browser}", "{platform}")

// ---------- potongan HTML ----------

func pg(text string) string {
	return `<p style="margin:0 0 12px;line-height:1.6;">` + text + `</p>`
}

func cta(label string) string {
	return `<p style="margin:16px 0 0;"><a href="{link}" style="color:#2b67ec;font-weight:600;text-decoration:none;">` + label + `</a></p>`
}

func muted(text string) string {
	return `<p style="margin:16px 0 0;color:#667085;font-size:13px;line-height:1.5;">` + text + `</p>`
}

// body merangkai sapaan dan paragraf. Semua email transaksional memakai
// sapaan yang sama supaya nadanya konsisten.
func body(parts ...string) string {
	return strings.Join(append([]string{pg("Halo {nama},")}, parts...), "")
}

// ---------- copy bersama (auth dipakai kedua audiens) ----------

var (
	bodyAuthReset = body(
		pg("Kami menerima permintaan untuk mengatur ulang password akun Tauthub Anda."),
		pg("Tautan di bawah berlaku 10 menit dan hanya bisa dipakai satu kali."),
		cta("Atur ulang password"),
		muted("Bila Anda tidak meminta pengaturan ulang, abaikan email ini. Password lama Anda tetap berlaku."),
	)

	bodyAuthChanged = body(
		pg("Password akun Tauthub Anda baru saja diperbarui."),
		pg("Bila Anda yang mengubahnya, tidak ada tindakan lain yang perlu dilakukan."),
		muted("Bila bukan Anda yang mengubah password, hubungi tim Tauthub lewat halaman Dukungan sekarang."),
	)

	bodyAuthInvite = body(
		pg("Akun Tauthub Anda sudah dibuat."),
		pg("Atur password lewat tautan di bawah. Tautan berlaku 60 menit."),
		cta("Atur password"),
		muted("Bila tautan kedaluwarsa, minta admin mengirim ulang undangan."),
	)

	// Kode OTP pendaftaran: {nama} belum ada (akun belum dibuat), pemanggil
	// mengisinya "Sahabat Tauthub".
	bodyAuthEmailVerification = body(
		pg("Gunakan kode berikut untuk melanjutkan pendaftaran akun Tauthub Anda:"),
		pg(`<b style="font-size:26px;letter-spacing:6px;">{kode}</b>`),
		pg("Kode berlaku 10 menit dan hanya bisa dipakai satu kali."),
		muted("Bila Anda tidak mendaftar di Tauthub, abaikan email ini."),
	)

	bodyAuthAccountExists = body(
		pg("Seseorang mencoba mendaftar dengan email ini, tetapi email ini sudah punya akun Tauthub."),
		pg("Masuk memakai akun yang ada. Bila lupa kata sandi, pakai \"Lupa password\" di halaman masuk."),
		cta("Masuk ke Tauthub"),
		muted("Bila bukan Anda yang mencoba mendaftar, abaikan email ini."),
	)

	bodyAuthLoginAlert = body(
		pg("Kami mendeteksi bahwa Anda berhasil login dengan perangkat berikut:"),
		pg("IP Address: {ip}<br>Tanggal dan Waktu: {waktu}<br>Web Browser: {browser}<br>Platform: {platform}"),
		pg("Jika ini adalah Anda, abaikan pesan ini. Jika bukan Anda, silakan hubungi kami di"),
		pg("WA : +62 851-2466-5015<br>Email : care@tauthub.com"),
	)
)

// securityEntry: seperti authEntry, dengan variabel tambahan kode/perangkat.
func securityEntry(audience, event, label, subject, html string) TemplateEvent {
	e := authEntry(audience, event, label, subject, html)
	e.Variables = securityVariables
	return e
}

func entry(audience, event, label, subject, html string, sender SenderMode) TemplateEvent {
	return TemplateEvent{
		Audience: audience, Event: event, Label: label,
		Subject: subject, BodyHTML: html,
		Variables: catalogVariables, Sender: sender,
	}
}

func genericEntry(audience, event, label, subject, html string) TemplateEvent {
	return entry(audience, event, label, subject, html, SenderGeneric)
}

func authEntry(audience, event, label, subject, html string) TemplateEvent {
	return entry(audience, event, label, subject, html, SenderAuth)
}

func directEntry(audience, event, label, subject, html string) TemplateEvent {
	return entry(audience, event, label, subject, html, SenderDirect)
}

var catalog = []TemplateEvent{
	// ---------- KLIEN ----------
	authEntry(AudienceClient, "security.password_reset", "Reset password", "Reset password Tauthub", bodyAuthReset),
	authEntry(AudienceClient, "security.password_changed", "Password diubah", "Password Tauthub Anda telah diubah", bodyAuthChanged),
	authEntry(AudienceClient, "security.account_invite", "Undangan akun", "Aktifkan akun Tauthub Anda", bodyAuthInvite),
	securityEntry(AudienceClient, "security.email_verification", "Kode verifikasi pendaftaran", "Kode verifikasi Tauthub: {kode}", bodyAuthEmailVerification),
	securityEntry(AudienceClient, "security.account_exists", "Email sudah terdaftar", "Email ini sudah punya akun Tauthub", bodyAuthAccountExists),
	securityEntry(AudienceClient, "security.login", "Notifikasi login", "Login baru ke akun Tauthub Anda", bodyAuthLoginAlert),

	genericEntry(AudienceClient, "publication.published", "Artikel terbit", "Artikel Anda sudah terbit",
		body(
			pg("Pesanan publikasi Anda sudah tayang di media yang dipilih."),
			pg("{isi}"),
			pg("Buka halaman Publikasi untuk melihat tautan artikel dan status tayangnya."),
			cta("Lihat publikasi"),
		)),
	genericEntry(AudienceClient, "publication.draft_sent", "Artikel terkirim sebagai draf", "Artikel Anda terkirim sebagai draf",
		body(
			pg("Artikel Anda sudah sampai di situs media sebagai draf dan belum tayang."),
			pg("{isi}"),
			pg("Media akan meninjau lalu menerbitkannya. Anda akan mendapat kabar lagi begitu artikelnya tayang."),
			cta("Lihat publikasi"),
		)),
	genericEntry(AudienceClient, "publication.failed", "Publikasi gagal", "Publikasi artikel gagal",
		body(
			pg("Upaya publikasi gagal dan belum ada artikel yang tayang."),
			pg("{isi}"),
			pg("Buka halaman Publikasi, periksa pesan galat pada baris pesanan, lalu tekan Retry. Kredit pesanan tetap direservasi sampai publikasi berhasil atau pesanan dibatalkan."),
			cta("Buka publikasi"),
		)),
	genericEntry(AudienceClient, "publication.partner_approved", "Disetujui media", "Pesanan Anda disetujui media",
		body(
			pg("Media menyetujui pesanan publikasi Anda."),
			pg("{isi}"),
			pg("Langkah berikutnya mengikuti mode integrasi media: artikel dikirim otomatis lewat API, atau menunggu mitra menempelkan URL artikel. Pantau statusnya di halaman Publikasi."),
			cta("Lihat pesanan"),
		)),
	genericEntry(AudienceClient, "publication.partner_rejected", "Ditolak media", "Pesanan Anda ditolak media",
		body(
			pg("Media menolak pesanan publikasi Anda, sehingga pesanan tidak diteruskan ke penerbitan."),
			pg("{isi}"),
			pg("Buka detail pesanan untuk melihat status terbaru. Bila Anda membatalkan pesanan dari halaman Publikasi, kredit yang direservasi dikembalikan ke Saldo Akun."),
			cta("Buka detail pesanan"),
		)),
	genericEntry(AudienceClient, "publication.revision_requested", "Revisi diminta", "Media meminta revisi",
		body(
			pg("Media meminta revisi untuk naskah yang Anda kirim."),
			pg("{isi}"),
			pg("Buka detail pesanan untuk membaca permintaan revisinya, perbaiki naskah di editor, lalu kirim ulang ke media."),
			cta("Buka permintaan revisi"),
		)),
	genericEntry(AudienceClient, "publication.revision_accepted", "Revisi diterima", "Revisi Anda diterima media",
		body(
			pg("Media menerima revisi naskah Anda."),
			pg("{isi}"),
			pg("Artikel lanjut ke proses penerbitan. Pantau statusnya di halaman Publikasi."),
			cta("Lihat status publikasi"),
		)),
	genericEntry(AudienceClient, "publication.revision_rejected", "Revisi ditolak", "Revisi Anda ditolak media",
		body(
			pg("Media menolak revisi yang Anda kirim."),
			pg("{isi}"),
			pg("Buka detail pesanan untuk membaca catatan media. Bila artikel tidak dilanjutkan, Anda bisa membatalkan pesanan dari halaman Publikasi dan memakai anggarannya untuk media lain."),
			cta("Buka detail pesanan"),
		)),
	genericEntry(AudienceClient, "publication.sla_cancelled", "Dibatalkan otomatis (SLA)", "Pesanan dibatalkan otomatis",
		body(
			pg("Pesanan publikasi dibatalkan otomatis karena media tidak menyetujuinya dalam batas 3×24 jam."),
			pg("Kredit yang direservasi untuk pesanan ini sudah dikembalikan ke Saldo Akun Anda."),
			pg("Buka halaman Publikasi untuk melihat catatan pembatalan dan memesan media lain bila diperlukan."),
			cta("Lihat publikasi"),
		)),

	genericEntry(AudienceClient, "publication.cancelled_by_admin", "Dibatalkan admin", "Pesanan publikasi dibatalkan oleh admin",
		body(
			pg("Admin membatalkan pesanan publikasi Anda."),
			pg("{isi}"),
			pg("Kredit yang direservasi untuk pesanan ini sudah dikembalikan ke Saldo Akun."),
			cta("Lihat publikasi"),
		)),

	genericEntry(AudienceClient, "billing.topup_needed", "Saldo menipis", "Saldo kredit Anda perlu diisi ulang",
		body(
			pg("Saldo kredit Anda menipis."),
			pg("{isi}"),
			pg("Isi ulang saldo agar kampanye dan publikasi berikutnya tidak tertahan."),
			cta("Top up saldo"),
		)),
	genericEntry(AudienceClient, "chat.message", "Pesan baru", "Pesan baru di Tauthub",
		body(
			pg("Ada pesan baru untuk Anda."),
			pg("{isi}"),
			pg("Balas lewat halaman Pesan agar seluruh percakapan tetap tercatat di satu tempat."),
			cta("Buka Pesan"),
		)),
	directEntry(AudienceClient, "report.monthly", "Laporan bulanan", "Laporan bulanan {brand}",
		body(
			pg("Berikut ringkasan aktivitas brand Anda bulan ini."),
			pg("{isi}"),
			cta("Buka dashboard"),
		)),

	genericEntry(AudienceClient, "secure_direct.requested", "Permintaan akses aman", "Permintaan akses data aman",
		body(
			pg("Permintaan akses data aman tercatat di akun Anda."),
			pg("{isi}"),
			pg("Buka detail permintaan untuk meninjau isinya dan melihat status persetujuan."),
			cta("Lihat permintaan"),
		)),

	// ---------- MITRA ----------
	authEntry(AudiencePartner, "security.password_reset", "Reset password", "Reset password Tauthub", bodyAuthReset),
	authEntry(AudiencePartner, "security.password_changed", "Password diubah", "Password Tauthub Anda telah diubah", bodyAuthChanged),
	authEntry(AudiencePartner, "security.account_invite", "Undangan akun", "Aktifkan akun Tauthub Anda", bodyAuthInvite),
	securityEntry(AudiencePartner, "security.login", "Notifikasi login", "Login baru ke akun Tauthub Anda", bodyAuthLoginAlert),

	directEntry(AudiencePartner, "media.approved", "Media disetujui", "Media {judul} telah disetujui",
		body(
			pg("Media {judul} sudah disetujui admin."),
			pg("Media kini tampil di direktori dan bisa dipesan klien. Lengkapi harga, kategori, dan mode integrasi bila belum semuanya terisi."),
			cta("Buka Daftar Media"),
		)),
	directEntry(AudiencePartner, "media.rejected", "Media ditolak", "Pengajuan media {judul} ditolak",
		body(
			pg("Pengajuan media {judul} ditolak admin."),
			pg("{isi}"),
			pg("Buka Daftar Media untuk membaca catatan peninjauan, perbaiki data yang diminta, lalu ajukan ulang."),
			cta("Buka Daftar Media"),
		)),
	directEntry(AudiencePartner, "media.dewan_pers.approved", "Verifikasi Dewan Pers disetujui", "Verifikasi Dewan Pers disetujui",
		body(
			pg("Verifikasi Dewan Pers untuk media {judul} disetujui."),
			pg("Status verifikasi tampil di halaman Daftar Media."),
			cta("Lihat status media"),
		)),
	directEntry(AudiencePartner, "media.dewan_pers.rejected", "Verifikasi Dewan Pers ditolak", "Verifikasi Dewan Pers ditolak",
		body(
			pg("Verifikasi Dewan Pers untuk media {judul} ditolak."),
			pg("{isi}"),
			pg("Buka detail media untuk membaca catatan peninjauan dan memperbaiki data yang diperlukan."),
			cta("Buka detail media"),
		)),
	directEntry(AudiencePartner, "publication.reminder", "Pengingat tenggat", "Pengingat: {judul} menunggu persetujuan",
		body(
			pg("Pesanan {judul} menunggu persetujuan Anda."),
			pg("Setujui atau tolak dari halaman Publikasi sebelum tenggat. Pesanan yang lewat 3×24 jam dibatalkan otomatis dan saldo klien dikembalikan."),
			cta("Tinjau pesanan"),
		)),
	directEntry(AudiencePartner, "publication.late_approval", "Persetujuan terlambat", "Pesanan {judul} melewati tenggat",
		body(
			pg("Pesanan {judul} melewati batas waktu persetujuan."),
			pg("Pesanan sudah ditandai terlambat dan bisa dibatalkan otomatis oleh sistem. Buka halaman Publikasi untuk melihat status terbaru."),
			cta("Buka Publikasi"),
		)),
	directEntry(AudiencePartner, "ticket.replied", "Balasan tiket", "Balasan baru tiket dukungan",
		body(
			pg("Tiket dukungan \"{judul}\" mendapat balasan baru dari admin."),
			pg("Balasan terbaru: {status}"),
			pg("Buka tiket untuk membaca percakapan lengkap dan membalas bila perlu."),
			cta("Buka tiket"),
		)),
	directEntry(AudiencePartner, "ticket.closed", "Tiket selesai", "Tiket dukungan ditandai selesai",
		body(
			pg("Tiket dukungan \"{judul}\" sudah ditandai selesai oleh admin."),
			pg("Bila masalah masih berlanjut, buka tiket dan kirim balasan, atau buat tiket baru dari halaman Dukungan."),
			cta("Buka Dukungan"),
		)),
	directEntry(AudiencePartner, "partner.wa_required", "Nomor WA belum diisi", "Lengkapi nomor WhatsApp Anda",
		body(
			pg("Nomor WhatsApp belum terdaftar di akun Anda."),
			pg("Lengkapi nomor di halaman Profil agar pemberitahuan penting seperti pesanan baru dan tenggat persetujuan juga sampai lewat WhatsApp."),
			cta("Lengkapi profil"),
		)),

	genericEntry(AudiencePartner, "publication.auto_replaced", "Artikel diganti otomatis", "Pesanan Anda dialihkan ke media pengganti",
		body(
			pg("Pesanan publikasi Anda dialihkan otomatis ke media pengganti."),
			pg("{isi}"),
			pg("Buka detail pesanan untuk melihat media baru, harga, dan status penerbitannya."),
			cta("Buka detail pesanan"),
		)),
	genericEntry(AudiencePartner, "publication.draft_sent", "Artikel baru menunggu review", "Artikel baru menunggu review Anda",
		body(
			pg("Ada artikel dari klien yang sudah dikirim ke situs media Anda sebagai draf."),
			pg("{isi}"),
			pg("Artikel belum tayang. Tinjau lalu terbitkan dari situs Anda, atau buka halaman Publikasi untuk detailnya."),
			cta("Buka publikasi"),
		)),
	genericEntry(AudiencePartner, "publication.waiting_partner", "Menunggu persetujuan Anda", "Pesanan baru menunggu persetujuan",
		body(
			pg("Ada pesanan baru yang menunggu persetujuan Anda."),
			pg("{isi}"),
			pg("Setujui atau tolak dari halaman Publikasi. Batas persetujuan 3×24 jam sejak jadwal terbit atau saat pesanan dibuat. Lewat batas itu, pesanan dibatalkan otomatis dan saldo klien dikembalikan."),
			cta("Tinjau pesanan"),
		)),
	genericEntry(AudiencePartner, "publication.client_approved", "Disetujui klien", "Klien menyetujui pesanan",
		body(
			pg("Klien menyetujui pesanan publikasi."),
			pg("{isi}"),
			pg("Lanjutkan penerbitan sesuai mode integrasi media Anda: otomatis lewat API, atau tempel URL artikel setelah tayang."),
			cta("Buka pesanan"),
		)),
	genericEntry(AudiencePartner, "publication.revision_requested", "Revisi diminta", "Klien meminta revisi",
		body(
			pg("Klien meminta revisi naskah."),
			pg("{isi}"),
			pg("Buka detail pesanan untuk membaca permintaan revisi dan menentukan langkah berikutnya."),
			cta("Buka detail pesanan"),
		)),
	genericEntry(AudiencePartner, "publication.revision_accepted", "Revisi diterima", "Klien menerima revisi",
		body(
			pg("Klien menerima revisi naskah."),
			pg("{isi}"),
			pg("Pesanan kembali ke antrean penerbitan. Pantau statusnya di halaman Publikasi."),
			cta("Lihat publikasi"),
		)),
	genericEntry(AudiencePartner, "publication.revision_rejected", "Revisi ditolak", "Klien menolak revisi",
		body(
			pg("Klien menolak revisi naskah."),
			pg("{isi}"),
			pg("Buka detail pesanan untuk membaca catatan klien. Bila perlu penjelasan, hubungi klien lewat halaman Pesan."),
			cta("Buka detail pesanan"),
		)),
	genericEntry(AudiencePartner, "publication.cancelled_by_admin", "Dibatalkan admin", "Pesanan publikasi dibatalkan oleh admin",
		body(
			pg("Admin membatalkan pesanan publikasi untuk media Anda."),
			pg("{isi}"),
			pg("Tidak ada tindakan lanjutan yang diperlukan. Bila pembatalan ini tidak sesuai, hubungi admin lewat halaman Dukungan."),
			cta("Buka Publikasi"),
		)),
	genericEntry(AudiencePartner, "media.submitted", "Media diajukan", "Pengajuan media diterima",
		body(
			pg("Pengajuan media Anda sudah diterima."),
			pg("{isi}"),
			pg("Tim admin meninjau data media. Anda akan menerima kabar setelah peninjauan selesai."),
			cta("Lihat status media"),
		)),
	genericEntry(AudiencePartner, "media.resubmitted", "Media diajukan ulang", "Pengajuan ulang media diterima",
		body(
			pg("Pengajuan ulang media Anda sudah diterima."),
			pg("{isi}"),
			pg("Tim admin meninjau data terbaru. Anda akan menerima kabar setelah peninjauan selesai."),
			cta("Lihat status media"),
		)),
	genericEntry(AudiencePartner, "media.negotiated", "Harga dinegosiasi", "Harga media Anda berubah",
		body(
			pg("Harga media berubah setelah negosiasi."),
			pg("{isi}"),
			pg("Buka Daftar Media untuk melihat harga yang berlaku sekarang."),
			cta("Buka Daftar Media"),
		)),
	genericEntry(AudiencePartner, "media.domain_down", "Domain bermasalah", "Domain media tidak dapat diakses",
		body(
			pg("Domain media tidak dapat diakses saat pemeriksaan."),
			pg("{isi}"),
			pg("Periksa domain dan pengaturan akses situs. Buka detail media untuk melihat hasil pemeriksaan terakhir."),
			cta("Buka detail media"),
		)),
	genericEntry(AudiencePartner, "media.crawl_blocked", "Crawl terblokir", "Crawler tidak dapat membaca media",
		body(
			pg("Crawler tidak dapat membaca halaman media Anda."),
			pg("{isi}"),
			pg("Periksa apakah situs memblokir crawler lewat robots.txt atau aturan keamanan, lalu jalankan pemeriksaan ulang dari detail media."),
			cta("Buka detail media"),
		)),
	genericEntry(AudiencePartner, "vip.enabled", "Fitur VIP aktif", "Fitur VIP akun Anda aktif",
		body(
			pg("Fitur VIP sudah aktif di akun Anda."),
			pg("Buka dashboard VIP untuk melihat masa berlaku dan ketentuan program yang tercatat."),
			cta("Buka dashboard VIP"),
		)),
	genericEntry(AudiencePartner, "vip.disabled", "Fitur VIP nonaktif", "Fitur VIP akun Anda dinonaktifkan",
		body(
			pg("Fitur VIP akun Anda sudah dinonaktifkan."),
			pg("Bila ini tidak sesuai dengan kesepakatan, hubungi admin lewat halaman Dukungan."),
			cta("Buka Dukungan"),
		)),
	genericEntry(AudiencePartner, "secure_direct.requested", "Permintaan akses aman", "Permintaan akses data aman",
		body(
			pg("Ada permintaan akses data aman yang tercatat di akun Anda."),
			pg("{isi}"),
			pg("Buka detail permintaan untuk meninjau isinya dan menentukan persetujuan."),
			cta("Lihat permintaan"),
		)),
	genericEntry(AudiencePartner, "secure_direct.approved", "Akses aman disetujui", "Akses data aman disetujui",
		body(
			pg("Permintaan akses data aman disetujui."),
			pg("{isi}"),
			pg("Akses berlaku sesuai ketentuan yang tercatat di detail permintaan."),
			cta("Lihat detail akses"),
		)),
	genericEntry(AudiencePartner, "pr.verification_completed", "Verifikasi PR selesai", "Verifikasi PR selesai",
		body(
			pg("Verifikasi PR untuk artikel Anda sudah selesai."),
			pg("{isi}"),
			pg("Buka detail untuk melihat hasil verifikasi dan temuan yang perlu ditindak."),
			cta("Lihat hasil verifikasi"),
		)),
	genericEntry(AudiencePartner, "pr.score_updated", "Skor PR diperbarui", "Skor PR Anda diperbarui",
		body(
			pg("Skor PR akun Anda diperbarui."),
			pg("{isi}"),
			pg("Buka detail untuk melihat perubahan skor dan komponen penilainya."),
			cta("Lihat skor PR"),
		)),
	genericEntry(AudiencePartner, "chat.message", "Pesan baru", "Pesan baru di Tauthub",
		body(
			pg("Ada pesan baru untuk Anda."),
			pg("{isi}"),
			pg("Balas lewat halaman Pesan agar seluruh percakapan tetap tercatat di satu tempat."),
			cta("Buka Pesan"),
		)),

	// ---------- MITRA: penarikan dana ----------
	genericEntry(AudiencePartner, "withdrawal.requested", "Permintaan penarikan diterima", "Permintaan penarikan dana diterima",
		body(
			pg("Permintaan penarikan dana Anda sudah diterima."),
			pg("{isi}"),
			pg("Permintaan menunggu review Admin. Dana dikirim ke rekening yang terdaftar setelah disetujui."),
			cta("Lihat penarikan"),
		)),
	genericEntry(AudiencePartner, "withdrawal.processing", "Penarikan diproses", "Penarikan dana sedang diproses",
		body(
			pg("Penarikan dana Anda sedang diproses Admin."),
			pg("{isi}"),
			pg("Status terbaru tampil di halaman Invoice. Tidak ada tindakan yang perlu Anda lakukan saat ini."),
			cta("Buka Invoice"),
		)),
	genericEntry(AudiencePartner, "withdrawal.paid", "Penarikan dibayar", "Dana penarikan sudah ditransfer",
		body(
			pg("Dana penarikan Anda sudah ditransfer ke rekening yang terdaftar."),
			pg("{isi}"),
			pg("Buka halaman Invoice untuk melihat bukti dan referensi transfer. Bila dana belum terlihat di rekening, hubungi Dukungan dengan menyertakan referensi tersebut."),
			cta("Buka Invoice"),
		)),
	genericEntry(AudiencePartner, "withdrawal.rejected", "Penarikan ditolak", "Penarikan dana ditolak",
		body(
			pg("Permintaan penarikan dana Anda ditolak."),
			pg("{isi}"),
			pg("Buka halaman Invoice untuk membaca rinciannya. Bila alasan penolakan belum jelas, hubungi Dukungan."),
			cta("Buka Invoice"),
		)),

	// ---------- MITRA: tiket dukungan ----------
	genericEntry(AudiencePartner, "ticket.created", "Tiket diterima", "Tiket dukungan Anda sudah diterima",
		body(
			pg("Tiket dukungan Anda sudah masuk ke antrean admin."),
			pg("Topik: {isi}"),
			pg("Balasan admin muncul di halaman Dukungan. Anda menerima email lagi saat balasan dikirim."),
			cta("Buka Dukungan"),
		)),
}

// Catalog mengembalikan salinan katalog.
func Catalog() []TemplateEvent {
	out := make([]TemplateEvent, len(catalog))
	copy(out, catalog)
	return out
}

// CatalogFor menyaring katalog per audiens.
func CatalogFor(audience string) []TemplateEvent {
	out := make([]TemplateEvent, 0, len(catalog))
	for _, e := range catalog {
		if e.Audience == audience {
			out = append(out, e)
		}
	}
	return out
}

// CatalogEvent mencari satu entri.
func CatalogEvent(audience, event string) (TemplateEvent, bool) {
	for _, e := range catalog {
		if e.Audience == audience && e.Event == event {
			return e, true
		}
	}
	return TemplateEvent{}, false
}

// IsGenericEvent: event yang dikirim oleh dispatcher notifikasi (bukan
// pengirim direct/auth).
func IsGenericEvent(event string) bool {
	for _, e := range catalog {
		if e.Event == event && e.Sender == SenderGeneric {
			return true
		}
	}
	return false
}

// IsCatalogEvent: event terdaftar untuk audiens apa pun.
func IsCatalogEvent(event string) bool {
	for _, e := range catalog {
		if e.Event == event {
			return true
		}
	}
	return false
}

// AudienceForRole memetakan role user ke audiens email. Admin tidak punya
// template audiens; pemanggil harus jatuh ke template default.
func AudienceForRole(role string) string {
	switch role {
	case "client":
		return AudienceClient
	case "partner":
		return AudiencePartner
	}
	return ""
}
