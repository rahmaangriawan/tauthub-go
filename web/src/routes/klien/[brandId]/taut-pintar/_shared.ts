/**
 * Konstanta + helper bersama Taut Pintar klien.
 *
 * Port dari app lama: `lib/taut/*` (campaign-stats, writer-links, matchmaker-ai,
 * profiler/schema, profiler/angle-fields) dan data presentasional dari komponen
 * client Next.js. Disimpan privat (prefiks `_`) supaya tidak jadi route.
 */

import { sanitizeHtml } from '$lib/sanitize-html';

/* ─── Daftar kampanye ─────────────────────────────────────────────────── */

export const STAGE_LABEL: Record<string, string> = {
	draft: 'Draft',
	profiling: 'Profiling',
	profiled: 'Profiled',
	materials: 'Bahan',
	budget: 'Anggaran',
	placement: 'Placement',
	production: 'Produksi',
	review: 'Verifikasi',
	scheduled: 'Terjadwal',
	submitted: 'Terkirim'
};

/** Nada visual per tahap (parity STAGE_COLOR Radix). */
export const STAGE_TONE: Record<string, 'muted' | 'warning' | 'accent' | 'success'> = {
	draft: 'muted',
	profiling: 'warning',
	profiled: 'accent',
	materials: 'accent',
	budget: 'accent',
	placement: 'accent',
	production: 'accent',
	review: 'warning',
	scheduled: 'accent',
	submitted: 'accent'
};

export const STATUS_OPTIONS: Array<{ value: string; label: string }> = [
	{ value: 'all', label: 'Semua Status' },
	...Object.entries(STAGE_LABEL).map(([k, v]) => ({ value: k, label: v }))
];

export function statusAccent(tautStatus: string | null, published: number): 'accent' | 'success' | 'warning' | 'muted' {
	if (published > 0) return 'success';
	return STAGE_TONE[tautStatus ?? 'draft'] ?? 'muted';
}

/* ─── Tahapan agent ───────────────────────────────────────────────────── */

export const AGENT_STEPS = ['Katalog Media', 'Penyedia Konten'] as const;
export const AGENT_STEP_ROUTES = ['matchmaker', 'writer', 'ringkasan'] as const;

/** Alasan The Profiler di-bypass (kampanye jalur manual). */
export const MANUAL_PROFILER_REASON =
	'The Profiler dilewati: kampanye ini dibuat lewat jalur Manual, jadi tidak ada brief AI yang disusun. Media dipilih sendiri dan artikelnya kamu siapkan.';

/** Status kampanye yang sudah final: hanya boleh dilihat, tidak boleh diedit lagi. */
export const DONE_STATUSES = new Set(['submitted', 'scheduled', 'distributed']);

export function isCampaignDone(status?: string | null): boolean {
	return DONE_STATUSES.has(status ?? '');
}

/* ─── Berita acara ────────────────────────────────────────────────────── */

export const MEDIA_LABEL: Record<string, string> = {
	blogger: 'Blogger',
	mainstream: 'Mainstream',
	kombinasi: 'Kombinasi'
};
export const BUDGET_LABEL: Record<string, string> = {
	efisien: 'Efisien',
	optimal: 'Optimal',
	brutal: 'Brutal'
};

/* ─── Profiler: enumerasi & dynamic fields ────────────────────────────── */

export const OBJECTIVES = [
	{ value: 'brand_awareness', label: 'Brand Awareness & Edukasi' },
	{ value: 'seo_dominance', label: 'Dominasi SEO & Backlink' },
	{ value: 'conversion', label: 'Konversi & Promosi Langsung' },
	{ value: 'reputation_management', label: 'Manajemen Reputasi / Klarifikasi' }
] as const;

export const OBJECTIVE_DESCRIPTIONS: Record<string, string> = {
	brand_awareness:
		'Fokus AI: Menyebarluaskan nama brand dan cerita di baliknya, pendekatan soft-selling.',
	seo_dominance:
		'Fokus AI: Struktur heading ketat, optimasi LSI, dan pemetaan keyword organik yang natural.',
	conversion: 'Fokus AI: Menonjolkan USP tajam dengan CTA menuju landing page.',
	reputation_management:
		'Fokus AI: Bahasa hati-hati, taktis, meredam sentimen negatif, menyajikan fakta kuat.'
};

export const OTHER_AUDIENCE = 'Other / Lainnya';

export const DEFAULT_AUDIENCE_OPTIONS = [
	'Mahasiswa',
	'Ibu Hamil',
	'Karyawan',
	'Profesional',
	'Pemilik Bisnis',
	'Pengusaha / Entrepreneur',
	'Freelancer',
	'Orang Tua',
	'Pelajar',
	'Fresh Graduate',
	'Manajer / Decision Maker',
	'Developer / Teknisi',
	'Marketing / PR Professional',
	'Investor',
	'Konsumen Rumah Tangga'
];

