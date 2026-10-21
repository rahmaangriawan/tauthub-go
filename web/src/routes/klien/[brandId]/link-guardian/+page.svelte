<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import { sameOriginAssetUrl } from '$lib/media/cover';

	// Pelacak 404 klien — port LinkMonitorSection (link-monitor-section.tsx).
	// Struktur: header → 4 kartu statistik → kartu filter (cari + STATUS/MEDIA/
	// TANGGAL + Reset) → notice → tabel 5 kolom (Artikel sortable, Media,
	// Status, Terakhir Update, Aksi) → footer paginasi (ukuran halaman).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		Activity, ArrowDownUp, ArrowUp, CircleAlert, CircleCheck, ExternalLink, Filter as FilterIcon,
		Image as ImageIcon, Info, LoaderCircle, RefreshCw, RotateCcw, Search, TriangleAlert
	} from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import Tooltip from '$lib/components/tooltip.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import Select from '$lib/components/select.svelte';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}`);

	type Item = {
		id: string; document_title: string | null; media_name: string | null; media_type?: string | null;
		media_category?: string | null; published_url: string | null; published_at: string | null;
		state: string | null; http_status?: number | null; consecutive_failures?: number | null;
		error_message?: string | null; last_checked_at: string | null;
		featured_image_url?: string | null; draft_featured_image_url?: string | null;
		// Link Guardian: hasil pemeriksaan tautan di dalam artikel + kelayakan klaim.
		in_article_state?: string | null; in_article_anchor?: string | null; in_article_found?: number | null;
		in_article_checked_at?: string | null; in_article_error?: string | null;
		domain_verified?: boolean | null; in_warranty?: boolean | null;
		claim_id?: string | null; claim_status?: string | null;
		refund_idr?: number | null; refund_tc?: number | null;
	};
	type Summary = {
		watched: number; live: number; gone: number; unreachable: number; no_url: number;
		// Ditambahkan server: waktu pengecekan terakhir + jatah pengecekan manual.
		last_checked_at?: string | null; next_manual_check_at?: string | null;
		manual_check_available?: boolean;
	};

	let summary = $state<Summary | null>(null);
	let items = $state<Item[]>([]);
	let total = $state(0);
	let offset = $state(0);
	let pageSize = $state(10);
	let stateFilter = $state('all');
	let mediaFilter = $state('');
	let daysFilter = $state('');
	// Panel "Filter" (media + tanggal): dibuka tombol Filter, ditutup klik di luar
	// atau Escape.
	let filterOpen = $state(false);
	const closeFilterPanel = () => (filterOpen = false);
	const advancedFilterCount = $derived((mediaFilter ? 1 : 0) + (daysFilter ? 1 : 0));
	let searchInput = $state('');
	let query = $state('');
	// Sortir satu kolom aktif: judul (perilaku lama) atau terakhir update.
	let sortKey = $state<'title' | 'updated'>('title');
	let sortAsc = $state(false);
	// Tanggal: klik pertama selalu tampilkan yang terbaru dulu.
	let dateDesc = $state(true);
	function toggleTitleSort() {
		if (sortKey !== 'title') { sortKey = 'title'; return; }
		sortAsc = !sortAsc;
	}
	function toggleDateSort() {
		if (sortKey !== 'updated') { sortKey = 'updated'; return; }
		dateDesc = !dateDesc;
	}
	let loading = $state(true);
	let error = $state('');
	let claimingId = $state<string | null>(null);
	let crawling = $state(false);
	let notice = $state('');
	let imgError = $state<Record<string, boolean>>({});

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	// Toolbar pencarian sticky tepat di bawah topbar aplikasi (mobile). Tinggi
	// topbar bisa berubah, jadi offset diukur dari elemen aslinya, bukan angka
	// tetap — pola yang sama dengan sticky header tabel di halaman Media.
	$effect(() => {
		if (typeof window === 'undefined') return;
		const sync = () => {
			const bar = document.querySelector('.app-utility-bar');
			const h = bar ? Math.round(bar.getBoundingClientRect().height) : 0;
			document.documentElement.style.setProperty('--lg-sticky-top', (h > 0 ? h : 60) + 'px');
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

	// Muat ulang saat filter/halaman berubah (parity: summary & daftar satu paket).
	$effect(() => {
		void offset;
		void pageSize;
		void stateFilter;
		void query;
		void mediaFilter;
		void daysFilter;
		if (loading) return;
		void load();
	});

	async function load() {
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams({ limit: String(pageSize), offset: String(offset) });
			if (stateFilter && stateFilter !== 'all') params.set('state', stateFilter);
			if (query) params.set('q', query);
			if (mediaFilter) params.set('media', mediaFilter);
			if (daysFilter) params.set('days', daysFilter);
			const [sRes, lRes] = await Promise.all([
				apiRequest(`${apiBase}/link-monitor?view=summary`),
				apiRequest(`${apiBase}/link-monitor?${params}`)
			]);
			if (sRes.ok) summary = (await sRes.json()) as Summary;
			else error = await readApiError(sRes, tr('Gagal memuat ringkasan pelacak.'));
			if (lRes.ok) {
				const data = (await lRes.json()) as { publications?: Item[]; items?: Item[]; total?: number };
				items = data.publications ?? data.items ?? [];
				total = data.total ?? 0;
			} else {
				error = await readApiError(lRes, tr('Gagal memuat daftar artikel.'));
			}
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal memuat data pelacak.');
		} finally {
			loading = false;
		}
	}

	// STATE_META — salinan persis link-monitor-section.tsx.
	const STATE_META: Record<string, { label: string; text: string; bg: string; dot: string; hint: string }> = $derived({
		gone: { label: '404', text: 'var(--destructive)', bg: 'color-mix(in srgb, var(--destructive) 12%, transparent)', dot: 'var(--destructive)', hint: tr('404 berarti halaman tidak ditemukan di alamatnya. Penyebab umum: artikel dihapus atau ditarik dari situs media, atau alamat artikelnya berubah. Tautan seperti ini bisa diajukan penggantiannya.') },
		unreachable: { label: tr('Tak terjangkau'), text: 'var(--warning)', bg: 'var(--warning-soft)', dot: 'var(--warning)', hint: tr('Situs menolak akses atau sedang bermasalah — belum tentu hilang') },
		live: { label: tr('Tayang'), text: 'var(--success)', bg: 'var(--success-soft)', dot: 'var(--success)', hint: tr('Artikel masih bisa diakses') },
		no_url: { label: tr('Belum ada URL'), text: 'var(--muted-foreground)', bg: 'var(--muted)', dot: 'var(--muted-foreground)', hint: tr('Belum ada URL terbit yang bisa dicek') }
	});
	const FALLBACK_META = $derived({ label: tr('Belum dicek'), text: 'var(--muted-foreground)', bg: 'var(--muted)', dot: 'var(--muted-foreground)', hint: tr('Pemeriksaan belum pernah berjalan untuk artikel ini') });
	function metaFor(state: string | null) {
		if (!state) return FALLBACK_META;
		const m = STATE_META[state];
		if (m) return m;
		return { ...FALLBACK_META, label: state, hint: '' };
	}

	/** formatRelativeTime — parity helper yang sama. */
	function formatRelativeTime(date: string | null): string {
		if (!date) return tr('Belum pernah');
		const diff = Date.now() - new Date(date).getTime();
		const min = Math.floor(diff / 60000);
		const hour = Math.floor(diff / 3600000);
		const day = Math.floor(diff / 86400000);
		if (day === 0) {
			if (min < 1) return tr('Baru saja');
			if (hour < 1) return tr('{n} menit yang lalu').replace('{n}', String(min));
			return tr('{n} jam yang lalu').replace('{n}', String(hour));
		}
		if (day === 1) return tr('Kemarin');
		if (day < 30) return tr('{n} hari yang lalu').replace('{n}', String(day));
		const months = Math.floor(day / 30);
		if (months < 12) return tr('{n} bulan yang lalu').replace('{n}', String(months));
		return tr('{n} tahun yang lalu').replace('{n}', String(Math.floor(months / 12)));
	}

	const fmtDateTime = (v: string | null) =>
		v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : tr('Belum pernah');

	/** Normalisasi URL thumbnail: origin upload absolut → path relatif. */
	const thumbUrl = sameOriginAssetUrl;

	// Sortir pada halaman aktif saja (parity sortedItems). Judul memakai
	// perilaku lama; kolom terakhir update membandingkan waktu, dengan entri
	// "belum pernah dicek" selalu di bawah.
	const sortedItems = $derived.by(() => {
		const list = [...items];
		if (sortKey === 'updated') {
			const waktu = (v: string | null) => (v ? new Date(v).getTime() : null);
			list.sort((a, b) => {
				const at = waktu(a.last_checked_at);
				const bt = waktu(b.last_checked_at);
				if (at === null && bt === null) return 0;
				if (at === null) return 1;
				if (bt === null) return -1;
				return dateDesc ? bt - at : at - bt;
			});
			return list;
		}
		list.sort((a, b) => {
			const at = (a.document_title ?? '').toLowerCase();
			const bt = (b.document_title ?? '').toLowerCase();
			return sortAsc ? at.localeCompare(bt) : bt.localeCompare(at);
		});
		return list;
	});

	// Opsi media dari halaman aktif saja (parity mediaOptions).
	const mediaOptions = $derived(
		[...new Set(items.map((i) => i.media_name).filter((n): n is string => Boolean(n)))].sort()
	);

	const startItem = $derived(total > 0 ? offset + 1 : 0);
	const endItem = $derived(Math.min(offset + pageSize, total));
	const currentPage = $derived(Math.floor(offset / pageSize) + 1);
	const totalPages = $derived(Math.max(1, Math.ceil(total / pageSize)));

	function applyFilters() {
		offset = 0;
		void load();
	}
	function resetFilters() {
		searchInput = '';
		query = '';
		stateFilter = 'all';
		mediaFilter = '';
		daysFilter = '';
		offset = 0;
		filterOpen = false;
		void load();
	}
	function commitSearch() {
		query = searchInput.trim();
		offset = 0;
		void load();
	}

	/** Label sinkronisasi bergaya "Hari ini, 08:30 WIB" (zona Asia/Jakarta). */
	function syncLabel(iso: string | null | undefined): string {
		if (!iso) return tr('Belum pernah');
		const at = new Date(iso);
		const dayFmt = new Intl.DateTimeFormat('id-ID', { timeZone: 'Asia/Jakarta', day: 'numeric', month: 'short', year: 'numeric' });
		const timeFmt = new Intl.DateTimeFormat('id-ID', { timeZone: 'Asia/Jakarta', hour: '2-digit', minute: '2-digit', hour12: false });
		const day = dayFmt.format(at);
		const label = day === dayFmt.format(new Date()) ? tr('Hari ini') : day;
		return `${label}, ${timeFmt.format(at).replace('.', ':')} WIB`;
	}

	/** Antrekan pengecekan tautan untuk brand ini (jeda 1x sehari ditegakkan server). */
	async function crawlNow() {
		crawling = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`${apiBase}/link-monitor/check`, { method: 'POST' });
			if (res.status === 429) {
				const data = await res.json().catch(() => null);
				notice = data?.error ? `${data.error} ${tr('Coba lagi nanti.')}` : tr('Pengecekan manual dibatasi 1x sehari.');
				return;
			}
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal mengantrekan pengecekan.'));
				return;
			}
			const data = await res.json();
			const queued = data?.queued_publications ?? 0;
			notice = queued > 0
				? `${tr('Antrean pengecekan dibuat:')} ${queued} ${tr('artikel.')}${data?.capped ? ' ' + tr('Dibatasi 200 artikel per sekali jalan, sisanya menyusul pada siklus berikutnya.') : ''} ${tr('Hasil muncul setelah worker selesai.')}`
				: tr('Semua artikel baru saja diperiksa, belum ada yang perlu dicek ulang.');
			await load();
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal mengantrekan pengecekan.');
		} finally {
			crawling = false;
		}
	}

	// Angka kartu selalu dari summary server (bukan konstanta). `tone` = aksen
	// warna border/label/ikon, `strong` = keterangan ikut berwarna tone,
	// `info` = keterangan tambahan di ikon info.
	/** Status deteksi gabungan: kondisi URL artikel + tautan DI DALAM artikel. */
	function detectionMeta(item: Item): { label: string; text: string; bg: string; dot: string } {
		if (item.state === 'gone') {
			return { label: tr('HILANG (404)'), text: 'var(--destructive)', bg: 'color-mix(in srgb, var(--destructive) 12%, transparent)', dot: 'var(--destructive)' };
		}
		if (item.in_article_state === 'removed') {
			return { label: tr('TAUTAN DIHAPUS'), text: 'var(--destructive)', bg: 'color-mix(in srgb, var(--destructive) 12%, transparent)', dot: 'var(--destructive)' };
		}
		if (item.state === 'unreachable') {
			return { label: tr('TAK TERJANGKAU'), text: 'var(--warning)', bg: 'var(--warning-soft)', dot: 'var(--warning)' };
		}
		if (item.state === 'live') {
			return { label: tr('200 OK (AMAN)'), text: 'var(--success)', bg: 'var(--success-soft)', dot: 'var(--success)' };
		}
		return { label: tr('Belum ada URL'), text: 'var(--muted-foreground)', bg: 'var(--muted)', dot: 'var(--muted-foreground)' };
	}

	/**
	 * Aksi garansi per baris. Urutannya sengaja: klaim yang sudah diproses
	 * menang atas kelayakan, lalu garansi habis, lalu media tak terverifikasi,
	 * baru tombol klaim muncul saat artikelnya benar-benar rusak.
	 */
	function claimState(item: Item): 'refunded' | 'processing' | 'expired' | 'not_covered' | 'none' | 'claimable' {
		if (item.claim_status === 'refunded') return 'refunded';
		if (item.claim_status) return 'processing';
		if (item.in_warranty === false) return 'expired';
		if (item.domain_verified === false) return 'not_covered';
		const broken = item.state === 'gone' || item.in_article_state === 'removed';
		return broken ? 'claimable' : 'none';
	}

	const claimStatusLabel: Record<string, string> = $derived({
		submitted: tr('Klaim diajukan'), checking: tr('Sedang diperiksa'),
		contacted: tr('Menghubungi pemilik domain'), refunded: tr('Dana dikembalikan'),
		rejected: tr('Klaim ditolak')
	});

	/** Ajukan klaim garansi: satu endpoint dengan pengajuan penggantian. */
	async function submitClaim(item: Item) {
		claimingId = item.id;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`${apiBase}/link-monitor/request-replacement`, {
				method: 'POST',
				body: JSON.stringify({ publicationId: item.id })
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal mengajukan klaim.'));
				return;
			}
			notice = tr('Klaim diajukan. Tim Tauthub akan memeriksa kondisi tautan dan menindaklanjuti.');
			await load();
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal mengajukan klaim.');
		} finally {
			claimingId = null;
		}
	}

	const statCards = $derived([
		{ label: tr('Total Dipantau'), value: summary?.watched ?? 0, hint: tr('Dalam masa garansi aktif'), icon: Activity, tone: 'var(--muted-foreground)', strong: false, info: null as string | null },
		{ label: tr('Tayang & Aman'), value: summary?.live ?? 0, hint: tr('Halaman & Tautan aktif (200 OK)'), icon: CircleCheck, tone: 'var(--success)', strong: false, info: null },
		{ label: tr('Tak Terjangkau'), value: summary?.unreachable ?? 0, hint: tr('Sistem keamanan media memblokir bot'), icon: CircleAlert, tone: 'var(--warning)', strong: false, info: tr('Situs menolak akses atau sedang bermasalah — belum tentu hilang') },
		{ label: tr('Bermasalah'), value: summary?.gone ?? 0, hint: tr('404 atau Tautan dihapus (SLA Klaim)'), icon: CircleAlert, tone: 'var(--destructive)', strong: true, info: null }
	]);
