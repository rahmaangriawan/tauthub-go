<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Analisis Kinerja: ringkasan hasil tiap kampanye dari data yang tercatat di
	// Tauthub (artikel terbit, media, kondisi tautan, garansi). Tauthub tidak
	// memantau klik atau perilaku pembaca.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ArrowRight, RefreshCw } from '@lucide/svelte';
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
	type Overview = { totals: { published: number; media: number; problem: number; claims_open: number }; campaigns: Camp[] };

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state('');
	let data = $state<Overview | null>(null);

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

	const fmtDate = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '-');
</script>

<PageTitle title={tr('Analisis Kinerja')} />

<section class="an">
	<header class="an-head">
		<div>
			<h1 class="page-title">{tr('Analisis Kinerja')}</h1>
			<p class="page-desc">{tr('Hasil tiap kampanye: artikel terbit, media, dan kondisi tautan.')}</p>
		</div>
		<button class="app-btn app-btn-soft an-refresh" type="button" disabled={loading || refreshing} onclick={() => void load(true)}>
			<RefreshCw size={15} class={refreshing ? 'animate-spin' : ''} />
			{tr('Muat ulang')}
		</button>
	</header>

	{#if error}
		<div class="app-card an-error" role="alert">{error}</div>
	{/if}

	{#if loading}
		<div class="app-card an-loading">{tr('Memuat analisis kinerja…')}</div>
	{:else if data}
		<dl class="an-tiles">
			<div class="app-card an-tile"><dt>{tr('Artikel terbit')}</dt><dd>{data.totals.published}</dd></div>
			<div class="app-card an-tile"><dt>{tr('Media')}</dt><dd>{data.totals.media}</dd></div>
			<div class="app-card an-tile"><dt>{tr('Tautan bermasalah')}</dt><dd class:an-bad={data.totals.problem > 0}>{data.totals.problem}</dd></div>
			<div class="app-card an-tile"><dt>{tr('Garansi menunggu pilihan')}</dt><dd class:an-bad={data.totals.claims_open > 0}>{data.totals.claims_open}</dd></div>
		</dl>

		{#if data.campaigns.length === 0}
			<div class="app-card an-empty">
				<p class="an-title">{tr('Belum ada kampanye')}</p>
				<p class="an-muted">{tr('Buat kampanye dan terbitkan artikel, lalu hasilnya muncul di sini.')}</p>
			</div>
		{:else}
			<ul class="app-card an-list">
				{#each data.campaigns as c (c.id)}
					<li>
						<a class="an-row" href={`/klien/${brandUrl}/analytics/${c.id}`}>
							<span class="an-name">
								<span class="an-title">{c.name}</span>
								<span class="an-muted">
									{c.last_published_at ? `${tr('Terbit terakhir')} ${fmtDate(c.last_published_at)}` : tr('Belum ada artikel terbit')}
									{#if c.in_progress > 0}, {c.in_progress} {tr('sedang diproses')}{/if}
								</span>
							</span>
							<span class="an-nums">
								<span><small>{tr('Artikel')}</small><b>{c.published}</b></span>
								<span><small>{tr('Media')}</small><b>{c.media}</b></span>
								<span>
									<small>{tr('Tautan')}</small>
									<b class:an-bad={c.problem > 0}>{c.published === 0 ? '-' : c.problem > 0 ? `${c.problem} ${tr('bermasalah')}` : tr('Sehat')}</b>
								</span>
							</span>
							<ArrowRight size={16} class="an-arrow" />
						</a>
					</li>
				{/each}
			</ul>
		{/if}

		<p class="an-note">{tr('Kondisi tautan diperiksa otomatis oleh Link Guardian. Tauthub tidak memantau klik atau perilaku pembaca.')}</p>
	{/if}
</section>

<style>
	.an { display: flex; flex-direction: column; gap: 24px; }
	.an-head { display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: 12px; }
	.an-refresh { min-height: 44px; }
	.an-error { padding: 12px 16px; font-size: 14px; border-color: var(--destructive); color: var(--destructive); }
	.an-loading { padding: 20px; text-align: center; font-size: 14px; color: var(--muted-foreground); }
	.an-muted { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.an-note { font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.an-title { font-size: 14px; font-weight: 600; line-height: 22px; color: var(--foreground); word-break: break-word; }
	.an-bad { color: var(--destructive) !important; }
	.an-tiles { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px; }
	.an-tile { padding: 16px 20px; display: flex; flex-direction: column; gap: 4px; }
	.an-tile dt { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.an-tile dd { font-size: 28px; line-height: 36px; font-weight: 600; letter-spacing: -0.02em; color: var(--foreground); font-variant-numeric: tabular-nums; }
	.an-empty { padding: 24px; display: flex; flex-direction: column; gap: 4px; }
	.an-list { padding: 0; list-style: none; overflow: hidden; }
	.an-list li + li { border-top: 1px solid var(--border); }
	.an-row { display: grid; grid-template-columns: minmax(0, 1.4fr) minmax(0, 1.6fr) auto; align-items: center; gap: 20px; padding: 16px 20px; color: inherit; text-decoration: none; transition: background-color 140ms ease; }
	.an-row:hover { background: color-mix(in srgb, var(--secondary) 60%, transparent); }
	.an-name { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
	.an-nums { display: flex; gap: 28px; }
	.an-nums span { display: flex; flex-direction: column; }
	.an-nums small { font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.an-nums b { font-size: 16px; line-height: 24px; font-weight: 600; color: var(--foreground); font-variant-numeric: tabular-nums; }
	:global(.an-arrow) { color: var(--muted-foreground); transition: transform 140ms ease; }
	.an-row:hover :global(.an-arrow) { transform: translateX(4px); }
	@media (max-width: 860px) {
		.an-tiles { grid-template-columns: repeat(2, minmax(0, 1fr)); }
		.an-row { grid-template-columns: minmax(0, 1fr) auto; gap: 12px; }
		.an-nums { grid-column: 1 / -1; }
	}
	@media (prefers-reduced-motion: reduce) {
		.an-row, :global(.an-arrow) { transition: none; }
	}
</style>