export type DynamicAngleField = { key: string; label: string; placeholder: string };

/** Port lib/taut/profiler/angle-fields.ts (satu sumber dengan DB slug). */
export const ANGLE_DYNAMIC_FIELDS: Record<string, DynamicAngleField[]> = {
	listicle: [
		{ key: 'list_items', label: 'Apa saja item/entitas yang masuk daftar rekomendasi?', placeholder: 'Contoh: 7 software PR untuk kantor pemasaran digital...' },
		{ key: 'ranking_basis', label: 'Apa kriteria urutan/penilaian daftar?', placeholder: 'Contoh: fitur monitoring, harga, integrasi, laporan...' },
		{ key: 'brand_edge', label: 'Apa pembeda brand Anda dibanding item lain di daftar?', placeholder: 'Contoh: satu-satunya dengan laporan terjadwal otomatis tiap minggu...' }
	],
	product_review: [
		{ key: 'product_name', label: 'Nama produk yang direview?', placeholder: 'Contoh: Speaker Bluetooth X100...' },
		{ key: 'reviewer_experience', label: 'Pengalaman penggunaan produknya seperti apa?', placeholder: 'Contoh: dipakai 2 minggu untuk rapat online, baterai tahan 8 jam...' },
		{ key: 'strengths_weaknesses', label: 'Kelebihan dan kekurangan utamanya?', placeholder: 'Contoh: suara jernih, tapi agak berat dibawa...' }
	],
	how_to: [
		{ key: 'task_goal', label: 'Masalah/tugas apa yang mau diselesaikan reader?', placeholder: 'Contoh: cara memilih laptop untuk desain grafis...' },
		{ key: 'steps_tips', label: 'Langkah-langkah atau tips utamanya?', placeholder: 'Contoh: cek RAM minimal 16GB, GPU dedicated, layar 100% sRGB...' },
		{ key: 'tools_needed', label: 'Alat/bahan yang dibutuhkan?', placeholder: 'Contoh: hanya butuh browser dan spreadsheet...' }
	],
	comparison: [
		{ key: 'compared_items', label: 'Produk/layanan apa saja yang dibandingkan?', placeholder: 'Contoh: paket hosting A vs paket hosting B vs VPS...' },
		{ key: 'comparison_aspects', label: 'Aspek apa yang dibandingkan?', placeholder: 'Contoh: harga, performa, support, kemudahan migrasi...' },
		{ key: 'your_position', label: 'Posisi produk Anda di perbandingan?', placeholder: 'Contoh: unggul di support 24/7 dan garansi uptime...' }
	],
	unboxing: [
		{ key: 'product_name', label: 'Nama produk yang di-unboxing?', placeholder: 'Contoh: Smartphone flagship merek X...' },
		{ key: 'package_contents', label: 'Isi paket/kemasannya apa saja?', placeholder: 'Contoh: unit, casing, kabel, kartu garansi...' },
		{ key: 'first_impression', label: 'Kesan pertama saat membuka paket?', placeholder: 'Contoh: build terasa solid, warna lebih premium dari foto...' }
	],
	success_case: [
		{ key: 'case_subject', label: 'Siapa/kasus siapa yang dibahas?', placeholder: 'Contoh: UMKM roti yang naik omzet 3x...' },
		{ key: 'before_condition', label: 'Kondisi sebelum memakai solusi Anda?', placeholder: 'Contoh: pencatatan manual, sering salah hitung stok...' },
		{ key: 'after_result', label: 'Hasil nyata setelahnya (angka bila ada)?', placeholder: 'Contoh: omzet naik dari 5 juta ke 15 juta per bulan...' }
	],
	niche_education: [
		{ key: 'topic_concept', label: 'Konsep/isu niche apa yang dijelaskan?', placeholder: 'Contoh: apa itu NVMe dan bedanya dengan SSD SATA...' },
		{ key: 'common_confusion', label: 'Kebingungan umum reader tentang topik ini?', placeholder: 'Contoh: banyak yang kira semua SSD sama kecepatannya...' },
		{ key: 'practical_takeaway', label: 'Takeaway praktis untuk reader?', placeholder: 'Contoh: cek slot M.2 di motherboard sebelum beli NVMe...' }
	],
	community_event: [
		{ key: 'event_name', label: 'Nama acara/event-nya?', placeholder: 'Contoh: Komunitas Pecinta Kopi Fest 2026...' },
		{ key: 'event_details', label: 'Kapan, di mana, dan siapa penyelenggaranya?', placeholder: 'Contoh: 12 Agustus di Balai Kartini Jakarta, oleh Asosiasi PR Indonesia...' },
		{ key: 'brand_involvement', label: 'Peran brand Anda di acara itu?', placeholder: 'Contoh: sponsor utama dan buka booth demo mesin kopi...' }
	],
	soft_sell_promo: [
		{ key: 'offer_details', label: 'Apa penawaran spesialnya (potongan, bonus, periode)?', placeholder: 'Contoh: diskon 30% semua layanan sampai akhir bulan...' },
		{ key: 'offer_terms', label: 'Syarat dan cara klaimnya?', placeholder: 'Contoh: min. transaksi 100rb, klaim via WhatsApp...' },
		{ key: 'why_now', label: 'Kenapa pembaca harus segera ambil penawaran ini?', placeholder: 'Contoh: kuota 50 pelanggan pertama saja...' }
	],
	product_launch: [
		{ key: 'product_name', label: 'Nama produk/fitur yang diluncurkan?', placeholder: 'Contoh: Aplikasi Kasir Tauthub Pay...' },
		{ key: 'launch_details', label: 'Kapan diluncurkan dan apa yang baru?', placeholder: 'Contoh: rilis 20 Agustus, fitur split bill otomatis...' },
		{ key: 'availability', label: 'Di mana/kapan bisa didapat dan harganya?', placeholder: 'Contoh: tersedia di Play Store, gratis 30 hari...' }
	],
	funding_announcement: [
		{ key: 'funding_round', label: 'Tahap dan besaran pendanaan?', placeholder: 'Contoh: Series A sebesar Rp 50 miliar...' },
		{ key: 'investors', label: 'Siapa investor yang terlibat?', placeholder: 'Contoh: dipimpin VenturX dengan partisipasi Angel Cap...' },
		{ key: 'fund_usage', label: 'Dana akan dipakai untuk apa?', placeholder: 'Contoh: ekspansi ke 5 kota dan riset AI...' }
	],
	c_level_change: [
		{ key: 'executive_name', label: 'Nama dan jabatan baru pejabat?', placeholder: 'Contoh: Budi Santoso ditunjuk sebagai CEO...' },
		{ key: 'background', label: 'Latar belakang profesionalnya?', placeholder: 'Contoh: 15 tahun di industri fintech, mantan direktur Bank X...' },
		{ key: 'mandate', label: 'Misi/tanggung jawab utamanya?', placeholder: 'Contoh: mempercepat ekspansi regional...' }
	],
	strategic_partnership_b2b: [
		{ key: 'partner_names', label: 'Siapa pihak yang bermitra?', placeholder: 'Contoh: PT Maju Jaya dengan Distribusi Nusantara...' },
		{ key: 'partnership_scope', label: 'Apa isi lingkup kerja samanya?', placeholder: 'Contoh: distribusi eksklusif produk di Sumatera...' },
		{ key: 'partnership_goal', label: 'Target hasil kemitraannya?', placeholder: 'Contoh: jangkau 500 toko dalam 1 tahun...' }
	],
	milestone_awards: [
		{ key: 'achievement', label: 'Pencapaian/penghargaan apa yang diraih?', placeholder: 'Contoh: penghargaan UMKM Digital Terbaik 2026...' },
		{ key: 'milestone_numbers', label: 'Angka milestone-nya (user, cabang, tahun)?', placeholder: 'Contoh: 10.000 pelanggan aktif, cabang ke-12...' },
		{ key: 'milestone_context', label: 'Pemberi penghargaan / konteks perayaannya?', placeholder: 'Contoh: dari Dinas Koperasi UKM provinsi...' }
	],
	research_data_trends: [
		{ key: 'research_subject', label: 'Topik riset/tren yang diangkat?', placeholder: 'Contoh: tren belanja online gens Z Indonesia...' },
		{ key: 'key_data', label: 'Data/angka kunci yang ada?', placeholder: 'Contoh: 68% Gen Z belanja via marketplace tiap bulan...' },
		{ key: 'data_source', label: 'Sumber data/risetnya dari mana?', placeholder: 'Contoh: survei internal 1.200 responden...' }
	],
	csr_sustainability: [
		{ key: 'csr_program', label: 'Nama program CSR/sustainability-nya?', placeholder: 'Contoh: Gerakan Hijau Sekolah...' },
		{ key: 'csr_activities', label: 'Kegiatan dan penerima manfaatnya?', placeholder: 'Contoh: tanam 1.000 pohon, melibatkan 20 sekolah...' },
		{ key: 'csr_impact', label: 'Dampak/hasil yang ditargetkan?', placeholder: 'Contoh: penghijauan 5 hektar lahan kritis...' }
	],
	rebranding_transformation: [
		{ key: 'rebrand_reason', label: 'Alasan utama rebranding/transformasi?', placeholder: 'Contoh: perluasan pasar ke segmen keluarga...' },
		{ key: 'rebrand_changes', label: 'Apa saja yang berubah (nama, logo, layanan)?', placeholder: 'Contoh: logo baru, nama singkat, layanan booking online...' },
		{ key: 'rebrand_continuity', label: 'Apa yang tetap dipertahankan?', placeholder: 'Contoh: tim konsultan yang sama dan standar layanan...' }
	],
	reputation_clarification: [
		{ key: 'issue_summary', label: 'Isu apa yang mau diklarifikasi?', placeholder: 'Contoh: isu produk mengandung bahan berbahaya...' },
		{ key: 'clarification_facts', label: 'Fakta/data pendukung klarifikasi?', placeholder: 'Contoh: hasil lab Sertifikasi No. 123/LAB/X...' },
		{ key: 'corrective_steps', label: 'Langkah perbaikan yang diambil?', placeholder: 'Contoh: recall sukarela dan penggantian gratis...' }
	],
	thought_leadership: [
		{ key: 'expert_identity', label: 'Siapa pakarnya (nama, jabatan)?', placeholder: 'Contoh: dr. Andi, Direktur Medis Klinik Sehat...' },
		{ key: 'opinion_topic', label: 'Isu/opini strategis yang diangkat?', placeholder: 'Contoh: krisis air bersih kota besar 2030...' },
		{ key: 'key_argument', label: 'Argumen utama/solusi yang ditawarkan?', placeholder: 'Contoh: recycling air wajib di gedung perkantoran...' }
	],
	inspirational_story: [
		{ key: 'main_figure', label: 'Siapa tokoh utama kisahnya?', placeholder: 'Contoh: Ibu Ratna, penjual nasi keliling...' },
		{ key: 'struggle_journey', label: 'Perjuangan dan titik baliknya?', placeholder: 'Contoh: dari modal 50 ribu kini punya 3 cabang...' },
		{ key: 'inspiration_message', label: 'Pesan inspirasi untuk pembaca?', placeholder: 'Contoh: konsisten kecil-kecil tetapi berdampak...' }
	],
	large_event_coverage: [
		{ key: 'event_name', label: 'Nama event/konferensinya?', placeholder: 'Contoh: Tech Summit Indonesia 2026...' },
		{ key: 'event_scale', label: 'Skala, waktu, dan tempat penyelenggaraan?', placeholder: 'Contoh: 5.000 peserta, 15-16 Sep, JCC Senayan...' },
		{ key: 'brand_participation', label: 'Partisipasi/presensi brand di event?', placeholder: 'Contoh: jadi exhibitor dan bicara di panel AI...' }
	],
	special_offer_advertorial: [
		{ key: 'offer_headline', label: 'Penawaran utama untuk advertorial?', placeholder: 'Contoh: paket website company profile 2 juta...' },
		{ key: 'offer_audience', label: 'Target pembeli penawaran ini?', placeholder: 'Contoh: UMKM yang belum punya website...' },
		{ key: 'offer_cta', label: 'Ajakan/CTA yang diinginkan di akhir artikel?', placeholder: 'Contoh: hubungi WhatsApp untuk konsultasi gratis...' }
	]
};

