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

export type StandardWordCountMode = (typeof WORD_COUNT_MODES)[number]['value'];
export type WordCountMode = StandardWordCountMode | string;

export const DEFAULT_WORD_COUNT_MODE: StandardWordCountMode = 'sesuai_permintaan';

const BY_VALUE = new Map<string, (typeof WORD_COUNT_MODES)[number]>(
	WORD_COUNT_MODES.map((m) => [m.value, m])
);

/** Parse format custom_{min}_{max} menjadi objek { min, max } */
export function parseCustomWordCount(mode: unknown): { min: number; max: number } | null {
	if (typeof mode !== 'string') return null;
	const trimmed = mode.trim();
	if (!trimmed.startsWith('custom_')) return null;
	const parts = trimmed.slice(7).split('_');
	if (parts.length !== 2) return null;
	const min = Number(parts[0]);
	const max = Number(parts[1]);
	if (!Number.isFinite(min) || !Number.isFinite(max) || min <= 0 || max < min) return null;
	return { min, max };
}

export function formatCustomWordCount(min: number, max: number): string {
	const safeMin = Math.max(50, Math.floor(min) || 500);
	const safeMax = Math.max(safeMin, Math.floor(max) || 1000);
	return `custom_${safeMin}_${safeMax}`;
}

export function normalizeWordCountMode(value: unknown): string {
	const raw = typeof value === 'string' ? value.trim() : '';
	if (BY_VALUE.has(raw)) return raw;
	if (parseCustomWordCount(raw) != null) return raw;
	return DEFAULT_WORD_COUNT_MODE;
}

export function resolveWordRange(mode: unknown): { min: number; max: number } | null {
	const custom = parseCustomWordCount(mode);
	if (custom) return custom;
	const entry = BY_VALUE.get(normalizeWordCountMode(mode));
	if (!entry || entry.min == null || entry.max == null) return null;
	return { min: entry.min, max: entry.max };
}

export function wordCountLabel(mode: unknown): string {
	const custom = parseCustomWordCount(mode);
	if (custom) {
		return `${custom.min.toLocaleString('id-ID')}–${custom.max.toLocaleString('id-ID')} kata`;
	}
	return BY_VALUE.get(normalizeWordCountMode(mode))?.short ?? WORD_COUNT_MODES[0].short;
}
