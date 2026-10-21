/**
 * SLA waktu terbit per media.
 *
 * Nilainya BUKAN taksiran dari jenis integrasi, melainkan rata-rata waktu
 * penerbitan NYATA media tersebut — diambil dari `health_breakdown.avg_publish_hours`
 * yang dihitung sistem dari riwayat publikasi (lama waktu dari artikel dikirim
 * sampai tayang). Media yang biasanya terbit 1 jam setelah artikel dikirim akan
 * tampil "SLA ± 1 jam".
 */

export type SlaBucket = 'super' | 'fast' | 'standard';

type MediaLike = {
	health_breakdown?: Record<string, unknown> | null;
} | null | undefined;

/** Rata-rata jam terbit nyata; null bila belum ada riwayat yang cukup. */
export function avgPublishHours(media: MediaLike): number | null {
	const raw = media?.health_breakdown?.avg_publish_hours;
	if (raw === null || raw === undefined || raw === '') return null;
	const hours = typeof raw === 'number' ? raw : Number(raw);
	return Number.isFinite(hours) && hours > 0 ? hours : null;
}

/**
 * Label SLA ringkas untuk sel tabel:
 * "SLA ± <1 jam" / "SLA ± 6 jam" / "SLA ± 4,2 hari" / "—" bila belum ada data.
 */
export function formatSla(hours: number | null): string {
	if (hours == null) return '—';
	if (hours < 1) return 'SLA ± <1 jam';
	if (hours < 24) return `SLA ± ${Math.max(1, Math.round(hours))} jam`;
	const days = Math.round((hours / 24) * 10) / 10;
	const daysText = Number.isInteger(days)
		? String(days)
		: days.toLocaleString('id-ID', { maximumFractionDigits: 1 });
	return `SLA ± ${daysText} hari`;
}

/** Durasi tanpa awalan "SLA ±", untuk kalimat tooltip. */
export function slaDurationText(hours: number | null): string {
	if (hours == null) return '';
	return formatSla(hours).replace('SLA ± ', '');
}

/** Kelompokkan SLA untuk filter cepat: <3 jam, ≤24 jam, >24 jam. */
export function slaBucket(hours: number | null): SlaBucket | null {
	if (hours == null) return null;
	if (hours < 3) return 'super';
	if (hours <= 24) return 'fast';
	return 'standard';
}
