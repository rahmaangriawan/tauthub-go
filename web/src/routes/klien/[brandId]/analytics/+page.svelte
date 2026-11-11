<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Analisis Kinerja: dampak tiap kampanye dari klik tautan pendek (30 hari).
	// Angka dihitung server (paket impact): bot dan pratinjau tautan dibuang, satu
	// pengunjung sekali per hari. Tidak ada klik per media atau negara.
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
		published: number;
		problem: number;
		clicks: number;
		visitors: number;
		daily: { day: string; clicks: number }[];
	};
	type Overview = { days: number; total_clicks: number; truncated: boolean; campaigns: Camp[] };

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
			const res = await apiRequest(`/v1/brands/${brandId}/impact`);
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

	const fmt = (n: number) => (n ?? 0).toLocaleString('id-ID');

	function spark(points: { clicks: number }[]): string {
		if (points.length < 2) return '';
		const max = Math.max(1, ...points.map((p) => p.clicks));
		const step = 100 / (points.length - 1);
		return points.map((p, i) => `${i === 0 ? 'M' : 'L'}${(i * step).toFixed(1)},${(26 - (p.clicks / max) * 24).toFixed(1)}`).join(' ');
	}
</script>

<PageTitle title={tr('Analisis Kinerja')} />

<section class="an">
	<header class="an-head">
		<div>
			<h1 class="page-title">{tr('Analisis Kinerja')}</h1>
			<p class="page-desc">{tr('Klik dari artikel ke situs Anda, per kampanye, 30 hari terakhir.')}</p>
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
		<div class="app-card an-total">
			<p class="an-total-num">{fmt(data.total_clicks)}</p>
			<p class="an-muted">{tr('klik dari semua kampanye dalam 30 hari terakhir')}</p>
		</div>

		{#if data.campaigns.length === 0}
			<div class="app-card an-empty">
				<p class="an-title">{tr('Belum ada kampanye')}</p>
				<p class="an-muted">{tr('Buat kampanye dan terbitkan artikel, lalu klik dari artikel muncul di sini.')}</p>
			</div>
		{:else}
			<ul class="app-card an-list">
				{#each data.campaigns as c (c.id)}
					<li>
						<a class="an-row" href={`/klien/${brandUrl}/analytics/${c.id}`}>
							<span class="an-name">
								<span class="an-title">{c.name}</span>
								<span class="an-muted">
									{c.published} {tr('artikel terbit')}{#if c.problem > 0}, <strong class="an-bad">{c.problem} {tr('tautan bermasalah')}</strong>{/if}
								</span>
							</span>
							<span class="an-nums">
								<span><small>{tr('Klik')}</small><b>{fmt(c.clicks)}</b></span>
								<span><small>{tr('Pengunjung')}</small><b>{fmt(c.visitors)}</b></span>
							</span>
							{#if c.clicks > 0}
								<svg viewBox="0 0 100 28" preserveAspectRatio="none" class="an-spark" role="img" aria-label={tr('Klik per hari')}>
									<path d={spark(c.daily)} />
								</svg>
							{:else}
								<span class="an-muted an-none">{tr('Belum ada klik')}</span>
							{/if}
							<ArrowRight size={16} class="an-arrow" />
						</a>
					</li>
				{/each}
			</ul>
		{/if}

		<p class="an-note">
			{tr('Klik dihitung dari tautan pendek kampanye. Klik dari bot dan pratinjau tautan tidak dihitung, dan satu pengunjung dihitung sekali per hari. Klik per media dan per negara belum tersedia.')}
			{#if data.truncated}{tr('Data sangat banyak, sebagian klik lama tidak ikut terhitung.')}{/if}
		</p>
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
	.an-bad { color: var(--destructive); font-weight: 500; }
	.an-total { padding: 24px; }
	.an-total-num { font-size: clamp(32px, 5vw, 44px); font-weight: 600; letter-spacing: -0.02em; line-height: 1.1; color: var(--foreground); font-variant-numeric: tabular-nums; }
	.an-empty { padding: 24px; display: flex; flex-direction: column; gap: 4px; }
	.an-list { padding: 0; list-style: none; overflow: hidden; }
	.an-list li + li { border-top: 1px solid var(--border); }
	.an-row { display: grid; grid-template-columns: minmax(0, 1.6fr) auto minmax(0, 1fr) auto; align-items: center; gap: 20px; padding: 16px 20px; color: inherit; text-decoration: none; transition: background-color 140ms ease; }
	.an-row:hover { background: color-mix(in srgb, var(--secondary) 60%, transparent); }
	.an-name { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
	.an-nums { display: flex; gap: 24px; }
	.an-nums span { display: flex; flex-direction: column; }
	.an-nums small { font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.an-nums b { font-size: 20px; line-height: 28px; font-weight: 600; color: var(--foreground); font-variant-numeric: tabular-nums; }
	.an-spark { width: 100%; height: 28px; }
	.an-spark path { fill: none; stroke: var(--primary); stroke-width: 1.8; vector-effect: non-scaling-stroke; }
	.an-none { text-align: left; }
	:global(.an-arrow) { color: var(--muted-foreground); transition: transform 140ms ease; }
	.an-row:hover :global(.an-arrow) { transform: translateX(4px); }
	@media (max-width: 720px) {
		.an-row { grid-template-columns: minmax(0, 1fr) auto; gap: 12px; }
		.an-spark, .an-none { grid-column: 1 / -1; }
		.an-nums { grid-column: 1 / -1; }
	}
	@media (prefers-reduced-motion: reduce) {
		.an-row, :global(.an-arrow) { transition: none; }
	}
</style>
