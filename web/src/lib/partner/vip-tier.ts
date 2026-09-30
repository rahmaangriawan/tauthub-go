/**
 * Label + warna tier keanggotaan mitra (kolom partners.vip_tier).
 * Nilai database: "regular" | "vip_reguler" | "brand_ambassador".
 *
 * Dipakai di dua tempat: kartu tier di halaman VIP mitra, dan pill di samping
 * lonceng header portal mitra. Satu sumber supaya label dan warnanya sama.
 */
import { tr } from '$lib/i18n/phrases';

type TierTone = { bg: string; fg: string; border: string };

const TONES: Record<string, TierTone> = {
	regular: { bg: 'var(--muted)', fg: 'var(--muted-foreground)', border: 'var(--border)' },
	vip_reguler: {
		bg: 'var(--warning-soft)',
		fg: 'var(--warning)',
		border: 'color-mix(in srgb, var(--warning) 35%, transparent)'
	},
	brand_ambassador: {
		bg: 'var(--accent)',
		fg: 'var(--accent-foreground)',
		border: 'color-mix(in srgb, var(--accent-foreground) 35%, transparent)'
	}
};

const LABELS: Record<string, () => string> = {
	regular: () => tr('Reguler'),
	vip_reguler: () => tr('VIP Reguler'),
	brand_ambassador: () => tr('Brand Ambassador')
};

const NEUTRAL: TierTone = { bg: 'var(--muted)', fg: 'var(--muted-foreground)', border: 'var(--border)' };

/** Tier punya warna khusus (bukan tier reguler)? */
export function isVipTier(tier: string | null | undefined): boolean {
	const t = (tier ?? '').trim();
	return t !== '' && t !== 'regular';
}

/** Label bahasa manusia untuk tier; nilai asing ditampilkan apa adanya. */
export function vipTierLabel(tier: string | null | undefined): string {
	const t = (tier ?? '').trim();
	return LABELS[t]?.() ?? (t || tr('Reguler'));
}

/** Badge tier: label + warna, siap dipakai di markup. */
export function vipTierBadge(tier: string | null | undefined): { label: string } & TierTone {
	const t = (tier ?? '').trim();
	return { label: vipTierLabel(t), ...(TONES[t] ?? NEUTRAL) };
}

/** Tier tertinggi VVIP (nilai database: brand_ambassador)? */
export function isVvipTier(tier: string | null | undefined): boolean {
	return (tier ?? '').trim() === 'brand_ambassador';
}
