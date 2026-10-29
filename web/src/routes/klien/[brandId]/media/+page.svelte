<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Katalog media klien — port setia media-section.tsx (Next.js).
	// Struktur: error banner → header eksplorasi (+stats card) → top bar
	// (search, kategori, filter lanjutan, impor, bookmark) → banner filter →
	// tabel 10 kolom + sort + multi-select → paginasi → modal → selection bar.
	import { getContext, onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { fly } from 'svelte/transition';
	import { flip } from 'svelte/animate';
	import { cubicOut, cubicIn } from 'svelte/easing';
	import AgentSteps from '../taut-pintar/_agent-steps.svelte';
	import { DONE_STATUSES } from '../taut-pintar/_shared';
	import {
		ArrowRight, BarChart3, Bookmark, Check, CircleAlert, Clock, Database, FileText, Info, LayoutGrid,
		Link as LinkIcon, Lock,
		Plus, RotateCcw, Search, ShieldCheck, SlidersHorizontal, Sparkles, Tag, Trash2, Upload, Wallet, X, Zap,
		ChevronLeft, ChevronRight
	} from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { aiTrustToneStyle } from '$lib/media/ai-trust-tone';
	import { campaignUrlId } from '$lib/campaign-url';
	import { mediaDomainLabel } from '$lib/media/domain-label';
	import { mediaDistribution } from '$lib/media/distribution';
	import { linkPolicyBadge, isPermanentLink } from '$lib/media/link-duration';
	import { avgPublishHours, formatSla, slaBucket, slaDurationText } from '$lib/media/sla';
	import { seededShuffle, randomShuffleSeed } from '$lib/media/shuffle';
	import { apiRequest, ensureValidSession, readApiError, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { getI18n } from '$lib/i18n/i18n.svelte';
	import MediaDetailModal from './_detail-modal.svelte';
	import MatchmakerModal from './_matchmaker-modal.svelte';
	import Select from '$lib/components/select.svelte';
	import SlideOver from '$lib/components/slide-over.svelte';
	import { Tooltip } from 'bits-ui';
	import { getCategoryTone } from '$lib/media/category-tone';
	import './media.css';
	import PageTitle from '$lib/components/page-title.svelte';
	import { swrPeek, swrPut } from '$lib/swr';

	type Media = {
		id: string; name: string; category: string | null; categories?: string[];
		description: string | null; website_url: string | null;
		contoh_artikel_url?: string | null; bahasa_artikel?: string[] | null;
		price: number; pricing_type?: string | null; client_price: number;
		markup_percent?: number | null; credit_cost: number;
		integration_type: string; traffic_estimate: number; media_tier: string | null;
		media_type?: string | null; media_quality_score: number | null;
		effective_quality: number | null; quality_source: 'media' | 'tier_default' | null;
		health_score: number | null; health_breakdown: Record<string, unknown> | null;
		health_computed_at?: string | null;
		dewan_pers_verified?: boolean | null; domain_verified?: boolean | null;
		seo_da?: number | null; seo_pa?: number | null; seo_spam_score?: number | null;
		seo_checked_at?: string | null; seo_ai_trust?: number | null; seo_ai_citation?: boolean | null;
		sw_visits?: number | null; sw_snapshot_date?: string | null;
		published_count?: number | null;
		rating_avg?: number | null; rating_count?: number | null;
		partner_id?: string | null; partner_public_id?: string | null; partner_name?: string | null;
		gsc_connected?: boolean; image_required?: boolean | null;
		label_sponsor?: string | null; topik_terlarang?: string[];
		max_links?: number | null; allow_hyperlink?: boolean | null;
		link_duration_months?: number | null;
		publication_mode?: string | null; auto_publish?: boolean | null; word_count_mode?: string | null;
		allow_embed?: boolean; pricing_type_raw?: string | null;
		status?: string | null;
	};

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}`);
	const { t } = getI18n();

	let rows = $state<Media[]>([]);
	let busy = $state(true);
	let err = $state('');
	let search = $state('');
	let debouncedSearch = $state('');
	let category = $state('');
	let speedFilter = $state<'' | 'super' | 'fast' | 'standard'>('');
	// Rentang harga bebas (Rupiah). Chip lama jadi preset yang mengisi input
	// ini, jadi budget aneh seperti 700rb–1,5jt tetap bisa dicari.
	let priceMinText = $state('');
	let priceMaxText = $state('');
	// Media tanpa harga pasti (price 0) tidak punya angka untuk dibandingkan.
	// Opsi ini memunculkannya kembali saat filter rentang aktif.
	let includePricePending = $state(false);
	const priceMin = $derived(parseRupiahInput(priceMinText));
	const priceMax = $derived(parseRupiahInput(priceMaxText));
	const priceRange = $derived.by(() => {
		let lo = priceMin;
		let hi = priceMax;
		if (lo != null && hi != null && lo > hi) [lo, hi] = [hi, lo];
		return { lo, hi };
	});
	const priceActive = $derived(priceRange.lo != null || priceRange.hi != null);
	function parseRupiahInput(value: string): number | null {
		const digits = value.replace(/\D/g, '');
		return digits ? Number(digits) : null;
	}
	function formatRupiahInput(value: string): string {
		const n = parseRupiahInput(value);
		return n == null ? '' : new Intl.NumberFormat('id-ID').format(n);
	}
	function normalizePriceInputs() {
		priceMinText = formatRupiahInput(priceMinText);
		priceMaxText = formatRupiahInput(priceMaxText);
		const lo = parseRupiahInput(priceMinText);
		const hi = parseRupiahInput(priceMaxText);
		// Urutan terbalik ditukar saat blur supaya yang tersimpan selalu masuk akal.
		if (lo != null && hi != null && lo > hi) {
			priceMinText = formatRupiahInput(String(hi));
			priceMaxText = formatRupiahInput(String(lo));
		}
	}
	// Minimum DA memakai slider (0 = tanpa batas), menggantikan chip DA ≥ 10/20/30.
	let minDa = $state(0);
	// Terbit otomatis: jalur bypass tanpa persetujuan mitra. Ini flag "Mode
	// Otomatis" per media (media.auto_publish) — artikel dari klien langsung
	// diproses dan terbit ke situs media tanpa menunggu acc mitra. Berbeda dari
	// publication_mode, yang hanya menentukan siapa yang menempel artikelnya.
	let autoPublishOnly = $state(false);
	// AI Citation: media yang terdeteksi dikutip atau dirujuk mesin AI.
	let aiCitationOnly = $state(false);
	let verifiedOnly = $state(false);
	let dewanPersOnly = $state(false);
	let bookmarkOnly = $state(false);
	let selectedIds = $state<Set<string>>(new Set());
	let bookmarkedIds = $state<Set<string>>(new Set());
	// ── Animasi "fly to campaign" saat media dipilih ──────────────────────
	// Hormati prefers-reduced-motion: kalau pengguna mematikannya, state tetap
	// diperbarui tetapi tanpa gerakan terbang.
	const motionOK = typeof window !== 'undefined' && !window.matchMedia('(prefers-reduced-motion: reduce)').matches;
	let flyLayer = $state<HTMLElement | null>(null);
	let flashRowId = $state<string | null>(null);
	let barPulse = $state(false);
	let createBtnEl = $state<HTMLButtonElement | null>(null);
	let bookmarkToolbarBtn = $state<HTMLButtonElement | null>(null);
	let bookmarkPulse = $state(false);
	let detail = $state<Media | null>(null);
	// AI Media Matchmaker: brief bebas → kriteria → pilihan media katalog.
	let matchmakerOpen = $state(false);
	// Detail yang dibuka dari dalam Matchmaker: tombol "Tambahkan" harus
	// mencentang media, bukan membuka modal kampanye di belakang modal AI.
	let detailFromMatchmaker = $state(false);
	// Media terarsip (soft delete oleh mitra) tetap tampil sebagai "maintenance"
	// di katalog klien, tapi tidak bisa dipilih untuk distribusi.
	const archivedIds = $derived(new Set(rows.filter((m) => m.status === 'archived').map((m) => m.id)));
	// Default "Acak": urutan rotasi per muat ulang supaya semua media kebagian
	// tampil di halaman awal, bukan selalu alfabetis dari atas.
	let sortKey = $state<'traffic' | 'da' | 'ai_trust' | 'ai_cited' | 'speed' | 'price' | 'name' | 'acak'>('acak');
	// Seed diacak sekali saat halaman dimuat; tombol "Acak" menggantinya.
	let shuffleSeed = $state(randomShuffleSeed());
	let sortDir = $state<'asc' | 'desc'>('asc');
	let catPopoverId = $state<string | null>(null);
	// Popover kategori diposisikan fixed (seperti portal Radix di app lama)
	// supaya tidak terpotong overflow:hidden milik .modernCard / .tableWrap.
	let catPopoverPos = $state<{ left: number; top: number; atas: boolean }>({ left: 0, top: 0, atas: false });
	let catPopoverCats = $state<string[]>([]);

	function openCategoryPopover(event: MouseEvent, id: string, cats: string[]) {
		event.stopPropagation();
		if (catPopoverId === id) {
			catPopoverId = null;
			return;
		}
		const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
		const width = 280;
		const left = Math.min(Math.max(8, rect.left), Math.max(8, window.innerWidth - width - 8));
		// Kondisional: kalau ruang di bawah tidak cukup (mis. baris di dekat dasar
		// layar), popover tampil KE ATAS; selain itu ke bawah seperti biasa.
		const perkiraanTinggi = 200;
		const ruangBawah = window.innerHeight - rect.bottom - 8;
		const ruangAtas = rect.top - 8;
		const bukaKeAtas = ruangBawah < perkiraanTinggi && ruangAtas > ruangBawah;
		catPopoverPos = { left, top: bukaKeAtas ? rect.top - 4 : rect.bottom + 4, atas: bukaKeAtas };
		catPopoverCats = cats;
		catPopoverId = id;
	}

	$effect(() => {
		if (!catPopoverId) return;
		const close = () => (catPopoverId = null);
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape') close();
		};
		window.addEventListener('scroll', close, true);
		window.addEventListener('resize', close);
		window.addEventListener('keydown', onKey);
		return () => {
			window.removeEventListener('scroll', close, true);
			window.removeEventListener('resize', close);
			window.removeEventListener('keydown', onKey);
		};
	});
	let page_ = $state(1);
	const pageSize = 24;
	let advOpen = $state(false);
	let advBtn = $state<HTMLButtonElement | null>(null);

	// Lebar layar: panel alat dari kanan HANYA untuk mobile. Di desktop alat
	// katalog tampil langsung di toolbar (kategori, Filter, Impor List,
	// Bookmark) seperti semula — corong + panel kanan baru dipakai di layar
	// sempit. Dipilih lewat matchMedia (bukan CSS dua-render) supaya markup
	// alatnya hanya ada sekali, jadi binding tombol bookmark tidak dobel.
	let isNarrow = $state(typeof window !== 'undefined' && window.matchMedia('(max-width: 1023px)').matches);
	$effect(() => {
		if (typeof window === 'undefined') return;
		const mq = window.matchMedia('(max-width: 1023px)');
		const sync = () => (isNarrow = mq.matches);
		sync();
		mq.addEventListener('change', sync);
		return () => mq.removeEventListener('change', sync);
	});
	// Panel filter lanjutan mengambang (parity Radix Popover.Content align="end"
	// sideOffset={8}) — bukan panel inline yang mendorong tabel ke bawah.
	let advPos = $state<{ left: number; top: number }>({ left: 0, top: 0 });

	function openAdvFilter() {
		if (advOpen) {
			advOpen = false;
			return;
		}
		const rect = advBtn?.getBoundingClientRect();
		const width = 440;
		const left = rect
			? Math.min(Math.max(8, rect.right - width), Math.max(8, window.innerWidth - width - 8))
			: 8;
		advPos = { left, top: (rect?.bottom ?? 0) + 8 };
		advOpen = true;
	}

	$effect(() => {
		if (!advOpen) return;
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape') advOpen = false;
		};
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});
	let importOpen = $state(false);
	let importText = $state('');
	let importError = $state('');
	let fileInput = $state<HTMLInputElement | undefined>(undefined);
	let bookmarkOpen = $state(false);
	let campaignOpen = $state(false);
	let campaignName = $state('');
	let campaignError = $state('');
	let creating = $state(false);
	// Profiler (workspace knowledge klien) opsional untuk kampanye baru.
	// Sementara disembunyikan atas permintaan: ubah ke true untuk memunculkan
	// kembali pilihan Profiler di modal "Buat Kampanye".
	const SHOW_PROFILER_FIELD = false;
	let profilerOptions = $state<Array<{ value: string; label: string }>>([]);
	let profilerId = $state('');
	let profilerLoading = $state(false);
	// Panel alat katalog (kategori, filter, impor, bookmark) dari kanan.
	let toolsOpen = $state(false);
	// Kategori: radio (maks 5 tampil, sisanya lewat tombol expand).
	let catExpand = $state(false);
	// Panel info cepat (mobile): detail kolom yang disembunyikan di tabel.
	let quickInfo = $state<any | null>(null);
	// Cek pesanan ganda: media yang sudah dipesan di kampanye aktif lain.
	// Saldo kredit (bar seleksi & modal) — null = belum/gagal terbaca.
	let walletAvailableTc = $state<number | null>(null);
	let walletRateIdr = $state(1000);
	// Jumlah media yang dipulihkan dari sesi terakhir (badge, bukan error).
	let restoredSelectionCount = $state(0);
	// Popover daftar media terpilih (dibuka dari kartu "MEDIA TERPILIH" di bar bawah).
	let selectedListOpen = $state(false);
	// Pilihan bisa memuat media di luar 250 baris yang sedang dimuat (filter lain)
	// — snapshot harga disimpan supaya total di bar tidak bohong.
	let selectedSnapshot = $state<Record<string, { name: string; price: number; credits: number; domain?: string }>>({});
	let selectionHydrated = $state(false);
	// Mode "tambah ke kampanye": katalog dibuka dari Penyedia Konten lewat
	// ?campaign=<urlId kampanye>. Berisi urlId tujuan, atau null untuk alur biasa.
	let addToCampaignId = $state<string | null>(page.url.searchParams.get('campaign'));
	// Media yang sudah ada di kampanye saat katalog dibuka — dipakai untuk tahu
	// mana yang benar-benar tambahan saat menyimpan, dan untuk tombol "Batal".
	let originalCampaignIds = $state<Set<string>>(new Set());
	let adding = $state(false);
	let addError = $state('');
	// Kampanye tujuan sudah selesai (submitted/scheduled/distributed): katalog
	// tetap boleh dilihat, tapi tidak boleh menambah media lagi.
	let campaignDone = $state(false);

	const BOOKMARK_KEY = 'tauthub_client_bookmarked_media';
	const SELECTED_KEY = 'tauthub_client_selected_media';

	// ── Helper parity media-section.tsx ────────────────────────────────
	/** Estimasi trafik. CATATAN: tren ↑/↓ di app lama DIREKAYASA dari
	 *  `Math.abs(val % 31)` — angka palsu, jadi tidak disalin ke sini. */
	function formatTraffic(swVisits?: number | null, est?: number | null): { text: string; source: string } {
		const val = swVisits && swVisits > 0 ? swVisits : est && est > 0 ? est : 0;
		if (!val) return { text: 'Estimasi', source: 'Via SimilarWeb' };
		let text: string;
		if (val >= 1_000_000) text = `± ${(val / 1_000_000).toFixed(1).replace(/\.0$/, '')}M / bulan`;
		else if (val >= 1_000) text = `± ${Math.round(val / 1_000)} rb / bulan`;
		else text = `± ${val.toLocaleString('id-ID')} / bulan`;
		return { text, source: 'Via SimilarWeb' };
	}

	// SLA waktu terbit = rata-rata waktu penerbitan NYATA media
	// (health_breakdown.avg_publish_hours), bukan taksiran dari tipe integrasi.
	// Media yang biasanya terbit 1 jam setelah artikel dikirim tampil "SLA ± 1 jam".
	function getSla(m: Media): string {
		return formatSla(avgPublishHours(m));
	}

	function slaTip(m: Media): string {
		const hours = avgPublishHours(m);
		if (hours == null) {
			return tr('Belum ada riwayat penerbitan yang cukup untuk menghitung SLA media ini.');
		}
		return tr('SLA ± {waktu} — rata-rata waktu terbit nyata media ini: artikel tayang sekitar {waktu} setelah dikirim.')
			.replace('{waktu}', slaDurationText(hours));
	}

	/** Kunci urut SLA: jam lebih kecil = lebih cepat; tanpa data paling bawah. */
	function slaSortKey(m: Media): number {
		const hours = avgPublishHours(m);
		return hours == null ? -1 : -hours;
	}

	function getDa(m: Media): string | number {
		// Hanya hasil pemeriksaan nyata (RapidAPI domain-metrics); sebelumnya DA
		// diturunkan dari skor kualitas media bila belum diperiksa.
		if (m.seo_da != null) return m.seo_da;
		return '—';
	}
	function getAiTrust(m: Media): string | number {
		// Hanya tampilkan hasil pemeriksaan yang sebenarnya. Sebelumnya nilai di
		// sini diturunkan dari skor kualitas media (q × 0.9) saat belum diperiksa,
		// sehingga angka yang tampil bukan hasil pemeriksaan apa pun.
		if (m.seo_ai_trust != null) return m.seo_ai_trust;
		return '—';
	}
	const fmt = (n: number) => n.toLocaleString('id-ID');
	const cleanDomain = (m: Media) =>
		m.website_url ? m.website_url.replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '') : m.id.slice(0, 8);
	// Nama tampil = domain (bukan nama brand), huruf pertama dibesarkan.
	const domainLabel = (m: Media) => mediaDomainLabel(m.website_url, m.name);
	// Domain untuk favicon stack di bar bawah (dipakai juga oleh entri yang
	// tidak sedang tampil, yang hanya menyimpan nama/website_url di snapshot).
	const domainOf = (website: string | null | undefined, fallback: string) =>
		(website ?? '').replace(/^https?:\/\/(www\.)?/i, '').replace(/\/+$/, '').trim() || fallback;
	// Favicon yang gagal dimuat jatuh ke inisial huruf (per id media).
	let faviconFailed = $state<Record<string, boolean>>({});
	// Rekam jejak distribusi media (jumlah artikel yang sudah terbit lewat media ini).
	const distribution = (m: Media) => mediaDistribution(m.health_breakdown, m.published_count);

	// ── Muat data ──────────────────────────────────────────────────────
	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	$effect(() => {
		const value = search;
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			debouncedSearch = value;
			void load();
		}, 300);
		return () => clearTimeout(searchTimer);
	});

	async function load() {
		err = '';
		const qs = new URLSearchParams({ limit: '250' });
		if (debouncedSearch) qs.set('q', debouncedSearch);
		const listUrl = `${apiBase}/media?${qs}`;
		// Data lama (<=10 menit) tampil seketika, refresh di latar.
		const cached = swrPeek<{ media?: Media[]; items?: Media[] }>(listUrl);
		if (cached) {
			rows = cached.media ?? cached.items ?? [];
			busy = false;
		} else {
			busy = true;
		}
		try {
			const res = await apiRequest(listUrl);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat media.')));
			const data = (await res.json()) as { media?: Media[]; items?: Media[] };
			rows = data.media ?? data.items ?? [];
			swrPut(listUrl, data);
		} catch (reason) {
			err = reason instanceof Error ? reason.message : tr('Gagal memuat media.');
		} finally {
			busy = false;
		}
	}

	/** Pulihkan pilihan sesi terakhir (badge + snapshot harga untuk total). */
	function restoreSelection() {
		try {
			const raw = localStorage.getItem(SELECTED_KEY);
			if (!raw) return;
			const items = JSON.parse(raw) as Array<{ id?: string; name?: string; price?: number; credits?: number }>;
			const valid = items.filter((x) => typeof x?.id === 'string' && x.id);
			if (!valid.length) return;
			selectedIds = new Set(valid.map((x) => x.id as string));
			selectedSnapshot = Object.fromEntries(
				valid.map((x) => [
					x.id as string,
					{ name: x.name ?? '', price: Number(x.price ?? 0) || 0, credits: Number(x.credits ?? 0) || 0, domain: domainOf(x.name ?? '', x.name ?? '') }
				])
			);
			restoredSelectionCount = valid.length;
		} catch {
			/* pilihan korup — abaikan */
		}
	}

	function clearSelection() {
		selectedIds = new Set();
		selectedSnapshot = {};
		restoredSelectionCount = 0;
		try { localStorage.removeItem(SELECTED_KEY); } catch { /* abaikan */ }
	}

	/** Mode "tambah ke kampanye": pra-pilih media yang sudah ada di kampanye
	 *  tujuan supaya klien melihat pilihannya dan tinggal menambah. */
	async function preselectCampaign() {
		if (!addToCampaignId) return;
		try {
			// Status kampanye menentukan boleh-tidaknya menambah media.
			const statusRes = await apiRequest(`${apiBase}/taut/campaigns/${addToCampaignId}`);
			if (statusRes.ok) {
				const campaign = (await statusRes.json()) as { taut_status?: string | null };
				campaignDone = DONE_STATUSES.has(campaign?.taut_status ?? '');
			}
			const res = await apiRequest(`${apiBase}/taut/campaigns/${addToCampaignId}/placement`);
			if (!res.ok) return;
			const data = (await res.json()) as {
				media?: Array<{ media_id?: string; name?: string; website_url?: string | null; credit_cost?: number | null; status?: string }>;
			};
			const items = (data.media ?? []).filter(
				(m) => (!m.status || m.status === 'selected') && typeof m.media_id === 'string'
			);
			if (!items.length) return;
			const ids = items.map((m) => m.media_id as string);
			selectedIds = new Set(ids);
			originalCampaignIds = new Set(ids);
			selectedSnapshot = Object.fromEntries(
				items.map((m) => {
					const id = m.media_id as string;
					const row = rows.find((x) => x.id === id);
					return [id, {
						name: mediaDomainLabel(m.website_url ?? null, m.name ?? ''),
						domain: domainOf(m.website_url, m.name ?? ''),
						price: row ? row.client_price ?? row.price ?? 0 : 0,
						credits: Number(m.credit_cost ?? 0) || 0
					}];
				})
			);
		} catch {
			/* gagal memuat pilihan kampanye — katalog tetap bisa dipakai */
		}
	}

	/** Simpan media tambahan ke kampanye yang SUDAH ada lalu kembali ke Penyedia
	 *  Konten. Hanya media baru yang dikirim (PUT placement {add}), jadi media
	 *  lama tidak tersentuh dan tidak ada kampanye baru yang dibuat. */
	async function addToCampaign() {
		if (!addToCampaignId) return;
		if (campaignDone) {
			addError = tr('Kampanye ini sudah selesai — mode lihat saja. Media tidak bisa ditambahkan lagi.');
			return;
		}
		const target = `/klien/${brandUrl}/taut-pintar/${addToCampaignId}/writer`;
		const toAdd = [...selectedIds].filter((id) => !originalCampaignIds.has(id));
		if (toAdd.length === 0) {
			await goto(target);
			return;
		}
		adding = true;
		addError = '';
		try {
			const res = await apiRequest(`${apiBase}/taut/campaigns/${addToCampaignId}/placement`, {
				method: 'PUT',
				body: JSON.stringify({ add: toAdd })
			});
			if (!res.ok) {
				addError = await readApiError(res, tr('Gagal menambah media ke kampanye.'));
				return;
			}
			clearSelection();
			await goto(target);
		} catch {
			addError = tr('Gagal menambah media ke kampanye.');
		} finally {
			adding = false;
		}
	}

	/** Saldo kredit untuk bar seleksi & ringkasan modal. Gagal baca bukan penghalang. */
	// Modal top up dimiliki layout klien. Halaman ini hanya meminta membukanya
	// lewat konteks, supaya alur pembayaran (submitTopUp/topupKey) tidak disalin
	// ke sini; callback-nya menyegarkan saldo yang tampil di bar bawah.
	const brandTopUp = getContext<{ open: (onClosed?: () => void) => void } | undefined>('brandTopUp');

	// Klik biasa membuka modal di tempat; klik dengan tombol pengubah tetap
	// membuka /credits seperti sebelumnya sehingga tautannya tidak pernah jadi
	// kontrol mati.
	function openTopUpFromBar(e: MouseEvent) {
		if (!brandTopUp || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
		e.preventDefault();
		brandTopUp.open(() => void loadWallet());
	}

	async function loadWallet() {
		try {
			const res = await apiRequest(`${apiBase}/credits?limit=1`);
			if (!res.ok) return;
			const d = (await res.json()) as { wallet?: { available_tc?: number; rate_idr?: number }; tc_config?: { rate_idr?: number } };
			const tc = Number(d?.wallet?.available_tc);
			if (Number.isFinite(tc)) walletAvailableTc = tc;
			const rate = Number(d?.tc_config?.rate_idr ?? d?.wallet?.rate_idr);
			if (Number.isFinite(rate) && rate > 0) walletRateIdr = rate;
		} catch {
			/* saldo tetap tak diketahui — jangan blokir katalog */
		}
	}

	// Simpan pilihan ke localStorage (pola BOOKMARK_KEY) supaya orang sibuk bisa
	// lanjut besok tanpa memilih ulang.
	$effect(() => {
		// Mode tambah ke kampanye tidak menulis draft ke localStorage: pilihan di
		// situ berasal dari kampanye, bukan draft baru, jadi jangan bocor ke
		// kampanye berikutnya.
		if (!selectionHydrated || addToCampaignId) return;
		const ids = [...selectedIds];
		try {
			if (ids.length === 0) {
				localStorage.removeItem(SELECTED_KEY);
				return;
			}
			const items = ids.map((id) => ({ id, ...(selectedSnapshot[id] ?? { name: '', price: 0, credits: 0 }) }));
			localStorage.setItem(SELECTED_KEY, JSON.stringify(items));
		} catch {
			/* penyimpanan penuh/diblokir — abaikan */
		}
	});

	/** Buka modal konfirmasi: reset state, tarik saldo, lalu cek pesanan ganda. */
	async function openCampaignModal(presetName = '') {
		campaignName = presetName;
		campaignError = '';
		profilerId = '';
		campaignOpen = true;
		if (walletAvailableTc === null) void loadWallet();
		if (SHOW_PROFILER_FIELD && profilerOptions.length === 0) void loadProfilerOptions();
	}

	/** Daftar Profiler milik klien yang login (untuk dipilih saat simpan kampanye). */
	async function loadProfilerOptions() {
		profilerLoading = true;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/profiler/profiles`);
			if (!res.ok) return;
			const data = await res.json().catch(() => null);
			const items = Array.isArray(data?.profiles) ? data.profiles : [];
			profilerOptions = items.map((p: { id: string; name: string }) => ({ value: p.id, label: p.name }));
		} finally {
			profilerLoading = false;
		}
	}

	/** Media terpilih yang sudah dipesan di kampanye aktif lain (peringatan, bukan blokir). */
	async function findReservations(): Promise<Array<{ mediaName: string; campaignName: string }>> {
		if (selectedIds.size === 0) return [];
		const ACTIVE = new Set(['profiled', 'profiling', 'placement', 'production', 'review', 'scheduled']);
		try {
			const res = await apiRequest(`${apiBase}/taut/campaigns?page=1`);
			if (!res.ok) return [];
			const data = (await res.json()) as { campaigns?: Array<{ id: string; name: string; taut_status?: string | null }> };
			const candidates = (data.campaigns ?? []).filter((c) => ACTIVE.has(c.taut_status ?? '')).slice(0, 10);
			const hits: Array<{ mediaName: string; campaignName: string }> = [];
			await Promise.all(
				candidates.map(async (c) => {
					const r = await apiRequest(`${apiBase}/taut/campaigns/${c.id}/placement`);
					if (!r.ok) return;
					const d = (await r.json().catch(() => null)) as { media?: Array<{ media_id?: string }> } | null;
					for (const item of d?.media ?? []) {
						const id = item.media_id;
						if (!id || !selectedIds.has(id)) continue;
						const m = rows.find((x) => x.id === id);
						hits.push({ mediaName: m ? domainLabel(m) : 'Media', campaignName: c.name });
					}
				})
			);
			return hits.slice(0, 6);
		} catch {
			return []; // cek gagal bukan penghalang order
		}
	}

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) {
				await goto('/login');
				return;
			}
			try {
				const raw = localStorage.getItem(BOOKMARK_KEY);
				if (raw) bookmarkedIds = new Set(JSON.parse(raw) as string[]);
			} catch { /* bookmark korup — abaikan */ }
			// Muat media dulu supaya snapshot harga untuk pra-pilih kampanye bisa
			// dibaca dari daftar (bar seleksi tidak menampilkan total yang salah).
			addToCampaignId = page.url.searchParams.get('campaign');
			await load();
			if (addToCampaignId) await preselectCampaign();
			else restoreSelection();
			selectionHydrated = true;
			void loadWallet();
		})();
		const onExpired = () => void goto('/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	// Header kolom tabel sticky tepat di bawah topbar aplikasi. Tinggi topbar
	// berubah (mis. pill tahapan membungkus di layar sempit), jadi offset-nya
	// diukur dari elemen asli, bukan angka tetap.
	$effect(() => {
		if (typeof window === 'undefined') return;
		const sync = () => {
			const bar = document.querySelector('.app-utility-bar');
			const h = bar ? Math.round(bar.getBoundingClientRect().height) : 0;
			document.documentElement.style.setProperty('--media-sticky-top', (h > 0 ? h : 60) + 'px');
		};
		sync();
		window.addEventListener('resize', sync);
		const ro = new ResizeObserver(sync);
		const bar = document.querySelector('.app-utility-bar');
		if (bar) ro.observe(bar);
		return () => {
			window.removeEventListener('resize', sync);
			ro.disconnect();
		};
	});

	function persistBookmarks() {
		try {
			localStorage.setItem(BOOKMARK_KEY, JSON.stringify([...bookmarkedIds]));
		} catch { /* penyimpanan penuh — abaikan */ }
	}
	function toggleBookmark(id: string, e: Event) {
		e.stopPropagation();
		const next = new Set(bookmarkedIds);
		if (next.has(id)) {
			// Dihapus: cukup transisi warna/scale, tanpa animasi terbang.
			next.delete(id);
			bookmarkedIds = next;
			persistBookmarks();
			return;
		}
		next.add(id);
		bookmarkedIds = next;
		persistBookmarks();
		// Ditambahkan: pop pada ikon sumber + chip terbang ke tombol Bookmark.
		const btn = e.currentTarget as HTMLElement | null;
		popBookmark(btn);
		flyToBookmark(btn, 'Bookmark');
	}

	/** Pop singkat + isi (filled) pada ikon sumber saat bookmark ditambahkan. */
	function popBookmark(btn: HTMLElement | null) {
		if (!motionOK || !btn) return;
		btn.classList.remove('bookmarkPop');
		// Paksa reflow agar animasi bisa dipicu ulang pada klik beruntun.
		void btn.offsetWidth;
		btn.classList.add('bookmarkPop');
		window.setTimeout(() => btn.classList.remove('bookmarkPop'), 420);
	}
	/** Buang satu media dari pilihan (dipakai popover daftar media terpilih). */
	function removeSelected(id: string) {
		const next = new Set(selectedIds);
		next.delete(id);
		const rest = { ...selectedSnapshot };
		delete rest[id];
		selectedIds = next;
		selectedSnapshot = rest;
		if (next.size === 0) selectedListOpen = false;
	}

	function toggleSelectRow(id: string, e: Event) {
		e.stopPropagation();
		if (archivedIds.has(id)) return; // media maintenance tidak bisa dipilih
		const next = new Set(selectedIds);
		if (next.has(id)) {
			next.delete(id);
			const rest = { ...selectedSnapshot };
			delete rest[id];
			selectedSnapshot = rest;
		} else {
			next.add(id);
			const m = rows.find((x) => x.id === id);
			if (m) {
				selectedSnapshot = {
					...selectedSnapshot,
					[id]: { name: domainLabel(m), price: m.client_price ?? m.price ?? 0, credits: m.credit_cost ?? 0 }
				};
			}
			// Feedback visual: highlight baris + chip terbang ke tombol Kampanye.
			flashRow(id);
			flyToCampaign(e.currentTarget as HTMLElement | null, m ? domainLabel(m) : 'Media');
		}
		selectedIds = next;
	}

	/**
	 * Animasi chip yang terbang dari elemen asal (checkbox/baris) ke tombol
	 * "Buat Kampanye" di floating action bar. Memakai Web Animations API dengan
	 * lintasan melengkung (3 keyframe), aman dipanggil berulang, dan elemen
	 * sementaranya selalu dibersihkan setelah selesai.
	 */
	function flyToCampaign(anchor: HTMLElement | null, label: string) {
		if (!motionOK || !anchor || !flyLayer) return;
		const start = anchor.getBoundingClientRect();
		const target = createBtnEl?.getBoundingClientRect();
		// Bar muncul setelah state berubah; ukur ulang pada frame berikutnya bila
		// tombol tujuan belum ada (mis. pemilihan media pertama).
		const run = (end: DOMRect | undefined) => {
			if (!end || end.width === 0) return;
			const chip = document.createElement('div');
			chip.className = 'flyChip';
			chip.setAttribute('aria-hidden', 'true');
			chip.innerHTML = `<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/></svg><span></span>`;
			const labelEl = chip.querySelector('span');
			if (labelEl) labelEl.textContent = label.length > 18 ? label.slice(0, 17) + '…' : label;
			flyLayer!.appendChild(chip);

			const sx = start.left + start.width / 2 - 12;
			const sy = start.top + start.height / 2 - 12;
			const ex = end.left + end.width / 2 - 12;
			const ey = end.top + end.height / 2 - 12;
			const mx = (sx + ex) / 2;
			// Puncak lengkungan: naik di atas kedua ujung agar lintasan terlihat busur.
			const my = Math.min(sy, ey) - 110;

			const anim = chip.animate(
				[
					{ transform: `translate(${sx}px, ${sy}px) scale(1)`, opacity: 1 },
					{ transform: `translate(${mx}px, ${my}px) scale(0.94)`, opacity: 0.98, offset: 0.45 },
					{ transform: `translate(${ex}px, ${ey}px) scale(0.28)`, opacity: 0 }
				],
				{ duration: 880, easing: 'cubic-bezier(0.33, 0.05, 0.28, 1)' }
			);
			const cleanup = () => chip.remove();
			anim.onfinish = () => {
				cleanup();
				pulseCreateButton();
			};
			anim.oncancel = cleanup;
		};

		if (createBtnEl) {
			run(target);
		} else {
			requestAnimationFrame(() => run(createBtnEl?.getBoundingClientRect()));
		}
	}

	function pulseCreateButton() {
		if (!motionOK) return;
		barPulse = false;
		requestAnimationFrame(() => {
			barPulse = true;
			window.setTimeout(() => (barPulse = false), 560);
		});
	}

	/** Pulse singkat pada tombol Bookmark di toolbar saat chip tiba. */
	function pulseBookmarkButton() {
		if (!motionOK) return;
		bookmarkPulse = false;
		requestAnimationFrame(() => {
			bookmarkPulse = true;
			window.setTimeout(() => (bookmarkPulse = false), 520);
		});
	}

	/**
	 * Animasi chip ikon bookmark terbang dari baris media menuju tombol Bookmark
	 * di toolbar kanan atas. Lintasan melengkung ke atas, elemen sementara
	 * dibersihkan setelah selesai, dan aman untuk klik beruntun.
	 */
	function flyToBookmark(anchor: HTMLElement | null, label: string) {
		if (!motionOK || !anchor || !flyLayer) return;
		const start = anchor.getBoundingClientRect();
		const run = (end: DOMRect | undefined) => {
			if (!end || end.width === 0) return;
			const chip = document.createElement('div');
			chip.className = 'flyChip flyChipBookmark';
			chip.setAttribute('aria-hidden', 'true');
			chip.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 3a2 2 0 0 1 2 2v15a1 1 0 0 1-1.496.868l-4.512-2.578a2 2 0 0 0-1.984 0l-4.512 2.578A1 1 0 0 1 5 20V5a2 2 0 0 1 2-2z"/></svg>';
			chip.title = label;
			flyLayer!.appendChild(chip);

			const sx = start.left + start.width / 2 - 11;
			const sy = start.top + start.height / 2 - 11;
			const ex = end.left + end.width / 2 - 11;
			const ey = end.top + end.height / 2 - 11;
			const mx = (sx + ex) / 2;
			const my = Math.min(sy, ey) - 70;

			const anim = chip.animate(
				[
					{ transform: `translate(${sx}px, ${sy}px) scale(0.9)`, opacity: 1 },
					{ transform: `translate(${sx}px, ${sy - 10}px) scale(1.18)`, opacity: 1, offset: 0.18 },
					{ transform: `translate(${mx}px, ${my}px) scale(1.02)`, opacity: 0.98, offset: 0.55 },
					{ transform: `translate(${ex}px, ${ey}px) scale(0.32)`, opacity: 0 }
				],
				{ duration: 600, easing: 'cubic-bezier(0.34, 0.06, 0.26, 1)' }
			);
			const cleanup = () => chip.remove();
			anim.onfinish = () => {
				cleanup();
				pulseBookmarkButton();
			};
			anim.oncancel = cleanup;
		};
		if (bookmarkToolbarBtn) {
			run(bookmarkToolbarBtn.getBoundingClientRect());
		} else {
			// Tombol tujuan tidak ada → state tetap berubah, tanpa animasi terbang.
			requestAnimationFrame(() => run(bookmarkToolbarBtn?.getBoundingClientRect()));
		}
	}

	/** Highlight biru halus pada baris yang baru dipilih (≈300 ms). */
	function flashRow(id: string) {
		if (!motionOK) return;
		flashRowId = id;
		window.setTimeout(() => {
			if (flashRowId === id) flashRowId = null;
		}, 380);
	}

	// ── Filter & sort (parity useMemo legacy) ──────────────────────────
	const filtered = $derived(
		rows.filter((m) => {
			if (bookmarkOnly && !bookmarkedIds.has(m.id)) return false;
			if (category) {
				const cats = m.categories && m.categories.length ? m.categories : m.category ? [m.category] : [];
				if (!cats.some((c) => c.toLowerCase() === category.toLowerCase())) return false;
			}
			if (verifiedOnly && !m.domain_verified) return false;
			if (dewanPersOnly && !m.dewan_pers_verified) return false;
			if (speedFilter) {
				// Kelompok SLA dihitung dari rata-rata waktu terbit nyata media.
				if (slaBucket(avgPublishHours(m)) !== speedFilter) return false;
			}
			if (priceActive) {
				const price = m.client_price ?? m.price ?? 0;
				// Media tanpa harga pasti hanya lolos bila opsi "harga nego /
				// menyusul" dicentang; selain itu tidak ada angka untuk dibandingkan.
				const pending = !(price > 0);
				if (!(includePricePending && pending)) {
					if (priceRange.lo != null && price < priceRange.lo) return false;
					if (priceRange.hi != null && price > priceRange.hi) return false;
				}
			}
			if (minDa > 0) {
				const da = getDa(m);
				if (typeof da !== 'number' || da < minDa) return false;
			}
			if (autoPublishOnly && m.auto_publish !== true) return false;
			if (aiCitationOnly && m.seo_ai_citation !== true) return false;
			return true;
		})
	);

	const sorted = $derived.by(() => {
		// Mode rotasi: urutan acak-deterministik dari seed halaman ini.
		if (sortKey === 'acak') return seededShuffle(filtered, shuffleSeed);
		const list = [...filtered];
		const dir = sortDir === 'asc' ? 1 : -1;
		const num = (v: string | number) => (typeof v === 'number' ? v : -1);
		list.sort((a, b) => {
			let av: number | string = 0;
			let bv: number | string = 0;
			switch (sortKey) {
				case 'traffic': av = a.sw_visits ?? a.traffic_estimate ?? 0; bv = b.sw_visits ?? b.traffic_estimate ?? 0; break;
				case 'da': av = num(getDa(a)); bv = num(getDa(b)); break;
				case 'ai_trust': av = num(getAiTrust(a)); bv = num(getAiTrust(b)); break;
				case 'ai_cited': av = a.seo_ai_citation ? 1 : 0; bv = b.seo_ai_citation ? 1 : 0; break;
				// SLA: makin kecil jamnya makin cepat. Dibalik agar arah "desc"
				// (klik pertama) menampilkan yang paling cepat lebih dulu; media
				// tanpa data ditaruh paling akhir.
				case 'speed': av = slaSortKey(a); bv = slaSortKey(b); break;
				case 'price': av = a.client_price ?? a.price ?? 0; bv = b.client_price ?? b.price ?? 0; break;
				default: av = domainLabel(a).toLowerCase(); bv = domainLabel(b).toLowerCase();
			}
			if (av < bv) return -1 * dir;
			if (av > bv) return 1 * dir;
			return domainLabel(a).localeCompare(domainLabel(b), 'id');
		});
		return list;
	});

	const totalPages = $derived(Math.max(1, Math.ceil(sorted.length / pageSize)));
	const visible = $derived(sorted.slice((page_ - 1) * pageSize, page_ * pageSize));
	$effect(() => { if (page_ > totalPages) page_ = 1; });

	const categories = $derived.by(() => {
		const map = new Map<string, number>();
		for (const m of rows) {
			const cats = m.categories && m.categories.length ? m.categories : m.category ? [m.category] : [];
			for (const c of cats) map.set(c, (map.get(c) ?? 0) + 1);
		}
		return [...map.entries()].sort((a, b) => b[1] - a[1]);
	});

	const activeFilterCount = $derived(
		(speedFilter ? 1 : 0) + (priceActive ? 1 : 0) + (priceActive && includePricePending ? 1 : 0) + (minDa > 0 ? 1 : 0) +
		(autoPublishOnly ? 1 : 0) + (aiCitationOnly ? 1 : 0) +
		(verifiedOnly ? 1 : 0) + (dewanPersOnly ? 1 : 0) + (category ? 1 : 0)
	);
	const selectedMedia = $derived(rows.filter((m) => selectedIds.has(m.id)));
	// Media terpilih yang tidak ada di baris termuat (filter/sort lain) — harganya
	// dari snapshot tersimpan supaya total tidak bohong.
	const offViewSelected = $derived([...selectedIds].filter((id) => !selectedMedia.some((m) => m.id === id)));
	const totalSelectedPrice = $derived(
		selectedMedia.reduce((s, m) => s + (m.client_price ?? m.price ?? 0), 0) +
			offViewSelected.reduce((s, id) => s + (selectedSnapshot[id]?.price ?? 0), 0)
	);
	const totalSelectedCredits = $derived(
		selectedMedia.reduce((s, m) => s + (m.credit_cost ?? 0), 0) +
			offViewSelected.reduce((s, id) => s + (selectedSnapshot[id]?.credits ?? 0), 0)
	);

	// Daftar media terpilih untuk popover: nama + harga. Baris yang sedang tampil
	// dipakai langsung; yang di luar tampilan memakai snapshot tersimpan.
	const selectedItems = $derived(
		[...selectedIds].map((id) => {
			const row = selectedMedia.find((m) => m.id === id);
			return {
				id,
				name: row ? domainLabel(row) : (selectedSnapshot[id]?.name ?? id),
				domain: row ? domainOf(row.website_url, domainLabel(row)) : (selectedSnapshot[id]?.domain ?? domainOf(selectedSnapshot[id]?.name, selectedSnapshot[id]?.name ?? id)),
				price: row ? (row.client_price ?? row.price ?? 0) : (selectedSnapshot[id]?.price ?? 0)
			};
		})
	);

	function toggleSort(key: typeof sortKey) {
		if (sortKey === key) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortKey = key;
			sortDir = 'desc';
		}
	}
	const sortIndicator = (key: string) => (sortKey !== key ? ' ↕' : sortDir === 'asc' ? ' ↑' : ' ↓');

	function resetFilters() {
		speedFilter = '';
		priceMinText = '';
		priceMaxText = '';
		includePricePending = false;
		minDa = 0;
		autoPublishOnly = false;
		aiCitationOnly = false;
		verifiedOnly = false;
		dewanPersOnly = false;
		category = '';
	}
	function toggleSelectAll(e: Event) {
		e.stopPropagation();
		// Pilih semua melewati media maintenance (terarsip).
		const ids = visible.filter((m) => !archivedIds.has(m.id)).map((m) => m.id);
		const allSelected = ids.length > 0 && ids.every((id) => selectedIds.has(id));
		if (allSelected) {
			selectedIds = new Set();
			selectedSnapshot = {};
			return;
		}
		selectedIds = new Set(ids);
		const snap: Record<string, { name: string; price: number; credits: number }> = {};
		for (const m of visible) {
			if (archivedIds.has(m.id)) continue;
			snap[m.id] = { name: domainLabel(m), price: m.client_price ?? m.price ?? 0, credits: m.credit_cost ?? 0 };
		}
		selectedSnapshot = snap;
	}

	function exportProposal() {
		// AI Trust & GEO (AI Cited) digabung jadi satu kolom: skor trust dulu,
		// lalu status GEO di belakangnya dengan tanda "|".
		const headers = ['Nama Media', 'Website', 'Kategori', 'SLA Terbit', 'DA', 'AI Trust / GEO', tr('Harga IDR')];
		const esc = (v: unknown) => `"${String(v ?? '').replace(/"/g, '""')}"`;
		// Pemisah ";" dipakai karena Excel/Sheets ber-locale Indonesia (dan Eropa)
		// memakai titik-koma sebagai list separator; CSV berkoma tampil menyatu
		// jadi satu kolom di aplikasi sheet tersebut.
		const lines = selectedMedia.map((m) => {
			const price = m.client_price ?? m.price ?? 0;
			return [
				esc(m.name), esc(m.website_url ?? ''), esc(m.category ?? 'Umum'),
				esc(getSla(m)), esc(getDa(m)),
				// Satu sel gabungan: skor trust + status GEO (AI Cited). Tiga keadaan
				// GEO: Ya / Tidak / belum bisa ditentukan (Google tidak menampilkan
				// AI Overview) — jangan ditulis "Tidak" untuk yang belum dinilai.
				esc(`${getAiTrust(m)} | GEO:${m.seo_ai_citation === true ? 'Ya' : m.seo_ai_citation === false ? 'Tidak' : '—'}`),
				esc(price)
			].join(';');
		});
		// BOM + CRLF supaya Excel membaca UTF-8 dan akhir baris dengan benar.
		const csv = '\uFEFF' + [headers.map(esc).join(';'), ...lines].join('\r\n');
		const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `proposal-media-${new Date().toISOString().slice(0, 10)}.csv`;
		a.click();
		URL.revokeObjectURL(url);
	}

	/** Baca file .csv/.txt: hasil ekspor proposal otomatis dikenali (kolom
	 *  pertama = nama media). Parity handleFileUpload app lama. */
	async function handleFileUpload(e: Event) {
		importError = '';
		const file = (e.target as HTMLInputElement).files?.[0];
		if (!file) return;
		try {
			importText = await file.text();
		} catch {
			importError = tr('Gagal membaca file.');
		}
	}

	/**
	 * Ambil sel pertama sebuah baris CSV/TSV. Hasil ekspor proposal memakai
	 * pemisah ";" dan mengutip nilai, sedangkan daftar tempel bebas hanya berisi
	 * satu domain per baris — keduanya terbaca di sini.
	 */
	function firstCsvCell(line: string): string {
		const s = line.trim();
		if (s.startsWith('"')) {
			let out = '';
			for (let i = 1; i < s.length; i++) {
				if (s[i] === '"') {
					if (s[i + 1] === '"') {
						out += '"';
						i++;
						continue;
					}
					break;
				}
				out += s[i];
			}
			return out;
		}
		return s.split(/[;,\t]/)[0].trim();
	}

	function applyImport() {
		importError = '';
		const lines = importText.split(/\r?\n/).map((l) => l.trim()).filter(Boolean);
		if (!lines.length) {
			importError = tr('Tidak ada baris teks yang terbaca.');
			return;
		}
		const matched = new Set<string>();
		for (const line of lines) {
			const cell = firstCsvCell(line);
			if (!cell) continue;
			const needle = cell.toLowerCase().replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '');
			const hit = rows.find(
				(m) => cleanDomain(m).toLowerCase() === needle || m.name.toLowerCase() === needle || m.id === cell
			);
			if (hit) matched.add(hit.id);
		}
		if (!matched.size) {
			importError = tr('Tidak ada media yang cocok dengan daftar Anda.');
			return;
		}
		selectedIds = new Set([...selectedIds, ...matched]);
		importOpen = false;
		importText = '';
	}

	// Matchmaker hanya MENGISI pilihan katalog (sama seperti Impor List);
	// kampanye tetap dibuat lewat tombol yang sudah ada.
	function applyMatchmaker(items: Array<{ id: string; name: string; price: number; credits: number }>) {
		const snap: typeof selectedSnapshot = {};
		for (const it of items) snap[it.id] = { name: it.name, price: it.price, credits: it.credits };
		selectedIds = new Set(items.map((it) => it.id));
		selectedSnapshot = { ...selectedSnapshot, ...snap };
		matchmakerOpen = false;
	}

	function openMatchmakerDetail(id: string) {
		const hit = rows.find((m) => m.id === id);
		if (hit) {
			detailFromMatchmaker = true;
			detail = hit;
		}
	}

	// Detail dibuka dari Matchmaker: "Tambahkan" cukup mencentang media itu di
	// katalog lalu menutup kedua modal — modal kampanye tidak boleh terbuka di
	// belakang Matchmaker yang masih tampil.
	function pickFromMatchmakerDetail(m: Media) {
		const snap = selectedSnapshot[m.id] ?? {
			name: domainLabel(m),
			price: m.client_price ?? m.price ?? 0,
			credits: m.credit_cost ?? 0
		};
		selectedIds = new Set([...selectedIds, m.id]);
		selectedSnapshot = { ...selectedSnapshot, [m.id]: snap };
		detail = null;
		detailFromMatchmaker = false;
		matchmakerOpen = false;
	}

	function proceedFromDetail(m: Media) {
		const snap = selectedSnapshot[m.id] ?? {
			name: domainLabel(m),
			price: m.client_price ?? m.price ?? 0,
			credits: m.credit_cost ?? 0
		};
		selectedIds = new Set([...selectedIds, m.id]);
		selectedSnapshot = { ...selectedSnapshot, [m.id]: snap };
		detail = null;
		detailFromMatchmaker = false;
		// Mode tambah: cukup pilih medianya; penyimpanan lewat tombol
		// "Tambahkan ke Kampanye" di bar bawah, bukan modal kampanye baru.
		if (addToCampaignId) return;
		void openCampaignModal(`Kampanye ${domainLabel(m)}`);
	}

	async function createCampaign() {
		const name = campaignName.trim();
		if (name.length === 1) {
			campaignError = tr('Nama kampanye minimal 2 karakter — atau kosongkan untuk nama otomatis.');
			return;
		}
		campaignError = '';
		creating = true;
		try {
			const res = await apiRequest(`${apiBase}/taut/campaigns`, {
				method: 'POST',
				body: JSON.stringify({
					// Mode 'ai': kampanye dari katalog tetap bisa generate artikel
					// dengan AI (dulu dipaksa 'manual' sehingga tombol Generate
					// tak pernah muncul). Media terpilih tetap dipasang server.
					// Nama kosong → otomatis "Kampanye Baru #N" (ditomori server).
					name: name || 'Kampanye Baru',
					starts_at: new Date().toISOString(),
					entry_mode: 'ai',
					media_ids: [...selectedIds],
					profiler_id: profilerId || undefined
				})
			});
			if (res.ok) {
				const data = (await res.json()) as { id: string; public_id?: string | null };
				campaignOpen = false;
				campaignName = '';
				clearSelection();
				await goto(`/klien/${brandUrl}/taut-pintar/${campaignUrlId(data)}/writer`);
			} else {
				campaignError = await readApiError(res, tr('Gagal membuat kampanye. Coba lagi.'));
			}
		} catch {
			campaignError = tr('Gagal membuat kampanye. Coba lagi.');
		} finally {
			creating = false;
		}
	}
