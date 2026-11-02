/**
 * Estimasi trafik bulanan — dipakai katalog klien dan tab Transparansi mitra
 * supaya angka & formatnya identik. SimilarWeb menang bila ada; jika tidak,
 * pakai estimasi internal. Tanpa data tulis "0" (bukan strip).
 */
export function formatTraffic(
	swVisits?: number | null,
	est?: number | null
): { text: string; source: string } {
	const val = swVisits && swVisits > 0 ? swVisits : est && est > 0 ? est : 0;
	if (!val) return { text: '0', source: '' };
	let text: string;
	if (val >= 1_000_000) text = `± ${(val / 1_000_000).toFixed(1).replace(/\.0$/, '')}M / bulan`;
	else if (val >= 1_000) text = `± ${Math.round(val / 1_000)} rb / bulan`;
	else text = `± ${val.toLocaleString('id-ID')} / bulan`;
	return { text, source: 'Via SimilarWeb' };
}
