<script lang="ts">
	// Pelacak 404 klien — port LinkMonitorSection (link-monitor-section.tsx).
	// Struktur: header → 4 kartu statistik → kartu filter (cari + STATUS/MEDIA/
	// TANGGAL + Reset) → notice → tabel 5 kolom (Artikel sortable, Media,
	// Status, Terakhir Update, Aksi) → footer paginasi (ukuran halaman).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		ArrowDownUp, ExternalLink, CircleCheck, Image as ImageIcon,
		Link2Off, LoaderCircle, RotateCcw, Search, TriangleAlert, ClipboardList
	} from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import Select from '$lib/components/select.svelte';

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}`);

	type Item = {
		id: string; document_title: string | null; media_name: string | null; media_type?: string | null;
		media_category?: string | null; published_url: string | null; published_at: string | null;
		state: string | null; http_status?: number | null; consecutive_failures?: number | null;
		error_message?: string | null; last_checked_at: string | null;
		featured_image_url?: string | null; draft_featured_image_url?: string | null;
	};
	type Summary = { watched: number; live: number; gone: number; unreachable: number; no_url: number };

	let summary = $state<Summary | null>(null);
	let items = $state<Item[]>([]);
	let total = $state(0);
	let offset = $state(0);
	let pageSize = $state(10);
	let stateFilter = $state('problematic');
	let mediaFilter = $state('');
	let daysFilter = $state('');
	let searchInput = $state('');
	let query = $state('');
	let sortAsc = $state(false);
	let loading = $state(true);
	let error = $state('');
	let requestingId = $state<string | null>(null);
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
			else error = await readApiError(sRes, 'Gagal memuat ringkasan pelacak.');
			if (lRes.ok) {
				const data = (await lRes.json()) as { publications?: Item[]; items?: Item[]; total?: number };
				items = data.publications ?? data.items ?? [];
				total = data.total ?? 0;
			} else {
				error = await readApiError(lRes, 'Gagal memuat daftar artikel.');
			}
		} catch (reason) {
			error = reason instanceof Error ? reason.message : 'Gagal memuat data pelacak.';
		} finally {
			loading = false;
		}
	}

	// STATE_META — salinan persis link-monitor-section.tsx.
	const STATE_META: Record<string, { label: string; text: string; bg: string; dot: string; hint: string }> = {
		gone: { label: 'Hilang (404)', text: 'var(--destructive)', bg: 'color-mix(in srgb, var(--destructive) 12%, transparent)', dot: 'var(--destructive)', hint: 'Artikel tidak ditemukan di URL-nya (404/410)' },
		unreachable: { label: 'Tak terjangkau', text: 'var(--warning)', bg: 'var(--warning-soft)', dot: 'var(--warning)', hint: 'Situs menolak akses atau sedang bermasalah — belum tentu hilang' },
		live: { label: 'Tayang', text: 'var(--success)', bg: 'var(--success-soft)', dot: 'var(--success)', hint: 'Artikel masih bisa diakses' },
		no_url: { label: 'Belum ada URL', text: 'var(--muted-foreground)', bg: 'var(--muted)', dot: 'var(--muted-foreground)', hint: 'Belum ada URL terbit yang bisa dicek' }
	};
	const FALLBACK_META = { label: 'Belum dicek', text: 'var(--muted-foreground)', bg: 'var(--muted)', dot: 'var(--muted-foreground)', hint: 'Pemeriksaan belum pernah berjalan untuk artikel ini' };
	function metaFor(state: string | null) {
		if (!state) return FALLBACK_META;
		const m = STATE_META[state];
		if (m) return m;
		return { ...FALLBACK_META, label: state, hint: '' };
	}

	/** mediaSubtitle — parity helper yang sama. */
	function mediaSubtitle(mediaType?: string | null, category?: string | null): string {
		if (mediaType === 'blogger') return 'Blog Personal';
		if (mediaType === 'mainstream') return 'Media Online';
		return category || 'Media Online';
	}

	/** formatRelativeTime — parity helper yang sama. */
	function formatRelativeTime(date: string | null): string {
		if (!date) return 'Belum pernah';
		const diff = Date.now() - new Date(date).getTime();
		const min = Math.floor(diff / 60000);
		const hour = Math.floor(diff / 3600000);
		const day = Math.floor(diff / 86400000);
		if (day === 0) {
			if (min < 1) return 'Baru saja';
			if (hour < 1) return `${min} menit yang lalu`;
			return `${hour} jam yang lalu`;
		}
		if (day === 1) return 'Kemarin';
		if (day < 30) return `${day} hari yang lalu`;
		const months = Math.floor(day / 30);
		if (months < 12) return `${months} bulan yang lalu`;
		return `${Math.floor(months / 12)} tahun yang lalu`;
	}

	const fmtDateTime = (v: string | null) =>
		v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : 'Belum pernah';

	/** Normalisasi URL thumbnail: origin upload lama → path relatif (parity). */
	function thumbUrl(url: string | null | undefined): string | null {
		if (!url) return null;
		if (url.startsWith('https://app.tauthub.com/uploads/')) return url.replace('https://app.tauthub.com', '');
		return url;
	}

	// Sortir judul pada halaman aktif saja (parity sortedItems).
	const sortedItems = $derived.by(() => {
		const list = [...items];
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
		stateFilter = 'problematic';
		mediaFilter = '';
		daysFilter = '';
		offset = 0;
		void load();
	}
	function commitSearch() {
		query = searchInput.trim();
		offset = 0;
		void load();
	}

	async function requestReplacement(item: Item) {
		requestingId = item.id;
		error = '';
		try {
			const res = await apiRequest(`${apiBase}/link-monitor/request-replacement`, {
				method: 'POST',
				body: JSON.stringify({ publicationId: item.id })
			});
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal mengirim permintaan penggantian.'));
			notice = 'Permintaan penggantian terkirim. Tim Tauthub akan menindaklanjuti.';
		} catch (reason) {
			error = reason instanceof Error ? reason.message : 'Gagal mengirim permintaan penggantian.';
		} finally {
			requestingId = null;
		}
	}

	const statCards = $derived([
		{ label: 'Artikel Dipantau', value: summary?.watched ?? 0, hint: 'Artikel tayang dalam masa garansi 12 bulan', icon: ClipboardList },
		{ label: 'Tayang', value: summary?.live ?? 0, hint: 'Terverifikasi masih bisa diakses', icon: CircleCheck },
		{ label: 'Hilang (404)', value: summary?.gone ?? 0, hint: 'URL-nya mengembalikan 404 atau 410', icon: Link2Off },
		{ label: 'Tak Terjangkau', value: summary?.unreachable ?? 0, hint: 'Situs menolak akses atau sedang bermasalah', icon: TriangleAlert }
	]);
</script>

<section class="mx-auto max-w-6xl">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Pelacak 404</h1>
		<p class="mt-1 text-sm text-muted-foreground">
			Kami memeriksa berkala artikel Anda yang sudah tayang selama masa garansi 12 bulan.
		</p>
	</header>

	{#if loading && !summary}
		<p class="mt-6 flex items-center gap-2 text-sm text-muted-foreground">
			<LoaderCircle size={16} class="animate-spin" /> Memuat Pelacak 404
		</p>
	{:else if error && !summary}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm font-medium">Halaman tidak dapat dimuat</p>
			<p class="mt-1 text-sm text-muted-foreground">{error}</p>
			<button class="app-btn app-btn-soft mt-3" type="button" onclick={() => void load()}>Coba lagi</button>
		</div>
	{:else}
		<!-- 4 kartu statistik -->
		<div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
			{#each statCards as c (c.label)}
				<div class="app-card flex items-center justify-between gap-3 p-4">
					<div>
						<p class="text-[13px]" style="color: var(--muted-foreground);">{c.label}</p>
						<p class="text-[34px] font-bold leading-tight tabular-nums">{c.value}</p>
						<p class="text-xs" style="color: var(--muted-foreground);">{c.hint}</p>
					</div>
					<span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg" style="background-color: var(--accent); color: var(--primary);">
						<c.icon size={18} strokeWidth={2.4} />
					</span>
				</div>
			{/each}
		</div>

		<!-- Kartu filter -->
		<div class="app-card mt-4 p-3">
			<div class="flex flex-wrap items-end gap-3">
				<label class="flex min-w-[220px] flex-1 flex-col gap-1">
					<span class="text-[11px] font-semibold tracking-wide" style="color: var(--muted-foreground);">CARI</span>
					<span class="app-input flex items-center gap-2 !py-1.5">
						<Search size={15} class="text-muted-foreground" />
						<input
							class="w-full bg-transparent text-sm outline-none"
							placeholder="Cari artikel, media, atau URL…"
							bind:value={searchInput}
							onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); commitSearch(); } }}
						/>
					</span>
				</label>
				<label class="flex flex-col gap-1">
					<span class="text-[11px] font-semibold tracking-wide" style="color: var(--muted-foreground);">STATUS</span>
					<Select
						bind:value={stateFilter}
						ariaLabel="Filter status"
						minWidth={170}
						onchange={applyFilters}
						options={[
							{ value: 'problematic', label: '• Yang bermasalah' },
							{ value: 'all', label: 'Semua Status' },
							{ value: 'gone', label: 'Hilang (404)' },
							{ value: 'unreachable', label: 'Tak terjangkau' },
							{ value: 'live', label: 'Tayang' },
							{ value: 'no_url', label: 'Belum ada URL' }
						]}
					/>
				</label>
				<label class="flex flex-col gap-1">
					<span class="text-[11px] font-semibold tracking-wide" style="color: var(--muted-foreground);">MEDIA</span>
					<Select
						bind:value={mediaFilter}
						ariaLabel="Filter media"
						placeholder="Semua Media"
						minWidth={150}
						onchange={applyFilters}
						options={[
							{ value: '', label: 'Semua Media' },
							...mediaOptions.map((m) => ({ value: m, label: m }))
						]}
					/>
				</label>
				<label class="flex flex-col gap-1">
					<span class="text-[11px] font-semibold tracking-wide" style="color: var(--muted-foreground);">TANGGAL</span>
					<Select
						bind:value={daysFilter}
						ariaLabel="Filter tanggal"
						placeholder="Semua Tanggal"
						minWidth={150}
						onchange={applyFilters}
						options={[
							{ value: '', label: 'Semua Tanggal' },
							{ value: '7', label: '7 Hari Terakhir' },
							{ value: '30', label: '30 Hari Terakhir' },
							{ value: '90', label: '3 Bulan Terakhir' }
						]}
					/>
				</label>
				<button class="app-btn app-btn-soft" type="button" title="Reset semua filter ke kondisi awal" onclick={resetFilters}>
					<RotateCcw size={14} /> Reset Filter
				</button>
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
				<p class="font-medium">Tidak ada masalah pada artikel Anda</p>
				<p class="mx-auto mt-1 max-w-lg text-sm text-muted-foreground">
					Semua artikel yang tayang dalam masa garansi 12 bulan terpantau masih bisa diakses. Halaman ini
					akan menampilkan artikel begitu ada yang hilang atau tidak terjangkau.
				</p>
			</div>
		{:else}
			<div class="app-card mt-4">
				<!-- table-layout fixed + lebar kolom persen: isi kolom dipotong
				     dengan ellipsis, sehingga tabel tidak pernah melebihi kartu. -->
					<table class="linkmon-table w-full text-sm">
						<colgroup>
							<col style="width:34%" /><col style="width:13%" /><col style="width:13%" />
							<col style="width:19%" /><col style="width:21%" />
						</colgroup>
						<thead>
							<tr style="background-color: var(--muted);">
								<th class="px-5 py-3.5 text-left text-[11.5px] font-semibold uppercase tracking-wide" style="color: var(--muted-foreground);">
									<button class="inline-flex items-center gap-1.5" type="button" onclick={() => (sortAsc = !sortAsc)}>
										<span>Artikel</span><ArrowDownUp size={13} strokeWidth={2.4} />
									</button>
								</th>
								<th class="px-5 py-3.5 text-left text-[11.5px] font-semibold uppercase tracking-wide" style="color: var(--muted-foreground);">Media</th>
								<th class="px-5 py-3.5 text-left text-[11.5px] font-semibold uppercase tracking-wide" style="color: var(--muted-foreground);">Status</th>
								<th class="px-5 py-3.5 text-left text-[11.5px] font-semibold uppercase tracking-wide" style="color: var(--muted-foreground);">Terakhir Update</th>
								<th class="px-5 py-3.5 text-right text-[11.5px] font-semibold uppercase tracking-wide" style="color: var(--muted-foreground);">Aksi</th>
							</tr>
						</thead>
						<tbody>
							{#each sortedItems as item (item.id)}
								{@const meta = metaFor(item.state)}
								{@const thumb = thumbUrl(item.featured_image_url ?? item.draft_featured_image_url)}
								<tr class="linkmon-row border-b last:border-0" style="vertical-align: top;">
									<td class="linkmon-cell linkmon-cell-article px-5 py-4">
										<div class="flex items-center gap-3">
											{#if thumb && !imgError[item.id]}
												<img
													src={thumb}
													alt={item.document_title ?? 'Cover artikel'}
													class="h-12 w-[72px] shrink-0 rounded-md object-cover"
													loading="lazy"
													onerror={() => (imgError = { ...imgError, [item.id]: true })}
												/>
											{:else}
												<span class="flex h-12 w-[72px] shrink-0 items-center justify-center rounded-md" style="background-color: var(--muted); color: var(--muted-foreground);" title={item.document_title ?? undefined}>
													<ImageIcon size={20} />
												</span>
											{/if}
											<div class="min-w-0">
												<p class="linkmon-title font-medium">{item.document_title ?? 'Tanpa judul'}</p>
												{#if item.published_url}
													<a class="linkmon-url inline-flex items-center gap-1 text-xs" style="color: var(--primary);" href={item.published_url} target="_blank" rel="noreferrer" title={item.published_url}>
														<span class="truncate">{item.published_url}</span><ExternalLink size={12} />
													</a>
												{/if}
											</div>
										</div>
									</td>
									<td class="linkmon-cell px-5 py-4" data-label="Media">
										<p class="font-medium">{item.media_name ?? '—'}</p>
										<p class="text-xs text-muted-foreground">{mediaSubtitle(item.media_type, item.media_category)}</p>
									</td>
									<td class="linkmon-cell px-5 py-4" data-label="Status">
										<span
											class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-semibold"
											style="color: {meta.text}; background-color: {meta.bg}; border: 1px solid color-mix(in srgb, {meta.dot} 20%, transparent);"
											title={meta.hint}
										>
											<span class="h-1.5 w-1.5 rounded-full" style="background-color: {meta.dot};"></span>
											<span>{meta.label}</span>
										</span>
									</td>
									<td class="linkmon-cell px-5 py-4" data-label="Terakhir diperiksa">
										<p>{item.last_checked_at ? fmtDateTime(item.last_checked_at) : 'Belum pernah'}</p>
										<p class="text-xs text-muted-foreground">{formatRelativeTime(item.last_checked_at)}</p>
									</td>
									<td class="linkmon-cell px-5 py-4 text-right" data-label="Aksi">
										{#if item.state === 'gone'}
											<button
												class="app-btn app-btn-soft linkmon-action !min-h-8 !text-xs"
												type="button"
												disabled={requestingId !== null}
												onclick={() => void requestReplacement(item)}
											>
												{#if requestingId === item.id}<LoaderCircle size={14} class="animate-spin" />{:else}<RotateCcw size={14} />{/if}
												{requestingId === item.id ? 'Mengirim…' : 'Ajukan penggantian'}
											</button>
										{:else}
											<span class="text-[13px]" style="color: var(--muted-foreground);">—</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>

				<!-- Footer paginasi -->
				<div class="flex flex-wrap items-center justify-between gap-3 border-t px-5 py-3">
					<span class="text-sm text-muted-foreground">Menampilkan {startItem}–{endItem} dari {total} artikel</span>
					<div class="flex flex-wrap items-center gap-2">
						<Select
							value={String(pageSize)}
							ariaLabel="Jumlah per halaman"
							minWidth={150}
							align="end"
							onchange={(v) => { pageSize = Number(v); offset = 0; void load(); }}
							options={[
								{ value: '10', label: '10 per halaman' },
								{ value: '20', label: '20 per halaman' },
								{ value: '50', label: '50 per halaman' }
							]}
						/>
						<button class="app-btn app-btn-soft !min-h-8 !px-2.5 !text-xs" type="button" disabled={currentPage <= 1 || loading} onclick={() => { offset = Math.max(0, (currentPage - 2) * pageSize); void load(); }}>
							‹ Sebelumnya
						</button>
						<span class="text-sm text-muted-foreground">{currentPage} / {totalPages}</span>
						<button class="app-btn app-btn-soft !min-h-8 !px-2.5 !text-xs" type="button" disabled={currentPage >= totalPages || loading} onclick={() => { offset = currentPage * pageSize; void load(); }}>
							Berikutnya ›
						</button>
					</div>
				</div>
			</div>
		{/if}
	{/if}
</section>

<style>
	/* Tabel Pelacak 404: layout fixed supaya kolom tidak melebar mengikuti
	   panjang URL. Semua isi panjang dipotong dengan ellipsis, sehingga tidak
	   pernah muncul scroll horizontal (parity proporsi kolom app lama). */
	.linkmon-table { table-layout: fixed; }

	.linkmon-title {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		word-break: break-word;
	}

	.linkmon-url {
		max-width: 100%;
		overflow: hidden;
	}
	.linkmon-url span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }

	.linkmon-action { max-width: 100%; white-space: normal; line-height: 1.25; }

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

		.linkmon-row { padding: 14px 16px; }
		.linkmon-row:last-child { border-bottom: 0; }

		.linkmon-cell { padding: 0; text-align: left !important; }
		.linkmon-cell + .linkmon-cell { margin-top: 10px; }
		.linkmon-cell-article { margin-bottom: 4px; }

		.linkmon-cell[data-label]::before {
			content: attr(data-label);
			display: block;
			margin-bottom: 3px;
			color: var(--muted-foreground);
			font-size: 10.5px;
			font-weight: 700;
			letter-spacing: 0.06em;
			text-transform: uppercase;
		}
	}
</style>
