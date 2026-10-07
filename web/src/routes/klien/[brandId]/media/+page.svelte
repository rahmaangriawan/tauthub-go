<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Katalog media klien — port setia media-section.tsx (Next.js).
	// Struktur: error banner → header eksplorasi (+stats card) → top bar
	// (search, kategori, filter lanjutan, impor, bookmark) → banner filter →
	// tabel 10 kolom + sort + multi-select → paginasi → modal → selection bar.
	import { getContext, onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import {
		ArrowRight, Bookmark, Check, Clock, FileText, Info, LayoutGrid,
		Plus, Search, SlidersHorizontal, Trash2, Upload, X, Zap,
		ChevronLeft, ChevronRight
	} from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { campaignUrlId } from '$lib/campaign-url';
	import { mediaDomainLabel } from '$lib/media/domain-label';
	import { mediaDistribution } from '$lib/media/distribution';
	import { seededShuffle, randomShuffleSeed } from '$lib/media/shuffle';
	import { apiRequest, ensureValidSession, readApiError, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { getI18n } from '$lib/i18n/i18n.svelte';
	import MediaDetailModal from './_detail-modal.svelte';
	import Select from '$lib/components/select.svelte';
	import { Tooltip } from 'bits-ui';
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
		gsc_connected?: boolean; image_required?: boolean | null;
		label_sponsor?: string | null; topik_terlarang?: string[];
		max_links?: number | null; allow_hyperlink?: boolean | null;
		publication_mode?: string | null; word_count_mode?: string | null;
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
	let priceFilter = $state<'' | 'low' | 'mid' | 'high'>('');
	let minDaFilter = $state<'' | '10' | '20' | '30'>('');
	let verifiedOnly = $state(false);
	let dewanPersOnly = $state(false);
	let bookmarkOnly = $state(false);
	let selectedIds = $state<Set<string>>(new Set());
	let bookmarkedIds = $state<Set<string>>(new Set());
	let detail = $state<Media | null>(null);
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
	let catPopoverPos = $state<{ left: number; top: number }>({ left: 0, top: 0 });
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
		catPopoverPos = { left, top: rect.bottom + 4 };
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
	// Cek pesanan ganda: media yang sudah dipesan di kampanye aktif lain.
	let dupHits = $state<Array<{ mediaName: string; campaignName: string }>>([]);
	let checkingDup = $state(false);
	// Saldo kredit (bar seleksi & modal) — null = belum/gagal terbaca.
	let walletAvailableTc = $state<number | null>(null);
	let walletRateIdr = $state(1000);
	// Jumlah media yang dipulihkan dari sesi terakhir (badge, bukan error).
	let restoredSelectionCount = $state(0);
	// Pilihan bisa memuat media di luar 250 baris yang sedang dimuat (filter lain)
	// — snapshot harga disimpan supaya total di bar tidak bohong.
	let selectedSnapshot = $state<Record<string, { name: string; price: number; credits: number }>>({});
	let selectionHydrated = $state(false);

	const BOOKMARK_KEY = 'tauthub_client_bookmarked_media';
	const SELECTED_KEY = 'tauthub_client_selected_media';

	// ── Helper parity media-section.tsx ────────────────────────────────
	/** Estimasi trafik. CATATAN: tren ↑/↓ di app lama DIREKAYASA dari
	 *  `Math.abs(val % 31)` — angka palsu, jadi tidak disalin ke sini. */
	function formatTraffic(swVisits?: number | null, est?: number | null): { text: string; source: string } {
		const val = swVisits && swVisits > 0 ? swVisits : est && est > 0 ? est : 0;
		if (!val) return { text: 'Estimasi', source: 'Via Estimasi Media' };
		let text: string;
		if (val >= 1_000_000) text = `± ${(val / 1_000_000).toFixed(1).replace(/\.0$/, '')}M / bulan`;
		else if (val >= 1_000) text = `± ${Math.round(val / 1_000)} rb / bulan`;
		else text = `± ${val.toLocaleString('id-ID')} / bulan`;
		return { text, source: swVisits ? 'Via Similarweb' : 'Via Estimasi Media' };
	}

	const AUTO_TYPES = ['wordpress', 'blogger', 'tulisio_api', 'custom_web', 'webhook'];
	function getReleaseSpeed(integrationType: string): { cls: string; label: string; sub: string; isSuper: boolean } {
		if (AUTO_TYPES.includes(integrationType)) {
			return { cls: 'releaseBadgeSuper', label: 'Super Cepat', sub: '< 3 Jam', isSuper: true };
		}
		if (integrationType === 'rss') {
			return { cls: 'releaseBadgeFast', label: 'Cepat', sub: '< 24 Jam', isSuper: false };
		}
		return { cls: 'releaseBadgeStandard', label: 'Standar', sub: '1 - 2 Hari', isSuper: false };
	}

	// Tooltip badge kecepatan: ambangnya sama dengan legenda di header kolom
	// (Super Cepat < 3 jam, Cepat < 24 jam, Standar 1-2 hari).
	function releaseTip(sub: string): string {
		return tr('Estimasi terbit {waktu} setelah artikel dikirim.').replace('{waktu}', sub);
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
					{ name: x.name ?? '', price: Number(x.price ?? 0) || 0, credits: Number(x.credits ?? 0) || 0 }
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
		if (!selectionHydrated) return;
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
		dupHits = [];
		campaignOpen = true;
		if (walletAvailableTc === null) void loadWallet();
		checkingDup = true;
		dupHits = await findReservations();
		checkingDup = false;
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
			restoreSelection();
			selectionHydrated = true;
			await load();
			void loadWallet();
		})();
		const onExpired = () => void goto('/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	function persistBookmarks() {
		try {
			localStorage.setItem(BOOKMARK_KEY, JSON.stringify([...bookmarkedIds]));
		} catch { /* penyimpanan penuh — abaikan */ }
	}
	function toggleBookmark(id: string, e: Event) {
		e.stopPropagation();
		const next = new Set(bookmarkedIds);
		if (next.has(id)) next.delete(id); else next.add(id);
		bookmarkedIds = next;
		persistBookmarks();
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
		}
		selectedIds = next;
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
				const speed = getReleaseSpeed(m.integration_type);
				const bucket = speed.isSuper ? 'super' : m.integration_type === 'rss' ? 'fast' : 'standard';
				if (bucket !== speedFilter) return false;
			}
			if (priceFilter) {
				const price = m.client_price ?? m.price ?? 0;
				if (priceFilter === 'low' && !(price > 0 && price <= 500_000)) return false;
				if (priceFilter === 'mid' && !(price > 500_000 && price <= 2_000_000)) return false;
				if (priceFilter === 'high' && !(price > 2_000_000)) return false;
			}
			if (minDaFilter) {
				const da = getDa(m);
				if (typeof da !== 'number' || da < Number(minDaFilter)) return false;
			}
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
				case 'speed': av = AUTO_TYPES.includes(a.integration_type) ? 2 : 1; bv = AUTO_TYPES.includes(b.integration_type) ? 2 : 1; break;
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
		(speedFilter ? 1 : 0) + (priceFilter ? 1 : 0) + (minDaFilter ? 1 : 0) +
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
	// Waktu rilis terlama di antara media terpilih — ekspektasi jujur di modal.
	const slowestRelease = $derived.by(() => {
		const pick = selectedMedia.map((m) => getReleaseSpeed(m.integration_type));
		if (pick.some((s) => s.label === 'Standar')) return getReleaseSpeed('manual');
		if (pick.some((s) => s.label === 'Cepat')) return getReleaseSpeed('rss');
		return pick[0] ?? null;
	});

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
		priceFilter = '';
		minDaFilter = '';
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
		const headers = ['Nama Media', 'Website', 'Kategori', 'Kecepatan', 'DA', 'AI Trust', 'AI Cited', tr('Harga IDR'), 'Kredit TC'];
		const esc = (v: unknown) => `"${String(v ?? '').replace(/"/g, '""')}"`;
		// Pemisah ";" dipakai karena Excel/Sheets ber-locale Indonesia (dan Eropa)
		// memakai titik-koma sebagai list separator; CSV berkoma tampil menyatu
		// jadi satu kolom di aplikasi sheet tersebut.
		const lines = selectedMedia.map((m) => {
			const speed = getReleaseSpeed(m.integration_type);
			const price = m.client_price ?? m.price ?? 0;
			return [
				esc(m.name), esc(m.website_url ?? ''), esc(m.category ?? 'Umum'),
				esc(`${speed.label} (${speed.sub})`), esc(getDa(m)), esc(getAiTrust(m)),
				// Tiga keadaan: Ya / Tidak / belum bisa ditentukan (Google tidak
				// menampilkan AI Overview) — jangan ditulis "Tidak" untuk yang
				// belum pernah bisa dinilai.
				esc(m.seo_ai_citation === true ? 'Ya' : m.seo_ai_citation === false ? 'Tidak' : 'Belum bisa ditentukan'),
				esc(price), esc(m.credit_cost || Math.ceil(price / 1000))
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

	function proceedFromDetail(m: Media) {
		const snap = selectedSnapshot[m.id] ?? {
			name: domainLabel(m),
			price: m.client_price ?? m.price ?? 0,
			credits: m.credit_cost ?? 0
		};
		selectedIds = new Set([...selectedIds, m.id]);
		selectedSnapshot = { ...selectedSnapshot, [m.id]: snap };
		detail = null;
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
					// Kosong → nama otomatis "Kampanye Baru #N" (ditomori server).
					name: name || 'Kampanye Baru',
					starts_at: new Date().toISOString(),
					entry_mode: 'manual',
					media_ids: [...selectedIds]
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
		<div class="modernActionsGroup">
			<!-- Kategori -->
			<div class="modernSelectWrap">
				<Select
					bind:value={category}
					ariaLabel="Kategori"
					Icon={LayoutGrid}
					placeholder={tr("Semua Kategori")}
					minWidth={210}
					onchange={() => (page_ = 1)}
					options={[
						{ value: '', label: tr('Semua Kategori'), count: rows.length },
						...categories.map(([name, count]) => ({ value: name, label: name, count }))
					]}
				/>
			</div>

			<!-- Filter lanjutan (popover mengambang) -->
			<button
				bind:this={advBtn}
				class="modernActionBtn {activeFilterCount > 0 ? 'modernActionBtnActive' : ''}"
				type="button"
				aria-expanded={advOpen}
				onclick={openAdvFilter}
			>
				<SlidersHorizontal size={15} />
				<span>Filter</span>
				{#if activeFilterCount > 0}<span class="modernActionBadge">{activeFilterCount}</span>{/if}
			</button>

			<!-- Impor daftar -->
			<button class="modernActionBtn" type="button" onclick={() => (importOpen = true)}>
				<Upload size={15} /><span>Impor List</span>
			</button>

			<!-- Bookmark -->
			<button
				class="modernActionBtn {bookmarkOnly ? 'modernActionBtnActive' : ''}"
				type="button"
				onclick={() => (bookmarkOnly = !bookmarkOnly)}
			>
				<Bookmark size={15} />
				<span>Bookmark</span>
				{#if bookmarkedIds.size > 0}<span class="modernActionBadge">{bookmarkedIds.size}</span>{/if}
			</button>
		</div>
	</div>

	<!-- Panel filter lanjutan — mengambang di bawah tombol Filter, tidak menggeser tabel -->
	{#if advOpen}
		<button
			type="button"
			class="advFilterBackdrop"
			aria-label={tr("Tutup filter")}
			onclick={() => (advOpen = false)}
		></button>
		<div class="advFilterPanel" style="left: {advPos.left}px; top: {advPos.top}px;">
			<div class="advFilterCard">
				<div class="advFilterHeader">
					<div class="advFilterTitle"><SlidersHorizontal size={16} /> Filter Lanjutan</div>
					<button class="advFilterResetBtn" type="button" onclick={() => (advOpen = false)} aria-label={tr("Tutup filter")}>
						<X size={16} />
					</button>
				</div>

				<div class="advFilterSection">
					<span class="advFilterSectionLabel">Kecepatan Rilis</span>
					<div class="advFilterChipsGroup">
						{#each [{ v: 'super', l: 'Super Cepat' }, { v: 'fast', l: 'Cepat' }, { v: 'standard', l: 'Standar' }] as opt (opt.v)}
							<button
								class="advFilterChip {speedFilter === opt.v ? 'advFilterChipActive' : ''}"
								type="button"
								onclick={() => { speedFilter = speedFilter === opt.v ? '' : (opt.v as typeof speedFilter); page_ = 1; }}
							>{opt.l}</button>
						{/each}
					</div>
				</div>

				<div class="advFilterSection">
					<span class="advFilterSectionLabel">{tr("Status Verifikasi")}</span>
					<div class="advFilterToggleRow">
						<label class="advFilterToggle">
							<input type="checkbox" bind:checked={verifiedOnly} onchange={() => (page_ = 1)} />
							<span>Domain terverifikasi</span>
						</label>
						<label class="advFilterToggle">
							<input type="checkbox" bind:checked={dewanPersOnly} onchange={() => (page_ = 1)} />
							<span>Dewan Pers</span>
						</label>
					</div>
				</div>

				<div class="advFilterSection">
					<span class="advFilterSectionLabel">{tr("Rentang Harga")}</span>
					<div class="advFilterChipsGroup">
						{#each [{ v: 'low', l: '≤ Rp 500rb' }, { v: 'mid', l: 'Rp 500rb – 2jt' }, { v: 'high', l: '> Rp 2jt' }] as opt (opt.v)}
							<button
								class="advFilterChip {priceFilter === opt.v ? 'advFilterChipActive' : ''}"
								type="button"
								onclick={() => { priceFilter = priceFilter === opt.v ? '' : (opt.v as typeof priceFilter); page_ = 1; }}
							>{opt.l}</button>
						{/each}
					</div>
				</div>

				<div class="advFilterSection">
					<span class="advFilterSectionLabel">Minimum DA</span>
					<div class="advFilterChipsGroup">
						{#each ['10', '20', '30'] as min (min)}
							<button
								class="advFilterChip {minDaFilter === min ? 'advFilterChipActive' : ''}"
								type="button"
								onclick={() => { minDaFilter = minDaFilter === min ? '' : (min as typeof minDaFilter); page_ = 1; }}
							>DA ≥ {min}</button>
						{/each}
					</div>
				</div>

				<div class="advFilterFooter">
					<button class="advFilterResetBtn" type="button" onclick={resetFilters}>Reset semua</button>
					<button class="advFilterApplyBtn" type="button" onclick={() => (advOpen = false)}>Terapkan</button>
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
							<th class="modernTh modernThSortable" onclick={() => toggleSort('name')}>NAMA MEDIA{sortIndicator('name')}</th>
							<th class="modernTh">KATEGORI</th>
							<th class="modernTh modernThSortable" onclick={() => toggleSort('traffic')}>
								<span class="thWithInfo">TRAFIK (EST)
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
								<span class="thWithInfo">AI TRUST
									<Tooltip.Root>
										<Tooltip.Trigger>
											{#snippet child({ props })}
												<button {...props} type="button" class="infoDot" aria-label="Keterangan AI Trust Score" onclick={(e) => e.stopPropagation()}><Info size={14} /></button>
											{/snippet}
										</Tooltip.Trigger>
										<Tooltip.Portal>
											<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
											<Tooltip.Arrow class="uiTipArrow" />
											<strong>AI Trust Score</strong> {tr("— Nilai 0-100: seberapa siap situs media ini dibaca dan dipercaya oleh mesin AI, dilihat dari kecepatan halaman dan kelengkapan data situsnya. Tanda — berarti kecepatannya belum bisa diukur.")}
										</Tooltip.Content>
										</Tooltip.Portal>
									</Tooltip.Root>
								</span>{sortIndicator('ai_trust')}
							</th>
							<th class="modernTh modernThSortable" style="text-align:center" onclick={() => toggleSort('ai_cited')}>
								<span class="thWithInfo">AI CITED
									<Tooltip.Root>
										<Tooltip.Trigger>
											{#snippet child({ props })}
												<button {...props} type="button" class="infoDot" aria-label="Keterangan AI Citation" onclick={(e) => e.stopPropagation()}><Info size={14} /></button>
											{/snippet}
										</Tooltip.Trigger>
										<Tooltip.Portal>
											<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
											<Tooltip.Arrow class="uiTipArrow" />
											Menandai apakah situs media ini disebut sebagai salah satu sumber pada jawaban AI Google. Tanda — berarti pemeriksaan terakhir tidak menerima data jawaban AI untuk situs ini, jadi belum bisa dinilai.
										</Tooltip.Content>
										</Tooltip.Portal>
									</Tooltip.Root>
								</span>{sortIndicator('ai_cited')}
							</th>
							<th class="modernTh modernThSortable" onclick={() => toggleSort('speed')}>
								<span class="thWithInfo">WAKTU RILIS
									<Tooltip.Root>
										<Tooltip.Trigger>
											{#snippet child({ props })}
												<button {...props} type="button" class="infoDot" aria-label="Keterangan estimasi waktu rilis" onclick={(e) => e.stopPropagation()}><Info size={14} /></button>
											{/snippet}
										</Tooltip.Trigger>
										<Tooltip.Portal>
											<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
											<Tooltip.Arrow class="uiTipArrow" />
											<span class="tipStack">
												<strong>Indikator Waktu Rilis</strong>
												<span><span style="color:#4ade80;font-weight:700;">Super Cepat:</span> &lt; 3 Jam</span>
												<span><span style="color:#60a5fa;font-weight:700;">Cepat:</span> &lt; 24 Jam</span>
												<span><span style="color:#d1d5db;font-weight:700;">Standar:</span> 1 - 2 Hari</span>
											</span>
										</Tooltip.Content>
										</Tooltip.Portal>
									</Tooltip.Root>

								</span>{sortIndicator('speed')}
							</th>
							<th class="modernTh modernThSortable" onclick={() => toggleSort('price')}>HARGA{sortIndicator('price')}</th>
							<th class="modernTh" style="text-align:right">AKSI</th>
						</tr>
					</thead>
					<tbody>
						{#each visible as m (m.id)}
							{@const traffic = formatTraffic(m.sw_visits, m.traffic_estimate)}
							{@const speed = getReleaseSpeed(m.integration_type)}
							{@const cats = m.categories && m.categories.length ? m.categories : m.category ? [m.category] : ['Umum']}
							{@const dist = distribution(m)}
							<tr
								class="modernTr"
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
										<div class="mediaTitleLine">
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
										</div>
										{#if m.contoh_artikel_url || dist.known}
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
										{#if m.dewan_pers_verified}
											<div class="dewanPersBadge"><Check size={13} /><span>Dewan Pers</span></div>
										{/if}
										{#if archivedIds.has(m.id)}
											<span class="maintBadge" title={tr('Media sedang diperbaiki oleh mitra')}>{tr('Maintenance')}</span>
										{/if}
									</div>
								</td>

								<td class="modernTd">
									<div class="categoryBadgeGroup">
										<span class="categoryPill">{cats[0]}</span>
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
								<td class="modernTd" style="text-align:center"><span class="scoreBoxPill">{getAiTrust(m)}</span></td>

								<td class="modernTd" style="text-align:center">
									{#if m.seo_ai_citation === true}
										<span class="aiCitationBadge"><Check size={11} />Sumber AI</span>
									{:else}
										<span class="mutedDash">—</span>
									{/if}
								</td>

								<td class="modernTd">
									<div class="releaseStack">
										<Tooltip.Root>
									<Tooltip.Trigger>
										{#snippet child({ props })}
											<span {...props} class={speed.cls}>
											{#if speed.isSuper}<Zap size={12} />{:else}<Clock size={12} />{/if}
											<span>{speed.label}</span>
											</span>
										{/snippet}
									</Tooltip.Trigger>
									<Tooltip.Portal>
										<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={4}>
										<Tooltip.Arrow class="uiTipArrow" />
										{releaseTip(speed.sub)}
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
											onclick={(e) => { e.stopPropagation(); detail = m; }}
										><span>{tr("Lihat Detail")}</span><ArrowRight size={13} /></button>
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
			<dialog open class="modalCard" aria-label={tr("Buat kampanye")} onclick={(e) => e.stopPropagation()}>
				<form onsubmit={(e) => { e.preventDefault(); void createCampaign(); }}>
					<div class="modalHeader">
						<h2 class="modalTitle"><LayoutGrid size={17} />{tr("Simpan ke Kampanye")}</h2>
						<button class="modalClose" type="button" aria-label={tr("Tutup")} onclick={() => (campaignOpen = false)}><X size={18} /></button>
					</div>
					<div class="campaignSummary">
						<div class="campaignSummaryRow">
							<span>{tr("Media terpilih")}</span><strong>{selectedIds.size}</strong>
						</div>
						<div class="campaignSummaryRow">
							<span>{tr("Total biaya publikasi")}</span>
							<strong>Rp {fmt(totalSelectedPrice)} <span class="campaignSummaryMuted">~ {fmt(totalSelectedCredits)} {tr("Kredit")}</span></strong>
						</div>
						{#if walletAvailableTc !== null}
							<div class="campaignSummaryRow">
								<span>{tr("Saldo kamu")}</span>
								<strong>Rp {fmt(walletAvailableTc * walletRateIdr)} <span class="campaignSummaryMuted">~ {fmt(walletAvailableTc)} {tr("Kredit")}</span></strong>
							</div>
							{#if totalSelectedCredits > walletAvailableTc}
								<p class="campaignWalletWarn">
									{tr("Saldo kurang")} ~{fmt(totalSelectedCredits - walletAvailableTc)} {tr("Kredit")} {tr("untuk pesanan ini.")} <a href={`/klien/${brandUrl}/credits`} onclick={openTopUpFromBar}>{tr("Top Up dulu")}</a>
								</p>
							{/if}
						{/if}
					</div>

					<div class="campaignNextSteps">
						<p class="campaignNextTitle">{tr("Setelah ini")}</p>
						<ol>
							<li>{tr("Siapkan artikel untuk tiap media — bisa dibuat AI, atau kamu unggah sendiri.")}</li>
							<li>{tr("Pemeriksaan mutu otomatis: keamanan konten, tautan, dan kesiapan dibaca mesin AI.")}</li>
							<li>{tr("Terbit ke media")}{#if slowestRelease} {tr("— waktu rilis terlama di pilihanmu:")} <strong>{slowestRelease.label} ({slowestRelease.sub})</strong>{/if}.</li>
						</ol>
					</div>

					{#if checkingDup}
						<p class="modalHint">{tr("Memeriksa pesanan ganda…")}</p>
					{:else if dupHits.length > 0}
						<div class="dupWarn">
							<strong>{tr("Sebagian media sudah dipesan di kampanye aktif lain:")}</strong>
							<ul>
								{#each dupHits as h (h.mediaName + h.campaignName)}
									<li>{h.mediaName} — {h.campaignName}</li>
								{/each}
							</ul>
							<span>{tr("Lanjutkan hanya jika kamu memang ingin memesan media yang sama dua kali.")}</span>
						</div>
					{/if}

					<label class="modalField">
						<span>{tr("Nama Kampanye")} <span class="campaignSummaryMuted">({tr("opsional")})</span></span>
						<input
							class="app-input"
							bind:value={campaignName}
							maxlength={120}
							placeholder={tr("Kosongkan untuk nama otomatis (Kampanye Baru #N)")}
						/>
					</label>
					{#if campaignError}<p class="modalError">{campaignError}</p>{/if}
					<div class="modalFooter">
						<button class="advFilterResetBtn" type="button" onclick={() => (campaignOpen = false)}>{tr("Batal")}</button>
						<button class="advFilterApplyBtn" type="submit" disabled={creating || checkingDup}>
							{creating ? tr('Membuat…') : dupHits.length > 0 ? tr('Pesan Lagi & Buat Kampanye') : tr('Buat Kampanye')}
						</button>
					</div>
				</form>
			</dialog>
		</div>
	{/if}

	<!-- Modal Detail Media -->
	{#if detail}
		{@const d = detail}
		<MediaDetailModal
			media={d}
			onclose={() => (detail = null)}
			onProceed={() => proceedFromDetail(d)}
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
		role="tooltip"
		style="left: {catPopoverPos.left}px; top: {catPopoverPos.top}px;"
	>
		<div class="categoryPopoverTitle">{tr("Semua Kategori")}</div>
		<div class="categoryPopoverList">
			{#each catPopoverCats as c (c)}<span class="categoryPill">{c}</span>{/each}
		</div>
	</div>
{/if}

{#if selectedIds.size > 0}<div style="height:74px"></div>{/if}

<!-- Selection bar -->
{#if selectedIds.size > 0}
	<div class="selectedBottomBar">
		<div class="selectedBarLeft">
			<div class="selectedStatItem">
				<span class="selectedStatLabel">MEDIA TERPILIH</span>
				<span class="selectedStatDomainVal">{selectedIds.size} Media</span>
			</div>
			<div class="selectedBarDivider"></div>
			<div class="selectedStatItem">
				<span class="selectedStatLabel">TOTAL BIAYA PUBLIKASI</span>
				<div class="selectedStatPriceGroup">
					<span class="selectedStatPriceVal">Rp {fmt(totalSelectedPrice)}</span>
					<span class="selectedStatCreditVal">~ {fmt(totalSelectedCredits)} {tr("Kredit")}</span>
				</div>
			</div>
			<div class="selectedBarDivider"></div>
			<div class="selectedStatItem">
				<span class="selectedStatLabel">SALDO KREDIT</span>
				{#if walletAvailableTc === null}
					<span class="selectedStatPriceVal" style="opacity:.6;">—</span>
				{:else}
					<div class="selectedStatPriceGroup">
						<span class="selectedStatPriceVal">Rp {fmt(walletAvailableTc * walletRateIdr)}</span>
						<span class="selectedStatCreditVal">~ {fmt(walletAvailableTc)} {tr("Kredit")}</span>
					</div>
					{#if totalSelectedCredits > walletAvailableTc}
						<span class="walletShortfall">{tr("Kurang")} ~{fmt(totalSelectedCredits - walletAvailableTc)} {tr("Kredit")} · <a href={`/klien/${brandUrl}/credits`} onclick={openTopUpFromBar}>{tr("Top Up")}</a></span>
					{:else}
						<span class="walletEnough">{tr("Cukup untuk pesanan ini")}</span>
					{/if}
				{/if}
			</div>
		</div>
		<div class="selectedBarRight">
			<button class="selectedCancelBtn" type="button" onclick={() => (selectedIds = new Set())}>{tr("Batal")}</button>
			<button class="selectedExportBtn" type="button" onclick={exportProposal} title={tr("Unduh proposal ringkasan media terpilih (format CSV)")}>
				<FileText size={15} /><span>Ekspor Proposal</span>
			</button>
			<button class="selectedCreateBtn" type="button" disabled={creating} onclick={() => void openCampaignModal()}>
				<Plus size={14} /><span>{tr("Buat Kampanye")}</span>
			</button>
		</div>
	</div>
{/if}

</Tooltip.Provider>

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
	.walletShortfall {
		display: block;
		margin-top: 2px;
		font-size: 11px;
		font-weight: 600;
		color: var(--warning, #b45309);
	}
	.walletShortfall a { text-decoration: underline; }
	.walletEnough {
		display: block;
		margin-top: 2px;
		font-size: 11px;
		color: var(--success, #15803d);
	}
	/* Ringkasan konfirmasi di modal kampanye. */
	.campaignSummary {
		margin: 4px 0 12px;
		padding: 12px 14px;
		border: 1px solid var(--border);
		border-radius: 12px;
		display: grid;
		gap: 6px;
	}
	.campaignSummaryRow {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 12px;
		font-size: 13px;
	}
	.campaignSummaryMuted { font-weight: 500; color: var(--muted-foreground); }
	.campaignWalletWarn { margin: 2px 0 0; font-size: 12px; color: var(--warning, #b45309); }
	.campaignWalletWarn a { text-decoration: underline; font-weight: 600; }
	.campaignNextSteps {
		margin: 0 0 12px;
		padding: 12px 14px;
		border-radius: 12px;
		background: var(--muted, #f6f7f9);
	}
	.campaignNextTitle {
		margin: 0 0 6px;
		font-size: 12px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--muted-foreground);
	}
	.campaignNextSteps ol { margin: 0; padding-left: 18px; display: grid; gap: 4px; font-size: 13px; }
	.dupWarn {
		margin: 0 0 12px;
		padding: 10px 12px;
		border-radius: 10px;
		border: 1px solid var(--warning, #f59e0b);
		background: #fffbeb;
		font-size: 12.5px;
		display: grid;
		gap: 4px;
	}
	.dupWarn ul { margin: 0; padding-left: 18px; }
</style>
