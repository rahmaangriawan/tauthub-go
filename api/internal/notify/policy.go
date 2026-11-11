package notify

// Registry kanal notifikasi: satu sumber kebenaran untuk memutuskan kanal mana
// yang boleh dipakai sebuah event.
//
// Sebelum paket ini ada, keputusan "event ini kirim email?" dan "event ini
// kirim WA?" tersebar sebagai if di banyak file pengirim (mpEmailFollowUp,
// mpWAFollowUp, notifyTicketPartner, dan seterusnya). Akibatnya menambah atau
// mematikan satu kanal berarti menyunting beberapa tempat sekaligus, dan tidak
// ada satu halaman yang bisa menjawab "event apa saja yang dibayar".
//
// Prinsip penyusunan matriks: kanal gratis (in-app + push) dipakai seluas
// mungkin, kanal berbayar hanya untuk hal yang tidak bisa ditunda atau tidak
// akan dibaca kalau hanya muncul di lonceng.

// DeliveryMode menentukan kapan sebuah kanal berbayar dipakai.
type DeliveryMode string

const (
	// ModeOff: kanal tidak dipakai untuk event ini.
	ModeOff DeliveryMode = "off"
	// ModeImmediate: kirim segera, tunduk pada gate baca dan kuota.
	ModeImmediate DeliveryMode = "immediate"
	// ModeDigest: kumpulkan lalu kirim sekali sehari.
	ModeDigest DeliveryMode = "digest"
)

// Tingkat kepentingan event. Angka kecil = lebih penting.
const (
	// TierSecurity: akses akun. Tidak lewat lonceng, langsung ke email.
	TierSecurity = 0
	// TierMoney: uang masuk atau ditolak. Selalu dibaca.
	TierMoney = 1
	// TierDeadline: butuh tindakan user dengan tenggat nyata.
	TierDeadline = 2
	// TierInfo: status dan informasi. In-app + push sudah cukup.
	TierInfo = 3
)

// ChannelPolicy adalah keputusan kanal untuk satu tipe notifikasi.
type ChannelPolicy struct {
	// InApp: buat baris di tabel notifications.
	InApp bool
	// Push: kirim web push. Mengikuti trigger DB, jadi nilai ini hanya
	// dokumentasi sampai trigger-nya menghormati policy.
	Push bool
	// Email: mode pengiriman email.
	Email DeliveryMode
	// WA: mode pengiriman WhatsApp.
	WA DeliveryMode
	// Tier: tingkat kepentingan, dipakai untuk urutan dan audit.
	Tier int
}