/* ─── Matchmaker: humanisasi slug angle ───────────────────────────────── */

export const ANGLE_LABELS: Record<string, string> = {
	product_launch: 'Peluncuran Produk',
	special_offer_advertorial: 'Promosi & Penawaran Spesial',
	thought_leadership: 'Opini Pakar',
	research_data_trends: 'Riset, Data & Tren Industri',
	strategic_partnership_b2b: 'Kemitraan Strategis & Ekspansi B2B',
	milestone_awards: 'Pencapaian & Penghargaan',
	funding_announcement: 'Pengumuman Pendanaan',
	csr_sustainability: 'CSR & Keberlanjutan',
	rebranding_transformation: 'Rebranding & Transformasi',
	c_level_change: 'Pergantian Manajemen',
	inspirational_story: 'Kisah Inspiratif',
	reputation_clarification: 'Manajemen Reputasi',
	large_event_coverage: 'Liputan Event',
	product_review: 'Review Produk',
	comparison: 'Perbandingan Produk',
	how_to: 'Tips & Trik / How-to',
	listicle: 'Listicle / Rekomendasi',
	success_case: 'Studi Kasus',
	unboxing: 'Unboxing',
	soft_sell_promo: 'Promo Soft-sell',
	niche_education: 'Edukasi Niche',
	community_event: 'Liputan Event Komunitas'
};

