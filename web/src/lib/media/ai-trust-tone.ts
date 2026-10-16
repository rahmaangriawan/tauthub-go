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

export const AI_TRUST_TONES: { min: number; tone: AiTrustTone }[] = [
	{ min: 80, tone: { text: '#15803D', bg: '#DCFCE7', border: '#86EFAC' } },
	{ min: 60, tone: { text: '#16A34A', bg: '#F0FDF4', border: '#BBF7D0' } },
	{ min: 40, tone: { text: '#D97706', bg: '#FFFBEB', border: '#FDE68A' } },
	{ min: 0, tone: { text: '#DC2626', bg: '#FEF2F2', border: '#FECACA' } }
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
