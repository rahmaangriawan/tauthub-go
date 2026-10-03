<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Publikasi mitra — port section "Publikasi": daftar permintaan artikel per
	// media, filter status, pratinjau/edit draf, setujui, tolak, kirim URL,
	// dan rewrite AI (berbayar dari saldo mitra).
	import { onMount } from 'svelte';
	import { Check, Clock, Eye, ExternalLink, Link2, MoreVertical, Paperclip, Pencil, Search, Sparkles, Trash2, XCircle } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import { sanitizeHtml } from '$lib/sanitize-html';

	type Pub = {
		id: string; order_code: string | null; status: string; client_name: string | null;
		document_title: string | null; document_id: string; media_name: string | null;
		media_category: string | null; media_integration_type: string | null;
		media_publication_mode: string | null; client_price: number; partner_amount: number;
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
	let quickUrlTarget = $state<Pub | null>(null);
	let quickUrlValue = $state('');
	let quickUrlBusy = $state(false);
	let quickUrlError = $state('');
	let draftPub = $state<Pub | null>(null);
	let draft = $state<any>(null);
	let draftLoading = $state(false);

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
		draft = await loadDraft(p);
		draftLoading = false;
	}

	function closeDraftEditor() {
		draftPub = null;
		draft = null;
	}

	/**
	 * Setujui artikel. `publishStatus` wajib eksplisit (parity app lama):
	 * "publish" = setujui & terbitkan, "draft" = setujui sebagai draf.
	 * Tanpa ini server memakai default mode media, sehingga mitra tidak bisa
	 * memilih draf saat mode medianya "publish".
	 */
	async function approve(pub: Pub, publishStatus: 'draft' | 'publish') {
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/publications/${pub.id}/approve`, {
				method: 'POST',
				body: JSON.stringify({ publish_status: publishStatus })
			});
			if (!res.ok) {
				error = await readApiError(res, "Permintaan gagal.");
				return;
			}
			notice = publishStatus === 'draft' ? tr('Artikel disetujui sebagai draf.') : tr('Artikel disetujui dan diterbitkan.');
			if (detail && detail.id === pub.id) await openDetail(pub);
			await load();
			// Mode manual + "Setujui & Terbitkan": langsung sodorkan input URL
			// artikel yang mitra terbitkan sendiri — satu alur, parity app lama.
			if (publishStatus === 'publish' && isManualRow(pub)) openQuickUrl(pub);
		} finally {
			busy = false;
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

	// ── Parity kolom tabel app lama ──────────────────────────────────────
	const pubMode = (m: string | null) => (m ?? 'draft') === 'publish'
		? { label: tr('Mode Integrasi'), tone: 'var(--success)' }
		: { label: tr('Mode Manual'), tone: 'var(--warning)' };
	const mediaHost = (u: string | null) => (u ?? '').replace(/^https?:\/\//, '').replace(/\/+$/, '');
	const integrationLabel: Record<string, string> = $derived({
		wordpress: 'WordPress', blogger: 'Blogger', custom_web: 'Custom Web', tulisio_api: 'Tulisio API', manual: tr('Manual')
	});
	const isManualRow = (p: Pub) => (p.media_publication_mode ?? 'draft') !== 'publish';
	const pendingUrl = (p: Pub) => p.status === 'partner_approved' && isManualRow(p) && !p.published_url;
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
</script>

<PageTitle title={tr("Publikasi")} />

<svelte:window onclick={closeRowMenu} onkeydown={(e) => { if (e.key === 'Escape') closeRowMenu(); }} />

<section>
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">{tr("Publikasi")}</h1>
		<p class="mt-1 text-sm text-muted-foreground">{total} {tr("permintaan artikel pada media Anda.")}</p>
	</header>

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	<form class="app-card mt-4 flex flex-wrap items-center gap-2 p-3" onsubmit={(e) => e.preventDefault()}>
		<div class="flex flex-wrap items-center gap-1" role="tablist" aria-label={tr("Filter status publikasi")}>
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
		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="w-[38%] px-4 py-2.5 font-medium">{tr('Artikel')}</th>
						<th class="px-4 py-2.5 font-medium">{tr("Mode Publikasi")}</th>
						<th class="px-4 py-2.5 font-medium">{tr("Media")}</th>
						<th class="px-4 py-2.5 text-right font-medium">{tr("Penghasilan")}</th>
						<th class="px-4 py-2.5 font-medium">Status</th>
						<th class="px-4 py-2.5 text-right font-medium">{tr('Aksi')}</th>
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
								<p>{p.media_name ?? '—'}</p>
								{#if p.media_website_url}
									<a class="text-xs" style="color: var(--primary);" href={p.media_website_url} target="_blank" rel="noreferrer" onclick={(e) => e.stopPropagation()}>
										{mediaHost(p.media_website_url)}
									</a>
								{/if}
								{#if p.media_integration_type}
									<p class="text-[11px] text-muted-foreground">{integrationLabel[p.media_integration_type] ?? p.media_integration_type}</p>
								{/if}
							</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{rp(p.partner_amount)}</td>
							<td class="px-4 py-2.5">
								<span class="whitespace-nowrap rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--{statusTone[p.status] ?? 'accent'}-soft, var(--accent)); color: var(--{statusTone[p.status] ?? 'accent'});">
									{pendingUrl(p) ? tr('Menunggu URL') : statusLabel[p.status] ?? p.status}
								</span>
								{#if !isManualRow(p) && LIVE.includes(p.status) && p.publish_progress != null}
									<span class="mt-1 block text-[11px] text-muted-foreground">{p.publish_progress}%</span>
								{/if}
								{#if p.rejection_reason}
									<span class="mt-1 block max-w-[220px] text-[11px]" style="color: var(--destructive);">{p.rejection_reason}</span>
								{/if}
								{#if rssBadge(p.rss_import_status)}
									<span class="mt-1 block rounded-full px-2 py-0.5 text-[11px] font-medium" style="border: 1px solid var(--primary); color: var(--primary);">{rssBadge(p.rss_import_status)}</span>
								{/if}
								{#if p.status === 'published'}
									{#if p.published_url}
										<span class="mt-1 block text-[11px] font-medium" style="color: var(--success);">URL ✓</span>
									{:else}
										<span class="mt-1 block text-[11px] font-medium" style="color: var(--warning);">{tr('URL belum terisi')}</span>
									{/if}
								{/if}
							</td>
							<td class="px-4 py-2.5 text-right">
								<div class="relative inline-block">
									<button
										type="button"
										class="rounded-lg p-1.5 hover:bg-secondary"
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
										<div class="fixed z-40 w-56 rounded-xl border py-1 shadow-lg" style="border-color: var(--border); background: var(--card, #fff); {menuPositionStyle}" role="menu" aria-label={tr('Aksi publikasi')}>
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
												{#if (p.revision_reject_count ?? 0) < (p.revision_reject_limit ?? 3)}
													<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" style="color: var(--destructive);" onclick={(e) => menuAction(e, p, 'reject')}>
														<XCircle size={14} /> {tr('Tolak Revisi')}
													</button>
												{/if}
											{/if}
											{#if p.status === 'waiting_partner'}
												<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={(e) => menuAction(e, p, 'edit')}>
													<Pencil size={14} /> {tr('Edit draf')}
												</button>
											{/if}
											{#if p.published_url}
												<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={(e) => { e.stopPropagation(); closeRowMenu(); window.open(p.published_url ?? '', '_blank', 'noopener'); }}>
													<ExternalLink size={14} /> {tr('Lihat di WordPress')}
												</button>
											{/if}
											{#if p.status === 'partner_approved'}
												<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={(e) => menuAction(e, p, 'url')}>
													<Link2 size={14} /> {tr('Kirim URL')}
												</button>
											{/if}
											{#if p.status === 'waiting_partner'}
												<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={(e) => menuAction(e, p, 'publish')}>
													<Check size={14} /> {tr('Setujui & Terbitkan')}
												</button>
												<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={(e) => menuAction(e, p, 'draft')}>
													<Check size={14} /> {tr('Setujui sebagai Draf')}
												</button>
												<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" style="color: var(--destructive);" onclick={(e) => menuAction(e, p, 'reject')}>
													<XCircle size={14} /> {tr('Tolak')}
												</button>
											{/if}
										</div>
									{/if}
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
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
									<p class="text-xs text-muted-foreground">{tr('75% × Harga Pengajuan')}</p>
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
								<button class="app-btn app-btn-soft" style="color: var(--warning);" disabled={busy} onclick={() => void approve(pub, 'draft')}><Clock size={15} />{tr('Setujui sebagai Draf')}</button>
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
		<dialog open class="app-card my-8 w-full max-w-[720px] p-5" aria-label={tr('Edit draf')} onclick={(e) => e.stopPropagation()}>
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
				<form class="mt-3 space-y-2" onsubmit={saveDraft}>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">{tr('Judul')}</span>
						<input class="app-input mt-1 w-full" maxlength="200" bind:value={draft.title} />
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">{tr('Meta Deskripsi')}</span>
						<input class="app-input mt-1 w-full" maxlength="500" bind:value={draft.meta_description} />
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">{tr('Gambar Utama (URL)')}</span>
						<input class="app-input mt-1 w-full" bind:value={draft.featured_image_url} />
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">{tr('Isi Artikel (HTML)')}</span>
						<textarea class="app-input mt-1 w-full font-mono text-xs" rows="12" bind:value={draft.content_html}></textarea>
					</label>
					<div class="flex flex-wrap items-center justify-between gap-2">
						<p class="text-xs text-muted-foreground">
							{draft.source === 'wp_draft' ? tr('Sumber: draf WordPress') : tr('Sumber: dokumen klien')}
							{#if draft.edited_at} · {tr('diedit')} {dt(draft.edited_at)}{/if}
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
