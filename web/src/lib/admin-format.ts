/**
 * Helper halaman admin: format angka/tanggal + fetch ringkas.
 * Parity format app lama (Intl id-ID) supaya tampilan identik.
 */

export const fmtNum = (n: unknown) => Number(n ?? 0).toLocaleString('id-ID');

export const fmtRp = (n: unknown) => `Rp ${Number(n ?? 0).toLocaleString('id-ID')}`;

export const fmtDate = (v: unknown) =>
	v ? new Date(v as string).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '—';

export const fmtDateTime = (v: unknown) =>
	v ? new Date(v as string).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—';

/** Label bahasa Indonesia untuk status yang muncul di banyak modul admin. */
export const STATUS_LABEL: Record<string, string> = {
	active: 'Aktif',
	inactive: 'Nonaktif',
	pending: 'Menunggu',
	pending_review: 'Menunggu review',
	approved: 'Disetujui',
	rejected: 'Ditolak',
	processing: 'Diproses',
	queued: 'Antrean',
	published: 'Terbit',
	failed: 'Gagal',
	cancelled: 'Dibatalkan',
	retrying: 'Coba ulang',
	paid: 'Dibayar',
	settled: 'Selesai',
	voided: 'Dibatalkan',
	refunded: 'Dikembalikan',
	waiting_partner: 'Menunggu mitra',
	partner_approved: 'Disetujui mitra',
	partner_rejected: 'Ditolak mitra',
	draft: 'Draf',
	archived: 'Diarsipkan',
	expired: 'Kedaluwarsa',
	verified: 'Terverifikasi',
	suspended: 'Ditangguhkan'
};

export function statusLabel(value: unknown): string {
	const key = String(value ?? '');
	return STATUS_LABEL[key] ?? (key || '—');
}

/** Tone badge per status (parity warna Badge radix app lama). */
export function statusTone(value: unknown): 'neutral' | 'accent' | 'success' | 'warning' | 'danger' {
	switch (String(value ?? '')) {
		case 'active':
		case 'approved':
		case 'published':
		case 'paid':
		case 'settled':
		case 'verified':
		case 'partner_approved':
			return 'success';
		case 'pending':
		case 'pending_review':
		case 'waiting_partner':
		case 'queued':
		case 'processing':
		case 'retrying':
		case 'draft':
			return 'warning';
		case 'failed':
		case 'rejected':
		case 'partner_rejected':
		case 'cancelled':
		case 'suspended':
		case 'expired':
			return 'danger';
		case 'archived':
		case 'inactive':
			return 'neutral';
		default:
			return 'accent';
	}
}
