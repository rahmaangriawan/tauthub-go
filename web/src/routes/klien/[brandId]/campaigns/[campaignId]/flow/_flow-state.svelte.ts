// Port `campaign-flow-module.tsx` (3.749 baris) app Next.js lama ke SvelteKit.
// Semua state wizard, pemanggilan API, dan aturan urutan tahap ada di sini
// supaya +page.svelte dan komponen tahap hanya menangani tampilan.
import { SvelteSet } from 'svelte/reactivity';
import { apiRequest, readApiError } from '$lib/auth';

/* ─── Konstanta tahap & mode ──────────────────────────────────────────── */

export const STAGES = [
	{ key: 'intake', label: '1. Content Intake' },
	{ key: 'content', label: '2. Content' },
	{ key: 'media_selection', label: '3. Media & Artikel' },
	{ key: 'final_review', label: '4. Review & Checkout' },
	{ key: 'distributing', label: '5. Distribusi' }
];

export const MODES: Array<{ value: string; title: string; desc: string }> = [
	{ value: 'artikel', title: 'Sudah Punya Naskah', desc: 'Naskah sudah siap dan bisa langsung digunakan.' },
	{ value: 'bahan', title: 'Belum Punya Naskah', desc: 'Buat naskah berdasarkan kebutuhan kampanye Anda.' }
];

export const LEVEL_COLOR: Record<string, string> = {
	ready: 'var(--success)',
	needs_more: 'var(--warning)',
	insufficient: 'var(--destructive)'
};
export const LEVEL_LABEL: Record<string, string> = {
	ready: 'Siap diproses',
	needs_more: 'Perlu dilengkapi',
	insufficient: 'Belum cukup'
};

/** Placeholder SVG (data URI) saat gambar thumbnail hilang/tidak tersedia. */
export const THUMB_FALLBACK =
	'data:image/svg+xml,' +
	encodeURIComponent(
		'<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180"><rect width="100%" height="100%" fill="#eef0f3"/><text x="50%" y="50%" fill="#98a2b3" font-size="13" font-family="sans-serif" text-anchor="middle" dominant-baseline="middle">Gambar tidak tersedia</text></svg>'
	);

/** Ganti src ke placeholder bila gambar gagal dimuat (mis. file upload hilang). */
export function thumbOnError(event: Event) {
	const img = event.currentTarget as HTMLImageElement | null;
	if (!img) return;
	img.onerror = null;
	img.src = THUMB_FALLBACK;
}

/* ─── Types ───────────────────────────────────────────────────────────── */

export type Question = {
	key: string;
	label: string;
	type: string;
	required: boolean;
	weight: number;
	helper?: string | null;
	placeholder?: string | null;
	options?: string[];
};
export type IntakeType = {
	id: string;
	code: string;
	name: string;
	description: string | null;
	questions: Question[];
	status: string;
};
export type Check = { key: string; label: string; status: 'ok' | 'warn' | 'missing'; weight: number; required: boolean };
export type Readiness = { score: number; level: 'ready' | 'needs_more' | 'insufficient'; checks: Check[]; missing: string[] };
export type Recommendation = {
	angle: string;
	topic_suggestion: string;
	structure: string[];
	missing_info: string[];
	follow_up_questions: string[];
	notes: string;
};
export type Intake = {
	id: string;
	mode: string;
	content_type_code: string | null;
	topic: string | null;
	answers: Record<string, unknown>;
	article_text: string | null;
	article_filename: string | null;
	key_messages: string[];
	readiness_score: number | null;
	readiness_checks: Check[];
	recommendation: Recommendation | null;
	status: string;
	job_id: string | null;
	created_at: string;
};
export type Campaign = {
	id: string;
	name: string;
	description: string | null;
	status: string;
	article_quota: number;
	budget: number;
	starts_at: string | null;
	ends_at: string | null;
	publication_count: number;
	created_at: string;
	content_intake_id?: string | null;
	campaign_flow_status?: string | null;
	media_type?: 'blogger' | 'mainstream' | null;
	selected_media_id?: string | null;
	editorial_standard_snapshot?: Record<string, unknown> | null;
};
export type DocSummary = { title: string; meta_title: string | null; meta_description: string | null; content: string };
export type CampaignMediaItem = {
	id: string;
	campaign_id: string;
	media_id: string;
	media_name: string;
	media_category: string | null;
	media_dewan_pers_verified?: boolean | null;
	status: string;
	angle_label: string | null;
	source_document_id: string | null;
	variant_document_id: string | null;
	source_document: DocSummary | null;
	variant_document: DocSummary | null;
	thumbnail_url: string | null;
	tc_cost: number;
	publication_status: string | null;
	publication_url: string | null;
	publication_id: string | null;
	publication_order_code: string | null;
	tracking_code: string | null;
};
export type MediaItem = {
	id: string;
	name: string;
	category: string | null;
	price: number;
	credit_cost: number;
	health_score?: number;
	status?: string;
	is_available?: boolean;
	availability_reason?: string | null;
	dewan_pers_verified?: boolean | null;
	created_at?: string | null;
	media_type?: 'blogger' | 'mainstream' | null;
};
export type MediaRecommendation = {
	id: string;
	name: string;
	category: string | null;
	price: number;
	credit_cost: number;
	health_score: number | null;
	score: number;
	reasons: string[];
	fit: string;
	status?: string;
	is_available?: boolean;
	availability_reason?: string | null;
	dewan_pers_verified?: boolean | null;
};
export type Wallet = { available_tc: number; reserved_tc: number; captured_tc: number };
export type FlowDocument = {
	id: string;
	title: string;
	meta_title: string;
	meta_description: string;
	body_html: string;
	status: string;
	evaluation: Record<string, unknown>;
	readiness_score: number | null;
	readiness_checks: Check[];
};
export type ContentJob = {
	id: string;
	status: string;
	document_id: string | null;
	progress: number;
	current_step: string;
	error_message?: string | null;
	outline?: Array<{ level: number; title: string }>;
};
export type StandardResult = { status: string; score?: number; failed?: string[]; warnings?: string[]; media_name?: string | null };
export type DocumentVersion = { version: number; html?: string; change_note?: string | null; attribution?: string | null; created_at?: string };
export type FlowData = {
	campaign: Campaign;
	flow_status: string | null;
	intake: Intake | null;
	media_items: CampaignMediaItem[];
	wallet: Wallet | null;
	tc_config: { rate_idr: number } | null;
};
export type AngleOption = { id: string; label: string };

/* ─── Helper parsing ──────────────────────────────────────────────────── */

/** Ekstrak H1 pertama dari HTML body, plain text. */
export function extractH1(html: string): { h1: string; body: string } {
	const match = /<h1[^>]*>([\s\S]*?)<\/h1>/i.exec(html);
	if (!match) return { h1: '', body: html };
	const text = match[1]
		.replace(/<[^>]+>/g, ' ')
		.replace(/\s+/g, ' ')
		.trim();
	return { h1: text, body: html.replace(match[0], '').trim() };
}