/** Slug angle teknis → label manusiawi (parity lib/taut/matchmaker-ai.ts). */
export function humanizeReason(text: string | null | undefined): string {
	if (!text) return '';
	let out = String(text);
	for (const [slug, label] of Object.entries(ANGLE_LABELS)) {
		const escaped = slug.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
		out = out.replace(new RegExp(`\\b${escaped}\\b`, 'g'), label);
	}
	return out.trim();
}

/* ─── Writer: URL terlacak (parity lib/taut/writer-links.ts) ──────────── */

export function buildTrackedUrl(
	rawUrl: string | null | undefined,
	opts?: { mode?: 'direct' | 'tauthub'; medium?: string; campaign?: string }
): string {
	const url = (rawUrl ?? '').trim();
	if (!url) return '';
	if (opts?.mode !== 'tauthub') return url;
	try {
		const parsed = new URL(url);
		const search = new URLSearchParams(parsed.search);
		search.set('utm_source', 'tauthub');
		search.set('utm_medium', (opts?.medium ?? 'taut').trim() || 'taut');
		if (opts?.campaign) search.set('utm_campaign', opts.campaign.trim());
		parsed.search = search.toString();
		return parsed.toString();
	} catch {
		return url;
	}
}

export function stripScheme(raw: string): string {
	return raw.trim().replace(/^https?:\/\//i, '').replace(/^\/+/, '').trim();
}

export function withScheme(raw: string): string {
	const v = raw.trim().replace(/^\/+/, '');
	if (!v) return '';
	return /^https?:\/\//i.test(v) ? v : `https://${v}`;
}

const BULLET_RE = /^[•◦▪‣∙·*\-–—]\s+/;
const NUMBERED_RE = /^\(?\d{1,2}[.)]\s+/;

/**
 * Bersihkan teks tempelan dari Google Docs / Word / PDF sebelum jadi artikel.
 *
 * Yang ditangani (semuanya khas copy-paste dokumen):
 * - CRLF/CR → LF, NBSP → spasi, tab → spasi, zero-width & BOM dibuang;
 * - spasi ganda dan spasi di ujung baris dirapikan;
 * - baris yang terpotong (hard wrap) disatukan kembali: baris sebelumnya tidak
 *   berakhir tanda baca DAN baris berikutnya mulai huruf kecil/angka — kecuali
 *   baris berikutnya adalah item bullet/nomor (penanda daftar tidak boleh
 *   tersedot ke item sebelumnya);
 * - lebih dari satu baris kosong berturut-turut diciutkan jadi satu.
 */
export function cleanPastedText(text: string): string {
	const normalized = text
		.replace(/\r\n?/g, '\n')
		.replace(/[\u200B-\u200D\uFEFF]/g, '')
		.replace(/\u00A0/g, ' ')
		.replace(/[\u2028\u2029]/g, '\n')
		.replace(/\t/g, ' ')
		.replace(/[ ]{2,}/g, ' ')
		.replace(/ +\n/g, '\n');

	const merged: string[] = [];
	for (const raw of normalized.split('\n')) {
		const line = raw.trim();
		if (line === '') {
			merged.push('');
			continue;
		}
		const prev = merged.length > 0 ? merged[merged.length - 1] : '';
		const penandaDaftar = BULLET_RE.test(line) || NUMBERED_RE.test(line);
		const lanjutan = prev !== '' && !penandaDaftar && !/[.!?;:"”’)\]]$/.test(prev) && /^[a-z0-9(]/.test(line);
		if (lanjutan) merged[merged.length - 1] = `${prev} ${line}`;
		else merged.push(line);
	}
	return merged.join('\n').replace(/\n{3,}/g, '\n\n').trim();
}

/**
 * Teks tempelan → HTML artikel. Paragraf mengikuti baris (Google Docs memberi
 * satu newline per paragraf), bullet/nomor berurutan dikelompokkan jadi
 * <ul>/<ol>, dan seluruh teks di-escape.
 */
export function textToHtml(text: string): string {
	const clean = cleanPastedText(text);
	if (!clean) return '';
	const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

	const out: string[] = [];
	let list: { type: 'ul' | 'ol'; items: string[] } | null = null;
	const flush = () => {
		if (!list) return;
		out.push(`<${list.type}>${list.items.map((i) => `<li>${i}</li>`).join('')}</${list.type}>`);
		list = null;
	};

	for (const block of clean.split(/\n{2,}/)) {
		for (const line of block.split('\n')) {
			const bullet = line.match(BULLET_RE);
			const numbered = bullet ? null : line.match(NUMBERED_RE);
			const marker = bullet ?? numbered;
			if (marker) {
				const type = bullet ? 'ul' : 'ol';
				const item = esc(line.slice(marker[0].length).trim());
				if (list && list.type === type) list.items.push(item);
				else {
					flush();
					list = { type, items: [item] };
				}
			} else {
				flush();
				out.push(`<p>${esc(line)}</p>`);
			}
		}
		flush();
	}
	return out.join('');
}

/* ─── Tempelan kaya (clipboard HTML) ────────────────────────────────── */

/**
 * Clipboard Google Docs menandai akar kopiannya dengan
 * id="docs-internal-guid-…". Dipakai hanya untuk memberi tahu klien bahwa
 * struktur aslinya terbaca — bukan syarat pemrosesan.
 */
export function isDocsClipboardHtml(html: string): boolean {
	return /docs-internal-guid|id="docs-[^"]*"|class="[^"]*\bdocs-/.test(html);
}

export type TempelKaya = { title: string; html: string; plain: string };

/**
 * HTML clipboard (Google Docs / Word web) → HTML artikel yang rapi.
 *
 * Textarea hanya menerima plain text, padahal clipboard membawa dua flavor
 * sekaligus (text/plain + text/html) dan yang HTML memuat struktur asli:
 * `<h1>/<h2>/<h3>`, `<table>`, `<ul>/<ol>`. Fungsi ini mempertahankan struktur
 * itu dan membuang sampah ekspor Docs:
 * - `<meta>`/`<style>`/`<colgroup>` dan pembungkus
 *   `<b style="font-weight:normal" id="docs-internal-guid-…">`;
 * - seluruh atribut kecuali href/src/alt/colspan/rowspan (style, class, id,
 *   dir, role) — penyebab tampilan "tempelan" yang berantakan di editor;
 * - NBSP, zero-width, dan blok kosong.
 *
 * Heading dipertahankan PERSIS seperti di Docs: "Judul 1" (`<h1>`) tetap `<h1>`,
 * "Judul 2" tetap `<h2>`, dan seterusnya. Judul artikel diambil dengan dua pola:
 * `<h1 class="title">` (gaya "Judul" dokumen di Docs), atau — bila tidak ada —
 * H1 pertama yang menjadi blok berisi pertama (pola umum "Judul 1" sebagai judul
 * artikel). Heading yang diangkat menjadi judul dikeluarkan dari isi.
 */
export function clipboardHtmlToArticle(rawHtml: string): TempelKaya | null {
	if (typeof DOMParser === 'undefined' || !rawHtml.trim()) return null;
	const doc = new DOMParser().parseFromString(rawHtml, 'text/html');
	const body = doc.body;
	if (!body) return null;

	body.querySelectorAll('meta, style, link, title, colgroup, col, script, noscript').forEach((el) => el.remove());

	// Pembungkus ekspor Docs: <b style="font-weight: normal" id="docs-internal-guid-…">.
	// Dibuka dulu supaya penilaian "H1 pertama di awal dokumen" melihat sibling
	// yang sebenarnya.
	body.querySelectorAll('b[style*="font-weight"], b[id*="docs-internal-guid"]').forEach((el) => {
		el.replaceWith(...Array.from(el.childNodes));
	});

	// Judul artikel, dua pola Docs:
	// 1. Gaya "Judul" (<h1 class="title">) — judul dokumen, selalu dipakai.
	// 2. Gaya "Judul 1" (<h1>) yang ditulis sebagai judul artikel — dipakai bila
	//    ia blok berisi PERTAMA (tidak ada isi lain sebelumnya). H1 berikutnya
	//    dibiarkan di badan sebagai sub-bagian.
	let title = '';
	const judulNode = body.querySelector('h1.title, h1[class*="title"]');
	if (judulNode) {
		title = (judulNode.textContent ?? '').replace(/\s+/g, ' ').trim();
		judulNode.remove();
	}
	if (!title) {
		const h1Pertama = body.querySelector('h1');
		if (h1Pertama) {
			let adaIsiSebelumnya = false;
			for (let s = h1Pertama.previousElementSibling; s; s = s.previousElementSibling) {
				if ((s.textContent ?? '').trim()) {
					adaIsiSebelumnya = true;
					break;
				}
			}
			const teks = (h1Pertama.textContent ?? '').replace(/\s+/g, ' ').trim();
			if (!adaIsiSebelumnya && teks && teks.length <= 200) {
				title = teks;
				h1Pertama.remove();
			}
		}
	}

	const KEEP_ATTR = new Set(['href', 'src', 'alt', 'colspan', 'rowspan']);
	for (const el of Array.from(body.querySelectorAll('*'))) {
		for (const attr of Array.from(el.attributes)) {
			if (!KEEP_ATTR.has(attr.name.toLowerCase())) el.removeAttribute(attr.name);
		}
	}

	const walker = doc.createTreeWalker(body, NodeFilter.SHOW_TEXT);
	const teksNodes: Text[] = [];
	while (walker.nextNode()) teksNodes.push(walker.currentNode as Text);
	for (const node of teksNodes) {
		node.data = node.data
			.replace(/[\u200B-\u200D\uFEFF]/g, '')
			.replace(/\u00A0/g, ' ')
			.replace(/[ \t]{2,}/g, ' ');
	}

	// <span> tanpa atribut hanyalah sisa pembungkus ekspor Docs — dibuka.
	body.querySelectorAll('span').forEach((el) => {
		if (el.attributes.length === 0) el.replaceWith(...Array.from(el.childNodes));
	});

	body.querySelectorAll('p, h2, h3, h4, li, td, th, tr, ul, ol, table').forEach((el) => {
		const adaMedia = el.querySelector('img, iframe, br, td, th, li');
		if (!adaMedia && !(el.textContent ?? '').trim()) el.remove();
	});
	// <tr> kosong yang tersisa (semua selnya terhapus) ikut dibuang.
	body.querySelectorAll('tr').forEach((tr) => {
		if (!tr.querySelector('td, th')) tr.remove();
	});

	const html = sanitizeHtml(body.innerHTML).trim();

	// Teks biasa per blok, dipakai untuk mengisi kotak tempel (perilaku lama).
	// Hanya blok terdalam yang diambil agar isi <li><p>…</p></li> tidak dobel,
	// dan tiap blok jadi satu baris (bukan hasil textContent yang menempel).
	const BLOK = 'p, h2, h3, h4, li, th, td';
	const plain = Array.from(body.querySelectorAll(BLOK))
		.filter((el) => !el.querySelector(BLOK))
		.map((el) => (el.textContent ?? '').replace(/\u00A0/g, ' ').replace(/\s+/g, ' ').trim())
		.filter(Boolean)
		.join('\n');

	if (!html || !plain) return null;
	if (!title) {
		const barisPertama = plain.split('\n')[0] ?? '';
		if (barisPertama.length > 0 && barisPertama.length <= 120) title = barisPertama;
	}
	return { title, html, plain };
}

/* ─── Format ──────────────────────────────────────────────────────────── */

export const fmtIdr = (n: number) => (n ?? 0).toLocaleString('id-ID');
export const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}`;
const rpFmt = new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0, roundingMode: 'trunc' });
export const rpCurrency = (n: number) => rpFmt.format(n ?? 0);

export function dt(v: string | null | undefined): string {
	if (!v) return '—';
	const d = new Date(v);
	if (Number.isNaN(d.getTime())) return '—';
	return d.toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' });
}

/** ISO → jam WIB (Asia/Jakarta), untuk jadwal distribusi. */
export function toJakartaDisplay(iso: string | null): string | null {
	if (!iso) return null;
	return new Date(iso).toLocaleString('id-ID', {
		timeZone: 'Asia/Jakarta',
		day: 'numeric',
		month: 'short',
		year: 'numeric',
		hour: '2-digit',
		minute: '2-digit'
	});
}

/** ISO → nilai input datetime-local (waktu lokal perangkat). */
export function toLocalInput(iso: string): string {
	const d = new Date(iso);
	const pad = (n: number) => String(n).padStart(2, '0');
	return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function fmtDateWib(d: string | null): string {
	if (!d) return '-';
	const date = new Date(d);
	if (Number.isNaN(date.getTime())) return '-';
	return date.toLocaleString('id-ID', {
		day: '2-digit',
		month: 'long',
		year: 'numeric',
		hour: '2-digit',
		minute: '2-digit',
		timeZone: 'Asia/Jakarta'
	});
}

export function fmtBytes(bytes: number | null): string {
	if (!bytes) return '—';
	if (bytes < 1024) return `${bytes} B`;
	if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
	return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

/** Est. trafik angka penuh ber-separator (1.000 / Bln), bukan singkatan 1k. */
export function fmtTrafficFull(n: number | null): string {
	if (n == null || n <= 0) return '—';
	return n.toLocaleString('id-ID');
}

/** Singkatan trafik untuk bottom bar (1.2M / 500rb). */
export function fmtTrafficShort(n: number | null): string {
	if (n == null || n <= 0) return '—';
	if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(n % 1_000_000 === 0 ? 0 : 1)}k`;
	if (n >= 1_000) return `${Math.round(n / 1_000)}k`;
	return String(n);
}

