/**
 * Pewarnaan badge skor AI Trust (0–100).
 *
 * Dipakai bersama oleh tabel Katalog Media dan tempat lain yang menampilkan
 * skor AI Trust, supaya nadanya konsisten: makin tinggi skor, makin kuat
 * nuansa hijaunya.
 */
export type AiTrustTone = {
	/** Warna teks/angka skor. */
	text: string;
	/** Latar lembut di belakang angka. */
	bg: string;
	/** Border tipis senada latar. */
	border: string;
};

/**
 * Palet 4 tingkat skor GEO / AI Trust (0–100), urut dari rendah ke tinggi:
 * - 0–39  (rendah)       : #D9361E
 * - 40–59 (menengah-bawah): #F5B52E
 * - 60–79 (menengah-atas) : #FFD16A
 * - 80–100 (tinggi)       : #78C841
 */
export const AI_TRUST_TONES: { min: number; hex: string; tone: AiTrustTone }[] = [
	{
		min: 80,
		hex: '#78C841',
		tone: {
			text: '#2E6B12',
			bg: 'color-mix(in srgb, #78C841 22%, #ffffff)',
			border: 'color-mix(in srgb, #78C841 62%, #ffffff)'
		}
	},
	{
		min: 60,
		hex: '#FFD16A',
		tone: {
			text: '#7A5400',
			bg: 'color-mix(in srgb, #FFD16A 38%, #ffffff)',
			border: 'color-mix(in srgb, #FFD16A 85%, #d49b1a)'
		}
	},
	{
		min: 40,
		hex: '#F5B52E',
		tone: {
			text: '#8A5A00',
			bg: 'color-mix(in srgb, #F5B52E 24%, #ffffff)',
			border: 'color-mix(in srgb, #F5B52E 65%, #ffffff)'
		}
	},
	{
		min: 0,
		hex: '#D9361E',
		tone: {
			text: '#B42814',
			bg: 'color-mix(in srgb, #D9361E 15%, #ffffff)',
			border: 'color-mix(in srgb, #D9361E 48%, #ffffff)'
		}
	}
];

/** Nada untuk satu nilai skor. `null` (belum diperiksa) → tanpa nada. */
export function aiTrustTone(score: number | string | null | undefined): AiTrustTone | null {
	const n = typeof score === 'string' ? Number(score) : score;
	if (n == null || !Number.isFinite(n)) return null;
	return AI_TRUST_TONES.find((t) => n >= t.min)?.tone ?? null;
}

/** Gaya inline siap pakai untuk badge skor; string kosong bila belum ada nilai. */
export function aiTrustToneStyle(score: number | string | null | undefined): string {
	const tone = aiTrustTone(score);
	if (!tone) return '';
	return `background-color:${tone.bg};color:${tone.text};border-color:${tone.border};`;
}
