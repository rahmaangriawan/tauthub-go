/**
 * Jumlah kata per media — setelan mitra yang menentukan panjang artikel yang
 * dihasilkan The Writer untuk media itu.
 *
 * Satu sumber kebenaran untuk tiga pemakai: dialog mitra (pilihan), popup
 * katalog klien (tampilan), dan prompt The Writer (aturan panjang).
 */

export const WORD_COUNT_MODES = [
	{
		value: 'sesuai_permintaan',
		label: 'Sesuai permintaan klien (bebas)',
		short: 'Sesuai Permintaan Klien',
		min: null,
		max: null
	},
	{
		value: 'kata_300_700',
		label: '300–700 kata',
		short: '300–700 kata',
		min: 300,
		max: 700
	},
	{
		value: 'kata_800_1500',
		label: '800–1500 kata',
		short: '800–1500 kata',
		min: 800,
		max: 1500
	}
] as const;

export type WordCountMode = (typeof WORD_COUNT_MODES)[number]['value'];

export const DEFAULT_WORD_COUNT_MODE: WordCountMode = 'sesuai_permintaan';

const BY_VALUE = new Map<string, (typeof WORD_COUNT_MODES)[number]>(
	WORD_COUNT_MODES.map((m) => [m.value, m])
);

export function normalizeWordCountMode(value: unknown): WordCountMode {
	const raw = typeof value === 'string' ? value.trim() : '';
	return BY_VALUE.has(raw) ? (raw as WordCountMode) : DEFAULT_WORD_COUNT_MODE;
}

export function resolveWordRange(mode: unknown): { min: number; max: number } | null {
	const entry = BY_VALUE.get(normalizeWordCountMode(mode));
	if (!entry || entry.min == null || entry.max == null) return null;
	return { min: entry.min, max: entry.max };
}

export function wordCountLabel(mode: unknown): string {
	return BY_VALUE.get(normalizeWordCountMode(mode))?.short ?? WORD_COUNT_MODES[0].short;
}