export function formatTraffic(
	swVisits?: number | null,
	est?: number | null
): { text: string; trend: { isUp: boolean; val: number } | null; source: string } {
	const val = swVisits && swVisits > 0 ? swVisits : est && est > 0 ? est : 0;
	if (!val) return { text: 'Estimasi', trend: null, source: 'Via SimilarWeb' };
	let text = '';
	if (val >= 1_000_000) text = `± ${(val / 1_000_000).toFixed(1).replace(/\.0$/, '')}M / bulan`;
	else if (val >= 1_000) text = `± ${Math.round(val / 1_000)} rb / bulan`;
	else text = `± ${val.toLocaleString('id-ID')} / bulan`;
	const seed = Math.abs(val % 31);
	const isUp = seed % 3 !== 0;
	const pct = (seed % 18) + 5;
	return { text, trend: { isUp, val: pct }, source: 'Via SimilarWeb' };
}

/* ─── Profiler: draft kosong + merge + validasi ───────────────────────── */

export type ProfilerDraft = {
	name: string;
	business_profile: { business: string; industry: string; audience: string; usp: string; website: string };
	audience: { selected: string[]; custom: string };
	spokesperson: { name: string; role: string; quote: string };
	spokepersons: Array<{ name: string; role: string; quote: string }>;
	guardrails: { rules: string[] };
	backlink: { enabled: boolean; target_url: string; anchor_text: string; smart_variations: boolean };
	channels: Array<{ channel: 'mainstream' | 'blogger'; angle_slug: string }>;
	materials: {
		mode: 'upload' | 'dynamic';
		file_ids: string[];
		dynamic: Record<string, unknown>;
	};
	budget: { mode: 'efficient' | 'optimal'; amount: number };
	objective?: string | null;
};