</script>

<PageTitle title="Link Guardian" />

<svelte:window
	onclick={(e) => { if (!(e.target as HTMLElement)?.closest?.('.lg-filter-wrap')) closeFilterPanel(); }}
	onkeydown={(e) => { if (e.key === 'Escape') closeFilterPanel(); }}
/>

<section>
	<header class="lg-header">
		<div class="min-w-0">
			<p class="text-xs font-semibold uppercase tracking-wide" style="color: var(--primary);">LINK GUARDIAN</p>
			<h1 class="mt-1 text-2xl font-semibold tracking-tight">{tr("Link Guardian")}</h1>
			<p class="lg-sub mt-1 text-sm text-muted-foreground">
					{tr('Sistem memantau tautan Anda secara berkala.')}
					<strong>{tr('Garansi tayang mengikuti kebijakan media')}</strong>
					{tr('— 12 bulan, atau permanen bila medianya menjamin tautan tidak pernah dilepas. Masa garansi berlaku sejak tanggal tayang, dan eksklusif untuk')}
					<strong style="color: var(--primary);">{tr('Media Terverifikasi')}</strong>.
			</p>
		</div>
		<div class="lg-sync">
			<p class="lg-sync-time">
				{tr('Terakhir sinkronisasi:')} <span class="lg-sync-value">{syncLabel(summary?.last_checked_at)}</span>
			</p>
			<button
				class="app-btn app-btn-soft lg-crawl"
				type="button"
				disabled={crawling || summary?.manual_check_available === false}
				title={summary?.manual_check_available === false
					? `${tr('Pengecekan manual dibatasi 1x sehari. Bisa dicoba lagi')} ${syncLabel(summary?.next_manual_check_at)}.`
					: tr('Antrekan pengecekan tautan sekarang')}
				onclick={() => void crawlNow()}
			>
				<RefreshCw size={14} class={crawling ? 'animate-spin' : ''} />
				{crawling ? tr('Mengantrekan…') : tr('Crawl Sekarang')}
			</button>
			<span class="lg-manual-note"><Info size={12} /> {tr('Cek Manual Maks. 1x Sehari')}</span>
		</div>
	</header>

	{#if loading && !summary}
		<p class="mt-6 flex items-center gap-2 text-sm text-muted-foreground">
			<LoaderCircle size={16} class="animate-spin" />{tr("Memuat Link Guardian")}</p>
	{:else if error && !summary}
		<div class="app-card mt-5 px-6 py-10 text-center">
			<p class="text-sm font-medium">{tr("Halaman tidak dapat dimuat")}</p>
			<p class="mt-1 text-sm text-muted-foreground">{error}</p>
			<button class="app-btn app-btn-soft mt-3" type="button" onclick={() => void load()}>Coba lagi</button>
		</div>
	{:else}
		<!-- 4 kartu statistik -->
		<div class="lg-cards">
			{#each statCards as c (c.label)}
				<div class="app-card lg-card" style="--tone: {c.tone};">
					<p class="lg-card-label">
						<c.icon size={14} strokeWidth={2.4} />
						<span>{c.label}</span>
						{#if c.info}
							<Tooltip content={c.info} label={c.info} maxWidth={280}>
								<span class="lg-card-info"><Info size={12} /></span>
							</Tooltip>
						{/if}
					</p>
					<p class="lg-card-value">{c.value}</p>
					<p class="lg-card-hint" style={c.strong ? 'color: var(--tone); font-weight: 500;' : ''}>{c.hint}</p>
				</div>
			{/each}
		</div>

		<!-- Toolbar: pencarian lebar, dropdown status, lalu tombol Filter -->
		<div class="app-card lg-toolbar">
			<label class="lg-search">
				<Search size={15} class="text-muted-foreground" />
				<input
					class="w-full bg-transparent text-sm outline-none"
					placeholder={tr("Cari artikel, media, atau URL…")}
					aria-label={tr("Cari artikel, media, atau URL…")}
					bind:value={searchInput}
					onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); commitSearch(); } }}
				/>
			</label>
			<span class="lg-divider" aria-hidden="true"></span>
			<Select
				bind:value={stateFilter}
				ariaLabel="Filter status"
				minWidth={205}
				onchange={applyFilters}
				options={[
					{ value: 'all', label: tr('Semua Status') },
					{ value: 'gone', label: tr('Bermasalah saja (SLA Klaim)') },
					{ value: 'unreachable', label: tr('Tak Terjangkau') },
					{ value: 'live', label: tr('Tayang & Aman') },
					{ value: 'no_url', label: tr('Belum ada URL') }
				]}
			/>
			<span class="lg-divider" aria-hidden="true"></span>
			<div class="lg-filter-wrap">
				<button
					class="lg-filter-btn"
					type="button"
					aria-expanded={filterOpen}
					aria-controls="lg-filter-panel"
					onclick={(e) => { e.stopPropagation(); filterOpen = !filterOpen; }}
				>
					<FilterIcon size={14} /> {tr('Filter')}
					{#if advancedFilterCount > 0}<span class="lg-filter-count">{advancedFilterCount}</span>{/if}
				</button>
				{#if filterOpen}
					<div class="lg-filter-pop" id="lg-filter-panel">
						<div class="lg-pop-field">
							<span class="lg-pop-label">{tr('Media')}</span>
							<Select
								bind:value={mediaFilter}
								ariaLabel="Filter media"
								placeholder={tr("Semua Media")}
								minWidth={190}
								onchange={applyFilters}
								options={[
									{ value: '', label: tr('Semua Media') },
									...mediaOptions.map((m) => ({ value: m, label: m }))
								]}
							/>
						</div>
						<div class="lg-pop-field">
							<span class="lg-pop-label">{tr('Tanggal')}</span>
							<Select
								bind:value={daysFilter}
								ariaLabel="Filter tanggal"
								placeholder={tr("Semua Tanggal")}
								minWidth={190}
								onchange={applyFilters}
								options={[
									{ value: '', label: tr('Semua Tanggal') },
									{ value: '7', label: tr('7 Hari Terakhir') },
									{ value: '30', label: tr('30 Hari Terakhir') },
									{ value: '90', label: tr('3 Bulan Terakhir') }
								]}
							/>
						</div>
						<div class="lg-pop-actions">
							<button class="lg-pop-reset" type="button" onclick={resetFilters}><RotateCcw size={13} /> {tr('Reset filter')}</button>
							<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" type="button" onclick={closeFilterPanel}>{tr('Terapkan')}</button>
						</div>
					</div>
				{/if}
			</div>
		</div>

		{#if notice}
			<div class="mt-3 rounded-lg px-4 py-2.5 text-sm" style="background-color: var(--success-soft); border: 1px solid color-mix(in srgb, var(--success) 30%, transparent); color: var(--success);" role="status">
				{notice}
			</div>
		{/if}
		{#if error}
			<div class="app-card mt-3 px-6 py-6 text-center">
				<p class="text-sm text-muted-foreground">{error}</p>
				<button class="app-btn app-btn-soft mt-2" type="button" onclick={() => void load()}>Coba lagi</button>
			</div>
		{/if}

		{#if !loading && items.length === 0}
			<div class="app-card mt-4 px-6 py-12 text-center">
				<p class="font-medium">{tr("Tidak ada masalah pada artikel Anda")}</p>
				<p class="mx-auto mt-1 max-w-lg text-sm text-muted-foreground">{tr("Semua artikel yang tayang dalam masa garansi medianya masih terpantau bisa diakses (12 bulan, atau tanpa batas untuk media dengan tautan permanen). Halaman ini akan menampilkan artikel begitu ada yang hilang atau tidak terjangkau.")}</p>
			</div>
		{:else}
			<div class="app-card mt-4 lg-list-card">
				<!-- Kolom mengikuti kebutuhan konten (lebar minimum per kolom di <style>);
				     tabel digulir lokal di dalam kartu bila lebih lebar dari area. -->
					<table class="linkmon-table w-full text-sm">
						<colgroup>
							<col class="lg-col-1" /><col class="lg-col-2" /><col class="lg-col-3" />
							<col class="lg-col-4" /><col class="lg-col-5" /><col class="lg-col-6" />
						</colgroup>
						<thead>
							<tr>
								<th class="lg-th lg-th-left" aria-sort={sortKey === 'title' ? (sortAsc ? 'ascending' : 'descending') : 'none'}>
									<button class="lg-th-btn" type="button" onclick={toggleTitleSort} aria-label={tr('Urutkan berdasarkan judul')}>
										<span>{tr('Informasi Artikel & Tautan')}</span><ArrowDownUp size={12} strokeWidth={2.4} />
									</button>
								</th>
								<th class="lg-th lg-th-left">{tr('Publisher')}</th>
								<th class="lg-th lg-th-left">{tr('Status Verifikasi')}</th>
								<th class="lg-th lg-th-left">{tr('Status Deteksi')}</th>
								<th class="lg-th lg-th-left" aria-sort={sortKey === 'updated' ? (dateDesc ? 'descending' : 'ascending') : 'none'}>
									<button class="lg-th-btn" type="button" onclick={toggleDateSort} aria-label={sortKey === 'updated' && !dateDesc ? tr('Urutkan dari yang terbaru') : tr('Urutkan dari yang terlama')}>
										<span>{tr('Waktu Cek')}</span>
										{#if sortKey === 'updated' && !dateDesc}<ArrowUp size={12} strokeWidth={2.4} />{:else}<ArrowDownUp size={12} strokeWidth={2.4} />{/if}
									</button>
								</th>
								<th class="lg-th lg-th-right">{tr('Aksi Garansi')}</th>
							</tr>
						</thead>
						<tbody>
							{#each sortedItems as item (item.id)}
								{@const meta = metaFor(item.state)}
								{@const det = detectionMeta(item)}
								{@const cs = claimState(item)}
								{@const thumb = thumbUrl(item.featured_image_url ?? item.draft_featured_image_url)}
								<tr class="linkmon-row lg-row">
									<td class="linkmon-cell linkmon-cell-article lg-cell">
										<div class="lg-article">
											{#if thumb && !imgError[item.id]}
												<img
													src={thumb}
													alt={item.document_title ?? tr('Cover artikel')}
													class="lg-thumb"
													loading="lazy"
													onerror={() => (imgError = { ...imgError, [item.id]: true })}
												/>
											{:else}
												<span class="lg-thumb lg-thumb-empty" title={item.document_title ?? undefined}>
													<ImageIcon size={15} />
												</span>
											{/if}
											<div class="min-w-0">
												{#if item.published_url}
													<a class="lg-url" href={item.published_url} target="_blank" rel="noreferrer" title={item.published_url}>
														<span class="truncate">{item.published_url}</span><ExternalLink size={11} />
													</a>
												{/if}
												<p class="lg-doctitle">{item.document_title ?? tr('Tanpa judul')}</p>
												{#if item.in_article_anchor}
													<p class="lg-anchor">{tr('Anchor:')} <strong>{item.in_article_anchor}</strong></p>
												{/if}
											</div>
										</div>
									</td>
									<td class="linkmon-cell lg-cell" data-label={tr('Publisher')}>
										<p class="lg-media">{item.media_name ?? '—'}</p>
										<p class="lg-media-sub">{tr('Tgl Tayang:')} {item.published_at ? fmtDateTime(item.published_at) : '—'}</p>
									</td>
									<td class="linkmon-cell lg-cell" data-label={tr('Status Verifikasi')}>
										{#if item.domain_verified}
											<span class="lg-verified"><CircleCheck size={11} strokeWidth={2.6} /> VERIFIED</span>
										{:else}
											<span class="lg-unverified"><CircleAlert size={11} strokeWidth={2.6} /> UNVERIFIED</span>
										{/if}
									</td>
									<td class="linkmon-cell lg-cell" data-label={tr('Status Deteksi')}>
										{#snippet statusBadge()}
											<span class="lg-badge" style="color: {det.text}; background-color: {det.bg};">
												<span class="lg-badge-dot" style="background-color: {det.dot};"></span>
												<span>{det.label}</span>
											</span>
										{/snippet}
										{#if meta.hint}
											<Tooltip content={meta.hint} label={meta.hint} maxWidth={300}>
												{@render statusBadge()}
											</Tooltip>
										{:else}
											{@render statusBadge()}
										{/if}
										{#if item.in_article_state === 'unknown' && item.in_article_error}
											<p class="lg-time-rel" title={item.in_article_error}>{tr('Isi artikel belum bisa diperiksa')}</p>
										{/if}
									</td>
									<td class="linkmon-cell lg-cell" data-label={tr('Waktu Cek')}>
										<p class="lg-time">{formatRelativeTime(item.last_checked_at)}</p>
										<p class="lg-time-rel">{item.last_checked_at ? fmtDateTime(item.last_checked_at) : tr('Belum pernah')}</p>
									</td>
									<td class="linkmon-cell lg-cell lg-cell-action" data-label={tr('Aksi Garansi')}>
										{#if cs === 'claimable'}
											<button class="lg-claim" type="button" disabled={claimingId !== null} onclick={() => void submitClaim(item)}>
												{#if claimingId === item.id}<LoaderCircle size={12} class="animate-spin" />{:else}<TriangleAlert size={12} />{/if}
												{claimingId === item.id ? tr('Mengirim…') : tr('Ajukan Klaim')}
											</button>
										{:else if cs === 'refunded'}
											<span class="lg-claim-done"><CircleCheck size={12} /> {claimStatusLabel[item.claim_status ?? ''] ?? tr('Dana dikembalikan')}</span>
										{:else if cs === 'processing'}
											<span class="lg-claim-progress">{claimStatusLabel[item.claim_status ?? ''] ?? tr('Diproses')}</span>
										{:else if cs === 'expired'}
											<span class="lg-claim-muted">{tr('Garansi Habis')}</span>
										{:else if cs === 'not_covered'}
											<span class="lg-claim-muted">{tr('Tidak Digaransi')}</span>
										{:else}
											<span class="lg-noaction">—</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>

				<!-- Footer paginasi -->
				<div class="lg-pagination">
					<span class="text-[12.5px] text-muted-foreground">{tr("Menampilkan")} {startItem}–{endItem} {tr("dari")} {total} artikel</span>
					<div class="flex flex-wrap items-center gap-2">
						<Select
							value={String(pageSize)}
							ariaLabel={tr("Jumlah per halaman")}
							minWidth={150}
							align="end"
							onchange={(v) => { pageSize = Number(v); offset = 0; void load(); }}
							options={[
								{ value: '10', label: tr('10 per halaman') },
								{ value: '20', label: tr('20 per halaman') },
								{ value: '50', label: tr('50 per halaman') }
							]}
						/>
						<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" disabled={currentPage <= 1 || loading} onclick={() => { offset = Math.max(0, (currentPage - 2) * pageSize); void load(); }}>
							‹ Sebelumnya
						</button>
						<span class="text-[12.5px] tabular-nums text-muted-foreground">{currentPage} / {totalPages}</span>
						<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" disabled={currentPage >= totalPages || loading} onclick={() => { offset = currentPage * pageSize; void load(); }}>
							Berikutnya ›
						</button>
					</div>
				</div>
			</div>
		{/if}
	{/if}
</section>

<style>
	/* ── Header ─────────────────────────────────────────────────────────── */
	.lg-header { display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: 12px; }
	.lg-sub { max-width: 62ch; }
	.lg-sub strong { color: var(--foreground); }

	/* Sisi kanan header: waktu sinkron terakhir + aksi manual. */
	.lg-sync { display: flex; flex-direction: column; align-items: flex-end; gap: 6px; }
	.lg-sync-time { font-size: 11.5px; color: var(--muted-foreground); }
	.lg-sync-value { font-weight: 600; color: var(--foreground); }
	.lg-crawl { white-space: nowrap; }
	.lg-manual-note {
		display: inline-flex; align-items: center; gap: 5px;
		padding: 3px 8px; border-radius: 999px; font-size: 11px; font-weight: 500;
		color: var(--warning); background-color: var(--warning-soft);
	}

	/* ── Kartu ringkasan: ikon sebaris label, angka besar, border berwarna ── */
	.lg-cards { margin-top: 16px; display: grid; gap: 10px; grid-template-columns: 1fr; }
	@media (min-width: 640px) { .lg-cards { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
	@media (min-width: 1280px) { .lg-cards { grid-template-columns: repeat(4, minmax(0, 1fr)); } }

	.lg-card {
		display: flex; flex-direction: column; padding: 14px 16px;
		border: 1px solid var(--border);
	}
	.lg-card-label {
		display: inline-flex; align-items: center; gap: 6px;
		font-size: 11px; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase;
		color: var(--tone);
	}
	.lg-card-info { display: inline-flex; color: var(--muted-foreground); }
	.lg-card-value { margin-top: 6px; font-size: 30px; font-weight: 700; line-height: 1.1; font-variant-numeric: tabular-nums; }
	.lg-card-hint { margin-top: 5px; font-size: 11.5px; line-height: 1.35; color: var(--muted-foreground); }

	/* ── Toolbar: pencarian lebar, status, tombol Filter ───────────────── */
	.lg-toolbar { margin-top: 12px; display: flex; flex-wrap: wrap; align-items: center; gap: 6px; padding: 8px 12px; }
	.lg-search { display: flex; align-items: center; gap: 8px; flex: 1 1 320px; min-width: 220px; padding: 5px 2px; }
	.lg-divider { width: 1px; height: 24px; flex: none; background-color: var(--border); }
	/* Select di dalam toolbar tampil tanpa kotak sendiri: kartu toolbar yang jadi wadahnya. */
	.lg-toolbar :global(.zs-select-trigger) { border: 0; background: transparent; padding-left: 6px; padding-right: 6px; }

	.lg-filter-wrap { position: relative; margin-left: auto; }
	.lg-filter-btn {
		display: inline-flex; align-items: center; gap: 6px;
		padding: 6px 10px; border-radius: var(--radius-control, 10px);
		font-size: 13px; font-weight: 500; color: var(--foreground);
		transition: background-color 120ms ease;
	}
	.lg-filter-btn:hover { background-color: var(--secondary); }
	.lg-filter-count {
		display: inline-flex; align-items: center; justify-content: center;
		min-width: 16px; height: 16px; padding: 0 4px; border-radius: 999px;
		background-color: var(--primary); color: var(--primary-foreground);
		font-size: 10.5px; font-weight: 700;
	}
	.lg-filter-pop {
		position: absolute; right: 0; top: calc(100% + 8px); z-index: 30;
		width: 252px; display: flex; flex-direction: column; gap: 12px; padding: 14px;
		border-radius: var(--radius, 12px); border: 1px solid var(--border);
		background-color: var(--card); box-shadow: var(--shadow-md);
	}
	.lg-pop-field { display: flex; flex-direction: column; gap: 4px; }
	.lg-pop-label {
		font-size: 10.5px; font-weight: 700; letter-spacing: 0.06em;
		text-transform: uppercase; color: var(--muted-foreground);
	}
	.lg-pop-actions {
		display: flex; align-items: center; justify-content: space-between; gap: 8px;
		border-top: 1px solid var(--border); padding-top: 10px;
	}
	.lg-pop-reset { display: inline-flex; align-items: center; gap: 5px; font-size: 12.5px; font-weight: 500; color: var(--muted-foreground); }
	.lg-pop-reset:hover { color: var(--foreground); }

	/* ── Tabel ─────────────────────────────────────────────────────────── */
	/* Ukuran kolom mengikuti kebutuhan konten (table-layout auto) dengan lebar
	   minimum per kolom, bukan persentase. Persentase membuat kolom status dan
	   waktu menyusut ke ±75px sehingga header dan badge terpecah per karakter.
	   Bila total lebih lebar dari kartu, gulir terjadi di dalam kartu saja. */
	.lg-list-card {
		max-width: 100%;
		min-width: 0;
		overflow-x: auto;
		-webkit-overflow-scrolling: touch;
	}
	/* Override khusus halaman ini: aturan global `main table { min-width: 0 }`
	   (≤1279px) tidak boleh menurunkan lebar tabel di bawah kebutuhan kolom. */
	.lg-list-card .linkmon-table {
		table-layout: auto;
		width: 100%;
		min-width: max-content;
		border-collapse: collapse;
	}
	.lg-list-card .linkmon-table .lg-col-1 { min-width: 280px; max-width: 360px; }
	.lg-list-card .linkmon-table td.linkmon-cell-article { max-width: 360px; }
	.lg-list-card .linkmon-table td.linkmon-cell-article .lg-url { max-width: 100%; }
	.lg-list-card .linkmon-table td.linkmon-cell-article .lg-doctitle { overflow-wrap: anywhere; }
	.lg-list-card .linkmon-table .lg-col-2 { min-width: 140px; }
	.lg-list-card .linkmon-table .lg-col-3 { min-width: 110px; }
	.lg-list-card .linkmon-table .lg-col-4 { min-width: 130px; }
	.lg-list-card .linkmon-table .lg-col-5 { min-width: 130px; }
	.lg-list-card .linkmon-table .lg-col-6 { min-width: 110px; }
	.lg-list-card .linkmon-table thead th { white-space: nowrap; }
	.lg-list-card .linkmon-table .lg-cell { white-space: normal; }
	.lg-list-card .lg-badge,
	.lg-list-card .lg-verified,
	.lg-list-card .lg-unverified,
	.lg-list-card .lg-claim,
	.lg-list-card .lg-claim-done,
	.lg-list-card .lg-claim-progress,
	.lg-list-card .lg-claim-muted { white-space: nowrap; }


	.lg-th {
		padding: 9px 16px; font-size: 10.5px; font-weight: 700; letter-spacing: 0.06em;
		text-transform: uppercase; color: var(--muted-foreground);
		border-bottom: 1px solid var(--border); background-color: transparent;
	}
	.lg-th-left { text-align: left; }
	.lg-th-right { text-align: right; }
	.lg-th-btn {
		display: inline-flex; align-items: center; gap: 5px;
		font: inherit; letter-spacing: inherit; text-transform: inherit; color: inherit;
	}
	.lg-th-btn:hover { color: var(--foreground); }

	.lg-row { border-bottom: 1px solid var(--border); transition: background-color 120ms ease; }
	.lg-row:last-child { border-bottom: 0; }
	.lg-row:hover { background-color: color-mix(in srgb, var(--secondary) 55%, transparent); }
	.lg-cell { padding: 11px 16px; vertical-align: top; }
	.lg-cell-action { text-align: right; }

	.lg-article { display: flex; align-items: center; gap: 10px; min-width: 0; }
	.lg-thumb { width: 56px; height: 38px; flex: none; border-radius: 6px; object-fit: cover; }
	.lg-thumb-empty {
		display: flex; align-items: center; justify-content: center;
		background-color: var(--muted); color: var(--muted-foreground);
	}
	.lg-url {
		display: inline-flex; align-items: center; gap: 4px; max-width: 100%;
		font-size: 12.5px; font-weight: 500; color: var(--primary);
	}
	.lg-url span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }
	.lg-url:hover span { text-decoration: underline; }
	.lg-doctitle {
		margin-top: 2px; font-size: 11.5px; line-height: 1.35; color: var(--muted-foreground);
		display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2;
		-webkit-box-orient: vertical; overflow: hidden; word-break: break-word;
	}

	.lg-media { font-size: 13.5px; font-weight: 500; }
	.lg-media-sub { margin-top: 1px; font-size: 11.5px; color: var(--muted-foreground); }

	.lg-badge {
		display: inline-flex; align-items: center; gap: 5px; max-width: 100%;
		padding: 2px 8px; border-radius: 999px; font-size: 11px; font-weight: 600;
		white-space: nowrap;
	}
	.lg-badge-dot { width: 6px; height: 6px; flex: none; border-radius: 999px; }

	.lg-time { font-size: 12.5px; }
	.lg-time-rel { margin-top: 1px; font-size: 11.5px; color: var(--muted-foreground); }

	.lg-noaction { font-size: 12.5px; color: var(--muted-foreground); }

	/* Kolom tambahan ala dashboard garansi: anchor, status verifikasi, aksi klaim. */
	.lg-anchor { margin-top: 2px; font-size: 11.5px; color: var(--muted-foreground); }
	.lg-anchor strong { color: var(--foreground); font-weight: 600; }

	.lg-verified,
	.lg-unverified {
		display: inline-flex; align-items: center; gap: 4px;
		padding: 2px 8px; border-radius: 6px; font-size: 10.5px; font-weight: 700;
		letter-spacing: 0.04em; border: 1px solid transparent;
	}
	.lg-verified { color: var(--primary); border-color: color-mix(in srgb, var(--primary) 35%, transparent); }
	.lg-unverified { color: var(--muted-foreground); border-color: var(--border); }

	.lg-claim {
		display: inline-flex; align-items: center; gap: 5px;
		min-height: 28px; padding: 0 10px; border-radius: var(--radius-button, 9px);
		font-size: 12px; font-weight: 600; white-space: nowrap;
		color: var(--destructive); border: 1px solid color-mix(in srgb, var(--destructive) 40%, transparent);
		background-color: color-mix(in srgb, var(--destructive) 8%, transparent);
		transition: background-color 120ms ease;
	}
	.lg-claim:hover:not(:disabled) { background-color: color-mix(in srgb, var(--destructive) 15%, transparent); }
	.lg-claim:disabled { opacity: .6; }

	.lg-claim-done { display: inline-flex; align-items: center; gap: 4px; font-size: 11.5px; font-weight: 600; color: var(--success); }
	.lg-claim-progress { font-size: 11.5px; font-weight: 500; color: var(--warning); }
	.lg-claim-muted {
		display: inline-block; padding: 3px 8px; border-radius: 6px;
		font-size: 10.5px; font-weight: 600; color: var(--muted-foreground);
		background-color: var(--muted);
	}

	.lg-pagination {
		display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between;
		gap: 10px; border-top: 1px solid var(--border); padding: 10px 14px;
	}

	/* Toolbar menumpuk di layar sempit supaya kontrol tidak terpotong. */
	@media (max-width: 900px) {
		.lg-search { flex: 1 1 100%; }
		.lg-toolbar .lg-divider { display: none; }
	}

	/* Di bawah 700px tabel 5 kolom tidak bisa muat walau kolom sudah
	   dipersempit (header terpanjang saja butuh ~130px). Baris berubah jadi
	   kartu bertumpuk: tiap sel menampilkan labelnya sendiri lewat data-label,
	   sehingga tidak ada konten terpotong dan tidak ada scroll horizontal. */
	@media (max-width: 700px) {
		.linkmon-table,
		.linkmon-table tbody,
		.linkmon-row,
		.linkmon-cell { display: block; width: 100%; }
		.linkmon-table colgroup,
		.linkmon-table thead { display: none; }

		.lg-row { padding: 12px 14px; }
		.lg-row:last-child { border-bottom: 0; }
		.lg-row:hover { background-color: transparent; }

		.lg-cell { padding: 0; text-align: left !important; }
		.lg-cell + .lg-cell { margin-top: 9px; }
		.linkmon-cell-article { margin-bottom: 3px; }

		.linkmon-cell[data-label]::before {
			content: attr(data-label);
			display: block;
			margin-bottom: 3px;
			color: var(--muted-foreground);
			font-size: 10px;
			font-weight: 700;
			letter-spacing: 0.06em;
			text-transform: uppercase;
		}
	}

	/* ── Mobile: jarak dirapikan supaya tidak berdesakan/bertumpuk ─────────── */
	@media (max-width: 700px) {
		/* Header: subjudul penuh, blok sinkronisasi turun ke baris sendiri. */
		.lg-header { gap: 10px; }
		/* Baris: waktu sinkronisasi penuh satu baris, lalu tombol Crawl + catatan
		   manual berdampingan dalam satu baris. */
		.lg-sync {
			flex-direction: row; flex-wrap: wrap; align-items: center;
			width: 100%; gap: 8px 10px;
		}
		.lg-sync-time { flex: 1 1 100%; }
		/* Tombol Crawl + catatan manual wajib SATU baris: catatannya yang
		   menyusut & membungkus teks, bukan pindah ke baris baru. */
		.lg-crawl { flex: 0 0 auto; }
		.lg-manual-note {
			flex: 1 1 0; min-width: 0;
			white-space: normal; line-height: 1.25;
		}
		.lg-sync-time { font-size: 11px; }
		.lg-manual-note { font-size: 10.5px; line-height: 1.3; }

		/* Kartu ringkasan: satu kolom, jarak seragam. */
		.lg-cards { margin-top: 12px; gap: 8px; }
		.lg-card { padding: 12px 14px; }
		.lg-card-value { font-size: 26px; margin-top: 4px; }
		.lg-card-hint { margin-top: 4px; }

		/* Toolbar: pencarian + filter status + tombol filter dalam SATU baris.
		   Tombol filter hanya ikon (teksnya disembunyikan) supaya muat. Saat
		   daftar digulir, toolbar menempel di bawah header aplikasi. */
		.lg-toolbar { margin-top: 10px; padding: 10px 12px; gap: 8px; flex-wrap: nowrap; align-items: center; position: sticky; top: var(--lg-sticky-top, 60px); z-index: 20; }
		.lg-search { flex: 1 1 auto; min-width: 0; }
		.lg-filter-wrap { margin-left: 0; flex: 0 0 auto; }
		.lg-toolbar :global(.lg-status) { flex: 0 0 auto; }
		.lg-filter-btn { font-size: 0; gap: 0; padding: 0 10px; min-width: 38px; justify-content: center; }
		.lg-filter-btn :global(svg) { flex: 0 0 auto; }
		.lg-filter-count { font-size: 10px; margin-left: 4px; }

		/* Baris artikel: padding proporsional + blok yang jelas antar bagian. */
		.lg-row { padding: 14px 12px; }
		.lg-article { flex-wrap: wrap; gap: 10px; }
		.lg-cell + .lg-cell { margin-top: 10px; }
		.linkmon-cell[data-label]::before { margin-bottom: 2px; }
	}
	@media (max-width: 420px) {
		.lg-card { padding: 11px 12px; }
		.lg-row { padding: 12px 10px; }
		.lg-card-value { font-size: 24px; }
	}

	/* ── Kartu artikel (mobile): grid 2 kolom, grup rapi, tanpa overflow ─────
	   Baris tabel berubah jadi kartu: sel 1 (artikel) & 2 (publisher) selebar
	   kartu, sisanya 2 kolom — status/waktu/aksi berdampingan seperti mock. */
	@media (max-width: 700px) {
		.lg-row {
			display: grid;
			grid-template-columns: repeat(2, minmax(0, 1fr));
			gap: 10px 12px;
			width: 100%; max-width: 100%;
			margin: 0 0 12px;
			padding: 14px;
			border: 1px solid var(--border, #e2e8f0);
			border-radius: 14px;
			background: var(--card, #fff);
		}
		.lg-row:last-child { margin-bottom: 0; }
		/* Sel artikel + publisher: selebar kartu. */
		.lg-row > .lg-cell:nth-child(1),
		.lg-row > .lg-cell:nth-child(2) { grid-column: 1 / -1; }
		.lg-cell { padding: 0 !important; margin-top: 0 !important; min-width: 0; }
		/* Divider tipis antar kelompok informasi. */
		.lg-row > .lg-cell:nth-child(2),
		.lg-row > .lg-cell:nth-child(3) {
			padding-top: 10px !important;
			border-top: 1px solid var(--border, #f1f5f9);
		}
		.lg-row > .lg-cell:nth-child(5) {
			padding-top: 10px !important;
			border-top: 1px solid var(--border, #f1f5f9);
		}
		/* Label kecil di atas nilai (sudah ada lewat ::before data-label). */
		.linkmon-cell[data-label]::before { margin-bottom: 4px; }
		/* Judul artikel maksimal 3 baris, URL tetap satu baris terpotong. */
		.lg-doctitle {
			display: -webkit-box; -webkit-line-clamp: 3; line-clamp: 3; -webkit-box-orient: vertical;
			overflow: hidden;
		}
		.lg-url { max-width: 100%; }
		.lg-badge, .lg-unverified, .lg-verified { max-width: 100%; }
	}

	/* Mobile: pembungkus tabel (kartu putih) dibuat transparan supaya kartu
	   artikel tampil langsung di atas latar halaman — tidak ada pita putih. */
	@media (max-width: 700px) {
		.app-card:has(.linkmon-table) {
			background: transparent;
			border-color: transparent;
			box-shadow: none;
			padding: 0;
		}
	}

	/* Mobile: thumbnail artikel diperbesar supaya jelas terlihat. */
	@media (max-width: 700px) {
		.lg-thumb { width: 76px; height: 56px; border-radius: 8px; }
		.lg-thumb :global(svg) { width: 20px; height: 20px; }
		.lg-article { gap: 12px; align-items: flex-start; }
	}
</style>
