<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Publikasi mitra — port section "Publikasi": daftar permintaan artikel per
	// media, filter status, pratinjau/edit draf, setujui, tolak, kirim URL,
	// dan rewrite AI (berbayar dari saldo mitra).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Building2, Calendar, CalendarClock, Check, CheckCircle2, CircleAlert, Clock, Copy, ExternalLink, Eye, FileText, Globe, Image as ImageIcon, Link2, LoaderCircle, MoreVertical, Paperclip, Pencil, RefreshCw, Search, Send, SlidersHorizontal, Smile, Sparkles, Trash2, User, Wallet, X } from '@lucide/svelte';
	import { Tooltip } from 'bits-ui';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import RichTextEditor from '$lib/components/rich-text-editor.svelte';
	import { mediaDomainLabel } from '$lib/media/domain-label';
	import { sanitizeHtml } from '$lib/sanitize-html';
	import { markPubSeen, refreshPubsUnreadNow } from '$lib/partner/pubs-unread.svelte';

	type Pub = {
		id: string; order_code: string | null; status: string; client_name: string | null;
		document_title: string | null; document_id: string; media_id: string; media_name: string | null;
		media_category: string | null; media_integration_type: string | null;
		media_publication_mode: string | null; client_price: number; partner_amount: number;
		partner_amount_gross?: number; partner_cut_percent?: number;
		rewrite_ai_count?: number; rewrite_remain?: number;
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
	// Nilai awal dari URL (?tab= & ?media=) — deep-link dari lencana "N menunggu"
	// di halaman Daftar Media. Tab tak dikenal jatuh ke "Semua" (find() kosong).
	let activeTab = $state(page.url.searchParams.get('tab') ?? '');
	let mediaFilter = $state(page.url.searchParams.get('media') ?? '');
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
		let copiedKey = $state<string | null>(null);

		function copyText(val: string | null | undefined, key: string) {
			if (!val) return;
			navigator.clipboard.writeText(val);
			copiedKey = key;
			setTimeout(() => {
				if (copiedKey === key) copiedKey = null;
			}, 2000);
		}

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
	// Statistik isi editor (kata/tautan/gambar) dihitung komponen editor;
	// footer kolom kiri hanya menampilkan jumlah kata aktual.
	let editorStats = $state({ words: 0, links: 0, images: 0 });
	// Umpan balik penyimpanan draf DI DALAM modal (pusat notifikasi halaman ada
	// di balik overlay modal, jadi tidak terlihat saat mengetik).
	let draftSavedNotice = $state('');

	// Sesi harus divalidasi lebih dulu (refresh token), baru daftar dimuat.
	let ready = $state(false);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			ready = true;
			// Publikasi baru menyalakan badge merah di menu; membuka halaman ini
			// dihitung sudah dibaca (angka hilang), publikasi lain menyalakannya lagi.
			void markPubSeen();
			void refreshPubsUnreadNow();
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
		if (mediaFilter) params.set('media', mediaFilter);
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
	// Label media untuk chip filter ?media= (server sudah memfilter; semua baris
	// berasal dari media yang sama). Kosong bila hasilnya 0 — chip tetap tampil.
	const filteredMediaName = $derived(items[0] ? mediaDomainLabel(items[0].media_website_url, items[0].media_name ?? '') : '');

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

	// Jatah Rewrite AI per artikel (gratis, maksimal 2 kali — aturan pemilik produk).
	let rewriteLeft = $state(2);
	// Opsi Kategori: dari kategori milik mitra + kategori yang dipilih klien.
	let catOptions = $state<string[]>([]);
	let catOptionsLoading = $state(false);
	let catOptionsLoaded = $state(false);

	async function loadCategoryOptions(current: string) {
		if (catOptionsLoaded && current) {
			ensureCatOption(current);
			return;
		}
		catOptionsLoading = true;
		try {
			const res = await apiRequest('/v1/partner/media/categories');
			if (res.ok) {
				const data = await res.json();
				catOptions = (data.categories ?? []).filter((c: unknown) => typeof c === 'string' && c.trim() !== '');
				catOptionsLoaded = true;
			}
		} finally {
			catOptionsLoading = false;
		}
		ensureCatOption(current);
	}

	function ensureCatOption(current: string) {
		const v = (current ?? '').trim();
		if (v && !catOptions.includes(v)) catOptions = [v, ...catOptions];
	}

	// Tambah kategori baru dari dalam dropdown Kategori: dieksekusi ke katalog
	// kategori media mitra (PUT media.categories) supaya dapat dipakai ulang di
	// semua halaman dan ikut sinkron ke sistem integrasi saat artikel terbit.
	let catNewInput = $state('');
	let catNewBusy = $state(false);

	async function addCategory() {
		const v = catNewInput.trim();
		if (!v || catNewBusy || !draftPub?.media_id) return;
		catNewBusy = true;
		try {
			// Kategori milik media dulu (list lengkap — PUT bersifat replace).
			const cur = await apiRequest(`/v1/partner/media/${draftPub.media_id}`);
			const existing: string[] = cur.ok ? ((await cur.json())?.categories ?? []) : [];
			const target = existing.some((c) => c.trim().toLowerCase() === v.toLowerCase())
				? existing
				: [...existing, v];
			const res = await apiRequest(`/v1/partner/media/${draftPub.media_id}`, {
				method: 'PUT',
				body: JSON.stringify({ categories: target })
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menambah kategori.'));
				return;
			}
			ensureCatOption(v);
			draft.category = v;
			catNewInput = '';
			notice = tr('Kategori baru tersimpan dan siap dipakai.');
			await load();
		} finally {
			catNewBusy = false;
		}
	}

	/** Editor draf — modal terpisah (parity tombol "Edit" app lama). */
	async function openDraftEditor(p: Pub) {
		rewriteLeft = Math.max(0, p.rewrite_remain ?? 2 - (p.rewrite_ai_count ?? 0));
		draftPub = p;
		draft = null;
		draftLoading = true;
		error = '';
		coverError = '';
		coverBusy = false;
		editorStats = { words: 0, links: 0, images: 0 };
		draftSavedNotice = '';
		draft = await loadDraft(p);
		draftLoading = false;
		if (draft) {
			if (!draft.category) draft.category = (p.media_category ?? '').trim();
			void loadCategoryOptions((draft.category ?? '').trim());
			void measureDraftImages();
		}
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
		editorStats = { words: 0, links: 0, images: 0 };
		draftSavedNotice = '';
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

		/** Verifikasi ulang URL yang sudah terbit */
		async function reverifyUrl() {
			if (!detail) return;
			const live = effectiveUrl(detail);
			if (live) urlValue = live;
			await updatePublishedUrl();
		}

		async function saveDraft(e: SubmitEvent) {
		e.preventDefault();
		if (!draftPub || !draft) return;
		if (busy) return; // cegah kirim ganda saat request masih diproses
		busy = true;
		draftSavedNotice = '';
		error = '';
		try {
			const payload: Record<string, unknown> = {
				title: draft.title,
				content_html: draft.content_html,
				meta_description: draft.meta_description ?? '',
				featured_image_url: draft.featured_image_url || null,
				category: (draft.category ?? '').trim()
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
			draftSavedNotice = tr('Perubahan draf tersimpan.');
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
				error = await readApiError(res, tr('Rewrite gagal. Coba lagi nanti.'));
				return;
			}
			draft.title = data.title;
			draft.content_html = data.content_html;
			rewriteLeft = Math.max(0, Number(data.rewrite_remain ?? rewriteLeft - 1) || 0);
			notice = tr('Rewrite selesai (gratis). AI menulis ulang dengan sudut pandang angle yang dipilih klien — tinjau lalu simpan draf.');
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

	/**
	 * Kompresi gambar cover: JPG/PNG dijadikan WebP dengan target ukuran
	 * maksimum COVER_MAX_BYTES. Turunkan kualitas lebih dulu, baru kecilkan
	 * dimensi (tumbuhkan * bertahap) bila masih melebihi target.
	 */
	const COVER_MAX_BYTES = 150 * 1024;
	// Format yang diizinkan pemilik produk: hanya JPG, PNG, WebP — GIF & BMP
	// dilarang (callback dipakai untuk pesan).
	const COVER_TYPES_OK = ['image/png', 'image/jpeg', 'image/webp'];

	async function kompresCoverWebp(file: File): Promise<File> {
		if (file.type === 'image/webp') return file;
		try {
			const bitmap = await createImageBitmap(file);
			const maxSide = 1600;
			const skala = Math.min(1, maxSide / Math.max(bitmap.width, bitmap.height));
			const baseW = Math.round(bitmap.width * skala);
			const baseH = Math.round(bitmap.height * skala);
			const nama = file.name.replace(/\.[^.]+$/, '') + '.webp';
			const render = async (w: number, h: number, quality: number): Promise<Blob | null> => {
				const canvas = document.createElement('canvas');
				canvas.width = w;
				canvas.height = h;
				const ctx = canvas.getContext('2d');
				if (!ctx) return null;
				ctx.drawImage(bitmap, 0, 0, w, h);
				return await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/webp', quality));
			};
			let last: Blob | null = null;
			for (let tile = 1; tile >= 0.4; tile -= 0.2) {
				const tileW = Math.max(1, Math.round(baseW * tile));
				const tileH = Math.max(1, Math.round(baseH * tile));
				for (let quality = 0.85; quality >= 0.25; quality -= 0.15) {
					const blob = await render(tileW, tileH, quality);
					if (!blob) continue;
					last = blob;
					if (blob.size <= COVER_MAX_BYTES) {
						bitmap.close?.();
						return new File([blob], nama, { type: 'image/webp' });
					}
				}
			}
			// Kualitas terendah paling baik jarang besar: pakai hasil percobaan
			// terakhir — tetap WebP dan jauh di bawah batas 10 MB server.
			bitmap.close?.();
			return last ? new File([last], nama, { type: 'image/webp' }) : file;
		} catch {
			return file; // gagal baca bitmap → file asli, server tetap memvalidasi
		}
	}

	async function pickCover(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file || !draft) return;
		coverBusy = true;
		coverError = '';
		try {
			if (!COVER_TYPES_OK.includes(file.type)) {
				coverError = tr('Format gambar harus JPG, PNG, atau WebP.');
				return;
			}
			const readyFile = await kompresCoverWebp(file);
			draft.featured_image_url = await uploadDraftImage(readyFile);
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
	 * Preview DARI DALAM editor: isi modanya ya isi yang sedang diketik (belum
	 * tentu tersimpan), jadi draf TIDAK dimuat ulang dari server — memuat ulang
	 * akan menghapus hasil suntingan. Modal preview tampil DI ATAS modal editor
	 * (z-index lebih tinggi bila draftPub masih terbuka).
	 */
	function previewCurrentDraft() {
		if (!draftPub) return;
		previewPub = draftPub;
		previewLoading = false;
		showPreview = true;
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
		{#if mediaFilter}
			<span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs" style="border: 1px solid var(--border); background: var(--secondary);">
				<FileText size={12} class="text-muted-foreground" />
				<span>{tr('Media')}{#if filteredMediaName}: <strong class="font-semibold">{filteredMediaName}</strong>{/if}</span>
				<button
					type="button"
					class="rounded-full p-0.5 hover:bg-background"
					aria-label={tr('Hapus filter media')}
					title={tr('Hapus filter media')}
					onclick={() => { mediaFilter = ''; pageNo = 1; }}
				>
					<X size={12} />
				</button>
			</span>
		{/if}
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
												class="h-10 w-10 flex-none rounded-[5px] border object-cover"
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
								class="h-11 w-11 flex-none rounded-[5px] border object-cover"
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
		<div
			class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-5"
			style="background-color: rgba(15, 23, 42, 0.45); backdrop-filter: blur(4px);"
			role="presentation"
			onclick={closeDetail}
			onkeydown={(e) => { if (e.key === 'Escape') closeDetail(); }}
		>
			<dialog
				open
				class="app-card my-auto flex max-h-[90vh] w-full max-w-[960px] flex-col overflow-hidden rounded-2xl border border-slate-200/80 p-0 bg-white shadow-2xl"
				aria-label={tr('Detail publikasi')}
				onclick={(e) => e.stopPropagation()}
			>
				<!-- ── Header tetap ── -->
				<header class="flex flex-none items-center justify-between border-b border-slate-100 bg-white px-5 py-3.5 sm:px-6 sm:py-4">
					<div class="flex items-center gap-3 min-w-0">
						<div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
							<FileText size={20} />
						</div>
						<div class="min-w-0">
							<h2 class="text-base sm:text-lg font-bold text-slate-900 truncate tracking-tight">{tr('Detail publikasi')}</h2>
							<p class="text-xs text-muted-foreground truncate">{tr('Informasi lengkap artikel dan riwayat pengiriman ke media Anda.')}</p>
						</div>
					</div>
					<button
						type="button"
						class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl text-slate-400 hover:bg-slate-100 hover:text-slate-700 transition-colors"
						onclick={closeDetail}
						aria-label={tr('Tutup')}
					>
						<X size={18} />
					</button>
				</header>

				<!-- ── Area Konten Scrollable ── -->
				<div class="min-h-0 flex-1 overflow-y-auto p-4 sm:p-6 space-y-4 sm:space-y-5 bg-slate-50/40">
					{#if error}
						<div class="rounded-xl border border-rose-200 bg-rose-50 p-3.5 text-xs text-rose-700">
							{error}
						</div>
					{/if}

					{#if detailLoading}
						<div class="flex flex-col items-center justify-center py-12 text-slate-400">
							<LoaderCircle size={28} class="animate-spin text-primary" />
							<p class="mt-2 text-xs font-medium text-muted-foreground">{tr('Memuat detail…')}</p>
						</div>
					{:else if detail}
						{@const pub = detail}
						{@const vBadge = verificationBadge(detail.verification)}
						{@const sBadge = sentimentBadge(detail.sentiment_analysis)}
						{@const mBadge = messageBadge(detail.message_analysis)}
						{@const liveUrl = effectiveUrl(detail)}

						<!-- 1. Section Informasi Publikasi (Top banner card) -->
						<section class="rounded-2xl border border-slate-200/80 bg-white p-4 sm:p-5 shadow-sm">
							<div class="flex flex-col sm:flex-row items-start gap-4 sm:gap-5">
								{#if detail.thumbnail_url ?? detail.featured_image_url}
									<img
										src={detail.thumbnail_url ?? detail.featured_image_url}
										alt=""
										class="h-24 w-36 sm:h-28 sm:w-44 shrink-0 rounded-xl object-cover border border-slate-200/60 shadow-sm"
										loading="lazy"
									/>
								{/if}
								<div class="min-w-0 flex-1">
									<p class="text-[11px] font-bold uppercase tracking-wider text-muted-foreground mb-1">
										{tr('Informasi Publikasi')}
									</p>
									<h3 class="text-base sm:text-lg font-bold text-slate-900 leading-snug break-words">
										{detail.document_title ?? tr('Artikel')}
									</h3>
									<p class="text-xs sm:text-sm text-muted-foreground mt-1.5 flex items-center gap-1.5 flex-wrap">
										<span class="font-medium text-slate-700">{detail.client_name ?? tr('Klien')}</span>
										<span class="text-slate-300">•</span>
										<span class="font-medium text-slate-700">{detail.media_name ?? tr('Media')}</span>
									</p>
								</div>

								<div class="flex sm:flex-col items-end justify-between sm:justify-start gap-2 shrink-0 w-full sm:w-auto">
									{#if detail.status === 'published'}
										<span class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200/70">
											<span class="h-2 w-2 rounded-full bg-emerald-500"></span>
											{detailStatusLabel[detail.status] ?? detail.status}
										</span>
										<div class="rounded-lg bg-emerald-50/70 border border-emerald-100/80 px-2.5 py-1.5 text-center text-[11px] font-medium text-emerald-800 leading-tight">
											{tr('Artikel telah dipublikasikan di media Anda.')}
										</div>
									{:else if detail.status === 'waiting_partner'}
										<span class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-semibold bg-amber-50 text-amber-700 border border-amber-200/70">
											<span class="h-2 w-2 rounded-full bg-amber-500"></span>
											{detailStatusLabel[detail.status] ?? detail.status}
										</span>
										<div class="rounded-lg bg-amber-50/70 border border-amber-100/80 px-2.5 py-1.5 text-center text-[11px] font-medium text-amber-800 leading-tight">
											{tr('Menunggu konfirmasi persetujuan dari Anda.')}
										</div>
									{:else if detail.status === 'revision'}
										<span class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-semibold bg-amber-50 text-amber-700 border border-amber-200/70">
											<span class="h-2 w-2 rounded-full bg-amber-500"></span>
											{detailStatusLabel[detail.status] ?? detail.status}
										</span>
										<div class="rounded-lg bg-amber-50/70 border border-amber-100/80 px-2.5 py-1.5 text-center text-[11px] font-medium text-amber-800 leading-tight">
											{tr('Klien mengirimkan revisi untuk artikel ini.')}
										</div>
									{:else if detail.status === 'partner_rejected' || detail.status === 'failed'}
										<span class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-semibold bg-rose-50 text-rose-700 border border-rose-200/70">
											<span class="h-2 w-2 rounded-full bg-rose-500"></span>
											{detailStatusLabel[detail.status] ?? detail.status}
										</span>
									{:else}
										<span class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-semibold bg-slate-100 text-slate-700 border border-slate-200">
											<span class="h-2 w-2 rounded-full bg-slate-400"></span>
											{detailStatusLabel[detail.status] ?? detail.status}
										</span>
									{/if}
								</div>
							</div>
						</section>

						<!-- 2. Section Informasi Transaksi dan Media (2 Kolom dengan Ikon Bulat) -->
						<section class="rounded-2xl border border-slate-200/80 bg-white p-5 shadow-sm">
							<div class="grid grid-cols-1 md:grid-cols-2 gap-y-4 gap-x-8">
								<!-- Kolom Kiri -->
								<div class="space-y-4">
									<!-- 1. No. Order -->
									<div class="flex items-start gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
											<FileText size={17} />
										</div>
										<div class="min-w-0 flex-1">
											<p class="text-xs text-muted-foreground">{tr('No. Order')}</p>
											<div class="flex items-center gap-1.5 mt-0.5">
												<p class="text-sm font-semibold text-slate-800 font-mono">{detail.order_code ?? '—'}</p>
													{#if detail.order_code}
														<button
															type="button"
															class="text-slate-400 hover:text-slate-700 transition-colors p-0.5 rounded"
															title={copiedKey === 'order' ? tr('Tersalin!') : tr('Salin No. Order')}
															onclick={() => copyText(pub.order_code, 'order')}
														>
															{#if copiedKey === 'order'}
																<Check size={13} class="text-emerald-600" />
															{:else}
																<Copy size={13} />
															{/if}
														</button>
													{/if}
											</div>
										</div>
									</div>

									<!-- 2. Media -->
									<div class="flex items-start gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
											<Building2 size={17} />
										</div>
										<div class="min-w-0 flex-1">
											<p class="text-xs text-muted-foreground">{tr('Media')}</p>
											<p class="text-sm font-semibold text-slate-800 mt-0.5 truncate">{detail.media_name ?? '—'}</p>
										</div>
									</div>

									<!-- 3. Situs media -->
									<div class="flex items-start gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
											<Globe size={17} />
										</div>
										<div class="min-w-0 flex-1">
											<p class="text-xs text-muted-foreground">{tr('Situs media')}</p>
											{#if detail.media_website_url}
												<a
													href={detail.media_website_url}
													target="_blank"
													rel="noreferrer"
													class="text-sm font-semibold text-blue-600 hover:underline mt-0.5 inline-flex items-center gap-1 truncate max-w-full"
												>
													<span class="truncate">{detail.media_website_url}</span>
													<ExternalLink size={12} class="shrink-0" />
												</a>
											{:else}
												<p class="text-sm font-semibold text-slate-800 mt-0.5">—</p>
											{/if}
										</div>
									</div>

									<!-- 4. Status -->
									<div class="flex items-start gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
											<CheckCircle2 size={17} />
										</div>
										<div class="min-w-0 flex-1">
											<p class="text-xs text-muted-foreground">{tr('Status')}</p>
											<div class="mt-0.5">
												<span class="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">
													{detailStatusLabel[detail.status] ?? detail.status}
												</span>
												<p class="text-xs text-muted-foreground mt-1">
													{#if detail.published_at}
														{tr('Dipublikasikan pada')} {dt(detail.published_at)}
													{:else if detail.status === 'waiting_partner'}
														{tr('Menunggu konfirmasi persetujuan')}
													{:else}
														{dt(detail.created_at)}
													{/if}
												</p>
											</div>
										</div>
									</div>
								</div>

								<!-- Kolom Kanan -->
								<div class="space-y-4">
									<!-- 1. Klien -->
									<div class="flex items-start gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
											<User size={17} />
										</div>
										<div class="min-w-0 flex-1">
											<p class="text-xs text-muted-foreground">{tr('Klien')}</p>
											<p class="text-sm font-semibold text-slate-800 mt-0.5 truncate">{detail.client_name ?? '—'}</p>
										</div>
									</div>

									<!-- 2. Dibuat -->
									<div class="flex items-start gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
											<Calendar size={17} />
										</div>
										<div class="min-w-0 flex-1">
											<p class="text-xs text-muted-foreground">{tr('Dibuat')}</p>
											<p class="text-sm font-semibold text-slate-800 mt-0.5">{dt(detail.created_at)}</p>
										</div>
									</div>

									<!-- 3. Terjadwal -->
									<div class="flex items-start gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
											<CalendarClock size={17} />
										</div>
										<div class="min-w-0 flex-1">
											<p class="text-xs text-muted-foreground">{tr('Terjadwal')}</p>
											<p class="text-sm font-semibold text-slate-800 mt-0.5">
												{detail.scheduled_at ? dt(detail.scheduled_at) : '—'}
											</p>
										</div>
									</div>

									<!-- 4. Integrasi -->
									<div class="flex items-start gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
											<Globe size={17} />
										</div>
										<div class="min-w-0 flex-1">
											<p class="text-xs text-muted-foreground">{tr('Integrasi')}</p>
											<p class="text-sm font-semibold text-slate-800 mt-0.5 capitalize">
												{integrationLabel[detail.media_integration_type ?? ''] ?? (detail.media_integration_type ?? '—')}
											</p>
										</div>
									</div>
								</div>
							</div>
						</section>

						<!-- 3. Section Pendapatan (Horizontal Green Card) -->
						<section class="rounded-2xl border border-emerald-200/90 bg-emerald-50/25 p-4 sm:p-5 flex flex-col md:flex-row md:items-center justify-between gap-4 shadow-sm">
							<div class="flex items-center gap-3">
								<div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-emerald-100 text-emerald-700 border border-emerald-200/50">
									<Wallet size={20} />
								</div>
								<div>
									<p class="text-sm font-bold text-slate-900">{tr('Pendapatan')}</p>
									<p class="text-xs text-muted-foreground">{tr('Pendapatan Anda dari publikasi ini.')}</p>
								</div>
							</div>

							<div class="flex flex-wrap items-center gap-3 sm:gap-4 text-xs sm:text-sm">
								<div>
									<p class="text-[11px] text-muted-foreground">{tr('Harga pengajuan')}</p>
									<p class="font-semibold text-slate-800 mt-0.5">{rp(incomeGross(detail))}</p>
								</div>
								<span class="text-slate-400 font-medium">—</span>
								<div>
									<p class="text-[11px] text-muted-foreground">{tr('Potongan platform')}</p>
									<p class="font-semibold text-slate-800 mt-0.5">{cutPercentLabel(detail) ?? 0}% ({rp(incomeCut(detail))})</p>
								</div>
								<span class="text-slate-400 font-medium">=</span>
								<div class="rounded-xl bg-emerald-100/60 border border-emerald-200 px-3.5 py-1.5 text-left">
									<p class="text-[10px] font-bold uppercase tracking-wider text-emerald-800">{tr('Pendapatan Anda')}</p>
									<p class="text-xl sm:text-2xl font-extrabold text-emerald-600 tabular-nums">{rp(detail.partner_amount)}</p>
								</div>
							</div>
						</section>

						<!-- 4. Section Preview / Content -->
						{#if detail.document}
							<section class="rounded-2xl border border-slate-200/80 bg-white p-5 space-y-4 shadow-sm">
								<div class="flex items-center gap-3">
									<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
										<FileText size={18} />
									</div>
									<div>
										<p class="text-sm font-bold text-slate-900">{tr('Preview / Content')}</p>
										<p class="text-xs text-muted-foreground">{tr('Lihat judul, slug, dan cuplikan konten artikel.')}</p>
									</div>
								</div>

								<div class="space-y-3 pt-1">
										{#if detail.document.slug}
											<div>
												<p class="text-xs font-medium text-muted-foreground mb-1">{tr('Slug')}</p>
												<div class="rounded-xl bg-slate-50 border border-slate-200/80 px-3.5 py-2.5 flex items-center justify-between gap-2">
													<span class="font-mono text-xs text-slate-700 truncate select-all">{detail.document.slug}</span>
													<button
														type="button"
														class="text-slate-400 hover:text-slate-700 transition-colors p-1 rounded shrink-0"
														title={copiedKey === 'slug' ? tr('Tersalin!') : tr('Salin Slug')}
														onclick={() => copyText(pub.document?.slug, 'slug')}
													>
														{#if copiedKey === 'slug'}
															<Check size={14} class="text-emerald-600" />
														{:else}
															<Copy size={14} />
														{/if}
													</button>
												</div>
											</div>
										{/if}

										{#if detail.document.meta_description}
											<div>
												<p class="text-xs font-medium text-muted-foreground mb-1">{tr('Cuplikan konten')}</p>
												<p class="text-xs sm:text-sm text-slate-600 leading-relaxed bg-slate-50/60 rounded-xl p-3 border border-slate-100">
													{detail.document.meta_description}
												</p>
											</div>
										{/if}

										<div>
											<button
												type="button"
												class="app-btn app-btn-soft !text-xs !py-1.5 !px-3 inline-flex items-center gap-1.5 font-medium"
												onclick={() => void previewDraft(pub)}
											>
												<ExternalLink size={13} />
												{tr('Lihat artikel lengkap')}
											</button>
										</div>
								</div>
							</section>
						{/if}

						<!-- 5. Section URL Terbit & Verifikasi -->
						{#if detail.status === 'published' || liveUrl}
							<section class="rounded-2xl border border-slate-200/80 bg-white p-5 space-y-4 shadow-sm">
								<div class="flex flex-wrap items-center justify-between gap-3">
									<div class="flex items-center gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
											<Link2 size={18} />
										</div>
										<div>
											<p class="text-sm font-bold text-slate-900">{tr('URL Terbit')}</p>
											<p class="text-xs text-muted-foreground">{tr('Link artikel yang sudah terpublish di media.')}</p>
										</div>
									</div>
									<div class="flex flex-wrap items-center gap-2">
										{#if vBadge}
											<span class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium bg-emerald-50 text-emerald-700 border border-emerald-200/70">
												<CheckCircle2 size={13} class="text-emerald-600" />
												{vBadge.label}
											</span>
										{/if}
										{#if sBadge}
											<span class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium bg-emerald-50 text-emerald-700 border border-emerald-200/70">
												<Smile size={13} class="text-emerald-600" />
												{sBadge.label}
											</span>
										{/if}
										{#if mBadge}
											<span class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium bg-slate-100 text-slate-700 border border-slate-200">
												{mBadge.label}
											</span>
										{/if}
									</div>
								</div>

								{#if liveUrl}
									<p class="text-xs sm:text-sm">
										<a href={liveUrl} target="_blank" rel="noreferrer" class="font-medium text-blue-600 hover:underline break-all inline-flex items-center gap-1">
											<span>{liveUrl}</span>
											<ExternalLink size={13} class="shrink-0" />
										</a>
									</p>
								{/if}

								<!-- Input bar + Tombol Buka URL / Edit URL -->
								<div class="flex items-center gap-2 flex-wrap sm:flex-nowrap">
									<div class="relative flex-1 min-w-[240px]">
										<input
											type="url"
											class="app-input w-full pr-9 text-xs sm:text-sm font-mono"
											placeholder={tr('https://situs-mitra.example/artikel/…')}
											readonly={!urlEditMode}
											disabled={!urlEditMode}
											bind:value={urlValue}
										/>
										{#if urlValue}
											<button
												type="button"
												class="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-700 transition-colors p-1 rounded"
												title={copiedKey === 'url' ? tr('Tersalin!') : tr('Salin URL')}
												onclick={() => copyText(urlValue, 'url')}
											>
												{#if copiedKey === 'url'}
													<Check size={14} class="text-emerald-600" />
												{:else}
													<Copy size={14} />
												{/if}
											</button>
										{/if}
									</div>

									{#if urlEditMode}
										<button
											type="button"
											class="app-btn app-btn-primary !text-xs !py-2 !px-4 shrink-0 inline-flex items-center gap-1.5 font-semibold"
											disabled={urlUpdating || !urlValue.trim()}
											onclick={() => void updatePublishedUrl()}
										>
											{urlUpdating ? tr('Menyimpan…') : tr('Simpan URL')}
										</button>
										<button
											type="button"
											class="app-btn app-btn-soft !text-xs !py-2 !px-3 shrink-0"
											disabled={urlUpdating}
											onclick={() => { urlEditMode = false; urlValue = liveUrl ?? ''; }}
										>
											{tr('Batal')}
										</button>
									{:else}
										{#if liveUrl}
											<button
												type="button"
												class="app-btn app-btn-primary !text-xs !py-2 !px-4 shrink-0 inline-flex items-center gap-1.5 font-semibold"
												onclick={() => window.open(liveUrl, '_blank', 'noopener')}
											>
												<ExternalLink size={13} />
												{tr('Buka URL')}
											</button>
										{/if}
										<button
											type="button"
											class="app-btn app-btn-soft !text-xs !py-2 !px-3 shrink-0 font-medium"
											disabled={urlUpdating}
											title={tr('Perbarui URL & verifikasi ulang')}
											onclick={() => (urlEditMode = true)}
										>
											{tr('Perbarui URL')}
										</button>
									{/if}
								</div>

								{#if urlSavedNotice}
									<p class="text-xs font-medium text-emerald-600">{urlSavedNotice}</p>
								{/if}
								{#if detail.verification?.error_message}
									<p class="text-xs font-medium text-amber-600">{verificationHint(detail.verification.error_message)}</p>
								{/if}

								<!-- Box status verifikasi URL -->
								<div class="rounded-xl border border-emerald-200/90 bg-emerald-50/50 p-3.5 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
									<div class="flex items-start gap-3">
										<CheckCircle2 size={20} class="text-emerald-600 shrink-0 mt-0.5" />
										<div>
											<p class="text-xs sm:text-sm font-bold text-emerald-950">{tr('Verifikasi URL')}</p>
											<p class="text-xs text-emerald-800 leading-snug mt-0.5">
												{tr('URL ini telah diverifikasi dan memenuhi kriteria untuk berpotensi tampil di AI Overview.')}
											</p>
										</div>
									</div>
										<button
											type="button"
											class="app-btn app-btn-soft !border-emerald-200 !bg-white hover:!bg-emerald-50 !text-emerald-800 !text-xs !py-1.5 !px-3 shrink-0 inline-flex items-center gap-1.5 font-semibold self-end sm:self-center"
											disabled={urlUpdating}
											onclick={() => void reverifyUrl()}
										>
											<RefreshCw size={13} class={urlUpdating ? 'animate-spin' : ''} />
											{urlUpdating ? tr('Memeriksa…') : tr('Cek Ulang')}
										</button>
								</div>
							</section>
						{/if}

						{#if detail.notes}
							<section class="rounded-2xl border border-slate-200/80 bg-white p-5 shadow-sm">
								<p class="text-xs font-bold uppercase tracking-wider text-muted-foreground mb-1">{tr('Catatan klien')}</p>
								<p class="text-sm text-slate-800 leading-relaxed bg-slate-50/60 p-3 rounded-xl border border-slate-100">{detail.notes}</p>
							</section>
						{/if}

						{#if detail.rejection_reason}
							<div class="rounded-xl border border-rose-200 bg-rose-50/70 p-3.5 text-xs text-rose-700">
								<span class="font-bold">{tr('Ditolak:')}</span> {detail.rejection_reason}
							</div>
						{/if}

						<!-- Revisi dari klien: klien mengubah artikel & memintanya dikirim ulang. -->
						{#if detail.status === 'revision'}
							<section class="rounded-2xl border border-amber-200/80 bg-amber-50/30 p-5 shadow-sm space-y-3">
								<p class="text-xs font-bold uppercase tracking-wider text-amber-800">{tr('Permintaan Revisi Klien')}</p>
								{#if (detail.revision_comments ?? []).length === 0}
									<p class="text-sm text-muted-foreground">{tr('Klien meminta revisi tanpa catatan tambahan.')}</p>
								{:else}
									{#each detail.revision_comments ?? [] as c (c.id)}
										<div class="border-t border-amber-100 pt-2.5 first:border-0 first:pt-0">
											<p class="text-sm text-slate-800 leading-relaxed">{c.body}</p>
											<p class="mt-0.5 text-xs text-muted-foreground">{dt(c.created_at)}</p>
											{#if c.attachment_url}
												<a href={c.attachment_url} target="_blank" rel="noreferrer" class="mt-1 inline-flex items-center gap-1 text-xs text-blue-600 underline font-medium">
													<Paperclip size={12} />{c.attachment_name ?? tr('Lampiran')}
												</a>
											{/if}
										</div>
									{/each}
								{/if}
								<p class="text-xs text-amber-800/80 leading-relaxed pt-1">
									{tr('Terima untuk mengirim ulang artikel revisi ke media Anda, atau tolak dengan alasan bila masih belum sesuai.')}
									{#if (detail.revision_reject_count ?? 0) > 0}
										{tr('Sudah ditolak')} {detail.revision_reject_count}/{detail.revision_reject_limit ?? 3} {tr('kali.')}
									{/if}
								</p>
							</section>
						{/if}

						<!-- 6. Section Riwayat Publikasi (Vertical Timeline) -->
						<section class="rounded-2xl border border-slate-200/80 bg-white p-5 space-y-4 shadow-sm">
							<div class="flex items-center gap-3">
								<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
									<Clock size={18} />
								</div>
								<div>
									<p class="text-sm font-bold text-slate-900">{tr('Riwayat')}</p>
									<p class="text-xs text-muted-foreground">{tr('Riwayat proses pengiriman dan publikasi artikel.')}</p>
								</div>
							</div>

							<div class="pt-2">
								{#if (detail.attempts ?? []).length > 0}
									<div class="relative pl-6 space-y-6 before:absolute before:left-2 before:top-2 before:bottom-2 before:w-[2px] before:bg-slate-100">
										{#each detail.attempts as a, idx (a.attempt_number)}
											{@const isFirst = idx === 0}
											<div class="relative flex flex-col sm:flex-row sm:items-center justify-between gap-1 text-xs">
												<span
													class="absolute -left-6 top-1 h-4 w-4 rounded-full border-2 border-white"
													class:bg-emerald-500={a.status === 'published'}
													class:ring-4={a.status === 'published'}
													class:ring-emerald-100={a.status === 'published'}
													class:bg-blue-400={a.status !== 'published' && isFirst}
													class:ring-blue-100={a.status !== 'published' && isFirst}
													class:bg-slate-300={a.status !== 'published' && !isFirst}
												></span>
													<div class="min-w-0 pr-4">
														<p class="text-sm font-semibold text-slate-800">
															{attemptLabels[a.status] ?? a.status}
														</p>
													{#if a.status === 'published'}
														<p class="text-xs text-muted-foreground mt-0.5">
															{tr('Artikel berhasil dipublikasikan di')} {detail.media_name ?? tr('media')}.
														</p>
													{:else if a.error_message}
														<p class="text-xs text-destructive mt-0.5">{a.error_message}</p>
													{:else}
														<p class="text-xs text-muted-foreground mt-0.5">
															{tr('Artikel diproses untuk')} {detail.media_name ?? tr('media')}.
														</p>
													{/if}
												</div>
												<span class="text-xs text-muted-foreground font-mono shrink-0">
													{dt(a.completed_at ?? a.started_at)}
												</span>
											</div>
										{/each}

										{#if detail.created_at}
											<div class="relative flex flex-col sm:flex-row sm:items-center justify-between gap-1 text-xs">
												<span class="absolute -left-6 top-1 h-4 w-4 rounded-full border-2 border-white bg-slate-300 ring-4 ring-slate-100"></span>
												<div>
													<p class="text-sm font-semibold text-slate-800">{tr('Pesanan dibuat')}</p>
													<p class="text-xs text-muted-foreground mt-0.5">{tr('Pesanan berhasil dibuat.')}</p>
												</div>
												<span class="text-xs text-muted-foreground font-mono shrink-0">{dt(detail.created_at)}</span>
											</div>
										{/if}
									</div>
								{:else}
									<div class="relative pl-6 space-y-6 before:absolute before:left-2 before:top-2 before:bottom-2 before:w-[2px] before:bg-slate-100">
											{#if detail.published_at}
												<div class="relative flex flex-col sm:flex-row sm:items-center justify-between gap-1 text-xs">
													<span class="absolute -left-6 top-1 h-4 w-4 rounded-full border-2 border-white bg-emerald-500 ring-4 ring-emerald-100"></span>
													<div>
														<p class="text-sm font-semibold text-slate-800">{tr('Terbit')}</p>
														<p class="text-xs text-muted-foreground mt-0.5">{tr('Artikel berhasil dipublikasikan di')} {detail.media_name ?? tr('media')}.</p>
													</div>
													<span class="text-xs text-muted-foreground font-mono shrink-0">{dt(detail.published_at)}</span>
												</div>
											{/if}
										{#if detail.created_at}
											<div class="relative flex flex-col sm:flex-row sm:items-center justify-between gap-1 text-xs">
												<span class="absolute -left-6 top-1 h-4 w-4 rounded-full border-2 border-white bg-slate-300 ring-4 ring-slate-100"></span>
												<div>
													<p class="text-sm font-semibold text-slate-800">{tr('Pesanan dibuat')}</p>
													<p class="text-xs text-muted-foreground mt-0.5">{tr('Pesanan berhasil dibuat.')}</p>
												</div>
												<span class="text-xs text-muted-foreground font-mono shrink-0">{dt(detail.created_at)}</span>
											</div>
										{/if}
									</div>
								{/if}
							</div>
						</section>
					{/if}
				</div>

					<!-- ── Footer Aksi (jika ada tombol persetujuan atau revisi) ── -->
					{#if detail && (detail.status === 'revision' || detail.status === 'waiting_partner')}
						{@const p = detail}
						<footer class="flex-none border-t border-slate-100 bg-white px-5 py-3.5 sm:px-6 sm:py-4 flex flex-wrap justify-end gap-2.5">
							{#if p.status === 'revision'}
								{#if (p.revision_reject_count ?? 0) < (p.revision_reject_limit ?? 3)}
									<button
										class="app-btn app-btn-soft"
										style="color: var(--destructive);"
										disabled={busy}
										onclick={() => { rejectTarget = p; rejectReason = ''; }}
									><Trash2 size={15} />{tr('Tolak Revisi')}</button>
								{/if}
								<button class="app-btn app-btn-primary" disabled={busy} onclick={() => void acceptRevision(p)}><Check size={15} />{tr('Terima Revisi')}</button>
							{:else if p.status === 'waiting_partner'}
								<button
									class="app-btn app-btn-soft"
									style="color: var(--destructive);"
									disabled={busy}
									onclick={() => { rejectTarget = p; rejectReason = ''; }}
								><Trash2 size={15} />{tr('Tolak')}</button>
								{#if isManualRow(p)}
									<button class="app-btn app-btn-primary" disabled={busy} onclick={() => void approve(p, 'publish')}><Check size={15} />{tr('Setujui')}</button>
								{:else}
									<button class="app-btn app-btn-primary" disabled={busy} onclick={() => void approve(p, 'publish')}><Check size={15} />{tr('Setujui & Terbitkan')}</button>
								{/if}
							{/if}
						</footer>
					{/if}
			</dialog>
		</div>
	{/if}

{#if showPreview && previewPub}
	<!-- Preview dari baris menu (z-50). Saat editor draf terbuka di belakangnya,
	     nilainya dinaikkan agar preview tampil DI ATAS editor. -->
	<div
		class="fixed inset-0 flex items-center justify-center p-4"
		style={`z-index: ${draftPub ? 90 : 50}; background-color: rgba(0,0,0,.45);`}
		role="presentation"
		onclick={() => (showPreview = false)}
		onkeydown={(e) => { if (e.key === 'Escape') showPreview = false; }}
	>
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

<!-- Editor draf — modal besar dua kolom: editor isi di kiri (scroll sendiri),
     pengaturan artikel di sidebar kanan (Gambar Utama → Judul → Meta → Kategori);
     header & footer tetap terlihat saat konten digulir. Mobile = layar penuh
     dengan kolom bertumpuk. -->
{#if draftPub}
	<div class="fixed inset-0 z-[60] flex items-stretch justify-center p-0 sm:items-center sm:p-4" style="background-color: rgba(0,0,0,.5);" role="presentation" onclick={closeDraftEditor}>
		<dialog
			open
			class="app-card flex h-full w-full flex-col overflow-hidden rounded-none sm:h-[min(92dvh,960px)] sm:max-w-[min(95vw,1600px)] sm:rounded-2xl"
			style="box-shadow: 0 24px 64px rgba(15, 23, 42, 0.28);"
			aria-label={tr('Edit draf')}
			onclick={(e) => e.stopPropagation()}
		>
			<!-- ── Header tetap ── -->
			<header class="flex flex-none items-center gap-3 border-b px-4 py-3 sm:px-5" style="border-color: var(--border);">
				<span class="flex h-10 w-10 flex-none items-center justify-center rounded-xl" style="background: var(--accent); color: var(--primary);" aria-hidden="true">
					<FileText size={19} />
				</span>
				<div class="min-w-0 flex-1">
					<h2 class="truncate text-base font-semibold">{tr('Isi Artikel')}</h2>
					<p class="truncate text-xs text-muted-foreground">{draftPub.document_title ?? tr('Artikel')}</p>
				</div>
				{#if draft && !draft.error}
					<button
						class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
						disabled={busy || rewriteLeft <= 0}
						title={rewriteLeft <= 0 ? tr('Batas Rewrite AI tercapai — maksimal 2 kali per artikel.') : tr('Gratis, maksimal 2 kali per artikel. AI The Writer menulis ulang isi artikel mengikuti sudut pandang angle yang dipilih klien di kampanyenya.')}
						onclick={rewrite}
					>
						{#if busy}<LoaderCircle size={13} class="animate-spin" />{:else}<Sparkles size={13} />{/if}
						{tr('Rewrite AI')} — {tr('gratis')} · {tr('sisa')} {rewriteLeft}/{2}
					</button>
				{/if}
				<button
					type="button"
					class="flex h-8 w-8 flex-none items-center justify-center rounded-lg text-muted-foreground hover:bg-secondary"
					aria-label={tr('Tutup')}
					title={tr('Tutup')}
					onclick={closeDraftEditor}
				>
					<X size={18} />
				</button>
			</header>

			{#if error}<p class="flex-none border-b px-4 py-2 text-sm sm:px-5" style="border-color: var(--border); color: var(--destructive);">{error}</p>{/if}

			{#if draftLoading}
				<div class="min-h-0 flex-1 overflow-y-auto p-5"><p class="text-sm text-muted-foreground">{tr('Memuat draf…')}</p></div>
			{:else if draft?.error}
				<div class="min-h-0 flex-1 overflow-y-auto p-5"><p class="text-sm" style="color: var(--destructive);">{draft.error}</p></div>
			{:else if draft}
				<form class="flex min-h-0 flex-1 flex-col" onsubmit={saveDraft}>
					<div class="flex min-h-0 flex-1 flex-col overflow-y-auto lg:flex-row lg:overflow-hidden">
						<!-- ── Kolom kiri: judul di atas editor (bold besar), lalu rich text editor (scroll sendiri) ── -->
						<div class="flex min-h-0 w-full flex-1 flex-col lg:border-r" style="border-color: var(--border); min-width: 0;">
							<div class="flex flex-none flex-col px-4 pb-2 pt-3 sm:px-5">
								<div class="flex items-center justify-between gap-3">
									<span class="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">{tr('Judul Artikel')}</span>
									<span
										class="pointer-events-none text-[11px] tabular-nums"
										style={(draft.title ?? '').length >= 200 ? 'color: var(--destructive);' : 'color: var(--muted-foreground);'}
									>{(draft.title ?? '').length}/200</span>
								</div>
								<input
									class="w-full bg-transparent text-[22px] font-bold leading-snug tracking-tight outline-none"
									maxlength="200"
									bind:value={draft.title}
									aria-label={tr('Judul Artikel')}
								/>
							</div>
							<div class="flex min-h-0 flex-1 flex-col px-3 pt-1 sm:px-4">
								<RichTextEditor
									bind:value={draft.content_html}
									bind:stats={editorStats}
									fill
									ariaLabel={tr('Isi artikel')}
									minHeight={320}
									uploadImage={uploadDraftImage}
									disabled={busy}
								/>
							</div>
							<div class="flex flex-none items-center justify-between gap-3 border-t px-4 py-2.5" style="border-color: var(--border);">
								<p class="text-xs tabular-nums text-muted-foreground">{editorStats.words} {tr('kata')}</p>
								<button type="button" class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" onclick={previewCurrentDraft}>
									<Eye size={14} /> {tr('Preview')}
								</button>
							</div>
						</div>

						<!-- ── Kolom kanan: sidebar metadata (scroll sendiri) ── -->
						<aside class="min-h-0 w-full flex-none overflow-y-auto px-4 py-4 sm:px-5 lg:w-[352px] xl:w-[400px]">
							<!-- A. Gambar Utama — selalu paling atas -->
							<section>
								<h3 class="text-[13px] font-semibold">{tr('Gambar Utama')}</h3>
								<p class="mt-0.5 text-xs text-muted-foreground">{tr('Gambar akan ditampilkan di Tauthub & dikirim ke media.')}</p>
								<div class="mt-2.5 overflow-hidden rounded-xl border" style="border-color: var(--border); background: var(--secondary); aspect-ratio: 16 / 9;">
									{#if draft.featured_image_url}
										<div class="relative h-full w-full">
											<img src={draft.featured_image_url} alt={tr('Gambar cover artikel')} class="h-full w-full object-cover" loading="lazy" />
											<button
												type="button"
												class="absolute right-2 top-2 flex h-6 w-6 items-center justify-center rounded-full"
												style="background: rgba(17, 24, 39, 0.55); color: #fff;"
												aria-label={tr('Hapus gambar')}
												title={tr('Hapus gambar')}
												onclick={() => { draft.featured_image_url = ''; void measureDraftImages(); }}
											>
												<X size={13} />
											</button>
										</div>
									{:else}
										<div class="flex h-full flex-col items-center justify-center gap-1.5 text-center">
											<ImageIcon size={22} style="color: var(--muted-foreground);" />
											<p class="text-xs text-muted-foreground">{tr('Belum ada gambar utama')}</p>
											<button type="button" class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" onclick={() => coverInput?.click()}>
												<ImageIcon size={13} /> {tr('Tambah gambar')}
											</button>
										</div>
									{/if}
								</div>
								{#if draft.featured_image_url}
									<div class="mt-2 flex flex-wrap gap-1.5">
										<button
											type="button"
											class="app-btn !min-h-8 !flex-1 !px-2 !text-xs"
											style="background: var(--accent); color: var(--primary);"
											disabled={coverBusy}
											onclick={() => coverInput?.click()}
										>
											{#if coverBusy}<LoaderCircle size={13} class="animate-spin" />{:else}<ImageIcon size={13} />{/if}
											{tr('Ganti gambar')}
										</button>
										<button
											type="button"
											class="app-btn app-btn-soft !min-h-8 !flex-1 !px-2 !text-xs"
											style="color: var(--destructive); background: color-mix(in srgb, var(--destructive) 8%, transparent);"
											onclick={() => { draft.featured_image_url = ''; void measureDraftImages(); }}
										>
											<Trash2 size={13} /> {tr('Hapus gambar')}
										</button>
									</div>
								{/if}
								<p class="mt-1.5 text-[11px] text-muted-foreground">{tr('Format: JPG, PNG, atau WebP · JPG/PNG otomatis dikonversi ke WebP maks. 150 KB · disarankan rasio 16:9.')}</p>
							</section>

							<!-- C. Meta Deskripsi -->
							<section class="mt-5 border-t pt-4" style="border-color: var(--border);">
								<label class="block">
									<span class="text-[13px] font-semibold">{tr('Meta Deskripsi')}</span>
									<div class="relative mt-1.5">
										<textarea
											class="app-input w-full resize-none leading-relaxed pb-7"
											rows="4"
											maxlength="500"
											bind:value={draft.meta_description}
										></textarea>
										<span
											class="pointer-events-none absolute right-3 bottom-2.5 text-[11px] tabular-nums"
											style={(draft.meta_description ?? '').length >= 500 ? 'color: var(--destructive);' : 'color: var(--muted-foreground);'}
										>{(draft.meta_description ?? '').length}/500</span>
									</div>
								</label>
							</section>

							<!-- D. Kategori -->
							<section class="mt-5 border-t pb-4 pt-4" style="border-color: var(--border);">
								<span class="text-[13px] font-semibold">{tr('Kategori')}</span>
								{#if draft}
									<div class="draft-cat mt-1.5">
										<Select
											ariaLabel={tr('Kategori')}
											variant="plain"
											minWidth={0}
											options={(catOptions.includes(draft.category) || !(draft.category ?? '').trim()
												? catOptions
												: [(draft.category ?? ''), ...catOptions]).map((c) => ({ value: c, label: c }))}
											bind:value={draft.category}
											onchange={() => ensureCatOption(draft.category ?? '')}
										>
											{#snippet footer()}
												<div class="flex items-center gap-1.5 p-2 pt-1">
													<input
														class="app-input min-w-0 flex-1 !py-1.5 !text-xs"
														bind:value={catNewInput}
														placeholder={tr('Kategori baru…')}
														onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); void addCategory(); } }}
													/>
													<button
														type="button"
														class="app-btn app-btn-primary !min-h-7 !px-2 !text-xs shrink-0"
														disabled={catNewBusy || !(catNewInput ?? '').trim()}
														onclick={() => void addCategory()}
													>{catNewBusy ? tr('Menyimpan…') : tr('Tambah')}</button>
												</div>
											{/snippet}
										</Select>
									</div>
								{/if}
							</section>

							<input bind:this={coverInput} type="file" accept="image/png,image/jpeg,image/webp" class="hidden" onchange={pickCover} />
						</aside>
					</div>

					<!-- ── Footer tetap ── -->
					<footer class="flex flex-none flex-wrap items-center justify-between gap-2 border-t px-4 py-3 sm:px-5" style="border-color: var(--border);">
						<div class="flex min-w-0 flex-1 items-center gap-1.5 text-xs text-muted-foreground" title={
							(draft.source === 'wp_draft' ? tr('Sumber: draf WordPress') : tr('Sumber: dokumen klien'))
							+ (draft.edited_at ? ` · ${tr('diedit')} ${dt(draft.edited_at)}` : '')
						}>
							<FileText size={14} class="flex-none" />
							<span class="min-w-0 truncate">
								{draft.source === 'wp_draft' ? tr('Sumber: draf WordPress') : tr('Sumber: dokumen klien')}
								{#if draft.edited_at} · {tr('diedit')} {dt(draft.edited_at)}{/if}
								{#if draftImgCount > 0}
									· {#if !draftImgUnknown && draftImgBytes != null}{fmtBytesID(draftImgBytes)}{:else}±{draftImgCount} {tr('gambar')}{/if}
								{/if}
							</span>
						</div>
						<div class="flex flex-none items-center gap-2">
							{#if draftSavedNotice}<p class="hidden text-xs sm:block" style="color: var(--success);">{draftSavedNotice}</p>{/if}
							{#if coverError}<p class="hidden text-xs sm:block" style="color: var(--destructive);">{coverError}</p>{/if}
							<button type="button" class="app-btn app-btn-soft" onclick={closeDraftEditor}>{tr('Tutup')}</button>
							<button type="submit" class="app-btn app-btn-primary" disabled={busy}>
								{#if busy}<LoaderCircle size={14} class="animate-spin" /> {tr('Menyimpan draf…')}{:else}{tr('Simpan Draf')}{/if}
							</button>
						</div>
					</footer>
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

	/* Sidebar Kategori: dropdown berstyle (bukan <select> bawaan) dipaksa
	   selebar sidebar, tinggi 44px, dengan border/focus yang sama seperti
	   app-input. Analisis :global diperlukan karena trigger ada di komponen
	   Select; PENUH lebar tidak mengubah pemakaian Select di halaman lain. */
	.draft-cat :global(.zs-select-trigger) {
		width: 100%;
		max-width: none;
		height: 44px;
		justify-content: space-between;
		border: 1px solid var(--input);
		border-radius: var(--radius-control);
		background: var(--card);
	}
	.draft-cat :global(.zs-select-trigger:hover:not(:disabled)) {
		background: var(--secondary);
		border-color: var(--border);
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