export function countWords(text: string): number {
	const trimmed = text.trim();
	return trimmed ? trimmed.split(/\s+/).filter(Boolean).length : 0;
}

/* ─── Minimal markdown → HTML (port apps/web/src/lib/markdown.ts) ─────── */

function inlineMd(text: string): string {
	return text
		.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
		.replace(/\*([^*]+?)\*/g, '<em>$1</em>')
		.replace(/~~([^~]+)~~/g, '<del>$1</del>')
		.replace(/`([^`]+)`/g, '<code>$1</code>')
		.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, '<img src="$2" alt="$1" style="max-width:100%;height:auto;border-radius:8px" />')
		.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer">$1</a>');
}

export function markdownToHtml(markdown: string): string {
	const html: string[] = [];
	let listType: 'ul' | 'ol' | null = null;
	const paragraph: string[] = [];
	const flush = () => {
		if (paragraph.length) {
			html.push(`<p>${inlineMd(paragraph.join(' '))}</p>`);
			paragraph.length = 0;
		}
	};
	const closeList = () => {
		if (listType) {
			html.push(`</${listType}>`);
			listType = null;
		}
	};
	const openList = (type: 'ul' | 'ol') => {
		if (listType !== type) {
			closeList();
			html.push(`<${type}>`);
			listType = type;
		}
	};

	for (const raw of markdown.replace(/\r\n/g, '\n').split('\n')) {
		const line = raw.trim();
		if (!line) {
			flush();
			closeList();
			continue;
		}
		if (line === '---') continue;

		const heading = /^(#{1,6})\s+(.*)$/.exec(line);
		if (heading) {
			flush();
			closeList();
			html.push(`<h${heading[1].length}>${inlineMd(heading[2])}</h${heading[1].length}>`);
			continue;
		}
		const quote = /^>\s+(.*)$/.exec(line);
		if (quote) {
			flush();
			closeList();
			html.push(`<blockquote>${inlineMd(quote[1])}</blockquote>`);
			continue;
		}
		const ordered = /^\d+[.)]\s+(.*)$/.exec(line);
		if (ordered) {
			flush();
			openList('ol');
			html.push(`<li>${inlineMd(ordered[1])}</li>`);
			continue;
		}
		const unordered = /^[-*+]\s+(.*)$/.exec(line);
		if (unordered) {
			flush();
			openList('ul');
			html.push(`<li>${inlineMd(unordered[1])}</li>`);
			continue;
		}
		paragraph.push(line);
	}
	flush();
	closeList();
	return html.join('\n');
}

/* ─── Baca berkas naskah (.txt / .docx) ──────────────────────────────── */

async function inflateRaw(data: Uint8Array): Promise<string> {
	const stream = new Blob([data as BlobPart]).stream().pipeThrough(new DecompressionStream('deflate-raw'));
	return new Response(stream).text();
}

function xmlToText(xml: string): string {
	return xml
		.replace(/<w:tab[^>]*\/>/g, '\t')
		.replace(/<\/w:p>/g, '\n')
		.replace(/<w:br[^>]*\/>/g, '\n')
		.replace(/<[^>]+>/g, '')
		.replace(/&lt;/g, '<')
		.replace(/&gt;/g, '>')
		.replace(/&amp;/g, '&')
		.replace(/&quot;/g, '"')
		.replace(/&#39;/g, "'")
		.replace(/\n{3,}/g, '\n\n')
		.trim();
}

/** Ekstrak teks dari .docx tanpa dependensi (ZIP + DecompressionStream). */
async function docxToText(file: File): Promise<string> {
	const buf = await file.arrayBuffer();
	const bytes = new Uint8Array(buf);
	const dv = new DataView(buf);
	let eocd = -1;
	const min = Math.max(0, bytes.length - 22 - 65535);
	for (let i = bytes.length - 22; i >= min; i--) {
		if (dv.getUint32(i, true) === 0x06054b50) {
			eocd = i;
			break;
		}
	}
	if (eocd < 0) throw new Error('bukan arsip docx');
	const count = dv.getUint16(eocd + 10, true);
	let off = dv.getUint32(eocd + 16, true);
	for (let i = 0; i < count; i++) {
		if (dv.getUint32(off, true) !== 0x02014b50) break;
		const method = dv.getUint16(off + 10, true);
		const compSize = dv.getUint32(off + 20, true);
		const nameLen = dv.getUint16(off + 28, true);
		const extraLen = dv.getUint16(off + 30, true);
		const commentLen = dv.getUint16(off + 32, true);
		const localOff = dv.getUint32(off + 42, true);
		const name = new TextDecoder().decode(bytes.subarray(off + 46, off + 46 + nameLen));
		if (name === 'word/document.xml') {
			const lNameLen = dv.getUint16(localOff + 26, true);
			const lExtraLen = dv.getUint16(localOff + 28, true);
			const start = localOff + 30 + lNameLen + lExtraLen;
			const data = bytes.subarray(start, start + compSize);
			const xml = method === 0 ? new TextDecoder().decode(data) : await inflateRaw(data);
			return xmlToText(xml);
		}
		off += 46 + nameLen + extraLen + commentLen;
	}
	throw new Error('word/document.xml tidak ditemukan');
}

export async function readArticleFile(file: File, setErr: (v: string) => void): Promise<{ text: string; filename: string } | null> {
	if (file.size > 10 * 1024 * 1024) {
		setErr('Ukuran file maksimal 10 MB.');
		return null;
	}
	try {
		let text: string;
		if (file.name.toLowerCase().endsWith('.docx')) {
			text = (await docxToText(file)).trim();
			if (!text) {
				setErr('File .docx tidak mengandung teks yang bisa dibaca.');
				return null;
			}
		} else {
			text = await file.text();
		}
		return { text, filename: file.name };
	} catch {
		setErr('File tidak bisa dibaca. Pastikan format .txt atau .docx.');
		return null;
	}
}

/* ─── Error helper ────────────────────────────────────────────────────── */

/**
 * Parity `readError()` app lama, tapi didelegasikan ke `readApiError` supaya
 * penanganan `detail` (string / array validasi / object {message,score}) sama
 * dengan halaman SvelteKit lain — selalu ada fallback.
 */
export function readError(res: Response, fallback: string): Promise<string> {
	return readApiError(res, fallback);
}

/* ─── State ───────────────────────────────────────────────────────────── */

export class FlowState {
	#segment: () => string;
	#campaignId: () => string;
	#booted = false;

	constructor(segment: () => string, campaignId: () => string) {
		this.#segment = segment;
		this.#campaignId = campaignId;
	}

	/* Brand resolution */
	resolved = $state<{ id: string } | null>(null);
	get campaignId() {
		return this.#campaignId();
	}
	get segment() {
		return this.#segment();
	}
	get brandId() {
		return this.resolved?.id ?? this.segment;
	}
	get apiBase() {
		return `/v1/brands/${this.brandId}`;
	}
	get flowBase() {
		return `${this.apiBase}/campaigns/${this.campaignId}`;
	}

	/* Global */
	loading = $state(true);
	busy = $state(false);
	err = $state('');
	flowData = $state<FlowData | null>(null);

	/* Stage 0 — intake */
	types = $state<IntakeType[]>([]);
	intakeMode = $state<string | null>(null);
	typeCode = $state('');
	topic = $state('');
	answers = $state<Record<string, unknown>>({});
	articleText = $state('');
	articleFilename = $state('');
	keyMessagesText = $state('');
	kmAutoBusy = $state(false);
	readiness = $state<Readiness | null>(null);
	recommendation = $state<Recommendation | null>(null);
	intakeStep = $state(0);
	intakeSaving = $state<'draft' | 'analyze' | null>(null);
	advancingContent = $state(false);
	draftSavedAt = $state<string | null>(null);
	aiFillOpen = $state(false);
	aiFillTopic = $state('');
	aiFillBusy = $state(false);
	aiFillNote = $state('');
	aiFillProgress = $state(0);
	#aiFillTimer: ReturnType<typeof setInterval> | null = null;
	#saveIntakeKey = 0;

	/* Stage 1 — content result */
	doc = $state<FlowDocument | null>(null);
	editorSaving = $state(false);
	editorError = $state<string | null>(null);
	editorReloadKey = $state(0);
	genBusy = $state(false);
	genMode = $state<'economy' | 'premium'>('premium');
	genProgress = $state(0);
	genStep = $state('');
	regenOpen = $state(false);
	regenConfirm = $state(false);
	regenAngleOptions = $state<AngleOption[]>([]);
	regenAngleId = $state('');
	regenBusy = $state(false);
	regenProgress = $state(0);
	#regenDocId = '';
	regenToast = $state<{ title: string; documentId: string } | null>(null);

	/* Stage 2 — media */
	mediaList = $state<MediaItem[]>([]);
	selectedMedia = new SvelteSet<string>();
	mediaSearch = $state('');
	mediaTypeBusy = $state(false);
	recommendations = $state<MediaRecommendation[]>([]);
	recommendationsExpanded = $state(true);
	recommendBusy = $state(false);
	paraphraseBusy = $state<string | null>(null);
	angleModal = $state<string | null>(null);
	angleOptions = $state<AngleOption[]>([]);
	submittingAngleId = $state<string | null>(null);
	submittingAngleLabel = $state('');
	angleProgress = $state(0);
	standardResults = $state<Record<string, StandardResult>>({});

	/* Stage 2 — post editor */
	editingPostItem = $state<CampaignMediaItem | null>(null);
	editPostDocId = $state('');
	editPostTitle = $state('');
	editPostThumbnail = $state('');
	editPostMetaDesc = $state('');
	editPostBusy = $state(false);
	editPostLoading = $state(false);
	editPostVersions = $state<DocumentVersion[]>([]);
	editPostActiveVersion = $state<number | null>(null);
	uploadingThumbId = $state<string | null>(null);
	thumbUploadError = $state('');

	/* Stage 3 — checkout */
	agreed = $state(false);
	checkoutBusy = $state(false);

	/* Stage 4 — distribution polling */
	distPoll = $state(0);

	/* ─── Derived ───────────────────────────────────────────────────── */

	get flowStatus() {
		return this.flowData?.flow_status ?? 'intake';
	}
	get stageIndex() {
		return STAGES.findIndex((s) => s.key === this.flowStatus);
	}
	get campaign() {
		return this.flowData?.campaign;
	}
	get intake() {
		return this.flowData?.intake;
	}
	get mediaItems() {
		return this.flowData?.media_items ?? [];
	}
	get wallet() {
		return this.flowData?.wallet;
	}
	get tcConfig() {
		return this.flowData?.tc_config;
	}
	get currentType() {
		return this.types.find((t) => t.code === this.typeCode) ?? null;
	}
	get filteredMedia() {
		return this.mediaList
			.filter((m) => {
				if (this.campaign?.media_type && m.media_type && m.media_type !== this.campaign.media_type) return false;
				if (!this.mediaSearch.trim()) return true;
				const q = this.mediaSearch.toLowerCase();
				return m.name.toLowerCase().includes(q) || (m.category ?? '').toLowerCase().includes(q);
			})
			.sort((a, b) => a.name.localeCompare(b.name));
	}
	get totalTc() {
		let sum = 0;
		for (const id of this.selectedMedia) sum += this.mediaList.find((m) => m.id === id)?.credit_cost ?? 0;
		return sum;
	}
	get itemsReady() {
		return this.mediaItems.filter((m) => m.status === 'article_ready').length;
	}
	get itemsNeedsRewrite() {
		return this.mediaItems.filter((m) => m.status === 'needs_rewrite').length;
	}
	get itemsProcessing() {
		return this.mediaItems.filter((m) => m.status === 'rewriting' || m.status === 'angle_select').length;
	}
	get allReady() {
		return this.mediaItems.length > 0 && this.mediaItems.every((m) => m.status === 'article_ready');
	}
	get activeDocId() {
		return (
			this.doc?.id ||
			this.mediaItems.find((m) => m.variant_document_id || m.source_document_id)?.variant_document_id ||
			this.mediaItems.find((m) => m.source_document_id)?.source_document_id ||
			undefined
		);
	}
	get standardSummary() {
		const checkedIds = Object.keys(this.standardResults).filter((id) => this.selectedMedia.has(id));
		if (checkedIds.length === 0) return null;
		const hasFail = checkedIds.some((id) => this.standardResults[id]?.status === 'fail');
		const hasWarn = checkedIds.some((id) => this.standardResults[id]?.status === 'warning');
		if (hasFail) return { type: 'fail' as const, text: '✕ Ada media yang belum memenuhi standar' };
		if (hasWarn) return { type: 'warn' as const, text: '⚠ Ada peringatan standar pada media' };
		return { type: 'pass' as const, text: '✓ Standar media terpenuhi' };
	}
	get allMediaPublished() {
		return this.mediaItems.length > 0 && this.mediaItems.every((m) => m.publication_status === 'published' || m.status === 'published');
	}
	get isStage5Completed() {
		return (this.flowStatus === 'distributing' || this.flowStatus === 'completed') && this.allMediaPublished;
	}
	get draftNotice() {
		return this.draftSavedAt
			? `✓ Draft tersimpan ke database (${this.draftSavedAt}). Buka lagi lewat URL flow kampanye ini kapan saja untuk melanjutkan: Lengkapi Informasi → Simpan & analisis.`
			: '';
	}

	/* ─── Boot & load ───────────────────────────────────────────────── */

	async boot() {
		if (this.#booted) return;
		this.#booted = true;
		let ok = false;
		try {
			const res = await apiRequest(`/v1/brands/${this.segment}`);
			if (res.ok) {
				const data = (await res.json()) as { id: string };
				this.resolved = { id: data.id };
				ok = true;
			}
		} catch {
			/* ditangani auth */
		}
		if (!ok) {
			// Brand tidak bisa di-resolve: tampilkan pesan, jangan nyangkut di skeleton.
			this.err = 'Gagal memuat data brand.';
			this.loading = false;
			return;
		}
		await Promise.all([this.loadFlow(), this.loadTypes(), this.loadCatalog()]);
		this.loading = false;
	}

	async loadFlow(silent = false) {
		if (!silent) this.loading = true;
		this.err = '';
		try {
			const res = await apiRequest(`${this.flowBase}/flow`);
			if (!res.ok) throw new Error(await readError(res, 'Gagal memuat flow.'));
			const data = (await res.json()) as FlowData;
			this.flowData = data;
			if (data.intake) {
				if (data.intake.mode !== this.intakeMode) this.intakeMode = data.intake.mode;
				this.typeCode = data.intake.content_type_code ?? '';
				this.topic = data.intake.topic ?? '';
				this.answers = data.intake.answers ?? {};
				this.articleText = data.intake.article_text ?? '';
				this.articleFilename = data.intake.article_filename ?? '';
				this.keyMessagesText = (data.intake.key_messages ?? []).join('\n');
				this.recommendation = data.intake.recommendation ?? null;
				if (data.intake.readiness_score !== null) {
					this.readiness = {
						score: data.intake.readiness_score,
						level: 'needs_more',
						checks: data.intake.readiness_checks ?? [],
						missing: []
					};
				}
				// Ada draft intake → langsung buka Lengkapi Informasi.
				if (this.intakeStep === 0) this.intakeStep = 2;
			} else {
				this.typeCode = '';
				this.topic = '';
				this.answers = {};
				this.articleText = '';
				this.articleFilename = '';
				this.keyMessagesText = '';
				this.recommendation = null;
				this.readiness = null;
				this.intakeMode = null;
				this.doc = null;
			}
			if (data.flow_status === 'content' && !data.intake?.job_id) this.intakeStep = 3;
			this.syncSelectedMedia();
			if (data.flow_status === 'media_selection') void this.loadRecommendations();
			if (['content', 'media_selection', 'final_review'].includes(data.flow_status ?? '')) void this.loadDocument();
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Gagal memuat flow.';
		} finally {
			if (!silent) this.loading = false;
		}
	}

	async loadTypes() {
		const res = await apiRequest(`${this.apiBase}/content-intake-types`);
		if (res.ok) this.types = (await res.json()) as IntakeType[];
	}

	async loadCatalog() {
		const res = await apiRequest(`${this.apiBase}/media?limit=100`);
		if (res.ok) {
			const data = (await res.json()) as { media: MediaItem[] };
			this.mediaList = data.media ?? [];
		}
	}

	// Sinkronkan pilihan media dengan item aktif di flow. Setelah "Buat Artikel
	// Baru" (intake null), media items lama tetap jadi riwayat tapi TIDAK tercentang.
	syncSelectedMedia() {
		if (this.intake && this.mediaItems.length > 0) {
			const currentIds = this.mediaItems.map((m) => m.media_id);
			const same = this.selectedMedia.size === currentIds.length && currentIds.every((id) => this.selectedMedia.has(id));
			if (!same) {
				this.selectedMedia.clear();
				for (const id of currentIds) this.selectedMedia.add(id);
			}
		} else if (this.selectedMedia.size > 0) {
			this.selectedMedia.clear();
		}
	}

	async loadRecommendations() {
		this.recommendBusy = true;
		try {
			const q = new URLSearchParams({ count: '5' });
			if (this.campaign && Number(this.campaign.budget) > 0) q.set('budget', String(this.campaign.budget));
			const res = await apiRequest(`${this.apiBase}/media/recommendations?${q.toString()}`);
			if (res.ok) {
				const data = (await res.json()) as { recommendations: MediaRecommendation[] };
				this.recommendations = data.recommendations ?? [];
			}
		} catch {
			/* non-fatal */
		} finally {
			this.recommendBusy = false;
		}
	}

	async loadDocument() {
		const intake = this.intake;
		if (!intake?.job_id) return;
		try {
			const res = await apiRequest(`${this.apiBase}/content-jobs/${intake.job_id}`);
			if (!res.ok) return;
			const job = (await res.json()) as ContentJob;
			// Card approval outline dimatikan di app lama (jurnalistik langsung ngalir).
			if (!job.document_id) return;
			const docRes = await apiRequest(`${this.apiBase}/documents/${job.document_id}`);
			if (!docRes.ok) return;
			const raw = (await docRes.json()) as {
				id: string;
				title: string;
				meta_title: string | null;
				meta_description: string | null;
				status: string;
				evaluation?: Record<string, unknown>;
				content?: { html?: string };
			};
			this.doc = {
				id: raw.id,
				title: raw.title,
				meta_title: raw.meta_title ?? '',
				meta_description: raw.meta_description ?? '',
				body_html: raw.content?.html ?? '',
				status: raw.status,
				evaluation: raw.evaluation ?? {},
				readiness_score: null,
				readiness_checks: []
			};
		} catch {
			/* non-fatal */
		}
	}

	/* ─── Stage 0: media type & intake ──────────────────────────────── */

	async saveMediaType(mediaType: 'blogger' | 'mainstream') {
		this.mediaTypeBusy = true;
		this.err = '';
		try {
			const res = await apiRequest(`${this.flowBase}/flow/media-type`, {
				method: 'POST',
				body: JSON.stringify({ media_type: mediaType })
			});
			if (!res.ok) throw new Error(await readError(res, 'Gagal menyimpan jenis media.'));
			await this.loadFlow();
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Gagal menyimpan jenis media.';
		} finally {
			this.mediaTypeBusy = false;
		}
	}

	async createIntake() {
		if (!this.intakeMode) {
			this.err = 'Pilih cara mulai.';
			return;
		}
		this.busy = true;
		this.err = '';
		try {
			const res = await apiRequest(`${this.apiBase}/content-intakes`, {
				method: 'POST',
				body: JSON.stringify({
					mode: this.intakeMode,
					content_type_code: this.typeCode.trim() || null,
					topic: this.topic.trim() || null
				})
			});
			if (!res.ok) throw new Error(await readError(res, 'Gagal membuat intake.'));
			const created = (await res.json()) as Intake;
			// Tautkan intake ke campaign — tanpa ini draft tidak bisa dibuka lagi.
			try {
				const link = await apiRequest(`${this.flowBase}/flow`, {
					method: 'POST',
					body: JSON.stringify({ intake_id: created.id })
				});
				if (!link.ok) throw new Error(await readError(link, 'Intake dibuat tapi gagal ditautkan ke kampanye.'));
				await this.loadFlow();
			} catch {
				this.flowData = this.flowData ? { ...this.flowData, intake: created } : this.flowData;
			}
			this.intakeStep = 2;
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Gagal membuat intake.';
		} finally {
			this.busy = false;
		}
	}

	async saveIntake(goResult = false) {
		const intake = this.flowData?.intake;
		if (!intake) return;
		const key = ++this.#saveIntakeKey;
		this.intakeSaving = goResult ? 'analyze' : 'draft';
		this.busy = true;
		this.err = '';
		try {
			const res = await apiRequest(`${this.apiBase}/content-intakes/${intake.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					content_type_code: this.typeCode.trim() || null,
					topic: this.topic.trim() || null,
					answers: this.answers,
					article_text: this.articleText.trim() || null,
					article_filename: this.articleFilename || null,
					key_messages: this.keyMessagesText
						.split('\n')
						.map((s) => s.trim())
						.filter(Boolean)
				})
			});
			if (!res.ok) throw new Error(await readError(res, 'Gagal menyimpan intake.'));
			const data = (await res.json()) as Intake & { readiness: Readiness };
			if (key !== this.#saveIntakeKey) return;
			this.flowData = this.flowData ? { ...this.flowData, intake: data } : this.flowData;
			this.readiness = data.readiness ?? null;
			this.recommendation = data.recommendation ?? null;
			if (goResult) this.intakeStep = 3;
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Gagal menyimpan intake.';
		} finally {
			this.busy = false;
			this.intakeSaving = null;
		}
	}

	async saveAsDraft() {
		await this.saveIntake(false);
		this.draftSavedAt = new Date().toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });
		const stamp = this.draftSavedAt;
		window.setTimeout(() => {
			if (this.draftSavedAt === stamp) this.draftSavedAt = null;
		}, 4000);
	}

	async submitAiFill() {
		const intake = this.flowData?.intake;
		if (!intake) return;
		if (!this.aiFillTopic.trim()) {
			this.aiFillNote = 'Masukkan topik atau judul naskah dulu.';
			return;
		}
		this.aiFillBusy = true;
		this.aiFillNote = '';
		this.err = '';
		this.aiFillProgress = 0;
		this.#aiFillTimer = setInterval(() => {
			this.aiFillProgress = Math.min(92, this.aiFillProgress + Math.max(1, Math.round((95 - this.aiFillProgress) / 12)));
		}, 180);
		try {
			const res = await apiRequest(`${this.apiBase}/content-intakes/${intake.id}/ai-fill`, {
				method: 'POST',
				body: JSON.stringify({
					topic: this.aiFillTopic.trim(),
					type_code: this.typeCode,
					answers: this.answers,
					key_messages: this.keyMessagesText
						.split('\n')
						.map((s) => s.trim())
						.filter(Boolean)
				})
			});
			if (!res.ok) {
				const body = (await res.json().catch(() => null)) as {
					detail?: string;
					code?: string;
					needed?: number;
					available?: number;
				} | null;
				if (body?.code === 'insufficient_credits') {
					this.aiFillNote = body.detail ?? `Saldo TC tidak cukup. Butuh ${body.needed} TC, tersedia ${body.available} TC.`;
					this.err = body.detail ?? 'Kredit Saldo tidak cukup. Silakan Top Up.';
					return;
				}
				this.aiFillNote = body?.detail ?? 'Gagal mengisi otomatis. Coba lagi.';
				return;
			}
			const data = (await res.json()) as { answers?: Record<string, unknown>; filled?: string[]; key_messages?: string[] };
			if (data.answers) this.answers = data.answers;
			if (data.key_messages?.length) {
				const existing = this.keyMessagesText
					.split('\n')
					.map((s) => s.trim())
					.filter(Boolean);
				const merged = [...new Set([...existing, ...data.key_messages])].slice(0, 20);
				this.keyMessagesText = merged.join('\n');
			}
			this.aiFillProgress = 100;
			window.setTimeout(() => (this.aiFillOpen = false), 450);
			this.aiFillTopic = '';
			const count = data.filled?.length ?? 0;
			const kmCount = data.key_messages?.length ?? 0;
			const total = count + kmCount;
			this.aiFillNote =
				total > 0
					? `Berhasil isi otomatis: ${count} field + ${kmCount} key messages. Silakan periksa dan sesuaikan sebelum melanjutkan.`
					: 'Semua field sudah terisi. Tidak ada yang diubah.';
			window.setTimeout(() => {
				if (this.aiFillNote.startsWith('Berhasil isi otomatis') || this.aiFillNote.startsWith('Semua field')) this.aiFillNote = '';
			}, 6000);
		} catch {
			this.aiFillProgress = 0;
			this.aiFillNote = 'Gagal menghubungi AI. Coba lagi.';
		} finally {
			if (this.#aiFillTimer) clearInterval(this.#aiFillTimer);
			this.#aiFillTimer = null;
			this.aiFillBusy = false;
			window.setTimeout(() => {
				if (!this.aiFillBusy) this.aiFillProgress = 0;
			}, 400);
		}
	}

	async autoFillKeyMessages() {
		const intake = this.flowData?.intake;
		if (!intake) return;
		if (!this.topic.trim()) {
			this.err = 'Isi topik dulu sebelum auto-fill key messages.';
			return;
		}
		this.kmAutoBusy = true;
		this.err = '';
		try {
			const res = await apiRequest(`${this.apiBase}/content-intakes/${intake.id}/auto-key-messages`, {
				method: 'POST',
				body: JSON.stringify({
					topic: this.topic.trim(),
					answers: this.answers,
					article_text: this.articleText || undefined,
					media_id: this.campaign?.selected_media_id ?? undefined,
					media_type: this.campaign?.media_type ?? undefined,
					existing_key_messages: this.keyMessagesText
						.split('\n')
						.map((s) => s.trim())
						.filter(Boolean)
				})
			});
			if (!res.ok) {
				const body = (await res.json().catch(() => null)) as { detail?: string } | null;
				this.err = body?.detail ?? 'Gagal auto-fill key messages. Coba lagi.';
				return;
			}
			const data = (await res.json()) as { key_messages?: string[] };
			if (data.key_messages?.length) {
				const existing = this.keyMessagesText
					.split('\n')
					.map((s) => s.trim())
					.filter(Boolean);
				const merged = [...new Set([...existing, ...data.key_messages])].slice(0, 20);
				this.keyMessagesText = merged.join('\n');
				this.err = '';
			} else {
				this.err = 'AI tidak menghasilkan key messages. Coba lagi.';
			}
		} catch {
			this.err = 'Gagal menghubungi AI. Coba lagi.';
		} finally {
			this.kmAutoBusy = false;
		}
	}

	async advanceAfterIntake() {
		const intake = this.flowData?.intake;
		if (!intake) return;
		this.advancingContent = true;
		this.busy = true;
		this.err = '';
		try {
			const res = await apiRequest(`${this.flowBase}/flow`, {
				method: 'POST',
				body: JSON.stringify({ intake_id: intake.id, flow_status: 'content' })
			});
			if (!res.ok) throw new Error(await readError(res, 'Gagal melanjutkan flow.'));
			await this.loadFlow();
			void this.loadDocument();
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Gagal melanjutkan flow.';
		} finally {
			this.busy = false;
			this.advancingContent = false;
		}
	}

	/* ─── Stage 1: dokumen & generasi ───────────────────────────────── */

	async saveDocument(title: string, html: string, publish: boolean) {
		if (!this.doc) return;
		this.editorSaving = true;
		this.editorError = null;
		try {
			const res = await apiRequest(`${this.apiBase}/documents/${this.doc.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					title,
					content: html,
					content_format: 'html',
					change_note: 'editor update (content flow)'
				})
			});
			if (!res.ok) throw new Error(await readError(res, 'Gagal menyimpan artikel.'));
			this.doc = { ...this.doc, title, body_html: html };
			this.editorReloadKey += 1;
			if (publish) {
				const flowRes = await apiRequest(`${this.flowBase}/flow`, {
					method: 'POST',
					body: JSON.stringify({ flow_status: 'media_selection' })
				});
				if (!flowRes.ok) throw new Error(await readError(flowRes, 'Gagal lanjut ke media.'));
				await this.loadFlow();
			}
		} catch (reason) {
			this.editorError = reason instanceof Error ? reason.message : 'Gagal menyimpan artikel.';
		} finally {
			this.editorSaving = false;
		}
	}

	async openRegenModal(docId?: string) {
		this.#regenDocId = docId ?? '';
		this.regenOpen = true;
		this.regenConfirm = false;
		this.regenAngleId = '';
		this.err = '';
		try {
			const res = await apiRequest(`${this.flowBase}/flow/media-items/none/angle-options`);
			if (res.ok) {
				const data = (await res.json()) as { options: AngleOption[] };
				this.regenAngleOptions = data.options;
			}
		} catch {
			/* fallback: manual input */
		}
	}

	async submitRegen() {
		if (!this.regenAngleId) return;
		this.regenConfirm = false;
		this.regenBusy = true;
		this.regenProgress = 6;
		this.err = '';
		const interval = window.setInterval(() => {
			const prev = this.regenProgress;
			if (prev < 28) this.regenProgress = prev + Math.floor(Math.random() * 5) + 3;
			else if (prev < 58) this.regenProgress = prev + Math.floor(Math.random() * 4) + 2;
			else if (prev < 84) this.regenProgress = prev + Math.floor(Math.random() * 3) + 1;
			else if (prev < 94) this.regenProgress = prev + 0.5;
		}, 240);
		try {
			const res = await apiRequest(`${this.flowBase}/flow/article-regen`, {
				method: 'POST',
				body: JSON.stringify({
					angle_id: this.regenAngleId,
					...(this.#regenDocId ? { document_id: this.#regenDocId } : {})
				})
			});
			if (!res.ok) throw new Error(await readError(res, 'Gagal membuat ulang artikel.'));
			const regenData = (await res.json()) as {
				ok?: boolean;
				document_id?: string;
				title?: string;
				angle_label?: string;
				tc_used?: number;
			};
			window.clearInterval(interval);
			this.regenProgress = 100;
			await new Promise((r) => setTimeout(r, 500));
			this.regenOpen = false;
			if (this.#regenDocId && this.editingPostItem) {
				this.editingPostItem = null;
			} else {
				this.doc = null;
				this.editorReloadKey += 1;
			}
			this.#regenDocId = '';
			await this.loadFlow();
			await this.loadDocument();
			if (regenData.document_id) {
				this.regenToast = { title: regenData.title ?? 'Naskah', documentId: regenData.document_id };
			}
		} catch (reason) {
			window.clearInterval(interval);
			this.err = reason instanceof Error ? reason.message : 'Buat ulang gagal.';
		} finally {
			this.regenBusy = false;
			this.regenProgress = 0;
		}
	}

	/** Tombol "Cek" pada toast buat ulang → buka Editor Post / stage content. */
	async checkRegenToast() {
		const toast = this.regenToast;
		if (!toast) return;
		this.regenToast = null;
		const target = this.mediaItems.find(
			(m) => m.variant_document_id === toast.documentId || m.source_document_id === toast.documentId
		);
		if (target) {
			await this.openPostEditor(target);
		} else if (this.doc && this.doc.id === toast.documentId) {
			await this.gotoStage('content');
		} else {
			this.err = 'Artikel selesai dibuat ulang — buka media untuk mengedit.';
		}
	}

	async generateArticle() {
		const intake = this.intake;
		if (!intake) return;
		this.genBusy = true;
		this.err = '';
		try {
			// target_words di-resolve server dari admin setting per brand.
			const res = await apiRequest(`${this.apiBase}/content-intakes/${intake.id}/create-job`, {
				method: 'POST',
				body: JSON.stringify({ model_tier: this.genMode })
			});
			if (!res.ok) {
				const body = (await res.clone().json().catch(() => null)) as { detail?: string; code?: string } | null;
				if (body?.code === 'insufficient_credits') throw new Error(body.detail ?? 'Saldo TC tidak cukup. Butuh 3 TC.');
				throw new Error(await readError(res, 'Gagal membuat artikel.'));
			}
			const job = (await res.json()) as { job_id: string };
			let failedReason: string | null = null;
			for (let attempt = 0; attempt < 200; attempt++) {
				await new Promise((r) => setTimeout(r, 1500));
				this.genProgress = Math.min(96, attempt * 5);
				this.genStep = attempt < 10 ? 'AI menulis artikel…' : 'Memeriksa kualitas…';
				try {
					const jRes = await apiRequest(`${this.apiBase}/content-jobs/${job.job_id}`);
					if (!jRes.ok) continue;
					const jData = (await jRes.json()) as ContentJob;
					if (jData.status === 'completed' && jData.document_id) {
						const docRes = await apiRequest(`${this.apiBase}/documents/${jData.document_id}`);
						if (docRes.ok) {
							const raw = (await docRes.json()) as {
								id: string;
								title: string;
								meta_title: string | null;
								meta_description: string | null;
								status: string;
								evaluation?: Record<string, unknown>;
								content?: { html?: string };
							};
							this.doc = {
								id: raw.id,
								title: raw.title,
								meta_title: raw.meta_title ?? '',
								meta_description: raw.meta_description ?? '',
								body_html: raw.content?.html ?? '',
								status: raw.status,
								evaluation: raw.evaluation ?? {},
								readiness_score: null,
								readiness_checks: []
							};
							this.genProgress = 100;
							this.genStep = 'Selesai';
							return;
						}
					}
					if (jData.status === 'failed') {
						failedReason = jData.error_message || 'Generasi artikel gagal.';
						break;
					}
				} catch {
					/* network hiccup — lanjut polling */
				}
			}
			if (failedReason) throw new Error(failedReason);
			this.err = 'Waktu proses habis — cek riwayat artikel.';
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Gagal membuat artikel.';
		} finally {
			this.genBusy = false;
			this.genProgress = 0;
			this.genStep = '';
		}
	}

	/* ─── Stage 2: media ────────────────────────────────────────────── */

	async toggleMedia(id: string) {
		const isCurrentlySelected = this.selectedMedia.has(id);
		const media = this.mediaList.find((item) => item.id === id);
		if (!isCurrentlySelected && media && media.is_available === false) {
			this.err = media.availability_reason ?? 'Media sedang menunggu review Admin dan belum dapat dipilih.';
			return;
		}
		const recommendation = this.recommendations.find((item) => item.id === id);
		if (!isCurrentlySelected && recommendation && recommendation.is_available === false) {
			this.err = recommendation.availability_reason ?? 'Media sedang menunggu review Admin dan belum dapat dipilih.';
			return;
		}

		if (isCurrentlySelected) this.selectedMedia.delete(id);
		else this.selectedMedia.add(id);

		this.err = '';
		try {
			if (isCurrentlySelected) {
				const item = this.mediaItems.find((mi) => mi.media_id === id);
				if (item) {
					const res = await apiRequest(`${this.flowBase}/flow/media-items/${item.id}`, { method: 'DELETE' });
					if (!res.ok) throw new Error(await readError(res, 'Gagal menghapus media.'));
				}
			} else {
				const res = await apiRequest(`${this.flowBase}/flow/media`, {
					method: 'POST',
					body: JSON.stringify({ media_ids: [id] })
				});
				if (!res.ok) throw new Error(await readError(res, 'Gagal menambahkan media.'));
			}
			await this.loadFlow(true);
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Gagal mengubah pilihan media.';
			await this.loadFlow(true);
		}
	}

	setThumbnailLocal(itemId: string, value: string) {
		if (!this.flowData) return;
		this.flowData = {
			...this.flowData,
			media_items: this.flowData.media_items.map((mi) => (mi.id === itemId ? { ...mi, thumbnail_url: value } : mi))
		};
	}

	async saveThumbnail(itemId: string, value: string) {
		try {
			const res = await apiRequest(`${this.flowBase}/flow/media-items/${itemId}`, {
				method: 'PATCH',
				body: JSON.stringify({ thumbnail_url: value.trim() || null })
			});
			if (!res.ok) throw new Error(await readError(res, 'Gagal menyimpan thumbnail.'));
			await this.loadFlow(true);
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Gagal menyimpan thumbnail.';
		}
	}

	async uploadAsset(file: File): Promise<string | null> {
		const fd = new FormData();
		fd.append('file', file);
		const res = await apiRequest(`${this.apiBase}/assets`, { method: 'POST', body: fd });
		if (!res.ok) {
			const body = (await res.json().catch(() => null)) as { detail?: string } | null;
			this.thumbUploadError = body?.detail ?? `Upload gagal (${res.status}).`;
			return null;
		}
		const data = (await res.json()) as { url: string };
		return data.url;
	}

	async uploadThumbnail(itemId: string, file: File) {
		this.uploadingThumbId = itemId;
		this.thumbUploadError = '';
		try {
			const url = await this.uploadAsset(file);
			if (url) {
				await this.saveThumbnail(itemId, url);
				this.uploadingThumbId = null;
			} else {
				this.uploadingThumbId = null;
			}
		} catch {
			this.uploadingThumbId = null;
			this.thumbUploadError = 'Upload gagal — periksa koneksi lalu coba lagi.';
		}
	}

	async checkStandards() {
		const activeDocId = this.activeDocId;
		if (!activeDocId || this.selectedMedia.size === 0) return;
		this.busy = true;
		this.err = '';
		try {
			const res = await apiRequest(`${this.apiBase}/documents/${activeDocId}/standard-check`, {
				method: 'POST',
				body: JSON.stringify({ media_ids: [...this.selectedMedia] })
			});
			if (!res.ok) throw new Error(await readError(res, 'Gagal memeriksa standar media.'));
			const data = (await res.json()) as { results?: Array<StandardResult & { media_id: string }> };
			const map: Record<string, StandardResult> = {};
			for (const r of data.results ?? []) map[r.media_id] = r;
			for (const mId of this.selectedMedia) {
				if (!map[mId]) map[mId] = { status: 'pass', media_name: null };
			}
			this.standardResults = map;
			await this.loadFlow(true);
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Gagal memeriksa standar media.';
		} finally {
			this.busy = false;
		}
	}

	async paraphraseArticle(itemId: string) {
		this.paraphraseBusy = itemId;
		this.err = '';
		try {
			const res = await apiRequest(`${this.flowBase}/flow/media-items/${itemId}/paraphrase`, {
				method: 'POST',
				body: JSON.stringify({})
			});
			if (!res.ok) throw new Error(await readError(res, 'Gagal memparafrase.'));
			await this.loadFlow();
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Parafrase gagal.';
		} finally {
			this.paraphraseBusy = null;
		}
	}

	async openAngleModal(itemId: string) {
		this.angleModal = itemId;
		this.paraphraseBusy = itemId;
		try {
			const res = await apiRequest(`${this.flowBase}/flow/media-items/${itemId}/angle-options`);
			if (res.ok) {
				const data = (await res.json()) as { seed: number; options: AngleOption[] };
				this.angleOptions = data.options;
			}
		} catch {
			/* fallback: manual input */
		} finally {
			this.paraphraseBusy = null;
		}
	}

	async submitAngle(itemId: string, angleId: string) {
		const matchedOption = this.angleOptions.find((o) => o.id === angleId);
		this.paraphraseBusy = itemId;
		this.submittingAngleId = angleId;
		this.submittingAngleLabel = matchedOption?.label || '';
		this.angleProgress = 8;
		this.err = '';

		const interval = window.setInterval(() => {
			const prev = this.angleProgress;
			if (prev < 28) this.angleProgress = prev + Math.floor(Math.random() * 5) + 3;
			else if (prev < 58) this.angleProgress = prev + Math.floor(Math.random() * 4) + 2;
			else if (prev < 84) this.angleProgress = prev + Math.floor(Math.random() * 3) + 1;
			else if (prev < 94) this.angleProgress = prev + 0.5;
		}, 240);

		try {
			const res = await apiRequest(`${this.flowBase}/flow/media-items/${itemId}/angle`, {
				method: 'POST',
				body: JSON.stringify({ angle_id: angleId })
			});
			if (!res.ok) throw new Error(await readError(res, 'Gagal menyimpan angle.'));
			window.clearInterval(interval);
			this.angleProgress = 100;
			await new Promise((r) => setTimeout(r, 450));
			this.angleModal = null;
			await this.loadFlow();
		} catch (reason) {
			window.clearInterval(interval);
			this.err = reason instanceof Error ? reason.message : 'Angle gagal.';
		} finally {
			window.clearInterval(interval);
			this.paraphraseBusy = null;
			this.submittingAngleId = null;
			this.submittingAngleLabel = '';
			this.angleProgress = 0;
		}
	}

	async removeItem(itemId: string) {
		if (!confirm('Hapus media ini dari campaign?')) return;
		this.busy = true;
		this.err = '';
		try {
			const res = await apiRequest(`${this.flowBase}/flow/media-items/${itemId}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(await readError(res, 'Gagal menghapus.'));
			await this.loadFlow();
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Gagal menghapus.';
		} finally {
			this.busy = false;
		}
	}

	/* ─── Stage 2: Post Editor ──────────────────────────────────────── */

	async openPostEditor(item: CampaignMediaItem) {
		const docId = item.variant_document_id ?? item.source_document_id ?? this.doc?.id;
		if (!docId) return;
		this.editingPostItem = item;
		this.editPostDocId = docId;
		this.editPostThumbnail = item.thumbnail_url ?? '';
		this.editPostActiveVersion = null;
		this.editPostLoading = true;
		this.editPostBusy = false;
		this.editPostMetaDesc = '';
		this.editPostVersions = [];

		try {
			const [detailRes, htmlRes] = await Promise.all([
				apiRequest(`${this.apiBase}/documents/${docId}`),
				apiRequest(`${this.apiBase}/documents/${docId}/html`)
			]);
			let html = '';
			if (detailRes.ok) {
				const data = (await detailRes.json()) as {
					title?: string;
					meta_title?: string;
					meta_description?: string;
					versions?: DocumentVersion[];
				};
				this.editPostTitle = data.title ?? data.meta_title ?? '';
				this.editPostMetaDesc = data.meta_description ?? '';
				if (Array.isArray(data.versions)) this.editPostVersions = data.versions;
			}
			if (htmlRes.ok) {
				const payload = (await htmlRes.json()) as { content?: string; content_format?: string };
				const raw = payload.content ?? '';
				html = String(payload.content_format ?? '').toLowerCase() === 'markdown' ? markdownToHtml(raw) : raw;
				// Ekstrak H1 pertama → field judul, hapus dari body (konsisten content editor).
				const { h1, body } = extractH1(html);
				if (h1) this.editPostTitle = h1;
				html = body;
			}
			this.editPostHtml = html;
		} catch {
			/* non-fatal */
		} finally {
			this.editPostLoading = false;
		}
	}

	// HTML awal untuk kanvas editor post (dibaca _post-editor.svelte).
	editPostHtml = $state('');

	async savePostEditor(htmlContent: string) {
		if (!this.editingPostItem || !this.editPostDocId) return;
		this.editPostBusy = true;
		try {
			const docRes = await apiRequest(`${this.apiBase}/documents/${this.editPostDocId}`, {
				method: 'PUT',
				body: JSON.stringify({
					title: this.editPostTitle.trim() || undefined,
					meta_title: this.editPostTitle.trim() || undefined,
					meta_description: this.editPostMetaDesc.trim() || null,
					content: htmlContent,
					content_format: 'html',
					change_note: 'flow editor update'
				})
			});
			if (!docRes.ok) throw new Error(await readError(docRes, 'Gagal menyimpan artikel.'));

			const item = this.editingPostItem;
			if (item.thumbnail_url !== this.editPostThumbnail) {
				await this.saveThumbnail(item.id, this.editPostThumbnail);
			}
			await this.loadFlow();
			this.editingPostItem = null;
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Gagal menyimpan artikel.';
		} finally {
			this.editPostBusy = false;
		}
	}

	/* ─── Navigasi tahap ────────────────────────────────────────────── */

	async gotoStage(target: string) {
		// Izinkan keluar dari content yang nyangkut → intake; selain itu hormati guard.
		if (
			target !== 'intake' &&
			this.campaign &&
			(this.campaign.campaign_flow_status === 'checkout' || this.campaign.campaign_flow_status === 'distributing')
		)
			return;
		this.busy = true;
		this.err = '';
		try {
			const res = await apiRequest(`${this.flowBase}/flow`, {
				method: 'POST',
				body: JSON.stringify({ flow_status: target })
			});
			if (!res.ok) throw new Error(await readError(res, 'Gagal berpindah tahap.'));
			await this.loadFlow();
			if (target === 'intake') {
				this.genBusy = false;
				this.genProgress = 0;
				this.genStep = '';
			}
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Gagal berpindah tahap.';
		} finally {
			this.busy = false;
		}
	}

	async advanceToReview() {
		this.busy = true;
		this.err = '';
		try {
			const res = await apiRequest(`${this.flowBase}/flow`, {
				method: 'POST',
				body: JSON.stringify({ flow_status: 'final_review' })
			});
			if (!res.ok) throw new Error(await readError(res, 'Gagal melanjutkan.'));
			await this.loadFlow();
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Gagal melanjutkan.';
		} finally {
			this.busy = false;
		}
	}

	async checkout() {
		this.checkoutBusy = true;
		this.err = '';
		try {
			const res = await apiRequest(`${this.flowBase}/flow/checkout`, {
				method: 'POST',
				body: JSON.stringify({})
			});
			if (!res.ok) throw new Error(await readError(res, 'Checkout gagal.'));
			await this.loadFlow();
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Checkout gagal.';
		} finally {
			this.checkoutBusy = false;
		}
	}

	async startNewArticleFlow() {
		this.busy = true;
		this.err = '';
		try {
			const res = await apiRequest(`${this.flowBase}/flow`, {
				method: 'POST',
				body: JSON.stringify({ new_article: true })
			});
			if (!res.ok) throw new Error(await readError(res, 'Gagal menyiapkan artikel baru.'));
			// Artikel baru = mulai dari nol.
			this.doc = null;
			this.topic = '';
			this.answers = {};
			this.articleText = '';
			this.articleFilename = '';
			this.keyMessagesText = '';
			this.readiness = null;
			this.recommendation = null;
			this.intakeMode = null;
			this.typeCode = '';
			this.intakeStep = 0;
			this.genMode = 'premium';
			this.genBusy = false;
			this.genProgress = 0;
			this.genStep = '';
			this.standardResults = {};
			this.draftSavedAt = null;
			this.editorReloadKey += 1;
			await this.loadFlow();
		} catch (reason) {
			this.err = reason instanceof Error ? reason.message : 'Gagal menyiapkan artikel baru.';
		} finally {
			this.busy = false;
		}
	}

	/* ─── Stage 4: polling distribusi ───────────────────────────────── */

	#distTimer: ReturnType<typeof setInterval> | null = null;
	startDistributionPolling() {
		if (this.#distTimer) return;
		// Parity app lama: poll 5 detik memakai loadFlow() non-silent, sehingga
		// tombol "Segarkan Sekarang" menampilkan "Menyegarkan…" selama muat.
		this.#distTimer = setInterval(() => {
			void this.loadFlow();
			this.distPoll += 1;
		}, 5000);
	}
	stopDistributionPolling() {
		if (this.#distTimer) clearInterval(this.#distTimer);
		this.#distTimer = null;
	}
}

export function createFlowState(segment: () => string, campaignId: () => string) {
	return new FlowState(segment, campaignId);
}
