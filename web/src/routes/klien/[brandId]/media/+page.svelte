<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Katalog media klien — port setia media-section.tsx (Next.js).
	// Struktur: error banner → header eksplorasi (+stats card) → top bar
	// (search, kategori, filter lanjutan, impor, bookmark) → banner filter →
	// tabel 10 kolom + sort + multi-select → paginasi → modal → selection bar.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import {
		ArrowRight, Bookmark, Bot, Check, Clock, FileText, Info, LayoutGrid,
		Plus, Search, SlidersHorizontal, Trash2, Upload, X, Zap, ExternalLink,
		ChevronLeft, ChevronRight
	} from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { campaignUrlId } from '$lib/campaign-url';
	import { apiRequest, ensureValidSession, readApiError, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { getI18n } from '$lib/i18n/i18n.svelte';
	import MediaDetailModal from './_detail-modal.svelte';
	import Select from '$lib/components/select.svelte';
	import { Tooltip } from 'bits-ui';
	import './media.css';
	import PageTitle from '$lib/components/page-title.svelte';

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
	let sortKey = $state<'traffic' | 'da' | 'ai_trust' | 'ai_cited' | 'speed' | 'price' | 'name'>('name');
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

	const BOOKMARK_KEY = 'tauthub_client_bookmarked_media';

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
		if (m.seo_da != null && m.seo_da > 0) return m.seo_da;
		const q = m.effective_quality ?? m.media_quality_score;
		return q ? Math.max(1, Math.round(q * 0.45)) : '—';
	}
	function getAiTrust(m: Media): string | number {
		if (m.seo_ai_trust != null && m.seo_ai_trust > 0) return m.seo_ai_trust;
		const q = m.effective_quality ?? m.media_quality_score;
		return q ? Math.max(1, Math.round(q * 0.9)) : '—';
	}
	const fmt = (n: number) => n.toLocaleString('id-ID');
	const cleanDomain = (m: Media) =>
		m.website_url ? m.website_url.replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '') : m.id.slice(0, 8);

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
		busy = true;
		err = '';
		try {
			const qs = new URLSearchParams({ limit: '250' });
			if (debouncedSearch) qs.set('q', debouncedSearch);
			const res = await apiRequest(`${apiBase}/media?${qs}`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat media.')));
			const data = (await res.json()) as { media?: Media[]; items?: Media[] };
			rows = data.media ?? data.items ?? [];
		} catch (reason) {
			err = reason instanceof Error ? reason.message : tr('Gagal memuat media.');
		} finally {
			busy = false;
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
			await load();
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
		if (next.has(id)) next.delete(id); else next.add(id);
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
				default: av = a.name.toLowerCase(); bv = b.name.toLowerCase();
			}
			if (av < bv) return -1 * dir;
			if (av > bv) return 1 * dir;
			return a.name.localeCompare(b.name, 'id');
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
	const minPrice = $derived(
		rows.length ? Math.min(...rows.map((m) => m.client_price ?? m.price ?? 0).filter((p) => p > 0)) : 0
	);
	const selectedMedia = $derived(rows.filter((m) => selectedIds.has(m.id)));
	const totalSelectedPrice = $derived(selectedMedia.reduce((s, m) => s + (m.client_price ?? m.price ?? 0), 0));
	const totalSelectedCredits = $derived(selectedMedia.reduce((s, m) => s + (m.credit_cost ?? 0), 0));

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
		selectedIds = allSelected ? new Set() : new Set(ids);
	}

	function exportProposal() {
		const headers = [tr('Nama Media'), 'Website', 'Kategori', 'Kecepatan', 'DA', 'AI Trust', 'AI Cited', tr('Harga IDR'), 'Kredit TC'];
		const esc = (v: unknown) => `"${String(v ?? '').replace(/"/g, '""')}"`;
		const lines = selectedMedia.map((m) => {
			const speed = getReleaseSpeed(m.integration_type);
			const price = m.client_price ?? m.price ?? 0;
			return [
				esc(m.name), esc(m.website_url ?? ''), esc(m.category ?? 'Umum'),
				esc(`${speed.label} (${speed.sub})`), esc(getDa(m)), esc(getAiTrust(m)),
				esc(m.seo_ai_citation === true ? 'Ya' : 'Tidak'), esc(price), esc(m.credit_cost || Math.ceil(price / 1000))
			].join(',');
		});
		const blob = new Blob([[headers.join(','), ...lines].join('\n')], { type: 'text/csv;charset=utf-8;' });
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

	function applyImport() {
		importError = '';
		const lines = importText.split(/\r?\n/).map((l) => l.trim()).filter(Boolean);
		if (!lines.length) {
			importError = tr('Tidak ada baris teks yang terbaca.');
			return;
		}
		const matched = new Set<string>();
		for (const line of lines) {
			const needle = line.toLowerCase().replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '');
			const hit = rows.find(
				(m) => cleanDomain(m).toLowerCase() === needle || m.name.toLowerCase() === needle || m.id === line
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

	async function createCampaign() {
		const name = campaignName.trim();
		if (name.length < 2) {
			campaignError = tr('Nama kampanye minimal 2 karakter.');
			return;
		}
		campaignError = '';
		creating = true;
		try {
			const res = await apiRequest(`${apiBase}/taut/campaigns`, {
				method: 'POST',
				body: JSON.stringify({
					name,
					starts_at: new Date().toISOString(),
					entry_mode: 'manual',
					media_ids: [...selectedIds]
				})
			});
			if (res.ok) {
				const data = (await res.json()) as { id: string; public_id?: string | null };
				campaignOpen = false;
				campaignName = '';
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

	<!-- Header Eksplorasi Media -->
	<div class="explorationHeader">
		<div class="explorationTitleCol">
			<h1 class="explorationTitle">{tr("Eksplorasi Media")}</h1>
			<p class="explorationSubtitle">{tr("Temukan publisher terbaik untuk kampanye PR Anda.")}</p>
		</div>
		<div class="explorationStatsCard">
			<div class="explorationStatItem">
				<span class="explorationStatLabel">TOTAL MEDIA TERSEDIA</span>
				<span class="explorationStatValueBlue">{fmt(rows.length)}</span>
			</div>
			<div class="explorationStatDivider"></div>
			<div class="explorationStatItem">
				<span class="explorationStatLabel">RATA-RATA TERBIT</span>
				<span class="explorationStatValueDark">&lt; 24 Jam</span>
			</div>
			<div class="explorationStatDivider"></div>
			<div class="explorationStatItem">
				<span class="explorationStatLabel">HARGA MULAI DARI</span>
				<span class="explorationStatValueDark">Rp {fmt(minPrice)}</span>
			</div>
		</div>
	</div>

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
							<th class="modernTh modernThSortable" onclick={() => toggleSort('traffic')}>TRAFIK (EST){sortIndicator('traffic')}</th>
							<th class="modernTh modernThSortable" style="text-align:center" onclick={() => toggleSort('da')}>DA{sortIndicator('da')}</th>
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
											<strong>AI Trust Score</strong> {tr("— Skor (0-100) kesiapan infrastruktur web (Schema, Speed) untuk dirayapi dan direkomendasikan oleh mesin LLM.")}
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
											Indikator otomatis via API apakah media ini terdeteksi aktif sebagai rujukan/referensi oleh mesin AI (misal: Perplexity atau AI Overviews).
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
											<span class="mediaHeading">{m.name}</span>
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
										<div class="mediaMetaLine">
											{#if m.website_url}
												<a
													href={m.website_url}
													target="_blank"
													rel="noreferrer"
													class="domainLink"
													onclick={(e) => e.stopPropagation()}
												>{cleanDomain(m)} <ExternalLink size={12} /></a>
											{:else}
												<span>{cleanDomain(m)}</span>
											{/if}
											{#if m.contoh_artikel_url}
												<a
													href={m.contoh_artikel_url}
													target="_blank"
													rel="noreferrer nofollow"
													class="sampleDocLink"
													onclick={(e) => e.stopPropagation()}
												><FileText size={13} /><span>Sample</span></a>
											{/if}
										</div>
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
										<span class="aiCitationBadge"><Bot size={11} />Sumber AI</span>
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
							<span class="bookmarkName">{m.name}</span>
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
			<div class="modalHeader">
				<h2 class="modalTitle"><LayoutGrid size={17} />{tr("Simpan ke Kampanye")}</h2>
				<button class="modalClose" type="button" aria-label={tr("Tutup")} onclick={() => (campaignOpen = false)}><X size={18} /></button>
			</div>
			<p class="modalHint">{tr("Anda memilih")}<strong>{selectedIds.size} media</strong>{tr(". Beri nama kampanye ini untuk masuk ke mode Dapur Produksi (The Writer).")}</p>
			<label class="modalField">
				<span>{tr("Nama Kampanye")}<b class="reqMark">*</b></span>
				<input class="app-input" bind:value={campaignName} placeholder={tr("Contoh: Launching Produk Q3 2026")} />
			</label>
			{#if campaignError}<p class="modalError">{campaignError}</p>{/if}
			<div class="modalFooter">
				<button class="advFilterResetBtn" type="button" onclick={() => (campaignOpen = false)}>{tr("Batal")}</button>
				<button class="advFilterApplyBtn" type="button" disabled={creating} onclick={() => void createCampaign()}>
					{creating ? 'Membuat…' : tr('Buat Kampanye')}
				</button>
			</div>
		</dialog>
	</div>
{/if}

<!-- Modal Detail Media -->
{#if detail}
	<MediaDetailModal media={detail} onclose={() => (detail = null)} />
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
				<span class="selectedStatDomainVal">{selectedIds.size} Domain</span>
			</div>
			<div class="selectedBarDivider"></div>
			<div class="selectedStatItem">
				<span class="selectedStatLabel">TOTAL BIAYA PUBLIKASI</span>
				<div class="selectedStatPriceGroup">
					<span class="selectedStatPriceVal">Rp {fmt(totalSelectedPrice)}</span>
					<span class="selectedStatCreditVal">~ {fmt(totalSelectedCredits)} {tr("Kredit")}</span>
				</div>
			</div>
		</div>
		<div class="selectedBarRight">
			<button class="selectedCancelBtn" type="button" onclick={() => (selectedIds = new Set())}>{tr("Batal")}</button>
			<button class="selectedExportBtn" type="button" onclick={exportProposal} title={tr("Unduh proposal ringkasan media terpilih (format CSV)")}>
				<FileText size={15} /><span>Ekspor Proposal</span>
			</button>
			<button class="selectedCreateBtn" type="button" disabled={creating} onclick={() => { campaignName = ''; campaignError = ''; campaignOpen = true; }}>
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
</style>
