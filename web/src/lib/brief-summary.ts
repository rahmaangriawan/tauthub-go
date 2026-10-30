// Parser ringkasan Brief (entitas) → struktur yang layak dibaca.
//
// Model peringkas menulis ringkasan sebagai dump `field_teknis: nilai` dengan
// judul bagian berupa baris pendek ("- Identitas", "Fakta resmi"). Dirender apa
// adanya lewat markdown, hasilnya terbaca seperti log: nama field snake_case
// bercampur nilai, daftar klaim menempel dalam satu paragraf.
//
// Parser ini mengubah teks itu jadi bagian + baris label/nilai. Semua bentuk
// yang pernah muncul tetap dikenali:
//   - judul: `## Identitas`, `**Identitas**`, `- Identitas`, atau baris pendek;
//   - field: `canonical_name: …`, `**Nama kanonik:** …`, `Nama kanonik: …`;
//   - nilai bersambung: baris bullet setelah field tanpa nilai;
//   - nilai berisi beberapa pasangan `Label: nilai.` (mis. fakta resmi).
//
// Tidak ada teks yang dibuang: baris yang tidak dikenali tetap jadi paragraf.

export type BriefField = {
	label: string;
	/** Nilai satu baris; kosong bila isinya daftar (`items`). */
	value?: string;
	/** Butir daftar untuk field yang isinya beberapa poin. */
	items?: string[];
};

export type BriefBlock =
	| { type: 'p'; text: string }
	| { type: 'ul'; items: string[] }
	| { type: 'fields'; rows: BriefField[] };

export type BriefSection = { title: string; blocks: BriefBlock[] };
export type BriefSummaryDoc = { sections: BriefSection[] };

/** Label manusia untuk field teknis yang lazim dipakai peringkas. */
const KEY_LABELS: Record<string, string> = {
	canonical_name: 'Nama kanonik',
	entity_category: 'Kategori entitas',
	canonical_description: 'Deskripsi kanonik',
	official_facts: 'Fakta resmi',
	allowed_claims: 'Klaim yang boleh dipakai',
	forbidden_claims: 'Klaim yang dilarang',
	forbidden_words: 'Kata terlarang',
	boilerplate: 'Boilerplate resmi',
	canonical_links: 'Tautan kanonik',
	target_keywords_urls: 'Kata kunci & URL target',
	target_questions: 'Pertanyaan target',
	brand_name: 'Nama brand',
	industry: 'Industri',
	products: 'Produk / menu',
	audience: 'Target audiens',
	usp: 'Keunggulan (USP)',
	region: 'Wilayah',
	website: 'Website',
	tone: 'Nada penulisan',
	formality: 'Tingkat formalitas',
	preferred_terms: 'Istilah yang dipakai',
	forbidden_terms: 'Istilah yang dihindari',
	primary_keyword: 'Kata kunci utama',
	required_topics: 'Topik wajib',
	forbidden_topics: 'Topik terlarang',
	allowed_cta: 'CTA yang boleh',
	spokesperson: 'Juru bicara',
	objective: 'Tujuan',
	notes: 'Catatan',
	sources: 'Sumber'
};