</script>

<PageTitle title={tr("Eksplorasi Media")} />

<Tooltip.Provider delayDuration={0} skipDelayDuration={300}>
<div class="katalogMedia">
	{#if err}<div class="errorBanner">{err}</div>{/if}

	{#if addToCampaignId}
		<div class="catalogContextBanner">
			<Plus size={14} />
			<span>{campaignDone ? tr('Kampanye ini sudah selesai — mode lihat saja. Katalog tetap bisa dijelajahi, tapi media tidak bisa ditambahkan lagi.') : tr('Kamu sedang menambah media ke kampanye yang sudah dibuat. Media yang sudah dipilih tetap tercentang — pilih tambahanmu, lalu klik "Tambahkan ke Kampanye" untuk kembali ke Penyedia Konten.')}</span>
		</div>
		{#if addError}<div class="errorBanner">{addError}</div>{/if}
	{/if}

	{#if page.url.searchParams.get('from') === 'new-campaign'}
		<div class="catalogContextBanner">
			<Zap size={14} />
			<span>{tr('Pilih media untuk kampanyemu — setelah ini kamu diarahkan ke produksi artikel: judul, cover, pemeriksaan mutu, lalu terbit ke media.')}</span>
		</div>
	{/if}

	<!-- Top Search & Filter Bar -->
	<div class="modernTopBar">
		<div class="modernSearchWrap">
			<Search size={18} class="modernSearchIcon" />
			<input class="modernSearchInput" placeholder={tr("Cari nama media, domain, atau niche...")} bind:value={search} />
		</div>
		{#if isNarrow}
			<div class="modernToolsWrap">
				<button
					class="modernToolsBtn {activeFilterCount > 0 || bookmarkOnly || category ? 'modernToolsBtnActive' : ''}"
					type="button"
					aria-expanded={toolsOpen}
					aria-label={tr('Buka alat katalog')}
					title={tr('Alat katalog: kategori, filter, impor, bookmark')}
					onclick={() => (toolsOpen = true)}
				>
					<!-- Ikon corong (funnel) untuk membuka panel alat katalog. -->
					<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" aria-hidden="true">
						<path fill="none" stroke="currentColor" stroke-width="2" d="m3 6l7 7v8h4v-8l7-7V3H3z" />
					</svg>
					{#if activeFilterCount + (bookmarkOnly ? 1 : 0) + (category ? 1 : 0) > 0}
						<span class="modernActionBadge">{activeFilterCount + (bookmarkOnly ? 1 : 0) + (category ? 1 : 0)}</span>
					{/if}
				</button>
			</div>
		{:else}
			<!-- Desktop: alat katalog di samping KANAN search box (kategori,
			     Filter, Impor List, Bookmark) — corong + panel kanan hanya mobile. -->
			<div class="modernActionsGroup catalogToolsInline">
				<!-- AI Matchmaker: di kiri pemilih kategori. -->
				<button class="modernActionBtn matchmakerBtn" type="button" onclick={() => (matchmakerOpen = true)}>
					<span>{tr('AI Matchmaker')}</span><span class="matchmakerNew">{tr('Baru')}</span>
				</button>
				<!-- Kategori desktop: satu tombol, diklik membuka dropdown-nya. -->
				<div class="modernSelectWrap toolsInlineCats">
					<Select
						bind:value={category}
						ariaLabel={tr('Kategori')}
						Icon={LayoutGrid}
						placeholder={tr('Semua Kategori')}
						minWidth={210}
						searchable={categories.length > 10}
						searchPlaceholder={tr('Cari kategori…')}
						onchange={() => (page_ = 1)}
						options={[
							{ value: '', label: tr('Semua Kategori'), count: rows.length },
							...categories.map(([name, count]) => ({ value: name, label: name, count }))
						]}
					/>
				</div>
				<div class="toolsInlineActions">
					<button
						bind:this={advBtn}
						class="modernActionBtn {activeFilterCount > 0 ? 'modernActionBtnActive' : ''}"
						type="button"
						aria-expanded={advOpen}
						onclick={openAdvFilter}
					>
						<SlidersHorizontal size={15} />
						<span>{tr('Filter')}</span>
						{#if activeFilterCount > 0}<span class="modernActionBadge">{activeFilterCount}</span>{/if}
					</button>
					<button class="modernActionBtn" type="button" onclick={() => (importOpen = true)}>
						<Upload size={15} /><span>{tr('Impor List')}</span>
					</button>
					<button
						class="modernActionBtn {bookmarkOnly ? 'modernActionBtnActive' : ''}"
						type="button"
						class:bookmarkPulse
						bind:this={bookmarkToolbarBtn}
						onclick={() => (bookmarkOnly = !bookmarkOnly)}
					>
						<Bookmark size={15} />
						<span>{tr('Bookmark')}</span>
						{#if bookmarkedIds.size > 0}<span class="modernActionBadge">{bookmarkedIds.size}</span>{/if}
					</button>
				</div>
			</div>
		{/if}
	</div>

	<!-- Alat katalog versi MOBILE: corong di toolbar membuka panel dari kanan. -->
	{#if isNarrow}
	<SlideOver open={toolsOpen} title={tr('Alat Katalog')} subtitle={tr('Kelola kategori, filter, impor daftar, dan media simpanan.')} closeLabel={tr('Tutup')} onClose={() => (toolsOpen = false)}>
		<div class="toolsPanel">
			<span class="toolsPanelLabel catRadioLabel">{tr('Kategori')}</span>
			{@render catRadios()}

			<div class="advFilterInline">{@render advFields()}</div>

			<button class="modernActionBtn matchmakerBtn" type="button" onclick={() => { toolsOpen = false; matchmakerOpen = true; }}>
				<span>{tr('AI Matchmaker')}</span><span class="matchmakerNew">{tr('Baru')}</span>
			</button>

			<button class="modernActionBtn" type="button" onclick={() => { toolsOpen = false; importOpen = true; }}>
				<Upload size={15} /><span>{tr('Impor List')}</span>
			</button>

			<button
				class="modernActionBtn {bookmarkOnly ? 'modernActionBtnActive' : ''}"
				type="button"
				class:bookmarkPulse
				bind:this={bookmarkToolbarBtn}
				onclick={() => (bookmarkOnly = !bookmarkOnly)}
			>
				<Bookmark size={15} />
				<span>{tr('Bookmark')}</span>
				{#if bookmarkedIds.size > 0}<span class="modernActionBadge">{bookmarkedIds.size}</span>{/if}
			</button>
		</div>
		</SlideOver>
	{/if}

	<!-- Info media (mobile): kolom yang disembunyikan di tabel, dibuka dari tombol Aksi. -->
	<SlideOver
		open={quickInfo !== null}
		title={quickInfo ? domainLabel(quickInfo) : tr('Info Media')}
		subtitle={tr('Rincian yang tidak ditampilkan di tabel versi HP.')}
		closeLabel={tr('Tutup')}
		onClose={() => (quickInfo = null)}
	>
		{#if quickInfo}
			{@const qi = quickInfo}
			{@const qtraffic = formatTraffic(qi.sw_visits, qi.traffic_estimate)}
			{@const qcats = qi.categories && qi.categories.length ? qi.categories : qi.category ? [qi.category] : ['Umum']}
			{@const qsla = getSla(qi)}
			<div class="quickInfoPanel">
				<div class="quickInfoRow">
					<span>{tr('Kategori')}</span>
					<span class="quickInfoCats">
						{#each qcats as c (c)}<span class="categoryPill categoryPill--{getCategoryTone(c)}">{c}</span>{/each}
					</span>
				</div>
				<div class="quickInfoRow"><span>{tr('Trafik (est)')}</span><strong>{qtraffic.text}</strong></div>
				<div class="quickInfoRow"><span>DA</span><strong>{qi.da ?? '—'}</strong></div>
				<div class="quickInfoRow"><span>{tr('AI Trust')}</span><strong>{qi.ai_trust_score ?? '—'}</strong></div>
				<div class="quickInfoRow"><span>GEO</span><strong>{qi.ai_cited === true ? tr('Dikutip') : qi.ai_cited === false ? tr('Belum dikutip') : '—'}</strong></div>
				<div class="quickInfoRow"><span>{tr('SLA')}</span><strong>{qsla}</strong></div>
				<div class="quickInfoRow">
					<span>{tr('Bookmark')}</span>
					<button
						class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
						type="button"
						class:app-btn-primary={bookmarkedIds.has(qi.id)}
						onclick={(e) => toggleBookmark(qi.id, e)}
					><Bookmark size={13} /> {bookmarkedIds.has(qi.id) ? tr('Tersimpan') : tr('Simpan')}</button>
				</div>
			</div>
		{/if}
	</SlideOver>

{#snippet catRadios()}
	<div class="catRadioGroup" role="radiogroup" aria-label={tr('Kategori')}>
		<button
			type="button"
			role="radio"
			aria-checked={category === ''}
			class="catRadio"
			class:catRadioActive={category === ''}
			onclick={() => { category = ''; page_ = 1; }}
		>{tr('Semua')} <span class="catRadioCount">{rows.length}</span></button>
		{#each (catExpand ? categories : categories.slice(0, 5)) as [name, count] (name)}
			<button
				type="button"
				role="radio"
				aria-checked={category === name}
				class="catRadio"
				class:catRadioActive={category === name}
				onclick={() => { category = name; page_ = 1; }}
			>{name} <span class="catRadioCount">{count}</span></button>
		{/each}
		{#if categories.length > 5}
			<button type="button" class="catExpandBtn" onclick={() => (catExpand = !catExpand)}>
				{catExpand ? tr('Lebih sedikit') : `${tr('Lihat semua')} (${categories.length})`}
			</button>
		{/if}
	</div>
{/snippet}

{#snippet advFields()}
				<div class="advFilterSection">
					<div class="advFilterSectionHead">
						<Clock size={15} />
						<span class="advFilterSectionLabel">{tr('SLA Waktu Terbit')}</span>
					</div>
					<div class="advFilterChipsGroup advFilterChipsGroupEqual">
						{#each [{ v: 'super', l: '< 3 jam' }, { v: 'fast', l: '≤ 24 jam' }, { v: 'standard', l: '> 24 jam' }] as opt (opt.v)}
							<button
								class="advFilterChip {speedFilter === opt.v ? 'advFilterChipActive' : ''}"
								type="button"
								onclick={() => { speedFilter = speedFilter === opt.v ? '' : (opt.v as typeof speedFilter); page_ = 1; }}
							>{opt.l}</button>
						{/each}
					</div>
				</div>

				<div class="advFilterSection">
					<div class="advFilterSectionHead">
						<ShieldCheck size={15} />
						<span class="advFilterSectionLabel">{tr('Status Verifikasi')}</span>
					</div>
					<div class="advFilterToggleRow">
						<label class="advFilterToggle">
							<input type="checkbox" bind:checked={verifiedOnly} onchange={() => (page_ = 1)} />
							<span>{tr('Domain terverifikasi')}</span>
						</label>
						<label class="advFilterToggle">
							<input type="checkbox" bind:checked={dewanPersOnly} onchange={() => (page_ = 1)} />
							<span>{tr('Dewan Pers')}</span>
						</label>
					</div>
				</div>

				<div class="advFilterSection">
					<div class="advFilterSectionHead">
						<Lock size={15} />
						<span class="advFilterSectionLabel">{tr('Secure Direct')}</span>
					</div>
					<label class="advFilterToggle" title={tr("Artikel dari klien langsung diterbitkan ke media tanpa persetujuan manual Mitra.")}>
						<input type="checkbox" bind:checked={autoPublishOnly} onchange={() => (page_ = 1)} />
						<span>{tr('Secure Direct')}</span>
					</label>
				</div>

				<div class="advFilterSection">
					<div class="advFilterSectionHead">
						<Sparkles size={15} />
						<span class="advFilterSectionLabel">{tr('AI Citation')}</span>
					</div>
					<label class="advFilterToggle" title={tr("Indikator otomatis via API apakah media ini terdeteksi aktif sebagai rujukan/referensi oleh mesin AI.")}>
						<input type="checkbox" bind:checked={aiCitationOnly} onchange={() => (page_ = 1)} />
						<span>{tr('Dikutip AI Overview')}</span>
					</label>
				</div>

				<div class="advFilterSection">
					<div class="advFilterSectionHead">
						<Tag size={15} />
						<span class="advFilterSectionLabel">{tr('Rentang Harga')}</span>
					</div>
					<div class="priceRangeRow">
						<input
							class="priceRangeInput"
							type="text"
							inputmode="numeric"
							placeholder={tr('Rp Min')}
							bind:value={priceMinText}
							oninput={() => (page_ = 1)}
							onblur={normalizePriceInputs}
						/>
						<span class="priceRangeDash">–</span>
						<input
							class="priceRangeInput"
							type="text"
							inputmode="numeric"
							placeholder={tr('Rp Maks')}
							bind:value={priceMaxText}
							oninput={() => (page_ = 1)}
							onblur={normalizePriceInputs}
						/>
					</div>
					{#if priceActive}
						<label class="advFilterToggle advFilterToggleCompact" title={tr('Ikut tampilkan media yang harganya belum pasti (nego / menyusul).')}>
							<input type="checkbox" bind:checked={includePricePending} onchange={() => (page_ = 1)} />
							<span>{tr('Sertakan harga nego / menyusul')}</span>
						</label>
					{/if}
				</div>

				<div class="advFilterSection">
					<div class="advFilterSectionHead">
						<BarChart3 size={15} />
						<span class="advFilterSectionLabel">{tr('Minimum DA')}</span>
					</div>
					<div class="advFilterSliderRow">
						<input
							class="advFilterRange"
							type="range"
							min="0"
							max="100"
							step="1"
							bind:value={minDa}
							oninput={() => (page_ = 1)}
							style="--pct: {minDa}%"
							aria-label={tr("Minimum DA")}
						/>
						<span class="advFilterRangeValue">{minDa > 0 ? `DA ≥ ${minDa}` : tr('Semua DA')}</span>
					</div>
				</div>

{/snippet}

	<!-- Panel filter lanjutan — mengambang di bawah tombol Filter, tidak menggeser tabel -->
	{#if advOpen}
		<button
			type="button"
			class="advFilterBackdrop"
			aria-label={tr("Tutup filter")}
			onclick={() => (advOpen = false)}
		></button>
		<div class="advFilterPanel" style="left: {advPos.left}px; top: {advPos.top}px; max-height: calc(100vh - {advPos.top}px - 12px);">
			<div class="advFilterCard">
				<div class="advFilterHeader">
					<div class="advFilterTitle">
						<span class="advFilterTitleIcon"><SlidersHorizontal size={16} /></span>
						<span>{tr('Filter Lanjutan')}</span>
					</div>
					<button class="advFilterCloseBtn" type="button" onclick={() => (advOpen = false)} aria-label={tr("Tutup filter")}>
						<X size={16} />
					</button>
				</div>

				<div class="advFilterBody">
					{@render advFields()}
				</div>

				<div class="advFilterFooter">
					<button class="advFilterResetLink" type="button" onclick={resetFilters}>{tr('Reset semua')}</button>
					<button class="advFilterApplyBtn" type="button" onclick={() => (advOpen = false)}>{tr('Terapkan')}</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Banner bookmark aktif -->
	{#if bookmarkOnly}
		<div class="bookmarkFilterBanner">
			<span>{tr("Menampilkan")} {fmt(bookmarkedIds.size)} {tr("media tersimpan")}</span>
			<button class="bookmarkBannerClear" type="button" onclick={() => (bookmarkOnly = false)}>Tampilkan semua</button>
		</div>
	{/if}

	<!-- Ringkasan filter aktif -->
	{#if activeFilterCount > 0}
		<div class="activeFilterBanner">
			<span><b class="bannerStrong">{tr("Filter Aktif:")}</b> {activeFilterCount} · {fmt(filtered.length)} {tr("media cocok")}</span>
			<button class="bookmarkBannerClear" type="button" onclick={resetFilters}>Reset filter</button>
		</div>
	{/if}

	{#if restoredSelectionCount > 0 && selectedIds.size > 0}
		<div class="restoredSelectionBar">
			<span><strong>{restoredSelectionCount}</strong> {tr('media dipulihkan dari sesi terakhir — kamu bisa lanjut memilih atau langsung buat kampanye.')}</span>
			<button class="bookmarkBannerClear" type="button" onclick={clearSelection}>{tr('Bersihkan pilihan')}</button>
		</div>
	{/if}

	<!-- Tabel -->
	<div class="modernCard">
		{#if busy && rows.length === 0}
			<div class="skeletonList">
				{#each Array(6) as _, i (i)}
					<div class="skeletonRow">
						<div class="skeletonLine" style="width: 35%"></div>
						<div class="skeletonLine" style="width: 20%; height: 10px"></div>
						<div class="skeletonLine" style="width: 60px; height: 24px; border-radius: 6px"></div>
						<div class="skeletonLine" style="width: 50px; height: 24px; border-radius: 6px"></div>
					</div>
				{/each}
			</div>
		{:else if visible.length === 0}
			<div class="emptyState">
				{rows.length === 0
					? tr('Belum ada media aktif di katalog.')
					: tr('Tidak ada media yang cocok dengan filter ini.')}
			</div>
		{:else}
			<div class="tableWrap">
				<table class="modernTable">
					<thead>
						<tr>
							<th class="modernTh" style="width:36px;padding-right:0">
								<input
									type="checkbox"
									class="modernCheckbox"
									checked={visible.length > 0 && visible.every((m) => selectedIds.has(m.id))}
									onclick={toggleSelectAll}
									aria-label={tr("Pilih semua media di halaman ini")}
								/>
							</th>
							<th class="modernTh modernThSortable" onclick={() => toggleSort('name')}>{tr('Nama Media')}{sortIndicator('name')}</th>
							<th class="modernTh">{tr('Kategori')}</th>
							<th class="modernTh modernThSortable" onclick={() => toggleSort('traffic')}>
								<span class="thWithInfo">{tr('Trafik (Est)')}
									<Tooltip.Root>
										<Tooltip.Trigger>
											{#snippet child({ props })}
												<button {...props} type="button" class="infoDot" aria-label="Keterangan trafik" onclick={(e) => e.stopPropagation()}><Info size={14} /></button>
											{/snippet}
										</Tooltip.Trigger>
										<Tooltip.Portal>
											<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
											<Tooltip.Arrow class="uiTipArrow" />
											Perkiraan kunjungan bulanan situs media. Dari Similarweb bila tersedia; kalau tidak, dari estimasi internal media.
										</Tooltip.Content>
										</Tooltip.Portal>
									</Tooltip.Root>
								</span>{sortIndicator('traffic')}
							</th>
							<th class="modernTh modernThSortable" style="text-align:center" onclick={() => toggleSort('da')}>
								<span class="thWithInfo">DA
									<Tooltip.Root>
										<Tooltip.Trigger>
											{#snippet child({ props })}
												<button {...props} type="button" class="infoDot" aria-label="Keterangan DA" onclick={(e) => e.stopPropagation()}><Info size={14} /></button>
											{/snippet}
										</Tooltip.Trigger>
										<Tooltip.Portal>
											<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
											<Tooltip.Arrow class="uiTipArrow" />
											<strong>DA (Domain Authority)</strong> — skor 0–100 seberapa kuat dan berwibawa sebuah domain di mata mesin pencari. Makin tinggi, makin besar peluang artikel ikut terangkat di Google.
										</Tooltip.Content>
										</Tooltip.Portal>
									</Tooltip.Root>
								</span>{sortIndicator('da')}
							</th>
								<th class="modernTh modernThSortable" style="text-align:center" onclick={() => toggleSort('ai_trust')}>
									<span class="thWithInfo">{tr('GEO')}
										<Tooltip.Root>
											<Tooltip.Trigger>
												{#snippet child({ props })}
													<button {...props} type="button" class="infoDot" aria-label="Keterangan GEO dan AI Trust" onclick={(e) => e.stopPropagation()}><Info size={14} /></button>
												{/snippet}
											</Tooltip.Trigger>
											<Tooltip.Portal>
												<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
												<Tooltip.Arrow class="uiTipArrow" />
												<strong>GEO</strong> — centang biru menandai situs media ini disebut sebagai salah satu sumber pada jawaban AI Google.
												<br /><strong>AI Trust Score</strong> {tr("— Nilai 0-100: seberapa siap situs media ini dibaca dan dipercaya oleh mesin AI, dilihat dari kecepatan halaman dan kelengkapan data situsnya. Tanda — berarti kecepatannya belum bisa diukur.")}
											</Tooltip.Content>
											</Tooltip.Portal>
										</Tooltip.Root>
									</span>{sortIndicator('ai_trust')}
								</th>
								<th class="modernTh modernThSortable" onclick={() => toggleSort('speed')}>
								<span class="thWithInfo">{tr('SLA')}
									<Tooltip.Root>
										<Tooltip.Trigger>
											{#snippet child({ props })}
												<button {...props} type="button" class="infoDot" aria-label="Keterangan SLA waktu terbit" onclick={(e) => e.stopPropagation()}><Info size={14} /></button>
											{/snippet}
										</Tooltip.Trigger>
										<Tooltip.Portal>
											<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
											<Tooltip.Arrow class="uiTipArrow" />
											<span class="tipStack">
												<strong>SLA Waktu Terbit</strong>
												<span>{tr('Rata-rata waktu terbit NYATA media ini, dihitung dari riwayat publikasinya: dari artikel dikirim sampai tayang. Media yang biasanya terbit 1 jam setelah dikirim akan tampil "SLA ± 1 jam".')}</span>
												<span>{tr('Tanda — berarti belum ada riwayat penerbitan yang cukup untuk menghitung SLA-nya.')}</span>
											</span>
										</Tooltip.Content>
										</Tooltip.Portal>
									</Tooltip.Root>

								</span>{sortIndicator('speed')}
							</th>
							<th class="modernTh modernThSortable" onclick={() => toggleSort('price')}>{tr('Harga')}{sortIndicator('price')}</th>
							<th class="modernTh" style="text-align:right">{tr('Aksi')}</th>
						</tr>
					</thead>
					<tbody>
						{#each visible as m (m.id)}
							{@const traffic = formatTraffic(m.sw_visits, m.traffic_estimate)}
							{@const sla = getSla(m)}
							{@const cats = m.categories && m.categories.length ? m.categories : m.category ? [m.category] : ['Umum']}
							{@const dist = distribution(m)}
							<tr
								class="modernTr"
								class:rowJustSelected={flashRowId === m.id}
								style={archivedIds.has(m.id)
									? 'opacity:.55;filter:grayscale(.85);'
									: selectedIds.has(m.id) ? 'background:#f8faff' : ''}
								onclick={() => (detail = m)}
							>
								<td class="modernTd" style="width:36px;padding-right:0">
									<input
										type="checkbox"
										class="modernCheckbox"
										checked={selectedIds.has(m.id)}
										disabled={archivedIds.has(m.id)}
										title={archivedIds.has(m.id) ? tr('Media sedang maintenance') : undefined}
										onclick={(e) => toggleSelectRow(m.id, e)}
										aria-label={`Pilih ${m.name}`}
									/>
								</td>

								<td class="modernTd">
									<div class="mediaRowCol">
									<!-- Harga versi HP: kolom Harganya disembunyikan di layar kecil. -->
									<span class="priceMobile">Rp {fmt(m.client_price ?? m.price ?? 0)}</span>
										<div class="mediaTitleLine">
											<!-- Favicon situs media (Google s2; pola sama dengan bar
											     terpilih di bawah). Gagal memuat → dihilangkan. -->
											{#if m.website_url && !faviconFailed[m.id]}
												<img
													class="mediaFavicon"
													src={`https://www.google.com/s2/favicons?domain=${encodeURIComponent(domainOf(m.website_url, ''))}&sz=64`}
													alt=""
													loading="lazy"
													decoding="async"
													onerror={() => (faviconFailed = { ...faviconFailed, [m.id]: true })}
												/>
											{/if}
											<button
												type="button"
												class="mediaHeading mediaHeadingLink"
												title={`Lihat detail ${domainLabel(m)}`}
												onclick={(e) => { e.stopPropagation(); detail = m; }}
											>{domainLabel(m)}</button>
											{#if m.domain_verified}
												<Tooltip.Root>
											<Tooltip.Trigger>
												{#snippet child({ props })}
													<span {...props} class="verifiedTick" aria-label="Domain Terverifikasi">
													<svg width="15" height="15" viewBox="0 0 22 22" fill="none" aria-hidden="true">
													<path d="M20.396 11c-.018-.646-.215-1.275-.57-1.816-.354-.54-.852-.972-1.438-1.246.223-.607.27-1.264.14-1.897-.131-.634-.437-1.218-.882-1.687-.47-.445-1.053-.75-1.687-.882-.633-.13-1.29-.083-1.897.14-.273-.587-.704-1.086-1.245-1.44S11.647 1.62 11 1.604c-.646.017-1.273.213-1.813.568s-.969.854-1.24 1.44c-.608-.223-1.267-.272-1.902-.14-.635.13-1.22.436-1.69.882-.445.47-.749 1.055-.878 1.688-.13.633-.08 1.29.144 1.896-.587.274-1.087.705-1.443 1.245-.356.54-.555 1.17-.574 1.817.02.647.218 1.276.574 1.817.356.54.856.972 1.443 1.245-.224.606-.274 1.263-.144 1.896.13.634.433 1.218.877 1.688.47.443 1.054.747 1.687.878.633.132 1.29.084 1.897-.136.274.586.705 1.084 1.246 1.439.54.354 1.17.551 1.816.569.647-.016 1.276-.213 1.817-.567s.972-.854 1.245-1.44c.604.239 1.266.296 1.903.164.636-.132 1.22-.447 1.68-.907.46-.46.776-1.044.908-1.681s.075-1.299-.165-1.903c.586-.274 1.084-.705 1.439-1.246.354-.54.551-1.17.569-1.816z" fill="#1D9BF0"/>
													<path d="M9.662 14.85l-3.429-3.428 1.293-1.302 2.072 2.072 4.4-4.794 1.347 1.246z" fill="#FFFFFF"/>
													</svg>
													</span>
												{/snippet}
											</Tooltip.Trigger>
											<Tooltip.Portal>
												<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
												<Tooltip.Arrow class="uiTipArrow" />
												Domain terverifikasi Tauthub
											</Tooltip.Content>
											</Tooltip.Portal>
										</Tooltip.Root>
											{/if}
											{#if m.dewan_pers_verified}
												<Tooltip.Root>
													<Tooltip.Trigger>
														{#snippet child({ props })}
															<span {...props} class="verifiedTick dewanPersTick" aria-label="Terverifikasi Dewan Pers">
																<svg width="15" height="15" viewBox="0 0 16 16" fill="none" aria-hidden="true">
																	<path d="M0 0h16v16H0z" fill="none" />
																	<path fill="currentColor" fill-rule="evenodd" d="M8 0c-.69 0-1.843.265-2.928.56c-1.11.3-2.229.655-2.887.87a1.54 1.54 0 0 0-1.044 1.262c-.596 4.477.787 7.795 2.465 9.99a11.8 11.8 0 0 0 2.517 2.453c.386.273.744.482 1.048.625c.28.132.581.24.829.24s.548-.108.829-.24a7 7 0 0 0 1.048-.625a11.8 11.8 0 0 0 2.517-2.453c1.678-2.195 3.061-5.513 2.465-9.99a1.54 1.54 0 0 0-1.044-1.263a63 63 0 0 0-2.887-.87C9.843.266 8.69 0 8 0m2.146 5.146a.5.5 0 0 1 .708.708l-3 3a.5.5 0 0 1-.708 0l-1.5-1.5a.5.5 0 1 1 .708-.708L7.5 7.793z" />
																</svg>
															</span>
														{/snippet}
													</Tooltip.Trigger>
													<Tooltip.Portal>
														<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
															<Tooltip.Arrow class="uiTipArrow" />
															Terverifikasi Dewan Pers
														</Tooltip.Content>
													</Tooltip.Portal>
												</Tooltip.Root>
											{/if}
										</div>
										{#if m.contoh_artikel_url || dist.known || m.link_duration_months != null}
											<div class="mediaMetaLine">
												{#if m.contoh_artikel_url}
													<a
														href={m.contoh_artikel_url}
														target="_blank"
														rel="noreferrer nofollow"
														class="sampleDocLink"
														onclick={(e) => e.stopPropagation()}
													><FileText size={13} /><span>Sample</span></a>
												{/if}
												<!-- Kebijakan tautan media: kuota backlink + masa berlakunya
												     (permanen atau sekian bulan). -->
												{#if m.link_duration_months != null}
													<Tooltip.Root>
														<Tooltip.Trigger>
															{#snippet child({ props })}
																<span
																	{...props}
																	class="linkPolicyPill"
																	class:linkPolicyPillPermanent={isPermanentLink(m.link_duration_months)}
																>
																	<LinkIcon size={11} />
																	<span>{linkPolicyBadge(m)}</span>
																</span>
															{/snippet}
														</Tooltip.Trigger>
														<Tooltip.Portal>
															<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
																<Tooltip.Arrow class="uiTipArrow" />
																{isPermanentLink(m.link_duration_months)
																	? tr('Media ini menjamin tautannya permanen — tidak pernah dilepas.')
																	: tr('Media ini menjamin tautannya {bulan} bulan sejak artikel tayang.').replace('{bulan}', String(m.link_duration_months))}
															</Tooltip.Content>
														</Tooltip.Portal>
													</Tooltip.Root>
												{/if}
												{#if dist.known}
												<Tooltip.Root>
												<Tooltip.Trigger>
												{#snippet child({ props })}
												<span
												{...props}
												class="distCount"
												class:distCountMuted={dist.published === 0}
												aria-label={dist.published > 0 ? `${dist.published} ${tr('artikel terdistribusi')}` : tr('Belum ada distribusi')}
												>
												<span class="distDot" class:distDotMuted={dist.published === 0}></span>
												<span>{dist.published > 0 ? `${dist.published} ${tr('artikel')}` : tr('Belum ada distribusi')}</span>
												</span>
												{/snippet}
												</Tooltip.Trigger>
												<Tooltip.Portal>
												<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
												<Tooltip.Arrow class="uiTipArrow" />
												<strong>{dist.published > 0 ? `${dist.published} ${tr('artikel terdistribusi')}` : tr('Belum ada distribusi')}</strong> {tr('— jumlah artikel yang sudah terbit lewat media ini di seluruh platform Tauthub.')}
												</Tooltip.Content>
												</Tooltip.Portal>
												</Tooltip.Root>
												{/if}
											</div>
										{/if}
										{#if archivedIds.has(m.id)}
											<span class="maintBadge" title={tr('Media sedang diperbaiki oleh mitra')}>{tr('Maintenance')}</span>
										{/if}
									</div>
								</td>

									<td class="modernTd">
										<div class="categoryBadgeGroup">
											<span class="categoryPill categoryPill--{getCategoryTone(cats[0])}">{cats[0]}</span>
											{#if cats.length > 1}
											<Tooltip.Root>
										<Tooltip.Trigger>
											{#snippet child({ props })}
												<button
													{...props}
													type="button"
													class="categoryPlusTag"
													aria-label={`${cats.length} kategori: ${cats.join(', ')}`}
													aria-expanded={catPopoverId === m.id}
													onclick={(e) => openCategoryPopover(e, m.id, cats)}
												>+{cats.length - 1}</button>
											{/snippet}
										</Tooltip.Trigger>
										<Tooltip.Portal>
											<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
											<Tooltip.Arrow class="uiTipArrow" />
											Kategori: {cats.join(', ')}
										</Tooltip.Content>
										</Tooltip.Portal>
									</Tooltip.Root>
										{/if}
									</div>
								</td>

								<td class="modernTd">
									<div class="trafficStack">
										<span class="trafficMainVal">{traffic.text}</span>
										<span class="trafficSourceText">{traffic.source}</span>
									</div>
								</td>

								<td class="modernTd" style="text-align:center"><span class="scoreBoxPill">{getDa(m)}</span></td>

								<td class="modernTd" style="text-align:center">
									<span class="scoreBoxPill aiTrustPill" style={aiTrustToneStyle(getAiTrust(m))}>{getAiTrust(m)}</span>
								</td>

								<td class="modernTd">
									<!-- SLA = rata-rata waktu terbit nyata media (bukan label taksiran). -->
									<div class="flex flex-col items-start gap-1">
										<Tooltip.Root>
											<Tooltip.Trigger>
												{#snippet child({ props })}
													<span {...props} class="slaPill">{sla}</span>
												{/snippet}
											</Tooltip.Trigger>
											<Tooltip.Portal>
												<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
												<Tooltip.Arrow class="uiTipArrow" />
												{slaTip(m)}
											</Tooltip.Content>
											</Tooltip.Portal>
											</Tooltip.Root>
										</div>
								</td>

								<td class="modernTd"><span class="priceBoldText">Rp {fmt(m.client_price ?? m.price ?? 0)}</span></td>

								<td class="modernTd" style="text-align:right">
									<div class="actionBtnGroup">
										<button
											type="button"
											class="bookmarkActionBtn {bookmarkedIds.has(m.id) ? 'bookmarkActionBtnActive' : ''}"
											onclick={(e) => toggleBookmark(m.id, e)}
											title={bookmarkedIds.has(m.id) ? tr('Hapus dari Bookmark') : tr('Simpan ke Bookmark')}
											aria-label="Bookmark"
										><Bookmark size={18} /></button>
										<button
											type="button"
											class="viewDetailBtn"
											onclick={(e) => {
												e.stopPropagation();
												// Mobile: panel kanan berisi info kolom yang disembunyikan.
												if (typeof window !== 'undefined' && window.matchMedia('(max-width: 768px)').matches) quickInfo = m;
												else detail = m;
											}}
										><span class="vdFull">{tr("Lihat Detail")}</span><span class="vdShort">{tr("Detail")}</span><ArrowRight size={13} /></button>
									</div>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<!-- Paginasi -->
			<div class="modernPaginationBar">
				<span class="modernPaginationInfo"> {tr("Menampilkan")} {(page_ - 1) * pageSize + 1}–{Math.min(page_ * pageSize, sorted.length)} {tr("dari")} {fmt(sorted.length)} media
				</span>
				<div class="paginationBtns">
					<button class="paginationBtn" type="button" disabled={page_ <= 1} onclick={() => (page_ -= 1)}>
						<ChevronLeft size={15} /> Sebelumnya
					</button>
					<span class="paginationPage">Hal {page_} / {totalPages}</span>
					<button class="paginationBtn" type="button" disabled={page_ >= totalPages} onclick={() => (page_ += 1)}>
						Berikutnya <ChevronRight size={15} />
					</button>
				</div>
				</div>
				{/if}
			</div>
		</div>

	<!-- Modal Impor List -->
{#if importOpen}
	<div class="modalOverlay" role="presentation" onclick={() => (importOpen = false)}>
		<dialog open class="modalCard" aria-label={tr("Impor daftar media")} onclick={(e) => e.stopPropagation()}>
			<div class="modalHeader">
				<h2 class="modalTitle"><Upload size={17} />{tr("Impor Daftar Media")}</h2>
				<button class="modalClose" type="button" aria-label={tr("Tutup")} onclick={() => (importOpen = false)}><X size={18} /></button>
			</div>
			<button class="importDrop" type="button" onclick={() => fileInput?.click()}>
				<input
					bind:this={fileInput}
					type="file"
					accept=".csv,.txt"
					hidden
					onchange={handleFileUpload}
				/>
				<Upload size={24} />
				<span class="importDropTitle">{tr("Pilih file proposal (.csv / .txt)")}</span>
				<span class="importDropSub">File CSV hasil unduh ekspor proposal otomatis dikenali</span>
			</button>
			<div class="importDivider"><span>Atau Tempel Teks</span></div>
			<textarea class="modalTextarea" rows="6" bind:value={importText} placeholder={'contoh.id\nDetik.com'}></textarea>
			{#if importError}<p class="modalError">{importError}</p>{/if}
			<div class="modalFooter">
				<button class="advFilterResetBtn" type="button" onclick={() => (importOpen = false)}>{tr("Batal")}</button>
				<button class="advFilterApplyBtn" type="button" onclick={applyImport}>Terapkan</button>
			</div>
		</dialog>
	</div>
{/if}

<!-- Modal Daftar Bookmark -->
{#if bookmarkOpen}
	<div class="modalOverlay" role="presentation" onclick={() => (bookmarkOpen = false)}>
		<dialog open class="modalCard" aria-label={tr("Media tersimpan")} onclick={(e) => e.stopPropagation()}>
			<div class="modalHeader">
				<h2 class="modalTitle"><Bookmark size={17} />{tr("Media Tersimpan")}</h2>
				<button class="modalClose" type="button" aria-label={tr("Tutup")} onclick={() => (bookmarkOpen = false)}><X size={18} /></button>
			</div>
			{#if bookmarkedIds.size === 0}
				<p class="modalHint">{tr("Belum ada media tersimpan. Tandai media dengan ikon bookmark untuk menyimpannya.")}</p>
			{:else}
				<ul class="bookmarkList">
					{#each rows.filter((m) => bookmarkedIds.has(m.id)) as m (m.id)}
						<li class="bookmarkRow">
							<span class="bookmarkName">{domainLabel(m)}</span>
							<button
								class="bookmarkRemove"
								type="button"
								aria-label={`Hapus ${m.name} dari bookmark`}
								onclick={(e) => toggleBookmark(m.id, e)}
							><Trash2 size={15} /></button>
						</li>
					{/each}
				</ul>
			{/if}
			<div class="modalFooter">
				<button class="advFilterApplyBtn" type="button" onclick={() => (bookmarkOpen = false)}>{tr("Tutup")}</button>
			</div>
		</dialog>
	</div>
{/if}

	<!-- Modal Nama Kampanye -->
	{#if campaignOpen}
		<div class="modalOverlay" role="presentation" onclick={() => (campaignOpen = false)}>
				<dialog open class="modalCard campaignCard" aria-label={tr("Buat kampanye")} onclick={(e) => e.stopPropagation()}>
					<span class="campaignHandle" aria-hidden="true"></span>
					<form class="campaignModalForm" onsubmit={(e) => { e.preventDefault(); void createCampaign(); }}>
						<div class="campaignBody">
						<div class="modalHeader">
						<h2 class="modalTitle"><LayoutGrid size={17} />{tr("Simpan ke Kampanye")}</h2>
						<button class="modalClose" type="button" aria-label={tr("Tutup")} onclick={() => (campaignOpen = false)}><X size={18} /></button>
					</div>
						<div class="campaignSummary">
							<div class="campaignSummaryRow">
								<div class="campaignSummaryLabel">
									<div class="campaignSummaryIcon"><FileText size={18} /></div>
									<span>{tr("Media terpilih")}</span>
								</div>
								<strong>{selectedIds.size}</strong>
							</div>
							<div class="campaignSummaryRow">
								<div class="campaignSummaryLabel">
									<div class="campaignSummaryIcon"><Database size={18} /></div>
									<span>{tr("Total biaya publikasi")}</span>
								</div>
								<strong>Rp {fmt(totalSelectedPrice)}</strong>
							</div>
							{#if walletAvailableTc !== null}
								<div class="campaignSummaryRow">
									<div class="campaignSummaryLabel">
										<div class="campaignSummaryIcon"><Wallet size={18} /></div>
										<span>{tr("Saldo kredit")}</span>
									</div>
									<strong>Rp {fmt(walletAvailableTc * walletRateIdr)}</strong>
								</div>
							{/if}
						</div>
						{#if walletAvailableTc !== null}
							{#if totalSelectedCredits > walletAvailableTc}
								<div class="campaignBalanceBadge campaignBalanceBadgeLow">
									<Info size={16} />
									<span>
										{tr("Saldo kurang")} Rp {fmt((totalSelectedCredits - walletAvailableTc) * walletRateIdr)} {tr("untuk pesanan ini.")}
										<a href={`/klien/${brandUrl}/credits`} onclick={openTopUpFromBar}>{tr("Top Up dulu")}</a>
									</span>
								</div>
							{:else}
								<div class="campaignBalanceBadge campaignBalanceBadgeOk">
									<Check size={16} />
									<span>
										<strong>{tr("Cukup untuk pesanan ini")}</strong>
										<em>{tr("Saldo kamu mencukupi untuk membuat kampanye")}</em>
									</span>
								</div>
							{/if}
							{/if}

						
					<label class="modalField">
						<span>{tr("Nama Kampanye")} <span class="campaignSummaryMuted">{tr("opsional")}</span></span>
						<input
							class="app-input"
							bind:value={campaignName}
							maxlength={120}
							placeholder={tr("Kosongkan untuk nama otomatis (Kampanye Baru #N)")}
						/>
					</label>
					{#if campaignError}<p class="modalError">{campaignError}</p>{/if}
					{#if SHOW_PROFILER_FIELD}
						<label class="modalField">
							<span class="inline-flex items-center gap-1">
								{tr("Profiler")} <span class="campaignSummaryMuted">{tr("opsional")}</span>
								<Tooltip.Root>
									<Tooltip.Trigger>
										{#snippet child({ props })}
											<button
												{...props}
												type="button"
												class="infoDot cursor-help"
												style="border: none; background: transparent; padding: 0; margin-left: 2px;"
												aria-label={tr("Penjelasan Profiler")}
												onclick={(e) => {
													e.preventDefault();
													e.stopPropagation();
												}}
											>
												<Info size={13} />
											</button>
										{/snippet}
									</Tooltip.Trigger>
									<Tooltip.Portal>
										<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={6} style="max-width: 300px;">
											<Tooltip.Arrow class="uiTipArrow" />
											{tr(
												"Dengan Profiler, artikel ditulis mengikuti karakter brand-mu: gaya bahasa, nada, dan sudut pandang yang sudah kamu tentukan. Tanpa Profiler, The Writer memakai pengaturan bawaan Tauthub."
											)}
										</Tooltip.Content>
									</Tooltip.Portal>
								</Tooltip.Root>
							</span>
							<Select
								ariaLabel={tr("Profiler")}
								placeholder={profilerLoading ? tr('Memuat…') : tr('Tanpa Profiler')}
								bind:value={profilerId}
								options={[{ value: '', label: tr('Tanpa Profiler') }, ...profilerOptions]}
								searchable={profilerOptions.length > 10}
								searchPlaceholder={tr("Cari Profiler…")}
								minWidth={0}
							/>
						<span class="campaignHint">{tr("Acuan brand & gaya penulisan untuk The Writer.")}</span>
							{#if !profilerLoading && profilerOptions.length === 0}
								<!-- SEMENTARA: pintasan ke Brief (chatbot Profiler) disembunyikan
								     sebelum deploy. Kembalikan saat fiturnya dirilis.
								<button class="profilerInviteBtn" type="button" onclick={() => goto(`/klien/${page.params.brandId}/profiler`)}>
									{tr('Lengkapi Profiler')}
								</button>
								-->
							{/if}
						</label>
					{/if}
					</div>
					<div class="modalFooter campaignFooter">
						<button class="advFilterResetBtn" type="button" onclick={() => (campaignOpen = false)}>{tr("Batal")}</button>
						<button class="advFilterApplyBtn" type="submit" disabled={creating}>
							{creating ? tr('Membuat…') : tr('Buat Kampanye')}
						</button>
					</div>
				</form>
			</dialog>
		</div>
	{/if}

	<!-- Modal AI Media Matchmaker -->
	{#if matchmakerOpen}
		<MatchmakerModal
			apiBase={apiBase}
			detailIds={new Set(rows.map((m) => m.id))}
			hasSelection={selectedIds.size > 0}
			escapeDisabled={detail !== null}
			onClose={() => (matchmakerOpen = false)}
			onShowDetail={openMatchmakerDetail}
			onApply={applyMatchmaker}
		/>
	{/if}

	<!-- Modal Detail Media -->
	{#if detail}
		{@const d = detail}
		<MediaDetailModal
			media={d}
			onclose={() => { detail = null; detailFromMatchmaker = false; }}
			onProceed={() => (detailFromMatchmaker ? pickFromMatchmakerDetail(d) : proceedFromDetail(d))}
		/>
	{/if}

<!-- Popover tr("Semua Kategori") — fixed agar tidak terpotong kartu tabel. -->
{#if catPopoverId}
	<button
		type="button"
		class="categoryPopoverBackdrop"
		aria-label={tr("Tutup daftar kategori")}
		onclick={() => (catPopoverId = null)}
	></button>
		<div
			class="categoryPopover"
			class:categoryPopoverUp={catPopoverPos.atas}
			role="tooltip"
			style="left: {catPopoverPos.left}px; top: {catPopoverPos.top}px;"
		>
		<div class="categoryPopoverTitle">{tr("Semua Kategori")}</div>
			<div class="categoryPopoverList">
				{#each catPopoverCats as c (c)}<span class="categoryPill categoryPill--{getCategoryTone(c)}">{c}</span>{/each}
			</div>
	</div>
{/if}

{#if selectedIds.size > 0}<div style="height:74px"></div>{/if}

	<!-- Tahap alur kampanye di header (portal ke utility bar):
	     Menggunakan komponen AgentSteps yang sama persis dengan The Writer
	     agar posisi, ukuran, jumlah langkah, dan styling pil 100% konsisten tanpa layout shift. -->
	{#if addToCampaignId}
		<AgentSteps
			current={1}
			campaignId={addToCampaignId}
			onSelect={(n, proceed) => {
				if (n === 2) {
					void addToCampaign();
				} else {
					proceed();
				}
			}}
		/>
	{:else if selectedIds.size > 0}
		<AgentSteps
			current={1}
			maxReached={2}
			onSelect={(n) => {
				if (n === 2) void openCampaignModal();
			}}
		/>
	{/if}

	<!-- Selection bar -->
	{#if selectedIds.size > 0}
		<div
			class="selectedBottomBar"
			class:barPulse
			in:fly={motionOK ? { y: 48, duration: 340, easing: cubicOut } : { duration: 0 }}
			out:fly={motionOK ? { y: 48, duration: 280, easing: cubicIn } : { duration: 0 }}
		>
				<div class="selectedBarMobile">
					<div class="selectedBarMobileRow">
						<div class="selectedBarMobileIcons" aria-hidden="true">
							{#each selectedItems.slice(0, 3) as it (it.id)}
								<span class="selectedBarAvatar">
									{#if it.domain && !faviconFailed[it.id]}
										<img
											src={`https://www.google.com/s2/favicons?domain=${encodeURIComponent(it.domain)}&sz=128`}
											alt=""
											loading="lazy"
														decoding="async"
											onerror={() => (faviconFailed = { ...faviconFailed, [it.id]: true })}
										/>
									{:else}
										{(it.name ?? it.id).trim().charAt(0).toUpperCase() || '•'}
									{/if}
								</span>
							{/each}
							{#if selectedIds.size > 3}
								<span class="selectedBarAvatar selectedBarAvatarMore">+{selectedIds.size - 3}</span>
							{/if}
						</div>
						<div class="selectedBarMobileText">
							<strong>{selectedIds.size} {tr('Media dipilih')}</strong>
							<span>{tr('Total')} Rp {fmt(totalSelectedPrice)}</span>
						</div>
						<!-- Kosongkan pilihan. Ikon saja dan merah supaya beda jelas
						     dari tombol ringkasan di sebelahnya. -->
						<button
							class="selectedBarClear"
							type="button"
							aria-label={tr('Hapus pilihan')}
							title={tr('Hapus pilihan')}
							onclick={clearSelection}
						>
							<Trash2 size={17} />
						</button>
						<!-- Buka modal ringkasan kampanye (fungsi sama dengan tombol utama). -->
						<button
							class="selectedBarToggle"
							type="button"
							aria-haspopup="dialog"
							aria-label={tr('Ringkasan kampanye')}
							onclick={() => void openCampaignModal()}
						>
							<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
								<path fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" d="m6 9 6 6 6-6" />
							</svg>
						</button>
					</div>

					<!-- Detail (expand/collapse): saldo + daftar media terpilih. -->
				</div>
				<div class="selectedBarStats">
					<div class="selectedStatItem selectedStatClickable">
						<button
							class="selectedStatBtn"
							class:active={selectedListOpen}
							type="button"
							aria-expanded={selectedListOpen}
							aria-label={tr('Lihat media terpilih')}
							onclick={() => (selectedListOpen = !selectedListOpen)}
						>
							<span class="selectedStatFaviconRow">
								<span class="selectedStatFavicons" aria-hidden="true">
									{#each selectedItems.slice(0, 4) as it, i (it.id)}
										<span
											class="selectedStatFavicon"
											style="z-index: {4 - i};"
											animate:flip={{ duration: 220 }}
										>
											{#if it.domain && !faviconFailed[it.id]}
												<img
													src={`https://www.google.com/s2/favicons?domain=${encodeURIComponent(it.domain)}&sz=128`}
													alt=""
													loading="lazy"
														decoding="async"
													onerror={() => (faviconFailed = { ...faviconFailed, [it.id]: true })}
												/>
											{:else}
												<span class="selectedStatFaviconFallback">{it.name.trim().charAt(0).toUpperCase() || '?'}</span>
											{/if}
										</span>
									{/each}
									{#if selectedIds.size > 4}
										<span class="selectedStatFavicon selectedStatFaviconMore" style="z-index: 0;">
											+{selectedIds.size - 4}
										</span>
									{/if}
								</span>
								<span class="selectedStatTextCol">
									<span class="selectedStatDomainVal">{selectedIds.size} Media Terpilih</span>
									<span class="selectedStatSub">Siap untuk kampanye</span>
								</span>
							</span>
						</button>

						{#if selectedListOpen}
							<button class="selectedListBackdrop" type="button" aria-label={tr('Tutup')} onclick={() => (selectedListOpen = false)}></button>
							<div class="selectedListPop" role="dialog" aria-label={tr('Media terpilih')}>
								<div class="selectedListHead">
									<span class="selectedListTitle">{tr('Media terpilih')} ({selectedIds.size})</span>
									<span class="selectedListActions">
									<button
										class="selectedListReset"
										type="button"
										aria-label={tr('Reset pilihan')}
										title={tr('Reset pilihan')}
										onclick={() => { clearSelection(); selectedListOpen = false; }}
									>
										<RotateCcw size={13} />
									</button>
									<button class="selectedListClose" type="button" aria-label={tr('Tutup')} onclick={() => (selectedListOpen = false)}>
										<X size={14} />
									</button>
									</span>
								</div>
								<div class="selectedListItems">
									{#each selectedItems as it (it.id)}
										<div class="selectedListRow">
											<span class="selectedListRowName" title={it.name}>{it.name}</span>
											<span class="selectedListRowPrice">Rp {fmt(it.price)}</span>
											<button
												class="selectedListRemove"
												type="button"
												aria-label={`${tr('Hapus')} ${it.name}`}
												onclick={() => removeSelected(it.id)}
											>
												<X size={12} />
											</button>
										</div>
									{/each}
								</div>
								<div class="selectedListFoot">
									<span>{tr('Total')}</span>
									<strong>Rp {fmt(totalSelectedPrice)}</strong>
								</div>
							</div>
						{/if}
					</div>
					<div class="selectedBarDivider"></div>
					<!-- Ringkasan finansial: total publikasi + status kecukupan saldo
					     dalam satu blok vertikal (blok saldo terpisah dihapus). -->
					<div class="selectedStatItem">
						<span class="selectedStatLabel">{tr('Total publikasi')}</span>
						<span class="selectedStatPriceVal">Rp {fmt(totalSelectedPrice)}</span>
						{#if walletAvailableTc !== null}
							{#if totalSelectedCredits > walletAvailableTc}
								<span class="walletStatus walletShort">
									<CircleAlert size={13} class="shrink-0" />
									<span>{tr('Saldo kurang')} Rp {fmt((totalSelectedCredits - walletAvailableTc) * walletRateIdr)}</span>
									<a href={`/klien/${brandUrl}/credits`} onclick={openTopUpFromBar}>{tr('Top Up')}</a>
								</span>
							{:else}
								<span class="walletStatus walletEnough">
									<Check size={13} class="shrink-0" />
									<span>{tr('Saldo mencukupi')}</span>
								</span>
							{/if}
						{/if}
					</div>
			</div>
		<div class="selectedBarRight">
			<button class="selectedCancelBtn" type="button" onclick={() => (addToCampaignId ? goto(`/klien/${brandUrl}/taut-pintar/${addToCampaignId}/writer`) : (selectedIds = new Set()))}>{tr("Batal")}</button>
			<button class="selectedExportBtn" type="button" onclick={exportProposal} title={tr("Unduh proposal ringkasan media terpilih (format CSV)")}>
				<FileText size={15} /><span>Ekspor Proposal</span>
			</button>
			<button bind:this={createBtnEl} class="selectedCreateBtn" type="button" disabled={creating || adding || campaignDone} onclick={() => (addToCampaignId ? void addToCampaign() : void openCampaignModal())}>
				<Plus size={14} /><span>{addToCampaignId ? (adding ? tr('Menambahkan…') : tr('Tambahkan ke Kampanye')) : tr("Buat Kampanye")}</span>
			</button>
		</div>
	</div>
{/if}

</Tooltip.Provider>

<!-- Lapisan animasi fly-to-campaign: fixed agar chip tidak terpotong overflow tabel. -->
<div class="flyLayer" bind:this={flyLayer} aria-hidden="true"></div>

<style>
.maintBadge {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	padding: 2px 8px;
	border-radius: 999px;
	font-size: 11px;
	font-weight: 600;
	color: #6b7280;
	background: #f1f3f6;
	border: 1px dashed #9aa4b2;
}
	/* Banner konteks saat masuk dari tombol "Kampanye Baru". */
	.catalogContextBanner {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 12px;
		padding: 10px 14px;
		border-radius: 12px;
		border: 1px solid var(--border);
		background: var(--accent, #eef4ff);
		font-size: 13px;
		color: var(--foreground);
	}
	/* Strip "N media dipulihkan dari sesi terakhir". */
	.restoredSelectionBar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		margin: 12px 0;
		padding: 8px 14px;
		border-radius: 10px;
		border: 1px dashed var(--border);
		background: var(--muted, #f6f7f9);
		font-size: 13px;
	}
	/* Status kecukupan saldo: satu baris rapat di bawah nominal total, boleh
	   membungkus di layar sempit tanpa mendorong horizontal overflow. */
	.walletStatus {
		display: inline-flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 5px;
		margin-top: 3px;
		font-size: 11.5px;
		font-weight: 600;
		line-height: 1.35;
	}
	.walletShort { color: var(--warning, #b45309); }
	.walletEnough { color: var(--success, #15803d); }
	.walletStatus a {
		color: #2563eb;
		font-weight: 700;
		text-decoration: none;
	}
	.walletStatus a:hover { text-decoration: underline; }
		/* Ringkasan konfirmasi di modal kampanye (kartu beraksen biru dengan ikon svg). */
		/* Jarak antarbagian modal kampanye di desktop: header → ringkasan →
		   saldo → field. Mobile punya aturannya sendiri di media.css. */
		@media (min-width: 768px) {
			.campaignBody {
				display: flex;
				flex-direction: column;
				gap: 16px;
				padding-bottom: 6px;
			}
			.campaignSummary {
				padding: 8px 18px;
			}
			.campaignSummaryRow {
				padding: 13px 0;
			}
		}
		.campaignSummary {
			margin: 0;
			padding: 4px 18px;
			border: 1px solid #e1effe;
			border-radius: 16px;
			background: #f4f8fe;
			display: flex;
			flex-direction: column;
		}
		.campaignSummaryRow {
			display: flex;
			align-items: center;
			justify-content: space-between;
			gap: 12px;
			padding: 12px 0;
		}
		.campaignSummaryRow + .campaignSummaryRow {
			border-top: 1px solid #e5effd;
		}
		.campaignSummaryLabel {
			display: flex;
			align-items: center;
			gap: 12px;
			color: #334155;
			font-size: 13.5px;
			font-weight: 500;
		}
		.campaignSummaryIcon {
			width: 36px;
			height: 36px;
			border-radius: 10px;
			background: #e0edfe;
			color: #2563eb;
			display: flex;
			align-items: center;
			justify-content: center;
			flex-shrink: 0;
		}
		.campaignSummaryRow strong {
			font-size: 14.5px;
			font-weight: 700;
			color: #0f172a;
			white-space: nowrap;
		}
		.campaignHint {
			display: block;
			font-weight: 500;
			font-size: 11.5px;
			color: var(--muted-foreground);
		}
		.campaignSummaryMuted {
			font-weight: 550;
			color: var(--muted-foreground);
			background: var(--secondary, #f1f5f9);
			border-radius: 6px;
			padding: 1px 7px;
			font-size: 11.5px;
		}
</style>
