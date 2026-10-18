<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Publikasi mitra — port section "Publikasi": daftar permintaan artikel per
	// media, filter status, pratinjau/edit draf, setujui, tolak, kirim URL,
	// dan rewrite AI (berbayar dari saldo mitra).
	import { onMount } from 'svelte';
	import { Check, CircleAlert, Eye, ExternalLink, Link2, LoaderCircle, MoreVertical, Paperclip, Pencil, Search, Send, SlidersHorizontal, Sparkles, Trash2, X } from '@lucide/svelte';
	import { Tooltip } from 'bits-ui';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import RichTextEditor from '$lib/components/rich-text-editor.svelte';
	import { mediaDomainLabel } from '$lib/media/domain-label';
	import { sanitizeHtml } from '$lib/sanitize-html';

	type Pub = {
		id: string; order_code: string | null; status: string; client_name: string | null;
		document_title: string | null; document_id: string; media_name: string | null;
		media_category: string | null; media_integration_type: string | null;
		media_publication_mode: string | null; client_price: number; partner_amount: number;
		partner_amount_gross?: number; partner_cut_percent?: number;
		publish_status: string | null; scheduled_at: string | null; published_at: string | null;
		published_url: string | null; remote_url: string | null; rejection_reason: string | null; notes: string | null;
		verification_status: string | null; verification_compliance_score: number | null;
		pr_sentiment: string | null; pr_message_score: number | null;
		thumbnail_url: string | null; featured_image_url: string | null;
		revision_reject_count?: number; revision_reject_limit?: number;
		media_website_url: string | null; publish_progress: number | null;
		rss_import_status: string | null; rss_published_at: string | null;
		partner_draft_edits: Record<string, unknown> | null;
		created_at: string;
	};

	// GET /v1/partner/publications/{id} — payload lebih kaya dari baris tabel
	// (parity PublicationDetail di app lama).
	type PubDetail = Pub & {
		document: { slug: string | null; meta_description: string | null } | null;
		attempts: Array<{
			attempt_number: number; status: string;
			error_code: string | null; error_message: string | null;
			started_at: string | null; completed_at: string | null;
		}>;
		verification: { status: string; compliance_score: number | null; error_message: string | null } | null;
		sentiment_analysis: { sentiment: string; confidence: number | null } | null;
		message_analysis: { consistency_score: number | null } | null;
		// Komentar revisi dari klien (review_comments) — hanya terisi saat klien
		// meminta revisi; sudah ditandai selesai kalau mitra menerimanya.
		revision_comments?: Array<{
			id: string; body: string;
			attachment_url: string | null; attachment_name: string | null; created_at: string | null;
		}>;
	};

	let items = $state<Pub[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');
	let q = $state('');
	// Tab status server-side (parity app lama): "Menunggu" mencakup 5 status
	// sekaligus lewat filter koma, bukan satu status per tab.
	// $derived: label memakai tr(), dan locale baru tersedia setelah root layout
	// memanggil initLocaleFromStorage(); konstanta biasa akan membeku di bahasa awal.
	const statusTabs = $derived<Array<{ key: string; label: string; status: string }>>([
		{ key: '', label: tr('Semua'), status: '' },
		{ key: 'waiting', label: tr('Menunggu'), status: 'waiting_partner,partner_approved,queued,processing,retrying' },
		{ key: 'revision', label: tr('Revisi'), status: 'revision' },
		{ key: 'published', label: tr('Sudah Terbit'), status: 'published' },
		{ key: 'failed', label: tr('Gagal'), status: 'failed' },
		{ key: 'rejected', label: tr('Ditolak Mitra'), status: 'partner_rejected' }
	]);
	const activeLabel = $derived(statusTabs.find((t) => t.key === activeTab)?.label ?? '');
	let activeTab = $state('');
	let pageNo = $state(1);
	const PAGE_SIZE = 20;
	// Status yang masih berjalan → daftar di-polling 4 dtk (parity app lama).
	const LIVE_STATUSES = ['queued', 'processing', 'retrying', 'partner_approved'];

	let selected = $state<Pub | null>(null);
	let detail = $state<PubDetail | null>(null);
	let detailLoading = $state(false);
	// Koreksi URL inline di kotak "URL Terbit" (parity app lama): input read-only
	// sampai mitra menekan "Perbarui URL", lalu disimpan + verifikasi diulang.
	let urlValue = $state('');
	let urlEditMode = $state(false);
	let urlSavedNotice = $state('');
	let urlUpdating = $state(false);

	// Modal-modal terpisah — parity app lama memisahkan tolak, kirim URL cepat,
	// dan editor draf dari dialog Detail (dialog Detail murni baca + setujui).
	let rejectTarget = $state<Pub | null>(null);
	let rejectReason = $state('');
	// Konfirmasi ganda untuk ikon "Setuju & Terbitkan" di kolom Aksi: satu klik
	// belum menerbitkan apa pun, keputusan diambil di modal ini.
	let publishConfirm = $state<Pub | null>(null);
	let publishBusy = $state(false);
	let quickUrlTarget = $state<Pub | null>(null);
	let quickUrlValue = $state('');
	let quickUrlBusy = $state(false);
	let quickUrlError = $state('');
	let draftPub = $state<Pub | null>(null);
	let draft = $state<any>(null);
	let draftLoading = $state(false);
	let coverInput = $state<HTMLInputElement | undefined>(undefined);
	let coverBusy = $state(false);
	let coverError = $state('');

	// Sesi harus divalidasi lebih dulu (refresh token), baru daftar dimuat.
	let ready = $state(false);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			ready = true;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		// Menu aksi memakai posisi fixed: tutup saat digulir atau ukuran jendela
		// berubah supaya tidak tampak melayang menjauh dari barisnya. capture=true
		// agar gulir di dalam kontainer (bukan hanya window) ikut tertangkap.
		const onViewportChange = () => closeRowMenu();
		window.addEventListener('scroll', onViewportChange, true);
		window.addEventListener('resize', onViewportChange);
		return () => {
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
			window.removeEventListener('scroll', onViewportChange, true);
			window.removeEventListener('resize', onViewportChange);
		};
	});

	// Muat ulang setiap tab filter atau halaman berubah. Sebelumnya tombol tab
	// dan tombol halaman hanya mengubah state tanpa memanggil load(), sehingga
	// filter status tampak tidak berfungsi (daftar tidak pernah dimuat ulang).
	$effect(() => {
		if (!ready) return;
		// `loading` hanya untuk perubahan filter/halaman — polling di bawah tidak
		// menyentuhnya supaya tabel tidak berkedip tiap 4 detik.
		loading = true;
		void load().finally(() => (loading = false));
	});

	async function load() {
		const tab = statusTabs.find((t) => t.key === activeTab);
		const params = new URLSearchParams({
			limit: String(PAGE_SIZE),
			offset: String((pageNo - 1) * PAGE_SIZE)
		});
		if (tab?.status) params.set('status', tab.status);
		const res = await apiRequest(`/v1/partner/publications?${params}`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat publikasi.'));
			return;
		}
		error = '';
		const data = await res.json();
		items = data.publications ?? [];
		total = data.total ?? 0;
	}

	// Polling ringan selama masih ada artikel yang diproses worker.
	$effect(() => {
		if (!items.some((p) => LIVE_STATUSES.includes(p.status))) return;
		const timer = setInterval(() => void load(), 4000);
		return () => clearInterval(timer);
	});

	/** Ekspor CSV (endpoint sama dengan app lama: format=csv + filter status). */
	async function exportCsv() {
		const tab = statusTabs.find((t) => t.key === activeTab);
		const params = new URLSearchParams({ format: 'csv' });
		if (tab?.status) params.set('status', tab.status);
		const res = await apiRequest(`/v1/partner/publications?${params}`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal mengekspor CSV.'));
			return;
		}
		const blob = await res.blob();
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `publikasi-mitra-${new Date().toISOString().slice(0, 10)}.csv`;
		a.click();
		URL.revokeObjectURL(url);
	}

	const filtered = $derived(
		items.filter((p) => {
			if (!q.trim()) return true;
			const term = q.trim().toLowerCase();
			return (
				(p.document_title ?? '').toLowerCase().includes(term) ||
				(p.media_name ?? '').toLowerCase().includes(term) ||
				(p.order_code ?? '').toLowerCase().includes(term)
			);
		})
	);

	async function openDetail(p: Pub) {
		selected = p;
		detail = null;
		detailLoading = true;
		urlEditMode = false;
		urlSavedNotice = '';
		urlValue = p.published_url ?? '';
		error = '';
		const res = await apiRequest(`/v1/partner/publications/${p.id}`);
		if (res.ok) {
			detail = await res.json();
			urlValue = detail?.published_url ?? p.published_url ?? '';
		} else {
			error = await readApiError(res, tr('Gagal memuat detail publikasi.'));
		}
		detailLoading = false;
	}

	function closeDetail() {
		selected = null;
		detail = null;
		detailLoading = false;
		urlEditMode = false;
		urlSavedNotice = '';
	}

	/** Editor draf — modal terpisah (parity tombol "Edit" app lama). */
	async function openDraftEditor(p: Pub) {
		draftPub = p;
		draft = null;
		draftLoading = true;
		error = '';
		coverError = '';
		coverBusy = false;
		draft = await loadDraft(p);
		draftLoading = false;
		if (draft) void measureDraftImages();
	}

	// ── Ukuran gambar di draf: jumlah + total berat (mis. "3 gambar · 412 KB")
	// muncul di sebelah teks Sumber berkat HEAD fetch ke tiap URL gambar;
	// gambar luar yang tidak membongkar Content-Length dihitung "tak terukur". ──
	let draftImgCount = $state(0);
	let draftImgBytes = $state<number | null>(null);
	let draftImgUnknown = $state(false);
	let draftImgFallbackCount = $state(0);
	let draftImgToken = 0;

	const fmtBytesID = (b: number) =>
		b >= 1048576 ? `${(b / 1048576).toFixed(1).replace('.', ',')} MB` : `${Math.round(b / 1024).toLocaleString('id-ID')} KB`;

	function collectDraftImageURLs(coverURL: string | null | undefined, html: string | null | undefined): string[] {
		const urls = new Set<string>();
		if (coverURL) urls.add(coverURL);
		const re = /<img[^>]*\ssrc=["']([^"']+)["']/gi;
		for (const m of (html ?? '').matchAll(re)) urls.add(m[1]);
		return [...urls];
	}

	async function measureDraftImages() {
		const token = ++draftImgToken;
		const urls = draft ? collectDraftImageURLs(draft.featured_image_url, draft.content_html) : [];
		draftImgCount = urls.length;
		draftImgBytes = null;
		draftImgUnknown = false;
		draftImgFallbackCount = 0;
		if (!urls.length) return;
		let total = 0;
		let unknown = false;
		await Promise.all(urls.map(async (u) => {
			try {
				const res = await fetch(u, { method: 'HEAD' });
				const len = res.headers.get('content-length');
				if (res.ok && len && Number(len) > 0) {
					total += Number(len);
					return;
				}
				// Header length sering hilang di proxy dev — ukur langsung lewat blob
				// untuk gambar satu asal (origin sama); gambar luar tetap tak terukur.
				const abs = new URL(u, window.location.href);
				if (abs.origin !== window.location.origin) {
					unknown = true;
					return;
				}
				const blob = await fetch(abs.href).then((r2) => (r2.ok ? r2.blob() : null));
				if (blob && blob.size > 0) total += blob.size;
				else unknown = true;
			} catch {
				unknown = true;
			}
		}));
		if (token !== draftImgToken) return;
		draftImgUnknown = unknown;
		draftImgBytes = unknown ? null : total;
		draftImgFallbackCount = unknown ? urls.length : 0;
	}

	function closeDraftEditor() {
		draftPub = null;
		draft = null;
		draftImgCount = 0;
		draftImgBytes = null;
		draftImgUnknown = false;
		draftImgFallbackCount = 0;
	}

	/**
	 * Setujui artikel. `publishStatus` wajib eksplisit (parity app lama):
	 * "publish" = setujui & terbitkan, "draft" = setujui sebagai draf.
	 * Tanpa ini server memakai default mode media, sehingga mitra tidak bisa
	 * memilih draf saat mode medianya "publish".
	 */
	async function approve(pub: Pub, publishStatus: 'draft' | 'publish'): Promise<boolean> {
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/publications/${pub.id}/approve`, {
				method: 'POST',
				body: JSON.stringify({ publish_status: publishStatus })
			});
			if (!res.ok) {
				error = await readApiError(res, "Permintaan gagal.");
				return false;
			}
			notice = publishStatus === 'draft' ? tr('Artikel disetujui sebagai draf.') : tr('Artikel disetujui dan diterbitkan.');
			if (detail && detail.id === pub.id) await openDetail(pub);
			await load();
			// Mode manual + "Setujui & Terbitkan": langsung sodorkan input URL
			// artikel yang mitra terbitkan sendiri — satu alur, parity app lama.
			if (publishStatus === 'publish' && isManualRow(pub)) openQuickUrl(pub);
			return true;
		} finally {
			busy = false;
		}
	}

	/** Konfirmasi kedua dari ikon pesawat; modal tetap terbuka bila server menolak. */
	async function confirmPublishNow() {
		const target = publishConfirm;
		if (!target || publishBusy) return;
		publishBusy = true;
		try {
			if (await approve(target, 'publish')) publishConfirm = null;
		} finally {
			publishBusy = false;
		}
	}

	/** Terima revisi klien → server mengantrekan ulang artikel ke CMS. */
	async function acceptRevision(pub: Pub) {
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/publications/${pub.id}/revision/accept`, { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, "Permintaan gagal.");
				return;
			}
			notice = tr('Revisi diterima. Artikel dikirim ulang ke media Anda.');
			if (detail && detail.id === pub.id) await openDetail(pub);
			await load();
		} finally {
			busy = false;
		}
	}

	async function rejectNow() {
		if (!rejectTarget) return;
		busy = true;
		error = '';
		try {
			const isRevision = rejectTarget.status === 'revision';
			const url = isRevision
				? `/v1/partner/publications/${rejectTarget.id}/revision/reject`
				: `/v1/partner/publications/${rejectTarget.id}/reject`;
			const res = await apiRequest(url, {
				method: 'POST',
				body: JSON.stringify({ reason: rejectReason.trim() })
			});
			if (!res.ok) {
				error = await readApiError(res, "Permintaan gagal.");
				return;
			}
			const target = rejectTarget;
			notice = isRevision ? tr('Revisi ditolak. Klien akan memperbaiki artikelnya.') : 'Artikel ditolak.';
			rejectTarget = null;
			rejectReason = '';
			if (detail && detail.id === target.id) await openDetail(target);
			await load();
		} finally {
			busy = false;
		}
	}

	function openQuickUrl(p: Pub) {
		quickUrlTarget = p;
		quickUrlValue = p.published_url ?? '';
		quickUrlError = '';
	}

	/** Validasi URL terbit — aturan sama dengan server (http/https). */
	function validatePublishedUrl(value: string): string | null {
		const v = value.trim();
		if (!v) return tr('URL artikel tidak boleh kosong.');
		if (!/^https?:\/\/\S+$/i.test(v)) return tr('URL harus diawali http:// atau https://');
		return null;
	}

	async function submitQuickUrl() {
		if (!quickUrlTarget) return;
		const value = quickUrlValue.trim();
		const invalid = validatePublishedUrl(value);
		if (invalid) {
			quickUrlError = invalid;
			return;
		}
		quickUrlBusy = true;
		quickUrlError = '';
		try {
			const res = await apiRequest(`/v1/partner/publications/${quickUrlTarget.id}/url`, {
				method: 'POST',
				body: JSON.stringify({ published_url: value })
			});
			if (!res.ok) {
				quickUrlError = await readApiError(res, tr('Gagal menyimpan URL.'));
				return;
			}
			notice = tr('URL artikel tersimpan — artikel ditandai terbit.');
			quickUrlTarget = null;
			await load();
		} finally {
			quickUrlBusy = false;
		}
	}

	/** Perbarui URL dari kotak "URL Terbit" (hanya status published). */
	async function updatePublishedUrl() {
		if (!detail) return;
		const value = urlValue.trim();
		const invalid = validatePublishedUrl(value);
		if (invalid) {
			error = invalid;
			return;
		}
		urlUpdating = true;
		error = '';
		urlSavedNotice = '';
		try {
			const res = await apiRequest(`/v1/partner/publications/${detail.id}/url`, {
				method: 'POST',
				body: JSON.stringify({ published_url: value })
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menyimpan URL.'));
				return;
			}
			const saved = tr('URL tersimpan & verifikasi sedang dijalankan ulang.');
			urlEditMode = false;
			const current = selected;
			await load();
			// openDetail mereset catatan; set ulang setelah detail dimuat.
			if (current) await openDetail(current);
			urlSavedNotice = saved;
		} finally {
			urlUpdating = false;
		}
	}

	async function saveDraft(e: SubmitEvent) {
		e.preventDefault();
		if (!draftPub || !draft) return;
		busy = true;
		error = '';
		try {
			const payload: Record<string, unknown> = {
				title: draft.title,
				content_html: draft.content_html,
				meta_description: draft.meta_description ?? '',
				featured_image_url: draft.featured_image_url || null
			};
			const res = await apiRequest(`/v1/partner/publications/${draftPub.id}/draft`, {
				method: 'PUT',
				body: JSON.stringify(payload)
			});
			if (!res.ok) {
				error = await readApiError(res, "Permintaan gagal.");
				return;
			}
			notice = 'Perubahan draf tersimpan.';
			void measureDraftImages();
			await load();
		} finally {
			busy = false;
		}
	}

	async function rewrite() {
		if (!draftPub || !draft) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/publications/${draftPub.id}/rewrite`, {
				method: 'POST',
				body: JSON.stringify({ title: draft.title, content_html: draft.content_html })
			});
			const data = await res.json();
			if (!res.ok) {
				error = tr('Rewrite gagal. Coba lagi nanti.');
				return;
			}
			draft.title = data.title;
			draft.content_html = data.content_html;
			notice = `Rewrite selesai (biaya Rp ${(data.amount_charged_idr ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}). Tinjau lalu simpan draf.`;
		} finally {
			busy = false;
		}
	}

	// Unggah gambar draf mitra: satu jalur untuk gambar inline di editor dan
	// untuk mengganti gambar utama, memakai endpoint draf yang sudah ada.
	async function uploadDraftImage(file: File): Promise<string> {
		if (!draftPub) throw new Error(tr('Artikel belum dipilih.'));
		const form = new FormData();
		form.append('file', file);
		const res = await apiRequest(`/v1/partner/publications/${draftPub.id}/draft/image`, { method: 'POST', body: form });
		if (!res.ok) throw new Error(await readApiError(res, tr('Gagal mengunggah gambar.')));
		const data = (await res.json()) as { url?: string };
		if (!data.url) throw new Error(tr('Gambar tidak diterima server.'));
		return data.url;
	}

	async function pickCover(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file || !draft) return;
		coverBusy = true;
		coverError = '';
		try {
			draft.featured_image_url = await uploadDraftImage(file);
			void measureDraftImages();
		} catch (err) {
			coverError = err instanceof Error ? err.message : tr('Gagal mengunggah gambar.');
		} finally {
			coverBusy = false;
		}
	}

	/** Tooltip header kolom Mode Integrasi: siapa yang menerbitkan artikel. */
	const modeColumnHint = () =>
		tr('Siapa yang menerbitkan artikel. API = di-push otomatis ke situs mitra lewat WordPress/Blogger/Custom/API key; Manual = mitra menempel artikelnya sendiri lalu mengirim URL artikelnya.');

	const statusLabel: Record<string, string> = $derived({
		waiting_partner: tr('Menunggu persetujuan'), partner_approved: tr('Disetujui'), partner_rejected: tr('Ditolak'),
		queued: tr('Antrean'), processing: tr('Diproses'), published: tr('Terbit'), failed: tr('Gagal'),
		pending: tr('Menunggu'), cancelled: tr('Dibatalkan'), retrying: tr('Coba ulang'), revision: tr('Perlu Revisi')
	});
	const statusTone: Record<string, string> = {
		waiting_partner: 'warning', partner_approved: 'accent', partner_rejected: 'destructive',
		published: 'success', failed: 'destructive', processing: 'accent', queued: 'accent',
		revision: 'warning'
	};
	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}`;
	const dt = (v: string | null) => (v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—');

	// ── Rincian penghasilan (popover) ────────────────────────────────────
	// amount = yang diterima mitra (sudah dipotong platform), gross = angka
	// sebelum potongan. Baris historis tanpa gross → potongan tampil "—" dan
	// pendapatan = amount, jangan mengarang angka potongan.
	const incomeGross = (p: Pub) => Math.max(Number(p.partner_amount_gross ?? p.partner_amount) || 0, 0);
	const incomeCut = (p: Pub) => Math.max(incomeGross(p) - (Number(p.partner_amount) || 0), 0);
	const cutPercentLabel = (p: Pub) => {
		const g = incomeGross(p);
		if (!g || incomeCut(p) <= 0) return null;
		return Number(p.partner_cut_percent ?? 5);
	};
	// Popover rincian; satu terbuka sekaligus per baris (bukan global).
	let openIncomeID = $state<string | null>(null);

	// ── Parity kolom tabel app lama ──────────────────────────────────────
	// Badge menunjukkan JENIS integrasinya: 'publish' berarti artikel dipindah
	// lewat jalur API situs mitra (WordPress/Blogger/Custom/Web API key), sisanya
	// mitra menempel sendiri (Manual).
	const pubMode = (m: string | null) => (m ?? 'draft') === 'publish'
		? { label: tr('API'), tone: 'var(--success)' }
		: { label: tr('Manual'), tone: 'var(--warning)' };
	const mediaHost = (u: string | null) => (u ?? '').replace(/^https?:\/\//, '').replace(/\/+$/, '');
	const integrationLabel: Record<string, string> = $derived({
		wordpress: 'WordPress', blogger: 'Blogger', custom_web: 'Custom Web', tulisio_api: 'Tulisio API', manual: tr('Manual')
	});
	const isManualRow = (p: Pub) => (p.media_publication_mode ?? 'draft') !== 'publish';
	const pendingUrl = (p: Pub) => p.status === 'partner_approved' && isManualRow(p) && !p.published_url;
	// Tolak kini berdiri sendiri di kolom Aksi (tong sampah), bukan lagi item
	// menu titik-tiga. Batas penolakan revisi tetap dihormati seperti menu lama.
	const canReject = (p: Pub) =>
		p.status === 'waiting_partner' ||
		(p.status === 'revision' && (p.revision_reject_count ?? 0) < (p.revision_reject_limit ?? 3));
	const rejectLabel = (p: Pub) => (p.status === 'revision' ? tr('Tolak revisi') : tr('Tolak publikasi'));
	const rssBadge = (s: string | null) => (!s || s === 'not_imported') ? null : s === 'published' ? tr('Terbit di situs mitra') : tr('Diimport mitra');
	const LIVE = ['queued', 'processing', 'retrying'];

	// ── Parity dialog "Detail publikasi" app lama ────────────────────────
	// Label status memakai PUB_STATUS_LABEL app lama (beda tipis dari label tabel).
	const detailStatusLabel: Record<string, string> = $derived({
		pending: 'Pending', waiting_partner: tr('Menunggu'), partner_approved: tr('Disetujui'),
		partner_rejected: tr('Ditolak'), queued: tr('Diantrekan'), processing: tr('Diproses'),
		retrying: tr('Mencoba ulang'), published: tr('Terbit'), failed: tr('Gagal'), cancelled: tr('Dibatalkan'),
		revision: tr('Revisi dari klien')
	});
	// ACTION_COLOR app lama → nada badge status di dialog.
	const detailStatusTone: Record<string, string> = {
		waiting_partner: 'amber', partner_approved: 'blue', partner_rejected: 'red', published: 'green', failed: 'red',
		revision: 'amber'
	};
	const TONE: Record<string, { bg: string; fg: string }> = {
		green: { bg: 'var(--success-soft)', fg: 'var(--success)' },
		amber: { bg: 'var(--warning-soft)', fg: 'var(--warning)' },
		red: { bg: '#fdecee', fg: 'var(--destructive)' },
		blue: { bg: 'var(--accent)', fg: 'var(--primary)' },
		gray: { bg: 'var(--muted)', fg: 'var(--muted-foreground)' }
	};
	const verificationLabels: Record<string, string | null> = $derived({
		passed: tr('URL terverifikasi'), failed: tr('URL bermasalah'), unavailable: null
	});
	const verificationTones: Record<string, string> = { passed: 'green', failed: 'red', unavailable: 'amber' };
	const sentimentLabels: Record<string, string> = $derived({
		positive: tr('Positif'), neutral: tr('Netral'), negative: tr('Negatif')
	});
	const attemptLabels: Record<string, string> = $derived({
		published: tr('Terbit'), retrying: tr('Mencoba ulang'), failed: tr('Gagal'), queued: tr('Diantrekan'),
		processing: tr('Diproses'), partner_approved: tr('Disetujui mitra'), waiting_partner: tr('Menunggu')
	});
	/** Badge verifikasi URL — null berarti disembunyikan (status unavailable). */
	const verificationBadge = (v: PubDetail['verification']) => {
		if (!v?.status) return null;
		const label = verificationLabels[v.status];
		if (!label) return null;
		return { label: v.compliance_score != null ? `${label} · ${v.compliance_score}/100` : label, tone: verificationTones[v.status] ?? 'gray' };
	};
	const sentimentBadge = (s: PubDetail['sentiment_analysis']) => {
		if (!s?.sentiment) return null;
		const pct = s.confidence != null ? ` · ${Math.round(s.confidence * 100)}%` : '';
		return {
			label: `${tr('Sentiment')} ${sentimentLabels[s.sentiment] ?? s.sentiment}${pct}`,
			tone: s.sentiment === 'positive' ? 'green' : s.sentiment === 'negative' ? 'red' : 'gray'
		};
	};
	const messageBadge = (m: PubDetail['message_analysis']) => {
		if (!m) return null;
		const score = m.consistency_score;
		return {
			label: score != null ? `${tr('Message')} ${score}/100` : `${tr('Message')} —`,
			tone: score != null && score >= 70 ? 'green' : score != null ? 'amber' : 'gray'
		};
	};
	/** URL terbit efektif: koreksi mitra (published_url) → hasil integrasi (remote_url). */
	const effectiveUrl = (row: { published_url?: string | null; remote_url?: string | null }) =>
		row.published_url?.trim() || row.remote_url?.trim() || null;
	/** Pesan teknis verifikasi diterjemahkan jadi petunjuk yang ramah. */
	const verificationHint = (msg: string | null | undefined) => {
		if (!msg) return null;
		if (/no published url|published url|belum ada/i.test(msg)) {
			return tr('URL artikel terbit belum terisi — lengkapi URL di kolom di bawah lalu klik "Perbarui URL" agar Tauthub bisa memverifikasi artikel.');
		}
		return msg;
	};

	// Menu aksi titik-tiga per baris (parity PublicationRowActions).
	let rowMenu = $state<string | null>(null);
	function closeRowMenu() {
		rowMenu = null;
	}

	// Menu diposisikan `fixed` mengikuti tombolnya: tidak ikut mempengaruhi layout
	// tabel, dan dibuka ke atas saat ruang di bawah tidak cukup (mis. baris
	// terakhir). Untuk arah ke atas, DASAR menu yang ditempelkan ke tombol, jadi
	// tingginya tidak perlu ditebak.
	const MENU_SPACE_NEEDED = 230; // perkiraan tinggi menu 6 item
	let menuRight = $state(0);
	let menuTop = $state(0);
	let menuBottom = $state<number | null>(null);

	function positionRowMenu(trigger: HTMLElement) {
		const rect = trigger.getBoundingClientRect();
		menuRight = Math.max(8, window.innerWidth - rect.right);
		const spaceBelow = window.innerHeight - rect.bottom;
		const spaceAbove = rect.top;
		if (spaceBelow < MENU_SPACE_NEEDED && spaceAbove > spaceBelow) {
			menuBottom = Math.max(8, window.innerHeight - rect.top + 4);
			menuTop = 0;
		} else {
			menuBottom = null;
			menuTop = rect.bottom + 4;
		}
	}

	const menuPositionStyle = $derived(
		menuBottom !== null ? `right: ${menuRight}px; bottom: ${menuBottom}px;` : `right: ${menuRight}px; top: ${menuTop}px;`
	);
	let showPreview = $state(false);
	let previewPub = $state<Pub | null>(null);
	let previewLoading = $state(false);

	/** Muat draf dari API — dipakai pratinjau dan editor (endpoint sama). */
	async function loadDraft(p: Pub) {
		const res = await apiRequest(`/v1/partner/publications/${p.id}/draft`);
		if (res.ok) return await res.json();
		return { error: await readApiError(res, "Permintaan gagal.") };
	}

	async function previewDraft(p: Pub) {
		previewPub = p;
		showPreview = true;
		previewLoading = true;
		draft = null;
		draft = await loadDraft(p);
		previewLoading = false;
	}

	/**
	 * Klik item menu baris. stopPropagation wajib: menu ini berada di dalam
	 * `<tr>` yang onclick-nya membuka Detail, sehingga tanpa ini memilih
	 * "Pratinjau draf" ikut membuka modal Detail di belakangnya.
	 */
	function menuAction(event: MouseEvent, p: Pub, action: 'detail' | 'preview' | 'edit' | 'url' | 'reject' | 'publish' | 'draft' | 'accept-revision') {
		event.stopPropagation();
		void rowAction(p, action);
	}

	/** Aksi dari menu baris (parity PublicationRowActions app lama). */
	async function rowAction(p: Pub, action: 'detail' | 'preview' | 'edit' | 'url' | 'reject' | 'publish' | 'draft' | 'accept-revision') {
		closeRowMenu();
		if (action === 'detail') {
			await openDetail(p);
			return;
		}
		if (action === 'preview') {
			await previewDraft(p);
			return;
		}
		if (action === 'edit') {
			await openDraftEditor(p);
			return;
		}
		if (action === 'url') {
			openQuickUrl(p);
			return;
		}
		if (action === 'reject') {
			rejectTarget = p;
			rejectReason = '';
			return;
		}
		if (action === 'publish') {
			await approve(p, 'publish');
			return;
		}
		if (action === 'accept-revision') {
			await acceptRevision(p);
			return;
		}
		await approve(p, 'draft');
	}
	let statusMenuOpen = $state(false);
</script>

<PageTitle title={tr("Publikasi")} />

<svelte:window onclick={closeRowMenu} onkeydown={(e) => { if (e.key === 'Escape') { closeRowMenu(); publishConfirm = null; openIncomeID = null; } }} />

<!-- ── Penghasilan: nominal = pendapatan bersih; klik membuka rincian
     (harga diajukan · potongan platform · pendapatan diterima). ───────── -->
{#snippet incomeCell(p: Pub)}
	{@const gross = incomeGross(p)}
	{@const cut = incomeCut(p)}
	{@const pct = cutPercentLabel(p)}
	<div class="relative flex justify-end">
		<button
			class="inline-flex items-center gap-1 text-right tabular-nums underline-offset-2 hover:underline"
			aria-haspopup="dialog"
			aria-expanded={openIncomeID === p.id}
			title={tr('Lihat rincian penghasilan')}
			onclick={(e) => { e.stopPropagation(); openIncomeID = openIncomeID === p.id ? null : p.id; }}
		>
			{rp(p.partner_amount)}
			<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>
		</button>
		{#if openIncomeID === p.id}
			<!-- Penutup transparan: klik di luar menutup popover tanpa membuka detail. -->
			<div
				class="fixed inset-0 z-40"
				role="presentation"
				onclick={(e) => { e.stopPropagation(); openIncomeID = null; }}
			></div>
			<div
				class="app-card absolute right-0 top-full z-50 mt-1 w-64 p-3 text-left shadow-lg"
				style="border: 1px solid var(--border);"
				role="dialog"
				tabindex="-1"
				aria-label={tr('Rincian penghasilan')}
				onclick={(e) => e.stopPropagation()}
				onkeydown={(e) => { if (e.key === 'Escape') openIncomeID = null; }}
			>
				<p class="mb-2 text-[11px] font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('Rincian penghasilan')}</p>
				<dl class="space-y-1.5 text-[13px] tabular-nums">
					<div class="flex items-center justify-between gap-3">
						<dt class="text-muted-foreground">{tr('Harga diajukan')}</dt>
						<dd>{rp(gross)}</dd>
					</div>
					<div class="flex items-center justify-between gap-3" style="color: var(--destructive, #d92d20);">
						<dt>{tr('Potongan platform')}{pct !== null ? ` (${pct}%)` : ''}</dt>
						<dd>{pct !== null ? `−${rp(cut)}` : '—'}</dd>
					</div>
					<div class="flex items-center justify-between gap-3 border-t pt-1.5 font-semibold" style="border-color: var(--border); color: var(--success);">
						<dt>{tr('Pendapatan Anda')}</dt>
						<dd>{rp(p.partner_amount)}</dd>
					</div>
				</dl>
			</div>
		{/if}
	</div>
{/snippet}

<section>
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">{tr("Publikasi")}</h1>
		<p class="mt-1 text-sm text-muted-foreground">{total} {tr("permintaan artikel pada media Anda.")}</p>
	</header>

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	<form class="app-card mt-4 flex flex-wrap items-center gap-2 p-3" onsubmit={(e) => e.preventDefault()}>
		<div class="hidden flex-wrap items-center gap-1 md:flex" role="tablist" aria-label={tr("Filter status publikasi")}>
			{#each statusTabs as t (t.key)}
				<button
					type="button"
					role="tab"
					aria-selected={activeTab === t.key}
					class="rounded-lg px-3 py-1.5 text-sm font-medium"
					style={activeTab === t.key
						? 'background: var(--primary); color: var(--primary-foreground);'
						: 'color: var(--muted-foreground);'}
					onclick={() => {
						activeTab = t.key;
						pageNo = 1;
					}}>{t.label}</button>
			{/each}
		</div>
		<!-- Mobile: filter status berupa tombol ikon + dropdown. -->
		<div class="relative md:hidden">
			<button
				type="button"
				class="app-btn app-btn-soft !min-h-9 !px-3"
				aria-haspopup="menu"
				aria-expanded={statusMenuOpen}
				aria-label={tr('Filter status publikasi')}
				title={tr('Filter status publikasi')}
				onclick={() => (statusMenuOpen = !statusMenuOpen)}
			>
				<SlidersHorizontal size={15} />
			</button>
			{#if statusMenuOpen}
				<button class="fixed inset-0 z-[5] cursor-default" type="button" aria-label={tr('Tutup')} onclick={() => (statusMenuOpen = false)}></button>
				<div class="absolute left-0 top-[calc(100%+6px)] z-10 w-56 overflow-hidden rounded-xl border bg-card p-1 shadow-lg" style="border-color: var(--border);" role="menu">
					{#each statusTabs as t (t.key)}
						<button
							type="button"
							role="menuitemradio"
							aria-checked={activeTab === t.key}
							class="flex w-full items-center justify-between gap-2 rounded-lg px-3 py-2 text-left text-sm"
							style={activeTab === t.key ? 'color: var(--primary); font-weight: 700;' : 'color: var(--foreground);'}
							onclick={() => { activeTab = t.key; pageNo = 1; statusMenuOpen = false; }}
						>
							{t.label}
							{#if activeTab === t.key}<Check size={14} />{/if}
						</button>
					{/each}
				</div>
			{/if}
		</div>
		<label class="app-input flex min-w-[200px] flex-1 items-center gap-2 !py-1.5">
			<Search size={15} class="text-muted-foreground" />
			<input class="w-full bg-transparent text-sm outline-none" placeholder={tr("Cari judul, media, atau nomor order…")} bind:value={q} />
		</label>
		<button class="app-btn app-btn-soft" type="button" onclick={() => void exportCsv()}>{tr("Ekspor CSV")}</button>
	</form>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat publikasi…")}</p>
	{:else if filtered.length === 0}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">
				{#if q}
					{tr('Tidak ada publikasi yang cocok dengan pencarian')} “{q}”{#if activeLabel} {tr('pada filter')} <strong>{activeLabel}</strong>{/if}.
				{:else if activeLabel}
					{tr('Tidak ada publikasi dengan status')} <strong>{activeLabel}</strong>.
				{:else}
					{tr('Belum ada permintaan publikasi.')}
				{/if}
			</p>
		</div>
	{:else}
		<!-- Desktop: tabel asli, tidak diubah (≥768px). -->
		<div class="app-card mt-4 hidden overflow-x-auto md:block">
			<table class="w-full text-sm">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="w-[38%] px-4 py-2.5 font-medium">{tr('Artikel')}</th>
						<th class="px-4 py-2.5 font-medium">
							<span class="inline-flex items-center gap-1">
								{tr("Mode Integrasi")}
								<Tooltip.Provider delayDuration={0}>
									<Tooltip.Root>
										<Tooltip.Trigger>
											{#snippet child({ props })}
												<button
													{...props}
													type="button"
													class="inline-flex cursor-help items-center text-muted-foreground"
													aria-label={modeColumnHint()}
												>
													<CircleAlert size={12} />
												</button>
											{/snippet}
										</Tooltip.Trigger>
										<Tooltip.Portal>
											<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
												<Tooltip.Arrow class="uiTipArrow" />
												{modeColumnHint()}
											</Tooltip.Content>
										</Tooltip.Portal>
									</Tooltip.Root>
								</Tooltip.Provider>
							</span>
						</th>
						<th class="px-4 py-2.5 font-medium">{tr("Media")}</th>
						<th class="px-4 py-2.5 text-right font-medium">{tr("Penghasilan")}</th>
						<th class="px-4 py-2.5 font-medium">Status</th>
						<th class="w-[136px] whitespace-nowrap px-4 py-2.5 text-right font-medium">{tr('Aksi')}</th>
					</tr>
				</thead>
				<tbody>
					{#each filtered as p (p.id)}
						<tr class="cursor-pointer border-t align-top hover:bg-secondary" onclick={() => void openDetail(p)}>
							<td class="px-4 py-2.5">
								<div class="flex items-center gap-2.5">
										{#if p.thumbnail_url ?? p.featured_image_url}
											<img
												src={p.thumbnail_url ?? p.featured_image_url}
												alt=""
												class="h-10 w-10 flex-none rounded-lg border object-cover"
												style="border-color: var(--border); background-color: var(--muted);"
												loading="lazy"
											/>
										{/if}
									<div class="min-w-0 max-w-[330px]">
										<p class="line-clamp-2 font-medium" title={p.document_title ?? tr('Tanpa judul')}>{p.document_title ?? tr('Tanpa judul')}</p>
										{#if p.order_code}<p class="text-xs text-muted-foreground">{p.order_code}</p>{/if}
									</div>
								</div>
							</td>
								<td class="px-4 py-2.5">
									<span class="whitespace-nowrap rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: {pubMode(p.media_publication_mode).tone}22; color: {pubMode(p.media_publication_mode).tone};">
										{pubMode(p.media_publication_mode).label}
									</span>
								</td>
									<td class="px-4 py-2.5">
										<p class="font-medium">{mediaDomainLabel(p.media_website_url, p.media_name ?? '')}</p>
										{#if effectiveUrl(p)}
										<a
											class="text-xs text-muted-foreground underline underline-offset-2 hover:text-foreground"
											href={effectiveUrl(p) ?? undefined}
											target="_blank"
											rel="noreferrer"
											onclick={(e) => e.stopPropagation()}
										>
											{tr('Lihat Artikel')}
										</a>
									{/if}
									{#if p.media_integration_type}
										<p class="mt-0.5 text-[11px] text-muted-foreground">{integrationLabel[p.media_integration_type] ?? p.media_integration_type}</p>
									{/if}
								</td>
							<td class="px-4 py-2.5 text-right tabular-nums">
								{@render incomeCell(p)}
							</td>
							<td class="px-4 py-2.5">
								{@render statusBadge(p, false)}
								{@render statusExtras(p)}
							</td>
							<td class="px-4 py-2.5 text-right">
								{@render rowActions(p)}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Mobile: setiap publikasi menjadi kartu vertikal (<768px). Data sama
			dengan tabel (snippet bersama); min-w-0 di flex child & grid supaya
			judul, domain, dan nominal tidak terpecah per karakter. -->
		<div class="mt-4 flex flex-col gap-2.5 md:hidden">
			{#each filtered as p (p.id)}
				<div
					class="app-card cursor-pointer p-3"
					role="button"
					tabindex="0"
					onclick={() => void openDetail(p)}
					onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); void openDetail(p); } }}
				>
					<div class="flex items-start gap-3">
						{#if p.thumbnail_url ?? p.featured_image_url}
							<img
								src={p.thumbnail_url ?? p.featured_image_url}
								alt=""
								class="h-11 w-11 flex-none rounded-lg border object-cover"
								style="border-color: var(--border); background-color: var(--muted);"
								loading="lazy"
							/>
						{/if}
						<div class="min-w-0 flex-1">
							<h3 class="line-clamp-2 text-[15px] font-semibold leading-snug" title={p.document_title ?? tr('Tanpa judul')}>{p.document_title ?? tr('Tanpa judul')}</h3>
							{#if p.order_code}<p class="mt-0.5 text-[11px] text-muted-foreground">{p.order_code}</p>{/if}
						</div>
						{@render rowActions(p)}
					</div>

					<div class="mt-3 border-t pt-3" style="border-color: var(--border);">
						<dl class="grid grid-cols-2 gap-x-3 gap-y-3">
							<div class="min-w-0">
								<dt class="text-[11px] text-muted-foreground">{tr('Mode Integrasi')}</dt>
								<dd class="mt-1">
									<span class="text-[13px] font-medium" style="color: {pubMode(p.media_publication_mode).tone};">{pubMode(p.media_publication_mode).label}</span>
									{#if p.media_integration_type}
										<span class="mt-0.5 block text-[11px] text-muted-foreground">{integrationLabel[p.media_integration_type] ?? p.media_integration_type}</span>
									{/if}
								</dd>
							</div>
							<div class="min-w-0">
								<dt class="text-[11px] text-muted-foreground">{tr('Media')}</dt>
								<dd class="mt-1 flex min-w-0 items-center text-[13px] font-medium">
									{#if effectiveUrl(p)}
										<a
											class="flex min-w-0 items-center gap-1 underline-offset-2 hover:underline"
											href={effectiveUrl(p) ?? undefined}
											target="_blank"
											rel="noreferrer"
											onclick={(e) => e.stopPropagation()}
										>
											<span class="truncate">{mediaDomainLabel(p.media_website_url, p.media_name ?? '')}</span>
											<ExternalLink size={13} class="flex-none" style="color: var(--primary);" />
										</a>
									{:else}
										<span class="truncate">{mediaDomainLabel(p.media_website_url, p.media_name ?? '')}</span>
									{/if}
								</dd>
							</div>
							<div class="min-w-0">
								<!-- Popover absolut di dalam <dd> tertutup oleh dialog detail,
								     jadi di sini cukup nominal bersih + potongan sekilas. -->
								<dt class="text-[11px] text-muted-foreground">{tr('Penghasilan')}</dt>
								<dd class="mt-1 text-[13px] font-semibold tabular-nums">{@render incomeCell(p)}</dd>
							</div>
							<div class="min-w-0">
								<dt class="text-[11px] text-muted-foreground">Status</dt>
								<dd class="mt-1">
									{@render statusBadge(p, true)}
									{@render statusExtras(p)}
								</dd>
							</div>
						</dl>
					</div>
				</div>
			{/each}
		</div>
		<div class="mt-3 flex flex-wrap items-center justify-between gap-2">
			<span class="text-xs text-muted-foreground">
				{tr('Menampilkan')} {items.length} {tr('dari')} {total} {tr('publikasi')}
			</span>
			<div class="flex gap-2">
				<button class="app-btn app-btn-soft" type="button" disabled={pageNo <= 1} onclick={() => { pageNo = Math.max(1, pageNo - 1); }}>{tr('Sebelumnya')}</button>
				<button class="app-btn app-btn-soft" type="button" disabled={pageNo * PAGE_SIZE >= total} onclick={() => { pageNo = pageNo + 1; }}>{tr('Berikutnya')}</button>
			</div>
		</div>
	{/if}
</section>

{#snippet toneBadge(label: string, tone: string)}
	<span class="whitespace-nowrap rounded-full px-2.5 py-1 text-xs font-medium" style="background-color: {TONE[tone]?.bg ?? TONE.gray.bg}; color: {TONE[tone]?.fg ?? TONE.gray.fg};">
		{label}
	</span>
{/snippet}

{#snippet statusBadge(p: any, withDot: boolean)}
	<span class="inline-flex max-md:whitespace-normal items-center gap-1.5 whitespace-nowrap rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--{statusTone[p.status] ?? 'accent'}-soft, var(--accent)); color: {(statusTone[p.status] ?? 'accent') === 'accent' ? 'var(--accent-foreground)' : `var(--${statusTone[p.status]})`};">
		{#if withDot}<span class="h-1.5 w-1.5 flex-none rounded-full" style="background-color: currentColor;"></span>{/if}
		{pendingUrl(p) ? tr('Menunggu URL') : statusLabel[p.status] ?? p.status}
	</span>
{/snippet}

{#snippet statusExtras(p: any)}
	{#if !isManualRow(p) && LIVE.includes(p.status) && p.publish_progress != null}
		<span class="mt-1 block text-[11px] text-muted-foreground">{p.publish_progress}%</span>
	{/if}
	{#if p.rejection_reason}
		<span class="mt-1 block max-w-[220px] text-[11px]" style="color: var(--destructive);">{p.rejection_reason}</span>
	{/if}
	{#if rssBadge(p.rss_import_status)}
		<span class="mt-1 block rounded-full px-2 py-0.5 text-[11px] font-medium" style="border: 1px solid var(--primary); color: var(--primary);">{rssBadge(p.rss_import_status)}</span>
	{/if}
	{#if p.status === 'published' && !p.published_url}
		<span class="mt-1 block text-[11px] font-medium" style="color: var(--warning);">{tr('URL belum terisi')}</span>
	{/if}
{/snippet}

{#snippet rowActions(p: any)}
	<div class="relative inline-flex items-center gap-1">
		{#if p.status === 'waiting_partner'}
			<Tooltip.Provider delayDuration={150}>
				<Tooltip.Root>
					<Tooltip.Trigger>
						{#snippet child({ props })}
							<button
								{...props}
								type="button"
								class="pub-act pub-act-publish"
								aria-label={tr('Setuju & Terbitkan')}
								onclick={(e) => {
									e.stopPropagation();
									closeRowMenu();
									error = '';
									publishConfirm = p;
								}}
							>
								<Send size={16} />
							</button>
						{/snippet}
					</Tooltip.Trigger>
					<Tooltip.Portal>
						<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
							<Tooltip.Arrow class="uiTipArrow" />
							{tr('Setuju & Terbitkan')}
						</Tooltip.Content>
					</Tooltip.Portal>
				</Tooltip.Root>
			</Tooltip.Provider>
		{/if}
		{#if canReject(p)}
			<Tooltip.Provider delayDuration={150}>
				<Tooltip.Root>
					<Tooltip.Trigger>
						{#snippet child({ props })}
							<button
								{...props}
								type="button"
								class="pub-act pub-act-reject"
								aria-label={rejectLabel(p)}
								onclick={(e) => {
									e.stopPropagation();
									closeRowMenu();
									error = '';
									rejectTarget = p;
									rejectReason = '';
								}}
							>
								<X size={16} />
							</button>
						{/snippet}
					</Tooltip.Trigger>
					<Tooltip.Portal>
						<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
							<Tooltip.Arrow class="uiTipArrow" />
							{rejectLabel(p)}
						</Tooltip.Content>
					</Tooltip.Portal>
				</Tooltip.Root>
			</Tooltip.Provider>
		{/if}
		<button
			type="button"
			class="pub-menu-trigger inline-flex items-center justify-center rounded-lg p-1.5 hover:bg-secondary"
			aria-haspopup="menu"
			aria-expanded={rowMenu === p.id}
			aria-label={`${tr('Aksi untuk')} ${p.document_title ?? tr('publikasi')}`}
			onclick={(e) => {
				e.stopPropagation();
				if (rowMenu === p.id) { rowMenu = null; } else { positionRowMenu(e.currentTarget as HTMLElement); rowMenu = p.id; }
			}}
		>
			<MoreVertical size={16} />
		</button>
		{#if rowMenu === p.id}
			<div class="pub-row-menu fixed z-40 w-56 rounded-xl border py-1 shadow-lg" style="border-color: var(--border); background: var(--card, #fff); {menuPositionStyle}" role="menu" aria-label={tr('Aksi publikasi')}>
				<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={(e) => menuAction(e, p, 'detail')}>
					<Eye size={14} /> {tr('Detail')}
				</button>
				{#if p.status === 'published' || p.status === 'waiting_partner' || p.status === 'revision'}
					<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={(e) => menuAction(e, p, 'preview')}>
						<Eye size={14} /> {tr('Pratinjau draf')}
					</button>
				{/if}
				{#if p.status === 'revision'}
					<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={(e) => menuAction(e, p, 'accept-revision')}>
						<Check size={14} /> {tr('Terima Revisi')}
					</button>
				{/if}
				{#if p.status === 'waiting_partner'}
					<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={(e) => menuAction(e, p, 'edit')}>
						<Pencil size={14} /> {tr('Edit draf')}
					</button>
				{/if}
				{#if p.published_url}
					<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={(e) => { e.stopPropagation(); closeRowMenu(); window.open(p.published_url ?? '', '_blank', 'noopener'); }}>
						<ExternalLink size={14} /> {tr('Lihat Langsung')}
					</button>
				{/if}
				{#if p.status === 'partner_approved'}
					<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={(e) => menuAction(e, p, 'url')}>
						<Link2 size={14} /> {tr('Kirim URL')}
					</button>
				{/if}
			</div>
		{/if}
	</div>
{/snippet}

{#if selected}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={closeDetail}>
		<dialog open class="app-card my-8 w-full max-w-[680px] p-5" aria-label={tr('Detail publikasi')} onclick={(e) => e.stopPropagation()}>
			<h2 class="text-lg font-semibold">{tr('Detail publikasi')}</h2>
			<p class="mt-1 text-sm text-muted-foreground">{tr('Informasi lengkap artikel dan riwayat pengiriman ke media Anda.')}</p>

			{#if error}
				<p class="mt-3 text-sm" style="color: var(--destructive);">{error}</p>
			{/if}

			{#if detailLoading}
				<p class="mt-3 text-sm text-muted-foreground">{tr('Memuat detail…')}</p>
			{:else if detail}
				{@const pub = detail}
				{@const vBadge = verificationBadge(detail.verification)}
				{@const sBadge = sentimentBadge(detail.sentiment_analysis)}
				{@const mBadge = messageBadge(detail.message_analysis)}
				{@const liveUrl = effectiveUrl(detail)}
				<div class="mt-3 flex flex-col gap-5">
					<!-- A. Informasi Publikasi -->
					<section>
						<p class="mb-2 text-xs font-bold uppercase tracking-[0.4px] text-muted-foreground">{tr('Informasi Publikasi')}</p>
						<div class="flex items-start justify-between gap-3">
							<div class="min-w-0">
								<p class="text-base font-bold">{detail.document_title ?? tr('Artikel')}</p>
								<p class="text-sm text-muted-foreground">
									{detail.client_name ?? tr('Klien')} · {detail.media_name ?? tr('Media')}
								</p>
							</div>
							{@render toneBadge(detailStatusLabel[detail.status] ?? detail.status, detailStatusTone[detail.status] ?? 'gray')}
						</div>
						<div class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
							<div>
								<p class="text-xs text-muted-foreground">{tr('No. Order')}</p>
								<p class="text-sm font-medium">{detail.order_code ?? '—'}</p>
							</div>
							<div>
								<p class="text-xs text-muted-foreground">{tr('Klien')}</p>
								<p class="text-sm font-medium">{detail.client_name ?? '—'}</p>
							</div>
							<div>
								<p class="text-xs text-muted-foreground">{tr('Media')}</p>
								<p class="text-sm font-medium">{detail.media_name ?? '—'}</p>
							</div>
							<div>
								<p class="text-xs text-muted-foreground">{tr('Dibuat')}</p>
								<p class="text-sm font-medium">{dt(detail.created_at)}</p>
							</div>
							<div>
								<p class="text-xs text-muted-foreground">{tr('Terbit')}</p>
								<p class="text-sm font-medium">{detail.published_at ? dt(detail.published_at) : '—'}</p>
							</div>
							{#if detail.scheduled_at}
								<div>
									<p class="text-xs text-muted-foreground">{tr('Terjadwal')}</p>
									<p class="text-sm font-medium">{dt(detail.scheduled_at)}</p>
								</div>
							{/if}
							{#if detail.media_website_url}
								<div>
									<p class="text-xs text-muted-foreground">{tr('Situs media')}</p>
									<p class="break-all text-sm">{detail.media_website_url}</p>
								</div>
							{/if}
							<div>
								<p class="text-xs text-muted-foreground">{tr('Integrasi')}</p>
								<p class="text-sm font-medium">{detail.media_integration_type ?? '—'}</p>
							</div>
						</div>
					</section>

					<!-- B. Pendapatan mitra dari harga pengajuan. -->
					<section>
						<p class="mb-2 text-xs font-bold uppercase tracking-[0.4px] text-muted-foreground">{tr('Pendapatan')}</p>
						<div class="rounded-[10px] p-3" style="background: var(--success-soft); border: 1px solid color-mix(in srgb, var(--success) 22%, transparent);">
							<div class="flex items-center justify-between gap-3">
								<div>
									<p class="text-sm font-bold">{tr('Pendapatan Anda')}</p>
									<p class="text-xs text-muted-foreground">{tr('Harga pengajuan')} {rp(incomeGross(detail))} · {tr('potongan platform')} {cutPercentLabel(detail) ?? 0}% ({rp(incomeCut(detail))})</p>
								</div>
								<p class="text-xl font-semibold tabular-nums" style="color: var(--success);">{rp(detail.partner_amount)}</p>
							</div>
						</div>
					</section>

					<!-- C. Preview / Content -->
					{#if detail.document}
						<section>
							<p class="mb-2 text-xs font-bold uppercase tracking-[0.4px] text-muted-foreground">{tr('Preview / Content')}</p>
							<p class="text-xs text-muted-foreground">{tr('Slug')}</p>
							<p class="mt-0.5 break-all rounded-md px-2 py-1 font-mono text-xs" style="background: var(--muted);">{detail.document.slug ?? '—'}</p>
							{#if detail.document.meta_description}
								<p class="mt-1 text-sm text-muted-foreground">{detail.document.meta_description}</p>
							{/if}
						</section>
					{/if}

					<!-- D. URL Terbit + verifikasi (hanya setelah artikel terbit). -->
					{#if detail.status === 'published'}
						<section class="rounded-[10px] border p-3" style="border-color: var(--border); background: var(--card);">
							<div class="mb-2 flex flex-wrap items-center justify-between gap-3">
								<p class="text-xs font-bold uppercase tracking-[0.4px] text-muted-foreground">{tr('URL Terbit')}</p>
								{#if vBadge}{@render toneBadge(vBadge.label, vBadge.tone)}{/if}
							</div>
							{#if sBadge || mBadge}
								<div class="mb-2 flex flex-wrap gap-1">
									{#if sBadge}{@render toneBadge(sBadge.label, sBadge.tone)}{/if}
									{#if mBadge}{@render toneBadge(mBadge.label, mBadge.tone)}{/if}
								</div>
							{/if}
							{#if liveUrl}
								<p class="mb-2 text-sm">
									<a href={liveUrl} target="_blank" rel="noreferrer" class="break-all" style="color: var(--primary);">{liveUrl}</a>
								</p>
							{/if}
							{#if detail.verification?.error_message}
								<p class="mb-2 text-xs" style="color: var(--warning);">{verificationHint(detail.verification.error_message)}</p>
							{/if}
							{#if urlSavedNotice}
								<p class="mb-2 text-xs" style="color: var(--success);">{urlSavedNotice}</p>
							{/if}
							<div class="flex items-center gap-2">
								<input
									type="url"
									class="app-input w-full flex-1"
									placeholder={tr('https://situs-mitra.example/artikel/…')}
									readonly={!urlEditMode}
									disabled={!urlEditMode}
									bind:value={urlValue}
								/>
								{#if urlEditMode}
									<button class="app-btn app-btn-primary" disabled={urlUpdating || !urlValue.trim()} onclick={() => void updatePublishedUrl()}>
										{urlUpdating ? tr('Menyimpan…') : tr('Simpan URL')}
									</button>
								{:else}
									<button class="app-btn app-btn-soft" disabled={urlUpdating} title={tr('Perbarui URL & verifikasi ulang')} onclick={() => (urlEditMode = true)}>{tr('Perbarui URL')}</button>
								{/if}
							</div>
							<p class="mt-2 text-xs text-muted-foreground">{tr('URL otomatis terisi dari WordPress. Untuk mengganti, klik Perbarui URL lalu simpan — Tauthub akan memverifikasi ulang otomatis.')}</p>
						</section>
					{/if}

					{#if detail.notes}
						<section>
							<p class="text-xs text-muted-foreground">{tr('Catatan klien')}</p>
							<p class="text-sm">{detail.notes}</p>
						</section>
					{/if}

					{#if detail.rejection_reason}
						<p class="rounded-[10px] px-3 py-2 text-sm" style="background: #fdecee; color: var(--destructive);">{tr('Ditolak:')} {detail.rejection_reason}</p>
					{/if}

					<!-- Revisi dari klien: klien mengubah artikel & memintanya dikirim ulang. -->
					{#if detail.status === 'revision'}
						<section class="rounded-xl p-3" style="background: var(--warning-soft);">
							<p class="text-xs font-bold uppercase tracking-[0.4px]" style="color: var(--warning);">{tr('Permintaan Revisi Klien')}</p>
							{#if (detail.revision_comments ?? []).length === 0}
								<p class="mt-1 text-sm text-muted-foreground">{tr('Klien meminta revisi tanpa catatan tambahan.')}</p>
							{:else}
								{#each detail.revision_comments ?? [] as c (c.id)}
									<div class="mt-2 border-t pt-2 first:mt-1 first:border-0 first:pt-0" style="border-color: var(--border);">
										<p class="text-sm">{c.body}</p>
										<p class="mt-0.5 text-xs text-muted-foreground">{dt(c.created_at)}</p>
										{#if c.attachment_url}
											<a href={c.attachment_url} target="_blank" rel="noreferrer" class="mt-1 inline-flex items-center gap-1 text-xs underline" style="color: var(--primary);">
												<Paperclip size={12} />{c.attachment_name ?? tr('Lampiran')}
											</a>
										{/if}
									</div>
								{/each}
							{/if}
							<p class="mt-2 text-xs text-muted-foreground">
								{tr('Terima untuk mengirim ulang artikel revisi ke media Anda, atau tolak dengan alasan bila masih belum sesuai.')}
								{#if (detail.revision_reject_count ?? 0) > 0}
									{tr('Sudah ditolak')} {detail.revision_reject_count}/{detail.revision_reject_limit ?? 3} {tr('kali.')}
								{/if}
							</p>
						</section>
					{/if}

					<!-- E. Riwayat pengiriman -->
					<section>
						<p class="mb-2 text-xs font-bold uppercase tracking-[0.4px] text-muted-foreground">{tr('Riwayat')}</p>
						{#if (detail.attempts ?? []).length === 0}
							<p class="text-sm text-muted-foreground">{tr('Belum ada percobaan pengiriman tercatat.')}</p>
						{:else}
							{#each detail.attempts as a (a.attempt_number)}
								<div class="flex items-start justify-between gap-3 border-b py-2" style="border-color: var(--border);">
									<div class="min-w-0">
										<p class="text-sm font-medium">#{a.attempt_number} {attemptLabels[a.status] ?? a.status}</p>
										{#if a.error_message}
											<p class="mt-1 text-xs" style="color: var(--destructive);">{a.error_message}</p>
										{/if}
									</div>
									<p class="flex-none text-xs text-muted-foreground">{dt(a.completed_at ?? a.started_at)}</p>
								</div>
							{/each}
						{/if}
					</section>

					{#if detail.status === 'revision'}
						<div class="flex flex-wrap justify-end gap-2">
							{#if (detail.revision_reject_count ?? 0) < (detail.revision_reject_limit ?? 3)}
								<button
									class="app-btn app-btn-soft"
									style="color: var(--destructive);"
									disabled={busy}
									onclick={() => { rejectTarget = pub; rejectReason = ''; }}
								><Trash2 size={15} />{tr('Tolak Revisi')}</button>
							{/if}
							<button class="app-btn app-btn-primary" disabled={busy} onclick={() => void acceptRevision(pub)}><Check size={15} />{tr('Terima Revisi')}</button>
						</div>
					{/if}

					{#if detail.status === 'waiting_partner'}
						<div class="flex flex-wrap justify-end gap-2">
							<button
								class="app-btn app-btn-soft"
								style="color: var(--destructive);"
								disabled={busy}
								onclick={() => { rejectTarget = pub; rejectReason = ''; }}
							><Trash2 size={15} />{tr('Tolak')}</button>
							{#if isManualRow(pub)}
								<button class="app-btn app-btn-primary" disabled={busy} onclick={() => void approve(pub, 'publish')}><Check size={15} />{tr('Setujui')}</button>
							{:else}
								<button class="app-btn app-btn-primary" disabled={busy} onclick={() => void approve(pub, 'publish')}><Check size={15} />{tr('Setujui & Terbitkan')}</button>
							{/if}
						</div>
					{/if}
				</div>
			{/if}
		</dialog>
	</div>
{/if}

{#if showPreview && previewPub}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={() => (showPreview = false)} onkeydown={(e) => { if (e.key === 'Escape') showPreview = false; }}>
		<dialog open class="app-card my-8 flex max-h-[85vh] w-full max-w-[720px] flex-col overflow-hidden p-5" aria-label={tr('Preview Artikel')} onclick={(e) => e.stopPropagation()}>
			<h2 class="text-lg font-semibold">{tr('Preview Artikel')}</h2>
			<p class="mt-1 text-sm text-muted-foreground">
				{draft?.source === 'client_document'
					? tr('Artikel asli klien (dokumen Tauthub) — bukan dari WordPress.')
					: `${tr('Artikel saat ini di WordPress')} (${draft?.wp_status ?? 'draft'}) — ${tr('bukan dokumen asli klien.')}`}
			</p>

			{#if previewLoading}
				<p class="mt-3 text-sm text-muted-foreground">{tr('Memuat preview…')}</p>
			{:else if draft?.error}
				<p class="mt-3 text-sm" style="color: var(--destructive);">{draft.error}</p>
			{:else}
				<div class="mt-3 min-h-0 flex-1 overflow-y-auto rounded-xl border" style="border-color: var(--border); background: var(--card);">
					<div class="border-b p-4" style="border-color: var(--border); background: var(--muted);">
						<p class="text-xl font-bold leading-snug">{draft?.title || tr('(tanpa judul)')}</p>
						{#if draft?.meta_description}
							<p class="mt-2 text-sm text-muted-foreground">{draft.meta_description}</p>
						{/if}
					</div>
					{#if draft?.featured_image_url}
						<div class="border-b" style="border-color: var(--border); background: var(--background);">
							<img
								src={draft.featured_image_url}
								alt={tr('Gambar cover artikel')}
								class="mx-auto block w-full max-w-[600px] object-cover"
								style="max-height: 360px; aspect-ratio: 16 / 9;"
							/>
						</div>
					{/if}
					<div class="p-4">
						<article class="article-html max-w-full break-words text-sm leading-relaxed [&_a]:underline [&_a]:text-[var(--primary)] [&_blockquote]:border-l-2 [&_blockquote]:pl-3 [&_h1]:text-xl [&_h1]:font-semibold [&_h2]:text-lg [&_h2]:font-semibold [&_h3]:font-semibold [&_img]:mx-auto [&_img]:my-3 [&_img]:max-h-[360px] [&_img]:w-auto [&_img]:rounded-lg [&_li]:ml-5 [&_li]:list-disc [&_ol_li]:list-decimal [&_p]:my-2">
							<!-- eslint-disable-next-line svelte/no-at-html-tags -- konten draf milik mitra sendiri dan sudah melalui editor -->
							{@html sanitizeHtml(draft?.content_html ?? draft?.html ?? '')}
						</article>
					</div>
				</div>
			{/if}

			<div class="mt-3 flex justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (showPreview = false)}>{tr('Tutup')}</button>
			</div>
		</dialog>
	</div>
{/if}

<!-- Konfirmasi ganda "Setuju & Terbitkan" — dipicu ikon pesawat di kolom Aksi. -->
{#if publishConfirm}
	<div class="fixed inset-0 z-[70] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={() => (publishConfirm = null)}>
		<dialog open class="app-card w-full max-w-[480px] p-5" aria-label={tr('Setuju & Terbitkan')} onclick={(e) => e.stopPropagation()}>
			<h2 class="text-lg font-semibold">{tr('Setuju & Terbitkan')}</h2>
			<p class="mt-1 text-sm text-muted-foreground">
				{tr('Apakah Anda yakin ingin menyetujui dan menerbitkan')} <strong>{publishConfirm.document_title ?? tr('artikel ini')}</strong>
				{tr('ke')} {publishConfirm.media_name ?? tr('media')}?
				{#if isManualRow(publishConfirm)}
					{tr('Setelah disetujui, Anda diminta menempel URL artikel yang sudah tayang di website Anda.')}
				{:else}
					{tr('Artikel langsung dikirim ke website mitra lewat integrasi.')}
				{/if}
			</p>
			{#if error}<p class="mt-3 text-sm" style="color: var(--destructive);">{error}</p>{/if}
			<div class="mt-4 flex justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" disabled={publishBusy} onclick={() => (publishConfirm = null)}>{tr('Batal')}</button>
				<button class="app-btn app-btn-primary" type="button" disabled={publishBusy} onclick={() => void confirmPublishNow()}>
					{publishBusy ? tr('Menerbitkan…') : tr('Ya, Setuju & Terbitkan')}
				</button>
			</div>
		</dialog>
	</div>
{/if}

<!-- Tolak publikasi — modal terpisah (parity FormDialog "Tolak publikasi"). -->
{#if rejectTarget}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={() => (rejectTarget = null)}>
		<dialog open class="app-card w-full max-w-[480px] p-5" aria-label={tr('Tolak publikasi')} onclick={(e) => e.stopPropagation()}>
			<h2 class="text-lg font-semibold">{rejectTarget.status === 'revision' ? tr('Tolak revisi') : tr('Tolak publikasi')}</h2>
			<p class="mt-1 text-sm text-muted-foreground">
				{#if rejectTarget.status === 'revision'}
					{tr('Revisi untuk')} {rejectTarget.media_name ?? tr('media')} {tr('ditolak dan dikembalikan ke klien untuk diperbaiki. Artikel yang sudah tayang tidak diubah.')}
				{:else}
					{tr('Publikasi ke')} {rejectTarget.media_name ?? tr('media')} {tr('oleh')} {rejectTarget.client_name ?? tr('klien')} {tr('akan ditolak dan dibatalkan.')}
				{/if}
			</p>
			{#if error}<p class="mt-3 text-sm" style="color: var(--destructive);">{error}</p>{/if}
			<label class="mt-4 block">
				<span class="text-xs font-medium text-muted-foreground">{tr('Alasan (min. 5 karakter)')}</span>
				<textarea class="app-input mt-1 w-full" rows="3" required minlength="5" placeholder={tr('Contoh: topik di luar niche media')} bind:value={rejectReason}></textarea>
			</label>
			<div class="mt-4 flex justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (rejectTarget = null)}>{tr('Batal')}</button>
				<button class="app-btn app-btn-primary" type="button" disabled={busy || rejectReason.trim().length < 5} onclick={() => void rejectNow()}>{rejectTarget.status === 'revision' ? tr('Tolak revisi') : tr('Tolak publikasi')}</button>
			</div>
		</dialog>
	</div>
{/if}

<!-- Kirim URL cepat (tanpa buka Detail) — parity dialog "Kirim URL Artikel". -->
{#if quickUrlTarget}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={() => (quickUrlTarget = null)}>
		<dialog open class="app-card w-full max-w-[480px] p-5" aria-label={tr('Kirim URL Artikel')} onclick={(e) => e.stopPropagation()}>
			<h2 class="text-lg font-semibold">{tr('Kirim URL Artikel')}</h2>
			<p class="mt-1 text-sm text-muted-foreground">
				{quickUrlTarget.document_title ?? tr('Artikel')} — {tr('masukkan URL artikel yang sudah terbit di website Anda.')}
			</p>
			{#if quickUrlError}
				<p class="mt-3 rounded-[10px] px-3 py-2 text-sm" style="background: #fdecee; color: var(--destructive);">{quickUrlError}</p>
			{/if}
			<label class="mt-4 block">
				<span class="text-xs font-medium text-muted-foreground">{tr('URL artikel terbit')}</span>
				<input class="app-input mt-1 w-full" type="url" placeholder={tr('https://situs-mitra.example/artikel/…')} bind:value={quickUrlValue} />
			</label>
			<p class="mt-2 text-xs text-muted-foreground">{tr('Tauthub akan memverifikasi URL ini dan menandai artikel sebagai terverifikasi.')}</p>
			<div class="mt-4 flex justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (quickUrlTarget = null)}>{tr('Batal')}</button>
				<button class="app-btn app-btn-primary" type="button" disabled={quickUrlBusy || !quickUrlValue.trim()} onclick={() => void submitQuickUrl()}>
					{quickUrlBusy ? tr('Mengirim…') : tr('Kirim URL')}
				</button>
			</div>
		</dialog>
	</div>
{/if}

<!-- Editor draf — modal terpisah (parity tombol "Edit" app lama). -->
{#if draftPub}
	<div class="fixed inset-0 z-[60] flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={closeDraftEditor}>
		<dialog open class="app-card my-8 flex max-h-[85vh] w-full max-w-[860px] flex-col overflow-hidden p-5" aria-label={tr('Edit draf')} onclick={(e) => e.stopPropagation()}>
			<div class="flex items-start justify-between gap-3">
				<div class="min-w-0">
					<h2 class="text-lg font-semibold">{tr('Isi Artikel')}</h2>
					<p class="mt-1 text-sm text-muted-foreground">{draftPub.document_title ?? tr('Artikel')}</p>
				</div>
				{#if draft && !draft.error}
					<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" disabled={busy} onclick={rewrite}>
						<Sparkles size={13} /> Rewrite AI (Rp 1.000)
					</button>
				{/if}
			</div>

			{#if error}<p class="mt-3 text-sm" style="color: var(--destructive);">{error}</p>{/if}

			{#if draftLoading}
				<p class="mt-3 text-sm text-muted-foreground">{tr('Memuat draf…')}</p>
			{:else if draft?.error}
				<p class="mt-3 text-sm" style="color: var(--destructive);">{draft.error}</p>
			{:else if draft}
				<form class="mt-4 flex min-h-0 flex-1 flex-col gap-4" onsubmit={saveDraft}>
					<div class="min-h-0 flex-1 space-y-4 overflow-y-auto pr-1">
						<div class="grid gap-4 sm:grid-cols-2">
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">{tr('Judul')}</span>
								<input class="app-input mt-1 w-full" maxlength="200" bind:value={draft.title} />
							</label>
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">{tr('Meta Deskripsi')}</span>
								<input class="app-input mt-1 w-full" maxlength="500" bind:value={draft.meta_description} />
							</label>
						</div>

						<div>
							<span class="text-xs font-medium text-muted-foreground">{tr('Gambar Utama')}</span>
							<div class="mt-1 flex flex-wrap items-start gap-3 rounded-xl border p-3" style="border-color: var(--border);">
								{#if draft.featured_image_url}
									<img
										src={draft.featured_image_url}
										alt={tr('Gambar cover artikel')}
										class="w-40 shrink-0 rounded-lg object-cover"
										style="aspect-ratio: 16 / 9;"
									/>
								{:else}
									<div class="flex w-40 shrink-0 items-center justify-center rounded-lg border border-dashed text-xs text-muted-foreground" style="aspect-ratio: 16 / 9; border-color: var(--border);">
										{tr('Belum ada gambar')}
									</div>
								{/if}
								<div class="flex min-w-0 flex-1 flex-col gap-2">
									<input class="app-input w-full" placeholder="/uploads/…" bind:value={draft.featured_image_url} />
									<div class="flex flex-wrap gap-2">
										<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={coverBusy} onclick={() => coverInput?.click()}>
											{#if coverBusy}<LoaderCircle size={13} class="animate-spin" />{:else}<Paperclip size={13} />{/if}
											{tr('Ganti gambar')}
										</button>
										{#if draft.featured_image_url}
											<a class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" href={draft.featured_image_url} target="_blank" rel="noopener">{tr('Buka di tab baru')}</a>
											<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={() => { draft.featured_image_url = ''; void measureDraftImages(); }}>{tr('Hapus gambar')}</button>
										{/if}
									</div>
									<p class="text-xs text-muted-foreground">{tr('Gambar disimpan di Tauthub; saat artikel terbit ke media, tautannya dikirim lengkap dengan domain.')}</p>
								</div>
							</div>
							<input bind:this={coverInput} type="file" accept="image/*" class="hidden" onchange={pickCover} />
							{#if coverError}<p class="mt-1 text-xs" style="color: var(--destructive);">{coverError}</p>{/if}
						</div>

						<div>
							<span class="text-xs font-medium text-muted-foreground">{tr('Isi Artikel')}</span>
							<div class="mt-1">
								<RichTextEditor
									bind:value={draft.content_html}
									ariaLabel={tr('Isi artikel')}
									minHeight={320}
									uploadImage={uploadDraftImage}
									disabled={busy}
								/>
							</div>
						</div>
					</div>

					<div class="flex flex-wrap items-center justify-between gap-2 border-t pt-3" style="border-color: var(--border);">
						<p class="text-xs text-muted-foreground">
							{draft.source === 'wp_draft' ? tr('Sumber: draf WordPress') : tr('Sumber: dokumen klien')}
							{#if draft.edited_at} · {tr('diedit')} {dt(draft.edited_at)}{/if}
							{#if draftImgCount > 0}
								<!-- Total UKURAN gambar artikel yang diminta pemilik produk; kalau
								     ada gambar luar yang tak terukur, tampil perkiraan jumlah saja. -->
								· {#if !draftImgUnknown && draftImgBytes != null}{fmtBytesID(draftImgBytes)}{:else}±{draftImgCount} {tr('gambar')}{/if}
							{/if}
						</p>
						<div class="flex gap-2">
							<button class="app-btn app-btn-soft" type="button" onclick={closeDraftEditor}>{tr('Tutup')}</button>
							<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{tr('Simpan Draf')}</button>
						</div>
				</div>
			</form>
		{/if}
	</dialog>
	</div>
{/if}

<style>
	/* Aksi baris yang berdiri sendiri di kolom Aksi: lencana tonal, bukan ikon
		tertelanjang. Warnanya membawa makna (terbitkan = aksen utama, tolak =
		destruktif); animasinya hanya bereaksi saat disorot — satu gerakan singkat
		tanpa apa pun yang berdenyut sendiri. */
	.pub-act {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 30px;
		height: 30px;
		border-radius: 9px;
		transition: background-color 0.14s ease, transform 0.14s ease, box-shadow 0.14s ease;
	}
	.pub-act-publish {
		background: color-mix(in srgb, var(--primary) 12%, transparent);
		color: var(--primary);
	}
	.pub-act-publish:hover {
		background: color-mix(in srgb, var(--primary) 20%, transparent);
		transform: translateY(-1px) scale(1.05);
		box-shadow: 0 4px 10px color-mix(in srgb, var(--primary) 24%, transparent);
	}
	.pub-act-reject {
		background: color-mix(in srgb, var(--destructive) 10%, transparent);
		color: var(--destructive);
	}
	.pub-act-reject:hover {
		background: color-mix(in srgb, var(--destructive) 18%, transparent);
		transform: translateY(-1px) scale(1.05);
		box-shadow: 0 4px 10px color-mix(in srgb, var(--destructive) 22%, transparent);
	}
	.pub-act:active {
		transform: translateY(0) scale(0.96);
		box-shadow: none;
	}
	@media (prefers-reduced-motion: reduce) {
		.pub-act,
		.pub-act:hover,
		.pub-act:active {
			transition: none;
			transform: none;
		}
	}

	/* Menu aksi baris dipakai bersama tabel (desktop) dan kartu (mobile). Di
		layar sentuh (<768px) pemicu, tombol aksi, dan item menunya minimal 44px;
		desktop tidak berubah. */
	@media (max-width: 767px) {
		.pub-menu-trigger,
		.pub-act {
			min-width: 44px;
			min-height: 44px;
		}
		.pub-row-menu button[role='menuitem'] {
			min-height: 44px;
		}
	}
</style>