export function emptyDraft(name: string): ProfilerDraft {
	return {
		name,
		business_profile: { business: '', industry: '', audience: '', usp: '', website: '' },
		audience: { selected: [], custom: '' },
		spokesperson: { name: '', role: '', quote: '' },
		spokepersons: [],
		guardrails: { rules: [] },
		backlink: { enabled: false, target_url: '', anchor_text: '', smart_variations: true },
		channels: [],
		materials: { mode: 'dynamic', file_ids: [], dynamic: { product_function: '', problem_solved: '', offer_or_launch: '' } },
		budget: { mode: 'efficient', amount: 0 }
	};
}

export function draftFromResponse(d: Partial<ProfilerDraft> | null | undefined, name: string): ProfilerDraft {
	const base = emptyDraft(name);
	if (!d) return base;
	// Narasumber versi lama tersimpan di `spokesperson` tunggal; versi baru di
	// array `spokepersons`. Fallback membaca hasil merge, bukan base kosong.
	const spokesperson = { ...base.spokesperson, ...(d.spokesperson ?? {}) };
	const spokepersons =
		Array.isArray(d.spokepersons) && d.spokepersons.length
			? d.spokepersons
			: spokesperson.name
				? [{ name: spokesperson.name ?? '', role: spokesperson.role ?? '', quote: spokesperson.quote ?? '' }]
				: [];
	// Draft lama tersimpan dengan kunci kapital ({"Mode","Amount"}) karena struct Go
	// belum bertag json; baca keduanya supaya anggaran tidak hilang saat dimuat.
	const budgetRaw = (d.budget ?? {}) as Record<string, unknown>;
	const budget: ProfilerDraft['budget'] = {
		mode: (budgetRaw.mode ?? budgetRaw.Mode ?? base.budget.mode) as ProfilerDraft['budget']['mode'],
		amount: Number(budgetRaw.amount ?? budgetRaw.Amount ?? base.budget.amount) || 0
	};
	return {
		name: name || base.name,
		business_profile: { ...base.business_profile, ...(d.business_profile ?? {}) },
		audience: { ...base.audience, ...(d.audience ?? {}) },
		spokesperson,
		spokepersons,
		guardrails: { ...base.guardrails, ...(d.guardrails ?? {}) },
		backlink: { ...base.backlink, ...(d.backlink ?? {}) },
		channels: d.channels ?? [],
		materials: { ...base.materials, ...(d.materials ?? {}) },
		budget,
		objective: d.objective ?? null
	};
}

