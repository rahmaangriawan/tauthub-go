/**
 * Palet warna badge kategori media — Corporate Bold (B2B SaaS).
 *
 * 24 warna terkontrol (tint latar tipis + border tegas + teks berkontras tinggi,
 * semua ≥ 4.5:1). Warna dipilih deterministik dari nama kategori (hash FNV-1a)
 * supaya kategori yang sama selalu dapat warna yang sama, dan kategori berbeda
 * tersebar ke banyak warna — bukan menumpuk di 2-3 warna.
 */
export type CategoryTone =
	| 'charcoal' | 'slate' | 'stone' | 'navy' | 'cobalt' | 'blue'
	| 'sky' | 'cyan' | 'teal' | 'emerald' | 'green' | 'lime'
	| 'gold' | 'amber' | 'orange' | 'coral' | 'red' | 'burgundy'
	| 'pink' | 'rose' | 'fuchsia' | 'purple' | 'violet' | 'indigo';

export const CATEGORY_TONES: CategoryTone[] = [
	'charcoal', 'slate', 'stone', 'navy', 'cobalt', 'blue',
	'sky', 'cyan', 'teal', 'emerald', 'green', 'lime',
	'gold', 'amber', 'orange', 'coral', 'red', 'burgundy',
	'pink', 'rose', 'fuchsia', 'purple', 'violet', 'indigo'
];

/** Warna tetap per kategori yang paling sering dipakai (biar konsisten maknanya). */
const PINNED: Record<string, CategoryTone> = {
	bisnis: 'emerald',
	ekonomi: 'teal',
	berita: 'charcoal',
	nasional: 'navy',
	teknologi: 'cobalt',
	umum: 'slate'
};

export function getCategoryTone(category: string | null | undefined): CategoryTone {
	if (!category) return 'charcoal';
	const c = category.toLowerCase().trim();
	if (PINNED[c]) return PINNED[c];
	// Hash deterministik (FNV-1a 32-bit) + finalizer murmur agar sebarannya
	// merata untuk nama-nama pendek yang mirip (mis. "bisnis"/"Bisnis").
	let hash = 0x811c9dc5;
	for (let i = 0; i < c.length; i++) {
		hash ^= c.charCodeAt(i);
		hash = Math.imul(hash, 0x01000193) >>> 0;
	}
	hash ^= hash >>> 16;
	hash = Math.imul(hash, 0x7feb352d) >>> 0;
	hash ^= hash >>> 15;
	hash = Math.imul(hash, 0x846ca68b) >>> 0;
	hash ^= hash >>> 16;
	return CATEGORY_TONES[hash % CATEGORY_TONES.length];
}
