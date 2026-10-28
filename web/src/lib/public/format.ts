/** Format tampilan katalog publik. */

export function idr(value: number | null | undefined): string {
	const n = Number(value ?? 0);
	return new Intl.NumberFormat('id-ID', {
		style: 'currency',
		currency: 'IDR',
		maximumFractionDigits: 0,
		roundingMode: 'trunc'
	}).format(n);
}

export function compact(value: number | null | undefined): string | null {
	const n = Number(value ?? 0);
	if (!(n > 0)) return null;
	return new Intl.NumberFormat('id-ID', { notation: 'compact', maximumFractionDigits: 1 }).format(n);
}

export function number(value: number | null | undefined): string {
	return new Intl.NumberFormat('id-ID').format(Number(value ?? 0));
}

const TIER_LABEL: Record<string, string> = {
	tier1: 'Tier 1',
	tier2: 'Tier 2',
	tier3: 'Tier 3'
};

export function tierLabel(tier: string | null | undefined): string {
	if (!tier) return 'Media';
	return TIER_LABEL[tier] ?? tier;
}

/**
 * Slug kategori untuk URL. Harus identik dengan ekspresi SQL di API
 * (`categorySlugExpr`): lower → non-alfanumerik jadi '-' → trim '-'.
 */
export function categorySlug(name: string | null | undefined): string {
	return (name ?? '')
		.toLowerCase()
		.trim()
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '');
}

export function linkText(policy: { max_links: number | null; dofollow: boolean; duration_months: number }): string {
	const duration = policy.duration_months === 0 ? 'permanen' : `${policy.duration_months} bulan`;
	if (policy.max_links === 0) return 'Tanpa backlink';
	const kind = policy.dofollow ? 'Dofollow' : 'Nofollow';
	if (policy.max_links == null) return `${kind} · ${duration}`;
	return `${policy.max_links} ${kind} · ${duration}`;
}

export function trafficText(traffic: { monthly: number | null; source: string }): string {
	const c = compact(traffic.monthly);
	if (!c) return '-';
	return `${c}/bln`;
}

export function dateText(iso: string | null | undefined): string {
	if (!iso) return '-';
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return '-';
	return new Intl.DateTimeFormat('id-ID', { day: 'numeric', month: 'long', year: 'numeric' }).format(d);
}
