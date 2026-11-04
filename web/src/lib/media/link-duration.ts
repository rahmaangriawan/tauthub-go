/**
 * Masa berlaku tautan (backlink) per media.
 *
 * Kebijakan tiap media berbeda: sebagian menjamin tautannya permanen (tidak
 * pernah dilepas), sebagian hanya tahan sekian bulan — umumnya 12 bulan
 * ("12 Bln SLA"). Kolom `media.link_duration_months` menyimpan angkanya:
 * 0 = permanen, >0 = jumlah bulan sejak tanggal tayang.
 *
 * Satu sumber kebenaran untuk label di form mitra, form admin, badge katalog
 * klien, popup detail media, dan kartu media di The Writer. Angka ini juga yang
 * dipakai server untuk masa pemantauan 404 dan masa garansi klaim Link Guardian.
 */

export const LINK_DURATION_OPTIONS = [
	{ value: 0, label: 'Permanen — Tautan Tidak Pernah Dilepas', short: 'Permanen' },
	{ value: 6, label: '6 Bulan', short: '6 Bln SLA' },
	{ value: 12, label: '12 Bulan', short: '12 Bln SLA' },
	{ value: 24, label: '24 Bulan', short: '24 Bln SLA' },
	{ value: 36, label: '36 Bulan', short: '36 Bln SLA' }
] as const;

export const DEFAULT_LINK_DURATION_MONTHS = 0;

/** Nilai apa pun dari API → bulan yang wajar (0 tetap 0 = permanen). */
export function normalizeLinkDurationMonths(value: unknown): number {
	if (value === null || value === undefined || value === '') return DEFAULT_LINK_DURATION_MONTHS;
	const n = Number(value);
	if (!Number.isFinite(n)) return DEFAULT_LINK_DURATION_MONTHS;
	const months = Math.trunc(n);
	return months < 0 || months > 120 ? DEFAULT_LINK_DURATION_MONTHS : months;
}

export function isPermanentLink(value: unknown): boolean {
	return normalizeLinkDurationMonths(value) === 0;
}

/** Label pendek untuk badge: "Permanen" atau "12 Bln SLA". */
export function linkDurationBadge(value: unknown): string {
	const months = normalizeLinkDurationMonths(value);
	return months === 0 ? 'Permanen' : `${months} Bln SLA`;
}

/** Kalimat panjang untuk baris parameter/laporan media. */
export function linkDurationLong(value: unknown): string {
	const months = normalizeLinkDurationMonths(value);
	if (months === 0) return 'Permanen — tautan tidak pernah dilepas';
	return `${months} bulan sejak artikel tayang`;
}

/**
 * Badge kebijakan tautan lengkap: "2 Dofollow • Permanen" / "1 Nofollow • 12 Bln SLA".
 * Jumlahnya mengikuti kuota tautan media (max_links); 0 = tanpa backlink.
 */
export function linkPolicyBadge(input: {
	max_links?: number | null;
	allow_hyperlink?: boolean | null;
	/** Rel tautan (dofollow | nofollow | sponsored); terpisah dari label sponsor. */
	link_rel?: string | null;
	link_duration_months?: number | null;
}): string {
	const duration = linkDurationBadge(input.link_duration_months);
	const max = input.max_links;
	if (max === 0) return 'Tanpa backlink';
	if (max === null || max === undefined) return `Backlink • ${duration}`;
	const kind =
		input.link_rel === 'sponsored'
			? 'Sponsored'
			: input.link_rel === 'nofollow' || input.allow_hyperlink === false
				? 'Nofollow'
				: 'Dofollow';
	return `${max} ${kind} • ${duration}`;
}