/** Ubah `snake_case`/`kebab-case` jadi label kalimat. */
export function humanizeFieldKey(raw: string): string {
	const key = raw.trim().toLowerCase().replace(/[\s-]+/g, '_');
	if (KEY_LABELS[key]) return KEY_LABELS[key];
	const spaced = raw.replace(/[_-]+/g, ' ').trim();
	return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

const reHeadingHash = /^#{1,6}\s+(.+?)\s*#*$/;
const reBoldLine = /^\*\*(.+?)\*\*:?\s*(.*)$/;
const reBullet = /^[-*•]\s+(.+)$/;
// Label field sengaja dibatasi tanpa tanda kurung/titik supaya kalimat seperti
// "Cocok untuk rombongan (sumber: website resmi)" tidak terbaca sebagai field.
const reField = /^([A-Za-z][A-Za-z0-9 _/-]{0,38}):\s*(.*)$/;
const reUrl = /https?:\/\/[^\s)<>"']+/g;
// Baris yang hanya berisi penanda bullet/kosong (mis. "-" hasil konversi
// markdown) tidak boleh jadi judul bagian.
const reOnlyDecoration = /^[-*•_—–\s]+$/;

/** Baris pendek tanpa titik dua yang berfungsi sebagai judul bagian. */
function looksLikeHeading(text: string): boolean {
	const t = text.trim();
	if (!t || t.length > 52) return false;
	if (t.includes(':') || /https?:\/\//i.test(t)) return false;
	if (/[.!?]$/.test(t)) return false;
	return t.split(/\s+/).length <= 5;
}

/**
 * Apakah baris ini memulai FIELD BARU (bukan butir lanjutan)? Dipakai saat ada
 * field daftar yang masih terbuka: `forbidden_claims: …` harus menutup
 * `allowed_claims`, sedangkan `Instagram: https://…` tetap jadi butir tautan.
 * Penandanya: label ber-underscore (nama field teknis) atau label yang memang
 * sudah dikenal sebagai field.
 */
function startsNewField(text: string): boolean {
	const m = text.match(reField);
	if (!m) return false;
	const label = m[1].trim();
	if (label.includes('_')) return true;
	const key = label.toLowerCase().replace(/[\s-]+/g, '_');
	return key in KEY_LABELS;
}

function stripBullet(line: string): string {
	const m = line.trim().match(reBullet);
	return m ? m[1].trim() : line.trim();
}

function newSection(title: string): BriefSection {
	return { title, blocks: [] };
}

/**
 * Paragraf baru. Baris yang dimulai huruf kecil dianggap lanjutan kalimat yang
 * terpotong (wrap), jadi digabung ke paragraf sebelumnya; baris berhuruf besar
 * selalu jadi paragraf sendiri supaya butir USP tidak menumpuk jadi satu blok.
 */
function pushParagraph(section: BriefSection, text: string) {
	const last = section.blocks[section.blocks.length - 1];
	if (last && last.type === 'p' && /^[a-z]/.test(text)) {
		last.text += ' ' + text;
		return;
	}
	section.blocks.push({ type: 'p', text });
}

function pushBullet(section: BriefSection, text: string) {
	const last = section.blocks[section.blocks.length - 1];
	if (last && last.type === 'ul') {
		last.items.push(text);
		return;
	}
	section.blocks.push({ type: 'ul', items: [text] });
}

/**
 * Pisahkan nilai `Tahun berdiri: 2018. Kapasitas: 1.000 orang.` jadi pasangan
 * label/nilai. Kosong bila nilainya bukan rangkaian pasangan seperti itu.
 */
export function splitFactPairs(value: string): Array<{ label: string; value: string }> {
	const parts = value
		.split(/\.\s+(?=[A-Za-z][^.:]{0,40}:)/)
		.map((s) => s.replace(/\.\s*$/, '').trim())
		.filter(Boolean);
	if (parts.length < 2) return [];
	const out: Array<{ label: string; value: string }> = [];
	for (const part of parts) {
		const m = part.match(/^([^:]{2,40}):\s*(.+)$/);
		if (!m) return [];
		out.push({ label: m[1].trim(), value: m[2].trim() });
	}
	return out;
}

/** Pecah teks jadi potongan teks/tautan agar URL bisa dirender sebagai anchor. */
export function linkify(text: string): Array<{ text: string; href?: string }> {
	const out: Array<{ text: string; href?: string }> = [];
	let last = 0;
	for (const m of text.matchAll(reUrl)) {
		const start = m.index ?? 0;
		if (start > last) out.push({ text: text.slice(last, start) });
		const href = m[0].replace(/[.,;]+$/, '');
		out.push({ text: href, href });
		last = start + href.length;
	}
	if (last < text.length) out.push({ text: text.slice(last) });
	return out.length > 0 ? out : [{ text }];
}

/**
 * Parse teks ringkasan jadi bagian-bagian. Selalu mengembalikan minimal satu
 * bagian (judul kosong) supaya pemanggil tidak perlu menangani kasus kosong.
 */
export function parseBriefSummary(raw: string): BriefSummaryDoc {
	const lines = (raw ?? '').split(/\r?\n/);
	const sections: BriefSection[] = [];
	let section = newSection('');
	sections.push(section);
	// Field terakhir yang masih boleh menampung butir lanjutan.
	let openField: BriefField | null = null;

	const closeField = () => {
		openField = null;
	};
	const startSection = (title: string) => {
		closeField();
		if (section.blocks.length === 0 && section.title === '') {
			section.title = title;
			return;
		}
		section = newSection(title);
		sections.push(section);
	};

	for (let i = 0; i < lines.length; i++) {
		const line = lines[i];
		const trimmed = line.trim();
		if (!trimmed) {
			closeField();
			continue;
		}
		// Baris hanya-penanda ("-", "---", "•") hasil konversi markdown: bukan
		// judul, bukan isi. Dilewati tanpa menutup field aktif.
		if (reOnlyDecoration.test(trimmed)) continue;

		// Judul eksplisit: `## Judul` atau `**Judul**` tanpa isi.
		const hash = trimmed.match(reHeadingHash);
		if (hash) {
			startSection(hash[1].trim());
			continue;
		}
		const bold = trimmed.match(reBoldLine);
		if (bold) {
			const label = bold[1].trim();
			const rest = (bold[2] ?? '').trim();
			if (!rest) {
				startSection(label);
				continue;
			}
			const field: BriefField = { label: humanizeFieldKey(label.replace(/:$/, '')) };
			if (rest.startsWith('- ') || rest.startsWith('• ')) {
				field.items = [stripBullet(rest)];
			} else {
				field.value = rest;
			}
			section.blocks.push({ type: 'fields', rows: [field] });
			openField = field.items ? field : null;
			continue;
		}

		// Bullet.
		const bullet = trimmed.match(reBullet);
		if (bullet) {
			let body = bullet[1].trim();
			// Label tebal di dalam butir: `- **Nama resmi:** Salsari Resto`
			// dinormalkan dulu supaya ikut jalur field yang sama.
			const boldInBullet = body.match(reBoldLine);
			if (boldInBullet) {
				const label = boldInBullet[1].trim().replace(/:$/, '');
				const rest = (boldInBullet[2] ?? '').trim();
				body = rest ? `${label}: ${rest}` : label;
			}
			// `- Identitas` sebagai judul bagian hanya bila bagian ini masih kosong;
			// butir pendek setelah ada isi tetap butir (mis. "Ramah anak").
			if (!body.includes(':') && looksLikeHeading(body) && section.blocks.length === 0) {
				startSection(body);
				continue;
			}
			// Butir lanjutan untuk field yang nilainya daftar — kecuali baris itu
			// jelas memulai field baru (`forbidden_claims: …`).
			if (openField && !startsNewField(body)) {
				openField.items = [...(openField.items ?? []), body];
				continue;
			}
			// `- Website: https://…` di dalam bagian tautan → baris field.
			const fieldMatch = body.match(reField);
			if (fieldMatch && fieldMatch[2].trim() !== '') {
				const field: BriefField = {
					label: humanizeFieldKey(fieldMatch[1]),
					value: fieldMatch[2].trim()
				};
				section.blocks.push({ type: 'fields', rows: [field] });
				openField = null;
				continue;
			}
			closeField();
			pushBullet(section, body);
			continue;
		}

		// Baris pendek tanpa titik dua = judul bagian (mis. "Boilerplate").
		const next = lines.slice(i + 1).find((l) => l.trim() !== '' && !reOnlyDecoration.test(l.trim())) ?? '';
		const isHeading = !trimmed.includes(':') && looksLikeHeading(trimmed) && next.trim() !== '';

		// Nilai field yang masih terbuka menampung baris lanjutan (butir daftar
		// atau deskripsi panjang yang terpotong wrap) sebelum aturan lain.
		if (openField && !isHeading && !startsNewField(trimmed)) {
			if (openField.items) {
				openField.items = [...openField.items, trimmed];
			} else if (openField.value) {
				openField.value += ' ' + trimmed;
			}
			continue;
		}

		// Field `key: value` atau `Label: value`.
		const fieldMatch = trimmed.match(reField);
		if (fieldMatch) {
			const labelRaw = fieldMatch[1].trim();
			const rest = fieldMatch[2].trim();
			const field: BriefField = { label: humanizeFieldKey(labelRaw) };
			if (rest.startsWith('- ') || rest.startsWith('• ')) {
				field.items = [stripBullet(rest)];
			} else if (rest) {
				field.value = rest;
			}
			section.blocks.push({ type: 'fields', rows: [field] });
			// Nilai panjang biasanya deskripsi yang berlanjut di baris berikutnya;
			// nilai pendek sudah lengkap, jadi field ditutup.
			openField = field.items || (field.value ?? '').length >= 120 ? field : null;
			continue;
		}

		if (isHeading) {
			startSection(trimmed.replace(/^[-*•]\s+/, ''));
			continue;
		}

		closeField();
		pushParagraph(section, trimmed);
	}

	// Buang bagian yang benar-benar kosong (mis. hanya judul tanpa isi tetap
	// dipertahankan; yang dibuang hanya bagian tanpa judul dan tanpa isi).
	return { sections: sections.filter((s) => s.title !== '' || s.blocks.length > 0) };
}