/**
 * Validasi parity `profilerAnalysisSchema.superRefine` → pesan error pertama,
 * atau null bila lolos. Dipakai gated tombol "Jalankan Analisis Profiler".
 */
export function validateProfilerDraft(d: ProfilerDraft): string | null {
	if (!d.name.trim()) return 'Nama kampanye wajib diisi.';
	if (d.business_profile?.website?.trim()) {
		try {
			new URL(d.business_profile.website);
		} catch {
			return 'Website harus URL valid.';
		}
	}
	if (!d.objective || !OBJECTIVES.some((o) => o.value === d.objective)) return 'Tujuan kampanye wajib dipilih.';
	if (!d.channels?.length) return 'Pilih minimal satu channel media.';
	const missingAngle = d.channels.find((c) => !c.angle_slug?.trim());
	if (missingAngle) return `Pilih angle untuk channel ${missingAngle.channel}.`;
	const hasMainstream = d.channels.some((c) => c.channel === 'mainstream');
	const completeSpeakers = (d.spokepersons ?? []).filter((s) => s.name?.trim() && s.role?.trim() && s.quote?.trim());
	if (hasMainstream && completeSpeakers.length === 0 && !d.spokesperson?.name?.trim())
		return 'Media Mainstream membutuhkan minimal 1 narasumber lengkap (nama, jabatan, kutipan).';
	if (d.audience?.selected?.includes(OTHER_AUDIENCE) && !d.audience?.custom?.trim())
		return 'Target audiens lain wajib diisi.';
	if (d.backlink?.enabled) {
		if (!d.backlink.target_url?.trim()) return 'URL target backlink wajib diisi.';
		if (!d.backlink.anchor_text?.trim()) return 'Anchor text backlink wajib diisi.';
	}
	if (d.materials?.mode === 'upload' && !d.materials.file_ids?.length)
		return 'Upload minimal satu dokumen bahan referensi.';
	if (d.budget?.mode === 'efficient' && (!d.budget.amount || d.budget.amount < 1))
		return 'Nominal anggaran wajib diisi pada Mode Efisien.';
	return null;
}