// policies memetakan tipe notifikasi ke kebijakan kanalnya.
//
// Tipe yang tidak ada di peta ini jatuh ke TierInfo (in-app + push saja).
// Arah default itu disengaja: menambah event baru tanpa mendaftarkannya tidak
// akan diam-diam mengirim email berbayar.
var policies = map[string]ChannelPolicy{
	// --- Tier 0: keamanan dan akses -------------------------------------
	// Email langsung, tanpa lonceng dan tanpa push. Email keamanan bukan
	// "kabar", jadi tidak perlu muncul sebagai notifikasi yang bisa
	// terlewat di antara chat dan status publikasi.
	"security.password_reset":       {InApp: false, Push: false, Email: ModeImmediate, WA: ModeOff, Tier: TierSecurity},
	"security.password_changed":     {InApp: false, Push: false, Email: ModeImmediate, WA: ModeOff, Tier: TierSecurity},
	"security.account_invite":       {InApp: false, Push: false, Email: ModeImmediate, WA: ModeOff, Tier: TierSecurity},
	"security.admin_password_reset": {InApp: false, Push: false, Email: ModeImmediate, WA: ModeOff, Tier: TierSecurity},
	// Notifikasi login: email + WA pada setiap login berhasil.
	"security.login": {InApp: false, Push: false, Email: ModeImmediate, WA: ModeImmediate, Tier: TierSecurity},

	// --- Tier 1: uang ---------------------------------------------------
	//
	// PENTING: tier 1 saat ini BELUM punya pengirim email sama sekali.
	// settNotifyPartner (platform/misc_finance.go) dan insert referral menulis
	// baris notifikasi in-app lalu memanggil notify.EnqueueUserWA untuk kanal
	// WhatsApp. Karena itu Email tetap off, sedangkan WA immediate: gate baca,
	// dedupe, dan kuota otomatis berlaku tanpa menyunting titik pemanggil.
	"withdrawal.requested":         {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierMoney},
	"withdrawal.processing":        {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierMoney},
	"withdrawal.paid":              {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierMoney},
	"withdrawal.rejected":          {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierMoney},
	"referral.cpa.credited":        {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierMoney},
	"referral.commission.credited": {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierMoney},
	"referral.commission.reversed": {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierMoney},
	"referral.domain.approved":     {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierMoney},
	"referral.domain.rejected":     {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierMoney},

	// --- Tier 2: tindakan dengan tenggat --------------------------------
	// Event publikasi hanya menulis notifikasi in-app (partner/shared.go
	// publicationEventNotification dan brandflow/support.go), lalu memanggil
	// notify.EnqueueUserWA untuk audiens mitra dan klien. Email tetap off
	// karena belum ada pengirim email untuk keluarga event ini.
	"publication.waiting_partner":    {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierDeadline},
	"publication.draft_sent":         {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierDeadline},
	"publication.revision_requested": {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierDeadline},
	"publication.url_requested":      {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierDeadline},
	"publication.partner_approved":   {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierDeadline},
	"publication.partner_rejected":   {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierDeadline},
	"publication.client_approved":    {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierDeadline},
	"publication.revision_accepted":  {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierDeadline},
	"publication.revision_rejected":  {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierDeadline},
	"publication.failed":             {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierDeadline},
	// Pengingat tenggat memakai email karena hanya jalur itu yang benar-benar
	// hidup: scheduler brandflow mengirim email, dan belum ada pengirim WA di
	// paket itu (pengirim WA hanya ada di paket platform, dan scheduler
	// sengaja tidak memanggil lintas paket). Mematikan email di sini akan
	// menghentikan pengingat sama sekali, bukan memindahkannya ke WA.
	//
	// WA sengaja off, bukan immediate: menyalakannya sekarang hanya akan
	// menghasilkan baris antrean yang tidak pernah dikirim siapa pun.
	"publication.reminder":      {InApp: true, Push: true, Email: ModeImmediate, WA: ModeOff, Tier: TierDeadline},
	"publication.late_approval": {InApp: true, Push: true, Email: ModeImmediate, WA: ModeOff, Tier: TierDeadline},
	// SLA 3×24 jam persetujuan media: lewat tenggat langsung dibatalkan
	// otomatis. Email segera supaya mitra/klien tidak ketinggalan tenggat;
	// WA tetap off (belum ada pengirim WA di paket ini, sama seperti reminder).
	"publication.sla_cancelled": {InApp: true, Push: true, Email: ModeImmediate, WA: ModeOff, Tier: TierDeadline},
	// Pembatalan oleh admin operasi. Email sengaja off: admin bisa
	// menyalakannya dari panel Template bila pembatalan perlu diberitahukan
	// lewat email juga.
	"publication.cancelled_by_admin": {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierDeadline},

	// --- Tier 3: informasi (in-app + push saja) -------------------------
	"publication.published":                  {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierInfo},
	"publication.link_lost":                  {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"publication.link_replacement_requested": {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"publication.auto_replaced":              {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	// Garansi tautan: klien harus memilih pengganti atau kredit, jadi tier
	// batas waktu (tidak tersenyap). Hasilnya dan pembatalan penghasilan mitra
	// cukup informasi.
	"publication.guarantee_triggered": {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierDeadline},
	"publication.guarantee_update":    {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"partner.guarantee_reversed":      {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	// Pusat Penjual: order hampir kedaluwarsa butuh tindakan hari ini (WA segera);
	// penurunan trafik cukup informasi supaya WhatsApp tidak berisik.
	// Saldo menipis: info (in-app dan push). Email menunggu template di catalog.
	// Rencana Bulanan: info untuk klien (in-app dan push).
	"plan.week_ready":            {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"plan.needs_attention":       {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"plan.renewal_draft":         {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"billing.balance_low":        {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"seller.order_expiring":      {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierDeadline},
	"seller.traffic_drop":        {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"media.approved":             {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierInfo},
	"media.rejected":             {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierInfo},
	"media.submitted":            {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"media.resubmitted":          {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"media.negotiated":           {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"media.dewan_pers.submitted": {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"media.dewan_pers.approved":  {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"media.dewan_pers.rejected":  {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"media.domain_down":          {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"media.crawl_blocked":        {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"media.integration_missing":  {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"ticket.created":             {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	// --- Khusus admin: WA langsung (Fonnte) ------------------------------
	// Bukan notifikasi in-app (event aslinya di atas sudah punya), melainkan
	// kanal WA ke nomor admin agar pengajuan media, penarikan dana, dan tiket
	// dukungan baru tidak menunggu admin membuka dashboard.
	"admin.media_submitted":      {InApp: false, Push: false, Email: ModeOff, WA: ModeImmediate, Tier: TierDeadline},
	"admin.withdrawal_requested": {InApp: false, Push: false, Email: ModeOff, WA: ModeImmediate, Tier: TierMoney},
	"admin.ticket_created":       {InApp: false, Push: false, Email: ModeOff, WA: ModeImmediate, Tier: TierDeadline},
	"ticket.replied":             {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierInfo},
	"ticket.closed":              {InApp: true, Push: true, Email: ModeOff, WA: ModeImmediate, Tier: TierInfo},
	"chat.message":               {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"announcement.new":           {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"vip.enabled":                {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"vip.disabled":               {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"secure_direct.requested":    {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"secure_direct.approved":     {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"pr.verification_completed":  {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"pr.score_updated":           {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"partner.wa_required":        {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"variant_batch.failed":       {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"changelog.new":              {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
	"billing.topup_needed":       {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo},
}

// prefixPolicies menangani tipe yang belum terdaftar tetapi keluarganya tidak
// ambigu. Sengaja hanya memuat keluarga yang seluruh anggotanya satu tier.
//
// "publication" TIDAK ada di sini walau banyak anggotanya tier 2: keluarga itu
// campuran (tier 2 dan tier 3), jadi tipe publication baru yang belum
// didaftarkan harus jatuh ke default in-app saja. Kalau di-prefix, event baru
// akan mewarisi email berbayar tanpa ada yang memutuskan.
var prefixPolicies = map[string]ChannelPolicy{
	"security": {InApp: false, Push: false, Email: ModeImmediate, WA: ModeOff, Tier: TierSecurity},
	// withdrawal dan referral sengaja TANPA email: belum ada pengirim email
	// untuk kedua keluarga ini. Tipe baru di keluarga itu tetap in-app sampai
	// pengirimnya benar-benar dibuat.
	"withdrawal": {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierMoney},
	"referral":   {InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierMoney},
}

// defaultPolicy: in-app + push, tanpa kanal berbayar.
var defaultPolicy = ChannelPolicy{InApp: true, Push: true, Email: ModeOff, WA: ModeOff, Tier: TierInfo}

// PolicyFor mengembalikan kebijakan kanal untuk satu tipe notifikasi.
//
// Urutan pencarian: tipe persis, lalu prefix sebelum titik pertama, lalu
// default. Pemanggil tidak perlu tahu tipe mana yang terdaftar.
func PolicyFor(notifType string) ChannelPolicy {
	if p, ok := policies[notifType]; ok {
		return p
	}
	if prefix, _, found := cutPrefix(notifType); found {
		if p, ok := prefixPolicies[prefix]; ok {
			return p
		}
	}
	return defaultPolicy
}

// cutPrefix memisahkan "publication.published" menjadi "publication" dan
// "published". Ditulis sendiri agar paket ini tidak bergantung pada versi Go
// yang punya strings.CutPrefix.
func cutPrefix(s string) (prefix, rest string, found bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

// AllPolicies mengembalikan salinan seluruh matriks, dipakai halaman admin dan
// audit supaya daftar kanal berbayar bisa dilihat tanpa membaca kode.
func AllPolicies() map[string]ChannelPolicy {
	out := make(map[string]ChannelPolicy, len(policies))
	for k, v := range policies {
		out[k] = v
	}
	return out
}

// PaidChannels mengembalikan kanal berbayar yang aktif untuk satu tipe.
// Dipakai untuk audit: "event apa saja yang menghabiskan biaya".
func PaidChannels(notifType string) []string {
	p := PolicyFor(notifType)
	out := make([]string, 0, 2)
	if p.Email != ModeOff {
		out = append(out, "email")
	}
	if p.WA != ModeOff {
		out = append(out, "wa")
	}
	return out
}
