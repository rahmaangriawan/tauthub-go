/**
 * Statistik distribusi artikel per media.
 *
 * Sumber utama `published_count` dari API (dihitung langsung dari tabel
 * publications), dengan `health_breakdown.published` sebagai cadangan untuk
 * respons lama. `known` false berarti tidak ada angka sama sekali, jadi UI
 * sebaiknya tidak menampilkan apa-apa.
 */
export type MediaDistribution = {
	published: number;
	rejected: number;
	failed: number;
	known: boolean;
};

export function mediaDistribution(
	breakdown: Record<string, unknown> | null | undefined,
	publishedCount?: number | null
): MediaDistribution {
	const b = breakdown ?? {};
	const num = (v: unknown) => (typeof v === 'number' && Number.isFinite(v) ? v : 0);
	const liveCount = typeof publishedCount === 'number' && Number.isFinite(publishedCount) ? publishedCount : null;
	const fromBreakdown = typeof b.published === 'number' ? num(b.published) : null;
	return {
		published: liveCount ?? fromBreakdown ?? 0,
		rejected: num(b.rejected),
		failed: num(b.failed),
		known: liveCount !== null || fromBreakdown !== null
	};
}
