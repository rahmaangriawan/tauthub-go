/**
 * Palet warna badge kategori media (kustom, disebar acak-deterministik via hash).
 *
 * Daftar hex utama + cadangan disimpan di `CATEGORY_HEX_PALETTE` supaya kalau
 * ke depannya ada kategori baru, otomatis langsung kebagian warna dari pool ini.
 */
export const CATEGORY_HEX_PALETTE = [
	'#2373F4', // cobalt
	'#578EF5', // blue
	'#65D0F4', // sky
	'#F2F7A0', // lime
	'#006199', // navy
	'#8ACFF8', // cyan
	'#F4EB6C', // gold
	'#FFD444', // amber
	'#2196F3', // slate
	'#FF7F3E', // orange
	'#AC87C5', // purple
	'#756AB6', // indigo
	'#3F979B', // teal
	'#91D8E4', // stone
	'#570A57', // burgundy
	'#42DEE1', // emerald
	'#F18A9B', // rose
	'#FFB480', // coral
	'#A26EA1', // fuchsia
	'#FFCEF3', // pink
	'#C9FDD7', // green
	'#6892D5', // violet
	'#F4EB6C', // charcoal (cadangan)
	'#2373F4'  // red (cadangan)
] as const;

export type CategoryTone =
	| 'charcoal' | 'slate' | 'stone' | 'navy' | 'cobalt' | 'blue'
	| 'sky' | 'cyan' | 'teal' | 'emerald' | 'green' | 'lime'
	| 'gold' | 'amber' | 'orange' | 'coral' | 'red' | 'burgundy'
	| 'pink' | 'rose' | 'fuchsia' | 'purple' | 'violet' | 'indigo';

export const CATEGORY_TONES: CategoryTone[] = [
	'cobalt', 'blue', 'sky', 'lime', 'navy', 'cyan',
	'gold', 'amber', 'slate', 'orange', 'purple', 'indigo',
	'teal', 'stone', 'burgundy', 'emerald', 'rose', 'coral',
	'fuchsia', 'pink', 'green', 'violet', 'charcoal', 'red'
];

export function getCategoryTone(category: string | null | undefined): CategoryTone {
	if (!category) return 'cobalt';
	const c = category.toLowerCase().trim();
	// Hash deterministik (FNV-1a 32-bit) + finalizer murmur agar tiap kategori
	// mendapat warna acak yang konsisten dari pool 22+ warna di atas.
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
