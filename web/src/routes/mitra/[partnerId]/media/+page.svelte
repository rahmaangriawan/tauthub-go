<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Daftar Media mitra — port section "Daftar Media" portal mitra:
	// daftar + filter status, tambah media, edit, arsip, dan impor massal XLSX
	// (pratinjau → konfirmasi, parity alur bulk import app lama).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Building2, Check, ChevronRight, CircleAlert, CircleDollarSign, Clock, Copy, ExternalLink, FileText, Info, Layers, Link, Link2, Lock, MoreVertical, Pencil, Plug, Plus, RotateCcw, Search, ShieldCheck, Tag, Trash2, TriangleAlert, Upload, Wallet, X, Zap } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import { Tooltip } from 'bits-ui';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import { mediaDomainLabel } from '$lib/media/domain-label';
	import { normalizeLinkDurationMonths } from '$lib/media/link-duration';
	import { WORD_COUNT_MODES, parseCustomWordCount, formatCustomWordCount } from '$lib/media/word-count';

	const partnerId = $derived(page.params.partnerId as string);
	const base = $derived(`/mitra/${getPartnerUrlId() || partnerId}`);

	type Media = {
		id: string; name: string; category: string | null; categories: string[];
		website_url: string | null; price: number; effective_price: number; client_price: number;
		markup_percent: number; status: string; integration_type: string;
		publication_mode: string; traffic_estimate: number; max_links: number | null;
		/** Masa berlaku tautan: 0 = permanen, >0 = bulan. */
		link_duration_months?: number | null;
		/** Izin subjudul: null = ikut tipe media. */
		allow_heading?: boolean | null;
		bahasa_artikel: string[]; label_sponsor: string; dewan_pers_verified: boolean;
		pending_publications: number; total_terbit: number; price_status: string;
		topik_terlarang: string[]; contoh_artikel_url: string | null;
		// Kolom pembanding dialog tambah/edit (semua dikirim mediaJSON server).
		media_type: string; description: string | null; adapter_url: string | null;
		notes: string | null; pricing_type: string; word_count_mode: string;
		allow_embed: boolean; ai_summary: string | null; ai_summary_generated_at: string | null;
		// Kolom tabel app lama (dipakai untuk badge & aksi).
		auto_publish: boolean; dewan_pers_status: string | null; dewan_pers_rejection_reason: string | null;
		// Kiriman Dewan Pers yang sudah tersimpan — dipakai untuk prefilled dialog verifikasi.
		dewan_pers_cert_no: string | null; dewan_pers_doc_key: string | null;
		dewan_pers_submitted_at: string | null; dewan_pers_reviewed_at: string | null;
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
	// Tri-state: `null` = belum diketahui, yaitu sebelum load pertama berhasil.
	// Tanpa ini, default `false` membuat banner "belum terbuka" sempat tampil
	// selama beberapa detik saat halaman masih memuat daftar media.
	let bulkUnlocked = $state<boolean | null>(null);
	// true hanya setelah request daftar media berhasil. Dipakai untuk menahan
	// teks jumlah media agar tidak sempat menampilkan "0 media" saat memuat.
	let mediaLoaded = $state(false);

	// Satu-satunya sumber kebenaran untuk UI "impor terkunci". Hanya bernilai
	// true kalau request daftar media SUDAH selesai DAN server menyatakan belum
	// ada domain terverifikasi. Status "belum diketahui" (null) atau error tidak
	// pernah dihitung sebagai terkunci, jadi banner tidak bisa muncul lebih awal.
	const bulkLocked = $derived(mediaLoaded && bulkUnlocked === false);
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
		traffic_estimate: number; max_links: string; link_duration_months: string; description: string; notes: string;
		allow_heading: string;
		integration_type: string; publication_mode: string; bahasa_artikel: string[];
		label_sponsor: string; word_count_mode: string; allow_embed: boolean;
		topik_terlarang: string[]; topikQuery: string; contoh_artikel_url: string;
	};
	/** Pilihan dropdown: mode bawaan atau 'custom' (rentang angka sendiri). */
	const WORD_COUNT_OPTIONS = [
		...WORD_COUNT_MODES.map((m) => ({ value: m.value as string, label: m.label })),
		{ value: 'custom', label: 'Custom…' }
	];
	/** Nilai tak dikenal dari DB → mode bawaan (parity lib/media/word-count). */
	const normalizeWordCountMode = (v: unknown) => {
		const raw = typeof v === 'string' ? v.trim() : '';
		return WORD_COUNT_MODES.some((m) => m.value === raw) || parseCustomWordCount(raw) ? raw : 'sesuai_permintaan';
	};
	/** Isi form menyimpan 'custom' sebagai pilihan; angka min/max di state terpisah. */
	const openWordCountMode = (v: unknown) => (parseCustomWordCount(v) ? 'custom' : normalizeWordCountMode(v));
	const emptyMediaForm = (): MediaForm => ({
		name: '', media_type: 'mainstream', categories: [], categoryQuery: '',
		website_url: '', adapter_url: '', price: '', pricing_type: 'nett',
		traffic_estimate: 0, max_links: '1', link_duration_months: '0', description: '', notes: '',
		allow_heading: 'default',
		integration_type: 'wordpress', publication_mode: 'draft',
		bahasa_artikel: ['Indonesia'], label_sponsor: 'tidak_ada',
		word_count_mode: 'sesuai_permintaan', allow_embed: false,
		topik_terlarang: [], topikQuery: '', contoh_artikel_url: ''
	});
	let showForm = $state(false);
	let editingId = $state<string | null>(null);
	let editingRow = $state<Media | null>(null);
	let form = $state<MediaForm>(emptyMediaForm());
	/** Rentang kata untuk pilihan "Custom…" di dropdown jumlah kata. */
	let customWordMin = $state('500');
	let customWordMax = $state('1000');
	// Masa berlaku tautan punya dua mode: Permanen (0 bulan) atau Jangka Waktu
	// (12/24/36 bulan). Pilihan bulannya baru muncul setelah mode Jangka Waktu
	// dipilih, jadi mitra tidak perlu membaca daftar angka panjang.
	const linkDurationPermanent = $derived(Number(form.link_duration_months) === 0);
	const linkDurationChoices = $derived.by(() => {
		const current = Number(form.link_duration_months);
		const base = [12, 24, 36];
		// Nilai lama di luar daftar (mis. 6 bulan dari data sebelumnya) tetap
		// ditampilkan supaya tidak berubah diam-diam saat disimpan.
		return current > 0 && !base.includes(current) ? [...base, current].sort((a, b) => a - b) : base;
	});
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
			f.pricing_type, f.traffic_estimate, f.max_links, f.link_duration_months, f.description, f.notes,
			f.integration_type, f.publication_mode, f.bahasa_artikel, f.label_sponsor,
			f.word_count_mode, customWordMin, customWordMax, f.allow_embed, f.topik_terlarang, f.contoh_artikel_url
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
	// Dialog Bulk Import: dibuka sebelum memilih berkas, memuat template
	// spreadsheet dari admin (/v1/partner/media/template-url) sebagai pedoman kolom.
	let showImportDialog = $state(false);
	let templateUrl = $state<string | null>(null);
	let templateUrlBusy = $state(false);
	let importDragOver = $state(false);
	// Popover kelola kategori (draf lokal dulu, disimpan saat klik Simpan).
	let openCatID = $state<string | null>(null);
	let catDraft = $state<string[]>([]);
	let catInput = $state('');
	let catBusy = $state(false);
	let catError = $state('');

	function openCats(m: Media) {
		// Fallback: media tanpa kategori modern (category lama utuh).
		catDraft = m.categories.length ? [...m.categories] : m.category ? [m.category] : [];
		catInput = '';
		catError = '';
		openPriceID = null;
		openCatID = m.id;
	}

	function removeCat(name: string) {
		catDraft = catDraft.filter((c) => c !== name);
	}

	function addCat() {
		const v = catInput.trim();
		if (!v) return;
		if (catDraft.length >= 20) {
			catError = tr('Maksimal 20 kategori per media.');
			return;
		}
		const lower = v.toLowerCase();
		if (catDraft.some((c) => c.trim().toLowerCase() === lower)) {
			catError = tr('Kategori sudah ada.');
			return;
		}
		catDraft = [...catDraft, v];
		catInput = '';
		catError = '';
	}

	async function saveCat(m: Media) {
		catBusy = true;
		catError = '';
		try {
			const cats = Array.from(new Set(catDraft.map((c) => c.trim()).filter(Boolean)));
			const res = await apiRequest(`/v1/partner/media/${m.id}`, {
				method: 'PUT',
				body: JSON.stringify({ categories: cats })
			});
			if (!res.ok) {
				catError = await readApiError(res, tr('Gagal menyimpan kategori.'));
				return;
			}
			notice = `${tr('Kategori diperbarui untuk')} ${m.name}.`;
			openCatID = null;
			await load();
		} finally {
			catBusy = false;
		}
	}

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
			// Gagal memuat: status verifikasi tetap "belum diketahui" (bukan
			// "belum terverifikasi"), supaya banner tidak muncul karena error.
			error = await readApiError(res, tr('Gagal memuat daftar media.'));
			return;
		}
		error = '';
		const data = await res.json();
		items = data.media ?? [];
		total = data.total ?? items.length;
		categories = data.categories ?? [];
		autoPublishEnabled = Boolean(data.auto_publish_enabled);
		// Baru di sini status verifikasi domain dianggap diketahui.
		bulkUnlocked = Boolean(data.bulk_unlocked);
		mediaLoaded = true;
		// Status pengajuan Secure Direct (non-blocking): dipakai tombol Ajuan.
		void loadSecureDirectStatus();
	}

	// ── Pengajuan Secure Direct (fitur terbit otomatis) ─────────────────────
	type SdStatus = 'none' | 'pending' | 'approved' | 'rejected';
	let sdOpen = $state(false);
	let sdStatus = $state<SdStatus>('none');
	let sdNote = $state('');
	let sdBusy = $state(false);
	let sdError = $state('');
	let sdNotice = $state('');

	async function loadSecureDirectStatus() {
		try {
			const res = await apiRequest('/v1/partner/secure-direct/request');
			if (!res.ok) return;
			const data = await res.json();
			const req = data?.request ?? null;
			sdStatus = req?.status === 'pending' || req?.status === 'approved' || req?.status === 'rejected' ? req.status : 'none';
			sdNote = typeof req?.note === 'string' ? req.note : '';
		} catch {
			// Status pengajuan opsional; kegagalan tidak boleh mengganggu daftar media.
		}
	}

	function openSecureDirectDialog() {
		sdError = '';
		sdNotice = '';
		sdOpen = true;
	}

	async function submitSecureDirect() {
		sdBusy = true;
		sdError = '';
		try {
			const res = await apiRequest('/v1/partner/secure-direct/request', { method: 'POST' });
			if (!res.ok) {
				sdError = await readApiError(res, tr('Gagal mengirim pengajuan.'));
				return;
			}
			sdStatus = 'pending';
			sdNotice = tr('Pengajuan terkirim. Admin Tauthub akan meninjau dan memberi tahu kamu setelah diaktifkan.');
			await loadSecureDirectStatus();
		} finally {
			sdBusy = false;
		}
	}

	/** Toggle "Secure Direct" per media (parity app lama: PUT {auto_publish}). */
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
	const mediaHost = (u: string | null) => (u ?? '').replace(/^https?:\/\//, '').replace(/\/+$/, '');

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

	/** Tooltip header kolom Mode Integrasi: siapa yang menuntaskan penerbitan. */
	const modeColumnHint = () =>
		tr('Siapa yang menerbitkan artikel. API = di-push otomatis ke situs mitra lewat WordPress/Blogger/Custom/API key; Manual = mitra menempel artikelnya sendiri lalu mengirim URL artikelnya.');

	/** Tooltip header kolom Platform: platform yang dipakai media. */
	const platformColumnHint = () => tr('Platform yang digunakan media, WordPress, Blogger, Emdash, API');

	/** Tooltip centang Secure Direct: apa akibatnya dan kapan dipakai. */
	const autoPublishHint = () =>
		tr('Kalau dicentang, artikel dari klien langsung diproses dan terbit ke situs media tanpa menunggu persetujuan mitra. Kalau tidak dicentang, mitra harus menyetujui dulu setiap artikel.');

	/** Tooltip lencana "N menunggu": artikel klien yang butuh konfirmasi mitra. */
	const pendingHint = () =>
		tr('Ada artikel dari klien yang menunggu konfirmasi Anda. Buka menu Publikasi untuk menyetujui atau menolaknya.');

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

	// ── Verifikasi kepemilikan (domain + Dewan Pers dalam SATU dialog 2 tab) ─
	let domTarget = $state<Media | null>(null);
	let verifTab = $state<'domain' | 'dewan'>('domain');
	let domToken = $state('');
	let domBusy = $state(false);
	let domResult = $state<{ status: string; found_token?: string | null } | null>(null);
	let domCopied = $state<string | null>(null);

	// ── Harga kolom tabel media = pendapatan bersih (potongan platform),
	//    sama seperti halaman Publikasi; klik membuka rincian. ────────────
	let openPriceID = $state<string | null>(null);
	// Potongan platform 25% (mitra menerima 75% dari harga pengajuan) — parity
	// brandflow pePlatformCutRate = 0.25 yang berlaku untuk earning baru.
	const PLATFORM_CUT_RATE = 0.25;
	const PARTNER_SHARE_PCT = Math.round((1 - PLATFORM_CUT_RATE) * 100);
	const PLATFORM_CUT_PCT = Math.round(PLATFORM_CUT_RATE * 100);
	const netPrice = (v: number) => Math.max(Math.round((Number(v) || 0) * (1 - PLATFORM_CUT_RATE)), 0);
	const priceCut = (v: number) => Math.max((Number(v) || 0) - netPrice(v), 0);
	const formPriceNumber = $derived(Math.max(Number(form.price) || 0, 0));
	const formPartnerIncome = $derived(netPrice(formPriceNumber));
	const formPlatformCut = $derived(priceCut(formPriceNumber));

	/** Ringkasan status verifikasi satu media — tooltip wrapper sel tabel. */
	function verifHint(m: Media) {
		const dewan = m.dewan_pers_status === 'verified'
			? tr('Dewan Pers: terverifikasi')
			: m.dewan_pers_status === 'pending' || m.dewan_pers_status === 'conflict' || m.dewan_pers_status === 'rejected'
				? `${tr('Dewan Pers')}: ${dpBadge(m.dewan_pers_status).label}`
				: tr('Dewan Pers: belum diajukan');
		return `${m.domain_verified ? tr('Domain: terverifikasi') : tr('Domain: belum terverifikasi')} · ${dewan}`;
	}

	async function loadDomToken(m: Media) {
		domToken = '';
		domBusy = false;
		domResult = null;
		const res = await apiRequest(`/v1/partner/media/${m.id}/domain-verification`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat token verifikasi domain.'));
			domTarget = null;
			return;
		}
		domToken = (await res.json()).token ?? '';
	}

	async function openDomainVerify(m: Media) {
		domTarget = m;
		verifTab = 'domain';
		await loadDomToken(m);
	}

	/** Klik wrapper sel: buka tab verifikasi yang masih kurang dahulu. */
	async function openVerification(m: Media) {
		domTarget = m;
		if (!m.domain_verified) {
			verifTab = 'domain';
			await loadDomToken(m);
			return;
		}
		verifTab = 'dewan';
		resetDewanForm(m);
	}

	async function switchVerifTab(t: 'domain' | 'dewan') {
		verifTab = t;
		if (t === 'domain' && domTarget && !domToken) await loadDomToken(domTarget);
		if (t === 'dewan' && domTarget) {
			// Form dewan mulai dari kiriman yang sudah tersimpan supaya mitra tetap
			// melihat nomor sertifikat & status pengajuan yang pernah diajukannya.
			resetDewanForm(domTarget);
		}
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
		let confirmDeleteContoh = $state(false);

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

	// ── Pengajuan verifikasi Dewan Pers (tab kedua dialog verifikasi) ─────
	let dpCert = $state('');
	let dpDoc = $state<File | null>(null);
	let dpBusy = $state(false);
	// Box unggah dokumen: status tarik & lepas + pratinjau gambar + validasi
	// klien (paritas server handler dewan-pers-document: PDF/JPG/PNG, maks 10 MB).
	let dpDocDragOver = $state(false);
	let dpDocError = $state('');
	let dpDocUrl = $state<string | null>(null);
	// Konfirmasi dua langkah untuk mencabut pengajuan (tanpa dialog tambahan).
	let dpConfirmDelete = $state(false);

	function clearDpDocUrl() {
		if (dpDocUrl) URL.revokeObjectURL(dpDocUrl);
		dpDocUrl = null;
	}

	const DP_DOC_OK = /\.(pdf|jpe?g|png)$/i;
	const DP_DOC_MAX = 10 * 1024 * 1024;

	/** Validasi klien supaya pengguna tidak menunggu penolakan dari server. */
	function pickDpDoc(file: File | null) {
		dpDocError = '';
		if (!file) return;
		if (!DP_DOC_OK.test(file.name)) dpDocError = tr('Format dokumen harus PDF, JPG, atau PNG.');
		else if (file.size > DP_DOC_MAX) dpDocError = tr('Ukuran file maksimal 10 MB.');
		else {
			clearDpDocUrl();
			dpDoc = file;
			dpDocUrl = file.type.startsWith('image/') ? URL.createObjectURL(file) : null;
		}
	}

	const fmtBytes = (n: number) =>
		n >= 1024 * 1024
			? `${(n / (1024 * 1024)).toFixed(1).replace('.', ',')} MB`
			: `${Math.max(1, Math.round(n / 1024))} KB`;

	/** Form dewan mulai dari kiriman yang sudah tersimpan: mitra tetap melihat
	 * nomor sertifikat yang pernah diajukan. Pilihan berkas baru dikosongkan. */
	function resetDewanForm(m: Media) {
		dpCert = m.dewan_pers_cert_no ?? '';
		dpDoc = null;
		dpDocDragOver = false;
		dpDocError = '';
		clearDpDocUrl();
		dpBusy = false;
		dpConfirmDelete = false;
	}

	/** Status pengajuan yang boleh dicabut mitra (bukan yang sudah terverifikasi). */
	const dpCanDelete = $derived(
		Boolean(domTarget) && ['pending', 'rejected', 'conflict'].includes(domTarget?.dewan_pers_status ?? '')
	);

	async function deleteDewanPers() {
		if (!domTarget || dpBusy) return;
		if (!dpConfirmDelete) {
			dpConfirmDelete = true;
			return;
		}
		dpConfirmDelete = false;
		dpBusy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/media/${domTarget.id}/dewan-pers-verification`, { method: 'DELETE' });
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menghapus pengajuan Dewan Pers.'));
				return;
			}
			notice = tr('Pengajuan Dewan Pers dihapus. Kamu bisa mengajukan ulang kapan saja.');
			domTarget = null;
			await load();
		} finally {
			dpBusy = false;
		}
	}

	async function submitDewanPers(e: SubmitEvent) {
		e.preventDefault();
		if (!domTarget) return;
		dpBusy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/media/${domTarget.id}/dewan-pers-verification`, {
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
				const up = await apiRequest(`/v1/partner/media/${domTarget.id}/dewan-pers-document`, { method: 'POST', body: form });
				if (!up.ok) {
					error = await readApiError(up, tr('Sertifikat tersimpan, tetapi unggah dokumen gagal.'));
					await load();
					return;
				}
			}
			notice = tr('Pengajuan verifikasi Dewan Pers terkirim dan menunggu review admin.');
			domTarget = null;
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
			confirmDeleteContoh = false;
			aiSummaryStatus = null;
			form = emptyMediaForm();
			customWordMin = '500';
			customWordMax = '1000';
			showForm = true;
		}

		function openEdit(m: Media) {
			editingId = m.id;
			editingRow = m;
			dialogError = '';
			confirmDeleteContoh = false;
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
			link_duration_months: String(normalizeLinkDurationMonths(m.link_duration_months)),
			description: m.description ?? '',
			notes: m.notes ?? '',
			integration_type: m.integration_type || 'manual',
			publication_mode: m.publication_mode === 'publish' ? 'publish' : 'draft',
			bahasa_artikel: m.bahasa_artikel.length ? [...m.bahasa_artikel] : ['Indonesia'],
			label_sponsor: m.label_sponsor === 'ada_sponsor' ? 'ada_sponsor' : 'tidak_ada',
			word_count_mode: openWordCountMode(m.word_count_mode),
			allow_embed: m.allow_embed === true,
			allow_heading:
				m.allow_heading === true ? 'yes' : m.allow_heading === false ? 'no' : 'default',
			topik_terlarang: [...m.topik_terlarang],
			topikQuery: '',
			contoh_artikel_url: m.contoh_artikel_url ?? ''
		};
		const custom = parseCustomWordCount(m.word_count_mode);
		customWordMin = String(custom?.min ?? 500);
		customWordMax = String(custom?.max ?? 1000);
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
			let wordCountMode = form.word_count_mode;
			if (wordCountMode === 'custom') {
				const min = Math.floor(Number(customWordMin));
				const max = Math.floor(Number(customWordMax));
				if (!Number.isFinite(min) || !Number.isFinite(max) || min < 50 || max > 50000) {
					dialogError = tr('Rentang jumlah kata harus antara 50 dan 50.000 kata.');
					return;
				}
				if (max < min) {
					dialogError = tr('Jumlah kata maksimal tidak boleh lebih kecil dari minimal.');
					return;
				}
				wordCountMode = formatCustomWordCount(min, max);
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
			link_duration_months: Number(form.link_duration_months),
				description: form.description.trim() || null,
				notes: form.notes.trim() || null,
				integration_type: form.integration_type,
				publication_mode: form.publication_mode,
				bahasa_artikel: form.bahasa_artikel.length ? form.bahasa_artikel : ['Indonesia'],
				label_sponsor: form.label_sponsor,
				word_count_mode: wordCountMode,
				allow_embed: form.allow_embed,
				// 'default' = jangan kirim, biarkan NULL (ikut tipe media).
				...(form.allow_heading === 'default'
					? {}
					: { allow_heading: form.allow_heading === 'yes' }),
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

	/** Buka dialog Bulk Import; template spreadsheet dimuat sekali per pembukaan. */
	async function openImportDialog() {
		showImportDialog = true;
		importError = '';
		if (templateUrl !== null) return;
		templateUrlBusy = true;
		try {
			const res = await apiRequest('/v1/partner/media/template-url');
			if (res.ok) {
				const data = (await res.json()) as { url?: string | null };
				templateUrl = data.url ?? '';
			}
		} catch {
			templateUrl = '';
		} finally {
			templateUrlBusy = false;
		}
	}

	async function uploadImport(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0] ?? null;
		// Kosongkan input supaya berkas yang sama bisa dipilih ulang setelah gagal.
		input.value = '';
		await startImport(file);
	}

	/** Titik masuk bersama untuk pilih-berkas dan tarik & lepas. */
	async function startImport(file: File | null) {
		if (!file) return;
		showImportDialog = false;
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
			return;
		}
		importResult = await res.json();
	}

	function dropImport(e: DragEvent) {
		e.preventDefault();
		importDragOver = false;
		void startImport(e.dataTransfer?.files?.[0] ?? null);
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

<!-- Kesimpulan Website dipakai di dua tempat: kolom kiri (desktop) dan bagian
     paling bawah form (HP). -->
{#snippet summarySection()}
	<!-- 4. Kesimpulan Website (hanya edit) -->
	{#if editingRow}
		<section class="rounded-2xl border border-slate-200/80 bg-white p-5 space-y-2.5 shadow-sm">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<h3 class="text-xs font-bold uppercase tracking-wider text-muted-foreground">{tr('Kesimpulan Website')}</h3>
				{#if editingRow.ai_summary_generated_at}
					<span class="text-[11px] text-muted-foreground font-medium">{tr('Dibuat otomatis oleh AI')} · {dt(editingRow.ai_summary_generated_at)}</span>
				{/if}
			</div>
			{#if editingRow.ai_summary}
				<div class="whitespace-pre-wrap text-xs sm:text-sm text-slate-700 leading-relaxed bg-slate-50/60 p-3.5 rounded-xl border border-slate-100">{editingRow.ai_summary}</div>
			{:else}
				<div class="text-xs text-muted-foreground leading-relaxed">{tr('Dibuat otomatis oleh AI setelah media melewati antrean global. Kesimpulan akan tampil di sini untuk dicek ulang, dan dipakai Taut Pintar saat mencocokkan kampanye.')}</div>
			{/if}
			{#if aiSummaryLine}
				<div class="text-xs italic text-muted-foreground pt-1">{aiSummaryLine}</div>
			{/if}
		</section>
	{/if}
{/snippet}


<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{tr("Daftar Media")}</h1>
			{#if mediaLoaded}
				<p class="mt-1 text-sm text-muted-foreground">{items.length} {tr("media terdaftar pada akun mitra Anda.")}</p>
			{/if}
		</div>
		<div class="flex flex-wrap gap-2">
			{#if bulkUnlocked === true}
				<button class="app-btn app-btn-soft" type="button" onclick={() => void openImportDialog()}>
					<Upload size={15} /> Impor XLSX
				</button>
			{:else if bulkLocked}
				<button
					class="app-btn app-btn-soft"
					type="button"
					disabled
					title={tr('Impor massal terbuka setelah media pertama Anda disetujui admin dan domainnya terverifikasi.')}
				>
					<Lock size={15} /> Impor XLSX
				</button>
			{:else}
				<!-- Status verifikasi belum diketahui (load pertama belum selesai):
					placeholder tanpa ikon kunci supaya tidak terlihat seperti "terkunci". -->
				<button class="app-btn app-btn-soft" type="button" disabled aria-busy="true">
					<Upload size={15} /> Impor XLSX
				</button>
			{/if}
			{#if mediaLoaded && !autoPublishEnabled}
				<button
					class="app-btn app-btn-soft"
					type="button"
					onclick={openSecureDirectDialog}
					title={sdStatus === 'pending' ? tr('Pengajuan Secure Direct kamu sedang menunggu persetujuan admin Tauthub.') : tr('Ajukan pemakaian mode terbit otomatis untuk akun mitra kamu.')}
				>
					<Zap size={15} /> {tr('Ajukan Secure Direct')}
					{#if sdStatus === 'pending'}<span class="ml-1 inline-flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[10px] font-bold leading-none" style="background: var(--warning-soft); color: var(--warning);">●</span>{/if}
				</button>
			{/if}
			<button class="app-btn app-btn-primary" onclick={openCreate}><Plus size={15} />{tr("Tambah Media")}</button>
		</div>
	</header>

	{#if bulkLocked}
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

	{#if showImportDialog}
		<div
			class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4"
			style="background-color: rgba(0,0,0,.45);"
			role="presentation"
			onclick={() => (showImportDialog = false)}
			onkeydown={(e) => { if (e.key === 'Escape') showImportDialog = false; }}
		>
			<dialog open class="app-card my-8 w-full max-w-lg p-5" aria-label={tr('Impor Media Massal')} onclick={(e) => e.stopPropagation()}>
				<div class="flex items-start justify-between gap-3">
					<div>
						<h2 class="text-lg font-semibold">{tr('Impor Media Massal')}</h2>
						<p class="mt-0.5 text-xs text-muted-foreground">{tr('Tambahkan beberapa media sekaligus dari satu berkas XLSX.')}</p>
					</div>
					<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label={tr('Tutup')} onclick={() => (showImportDialog = false)}><X size={18} /></button>
				</div>

				<ol class="mt-4 list-decimal space-y-1 pl-5 text-sm text-muted-foreground">
					<li>{tr('Unduh template spreadsheet di bawah, lalu isi satu baris untuk tiap media.')}</li>
					<li>{tr('Simpan sebagai XLSX (Google Spreadsheet: File → Download → Microsoft Excel).')}</li>
					<li>{tr('Unggah berkasnya di sini; Tauthub menampilkan pratinjau sebelum data disimpan.')}</li>
				</ol>

				<div class="mt-4 rounded-[10px] border p-3" style="border-color: var(--border); background: var(--muted);">
					{#if templateUrlBusy}
						<p class="text-sm text-muted-foreground">{tr('Memuat template…')}</p>
					{:else if templateUrl}
						<a class="app-btn app-btn-soft" href={templateUrl} target="_blank" rel="noreferrer">
							<ExternalLink size={14} /> {tr('Buka Template Spreadsheet')}
						</a>
					{:else}
						<p class="text-sm text-muted-foreground">{tr('Template spreadsheet belum diatur admin. Minta admin mengisi Template URL di Pengaturan Media.')}</p>
					{/if}
				</div>

				{#if importError}
					<p class="mt-3 text-sm" style="color: var(--destructive);">{importError}</p>
				{/if}

				<label
					class="mt-4 flex cursor-pointer flex-col items-center justify-center gap-1.5 rounded-xl border border-dashed px-6 py-8 text-center focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-[var(--primary)]"
					style="border-color: {importDragOver ? 'var(--primary)' : 'var(--border)'}; background: {importDragOver ? 'var(--accent)' : 'transparent'};"
					ondragover={(e) => { e.preventDefault(); importDragOver = true; }}
					ondragleave={() => (importDragOver = false)}
					ondrop={dropImport}
				>
					<Upload size={22} class="text-muted-foreground" />
					<span class="text-sm font-medium">{tr('Tarik & lepas berkas XLSX di sini')}</span>
					<span class="text-xs text-muted-foreground">{tr('atau klik untuk memilih berkas')}</span>
					<input class="sr-only" type="file" accept=".xlsx" onchange={uploadImport} />
				</label>
			</dialog>
		</div>
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

	<!-- Dialog pengajuan Secure Direct -->
	{#if sdOpen}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
		style="background-color: rgba(0,0,0,.45);"
		role="presentation"
		tabindex="-1"
		onclick={(e) => { if (e.target === e.currentTarget) sdOpen = false; }}
		onkeydown={(e) => { if (e.key === 'Escape') sdOpen = false; }}
	>
		<dialog
			open
			class="app-card my-auto w-full max-w-md overflow-hidden p-0"
			aria-label={tr('Ajukan Secure Direct')}
			aria-modal="true"
			onclick={(e) => e.stopPropagation()}
		>
			<!-- Kepala: ikon, judul, dan satu kalimat penjelas. -->
			<header class="flex items-start gap-3 border-b px-5 py-4" style="border-color: var(--border);">
				<span
					class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl"
					style="background: var(--accent); color: var(--primary); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary) 20%, transparent);"
				>
					<Zap size={18} />
				</span>
				<div class="min-w-0 flex-1">
					<h2 class="m-0 text-[15px] font-semibold tracking-[-0.01em]">{tr('Ajukan Secure Direct')}</h2>
					<p class="m-0 mt-0.5 text-[12.5px] leading-relaxed text-muted-foreground">
						{tr('Mode terbit otomatis untuk media Anda.')}
					</p>
				</div>
				<button
					class="shrink-0 rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
					type="button"
					aria-label={tr('Tutup')}
					onclick={() => (sdOpen = false)}
				><X size={18} /></button>
			</header>

			<div class="flex flex-col gap-3 px-5 py-4">
				{#if sdStatus === 'pending'}
					<div class="flex items-start gap-3 rounded-xl px-3.5 py-3.5" style="background-color: var(--warning-soft); color: var(--warning);">
						<Clock size={16} class="mt-0.5 shrink-0" />
						<div class="min-w-0">
							<p class="m-0 text-[13px] font-semibold">{tr('Menunggu persetujuan')}</p>
							<p class="m-0 mt-1 text-[12.5px] leading-relaxed">
								{tr('Pengajuan kamu sedang ditinjau admin Tauthub. Kamu akan menerima notifikasi di aplikasi begitu mode ini diaktifkan.')}
							</p>
						</div>
					</div>
				{:else}
					{#if sdStatus === 'rejected'}
						<div class="flex items-start gap-2.5 rounded-xl px-3.5 py-3 text-[12.5px] leading-relaxed" style="background-color: color-mix(in srgb, var(--destructive) 10%, transparent); color: var(--destructive);">
							<CircleAlert size={15} class="mt-0.5 shrink-0" />
							<span>
								{tr('Pengajuan kamu sebelumnya ditolak.')}{sdNote ? ` ${sdNote}` : ''} {tr('Kamu bisa mengajukan ulang di sini.')}
							</span>
						</div>
					{/if}

					<!-- Ringkasan cara kerja, bukan paragraf berjalan. -->
					<div class="rounded-xl p-3.5" style="background-color: var(--secondary);">
						<p class="m-0 text-[13px] font-semibold text-foreground">{tr('Cara kerjanya:')}</p>
						<ul class="m-0 mt-2 flex list-none flex-col gap-2 p-0 text-[12.5px] leading-relaxed text-muted-foreground">
							<li class="flex items-start gap-2">
								<Check size={14} class="mt-0.5 shrink-0" style="color: var(--success);" />
								<span>{tr('Centang “Secure Direct” pada media pilihan, artikel klien langsung diproses tanpa menunggu persetujuan kamu per artikel.')}</span>
							</li>
							<li class="flex items-start gap-2">
								<Check size={14} class="mt-0.5 shrink-0" style="color: var(--success);" />
								<span>{tr('Setiap pengaktifan direview admin Tauthub terlebih dahulu, untuk menjaga mutu distribusi.')}</span>
							</li>
						</ul>
					</div>

					{#if sdError}
						<p class="m-0 text-[12.5px]" style="color: var(--destructive);">{sdError}</p>
					{/if}
				{/if}

				{#if sdNotice}
					<div class="flex items-start gap-2.5 rounded-xl px-3.5 py-3 text-[12.5px] leading-relaxed" style="background-color: var(--success-soft); color: var(--success);">
						<Check size={15} class="mt-0.5 shrink-0" />
						<span>{sdNotice}</span>
					</div>
				{/if}
			</div>

			<!-- Kaki: aksi dipisah dari isi supaya jelas mana yang menutup, mana yang mengirim. -->
			<footer class="flex items-center justify-end gap-2 border-t px-5 py-3.5" style="border-color: var(--border);">
				<button class="app-btn app-btn-soft !px-4" type="button" onclick={() => (sdOpen = false)}>
					{sdStatus === 'pending' ? tr('Tutup') : tr('Batal')}
				</button>
				{#if sdStatus !== 'pending'}
					<button class="app-btn app-btn-primary !px-4" type="button" disabled={sdBusy} onclick={() => void submitSecureDirect()}>
						{sdBusy ? tr('Mengirim…') : tr('Kirim pengajuan')}
					</button>
				{/if}
			</footer>
		</dialog>
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
		<!-- Desktop: tabel asli, tidak diubah (≥768px). -->
		<div class="app-card mt-4 hidden overflow-x-auto md:block">
			<table class="w-full text-sm">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">{tr("Media")}</th>
						<th class="px-4 py-2.5 font-medium">{tr('Verifikasi Kepemilikan')}</th>
						<th class="px-4 py-2.5 font-medium">{tr('Kategori')}</th>
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
						<th class="px-4 py-2.5 font-medium">{tr('Menunggu')}</th>
						<th class="px-4 py-2.5 text-right font-medium">{tr('Aksi')}</th>
					</tr>
				</thead>
				<tbody>
					{#each filtered as m (m.id)}
						<tr class="border-t align-top">
							<td class="px-4 py-2.5 text-left">
								<div class="flex flex-wrap items-center gap-1.5">
										{#if m.website_url}
											<a class="font-medium hover:underline" href={m.website_url} target="_blank" rel="noreferrer">
												{mediaDomainLabel(m.website_url, m.name)}
											</a>
										{:else}
										<span class="font-medium">{m.name}</span>
									{/if}
								</div>
								{@render contohLink(m)}
							</td>
							<td class="px-4 py-2.5">
								{@render verifCell(m)}
							</td>
							<td class="px-4 py-2.5">
								{@render categoryCell(m)}
							</td>
								<td class="px-4 py-2.5">
									<div class="flex flex-col items-start gap-1">
										{@render modeBadge(m)}
										{@render autoPublish(m, true)}
									</div>
								</td>
							<td class="px-4 py-2.5 tabular-nums">
								{@render priceCell(m)}
							</td>
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
								{@render connectionStatus(m)}
							</td>
							<td class="px-4 py-2.5">
								{@render statusBadges(m)}
								{@render statusRejection(m)}
							</td>
							<td class="px-4 py-2.5">
								{@render pendingBadge(m)}
							</td>
							<td class="px-4 py-2.5 text-right">
								{@render rowActions(m)}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Mobile: setiap media menjadi kartu vertikal (<768px). Data sama dengan
			tabel (variabel & snippet bersama), hanya representasinya yang berbeda.
			Grid memakai repeat(2, minmax(0,1fr)) + min-w-0 supaya kolom menyusut
			aman tanpa teks terpecah per karakter. -->
		<div class="mt-4 flex flex-col gap-2.5 md:hidden">
			{#each filtered as m (m.id)}
				<article class="app-card p-3.5">
					<!-- A. Header: domain + status, kategori, contoh artikel, menu aksi -->
					<div class="flex items-start justify-between gap-2">
						<div class="min-w-0">
							<div class="flex flex-wrap items-center gap-x-2 gap-y-1">
								{#if m.website_url}
									<a class="break-words text-[15px] font-semibold leading-snug hover:underline" href={m.website_url} target="_blank" rel="noreferrer">{mediaDomainLabel(m.website_url, m.name)}</a>
								{:else}
									<span class="break-words text-[15px] font-semibold leading-snug">{m.name}</span>
								{/if}
								{@render statusBadges(m)}
								{@render pendingBadge(m)}
							</div>
							{#if m.categories.length}
								<div class="mt-1.5 flex flex-wrap gap-1">
									{#each m.categories as c (c)}
										<span class="rounded px-1.5 py-0.5 text-[10px] font-medium" style="background: var(--muted); color: var(--muted-foreground);">{c}</span>
									{/each}
								</div>
							{/if}
							{@render statusRejection(m)}
							<div class="mt-1.5 flex min-w-0 items-center justify-start">
								{@render contohLink(m)}
							</div>
						</div>
						{@render rowActions(m)}
					</div>

					<!-- B. Informasi utama -->
					<dl class="mt-3.5 grid grid-cols-2 gap-x-3 gap-y-3">
						<div class="min-w-0">
							<dt class="text-[11px] text-muted-foreground">{tr('Harga')}</dt>
							<dd class="mt-0.5 text-[13px] font-semibold tabular-nums">{@render priceCell(m)}</dd>
						</div>
						<div class="min-w-0">
							<dt class="text-[11px] text-muted-foreground">{tr('Kualitas')}</dt>
							<dd class="mt-0.5 text-[13px] font-semibold tabular-nums">
								{#if m.health_score != null}
									<span title={JSON.stringify(m.health_breakdown ?? {})}>{m.health_score}</span>
								{:else}
									<span class="text-[11px] font-normal text-muted-foreground">{tr('Belum dihitung')}</span>
								{/if}
							</dd>
						</div>
						<div class="min-w-0">
							<dt class="text-[11px] text-muted-foreground">DA</dt>
							<dd class="mt-0.5 text-[13px] font-semibold tabular-nums">{#if m.seo_da != null}{m.seo_da}{:else}<span class="font-normal text-muted-foreground">—</span>{/if}</dd>
						</div>
						<div class="min-w-0">
							<dt class="text-[11px] text-muted-foreground">Platform</dt>
							<dd class="mt-0.5 text-[13px] font-semibold">{platLabel(m)}</dd>
							<div class="font-normal">{@render connectionStatus(m)}</div>
						</div>
					</dl>

					<!-- C. Informasi pengelolaan media -->
					<div class="mt-3.5 border-t pt-3.5" style="border-color: var(--border);">
						<dl class="grid grid-cols-2 gap-x-3 gap-y-3">
							<div class="min-w-0">
								<dt class="text-[11px] text-muted-foreground">{tr('Verifikasi Kepemilikan')}</dt>
								<dd class="mt-1">{@render verifCell(m)}</dd>
							</div>
							<div class="min-w-0">
								<dt class="text-[11px] text-muted-foreground">{tr('Mode Integrasi')}</dt>
								<dd class="mt-1">{@render modeBadge(m)}</dd>
							</div>
							{#if autoPublishEnabled}
								<div class="min-w-0 self-center">
									{@render autoPublish(m, true)}
								</div>
							{/if}
						</dl>
					</div>
				</article>
			{/each}
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

{#snippet contohLink(m: Media)}
	{#if m.contoh_artikel_url}
		<a class="mt-0.5 inline-flex w-fit items-center justify-start gap-1 text-left text-xs" style="color: var(--primary);" href={m.contoh_artikel_url} target="_blank" rel="noreferrer">{tr('Contoh artikel')} <ExternalLink size={10} /></a>
	{:else}
		<button type="button" class="media-contoh mt-0.5 inline-flex w-fit items-center justify-start gap-1 text-left text-[11px]" style="color: var(--primary);" onclick={() => openContoh(m)}>
			<Plus size={11} /> {tr('Tambah contoh artikel')}
		</button>
	{/if}
{/snippet}

{#snippet domainVerify(m: Media)}
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
			class="media-verify app-btn app-btn-soft !px-2.5 !text-xs"
			onclick={() => void openDomainVerify(m)}
		>
			<ShieldCheck size={12} /> {tr('Verifikasi')}
		</button>
	{/if}
{/snippet}

{#snippet modeBadge(m: Media)}
	<!-- Badge menunjukkan JENIS integrasinya: 'publish' = jalur API situs mitra
	(WordPress/Blogger/Custom/sites API key), sisanya mitra menempel sendiri. -->
	{#if (m.publication_mode ?? 'draft') === 'publish'}
		<span class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--success-soft, #e8f7ee); color: var(--success);">{tr('API')}</span>
	{:else}
		<span class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--warning-soft, #fdf3e3); color: var(--warning);">{tr('Manual')}</span>
	{/if}
{/snippet}

{#snippet autoPublish(m: Media, showLabel: boolean)}
	{#if autoPublishEnabled}
		<div class="flex items-center gap-1">
			<label class="media-auto-label inline-flex cursor-pointer items-center gap-1.5 text-[11px] text-muted-foreground">
				<input
					type="checkbox"
					class="media-auto-check"
					checked={m.auto_publish}
					disabled={autoBusy === m.id}
					onchange={(e) => void toggleAutoPublish(m, (e.currentTarget as HTMLInputElement).checked)}
					aria-label={`${tr('Secure Direct untuk')} ${m.name}`}
				/>
				{#if showLabel}{tr('Secure Direct')}{/if}
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
{/snippet}

<!-- Harga media: nominal bersih; klik membuka rincian harga diajukan ·
     potongan platform · pendapatan. Pola sama dengan halaman Publikasi. -->
{#snippet priceCell(m: Media)}
	{@const net = netPrice(m.price)}
	{@const cut = priceCut(m.price)}
	<div class="relative flex justify-start">
		<button
			class="inline-flex items-center gap-1 text-left tabular-nums underline-offset-2 hover:underline"
			aria-haspopup="dialog"
			aria-expanded={openPriceID === m.id}
			title={tr('Lihat rincian penghasilan')}
			onclick={(e) => { e.stopPropagation(); openPriceID = openPriceID === m.id ? null : m.id; }}
		>
			{rp(net)}
			<svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>
		</button>
		{#if openPriceID === m.id}
			<div
				class="fixed inset-0 z-40"
				role="presentation"
				onclick={(e) => { e.stopPropagation(); openPriceID = null; }}
			></div>
			<div
				class="app-card absolute left-0 top-full z-50 mt-1 w-64 p-3 text-left shadow-lg"
				style="border: 1px solid var(--border);"
				role="dialog"
				tabindex="-1"
				aria-label={tr('Rincian penghasilan')}
				onclick={(e) => e.stopPropagation()}
				onkeydown={(e) => { if (e.key === 'Escape') openPriceID = null; }}
			>
				<p class="mb-2 text-[11px] font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('Rincian penghasilan')}</p>
				<dl class="space-y-1.5 text-[13px] tabular-nums">
					<div class="flex items-center justify-between gap-3">
						<dt class="text-muted-foreground">{tr('Harga diajukan')}</dt>
						<dd>{rp(m.price)}</dd>
					</div>
					<div class="flex items-center justify-between gap-3" style="color: var(--destructive, #d92d20);">
						<dt>{tr('Potongan platform')} (25%)</dt>
						<dd>−{rp(cut)}</dd>
					</div>
					<div class="flex items-center justify-between gap-3 border-t pt-1.5 font-semibold" style="border-color: var(--border); color: var(--success);">
						<dt>{tr('Pendapatan Anda')}</dt>
						<dd>{rp(net)}</dd>
					</div>
				</dl>
		</div>
	{/if}
	</div>
{/snippet}

<!-- Kategori media: klik membuka popover kelola kategori (tambah/hapus/ganti),
     disimpan per media lewat PUT {categories}. Pola cocok dengan sel Harga. -->
{#snippet categoryCell(m: Media)}
	<div class="relative flex justify-start">
		<button
			class="inline-flex max-w-[150px] items-center gap-1 truncate text-left text-[13px] text-muted-foreground underline-offset-2 hover:underline"
			aria-haspopup="dialog"
			aria-expanded={openCatID === m.id}
			title={tr('Kelola kategori')}
			onclick={(e) => { e.stopPropagation(); if (openCatID === m.id) { openCatID = null; } else { openCats(m); } }}
		>
			<span class="truncate">{m.categories.join(', ') || '—'}</span>
			<Pencil size={11} class="shrink-0" />
		</button>
		{#if openCatID === m.id}
			<div
				class="fixed inset-0 z-40"
				role="presentation"
				onclick={(e) => { e.stopPropagation(); openCatID = null; }}
			></div>
			<div
				class="app-card absolute left-0 top-full z-50 mt-1 w-64 p-3 text-left shadow-lg"
				style="border: 1px solid var(--border);"
				role="dialog"
				tabindex="-1"
				aria-label={tr('Kelola Kategori')}
				onclick={(e) => e.stopPropagation()}
				onkeydown={(e) => { if (e.key === 'Escape') openCatID = null; }}
			>
				<p class="mb-2 text-[11px] font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('Kelola Kategori')}</p>
				<div class="flex flex-wrap gap-1.5">
					{#each catDraft as c (c)}
						<span class="inline-flex items-center gap-1 rounded-lg border px-2 py-1 text-xs" style="border-color: var(--border); color: var(--foreground);">
							{c}
							<button
								type="button"
								class="inline-flex items-center justify-center rounded p-0.5 hover:bg-secondary"
								aria-label={`${tr('Hapus kategori')} ${c}`}
								onclick={() => removeCat(c)}
							>
								<X size={11} />
							</button>
						</span>
					{:else}
						<span class="text-[12.5px] text-muted-foreground">{tr('Belum ada kategori.')}</span>
					{/each}
				</div>
				<div class="mt-2.5 flex items-center gap-1.5">
					<input
						class="app-input min-w-0 flex-1 !py-1.5 !text-xs"
						bind:value={catInput}
						list="media-category-options"
						placeholder={tr('Tambah atau ganti kategori…')}
						onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addCat(); } }}
					/>
					<button type="button" class="app-btn app-btn-soft !min-h-8 !px-2.5 !text-xs" onclick={addCat}>{tr('Tambah')}</button>
				</div>
				<datalist id="media-category-options">
					{#each categories as c (c)}
						<option value={c}></option>
					{/each}
				</datalist>
				{#if catError}
					<p class="mt-2 text-[12px]" style="color: var(--destructive);">{catError}</p>
				{/if}
				<div class="mt-3 flex items-center justify-between gap-2 border-t pt-2.5" style="border-color: var(--border);">
					<span class="text-[11px] text-muted-foreground">{catDraft.length}/20</span>
					<div class="flex items-center gap-2">
						<button type="button" class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" onclick={() => (openCatID = null)}>{tr('Batal')}</button>
						<button type="button" class="app-btn app-btn-primary !min-h-7 !px-2.5 !text-xs" disabled={catBusy || catDraft.length === 0} onclick={() => void saveCat(m)}>
							{catBusy ? tr('Menyimpan…') : tr('Simpan')}
						</button>
					</div>
				</div>
			</div>
		{/if}
	</div>
{/snippet}

{#snippet verifCell(m: Media)}
	<!-- IKON SAJA (pemilik produk): interaksi lewat wrapper — klik membuka
	     dialog 2 tab di bagian yang belum selesai; hover = ringkasan status. -->
	<Tooltip.Provider delayDuration={0}>
		<Tooltip.Root>
			<Tooltip.Trigger>
				{#snippet child({ props })}
					<button
						{...props}
						type="button"
						class="inline-flex items-center gap-1.5 rounded-lg py-0.5"
						aria-label={verifHint(m)}
						title={verifHint(m)}
						onclick={() => void openVerification(m)}
					>
						<!-- Domain: badge shield-check biru (pembeda dari cek Dewan Pers);
							garis putus kuning = perlu verifikasi. -->
						{#if m.domain_verified}
							<span
								class="inline-flex h-5 w-5 items-center justify-center rounded-full"
								style="background: var(--accent); color: var(--accent-foreground);"
							>
								<ShieldCheck size={12} />
							</span>
						{:else}
							<span
								class="inline-flex h-5 w-5 items-center justify-center rounded-full border border-dashed"
								style="border-color: var(--warning); color: var(--warning);"
							>
								<ShieldCheck size={11} />
							</span>
						{/if}
						<!-- Dewan Pers: hijau = terverifikasi; kuning = diajukan/ditolak; merah = belum. -->
						{#if m.dewan_pers_status === 'verified'}
							<span
								class="inline-flex h-5 w-5 items-center justify-center rounded-full"
								style="background: var(--primary); color: var(--primary-foreground, #fff);"
							>
								<Check size={12} />
							</span>
						{:else if m.dewan_pers_status === 'pending' || m.dewan_pers_status === 'rejected' || m.dewan_pers_status === 'conflict'}
							<span
								class="inline-flex h-5 w-5 items-center justify-center rounded-full border"
								style="border-color: {dpBadge(m.dewan_pers_status).tone}; color: {dpBadge(m.dewan_pers_status).tone};"
							>
								<CircleAlert size={11} />
							</span>
						{:else}
							<span
								class="inline-flex h-5 w-5 items-center justify-center rounded-full"
								style="background: color-mix(in srgb, var(--destructive) 14%, transparent); color: var(--destructive);"
							>
								<X size={12} />
							</span>
						{/if}
					</button>
				{/snippet}
			</Tooltip.Trigger>
			<Tooltip.Portal>
				<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
					<Tooltip.Arrow class="uiTipArrow" />
					{verifHint(m)}
				</Tooltip.Content>
			</Tooltip.Portal>
		</Tooltip.Root>
	</Tooltip.Provider>
{/snippet}

{#snippet dewanPers(m: Media)}
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
		<span class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium" style="border: 1px solid {dpBadge(m.dewan_pers_status).tone}; color: {dpBadge(m.dewan_pers_status).tone};">{dpBadge(m.dewan_pers_status).label}</span>
		{:else}
			<span
				class="inline-flex h-5 w-5 items-center justify-center rounded-full"
				style="background: color-mix(in srgb, var(--destructive) 14%, transparent); color: var(--destructive);"
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
{/snippet}

{#snippet connectionStatus(m: Media)}
	{#if (m.integration_type === 'blogger' && m.blogger_status !== 'connected') || (m.integration_type === 'custom_web' && m.custom_web_status !== 'connected')}
		<span class="mt-0.5 block text-[11px]" style="color: var(--destructive);">{tr('Belum terhubung')}</span>
	{/if}
{/snippet}

{#snippet statusBadges(m: Media)}
	<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">
		{statusLabel[m.status] ?? m.status}
	</span>
{/snippet}

{#snippet pendingBadge(m: Media)}
	{#if m.pending_publications > 0}
		<Tooltip.Provider delayDuration={150}>
			<Tooltip.Root>
				<Tooltip.Trigger>
					{#snippet child({ props })}
						<!-- Klik lencana = ke publikasi media ini, tab Menunggu. -->
						<a
							{...props}
							href={`${base}/publications?media=${encodeURIComponent(m.id)}&tab=waiting`}
							class="rounded-full px-2 py-0.5 text-[11px] font-medium underline-offset-2 hover:underline"
							style="border: 0; background-color: var(--warning-soft, #fdf3e3); color: var(--warning);"
						>
							{m.pending_publications} {tr('menunggu')}
						</a>
					{/snippet}
				</Tooltip.Trigger>
				<Tooltip.Portal>
					<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
						<Tooltip.Arrow class="uiTipArrow" />
						{pendingHint()}
					</Tooltip.Content>
				</Tooltip.Portal>
			</Tooltip.Root>
		</Tooltip.Provider>
	{:else}
		<span class="text-[11px] text-muted-foreground">0 {tr('menunggu')}</span>
	{/if}
{/snippet}

{#snippet statusRejection(m: Media)}
	{#if m.status === 'rejected' && m.rejection_reason}
		<span class="mt-1 block max-w-[200px] text-[11px]" style="color: var(--destructive);">{tr('Alasan')}: {m.rejection_reason}</span>
	{/if}
{/snippet}

{#snippet rowActions(m: Media)}
	{#if m.status === 'archived'}
		<div class="flex items-center justify-end gap-2">
			<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--muted); color: var(--muted-foreground);">{tr('Terarsip')}</span>
			<button class="media-restore app-btn app-btn-soft !px-2.5 !text-xs" onclick={() => restore(m)}><RotateCcw size={13} /> {tr('Pulihkan')}</button>
		</div>
	{:else}
		<div class="relative inline-block text-right">
			<button
				type="button"
				class="media-menu-trigger inline-flex items-center justify-center rounded-lg p-1.5 hover:bg-secondary"
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
					class="media-row-menu fixed z-40 w-52 rounded-xl border py-1 shadow-lg"
					style="border-color: var(--border); background: var(--card, #fff); {menuPositionStyle}"
					role="menu"
					aria-label={`${tr('Menu aksi')} ${m.name}`}
				>
					{#if m.status === 'rejected'}
						<button type="button" role="menuitem" class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-secondary" onclick={() => { closeMenu(); void resubmit(m); }}>
							<RotateCcw size={14} /> {tr('Ajukan Kembali')}
						</button>
					{/if}
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
{/snippet}

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
				class="app-card my-auto flex max-h-[92vh] w-full max-w-[95vw] lg:max-w-[1500px] flex-col overflow-hidden rounded-2xl border border-slate-200/80 p-0 bg-white shadow-2xl"
				aria-label={editingId ? tr('Edit media') : tr('Tambah media')}
				onclick={(e) => e.stopPropagation()}
			>
				<!-- ── Header Tetap ── -->
				<header class="flex flex-none items-center justify-between border-b border-slate-100 bg-white px-6 py-4">
					<div class="flex items-center gap-3 min-w-0">
						<div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
							<Building2 size={20} />
						</div>
						<div class="min-w-0">
							<h2 class="text-lg font-bold text-slate-900 truncate tracking-tight">{editingId ? tr('Edit media') : tr('Tambah media')}</h2>
							<p class="text-xs text-muted-foreground truncate">{editingId ? tr('Kelola informasi, kategori, dan pengaturan kerja sama untuk media ini.') : tr('Daftarkan media baru dan atur kerja sama untuk media ini.')}</p>
						</div>
					</div>
					<button
						type="button"
						class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl text-slate-400 hover:bg-slate-100 hover:text-slate-700 transition-colors"
						onclick={() => (showForm = false)}
						aria-label={tr('Tutup')}
					>
						<X size={18} />
					</button>
				</header>

				<!-- ── Form Body Scrollable ── -->
				<form class="flex min-h-0 flex-1 flex-col overflow-hidden" onsubmit={saveMedia}>
					<div class="custom-scrollbar min-h-0 flex-1 overflow-y-auto p-6 bg-slate-50/40">
						{#if dialogError}
							<div class="mb-5 flex items-start gap-2.5 rounded-xl border border-rose-200 bg-rose-50 p-3.5 text-xs text-rose-700" role="alert">
								<TriangleAlert size={15} class="mt-0.5 shrink-0" />
								<span>{dialogError}</span>
							</div>
						{/if}

						<div class="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
							<!-- ════════ KOLOM KIRI (7 cols ~58%) ════════ -->
							<div class="lg:col-span-7 space-y-5">
								<!-- 1. Informasi media -->
								<section class="rounded-2xl border border-slate-200/80 bg-white p-5 space-y-4 shadow-sm">
									<div class="flex items-center gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
											<FileText size={18} />
										</div>
										<div>
											<h3 class="text-sm font-bold text-slate-900">{tr('Informasi media')}</h3>
											<p class="text-xs text-muted-foreground">{tr('Informasi dasar media yang digunakan untuk publikasi artikel.')}</p>
										</div>
									</div>

									<div class="grid grid-cols-1 sm:grid-cols-2 gap-3.5 pt-1">
										<label class="block">
											<span class="mb-1 block text-xs font-semibold text-slate-700">{tr('Nama media')} <span class="text-destructive">*</span></span>
											<input class="app-input w-full text-sm" required maxlength="160" placeholder={tr('Contoh: Media Nasional Indonesia')} bind:value={form.name} />
										</label>
										<label class="block">
											<span class="mb-1 block text-xs font-semibold text-slate-700">{tr('Website / Domain')}</span>
											<div class="relative">
												<input class="app-input w-full pr-8 text-sm" type="url" placeholder="https://medianasional.example" bind:value={form.website_url} />
												{#if form.website_url}
													<a href={form.website_url} target="_blank" rel="noreferrer" class="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-blue-600 transition-colors p-0.5" title={tr('Buka tautan')}>
														<ExternalLink size={13} />
													</a>
												{/if}
											</div>
										</label>
									</div>

										<div>
											<span class="mb-1.5 block text-xs font-semibold text-slate-700">{tr('Bahasa artikel')}</span>
											<div class="flex flex-wrap gap-4 pt-0.5">
												{#each ['Indonesia', 'Inggris', 'China', 'Arab'] as lang (lang)}
													<label class="flex cursor-pointer items-center gap-2 text-sm text-slate-700 select-none">
														<input type="checkbox" class="h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary/20" checked={form.bahasa_artikel.includes(lang)} onchange={() => toggleBahasa(lang)} />
														<span>{lang}</span>
													</label>
												{/each}
											</div>
										</div>

											<div class="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
												<div>
													<span class="mb-1 block text-xs font-semibold text-slate-700">{tr('Jumlah kata')}</span>
													<div class="flex flex-wrap items-center gap-2">
														<Select
															bind:value={form.word_count_mode}
															ariaLabel={tr('Jumlah kata artikel')}
															minWidth={0}
															fitContent
															options={WORD_COUNT_OPTIONS.map((m) => ({ value: m.value, label: tr(m.label) }))}
														/>
														{#if form.word_count_mode === 'custom'}
															<div class="flex items-center gap-1.5">
																<input
																	type="number"
																	min="50"
																	max="50000"
																	step="50"
																	bind:value={customWordMin}
																	aria-label={tr('Jumlah kata minimal')}
																	class="h-[38px] w-24 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-800 tabular-nums outline-none transition focus:border-primary focus:ring-3 focus:ring-primary/10"
																/>
																<span class="text-sm text-slate-400">–</span>
																<input
																	type="number"
																	min="50"
																	max="50000"
																	step="50"
																	bind:value={customWordMax}
																	aria-label={tr('Jumlah kata maksimal')}
																	class="h-[38px] w-24 rounded-lg border border-slate-200 bg-white px-3 text-sm text-slate-800 tabular-nums outline-none transition focus:border-primary focus:ring-3 focus:ring-primary/10"
																/>
																<span class="text-sm text-slate-500">{tr('kata')}</span>
															</div>
														{/if}
													</div>
												</div>
											<div>
												<div class="mb-1 flex items-center gap-1.5">
													<span class="text-xs font-semibold text-slate-700">{tr('Support embed (Video/Maps)')}</span>
													<Tooltip.Provider delayDuration={100}>
														<Tooltip.Root>
															<Tooltip.Trigger>
																{#snippet child({ props })}
																	<button
																		{...props}
																		type="button"
																		class="inline-flex cursor-help items-center text-slate-400 hover:text-slate-600 transition-colors"
																		aria-label={tr('Bila diizinkan, artikel berita bisa menyematkan video/peta interaktif.')}
																	>
																		<CircleAlert size={13} />
																	</button>
																{/snippet}
															</Tooltip.Trigger>
															<Tooltip.Portal>
																<Tooltip.Content class="uiTip max-w-xs text-xs" strategy="fixed" sideOffset={4}>
																	<Tooltip.Arrow class="uiTipArrow" />
																	{tr('Bila diizinkan, artikel berita bisa menyematkan video/peta interaktif.')}
																</Tooltip.Content>
															</Tooltip.Portal>
														</Tooltip.Root>
													</Tooltip.Provider>
												</div>
												<div class="flex items-center gap-2.5 h-10">
													<label class="inline-flex cursor-pointer items-center gap-2">
														<input type="checkbox" class="peer sr-only" aria-label={tr('Izinkan embed video atau peta')} bind:checked={form.allow_embed} />
														<span class="relative h-5 w-9 rounded-full bg-slate-200 transition-colors after:absolute after:left-0.5 after:top-0.5 after:h-4 after:w-4 after:rounded-full after:bg-white after:shadow-sm after:transition-transform after:content-[''] peer-checked:bg-primary peer-checked:after:translate-x-4"></span>
														<span class="text-xs font-medium text-slate-700">{form.allow_embed ? tr('Diizinkan') : tr('Tidak diizinkan')}</span>
													</label>
												</div>
											</div>
										</div>

										<div class="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
											<div>
												<span class="mb-1 block text-xs font-semibold text-slate-700">{tr('Label sponsor')}</span>
												<div>
													<Select
														bind:value={form.label_sponsor}
														ariaLabel={tr('Label sponsor')}
														minWidth={0}
														fitContent
														options={[
															{ value: 'tidak_ada', label: tr('Tidak ada (tayang organik)') },
															{ value: 'ada_sponsor', label: tr('Ada tag sponsor') }
														]}
													/>
												</div>
											</div>
											<div>
												<span class="mb-1 block text-xs font-semibold text-slate-700">{tr('Topik terlarang')}</span>
												{#if form.topik_terlarang.length > 0}
													<div class="mb-2 flex flex-wrap gap-1">
														{#each form.topik_terlarang as t (t)}
															<span class="inline-flex items-center gap-1.5 rounded-lg px-2 py-0.5 text-xs font-medium bg-rose-50 text-rose-700 border border-rose-200/60">
																{t}
																<button type="button" aria-label={`${tr('Hapus')} ${t}`} class="cursor-pointer hover:text-rose-900 leading-none" onclick={() => removeFormTopik(t)}>×</button>
															</span>
														{/each}
													</div>
												{/if}
												<div class="flex items-center gap-1.5">
													<input
														class="app-input min-w-0 flex-1 text-xs"
														placeholder={tr('Contoh: Politik, SARA, Judi…')}
														bind:value={form.topikQuery}
														onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addFormTopik(form.topikQuery); } }}
													/>
													<button type="button" class="app-btn app-btn-soft shrink-0 !text-xs !px-2.5" onclick={() => addFormTopik(form.topikQuery)}>＋ {tr('Tambah')}</button>
												</div>
											</div>
										</div>
								</section>

								<!-- 2. Kategori media -->
								<section class="rounded-2xl border border-slate-200/80 bg-white p-5 space-y-3.5 shadow-sm">
									<div class="flex items-center gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
											<Tag size={18} />
										</div>
										<div>
											<h3 class="text-sm font-bold text-slate-900">{tr('Kategori media')}</h3>
											<p class="text-xs text-muted-foreground">{tr('Tambahkan satu atau beberapa kategori yang tersedia di media ini. Kategori akan digunakan sebagai filter di Dashboard Klien.')}</p>
										</div>
									</div>

									{#if form.categories.length > 0}
										<div class="flex flex-wrap gap-1.5 pt-1">
											{#each form.categories as c (c)}
												<span class="inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1 text-xs font-medium bg-blue-50 text-blue-700 border border-blue-200/70">
													{c}
													<button type="button" aria-label={`${tr('Hapus')} ${c}`} class="cursor-pointer hover:text-blue-900 font-bold leading-none" onclick={() => removeFormCategory(c)}>×</button>
												</span>
											{/each}
										</div>
									{/if}

									<div class="flex flex-wrap items-stretch gap-2">
										<div class="relative min-w-0 flex-1 basis-full sm:basis-0">
											<Search size={14} class="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
											<input
												class="app-input w-full pl-8 text-xs sm:text-sm"
												placeholder={tr('Cari kategori…')}
												bind:value={form.categoryQuery}
												onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addFormCategory(form.categoryQuery); } }}
											/>
										</div>
										<button type="button" class="app-btn app-btn-soft shrink-0 !text-xs !px-3 font-medium whitespace-nowrap w-full sm:w-auto" onclick={() => addFormCategory(form.categoryQuery)}>＋ {tr('Tambah kategori')}</button>
									</div>

									{#if form.categories.length >= 20}
										<div class="text-xs font-medium text-amber-600">{tr('Maksimal 20 kategori per media.')}</div>
									{:else if catSuggestions.length > 0 || (catQuery && !catPool.some((c) => c.toLowerCase() === catQuery))}
										<div class="flex flex-wrap gap-1.5 pt-0.5">
											{#each catSuggestions as c (c)}
												<button type="button" class="rounded-lg border border-slate-200 bg-slate-50/60 px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100 hover:border-slate-300 transition-colors" onclick={() => addFormCategory(c)}>＋ {c}</button>
											{/each}
											{#if catQuery && !catPool.some((c) => c.toLowerCase() === catQuery)}
												<button
													type="button"
													class="rounded-lg px-2.5 py-1 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50"
													style="background: color-mix(in srgb, var(--primary) 10%, transparent); color: var(--primary);"
													disabled={form.categories.some((c) => c.trim().toLowerCase() === catQuery)}
													onclick={() => addFormCategory(form.categoryQuery.trim())}
												>＋ {tr('Buat kategori baru')} "{form.categoryQuery.trim()}"</button>
											{/if}
										</div>
									{/if}
								</section>

								<!-- 3. Contoh artikel -->
								<section class="rounded-2xl border border-slate-200/80 bg-white p-5 space-y-3 shadow-sm">
									<div class="flex items-center gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
											<FileText size={18} />
										</div>
										<div>
											<h3 class="text-sm font-bold text-slate-900">{tr('Contoh artikel')}</h3>
											<p class="text-xs text-muted-foreground">{tr('Tunjukkan satu artikel asli dari media Anda. Pembeli memakainya untuk melihat layout, ukuran font, peletakan gambar, dan penempatan backlink.')}</p>
										</div>
									</div>

									{#if form.contoh_artikel_url}
										<div class="rounded-xl border border-slate-200/90 bg-slate-50/70 p-3.5 flex flex-wrap items-center justify-between gap-3">
											<div class="flex items-center gap-2.5 min-w-0 flex-1">
												<span class="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-medium bg-emerald-50 text-emerald-700 border border-emerald-200/70 shrink-0">
													<Check size={12} /> {tr('Tersedia')}
												</span>
												<a href={form.contoh_artikel_url} target="_blank" rel="noreferrer" class="truncate text-xs sm:text-sm font-medium text-blue-600 hover:underline inline-flex items-center gap-1">
													<span class="truncate">{form.contoh_artikel_url}</span>
													<ExternalLink size={12} class="shrink-0" />
												</a>
											</div>
												<div class="flex items-center gap-1.5 shrink-0">
													<button type="button" class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs font-medium" onclick={openContohFromForm}><Pencil size={12} /> {tr('Ganti')}</button>
													<div class="relative inline-block">
														<button
															type="button"
															class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs font-medium text-rose-600 hover:text-rose-700"
															onclick={() => (confirmDeleteContoh = !confirmDeleteContoh)}
														>
															<X size={12} /> {tr('Hapus')}
														</button>

														{#if confirmDeleteContoh}
															<!-- Backdrop transparan untuk click outside -->
															<button
																type="button"
																class="fixed inset-0 z-40 cursor-default bg-transparent"
																aria-label={tr('Tutup')}
																onclick={() => (confirmDeleteContoh = false)}
															></button>

															<!-- Popover konfirmasi -->
															<div
																class="absolute right-0 bottom-full mb-1.5 z-50 w-64 rounded-xl border border-slate-200/90 bg-white p-3 shadow-xl text-left"
																style="box-shadow: 0 10px 25px -5px rgba(15, 23, 42, 0.15);"
															>
																<div class="flex items-start gap-2.5">
																	<div class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-rose-50 text-rose-600 border border-rose-100">
																		<Trash2 size={13} />
																	</div>
																	<div>
																		<p class="text-xs font-bold text-slate-900">{tr('Hapus contoh artikel?')}</p>
																		<p class="text-[11px] text-muted-foreground mt-0.5 leading-snug">
																			{tr('Tautan contoh artikel yang tersimpan akan dikosongkan.')}
																		</p>
																	</div>
																</div>
																<div class="mt-2.5 flex items-center justify-end gap-1.5 pt-2 border-t border-slate-100">
																	<button
																		type="button"
																		class="app-btn app-btn-soft !min-h-6 !px-2 !text-[11px]"
																		onclick={() => (confirmDeleteContoh = false)}
																	>
																		{tr('Batal')}
																	</button>
																	<button
																		type="button"
																		class="app-btn !min-h-6 !px-2.5 !text-[11px] bg-rose-600 hover:bg-rose-700 text-white font-semibold shadow-sm transition-colors"
																		onclick={() => {
																			form.contoh_artikel_url = '';
																			confirmDeleteContoh = false;
																		}}
																	>
																		{tr('Ya, hapus')}
																	</button>
																</div>
															</div>
														{/if}
													</div>
												</div>
										</div>
									{:else}
										<button
											type="button"
											class="w-full rounded-xl border border-slate-200/80 bg-slate-50/60 hover:bg-slate-50 p-3.5 transition flex items-center justify-between group cursor-pointer text-left"
											onclick={openContohFromForm}
										>
											<div class="flex items-center gap-2.5">
												<div class="flex h-8 w-8 items-center justify-center rounded-lg bg-blue-50 text-blue-600 border border-blue-100/40">
													<Link2 size={15} />
												</div>
												<span class="text-sm font-semibold text-slate-800">{tr('Tambah contoh artikel')}</span>
											</div>
											<ChevronRight size={16} class="text-slate-400 group-hover:text-slate-600 transition-colors" />
										</button>
									{/if}
									<p class="text-[11px] text-muted-foreground leading-relaxed pt-0.5">
										{tr('Pastikan contoh artikel yang Anda input, terbebas dari perjanjian NDA, Tauthub mendorong mitra media untuk menjaga kerasahasiaan kerjasama dengan klien.')}
									</p>
								</section>

								<!-- Kesimpulan Website: di HP bagian ini dipindah ke paling bawah form. -->
								<div class="hidden lg:block">{@render summarySection()}</div>
							</div>

							<!-- ════════ KOLOM KANAN (5 cols ~42%) ════════ -->
							<div class="lg:col-span-5 space-y-5">
								<!-- 1. Jenis media -->
								<section class="rounded-2xl border border-slate-200/80 bg-white p-5 space-y-3.5 shadow-sm">
									<div class="flex items-center gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
											<Building2 size={18} />
										</div>
										<div>
											<h3 class="text-sm font-bold text-slate-900">{tr('Jenis media')}</h3>
											<p class="text-xs text-muted-foreground">{tr('Pilih jenis media — bukan kategori. Kategori ada di bagian Kategori media.')}</p>
										</div>
									</div>

									<div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5 pt-0.5">
										{#each [
											{ v: 'blogger', label: 'Media Blogger', desc: 'Blog personal / Blogger yang digunakan sebagai media publikasi.' },
											{ v: 'mainstream', label: 'Media Mainstream', desc: 'Media berita / publisher profesional.' }
										] as opt (opt.v)}
											{@const active = form.media_type === opt.v}
											<button
												type="button"
												class="rounded-xl border p-3.5 text-left transition cursor-pointer flex flex-col justify-between"
												style="border-color: {active ? 'var(--primary)' : 'var(--border)'}; background: {active ? 'var(--accent)' : 'var(--card)'};"
												onclick={() => (form.media_type = opt.v)}
											>
												<div>
													<div class="flex items-center gap-2 mb-1.5">
														<span class="flex h-4 w-4 shrink-0 items-center justify-center rounded-full border transition-colors" style="border-color: {active ? 'var(--primary)' : 'var(--muted-foreground)'};">
															{#if active}<span class="h-2 w-2 rounded-full" style="background: var(--primary);"></span>{/if}
														</span>
														<span class="text-sm font-bold text-slate-900">{tr(opt.label)}</span>
													</div>
													<p class="text-xs text-muted-foreground leading-snug">{tr(opt.desc)}</p>
												</div>
											</button>
										{/each}
									</div>
								</section>

								<!-- 2. Platform & integrasi -->
								<section class="rounded-2xl border border-slate-200/80 bg-white p-5 space-y-4 shadow-sm">
									<div class="flex items-center gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600 border border-blue-100/50">
											<Layers size={18} />
										</div>
										<div>
											<h3 class="text-sm font-bold text-slate-900">{tr('Platform & integrasi')}</h3>
											<p class="text-xs text-muted-foreground">{tr('Pilih metode integrasi untuk pengiriman artikel.')}</p>
										</div>
									</div>

									<div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5 pt-0.5">
										{#each [
											{ v: 'draft', label: tr('Manual'), helper: 'Mitra copy-paste artikel ke web-nya sendiri dan input URL publikasi di artikel klien. Status jadi Terbit otomatis saat URL dikirim.' },
											{ v: 'publish', label: tr('API'), helper: 'Artikel di-push otomatis ke WordPress/Blogger/Custom Web. Status jadi Terbit otomatis setelah klik ACC.' }
										] as opt (opt.v)}
											{@const active = form.publication_mode === opt.v}
											<button
												type="button"
												class="rounded-xl border p-3.5 text-left transition cursor-pointer flex flex-col justify-between"
												style="border-color: {active ? 'var(--primary)' : 'var(--border)'}; background: {active ? 'var(--accent)' : 'var(--card)'};"
												onclick={() => (form.publication_mode = opt.v)}
											>
												<div>
													<div class="flex items-center gap-2 mb-1.5">
														<span class="flex h-4 w-4 shrink-0 items-center justify-center rounded-full border transition-colors" style="border-color: {active ? 'var(--primary)' : 'var(--muted-foreground)'};">
															{#if active}<span class="h-2 w-2 rounded-full" style="background: var(--primary);"></span>{/if}
														</span>
														<span class="text-sm font-bold text-slate-900">{tr(opt.label)}</span>
													</div>
													<p class="text-xs text-muted-foreground leading-snug">{tr(opt.helper)}</p>
												</div>
											</button>
										{/each}
									</div>

									{#if form.publication_mode === 'publish'}
										<div class="space-y-2.5 pt-1">
											<div class="text-xs font-semibold text-slate-700">{tr('Platform Tujuan')}</div>
											<div class="flex flex-wrap items-center gap-2">
												<button type="button" class="app-btn !min-h-7 !px-2.5 !text-xs font-medium" class:app-btn-primary={form.integration_type === 'wordpress'} class:app-btn-soft={form.integration_type !== 'wordpress'} onclick={() => (form.integration_type = 'wordpress')}>WordPress</button>
												<button type="button" class="app-btn !min-h-7 !px-2.5 !text-xs font-medium" class:app-btn-primary={form.integration_type === 'blogger'} class:app-btn-soft={form.integration_type !== 'blogger'} onclick={() => (form.integration_type = 'blogger')}>Blogger</button>
												<button type="button" class="app-btn !min-h-7 !px-2.5 !text-xs font-medium" class:app-btn-primary={form.integration_type === 'custom_web'} class:app-btn-soft={form.integration_type !== 'custom_web'} onclick={() => (form.integration_type = 'custom_web')}>Custom Web</button>
												<span class="text-xs text-muted-foreground">
													{form.integration_type === 'wordpress' ? tr('Butuh koneksi WordPress') : form.integration_type === 'blogger' ? tr('Butuh koneksi Blogger') : tr('Butuh endpoint API')}
												</span>
											</div>
											{#if form.integration_type === 'wordpress'}
												<div class="flex items-start gap-2 rounded-xl p-3 text-xs bg-blue-50 text-blue-800 border border-blue-100">
													<Info size={14} class="mt-0.5 shrink-0 text-blue-600" />
													<span>{tr('Domain perlu di-ACC Admin terlebih dahulu, setelah disetujui silakan koneksikan di halaman')} <a class="underline font-semibold" href={`${base}/connections`}>{tr('WordPress')}</a>.</span>
												</div>
											{:else if form.integration_type === 'blogger'}
												<div class="flex items-start gap-2 rounded-xl p-3 text-xs bg-blue-50 text-blue-800 border border-blue-100">
													<Info size={14} class="mt-0.5 shrink-0 text-blue-600" />
													<span>{tr('Hubungkan akun Google dan pilih blog di halaman')} <a class="underline font-semibold" href={`${base}/blogger`}>{tr('Blogger')}</a> {tr('setelah media ini disetujui Admin.')}</span>
												</div>
											{:else if form.integration_type === 'custom_web'}
												<div class="flex items-start gap-2 rounded-xl p-3 text-xs bg-amber-50 text-amber-800 border border-amber-200/70">
													<TriangleAlert size={14} class="mt-0.5 shrink-0 text-amber-600" />
													<span>{tr('Endpoint URL, API Key, dan Test Connection diatur di halaman')} <a class="underline font-semibold" href={`${base}/publishing-api`}>{tr('Web Custom')}</a> {tr('setelah media ini disetujui Admin.')}</span>
												</div>
											{/if}
											<div class="rounded-xl border border-slate-200/80 bg-slate-50/70 p-3">
												<label class="flex cursor-pointer items-start gap-2 text-xs text-slate-700">
													<input type="checkbox" class="mt-0.5 h-4 w-4 rounded border-slate-300 text-primary focus:ring-primary/20" checked={integrasiConsent} onchange={(e) => (integrasiConsent = (e.currentTarget as HTMLInputElement).checked)} />
													<span>
														{tr('Saya telah membaca dan menyetujui')}
														<a href="https://tauthub.com/kerja-sama-mitra/" target="_blank" rel="noreferrer" class="underline font-semibold text-primary">{tr('Ketentuan Kerja Sama Mitra')}</a>
														{tr('untuk mode integrasi. Wajib dicentang sebelum media dapat disimpan.')}
													</span>
												</label>
											</div>
										</div>
									{/if}

										<div class="space-y-3.5 pt-1">
											<div>
												<span class="mb-1 block text-xs font-semibold text-slate-700">{tr('Jumlah backlink yang diperbolehkan dalam artikel')}</span>
												<div>
													<Select
														bind:value={form.max_links}
														ariaLabel={tr('Jumlah backlink yang diperbolehkan dalam artikel')}
														minWidth={0}
														fitContent
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

											<div>
												<span class="mb-1 block text-xs font-semibold text-slate-700">{tr('Subjudul (H2/H3) dalam artikel')}</span>
												<div>
													<Select
														bind:value={form.allow_heading}
														ariaLabel={tr('Izin subjudul dalam artikel')}
														minWidth={0}
														fitContent
														options={[
															{ value: 'default', label: tr('Ikut bawaan tipe media') },
															{ value: 'yes', label: tr('Boleh memakai subjudul') },
															{ value: 'no', label: tr('Tanpa subjudul') }
														]}
													/>
												</div>
												<span class="mt-1 block text-xs text-muted-foreground">{tr('Bawaan: media blog boleh memakai subjudul, media berita tidak. Pilih "Boleh" bila media berita Anda menerima artikel dengan subjudul.')}</span>
											</div>

										<div>
											<span class="mb-1.5 block text-xs font-semibold text-slate-700">{tr('Masa berlaku tautan (backlink)')}</span>
											<div class="inline-flex w-full rounded-xl border border-slate-200 bg-slate-100/70 p-1">
												<button
													type="button"
													class="flex-1 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all {linkDurationPermanent ? 'bg-white text-slate-900 shadow-sm border border-slate-200/60' : 'text-muted-foreground hover:text-slate-800'}"
													aria-pressed={linkDurationPermanent}
													onclick={() => (form.link_duration_months = '0')}
												>{tr('Permanen')}</button>
												<button
													type="button"
													class="flex-1 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all {!linkDurationPermanent ? 'bg-white text-slate-900 shadow-sm border border-slate-200/60' : 'text-muted-foreground hover:text-slate-800'}"
													aria-pressed={!linkDurationPermanent}
													onclick={() => { if (linkDurationPermanent) form.link_duration_months = '12'; }}
												>{tr('Jangka Waktu')}</button>
											</div>

											{#if linkDurationPermanent}
												<div class="mt-2.5 rounded-xl border border-blue-100 bg-blue-50/50 p-3 flex items-start gap-2.5">
													<Info size={16} class="text-blue-600 shrink-0 mt-0.5" />
													<p class="text-xs leading-relaxed text-blue-900 font-medium">
														{tr('Tautan tetap tayang terus — tidak pernah dilepas dan tidak ada masa kedaluwarsanya. Klien tidak perlu khawatir tautannya hilang sendiri.')}
													</p>
												</div>
											{:else}
												<div class="mt-2.5 rounded-xl border border-blue-100 bg-blue-50/50 p-3 space-y-2">
													<div class="flex items-start gap-2.5">
														<Info size={16} class="text-blue-600 shrink-0 mt-0.5" />
														<p class="text-xs leading-relaxed text-blue-900 font-medium">
															{tr('Klien akan memprioritaskan website yang memberikan garansi hyperlink tayang permanen')}
														</p>
													</div>
													<div class="space-y-1.5 pl-6 pt-1">
														{#each linkDurationChoices as months (months)}
															<label class="flex cursor-pointer items-center gap-2 text-xs text-slate-700">
																<input type="radio" name="link-duration-months" value={String(months)} bind:group={form.link_duration_months} class="h-3.5 w-3.5 text-primary focus:ring-primary/20" />
																<span>{months === 12 ? tr('12 Bulan') : `${months} ${tr('Bulan')}`}</span>
															</label>
														{/each}
													</div>
												</div>
											{/if}
										</div>
									</div>
								</section>

								<!-- 3. Harga & model kerja sama -->
								<section class="rounded-2xl border border-emerald-200/80 bg-white p-5 space-y-4 shadow-sm">
									<div class="flex items-center gap-3">
										<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600 border border-emerald-100/50">
											<Wallet size={18} />
										</div>
										<div>
											<h3 class="text-sm font-bold text-slate-900">{tr('Harga & model kerja sama')}</h3>
											<p class="text-xs text-muted-foreground">{tr('Harga dan kesepakatan untuk kerja sama publikasi.')}</p>
										</div>
									</div>

									<div>
										<span class="mb-1 block text-xs font-semibold text-slate-700">{tr('Harga Pengajuan (IDR)')}</span>
										<input
											class="app-input w-full text-sm font-semibold text-slate-800"
											type="text"
											inputmode="numeric"
											placeholder="50.000"
											value={form.price ? Number(form.price).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' }) : ''}
											oninput={(e) => (form.price = (e.currentTarget as HTMLInputElement).value.replace(/\D/g, ''))}
										/>
									</div>

									<div class="rounded-xl border border-emerald-200/90 bg-emerald-50/50 p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
										<div class="flex items-start gap-2.5 min-w-0 flex-1">
											<div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-emerald-100 text-emerald-700 mt-0.5">
												<CircleDollarSign size={18} />
											</div>
											<div>
												<p class="text-xs font-bold text-emerald-950">{tr('Kesepakatan')}</p>
												<p class="text-xs text-emerald-800 leading-snug mt-0.5">
													{tr('Mitra menerima')} {PARTNER_SHARE_PCT}% {tr('dari harga pengajuan.')}
												</p>
											</div>
										</div>
										<div class="text-left sm:text-right shrink-0 sm:pl-3">
											<p class="text-xl sm:text-2xl font-extrabold text-emerald-600 tabular-nums">
												Rp{formPartnerIncome.toLocaleString('id-ID')}
											</p>
											<p class="text-[11px] font-medium text-emerald-800/80 mt-0.5 tabular-nums">
												{tr('Bagian Tauthub')} {PLATFORM_CUT_PCT}% · Rp{formPlatformCut.toLocaleString('id-ID')}
											</p>
										</div>
									</div>
								</section>
							</div>
							<div class="lg:hidden">{@render summarySection()}</div>
						</div>
					</div>

					<!-- ── Footer Tetap ── -->
					<footer class="flex flex-none items-center justify-end gap-3 border-t border-slate-100 bg-white px-6 py-4">
						<button class="app-btn app-btn-soft !px-5" type="button" onclick={() => (showForm = false)}>{tr('Batal')}</button>
						<button class="app-btn app-btn-primary !px-5 font-semibold" type="submit" disabled={busy || (form.publication_mode === 'publish' && !integrasiConsent)}>
							{busy ? tr('Menyimpan…') : editingId ? tr('Simpan perubahan') : tr('Tambah media')}
						</button>
					</footer>
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
		<dialog open class="app-card my-8 w-full max-w-xl p-5" aria-label={tr('Verifikasi Kepemilikan')} onclick={(e) => e.stopPropagation()}>
			<div class="flex items-start justify-between gap-3">
				<div>
					<h2 class="text-lg font-semibold">{tr('Verifikasi Kepemilikan')}</h2>
					<p class="mt-0.5 text-xs text-muted-foreground">{domTarget.name}</p>
				</div>
				<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label={tr('Tutup')} onclick={() => (domTarget = null)}><X size={18} /></button>
			</div>
			<!-- Dua tab: verifikasi domain (dulu dialog sendiri) + verifikasi Dewan Pers. -->
			<div class="mt-3 flex gap-1 rounded-[10px] p-1" style="background: var(--secondary);" role="tablist" aria-label={tr('Jenis verifikasi')}>
				<button type="button" role="tab" aria-selected={verifTab === 'domain'} class="flex-1 rounded-lg px-3 py-1.5 text-sm font-medium" style:background={verifTab === 'domain' ? 'var(--card)' : 'transparent'} onclick={() => void switchVerifTab('domain')}>
					{tr('Verifikasi Domain')}
				</button>
				<button type="button" role="tab" aria-selected={verifTab === 'dewan'} class="flex-1 rounded-lg px-3 py-1.5 text-sm font-medium" style:background={verifTab === 'dewan' ? 'var(--card)' : 'transparent'} onclick={() => (verifTab = 'dewan')}>
					{tr('Dewan Pers')}
				</button>
			</div>
			{#if verifTab === 'domain'}
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
				<div class="mt-4 flex flex-wrap items-center justify-end gap-3">
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
			{:else}
				{#if domTarget.dewan_pers_cert_no || domTarget.dewan_pers_doc_key || domTarget.dewan_pers_status}
					<div class="mt-2 flex flex-wrap items-center gap-2 rounded-lg border p-2.5 text-xs" style="border-color: var(--border); background: var(--muted);">
						{#if domTarget.dewan_pers_status}
							<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="border: 1px solid {dpBadge(domTarget.dewan_pers_status).tone}; color: {dpBadge(domTarget.dewan_pers_status).tone};">
								{dpBadge(domTarget.dewan_pers_status).label}
							</span>
						{/if}
						{#if domTarget.dewan_pers_submitted_at}
							<span class="text-muted-foreground">{tr('Diajukan')} {dt(domTarget.dewan_pers_submitted_at)}</span>
						{/if}
						{#if domTarget.dewan_pers_doc_key}
							<a class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" href={`/uploads/${domTarget.dewan_pers_doc_key}`} target="_blank" rel="noreferrer">
								<ExternalLink size={12} /> {tr('Lihat dokumen')}
							</a>
						{/if}
					</div>
					<p class="text-xs text-muted-foreground">{tr('Unggah berkas baru di bawah untuk mengganti dokumen lama.')}</p>
				{/if}
			<form class="mt-4 flex flex-col gap-3" onsubmit={submitDewanPers}>
				<label class="flex flex-col gap-1 text-sm">
					<span class="text-xs font-medium text-muted-foreground">{tr('Nomor sertifikat *')}</span>
					<input class="app-input" required minlength="3" maxlength="120" bind:value={dpCert} />
				</label>
				<div class="flex flex-col gap-1">
					<span class="text-xs font-medium text-muted-foreground">{tr('Dokumen sertifikat (PDF/JPG, opsional)')}</span>
					<label
						class="flex cursor-pointer flex-col items-center justify-center gap-1.5 rounded-xl border border-dashed px-4 py-6 text-center focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-[var(--primary)]"
						style="border-color: {dpDocDragOver ? 'var(--primary)' : 'var(--border)'}; background: {dpDocDragOver ? 'var(--accent)' : 'var(--muted)'};"
						ondragover={(e) => { e.preventDefault(); dpDocDragOver = true; }}
						ondragleave={() => (dpDocDragOver = false)}
						ondrop={(e) => {
							e.preventDefault();
							dpDocDragOver = false;
							pickDpDoc(e.dataTransfer?.files?.[0] ?? null);
						}}
					>
						{#if dpDoc}
							<div class="flex w-full flex-wrap items-center gap-3 text-left">
								{#if dpDocUrl}
									<img src={dpDocUrl} alt={dpDoc.name} class="h-14 w-14 shrink-0 rounded-lg border object-cover" style="border-color: var(--border);" />
								{:else}
									<span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg" style="background: color-mix(in srgb, var(--primary) 12%, var(--card)); color: var(--primary);">
										<FileText size={18} />
									</span>
								{/if}
								<div class="min-w-0 flex-1">
									<span class="block truncate text-sm font-medium">{dpDoc.name}</span>
									<span class="text-xs text-muted-foreground">{fmtBytes(dpDoc.size)}</span>
								</div>
								<button
									type="button"
									class="app-btn app-btn-soft shrink-0 !min-h-7 !px-2.5 !text-xs"
									onclick={(e) => {
										e.stopPropagation();
										clearDpDocUrl();
										dpDoc = null;
									}}
								><X size={13} /> {tr('Hapus')}</button>
							</div>
						{:else}
							<Upload size={20} class="text-muted-foreground" />
							<span class="text-sm font-medium">{tr('Tarik & lepas dokumen sertifikat di sini')}</span>
							<span class="text-xs text-muted-foreground">{tr('atau klik untuk memilih berkas · PDF, JPG, PNG (maksimal 10 MB)')}</span>
						{/if}
						<input
							class="sr-only"
							type="file"
							accept=".pdf,.jpg,.jpeg,.png"
							onchange={(e) => {
								const input = e.currentTarget as HTMLInputElement;
								pickDpDoc(input.files?.[0] ?? null);
								// Kosongkan input agar berkas yang sama bisa dipilih ulang.
								input.value = '';
							}}
						/>
					</label>
					{#if dpDocError}<span class="text-xs" style="color: var(--destructive);">{dpDocError}</span>{/if}
				</div>
				{#if error}<p class="text-sm" style="color: var(--destructive);">{error}</p>{/if}
				<div class="flex flex-wrap items-center justify-end gap-2">
					{#if dpCanDelete}
						<!-- Dua langkah: klik pertama meminta konfirmasi, klik kedua menghapus. -->
						<button
							class="app-btn app-btn-soft mr-auto"
							type="button"
							disabled={dpBusy}
							style="color: {dpConfirmDelete ? '#fff' : 'var(--destructive)'}; background-color: {dpConfirmDelete ? 'var(--destructive)' : 'var(--secondary)'};"
							onclick={() => void deleteDewanPers()}
						>
							<Trash2 size={14} />
							{dpConfirmDelete ? tr('Ya, hapus pengajuan') : tr('Hapus pengajuan')}
						</button>
					{/if}
					<button class="app-btn app-btn-soft" type="button" onclick={() => (domTarget = null)}>{tr('Batal')}</button>
					<button class="app-btn app-btn-primary" type="submit" disabled={dpBusy || dpCert.trim().length < 3}>
						{dpBusy ? tr('Mengirim…') : tr('Ajukan Verifikasi')}
					</button>
				</div>
			</form>
			{/if}
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

<style>
	/* Kontrol baris dipakai bersama tabel (desktop) dan kartu (mobile). Di layar
		sentuh (<768px) target ketuknya dijamin minimal 44px; desktop tidak berubah. */
	@media (max-width: 767px) {
		.media-menu-trigger {
			min-width: 44px;
			min-height: 44px;
		}
		.media-row-menu button[role='menuitem'] {
			min-height: 44px;
		}
		.media-verify {
			min-height: 44px;
		}
		.media-contoh {
			min-height: 44px;
		}
		.media-auto-label {
			width: 100%;
			min-height: 44px;
		}
		.media-restore {
			min-height: 44px;
		}
		.media-auto-check {
			width: 18px;
			height: 18px;
		}
	}
</style>
