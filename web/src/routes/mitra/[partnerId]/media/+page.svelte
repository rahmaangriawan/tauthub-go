<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Daftar Media mitra — port section "Daftar Media" portal mitra:
	// daftar + filter status, tambah media, edit, arsip, dan impor massal XLSX
	// (pratinjau → konfirmasi, parity alur bulk import app lama).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { BadgeCheck, Check, CircleAlert, Copy, ExternalLink, Info, Link, Lock, MoreVertical, Pencil, Plug, Plus, RotateCcw, Search, ShieldCheck, Trash2, TriangleAlert, Upload, X } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import { Tooltip } from 'bits-ui';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	const partnerId = $derived(page.params.partnerId as string);
	const base = $derived(`/mitra/${getPartnerUrlId() || partnerId}`);

	type Media = {
		id: string; name: string; category: string | null; categories: string[];
		website_url: string | null; price: number; effective_price: number; client_price: number;
		markup_percent: number; status: string; integration_type: string;
		publication_mode: string; traffic_estimate: number; max_links: number | null;
		bahasa_artikel: string[]; label_sponsor: string; dewan_pers_verified: boolean;
		pending_publications: number; total_terbit: number; price_status: string;
		topik_terlarang: string[]; contoh_artikel_url: string | null;
		// Kolom pembanding dialog tambah/edit (semua dikirim mediaJSON server).
		media_type: string; description: string | null; adapter_url: string | null;
		notes: string | null; pricing_type: string; word_count_mode: string;
		allow_embed: boolean; ai_summary: string | null; ai_summary_generated_at: string | null;
		// Kolom tabel app lama (dipakai untuk badge & aksi).
		auto_publish: boolean; dewan_pers_status: string | null; dewan_pers_rejection_reason: string | null;
		health_score: number | null; health_breakdown: Record<string, unknown> | null;
		seo_da: number | null; seo_pa: number | null; seo_spam_score: number | null; seo_checked_at: string | null;
		has_connection: boolean; custom_web_status: string | null; blogger_status: string | null;
		rejection_reason: string | null; domain_verified: boolean;
	};

	let items = $state<Media[]>([]);
	let total = $state(0);
	let pageNo = $state(1);
	const PAGE_SIZE = 50;
	let autoBusy = $state<string | null>(null);
	let categories = $state<string[]>([]);
	let autoPublishEnabled = $state(false);
	// Impor massal baru terbuka setelah media pertama disetujui admin dan
	// domainnya terverifikasi. Nilainya datang dari server (sumber kebenaran).
	let bulkUnlocked = $state(false);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');
	let q = $state('');
	let statusFilter = $state('');
	let dewanFilter = $state('');

	// ── Form tambah/edit — parity dialog app lama (sections.tsx): kartu-kartu
	// A. Informasi Media … Kesimpulan Website, gerbang persetujuan integrasi,
	// dan konfirmasi "Simpan & Ajukan Review" untuk media yang berubah. ──────
	type MediaForm = {
		name: string; media_type: string; categories: string[]; categoryQuery: string;
		website_url: string; adapter_url: string; price: string; pricing_type: string;
		traffic_estimate: number; max_links: string; description: string; notes: string;
		integration_type: string; publication_mode: string; bahasa_artikel: string[];
		label_sponsor: string; word_count_mode: string; allow_embed: boolean;
		topik_terlarang: string[]; topikQuery: string; contoh_artikel_url: string;
	};
	const WORD_COUNT_MODES = [
		{ value: 'sesuai_permintaan', label: 'Sesuai permintaan klien (bebas)' },
		{ value: 'kata_300_700', label: '300–700 kata' },
		{ value: 'kata_800_1500', label: '800–1500 kata' }
	];
	/** Nilai tak dikenal dari DB → mode bawaan (parity lib/media/word-count). */
	const normalizeWordCountMode = (v: unknown) =>
		WORD_COUNT_MODES.some((m) => m.value === v) ? String(v) : 'sesuai_permintaan';
	const emptyMediaForm = (): MediaForm => ({
		name: '', media_type: 'mainstream', categories: [], categoryQuery: '',
		website_url: '', adapter_url: '', price: '', pricing_type: 'nett',
		traffic_estimate: 0, max_links: '1', description: '', notes: '',
		integration_type: 'wordpress', publication_mode: 'draft',
		bahasa_artikel: ['Indonesia'], label_sponsor: 'tidak_ada',
		word_count_mode: 'sesuai_permintaan', allow_embed: false,
		topik_terlarang: [], topikQuery: '', contoh_artikel_url: ''
	});
	let showForm = $state(false);
	let editingId = $state<string | null>(null);
	let editingRow = $state<Media | null>(null);
	let form = $state<MediaForm>(emptyMediaForm());
	let integrasiConsent = $state(false);
	let dialogError = $state('');
	let editConfirmOpen = $state(false);
	/** Snapshot form saat openEdit — pembanding dirty sederhana. */
	let formSnapshot = $state('');
	type AiSummaryStatus = {
		status: string; summary: string | null; generated_at: string | null;
		pages: { url: string; ok: boolean; chars: number }[]; error: string | null;
	};
	let aiSummaryStatus = $state<AiSummaryStatus | null>(null);

	const comparableForm = (f: MediaForm) =>
		JSON.stringify([
			f.name, f.media_type, f.categories, f.website_url, f.adapter_url, f.price,
			f.pricing_type, f.traffic_estimate, f.max_links, f.description, f.notes,
			f.integration_type, f.publication_mode, f.bahasa_artikel, f.label_sponsor,
			f.word_count_mode, f.allow_embed, f.topik_terlarang, f.contoh_artikel_url
		]);
	const isDirty = $derived(editingId !== null && formSnapshot !== comparableForm(form));

	// Saran kategori (maks 8) dari pool server, minus yang sudah dipilih.
	const catQuery = $derived(form.categoryQuery.trim().toLowerCase());
	const catPool = $derived(
		categories.filter((c) => c.trim() !== '' && !form.categories.some((s) => s.trim().toLowerCase() === c.trim().toLowerCase()))
	);
	const catSuggestions = $derived(catQuery ? catPool.filter((c) => c.toLowerCase().includes(catQuery)) : catPool.slice(0, 8));

	// Baris status "Kesimpulan Website" dari GET /v1/partner/media/{id}/ai-summary.
	const aiSummaryLine = $derived.by(() => {
		if (!aiSummaryStatus) return null;
		const okPages = (aiSummaryStatus.pages ?? [])
			.filter((p) => p.ok)
			.map((p) => p.url.replace(/^https?:\/\/(www\.)?/, ''));
		if (aiSummaryStatus.status === 'success' || editingRow?.ai_summary) {
			return okPages.length > 0
				? `${tr('AI sudah menarik data dari:')} ${okPages.join(', ')}`
				: tr('Kesimpulan dibuat dari data mitra. Web media belum dapat diakses AI saat ini.');
		}
		if (aiSummaryStatus.status === 'processing') return tr('AI sedang menarik data dari web media…');
		if (aiSummaryStatus.status === 'queued') return tr('Menunggu antrean global AI…');
		if (aiSummaryStatus.status === 'scheduled') return tr('Masuk antrean global AI.');
		return tr('Pengambilan data gagal, sistem akan mencoba ulang otomatis.');
	});

	// Impor massal
	let importResult = $state<any>(null);
	let importError = $state('');
	// Server mem-parse XLSX sendiri, jadi langkah konfirmasi mengunggah ULANG file
	// yang sama. Simpan objek File-nya: input file dikosongkan setelah pratinjau,
	// tanpa ini tombol konfirmasi selalu gagal ("pilih ulang file").
	let importFile = $state<File | null>(null);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		// Menu aksi memakai posisi fixed: tutup saat digulir atau ukuran jendela
		// berubah supaya tidak tampak melayang menjauh dari barisnya. capture=true
		// agar gulir di dalam kontainer (bukan hanya window) ikut tertangkap.
		const onViewportChange = () => closeMenu();
		window.addEventListener('scroll', onViewportChange, true);
		window.addEventListener('resize', onViewportChange);
		return () => {
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
			window.removeEventListener('scroll', onViewportChange, true);
			window.removeEventListener('resize', onViewportChange);
		};
	});

	async function load() {
		const params = new URLSearchParams({
			limit: String(PAGE_SIZE),
			offset: String((pageNo - 1) * PAGE_SIZE)
		});
		if (statusFilter) params.set('status', statusFilter);
		// "Semua status" pada UI berarti tanpa arsip (label app lama); tanpa ini
		// media yang diarsipkan ikut tampil di daftar utama.
		else params.set('exclude_archived', 'true');
		if (dewanFilter) params.set('dewan_pers', dewanFilter);
		const res = await apiRequest(`/v1/partner/media?${params}`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat daftar media.'));
			return;
		}
		error = '';
		const data = await res.json();
		items = data.media ?? [];
		total = data.total ?? items.length;
		categories = data.categories ?? [];
		autoPublishEnabled = Boolean(data.auto_publish_enabled);
		bulkUnlocked = Boolean(data.bulk_unlocked);
	}

	/** Toggle "Mode Otomatis" per media (parity app lama: PUT {auto_publish}). */
	async function toggleAutoPublish(m: Media, checked: boolean) {
		autoBusy = m.id;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/media/${m.id}`, {
				method: 'PUT',
				body: JSON.stringify({ auto_publish: checked })
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal mengubah mode otomatis.'));
				await load();
				return;
			}
			notice = checked
				? `${tr('Mode otomatis diaktifkan untuk')} ${m.name}.`
				: `${tr('Mode otomatis dimatikan untuk')} ${m.name}.`;
			await load();
		} finally {
			autoBusy = null;
		}
	}

	/** Test koneksi publishing API media (parity app lama). */
	async function testConnection(m: Media) {
		error = '';
		notice = '';
		const res = await apiRequest(`/v1/partner/media/${m.id}/test-publishing-api`, { method: 'POST' });
		const data = await res.json().catch(() => null);
		if (!res.ok) {
			error = await readApiError(res, tr('Test koneksi gagal.'));
			return;
		}
		notice = `${m.name}: ${data?.message ?? tr('Koneksi berhasil diuji.')}`;
		await load();
	}

	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '—');

	/** Badge status Dewan Pers (parity kolom app lama). */
	const dpBadge = (s: string | null) =>
		s === 'verified' ? { label: tr('Terverifikasi'), tone: 'var(--success)' }
		: s === 'pending' ? { label: tr('Menunggu'), tone: 'var(--warning)' }
		: s === 'rejected' ? { label: tr('Ditolak'), tone: 'var(--destructive)' }
		: s === 'conflict' ? { label: tr('Konflik'), tone: 'var(--warning)' }
		: { label: tr('Tidak'), tone: 'var(--muted-foreground)' };
	const seoLabel = (m: Media) => (m.seo_da == null ? null : `DA ${m.seo_da} · PA ${m.seo_pa ?? '—'} · Spam ${m.seo_spam_score ?? '—'}`);

	// Nama platform memakai istilah yang sama dengan halaman Integrasi
	// (WordPress, Web Custom, Blogger). Tipe lain tetap punya sebutannya sendiri
	// supaya tidak semua jenis jatuh ke kata "Manual".
	const platLabel = (m: Media) => {
		switch (m.integration_type) {
			case 'wordpress':
				return 'WordPress';
			case 'custom_web':
				return 'Web Custom';
			case 'blogger':
				return 'Blogger';
			case 'tulisio_api':
				return 'Tulisio API';
			case 'rss':
				return tr('Feed RSS');
			case 'webhook':
				return 'Webhook';
			case 'emdash':
				return 'EmDash';
			default:
				return tr('Manual');
		}
	};

	/** Tooltip header kolom Mode Publikasi: siapa yang menuntaskan penerbitan. */
	const modeColumnHint = () =>
		tr('Siapa yang menerbitkan artikel. Mode Integrasi = di-push otomatis ke situs mitra lewat integrasi; Mode Manual = mitra menempel artikelnya sendiri lalu mengirim URL artikelnya.');

	/** Tooltip header kolom Platform: platform yang dipakai media. */
	const platformColumnHint = () => tr('Platform yang digunakan media, WordPress, Blogger, Emdash, API');

	/** Tooltip centang Mode Otomatis: apa akibatnya dan kapan dipakai. */
	const autoPublishHint = () =>
		tr('Kalau dicentang, artikel dari klien langsung diproses dan terbit ke situs media tanpa menunggu persetujuan mitra. Kalau tidak dicentang, mitra harus menyetujui dulu setiap artikel.');

	// ── Menu aksi titik-tiga (parity app lama) ────────────────────────────
	let openMenu = $state<string | null>(null);
	function closeMenu() {
		openMenu = null;
	}

	// Menu aksi diposisikan `fixed` mengikuti tombol pemicunya: dengan begitu ia
	// tidak ikut mempengaruhi layout tabel, dan bisa dibuka ke atas saat ruang di
	// bawah tidak cukup (mis. baris terakhir). Untuk arah ke atas, yang ditempelkan
	// adalah DASAR menu ke atas tombol, jadi tingginya tidak perlu ditebak.
	const MENU_SPACE_NEEDED = 230; // perkiraan tinggi menu 6 item
	let menuRight = $state(0);
	let menuTop = $state(0);
	let menuBottom = $state<number | null>(null);

	function positionMenu(trigger: HTMLElement) {
		const rect = trigger.getBoundingClientRect();
		// Minimal 8px dari tepi kanan supaya menu 208px tidak keluar viewport.
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

	// ── Verifikasi kepemilikan domain (meta tag tauthub-verification) ─────
	let domTarget = $state<Media | null>(null);
	let domToken = $state('');
	let domBusy = $state(false);
	let domResult = $state<{ status: string; found_token?: string | null } | null>(null);
	let domCopied = $state<string | null>(null);

	async function openDomainVerify(m: Media) {
		domTarget = m;
		domToken = '';
		domResult = null;
		domBusy = false;
		const res = await apiRequest(`/v1/partner/media/${m.id}/domain-verification`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat token verifikasi domain.'));
			domTarget = null;
			return;
		}
		domToken = (await res.json()).token ?? '';
	}

	async function verifyDomainNow() {
		if (!domTarget) return;
		domBusy = true;
		domResult = null;
		try {
			const res = await apiRequest(`/v1/partner/media/${domTarget.id}/domain-verification`, { method: 'POST' });
			const data = await res.json().catch(() => null);
			if (!res.ok) {
				error = await readApiError(res, tr('Verifikasi domain gagal.'));
				return;
			}
			domResult = { status: data?.status ?? (data?.ok ? 'verified' : 'unknown'), found_token: data?.found_token ?? null };
			if (data?.domain_verified || data?.status === 'verified') notice = tr('Kepemilikan domain terverifikasi.');
			await load();
		} finally {
			domBusy = false;
		}
	}

	async function copyText(label: string, value: string) {
		try {
			await navigator.clipboard.writeText(value);
			domCopied = label;
			setTimeout(() => (domCopied = null), 1600);
		} catch {
			error = tr('Gagal menyalin. Salin manual dari kotak teks.');
		}
	}

	// ── Contoh artikel (modal; parity app lama — dipakai dari daftar media
	// maupun dari dialog tambah/edit) ────────────────────────────────────────
	let contohTarget = $state<Media | null>(null);
	let contohFromForm = $state(false);
	let contohUrl = $state('');
	let contohBusy = $state(false);

	function openContoh(m: Media) {
		contohTarget = m;
		contohFromForm = false;
		contohUrl = m.contoh_artikel_url ?? '';
	}

	/** Dari dialog tambah/edit: URL disimpan ke form dulu, ikut payload utama. */
	function openContohFromForm() {
		contohTarget = null;
		contohFromForm = true;
		contohUrl = form.contoh_artikel_url;
	}

	function closeContoh() {
		contohTarget = null;
		contohFromForm = false;
	}

	async function saveContoh(e: SubmitEvent) {
		e.preventDefault();
		if (contohFromForm) {
			form.contoh_artikel_url = contohUrl.trim();
			closeContoh();
			return;
		}
		if (!contohTarget) return;
		contohBusy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/media/${contohTarget.id}`, {
				method: 'PUT',
				body: JSON.stringify({ contoh_artikel_url: contohUrl.trim() })
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menyimpan contoh artikel.'));
				return;
			}
			notice = tr('Contoh artikel tersimpan.');
			closeContoh();
			await load();
		} finally {
			contohBusy = false;
		}
	}

	// ── Pengajuan verifikasi Dewan Pers ───────────────────────────────────
	let dpTarget = $state<Media | null>(null);
	let dpCert = $state('');
	let dpDoc = $state<File | null>(null);
	let dpBusy = $state(false);

	function openDewanPers(m: Media) {
		dpTarget = m;
		dpCert = '';
		dpDoc = null;
		dpBusy = false;
	}

	async function submitDewanPers(e: SubmitEvent) {
		e.preventDefault();
		if (!dpTarget) return;
		dpBusy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/media/${dpTarget.id}/dewan-pers-verification`, {
				method: 'POST',
				body: JSON.stringify({ certificate_no: dpCert.trim() })
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Pengajuan Dewan Pers gagal.'));
				return;
			}
			if (dpDoc) {
				const form = new FormData();
				form.append('file', dpDoc);
				const up = await apiRequest(`/v1/partner/media/${dpTarget.id}/dewan-pers-document`, { method: 'POST', body: form });
				if (!up.ok) {
					error = await readApiError(up, tr('Sertifikat tersimpan, tetapi unggah dokumen gagal.'));
					await load();
					return;
				}
			}
			notice = tr('Pengajuan verifikasi Dewan Pers terkirim dan menunggu review admin.');
			dpTarget = null;
			await load();
		} finally {
			dpBusy = false;
		}
	}

	const filtered = $derived(
		items.filter((m) => {
			if (!q.trim()) return true;
			const term = q.trim().toLowerCase();
			return m.name.toLowerCase().includes(term) || (m.website_url ?? '').toLowerCase().includes(term);
		})
	);

	function openCreate() {
		editingId = null;
		editingRow = null;
		dialogError = '';
		integrasiConsent = false;
		aiSummaryStatus = null;
		form = emptyMediaForm();
		showForm = true;
	}

	function openEdit(m: Media) {
		editingId = m.id;
		editingRow = m;
		dialogError = '';
		// Preset persetujuan integrasi: media mode integrasi dianggap sudah
		// menyetujui ketentuan saat dialog dibuka (parity app lama).
		integrasiConsent = m.publication_mode === 'publish';
		aiSummaryStatus = null;
		const cats = m.categories.length ? [...m.categories] : m.category ? [m.category] : [];
		form = {
			name: m.name,
			media_type: m.media_type === 'blogger' ? 'blogger' : 'mainstream',
			categories: cats,
			categoryQuery: '',
			website_url: m.website_url ?? '',
			adapter_url: m.adapter_url ?? '',
			price: m.price ? String(m.price) : '',
			pricing_type: m.pricing_type ?? 'nett',
			traffic_estimate: m.traffic_estimate ?? 0,
			max_links: ['0', '1', '2', '3'].includes(String(m.max_links ?? 1)) ? String(m.max_links ?? 1) : '1',
			description: m.description ?? '',
			notes: m.notes ?? '',
			integration_type: m.integration_type || 'manual',
			publication_mode: m.publication_mode === 'publish' ? 'publish' : 'draft',
			bahasa_artikel: m.bahasa_artikel.length ? [...m.bahasa_artikel] : ['Indonesia'],
			label_sponsor: m.label_sponsor === 'ada_sponsor' ? 'ada_sponsor' : 'tidak_ada',
			word_count_mode: normalizeWordCountMode(m.word_count_mode),
			allow_embed: m.allow_embed === true,
			topik_terlarang: [...m.topik_terlarang],
			topikQuery: '',
			contoh_artikel_url: m.contoh_artikel_url ?? ''
		};
		formSnapshot = comparableForm(form);
		showForm = true;
		// Status "Kesimpulan Website" (endpoint Go tersedia) — sekali, best-effort.
		void (async () => {
			try {
				const res = await apiRequest(`/v1/partner/media/${m.id}/ai-summary`);
				if (res.ok && editingId === m.id) aiSummaryStatus = await res.json();
			} catch {
				/* pelacakan best-effort */
			}
		})();
	}

	function addFormCategory(name: string) {
		const v = name.trim();
		if (!v || form.categories.length >= 20) return;
		const lower = v.toLowerCase();
		if (form.categories.some((c) => c.trim().toLowerCase() === lower)) return;
		form.categories = [...form.categories, v];
		form.categoryQuery = '';
	}

	function removeFormCategory(name: string) {
		form.categories = form.categories.filter((c) => c !== name);
	}

	function toggleBahasa(lang: string) {
		form.bahasa_artikel = form.bahasa_artikel.includes(lang)
			? form.bahasa_artikel.filter((b) => b !== lang)
			: [...form.bahasa_artikel, lang];
	}

	function addFormTopik(name: string) {
		const v = name.trim();
		if (!v) return;
		const lower = v.toLowerCase();
		if (form.topik_terlarang.some((t) => t.trim().toLowerCase() === lower)) return;
		form.topik_terlarang = [...form.topik_terlarang, v];
		form.topikQuery = '';
	}

	function removeFormTopik(name: string) {
		form.topik_terlarang = form.topik_terlarang.filter((t) => t !== name);
	}

	/** Submit dialog: media yang berubah wajib lewat konfirmasi review dulu. */
	async function saveMedia(e: SubmitEvent) {
		e.preventDefault();
		if (editingId && isDirty) {
			editConfirmOpen = true;
			return;
		}
		await submitMedia();
	}

	async function submitMedia() {
		busy = true;
		dialogError = '';
		try {
			const cats = Array.from(new Set(form.categories.map((c) => c.trim()).filter(Boolean)));
			if (!form.name.trim()) {
				dialogError = tr('Nama media wajib diisi.');
				return;
			}
			if (!cats.length) {
				dialogError = tr('Tambahkan minimal 1 kategori media.');
				return;
			}
			if (!form.bahasa_artikel.length) {
				dialogError = tr('Pilih minimal 1 bahasa artikel.');
				return;
			}
			if (form.publication_mode === 'publish' && !integrasiConsent) {
				dialogError = tr('Centang persetujuan Ketentuan Kerja Sama Mitra terlebih dahulu sebelum menyimpan media integrasi.');
				return;
			}
			const payload: Record<string, unknown> = {
				name: form.name.trim(),
				media_type: form.media_type,
				categories: cats,
				category: cats[0] ?? null,
				website_url: form.website_url.trim() || null,
				adapter_url: form.adapter_url.trim() || null,
				price: Number(form.price) || 0,
				pricing_type: form.pricing_type,
				traffic_estimate: Number(form.traffic_estimate) || 0,
				max_links: Number(form.max_links),
				description: form.description.trim() || null,
				notes: form.notes.trim() || null,
				integration_type: form.integration_type,
				publication_mode: form.publication_mode,
				bahasa_artikel: form.bahasa_artikel.length ? form.bahasa_artikel : ['Indonesia'],
				label_sponsor: form.label_sponsor,
				word_count_mode: form.word_count_mode,
				allow_embed: form.allow_embed,
				topik_terlarang: form.topik_terlarang,
				contoh_artikel_url: form.contoh_artikel_url.trim() || null
			};
			const res = editingId
				? await apiRequest(`/v1/partner/media/${editingId}`, { method: 'PUT', body: JSON.stringify(payload) })
				: await apiRequest('/v1/partner/media', { method: 'POST', body: JSON.stringify(payload) });
			if (!res.ok) {
				dialogError = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			showForm = false;
			notice = editingId ? 'Media diperbarui.' : tr('Media diajukan untuk review admin.');
			await load();
		} finally {
			busy = false;
		}
	}

	async function archive(m: Media) {
		if (!confirm(`Arsipkan media "${m.name}"? Media yang diarsipkan tidak tampil di katalog klien.`)) return;
		const res = await apiRequest(`/v1/partner/media/${m.id}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await readApiError(res, "Permintaan gagal.");
			return;
		}
		notice = 'Media diarsipkan.';
		await load();
	}

	async function resubmit(m: Media) {
		const res = await apiRequest(`/v1/partner/media/${m.id}/submit`, { method: 'POST' });
		if (!res.ok) {
			error = await readApiError(res, "Permintaan gagal.");
			return;
		}
		notice = tr('Media diajukan ulang untuk review admin.');
		await load();
	}

	/** Pulihkan media terarsip (parity app lama: PUT status=pending_review). */
	async function restore(m: Media) {
		if (!confirm(`${tr('Pulihkan media')} "${m.name}"?`)) return;
		const res = await apiRequest(`/v1/partner/media/${m.id}`, {
			method: 'PUT',
			body: JSON.stringify({ status: 'pending_review' })
		});
		if (!res.ok) {
			error = await readApiError(res, "Permintaan gagal.");
			return;
		}
		notice = tr('Media dipulihkan dan menunggu review admin.');
		await load();
	}

	async function uploadImport(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		importError = '';
		importResult = null;
		importFile = file;
		const body = new FormData();
		body.append('file', file);
		body.append('confirm', 'false');
		const res = await apiRequest('/v1/partner/media/import', { method: 'POST', body });
		if (!res.ok) {
			importError = await readApiError(res, "Permintaan gagal.");
			importFile = null;
			input.value = '';
			return;
		}
		importResult = await res.json();
		input.value = '';
	}

	async function confirmImport() {
		if (!importResult) return;
		busy = true;
		importError = '';
		try {
			// Server mem-parse file aslinya (bukan baris hasil pratinjau), jadi
			// konfirmasi mengunggah ulang objek File yang disimpan saat pratinjau.
			const file = importFile;
			if (!file) {
				importError = tr('Pilih file XLSX lalu ulangi impor.');
				return;
			}
			const body = new FormData();
			body.append('file', file);
			body.append('confirm', 'true');
			const res = await apiRequest('/v1/partner/media/import', { method: 'POST', body });
			const data = await res.json();
			if (!res.ok) {
				importError = data?.detail ?? 'Impor gagal.';
				return;
			}
			importResult = data;
			importFile = null;
			notice = `${data.imported} media berhasil diimpor.`;
			await load();
		} finally {
			busy = false;
		}
	}

	const statusLabel: Record<string, string> = $derived({
		active: tr('Aktif'), inactive: tr('Nonaktif'), draft: 'Draf', archived: tr('Diarsipkan'),
		pending_review: tr('Menunggu review'), rejected: tr('Ditolak')
	});
	const fmt = (n: number) => (n ?? 0).toLocaleString('id-ID');
	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}`;
</script>

<PageTitle title={tr("Daftar Media")} />

<svelte:window onclick={closeMenu} onkeydown={(e) => { if (e.key === 'Escape') closeMenu(); }} />

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{tr("Daftar Media")}</h1>
			<p class="mt-1 text-sm text-muted-foreground">{items.length} {tr("media terdaftar pada akun mitra Anda.")}</p>
		</div>
		<div class="flex flex-wrap gap-2">
			{#if bulkUnlocked}
				<label class="app-btn app-btn-soft cursor-pointer">
					<Upload size={15} /> Impor XLSX
					<input id="bulk-file" class="hidden" type="file" accept=".xlsx" onchange={uploadImport} />
				</label>
			{:else}
				<button
					class="app-btn app-btn-soft"
					type="button"
					disabled
					title={tr('Impor massal terbuka setelah media pertama Anda disetujui admin dan domainnya terverifikasi.')}
				>
					<Lock size={15} /> Impor XLSX
				</button>
			{/if}
			<button class="app-btn app-btn-primary" onclick={openCreate}><Plus size={15} />{tr("Tambah Media")}</button>
		</div>
	</header>

	{#if !bulkUnlocked}
		<div class="app-card mt-3 flex items-start gap-3 px-4 py-3 text-sm">
			<ShieldCheck size={18} class="mt-0.5 shrink-0" style="color: var(--primary);" />
			<div>
				<p class="font-semibold">{tr('Impor massal belum terbuka')}</p>
				<ol class="mt-1 list-decimal space-y-0.5 pl-5 text-muted-foreground">
					<li>{tr('Klik “Verifikasi” pada baris media, lalu pasang kode verifikasi di situs Anda.')}</li>
					<li>{tr('Setelah domain pertama terverifikasi, Impor XLSX aktif dengan sendirinya.')}</li>
				</ol>
			</div>
		</div>
	{/if}

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	{#if importError}
		<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{importError}</div>
	{/if}

	{#if importResult}
		<div class="app-card mt-3 p-4">
			<div class="flex items-start justify-between gap-3">
				<div>
					<h2 class="text-sm font-semibold">
						{importResult.mode === 'preview' ? 'Pratinjau Impor' : 'Hasil Impor'}
					</h2>
					<p class="mt-0.5 text-xs text-muted-foreground">
						{importResult.total} baris · {importResult.validCount ?? importResult.imported ?? 0} valid
						{#if importResult.invalidCount}· {importResult.invalidCount} bermasalah{/if}
						{#if importResult.failed}· {importResult.failed} gagal{/if}
					</p>
				</div>
				<div class="flex gap-2">
					{#if importResult.mode === 'preview' && importResult.invalidCount === 0}
						<button class="app-btn app-btn-primary" disabled={busy} onclick={confirmImport}>Impor {importResult.validCount} media</button>
					{/if}
					<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label={tr("Tutup hasil impor")} onclick={() => (importResult = null)}><X size={16} /></button>
				</div>
			</div>

			{#if Array.isArray(importResult.rows)}
				<div class="mt-3 max-h-72 overflow-y-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="py-1.5 pr-3 font-medium">{tr('Baris')}</th>
								<th class="py-1.5 pr-3 font-medium">{tr("Nama")}</th>
								<th class="py-1.5 pr-3 font-medium">{tr('Kategori')}</th>
								<th class="py-1.5 pr-3 text-right font-medium">{tr("Harga")}</th>
								<th class="py-1.5 font-medium">{tr('Catatan')}</th>
							</tr>
						</thead>
						<tbody>
							{#each importResult.rows as r (r.rowNumber)}
								<tr class="border-t">
									<td class="py-1.5 pr-3 tabular-nums">{r.rowNumber}</td>
									<td class="py-1.5 pr-3">{r.parsed?.name ?? '—'}</td>
									<td class="py-1.5 pr-3">{r.parsed?.category ?? '—'}</td>
									<td class="py-1.5 pr-3 text-right tabular-nums">{rp(r.parsed?.price ?? 0)}</td>
									<td class="py-1.5">
										{#if r.errors?.length}
											<span style="color: var(--destructive);">{r.errors.join('; ')}</span>
										{:else if r.warnings?.length}
											<span class="text-muted-foreground">{r.warnings.join('; ')}</span>
										{:else}
											<span style="color: var(--success);">OK</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
			{#if Array.isArray(importResult.errors) && importResult.errors.length > 0}
				<ul class="mt-3 space-y-1 text-xs" style="color: var(--destructive);">
					{#each importResult.errors as e (e.rowNumber)}
						<li>Baris {e.rowNumber}: {e.detail}</li>
					{/each}
				</ul>
			{/if}
			{#if importResult.mode === 'preview'}
				<p class="mt-3 text-xs text-muted-foreground">{tr("Periksa baris di atas, lalu tekan Impor untuk menyimpan. Berkas yang sama dikirim ulang ke server saat konfirmasi.")}</p>
			{/if}
		</div>
	{/if}

	<form class="app-card mt-4 flex flex-wrap items-center gap-2 p-3" onsubmit={(e) => e.preventDefault()}>
		<label class="app-input flex min-w-[220px] flex-1 items-center gap-2 !py-1.5">
			<Search size={15} class="text-muted-foreground" />
			<input class="w-full bg-transparent text-sm outline-none" placeholder={tr("Cari nama media atau domain…")} bind:value={q} />
		</label>
		<Select
			bind:value={statusFilter}
			ariaLabel={tr("Status media")}
			placeholder={tr("Semua status (tanpa arsip)")}
			align="end"
			onchange={load}
			options={[
				{ value: '', label: tr('Semua status (tanpa arsip)') },
				{ value: 'active', label: tr('Aktif') },
				{ value: 'pending_review', label: tr('Menunggu review') },
				{ value: 'rejected', label: tr('Ditolak') },
				{ value: 'inactive', label: tr('Nonaktif') },
				{ value: 'archived', label: tr('Diarsipkan') }
			]}
		/>
		<Select
			bind:value={dewanFilter}
			ariaLabel={tr("Status Dewan Pers")}
			placeholder={tr("Semua status Dewan Pers")}
			align="end"
			onchange={load}
			options={[
				{ value: '', label: tr('Semua status Dewan Pers') },
				{ value: 'yes', label: tr('Terverifikasi Dewan Pers') },
				{ value: 'no', label: tr('Belum terverifikasi') }
			]}
		/>
	</form>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat media…")}</p>
	{:else if filtered.length === 0}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">
				{q || statusFilter ? tr('Tidak ada media yang cocok dengan filter ini.') : tr('Belum ada media. Tambahkan media pertama Anda.')}
			</p>
		</div>
	{:else}
		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">{tr("Media")}</th>
						<th class="px-4 py-2.5 font-medium">{tr('Domain')}</th>
						<th class="px-4 py-2.5 font-medium">{tr('Kategori')}</th>
						<th class="px-4 py-2.5 font-medium">
							<span class="inline-flex items-center gap-1">
								{tr("Mode Publikasi")}
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
						<th class="px-4 py-2.5 font-medium">{tr("Dewan Pers")}</th>
						<th class="px-4 py-2.5 font-medium">{tr('Harga')}</th>
						<th class="px-4 py-2.5 font-medium">{tr('Kualitas')}</th>
						<th class="px-4 py-2.5 font-medium">DA</th>
						<th class="px-4 py-2.5 font-medium">
							<span class="inline-flex items-center gap-1">
								Platform
								<Tooltip.Provider delayDuration={0}>
									<Tooltip.Root>
										<Tooltip.Trigger>
											{#snippet child({ props })}
												<button
													{...props}
													type="button"
													class="inline-flex cursor-help items-center text-muted-foreground"
													aria-label={platformColumnHint()}
												>
													<CircleAlert size={12} />
												</button>
											{/snippet}
										</Tooltip.Trigger>
										<Tooltip.Portal>
											<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
												<Tooltip.Arrow class="uiTipArrow" />
												{platformColumnHint()}
											</Tooltip.Content>
										</Tooltip.Portal>
									</Tooltip.Root>
								</Tooltip.Provider>
							</span>
						</th>
						<th class="px-4 py-2.5 font-medium">Status</th>
						<th class="px-4 py-2.5 text-right font-medium">{tr('Aksi')}</th>
					</tr>
				</thead>
				<tbody>
					{#each filtered as m (m.id)}
						<tr class="border-t align-top">
							<td class="px-4 py-2.5">
									<div class="flex flex-wrap items-center gap-1.5">
										<span class="font-medium">{m.name}</span>
									</div>
								{#if m.website_url}
									<a class="inline-flex items-center gap-1 text-xs" style="color: var(--primary);" href={m.website_url} target="_blank" rel="noreferrer">
										{m.website_url.replace(/^https?:\/\//, '').slice(0, 40)} <ExternalLink size={11} />
									</a>
								{/if}
								{#if m.contoh_artikel_url}
									<a class="mt-0.5 inline-flex items-center gap-1 text-xs" style="color: var(--primary);" href={m.contoh_artikel_url} target="_blank" rel="noreferrer">{tr('Contoh artikel')} <ExternalLink size={10} /></a>
								{:else}
									<button type="button" class="mt-0.5 inline-flex items-center gap-1 text-[11px]" style="color: var(--primary);" onclick={() => openContoh(m)}>
										<Plus size={11} /> {tr('Tambah contoh artikel')}
									</button>
								{/if}
							</td>
							<td class="px-4 py-2.5">
								{#if m.domain_verified}
									<span
										class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium"
										style="background: var(--accent); color: var(--accent-foreground);"
										title={tr('Kepemilikan domain terverifikasi')}
									>
										<ShieldCheck size={12} /> {tr('Terverifikasi')}
									</span>
								{:else}
									<button
										type="button"
										class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
										onclick={() => void openDomainVerify(m)}
									>
										<ShieldCheck size={12} /> {tr('Verifikasi')}
									</button>
								{/if}
							</td>
							<td class="px-4 py-2.5 text-muted-foreground">{m.categories.join(', ') || '—'}</td>
								<td class="px-4 py-2.5">
									<div class="flex flex-col items-start gap-1">
										{#if (m.publication_mode ?? 'draft') === 'publish'}
											<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--success-soft, #e8f7ee); color: var(--success);">{tr('Mode Integrasi')}</span>
										{:else}
											<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--warning-soft, #fdf3e3); color: var(--warning);">{tr('Mode Manual')}</span>
										{/if}
									{#if autoPublishEnabled}
										<div class="flex items-center gap-1">
											<label class="inline-flex cursor-pointer items-center gap-1.5 text-[11px] text-muted-foreground">
												<input
													type="checkbox"
													checked={m.auto_publish}
													disabled={autoBusy === m.id}
													onchange={(e) => void toggleAutoPublish(m, (e.currentTarget as HTMLInputElement).checked)}
													aria-label={`${tr('Mode otomatis untuk')} ${m.name}`}
												/>
												{tr('Mode Otomatis')}
											</label>
											<Tooltip.Provider delayDuration={0}>
												<Tooltip.Root>
													<Tooltip.Trigger>
														{#snippet child({ props })}
															<button
																{...props}
																type="button"
																class="inline-flex cursor-help items-center text-muted-foreground"
																aria-label={autoPublishHint()}
															>
																<CircleAlert size={12} />
															</button>
														{/snippet}
													</Tooltip.Trigger>
													<Tooltip.Portal>
														<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
															<Tooltip.Arrow class="uiTipArrow" />
															{autoPublishHint()}
														</Tooltip.Content>
													</Tooltip.Portal>
												</Tooltip.Root>
											</Tooltip.Provider>
										</div>
									{/if}
								</div>
							</td>
							<td class="px-4 py-2.5">
								{#if m.dewan_pers_status === 'verified'}
									<span
										class="inline-flex h-5 w-5 items-center justify-center rounded-full"
										style="background: var(--primary); color: var(--primary-foreground, #fff);"
										role="img"
										title={tr('Terverifikasi')}
										aria-label={tr('Terverifikasi')}
									>
										<Check size={13} />
									</span>
								{:else if m.dewan_pers_status === 'pending' || m.dewan_pers_status === 'rejected' || m.dewan_pers_status === 'conflict'}
									<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="border: 1px solid {dpBadge(m.dewan_pers_status).tone}; color: {dpBadge(m.dewan_pers_status).tone};">{dpBadge(m.dewan_pers_status).label}</span>
								{:else}
									<span
										class="inline-flex h-5 w-5 items-center justify-center rounded-full"
										style="background: #ff4c4c; color: var(--destructive-foreground, #fff);"
										role="img"
										title={tr('Tanpa Dewan Pers')}
										aria-label={tr('Tanpa Dewan Pers')}
									>
										<X size={13} />
									</span>
								{/if}
								{#if m.dewan_pers_status === 'rejected' && m.dewan_pers_rejection_reason}
									<span class="mt-1 block max-w-[180px] text-[11px]" style="color: var(--destructive);">{m.dewan_pers_rejection_reason}</span>
								{/if}
							</td>
							<td class="px-4 py-2.5 tabular-nums">{rp(m.price)}</td>
							<td class="px-4 py-2.5">
								{#if m.health_score != null}
									<span class="text-xs font-medium tabular-nums" title={JSON.stringify(m.health_breakdown ?? {})}>{m.health_score}</span>
								{:else}
									<span class="text-[11px] text-muted-foreground">{tr('Belum dihitung')}</span>
								{/if}
							</td>
							<td class="px-4 py-2.5">
								{#if m.seo_da != null}
									<span class="text-xs tabular-nums">{m.seo_da}</span>
								{:else}
									<span class="text-[11px] text-muted-foreground">—</span>
								{/if}
							</td>
							<td class="px-4 py-2.5">
								<span class="text-xs">{platLabel(m)}</span>
								{#if m.has_connection}
									<span class="mt-0.5 block text-[11px]" style="color: var(--success);">{tr('Koneksi siap')}</span>
								{/if}
								{#if (m.integration_type === 'blogger' && m.blogger_status !== 'connected') || (m.integration_type === 'custom_web' && m.custom_web_status !== 'connected')}
									<span class="mt-0.5 block text-[11px]" style="color: var(--destructive);">{tr('Belum terhubung')}</span>
								{/if}
							</td>
							<td class="px-4 py-2.5">
								<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">
									{statusLabel[m.status] ?? m.status}
								</span>
								{#if m.pending_publications > 0}
									<span class="ml-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--warning-soft, #fdf3e3); color: var(--warning);">
										{m.pending_publications} {tr('menunggu')}
									</span>
								{/if}
								{#if m.status === 'rejected' && m.rejection_reason}
									<span class="mt-1 block max-w-[200px] text-[11px]" style="color: var(--destructive);">{tr('Alasan')}: {m.rejection_reason}</span>
								{/if}
							</td>
							<td class="px-4 py-2.5 text-right">
								{#if m.status === 'archived'}
									<div class="flex items-center justify-end gap-2">
										<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--muted); color: var(--muted-foreground);">{tr('Terarsip')}</span>
										<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" onclick={() => restore(m)}><RotateCcw size={13} /> {tr('Pulihkan')}</button>
									</div>
								{:else}
									<div class="relative inline-block text-right">
										<button
											type="button"
											class="rounded-lg p-1.5 hover:bg-secondary"
											aria-label={`${tr('Aksi untuk')} ${m.name}`}
											aria-haspopup="menu"
											aria-expanded={openMenu === m.id}
											onclick={(e) => {
												e.stopPropagation();
												if (openMenu === m.id) { openMenu = null; } else { positionMenu(e.currentTarget as HTMLElement); openMenu = m.id; }
											}}
										>
											<MoreVertical size={16} />
										</button>
										{#if openMenu === m.id}
											<div
												class="fixed z-40 w-52 rounded-xl border py-1 shadow-lg"
												style="border-color: var(--border); background: var(--card, #fff); {menuPositionStyle}"
												role="menu"
												aria-label={`${tr('Menu aksi')} ${m.name}`}
											>
												{#if m.status === 'rejected'}
													<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={() => { closeMenu(); void resubmit(m); }}>
														<RotateCcw size={14} /> {tr('Ajukan Kembali')}
													</button>
												{/if}
												<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={() => { closeMenu(); openDewanPers(m); }}>
													<BadgeCheck size={14} /> {tr('Dewan Pers')}
												</button>
												<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={() => { closeMenu(); openEdit(m); }}>
													<Pencil size={14} /> Edit
												</button>
												{#if m.integration_type === 'tulisio_api'}
													<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={() => { closeMenu(); void testConnection(m); }}>
														<Plug size={14} /> {tr('Test Koneksi')}
													</button>
												{/if}
												<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" style="color: var(--destructive);" onclick={() => { closeMenu(); void archive(m); }}>
													<Trash2 size={14} /> {tr('Hapus')}
												</button>
											</div>
										{/if}
									</div>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		<div class="mt-3 flex flex-wrap items-center justify-between gap-2">
			<span class="text-xs text-muted-foreground">{tr('Menampilkan')} {items.length} {tr('dari')} {total} {tr('media')}</span>
			<div class="flex gap-2">
				<button class="app-btn app-btn-soft" type="button" disabled={pageNo <= 1} onclick={() => { pageNo = Math.max(1, pageNo - 1); }}>{tr('Sebelumnya')}</button>
				<button class="app-btn app-btn-soft" type="button" disabled={pageNo * PAGE_SIZE >= total} onclick={() => { pageNo = pageNo + 1; }}>{tr('Berikutnya')}</button>
			</div>
		</div>
	{/if}
</section>

{#if showForm}
	<div
		class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4"
		style="background-color: rgba(0,0,0,.45);"
		role="presentation"
		onclick={() => (showForm = false)}
		onkeydown={(e) => {
			// Escape dari dropdown Select sudah ditangani di sana (preventDefault)
			// — jangan ikut menutup dialog saat menutup dropdown.
			if (e.key === 'Escape' && !e.defaultPrevented) {
				editConfirmOpen = false;
				showForm = false;
			}
		}}
	>
		<dialog
			open
			class="app-card my-8 w-full max-w-[580px] p-5"
			aria-label={editingId ? tr('Edit media') : tr('Tambah media')}
			onclick={(e) => e.stopPropagation()}
		>
			<h2 class="text-lg font-semibold">{editingId ? tr('Edit media') : tr('Tambah media')}</h2>

			<form class="mt-4 flex flex-col gap-3.5" onsubmit={saveMedia}>
				{#if dialogError}
					<div class="flex items-start gap-2 rounded-lg p-3 text-sm" style="background: color-mix(in srgb, var(--destructive) 8%, transparent); color: var(--destructive);" role="alert">
						<TriangleAlert size={15} class="mt-0.5 shrink-0" />
						<span>{dialogError}</span>
					</div>
				{/if}

				<div class="flex flex-col gap-[14px]">
					<!-- A. Informasi Media -->
					<div class="rounded-[10px] border p-3" style="background: var(--muted);">
						<div class="mb-2 text-xs font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('A. Informasi Media')}</div>
						<div class="flex flex-col gap-3">
							<label class="block">
								<span class="mb-1 block text-xs font-bold text-muted-foreground">{tr('Nama media')}</span>
								<input class="app-input w-full" required maxlength="160" placeholder={tr('Contoh: Media Nasional Indonesia')} bind:value={form.name} />
							</label>
							<label class="block">
								<span class="mb-1 block text-xs font-bold text-muted-foreground">{tr('Website / Domain')}</span>
								<input class="app-input w-full" type="url" placeholder="https://medianasional.example" bind:value={form.website_url} />
							</label>
							<div>
								<span class="mb-1 block text-xs font-bold text-muted-foreground">{tr('Bahasa artikel')}</span>
								<div class="flex flex-wrap gap-4">
									{#each ['Indonesia', 'Inggris', 'China', 'Arab'] as lang (lang)}
										<label class="flex cursor-pointer items-center gap-2">
											<input type="checkbox" checked={form.bahasa_artikel.includes(lang)} onchange={() => toggleBahasa(lang)} />
											<span class="text-sm">{lang}</span>
										</label>
									{/each}
								</div>
								<span class="mt-1 block text-xs text-muted-foreground">{tr('Pilih satu atau beberapa bahasa yang digunakan media ini.')}</span>
							</div>
							<div>
								<span class="mb-1 block text-xs font-bold text-muted-foreground">{tr('Jumlah kata')}</span>
								<div class="[&_.zs-select-trigger]:w-full [&_.zs-select-trigger]:max-w-none">
									<Select
										bind:value={form.word_count_mode}
										ariaLabel={tr('Jumlah kata artikel')}
										minWidth={220}
										options={WORD_COUNT_MODES.map((m) => ({ value: m.value, label: tr(m.label) }))}
									/>
								</div>
								<span class="mt-1 block text-xs text-muted-foreground">{tr('Panjang artikel yang dibuat sistem menyesuaikan pilihan ini.')}</span>
							</div>
							<div>
								<span class="mb-1 block text-xs font-bold text-muted-foreground">{tr('Support Embed (Video/Maps)')}</span>
								<label class="inline-flex cursor-pointer items-center gap-2">
									<input type="checkbox" class="peer sr-only" aria-label={tr('Izinkan embed video atau peta')} bind:checked={form.allow_embed} />
									<span class="relative h-5 w-9 rounded-full bg-secondary transition-colors after:absolute after:left-0.5 after:top-0.5 after:h-4 after:w-4 after:rounded-full after:bg-white after:transition-transform after:content-[''] peer-checked:bg-primary peer-checked:after:translate-x-4"></span>
									<span class="text-sm text-muted-foreground">{form.allow_embed ? tr('Diizinkan') : tr('Tidak diizinkan')}</span>
								</label>
								<span class="mt-1 block text-xs text-muted-foreground">{tr('Bila diizinkan, artikel berita bisa menyematkan video/peta interaktif.')}</span>
							</div>
							<div>
								<span class="mb-1 block text-xs font-bold text-muted-foreground">{tr('Label sponsor')}</span>
								<div class="[&_.zs-select-trigger]:w-full [&_.zs-select-trigger]:max-w-none">
									<Select
										bind:value={form.label_sponsor}
										ariaLabel={tr('Label sponsor')}
										minWidth={220}
										options={[
											{ value: 'tidak_ada', label: tr('Tidak ada (tayang organik)') },
											{ value: 'ada_sponsor', label: tr('Ada tag sponsor') }
										]}
									/>
								</div>
							</div>
							<div>
								<span class="mb-1 block text-xs font-bold text-muted-foreground">{tr('Topik terlarang')}</span>
								{#if form.topik_terlarang.length > 0}
									<div class="mb-2 flex flex-wrap gap-1">
										{#each form.topik_terlarang as t (t)}
											<span class="inline-flex items-center gap-1.5 rounded px-2 py-0.5 text-xs font-medium" style="background: color-mix(in srgb, var(--destructive) 10%, var(--card)); color: var(--destructive);">
												{t}
												<button type="button" aria-label={`${tr('Hapus')} ${t}`} class="cursor-pointer leading-none" onclick={() => removeFormTopik(t)}>×</button>
											</span>
										{/each}
									</div>
								{/if}
								<div class="flex items-center gap-2">
									<input
										class="app-input min-w-0 flex-[1_1_200px]"
										placeholder={tr('Contoh: Politik, SARA, Judi…')}
										bind:value={form.topikQuery}
										onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addFormTopik(form.topikQuery); } }}
									/>
									<button type="button" class="app-btn app-btn-soft shrink-0 whitespace-nowrap" onclick={() => addFormTopik(form.topikQuery)}>＋ {tr('Tambah')}</button>
								</div>
								<span class="mt-1 block text-xs text-muted-foreground">{tr('Opsional. Topik yang TIDAK boleh dimuat oleh media ini.')}</span>
							</div>
						</div>
					</div>

					<!-- B. Jenis Media -->
					<div class="rounded-[10px] border p-3" style="background: var(--card);">
						<div class="mb-1 text-xs font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('B. Jenis Media')}</div>
						<div class="mb-2 text-xs text-muted-foreground">{tr('Pilih jenis media — bukan kategori. Kategori ada di bagian C.')}</div>
						<div class="flex flex-wrap gap-2">
							{#each [
								{ v: 'mainstream', label: 'Media Mainstream', desc: 'Media berita/publisher profesional.' },
								{ v: 'blogger', label: 'Media Blogger', desc: 'Blog personal / Blogger yang digunakan sebagai media publikasi.' }
							] as opt (opt.v)}
								<button
									type="button"
									class="flex-[1_1_200px] rounded-[10px] border px-3 py-2.5 text-left"
									style="border-color: {form.media_type === opt.v ? 'var(--primary)' : 'var(--border)'}; background: {form.media_type === opt.v ? 'var(--accent)' : 'var(--card)'}; cursor: pointer;"
									onclick={() => (form.media_type = opt.v)}
								>
									<span class="mb-1 flex items-center gap-2">
										<span class="flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full border" style="border-color: {form.media_type === opt.v ? 'var(--primary)' : 'var(--muted-foreground)'};">
											{#if form.media_type === opt.v}<span class="h-1.5 w-1.5 rounded-full" style="background: var(--primary);"></span>{/if}
										</span>
										<span class="text-sm font-medium">{tr(opt.label)}</span>
										{#if form.media_type === opt.v}
											<span class="rounded px-1.5 py-0.5 text-[10px] font-medium" style="background: color-mix(in srgb, var(--primary) 14%, var(--card)); color: var(--accent-foreground);">{tr('Pilihan')}</span>
										{/if}
									</span>
									<span class="block text-xs text-muted-foreground">{tr(opt.desc)}</span>
								</button>
							{/each}
						</div>
					</div>

					<!-- C. Kategori Media -->
					<div class="rounded-[10px] border p-3">
						<div class="mb-1 text-xs font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('C. Kategori Media')}</div>
						<div class="mb-2 text-xs text-muted-foreground">{tr('Tambahkan satu atau beberapa kategori yang tersedia di media ini. Kategori akan digunakan sebagai filter di Dashboard Klien.')}</div>
						{#if form.categories.length > 0}
							<div class="mb-2 flex flex-wrap gap-1">
								{#each form.categories as c (c)}
									<span class="inline-flex items-center gap-1.5 rounded px-2 py-0.5 text-xs font-medium" style="background: color-mix(in srgb, var(--primary) 12%, var(--card)); color: var(--primary);">
										{c}
										<button type="button" aria-label={`${tr('Hapus')} ${c}`} class="cursor-pointer leading-none" onclick={() => removeFormCategory(c)}>×</button>
									</span>
								{/each}
							</div>
						{/if}
						<div class="flex items-stretch gap-2">
							<input
								class="app-input min-w-0 flex-[1_1_220px]"
								placeholder={tr('Cari kategori…')}
								bind:value={form.categoryQuery}
								onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addFormCategory(form.categoryQuery); } }}
							/>
							<button type="button" class="app-btn app-btn-soft shrink-0 whitespace-nowrap" onclick={() => addFormCategory(form.categoryQuery)}>＋ {tr('Tambah kategori')}</button>
						</div>
						{#if form.categories.length >= 20}
							<div class="mt-2 text-xs" style="color: var(--warning);">{tr('Maksimal 20 kategori per media.')}</div>
						{:else if catSuggestions.length > 0 || (catQuery && !catPool.some((c) => c.toLowerCase() === catQuery))}
							<div class="mt-2 flex flex-wrap gap-1">
								{#each catSuggestions as c (c)}
									<button type="button" class="rounded-md border bg-card px-2 py-1 text-xs hover:bg-secondary" onclick={() => addFormCategory(c)}>＋ {c}</button>
								{/each}
								{#if catQuery && !catPool.some((c) => c.toLowerCase() === catQuery)}
									<button
										type="button"
										class="rounded-md px-2 py-1 text-xs font-medium disabled:cursor-not-allowed disabled:opacity-50"
										style="background: color-mix(in srgb, #8b5cf6 12%, transparent); color: #7c3aed;"
										disabled={form.categories.some((c) => c.trim().toLowerCase() === catQuery)}
										onclick={() => addFormCategory(form.categoryQuery.trim())}
									>＋ {tr('Buat kategori baru')} "{form.categoryQuery.trim()}"</button>
								{/if}
							</div>
						{/if}
					</div>

					<!-- Contoh Artikel -->
					<div class="rounded-[10px] border p-3" style="background: var(--muted);">
						<div class="mb-1 text-xs font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('Contoh Artikel')}</div>
						<div class="mb-2 text-xs text-muted-foreground">{tr('Tunjukkan satu artikel asli dari media Anda. Pembeli memakainya untuk melihat layout, ukuran font, peletakan gambar, dan penempatan backlink.')}</div>
						{#if form.contoh_artikel_url}
							<div class="flex flex-wrap items-center gap-2">
								<span class="inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium" style="background: var(--success-soft); color: var(--success);">
									<Check size={12} /> {tr('Tersedia')}
								</span>
								<span class="max-w-[320px] break-all text-sm">{form.contoh_artikel_url}</span>
								<button type="button" class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" onclick={openContohFromForm}><Pencil size={13} /> {tr('Ganti')}</button>
								<button type="button" class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" style="color: var(--destructive);" onclick={() => (form.contoh_artikel_url = '')}><X size={13} /> {tr('Hapus')}</button>
							</div>
						{:else}
							<button type="button" class="app-btn app-btn-soft" onclick={openContohFromForm}><Link size={14} /> {tr('Tambah contoh artikel')}</button>
						{/if}
						<div class="mt-2 text-xs text-muted-foreground">{tr('Pastikan contoh artikel yang Anda input, terbebas dari perjanjian NDA, Tauthub mendorong mitra media untuk menjaga kerasahasiaan kerjasama dengan klien.')}</div>
					</div>

					<!-- B. Platform & Integrasi -->
					<div class="rounded-[10px] border p-3">
						<div class="mb-2 text-xs font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('B. Platform & Integrasi')}</div>
						<div class="mb-2">
							<span class="mb-1 block text-xs font-bold text-muted-foreground">{tr('Mode')}</span>
							<div class="flex flex-col gap-2">
								{#each [
									{ v: 'draft', label: 'Mode Manual', helper: 'Mitra copy-paste artikel ke web-nya sendiri dan input URL publikasi di artikel klien. Status jadi Terbit otomatis saat URL dikirim.' },
									{ v: 'publish', label: 'Mode Integrasi', helper: 'Artikel di-push otomatis ke WordPress/Blogger/Custom Web. Status jadi Terbit otomatis setelah klik ACC.' }
								] as opt (opt.v)}
									<button
										type="button"
										class="flex items-start gap-2.5 rounded-[10px] border px-3 py-2.5 text-left"
										style="border-color: {form.publication_mode === opt.v ? 'var(--primary)' : 'var(--border)'}; background: {form.publication_mode === opt.v ? 'var(--accent)' : 'var(--card)'}; color: var(--foreground); cursor: pointer;"
										onclick={() => (form.publication_mode = opt.v)}
									>
										<span class="mt-1 flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full border" style="border-color: {form.publication_mode === opt.v ? 'var(--primary)' : 'var(--muted-foreground)'};">
											{#if form.publication_mode === opt.v}<span class="h-1.5 w-1.5 rounded-full" style="background: var(--primary);"></span>{/if}
										</span>
										<span class="min-w-0">
											<span class="block text-sm font-medium">{tr(opt.label)}</span>
											<span class="mt-1 block text-xs text-muted-foreground">{tr(opt.helper)}</span>
										</span>
									</button>
								{/each}
							</div>
						</div>
						{#if form.publication_mode === 'publish'}
							<div>
								<div class="mb-1 text-xs font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('Platform Tujuan')}</div>
								<div class="mb-2 flex flex-wrap items-center gap-2">
									<button type="button" class="app-btn !min-h-7 !px-2.5 !text-xs" class:app-btn-primary={form.integration_type === 'wordpress'} class:app-btn-soft={form.integration_type !== 'wordpress'} onclick={() => (form.integration_type = 'wordpress')}>WordPress</button>
									<button type="button" class="app-btn !min-h-7 !px-2.5 !text-xs" class:app-btn-primary={form.integration_type === 'blogger'} class:app-btn-soft={form.integration_type !== 'blogger'} onclick={() => (form.integration_type = 'blogger')}>Blogger</button>
									<button type="button" class="app-btn !min-h-7 !px-2.5 !text-xs" class:app-btn-primary={form.integration_type === 'custom_web'} class:app-btn-soft={form.integration_type !== 'custom_web'} onclick={() => (form.integration_type = 'custom_web')}>Custom Web</button>
									<span class="text-xs text-muted-foreground">
										{form.integration_type === 'wordpress' ? tr('Butuh koneksi WordPress') : form.integration_type === 'blogger' ? tr('Butuh koneksi Blogger') : tr('Butuh endpoint API')}
									</span>
								</div>
								{#if form.integration_type === 'wordpress'}
									<div class="flex items-start gap-2 rounded-lg p-3 text-sm" style="background: var(--accent); color: var(--accent-foreground);">
										<Info size={14} class="mt-0.5 shrink-0" />
										<span>{tr('Domain perlu di-ACC Admin terlebih dahulu, setelah disetujui silakan koneksikan di halaman')} <a class="underline" style="color: var(--accent-foreground);" href={`${base}/connections`}>{tr('WordPress')}</a>.</span>
									</div>
								{:else if form.integration_type === 'blogger'}
									<div class="flex items-start gap-2 rounded-lg p-3 text-sm" style="background: var(--accent); color: var(--accent-foreground);">
										<Info size={14} class="mt-0.5 shrink-0" />
										<span>{tr('Hubungkan akun Google dan pilih blog di halaman')} <a class="underline" style="color: var(--accent-foreground);" href={`${base}/blogger`}>{tr('Blogger')}</a> {tr('setelah media ini disetujui Admin.')}</span>
									</div>
								{:else if form.integration_type === 'custom_web'}
									<div class="flex items-start gap-2 rounded-lg p-3 text-sm" style="background: var(--warning-soft); color: var(--warning);">
										<TriangleAlert size={14} class="mt-0.5 shrink-0" />
										<span>{tr('Endpoint URL, API Key, dan Test Connection diatur di halaman')} <a class="underline" style="color: var(--warning);" href={`${base}/publishing-api`}>{tr('Web Custom')}</a> {tr('setelah media ini disetujui Admin.')}</span>
									</div>
								{:else}
									<div class="text-xs text-muted-foreground">{tr('Integrasi')} {form.integration_type} {tr('tidak memerlukan konfigurasi tambahan di sini.')}</div>
								{/if}
								<div class="mt-2 rounded-lg border p-3" style="background: var(--muted);">
									<label class="flex cursor-pointer items-start gap-2">
										<input type="checkbox" class="mt-0.5" checked={integrasiConsent} onchange={(e) => (integrasiConsent = (e.currentTarget as HTMLInputElement).checked)} />
										<span class="text-xs text-muted-foreground">
											{tr('Saya telah membaca dan menyetujui')}
											<a href="https://tauthub.com/kerja-sama-mitra/" target="_blank" rel="noreferrer" class="underline" style="color: var(--accent-foreground);">{tr('Ketentuan Kerja Sama Mitra')}</a>
											{tr('untuk mode integrasi. Wajib dicentang sebelum media dapat disimpan.')}
										</span>
									</label>
								</div>
							</div>
						{/if}
						<div class="mt-2">
							<span class="mb-1 block text-xs font-bold text-muted-foreground">{tr('Jumlah backlink yang diperbolehkan dalam artikel')}</span>
							<div class="[&_.zs-select-trigger]:w-full [&_.zs-select-trigger]:max-w-none">
								<Select
									bind:value={form.max_links}
									ariaLabel={tr('Jumlah backlink yang diperbolehkan dalam artikel')}
									minWidth={220}
									options={[
										{ value: '0', label: tr('0 — Tanpa backlink') },
										{ value: '1', label: tr('1 Link (default)') },
										{ value: '2', label: tr('2 Link') },
										{ value: '3', label: tr('3 Link') }
									]}
								/>
							</div>
							<span class="mt-1 block text-xs text-muted-foreground">{tr('Jumlah maksimal backlink yang boleh ditanam dalam satu artikel di media ini. Default 1; pilih 0 bila artikel tidak boleh memuat backlink.')}</span>
						</div>
					</div>

					<!-- C. Harga & Model Kerja Sama -->
					<div class="rounded-[10px] border p-3" style="background: var(--accent);">
						<div class="mb-2 text-xs font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('C. Harga & Model Kerja Sama')}</div>
						<div class="mb-2">
							<span class="mb-1 block text-xs font-bold text-muted-foreground">{tr('Harga Pengajuan (IDR)')}</span>
							<input
								class="app-input w-full"
								type="text"
								inputmode="numeric"
								placeholder="50.000"
								value={form.price ? Number(form.price).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' }) : ''}
								oninput={(e) => (form.price = (e.currentTarget as HTMLInputElement).value.replace(/\D/g, ''))}
							/>
						</div>
						<div class="rounded-lg border p-2.5" style="background: var(--card);">
							<div class="text-xs font-bold text-muted-foreground">{tr('Kesepakatan')}</div>
							<div class="mt-1 text-xs text-muted-foreground">{tr('Mitra mendapatkan 75% dari harga pengajuan per website yang didaftarkan.')}</div>
							<div class="mt-1 text-xs text-muted-foreground">
								{tr('Tauthub diijinkan untuk menetapkan atau menyesuaikan harga jual kepada pelanggan, baik menggunakan harga yang disepakati bersama mitra dalam hal ini')}
								<span class="font-semibold" style="color: var(--success);">{form.price ? `(Rp ${Number(form.price).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })})` : tr('(nominal dinamis)')}</span>
								{tr('atau nominal lainnya.')}.
							</div>
						</div>
					</div>

					<!-- Kesimpulan Website (hanya edit) -->
					{#if editingRow}
						<div class="rounded-[10px] border p-3" style="background: var(--muted);">
							<div class="mb-1 flex flex-wrap items-center justify-between gap-2">
								<span class="text-xs font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('Kesimpulan Website')}</span>
								{#if editingRow.ai_summary_generated_at}
									<span class="text-xs text-muted-foreground">{tr('Dibuat otomatis oleh AI')} · {dt(editingRow.ai_summary_generated_at)}</span>
								{/if}
							</div>
							{#if editingRow.ai_summary}
								<div class="whitespace-pre-wrap text-sm">{editingRow.ai_summary}</div>
							{:else}
								<div class="text-xs text-muted-foreground">{tr('Dibuat otomatis oleh AI setelah media melewati antrean global. Kesimpulan akan tampil di sini untuk dicek ulang, dan dipakai Taut Pintar saat mencocokkan kampanye.')}</div>
							{/if}
							{#if aiSummaryLine}
								<div class="mt-2 text-xs italic text-muted-foreground">{aiSummaryLine}</div>
							{/if}
						</div>
					{/if}
				</div>

				<div class="mt-1 flex justify-end gap-2">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (showForm = false)}>{tr('Batal')}</button>
					<button class="app-btn app-btn-primary" type="submit" disabled={busy || (form.publication_mode === 'publish' && !integrasiConsent)}>
						{busy ? tr('Menyimpan…') : editingId ? tr('Simpan perubahan') : tr('Tambah media')}
					</button>
				</div>
			</form>
		</dialog>
	</div>
{/if}

{#if editConfirmOpen}
	<div
		class="fixed inset-0 z-[60] flex items-start justify-center overflow-y-auto p-4"
		style="background-color: rgba(0,0,0,.45);"
		role="presentation"
		onclick={() => { if (!busy) editConfirmOpen = false; }}
		onkeydown={(e) => { if (e.key === 'Escape' && !busy) editConfirmOpen = false; }}
	>
		<dialog open class="app-card my-8 w-full max-w-[520px] p-5" aria-label={tr('Konfirmasi perubahan media')} onclick={(e) => e.stopPropagation()}>
			<h2 class="text-lg font-semibold">{tr('Konfirmasi perubahan media')}</h2>
			<p class="mt-2 text-sm text-muted-foreground">{tr('Perubahan ini akan memengaruhi visibilitas media Anda di sisi Klien.')}</p>
			<div class="mt-3 flex items-start gap-2 rounded-lg p-3 text-sm" style="background: var(--warning-soft); color: var(--warning);">
				<TriangleAlert size={16} class="mt-0.5 shrink-0" />
				<span>{tr('Setelah disimpan, media akan berstatus')} <strong>{tr('Menunggu Review Admin')}</strong>. {tr('Media tetap tersimpan, tetapi sementara tampil abu-abu dan tidak dapat dipilih Klien sampai Admin menyetujui perubahan.')}</span>
			</div>
			<div class="mt-4 flex justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => (editConfirmOpen = false)}>{tr('Batal')}</button>
				<button
					type="button"
					class="app-btn"
					style="background-color: var(--warning); color: #ffffff;"
					disabled={busy}
					onclick={() => { editConfirmOpen = false; void submitMedia(); }}
				>{busy ? tr('Menyimpan…') : tr('Simpan & Ajukan Review')}</button>
			</div>
		</dialog>
	</div>
{/if}

{#if domTarget}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={() => (domTarget = null)} onkeydown={(e) => { if (e.key === 'Escape') domTarget = null; }}>
		<dialog open class="app-card my-8 w-full max-w-xl p-5" aria-label={tr('Verifikasi Kepemilikan Domain')} onclick={(e) => e.stopPropagation()}>
			<div class="flex items-start justify-between gap-3">
				<h2 class="text-lg font-semibold">{tr('Verifikasi Kepemilikan Domain')}</h2>
				<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label={tr('Tutup')} onclick={() => (domTarget = null)}><X size={18} /></button>
			</div>
			<p class="mt-2 text-sm text-muted-foreground">
				{tr('Tempel meta tag berikut di bagian head halaman utama')}
				<strong>{domTarget.website_url ?? domTarget.name}</strong>, {tr('lalu klik Verifikasi Sekarang.')}
			</p>
			<pre class="mt-3 overflow-x-auto rounded-lg border p-3 text-xs" style="border-color: var(--border);"><code>{`<meta name="tauthub-verification" content="${domToken}" />`}</code></pre>
			<div class="mt-2 flex flex-wrap gap-2">
				<button class="app-btn app-btn-soft" type="button" disabled={!domToken} onclick={() => void copyText('meta', `<meta name="tauthub-verification" content="${domToken}" />`)}>
					<Copy size={13} /> {domCopied === 'meta' ? tr('Tersalin') : tr('Salin meta tag')}
				</button>
				<button class="app-btn app-btn-soft" type="button" disabled={!domToken} onclick={() => void copyText('token', domToken)}>
					<Copy size={13} /> {domCopied === 'token' ? tr('Tersalin') : tr('Salin token')}
				</button>
			</div>
			<div class="mt-3 rounded-lg border p-3 text-xs" style="border-color: var(--border);">
				<p class="font-medium">{tr('Alternatif: verifikasi lewat DNS TXT')}</p>
				<p class="mt-1 text-muted-foreground">{tr('Kalau sulit mengubah kode situs (atau situs diblokir WAF), tambahkan TXT record berikut di DNS domain utama tanpa www:')}</p>
				<pre class="mt-2 overflow-x-auto rounded-md p-2 text-[11px]" style="background: var(--secondary);"><code>{`_tauthub-verify   TXT   "${domToken}"`}</code></pre>
				<p class="mt-1 text-muted-foreground">{tr('Perubahan DNS bisa perlu waktu sampai 24 jam sebelum terbaca sistem.')}</p>
			</div>
			{#if domTarget.domain_verified}
				<p class="mt-3 text-sm" style="color: var(--success);">{tr('Domain sudah terverifikasi.')}</p>
			{:else}
				<div class="mt-4 flex flex-wrap items-center gap-3">
					<button class="app-btn app-btn-primary" type="button" disabled={domBusy || !domToken} onclick={() => void verifyDomainNow()}>
						{domBusy ? tr('Memeriksa…') : tr('Verifikasi Sekarang')}
					</button>
					{#if domResult}
						<p class="text-sm">
							{#if domResult.status === 'verified'}
								<span style="color: var(--success);">{tr('Berhasil, domain terverifikasi.')}</span>
							{:else if domResult.status === 'token_not_found'}
								<span style="color: var(--warning);">{tr('Meta tag belum ditemukan di halaman utama. Pastikan sudah diunggah lalu coba lagi.')}</span>
							{:else if domResult.status === 'token_mismatch'}
								<span style="color: var(--destructive);">{tr('Token terpasang berbeda')}{domResult.found_token ? `: ${domResult.found_token}` : ''}.</span>
							{:else}
								<span class="text-muted-foreground">{tr('Status verifikasi')}: {domResult.status}</span>
							{/if}
						</p>
					{/if}
				</div>
			{/if}
		</dialog>
	</div>
{/if}

{#if dpTarget}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={() => (dpTarget = null)} onkeydown={(e) => { if (e.key === 'Escape') dpTarget = null; }}>
		<dialog open class="app-card my-8 w-full max-w-lg p-5" aria-label={tr('Verifikasi Dewan Pers')} onclick={(e) => e.stopPropagation()}>
			<div class="flex items-start justify-between gap-3">
				<h2 class="text-lg font-semibold">{tr('Verifikasi Dewan Pers')}</h2>
				<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label={tr('Tutup')} onclick={() => (dpTarget = null)}><X size={18} /></button>
			</div>
			<p class="mt-2 text-sm text-muted-foreground">
				{tr('Ajukan verifikasi untuk')} <strong>{dpTarget.name}</strong>.
				{tr('Masukkan nomor sertifikat Dewan Pers dan unggah dokumen pendukung bila ada.')}
			</p>
			<form class="mt-4 flex flex-col gap-3" onsubmit={submitDewanPers}>
				<label class="flex flex-col gap-1 text-sm">
					<span class="text-xs font-medium text-muted-foreground">{tr('Nomor sertifikat *')}</span>
					<input class="app-input" required minlength="3" maxlength="120" bind:value={dpCert} />
				</label>
				<label class="flex flex-col gap-1 text-sm">
					<span class="text-xs font-medium text-muted-foreground">{tr('Dokumen sertifikat (PDF/JPG, opsional)')}</span>
					<input
						class="app-input"
						type="file"
						accept=".pdf,.jpg,.jpeg,.png"
						onchange={(e) => (dpDoc = (e.currentTarget as HTMLInputElement).files?.[0] ?? null)}
					/>
				</label>
				{#if error}<p class="text-sm" style="color: var(--destructive);">{error}</p>{/if}
				<div class="flex justify-end gap-2">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (dpTarget = null)}>{tr('Batal')}</button>
					<button class="app-btn app-btn-primary" type="submit" disabled={dpBusy || dpCert.trim().length < 3}>
						{dpBusy ? tr('Mengirim…') : tr('Ajukan Verifikasi')}
					</button>
				</div>
			</form>
		</dialog>
	</div>
{/if}

{#if contohTarget || contohFromForm}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={closeContoh} onkeydown={(e) => { if (e.key === 'Escape') closeContoh(); }}>
		<dialog open class="app-card my-8 w-full max-w-lg p-5" aria-label={tr('Contoh Artikel')} onclick={(e) => e.stopPropagation()}>
			<div class="flex items-start justify-between gap-3">
				<div>
					<h2 class="text-lg font-semibold">{tr('Contoh Artikel')}</h2>
					{#if contohTarget}<p class="mt-0.5 text-xs text-muted-foreground">{contohTarget.name}</p>{/if}
				</div>
				<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label={tr('Tutup')} onclick={closeContoh}><X size={18} /></button>
			</div>
			<p class="mt-2 text-sm text-muted-foreground">
				{tr('Tautan satu artikel karya Anda yang terbit di media ini. Memiliki contoh membuat media lebih dipercaya klien.')}
			</p>
			<form class="mt-4 flex flex-col gap-3" onsubmit={saveContoh}>
				<label class="flex flex-col gap-1 text-sm">
					<span class="text-xs font-medium text-muted-foreground">{tr('URL artikel *')}</span>
					<input class="app-input" required type="url" placeholder={tr("https://media.example/artikel-contoh")} bind:value={contohUrl} />
				</label>
				<div class="flex justify-end gap-2">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (contohTarget = null)}>{tr('Batal')}</button>
					<button class="app-btn app-btn-primary" type="submit" disabled={contohBusy}>{contohBusy ? tr('Menyimpan…') : tr('Simpan')}</button>
				</div>
			</form>
		</dialog>
	</div>
{/if}
