// Parser changelog: web/static/changelog.md → entri siap impor ke API
// (POST /v1/platform/changelog/import). Sengaja tanpa dependensi apa pun dan
// tanpa alias $lib supaya bisa diuji langsung dengan `bun changelog.ts`.
//
// Format yang dibaca (lihat kepala file changelog.md):
//   ## v0.3.0 — 2026-10-07   pembuka satu versi
//   ### Fitur                kategori: fitur | perbaikan | performa | lainnya
//   - satu poin perubahan
// Baris di luar pola itu (termasuk komentar HTML) diabaikan. Kombinasi
// versi + kategori = satu entri dengan kunci "versi|kategori".

export type ChangelogCategory = 'fitur' | 'perbaikan' | 'performa' | 'lainnya';

export type ParsedChangelogEntry = {
	source_key: string;
	version: string;
	released_on: string;
	category: ChangelogCategory;
	title: string;
	summary: string;
	body: string;
};

export const CATEGORY_LABEL: Record<ChangelogCategory, string> = {
	fitur: 'Fitur',
	perbaikan: 'Perbaikan',
	performa: 'Performa',
	lainnya: 'Lainnya'
};

const SUMMARY_MAX = 300;

/** Label kategori untuk sembarang string (aman untuk data dari API). */
export function categoryLabel(category: string): string {
	return CATEGORY_LABEL[category as ChangelogCategory] ?? category;
}

function escapeHtml(text: string): string {
	return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

/** Escape HTML lalu dukung **tebal** dan `kode` sebaris. */
function inlineHtml(text: string): string {
	return escapeHtml(text)
		.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
		.replace(/`([^`]+)`/g, '<code>$1</code>');
}

function normalizeCategory(raw: string): ChangelogCategory {
	const key = raw.trim().toLowerCase();
	if (key === 'fitur' || key === 'features' || key === 'feature') return 'fitur';
	if (key === 'perbaikan' || key === 'fix' || key === 'fixes' || key === 'perbaikan bug') return 'perbaikan';
	if (key === 'performa' || key === 'performance' || key === 'perf') return 'performa';
	return 'lainnya';
}

function truncate(text: string, max: number): string {
	const runes = [...text];
	return runes.length > max ? runes.slice(0, max).join('') : text;
}

type Bucket = { category: ChangelogCategory; bullets: string[]; paragraphs: string[] };
type VersionBlock = { version: string; releasedOn: string; buckets: Bucket[] };

/** Parse isi changelog.md menjadi daftar entri (urut seperti di file). */
export function parseChangelog(markdown: string): ParsedChangelogEntry[] {
	const withoutComments = markdown.replace(/<!--[\s\S]*?-->/g, '');
	const lines = withoutComments.split(/\r?\n/);

	const blocks: VersionBlock[] = [];
	let block: VersionBlock | null = null;
	let bucket: Bucket | null = null;

	/** Buka (atau buat) bucket kategori pada versi yang sedang aktif. */
	const openBucket = (category: ChangelogCategory): Bucket => {
		const existing = block?.buckets.find((b) => b.category === category);
		if (existing) return existing;
		const fresh: Bucket = { category, bullets: [], paragraphs: [] };
		block?.buckets.push(fresh);
		return fresh;
	};

	for (const rawLine of lines) {
		const line = rawLine.trim();
		if (!line) continue;

		const version = /^##\s+(.+?)\s+[—–-]\s+(\d{4}-\d{2}-\d{2})\s*$/.exec(line);
		if (version) {
			block = { version: version[1].trim(), releasedOn: version[2], buckets: [] };
			blocks.push(block);
			bucket = null;
			continue;
		}

		const category = /^###\s+(.+?)\s*$/.exec(line);
		if (category) {
			bucket = openBucket(normalizeCategory(category[1]));
			continue;
		}

		const bullet = /^[-*]\s+(.+)$/.exec(line);
		if (bullet) {
			if (!block) continue;
			bucket = bucket ?? openBucket('lainnya');
			bucket.bullets.push(bullet[1].trim());
			continue;
		}

		if (!block) continue;
		bucket = bucket ?? openBucket('lainnya');
		bucket.paragraphs.push(line);
	}

	const entries: ParsedChangelogEntry[] = [];
	for (const b of blocks) {
		for (const item of b.buckets) {
			if (item.bullets.length === 0 && item.paragraphs.length === 0) continue;
			const bodyParts: string[] = [];
			if (item.bullets.length) {
				bodyParts.push(`<ul>${item.bullets.map((t) => `<li>${inlineHtml(t)}</li>`).join('')}</ul>`);
			}
			for (const p of item.paragraphs) bodyParts.push(`<p>${inlineHtml(p)}</p>`);
			const summarySource = item.bullets.length ? item.bullets.join(' · ') : item.paragraphs.join(' ');
			entries.push({
				source_key: `${b.version}|${item.category}`,
				version: b.version,
				released_on: b.releasedOn,
				category: item.category,
				title: `${b.version} — ${CATEGORY_LABEL[item.category]}`,
				summary: truncate(summarySource, SUMMARY_MAX),
				body: bodyParts.join('')
			});
		}
	}
	return entries;
}
