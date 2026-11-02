/**
 * Nada warna skor kesehatan media (0-100). Nilai status tidak ditulis sebagai
 * teks; komponen memakai warna + aria-label untuk pembaca layar.
 */
export type HealthTone = 'good' | 'warn' | 'bad' | 'unknown';

export function healthTone(score: number | null | undefined): HealthTone {
	const n = typeof score === 'string' ? Number(score) : score;
	if (n == null || !Number.isFinite(n)) return 'unknown';
	if (n >= 70) return 'good';
	if (n >= 40) return 'warn';
	return 'bad';
}

/** Gaya inline siap pakai untuk badge skor kesehatan. */
export function healthToneStyle(score: number | null | undefined): string {
	switch (healthTone(score)) {
		case 'good':
			return 'background: var(--success-soft, #eaf8f1); color: var(--success, #16794b); border-color: color-mix(in srgb, var(--success, #16794b) 25%, transparent);';
		case 'warn':
			return 'background: var(--warning-soft, #fff6df); color: var(--warning, #a15c00); border-color: color-mix(in srgb, var(--warning, #a15c00) 25%, transparent);';
		case 'bad':
			return 'background: color-mix(in srgb, var(--destructive, #c52b36) 12%, transparent); color: var(--destructive, #c52b36); border-color: color-mix(in srgb, var(--destructive, #c52b36) 30%, transparent);';
		default:
			return 'background: var(--muted, #f1f5f9); color: var(--muted-foreground, #667085); border-color: var(--border, #dfe5ef);';
	}
}
