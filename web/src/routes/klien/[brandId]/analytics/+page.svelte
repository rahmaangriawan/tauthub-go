<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Analisis Kinerja: ringkasan hasil tiap kampanye dari data yang tercatat di
	// Tauthub (artikel terbit, media, kondisi tautan, garansi). Tauthub tidak
	// memantau klik atau perilaku pembaca.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ArrowRight, ChevronLeft, ChevronRight, FileText, RefreshCw, Search, TriangleAlert } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type Camp = {
		id: string;
		name: string;
		status: string;
		created_at: string | null;
		last_published_at: string | null;
		published: number;
		in_progress: number;
		media: number;
		healthy: number;
		problem: number;
		claims_open: number;
		claims_used: number;
	};
	type Overview = {
		totals: { published: number; media: number; problem: number; claims_open: number };
		campaigns: Camp[];
	};

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);

	const PAGE_SIZE = 10;

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state('');
	let data = $state<Overview | null>(null);
	let searchQuery = $state('');
	let filterStatus = $state<'all' | 'issue' | 'healthy'>('all');
	let currentPage = $state(1);

	async function load(silent = false) {
		if (silent) refreshing = true;
		else loading = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/performance`);
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal memuat analisis kinerja.'));
				return;
			}
			data = (await res.json()) as Overview;
		} catch {
			error = tr('Gagal memuat analisis kinerja.');
		} finally {
			loading = false;
			refreshing = false;
		}
	}

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

	const fmtDate = (v: string | null) =>
		v ? new Date(v).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' }) : '—';

	// Kondisi tautan per kampanye: sehat vs bermasalah dari tautan yang sudah
	// diperiksa Link Guardian. Angka yang dipakai hanya yang memang tercatat.
	function linkInfo(c: Camp) {
		if (c.published === 0) return { tone: 'muted', label: '—', ok: 0, bad: 0 };
		if (c.problem > 0)
			return { tone: 'bad', label: `${c.problem} ${tr('bermasalah')}`, ok: c.healthy, bad: c.problem };
		if (c.healthy > 0) return { tone: 'good', label: tr('Sehat'), ok: c.healthy, bad: 0 };
		return { tone: 'muted', label: tr('Belum diperiksa'), ok: 0, bad: 0 };
	}

	const campaigns = $derived(data?.campaigns ?? []);

	const filtered = $derived.by(() => {
		const q = searchQuery.trim().toLowerCase();
		return campaigns.filter((c) => {
			if (q && !c.name.toLowerCase().includes(q)) return false;
			if (filterStatus === 'issue') return c.problem > 0;
			if (filterStatus === 'healthy') return c.problem === 0 && c.healthy > 0;
			return true;
		});
	});

	const totalPages = $derived(Math.max(1, Math.ceil(filtered.length / PAGE_SIZE)));
	const shown = $derived(filtered.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE));
	const firstIndex = $derived(filtered.length === 0 ? 0 : (currentPage - 1) * PAGE_SIZE + 1);
	const lastIndex = $derived(Math.min(currentPage * PAGE_SIZE, filtered.length));

	// Halaman aktif tetap valid setelah filter/pencarian menyusutkan daftar.
	$effect(() => {
		if (currentPage > totalPages) currentPage = totalPages;
	});

	const pageNumbers = $derived.by(() => {
		if (totalPages <= 5) return Array.from({ length: totalPages }, (_, i) => i + 1) as (number | 'gap')[];
		const list: (number | 'gap')[] = [1];
		if (currentPage > 3) list.push('gap');
		for (let p = Math.max(2, currentPage - 1); p <= Math.min(totalPages - 1, currentPage + 1); p++) list.push(p);
		if (currentPage < totalPages - 2) list.push('gap');
		list.push(totalPages);
		return list;
	});

	function setFilter(next: 'all' | 'issue' | 'healthy') {
		filterStatus = next;
		currentPage = 1;
	}

	function clearFilters() {
		searchQuery = '';
		filterStatus = 'all';
		currentPage = 1;
	}
</script>

<PageTitle title={tr('Analisis Kinerja')} />

<section class="an">
	<header class="an-head">
		<div>
			<h1 class="page-title">{tr('Analisis Kinerja')}</h1>
			<p class="page-desc">{tr('Hasil tiap kampanye: artikel terbit, media, dan kondisi tautan.')}</p>
		</div>
		<button class="app-btn app-btn-soft an-refresh" type="button" disabled={loading || refreshing} onclick={() => void load(true)}>
			<RefreshCw size={16} strokeWidth={1.5} class={refreshing ? 'animate-spin' : ''} />
			{tr('Muat ulang')}
		</button>
	</header>

	{#if error}
		<p class="app-card an-error" role="alert">
			<TriangleAlert size={16} strokeWidth={1.5} />
			{error}
		</p>
	{/if}

	{#if loading}
		<div class="app-card an-summary" aria-hidden="true">
			{#each [1, 2, 3, 4] as cell (cell)}
				<div class="an-cell">
					<span class="an-skel an-skel-label"></span>
					<span class="an-skel an-skel-figure"></span>
				</div>
			{/each}
		</div>
		<div class="app-card an-card" aria-hidden="true">
			<div class="an-toolbar"><span class="an-skel an-skel-label"></span></div>
			{#each [1, 2, 3, 4, 5, 6] as row (row)}
				<span class="an-skel an-skel-row"></span>
			{/each}
		</div>
	{:else if data}
		<!-- Ringkasan seluruh brand: satu papan angka, bukan empat kartu terpisah -->
		<dl class="app-card an-summary">
			<div class="an-cell">
				<dt>{tr('Artikel terbit')}</dt>
				<dd>{data.totals.published.toLocaleString('id-ID')}</dd>
			</div>
			<div class="an-cell">
				<dt>{tr('Media')}</dt>
				<dd>{data.totals.media.toLocaleString('id-ID')}</dd>
			</div>
			<div class="an-cell">
				<dt>{tr('Tautan bermasalah')}</dt>
				<dd class:is-bad={data.totals.problem > 0}>{data.totals.problem.toLocaleString('id-ID')}</dd>
			</div>
			<div class="an-cell">
				<dt>{tr('Garansi menunggu pilihan')}</dt>
				<dd class:is-warn={data.totals.claims_open > 0}>{data.totals.claims_open.toLocaleString('id-ID')}</dd>
			</div>
		</dl>

		<div class="app-card an-card">
			<div class="an-toolbar">
				<h2 class="an-h2">{tr('Kampanye')}</h2>
				<span class="an-count">{filtered.length}</span>

				<div class="an-tools">
					<div class="an-filters" role="group" aria-label={tr('Tautan')}>
						<button class="an-filter" type="button" aria-pressed={filterStatus === 'all'} onclick={() => setFilter('all')}>
							{tr('Semua')}
						</button>
						<button class="an-filter" type="button" aria-pressed={filterStatus === 'issue'} onclick={() => setFilter('issue')}>
							{tr('Tautan bermasalah')}
						</button>
						<button class="an-filter" type="button" aria-pressed={filterStatus === 'healthy'} onclick={() => setFilter('healthy')}>
							{tr('Sehat')}
						</button>
					</div>

					<div class="an-search">
						<Search size={15} strokeWidth={1.5} />
						<input
							type="text"
							value={searchQuery}
							aria-label={tr('Cari kampanye')}
							placeholder={tr('Cari kampanye')}
							oninput={(e) => {
								searchQuery = (e.currentTarget as HTMLInputElement).value;
								currentPage = 1;
							}}
						/>
					</div>
				</div>
			</div>

			{#if campaigns.length === 0}
				<div class="an-empty">
					<span class="an-empty-icon"><FileText size={20} strokeWidth={1.5} /></span>
					<p class="an-empty-title">{tr('Belum ada kampanye')}</p>
					<p class="an-empty-desc">{tr('Buat kampanye dan terbitkan artikel, lalu hasilnya muncul di sini.')}</p>
				</div>
			{:else if filtered.length === 0}
				<div class="an-empty">
					<span class="an-empty-icon"><Search size={20} strokeWidth={1.5} /></span>
					<p class="an-empty-title">{tr('Tidak ada hasil')}</p>
					<p class="an-empty-desc">{tr('Tidak ada kampanye yang cocok dengan filter ini. Kosongkan filter atau ubah kata kunci.')}</p>
					<button class="app-btn app-btn-soft an-empty-btn" type="button" onclick={clearFilters}>{tr('Bersihkan')}</button>
				</div>
			{:else}
				<div class="an-cols an-headrow" aria-hidden="true">
					<span>{tr('Kampanye')}</span>
					<span class="an-right">{tr('Artikel')}</span>
					<span class="an-right">{tr('Media')}</span>
					<span>{tr('Tautan')}</span>
					<span>{tr('Terbit terakhir')}</span>
					<span></span>
				</div>

				<ul class="an-list">
					{#each shown as c (c.id)}
						{@const link = linkInfo(c)}
						{@const checked = link.ok + link.bad}
						<li>
							<a class="an-cols an-row" href={`/klien/${brandUrl}/analytics/${c.id}`}>
								<span class="an-name-cell">
									<span class="an-name">{c.name}</span>
									{#if c.claims_open > 0}
										<span class="an-claim">{c.claims_open} {tr('garansi menunggu pilihan')}</span>
									{/if}
								</span>

								<span class="an-num">
									<span class="an-unit">{tr('Artikel')}</span><span class="an-num-value">{c.published}</span>
								</span>

								<span class="an-num">
									<span class="an-unit">{tr('Media')}</span><span class="an-num-value">{c.media}</span>
								</span>

								<span class="an-link">
									<span class="an-dot" class:is-good={link.tone === 'good'} class:is-bad={link.tone === 'bad'}></span>
									<span class="an-link-text" class:is-good={link.tone === 'good'} class:is-bad={link.tone === 'bad'}>{link.label}</span>
									{#if checked > 0}
										<span class="an-meter" title={`${link.ok} ${tr('Sehat')} · ${link.bad} ${tr('bermasalah')}`}>
											<i class="is-good" style={`width: ${(link.ok / checked) * 100}%`}></i>
											<i class="is-bad" style={`width: ${(link.bad / checked) * 100}%`}></i>
										</span>
									{/if}
								</span>

								<span class="an-date-cell">
									<span class="an-date">{fmtDate(c.last_published_at)}</span>
									{#if c.in_progress > 0}
										<span class="an-date-sub">{c.in_progress} {tr('sedang diproses')}</span>
									{/if}
								</span>

								<span class="an-action">
									<ArrowRight size={16} strokeWidth={1.5} />
								</span>
							</a>
						</li>
					{/each}
				</ul>

				<div class="an-foot">
					<p class="an-foot-info">
						{tr('Menampilkan')} {firstIndex}–{lastIndex} {tr('dari')} {filtered.length} {tr('kampanye')}
					</p>
					<div class="an-pager">
						<button
							class="an-page an-step"
							type="button"
							disabled={currentPage <= 1}
							aria-label={tr('Sebelumnya')}
							onclick={() => (currentPage = currentPage - 1)}
						>
							<ChevronLeft size={16} strokeWidth={1.5} />
						</button>
						{#each pageNumbers as p, i (i)}
							{#if p === 'gap'}
								<span class="an-gap">…</span>
							{:else}
								<button
									class="an-page"
									type="button"
									aria-current={p === currentPage ? 'page' : undefined}
									onclick={() => (currentPage = p as number)}
								>
									{p}
								</button>
							{/if}
						{/each}
						<button
							class="an-page an-step"
							type="button"
							disabled={currentPage >= totalPages}
							aria-label={tr('Berikutnya')}
							onclick={() => (currentPage = currentPage + 1)}
						>
							<ChevronRight size={16} strokeWidth={1.5} />
						</button>
					</div>
				</div>
			{/if}
		</div>

		<p class="an-note">
			{tr('Kondisi tautan diperiksa otomatis oleh Link Guardian. Tauthub tidak memantau klik atau perilaku pembaca.')}
			<a href={`/klien/${brandUrl}/link-guardian`}>{tr('Buka Link Guardian')}</a>
		</p>
	{/if}
</section>

<style>
	.an {
		display: flex;
		flex-direction: column;
		gap: 24px;
	}

	/* ── Kepala halaman ── */
	.an-head {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		justify-content: space-between;
		gap: 16px;
	}
	.an-refresh {
		min-height: 36px;
		color: var(--muted-foreground);
	}
	.an-error {
		display: flex;
		align-items: center;
		gap: 8px;
		margin: 0;
		padding: 12px 16px;
		font-size: 13px;
		line-height: 18px;
		border-color: color-mix(in srgb, var(--destructive) 40%, transparent);
		color: var(--destructive);
	}
	.an-note {
		font-size: 12.5px;
		line-height: 18px;
		color: var(--muted-foreground);
	}
	.an-note a {
		color: var(--foreground);
		text-decoration: underline;
		text-underline-offset: 2px;
	}
	.an-note a:hover {
		color: var(--primary);
	}

	/* ── Papan angka ringkasan: satu bidang, garis rambut di antaranya ── */
	.an-summary {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 1px;
		margin: 0;
		background-color: var(--border);
		overflow: hidden;
	}
	.an-cell {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 20px;
		background-color: var(--card);
	}
	.an-cell dt {
		font-size: 13px;
		line-height: 18px;
		color: var(--muted-foreground);
	}
	.an-cell dd {
		margin: 0;
		font-size: 30px;
		line-height: 34px;
		font-weight: 500;
		letter-spacing: 0;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
	}
	.an-cell dd.is-bad {
		color: var(--destructive);
	}
	.an-cell dd.is-warn {
		color: var(--warning);
	}

	/* ── Kerangka muat: diam, tanpa denyut ── */
	.an-skel {
		border-radius: 4px;
		background-color: var(--muted);
	}
	.an-skel-label {
		width: 96px;
		height: 13px;
	}
	.an-skel-figure {
		width: 48px;
		height: 28px;
	}
	.an-skel-row {
		height: 48px;
		margin: 0 20px 1px;
	}

	/* ── Kartu daftar ── */
	.an-card {
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}
	.an-toolbar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 8px 12px;
		padding: 16px 20px;
		border-bottom: 1px solid var(--border);
	}
	.an-h2 {
		font-size: 15px;
		line-height: 20px;
		font-weight: 500;
		color: var(--foreground);
	}
	.an-count {
		font-size: 12.5px;
		color: var(--muted-foreground);
		font-variant-numeric: tabular-nums;
	}
	.an-tools {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 12px;
		margin-left: auto;
	}
	.an-filters {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		padding: 2px;
		border-radius: 8px;
		background-color: var(--secondary);
	}
	.an-filter {
		min-height: 28px;
		padding: 0 10px;
		border: 0;
		border-radius: 6px;
		background-color: transparent;
		font: inherit;
		font-size: 12.5px;
		color: var(--muted-foreground);
		transition: color 120ms cubic-bezier(0.22, 1, 0.36, 1), background-color 120ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.an-filter:hover {
		color: var(--foreground);
	}
	.an-filter[aria-pressed='true'] {
		background-color: var(--card);
		color: var(--foreground);
		box-shadow: 0 1px 2px rgba(23, 43, 77, 0.06);
	}
	.an-search {
		position: relative;
		display: inline-flex;
		align-items: center;
	}
	.an-search :global(svg) {
		position: absolute;
		left: 9px;
		color: var(--muted-foreground);
		pointer-events: none;
	}
	.an-search input {
		width: 200px;
		min-height: 32px;
		padding: 0 10px 0 30px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background-color: var(--card);
		font-size: 13px;
		color: var(--foreground);
	}
	.an-search input::placeholder {
		color: var(--muted-foreground);
	}
	.an-search input:focus-visible {
		outline: none;
		border-color: var(--primary);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 18%, transparent);
	}

	/* ── Daftar: satu templat kolom untuk kepala tabel dan setiap baris ── */
	.an-cols {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 56px 56px 168px 132px 20px;
		align-items: center;
		gap: 16px;
	}
	.an-headrow {
		padding: 10px 20px;
		border-bottom: 1px solid var(--border);
		font-size: 12px;
		line-height: 16px;
		color: var(--muted-foreground);
	}
	.an-right {
		text-align: right;
	}
	.an-list {
		display: flex;
		flex-direction: column;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.an-list li + li {
		border-top: 1px solid var(--border);
	}
	.an-row {
		padding: 14px 20px;
		color: inherit;
		text-decoration: none;
		transition: background-color 120ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.an-row:hover {
		background-color: color-mix(in srgb, var(--secondary) 55%, transparent);
	}
	.an-row:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: -2px;
	}
	.an-name-cell {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
	}
	.an-name {
		font-size: 14px;
		line-height: 20px;
		font-weight: 500;
		color: var(--foreground);
		word-break: break-word;
	}
	.an-claim {
		font-size: 12.5px;
		line-height: 16px;
		color: var(--warning);
	}
	.an-num {
		display: flex;
		justify-content: flex-end;
		font-size: 14px;
		line-height: 20px;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
	}
	.an-unit {
		display: none;
	}
	.an-link {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
		font-size: 13px;
		line-height: 18px;
		color: var(--muted-foreground);
	}
	.an-dot {
		flex: 0 0 6px;
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background-color: var(--muted-foreground);
	}
	.an-dot.is-good {
		background-color: var(--success);
	}
	.an-dot.is-bad {
		background-color: var(--destructive);
	}
	.an-link-text {
		white-space: nowrap;
	}
	.an-link-text.is-good {
		color: var(--success);
	}
	.an-link-text.is-bad {
		color: var(--destructive);
	}
	.an-meter {
		display: flex;
		flex: 0 0 36px;
		width: 36px;
		height: 4px;
		border-radius: 2px;
		overflow: hidden;
		background-color: var(--muted);
	}
	.an-meter i {
		display: block;
		height: 100%;
	}
	.an-meter i.is-good {
		background-color: var(--success);
	}
	.an-meter i.is-bad {
		background-color: var(--destructive);
	}
	.an-date-cell {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.an-date {
		font-size: 13px;
		line-height: 18px;
		color: var(--muted-foreground);
		font-variant-numeric: tabular-nums;
	}
	.an-date-sub {
		font-size: 12px;
		line-height: 16px;
		color: var(--muted-foreground);
	}
	.an-action {
		display: flex;
		justify-content: flex-end;
		color: var(--muted-foreground);
	}
	.an-row:hover .an-action {
		color: var(--primary);
	}
	.an-action :global(svg) {
		transition: transform 120ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.an-row:hover .an-action :global(svg) {
		transform: translateX(2px);
	}

	/* ── Kaki: halaman ── */
	.an-foot {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 12px 20px;
		border-top: 1px solid var(--border);
	}
	.an-foot-info {
		font-size: 12.5px;
		line-height: 16px;
		color: var(--muted-foreground);
		font-variant-numeric: tabular-nums;
	}
	.an-pager {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}
	.an-page {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 30px;
		height: 30px;
		padding: 0 8px;
		border: 1px solid transparent;
		border-radius: 8px;
		background-color: transparent;
		font: inherit;
		font-size: 12.5px;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
		transition: background-color 120ms cubic-bezier(0.22, 1, 0.36, 1), color 120ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.an-page:hover:not(:disabled):not([aria-current='page']) {
		background-color: var(--secondary);
	}
	.an-page[aria-current='page'] {
		background-color: var(--primary);
		color: var(--primary-foreground);
	}
	.an-page:disabled {
		opacity: 0.35;
	}
	.an-gap {
		padding: 0 4px;
		font-size: 12.5px;
		color: var(--muted-foreground);
	}

	/* ── Keadaan kosong ── */
	.an-empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 6px;
		padding: 40px 24px;
		text-align: center;
	}
	.an-empty-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 44px;
		height: 44px;
		margin-bottom: 6px;
		border-radius: 12px;
		background-color: var(--secondary);
		color: var(--muted-foreground);
	}
	.an-empty-title {
		font-size: 14px;
		line-height: 20px;
		font-weight: 500;
		color: var(--foreground);
	}
	.an-empty-desc {
		font-size: 13px;
		line-height: 18px;
		color: var(--muted-foreground);
		max-width: 42ch;
	}
	.an-empty-btn {
		margin-top: 10px;
		min-height: 36px;
	}

	@media (max-width: 1024px) {
		.an-cols {
			grid-template-columns: minmax(0, 1fr) 48px 48px 150px 20px;
		}
		.an-cols > :nth-child(5) {
			display: none;
		}
	}

	@media (max-width: 768px) {
		.an-summary {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
		.an-tools {
			width: 100%;
			margin-left: 0;
		}
		.an-filters {
			flex: 1;
		}
		.an-filter {
			flex: 1;
			justify-content: center;
		}
		.an-search,
		.an-search input {
			width: 100%;
		}
		.an-search {
			flex: 1 1 100%;
		}
	}

	@media (max-width: 640px) {
		.an-cols {
			grid-template-columns: minmax(0, 1fr) 20px;
			gap: 4px 12px;
		}
		.an-headrow {
			display: none;
		}
		.an-row {
			padding: 16px 20px;
		}
		.an-name-cell {
			grid-column: 1;
		}
		.an-num {
			grid-column: 1;
			justify-content: flex-start;
			gap: 6px;
			font-size: 13px;
			color: var(--muted-foreground);
		}
		.an-unit {
			display: inline;
		}
		.an-link {
			grid-column: 1;
		}
		.an-date-cell {
			grid-column: 1;
		}
		.an-action {
			grid-row: 1;
			grid-column: 2;
			height: 100%;
		}
		.an-page,
		.an-step {
			min-width: 40px;
			height: 40px;
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.an-row,
		.an-filter,
		.an-page,
		.an-action :global(svg) {
			transition: none;
		}
	}
</style>
