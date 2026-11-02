// Shared helper untuk tab-tab /admin/media — parity lib/platform.ts app lama
// (sentinel RBAC "Resource not found" → pesan "Akses platform").
import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

const PLATFORM_ACCESS_ERROR = 'Akses platform administrator tidak tersedia.';

export async function platformRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
	const response = await apiRequest(`/v1/platform${path}`, init);
	if (response.status === 404) {
		const detail = await readApiError(response, 'Resource not found');
		if (detail === 'Resource not found') throw new Error(PLATFORM_ACCESS_ERROR);
		throw new Error(detail);
	}
	if (!response.ok) throw new Error(await readApiError(response, 'Permintaan platform gagal.'));
	if (response.status === 204) return undefined as T;
	return response.json() as Promise<T>;
}

export function errMessage(reason: unknown, fallback = 'Permintaan platform gagal.'): string {
	return reason instanceof Error ? reason.message : fallback;
}

export const isAccessError = (message: string) => message.includes('Akses platform');

/**
 * Bootstrap tab media: pastikan sesi segar sebelum memuat data, dan pasang
 * listener logout. Dipakai semua tab agar gating-nya konsisten.
 */
export function mountWithSession(load: () => unknown): () => void {
	void (async () => {
		const fresh = await ensureValidSession();
		if (!fresh) return;
		await load();
	})();
	const onExpired = () => (window.location.href = '/login');
	window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
	return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
}

export const idr = (n: unknown) =>
	new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0, roundingMode: 'trunc' }).format(
		Number(n ?? 0)
	);

export const compactNumber = (n: unknown) =>
	new Intl.NumberFormat('id-ID', { notation: 'compact', maximumFractionDigits: 1 }).format(Number(n ?? 0));

/** Semua kategori media (tabel link) — kolom legacy `category` hanya kategori pertama. */
export function categoryText(row: { category?: string | null; categories?: string[] | null }): string {
	const list = row.categories?.length ? row.categories : row.category ? [row.category] : [];
	return list.length ? list.join(', ') : '—';
}

const WORD_COUNT_MODES = [
	{ value: 'sesuai_permintaan', short: 'Sesuai Permintaan Klien' },
	{ value: 'kata_300_700', short: '300–700 kata' },
	{ value: 'kata_800_1500', short: '800–1500 kata' }
] as const;

export function wordCountLabel(mode: unknown): string {
	const found = WORD_COUNT_MODES.find((m) => m.value === mode);
	return found?.short ?? WORD_COUNT_MODES[0].short;
}

export const MEDIA_STATUSES = ['draft', 'active', 'inactive', 'archived', 'hidden', 'pending_review', 'rejected'];
export const MEDIA_INTEGRATION_TYPES_UI = ['manual', 'wordpress', 'rss', 'webhook', 'tulisio_api', 'emdash'];
export const MEDIA_TIERS = ['tier1', 'tier2', 'tier3'];

type PendingChange = { field: string; label: string; from: string | null; to: string | null };

export type Media = {
	id: string;
	name: string;
	slug: string | null;
	/** Slug URL publik (data.tauthub.com/media/<public_slug>). */
	public_slug?: string | null;
	/** false = disembunyikan dari katalog publik. */
	public_listed?: boolean | null;
	pricing_type?: 'nett' | 'nego' | null;
	price_status?: string | null;
	negotiated_price?: number | null;
	category: string | null;
	categories?: string[];
	pending_changes?: PendingChange[] | null;
	website_url: string | null;
	contoh_artikel_url?: string | null;
	price: number;
	client_price: number;
	markup_percent: number;
	markup_amount: number;
	credit_cost: number;
	status: string;
	integration_type: string;
	partner_id: string | null;
	partner_name?: string | null;
	/** Email akun mitra pemilik domain (users.email lewat partners.user_id). */
	partner_email?: string | null;
	/** true = kode verifikasi domain (meta tag di head / TXT DNS) terdeteksi. */
	domain_verified?: boolean;
	domain_verified_at?: string | null;
	partner_wa_set?: boolean;
	partner_wa_required?: boolean;
	referred_by_partner_id?: string | null;
	referrer_name?: string | null;
	publish_defaults: unknown;
	traffic_estimate: number;
	media_tier: string;
	media_quality_score: number | null;
	media_quality_notes: string | null;
	health_score: number | null;
	health_breakdown: Breakdown | null;
	health_computed_at: string | null;
	dewan_pers_verified?: boolean;
	dewan_pers_note?: string | null;
	dewan_pers_cert_no?: string | null;
	dewan_pers_doc_key?: string | null;
	publication_mode?: string;
	word_count_mode?: string | null;
	/** Masa berlaku tautan: 0 = permanen, >0 = bulan (default 12). */
	link_duration_months?: number | null;
	/** Jumlah backlink maksimal per artikel (0-3). allow_hyperlink = max_links > 0. */
	max_links?: number | null;
	allow_hyperlink?: boolean | null;
	/** 'tidak_ada' (organik) atau 'ada_sponsor' (tautan Sponsored). */
	label_sponsor?: string | null;
	rejection_reason?: string | null;
	seo_da?: number | null;
	seo_ai_trust?: number | null;
	seo_ai_citation?: boolean | null;
	seo_checked_at?: string | null;
	/** Waktu admin menyetujui (diisi saat keputusan review = approve). */
	reviewed_at?: string | null;
	/** Jumlah artikel yang sudah benar-benar terbit lewat media ini. */
	published_count?: number;
	/** Penangguhan admin (terpisah dari `status` — tidak bisa diubah mitra). */
	suspended_at?: string | null;
	suspended_reason?: string | null;
	is_suspended?: boolean;
	/** Tingkat pelanggaran aktif: none | ringan | berat. */
	violation_level?: 'none' | 'ringan' | 'berat';
	created_at: string;
	updated_at?: string | null;
};

export type Partner = { id: string; name: string | null; email: string; status: string };

export type Breakdown = {
	availability?: number;
	success_rate?: number;
	publish_time?: number;
	rejection_rate?: number;
	total?: number;
	published?: number;
	rejected?: number;
	failed?: number;
	avg_publish_hours?: number | null;
};

export type AuditDetail = {
	field?: string;
	label?: string;
	from?: string | null;
	to?: string | null;
};

export type AuditItem = {
	id: string;
	action: string;
	actor_email: string | null;
	created_at: string;
	detail?: AuditDetail | null;
};

export const AUDIT_LABEL: Record<string, string> = {
	'media.restored': 'Dipulihkan',
	'media.archived': 'Diarsipkan',
	'media.hidden': 'Disembunyikan dari mitra',
	'media.deleted': 'Dihapus permanen',
	'media.updated': 'Diperbarui',
	'media.da_updated': 'DA diperbarui manual',
	'media.created': 'Dibuat',
	// Perubahan konfigurasi oleh mitra (satu entri per field yang berubah).
	'media.setting_changed': 'Pengajuan perubahan'
};

export function auditTone(action: string): 'neutral' | 'accent' | 'success' | 'warning' | 'danger' {
	if (action === 'media.restored') return 'success';
	if (action === 'media.archived') return 'danger';
	return 'neutral';
}

export function scoreTone(score: number | null): 'neutral' | 'success' | 'warning' | 'danger' {
	if (score == null) return 'neutral';
	if (score >= 70) return 'success';
	if (score >= 40) return 'warning';
	return 'danger';
}

