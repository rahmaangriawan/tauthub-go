<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Rincian dampak satu kampanye: klik per hari, perangkat, asal kunjungan, dan
	// kondisi tautan artikel terbit. Hitungan dari server (paket impact).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ArrowLeft } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type Stats = {
		raw_events: number;
		bots_filtered: number;
		clicks: number;
		visitors: number;
		truncated: boolean;
		daily: { day: string; clicks: number }[];
		devices: Record<string, number>;
		referrers: Record<string, number>;
		by_kind: Record<string, number>;
	};
	type Detail = {
		campaign: { id: string; name: string };
		days: number;
		stats: Stats;
		publications: { published: number; live: number; problem: number; last_checked_at: string | null };
	};

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.campaignId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);

	let loading = $state(true);
	let error = $state('');
	let data = $state<Detail | null>(null);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			try {
				const res = await apiRequest(`/v1/brands/${brandId}/campaigns/${campaignId}/impact`);
				if (!res.ok) {
					error = await readApiError(res, tr('Gagal memuat rincian kampanye.'));
				} else {
					data = (await res.json()) as Detail;
				}
			} catch {
				error = tr('Gagal memuat rincian kampanye.');
			} finally {
				loading = false;
			}
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const fmt = (n: number) => (n ?? 0).toLocaleString('id-ID');
	const fmtDate = (v: string | null | undefined) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '-');

	const deviceLabel: Record<string, string> = { desktop: 'Komputer', mobile: 'Ponsel', tablet: 'Tablet' };
	const referrerLabel: Record<string, string> = {
		Direct: 'Langsung',
		Search: 'Mesin pencari',
		Social: 'Media sosial',
		Referral: 'Situs lain'
	};

	const bars = $derived.by(() => {
		const series = data?.stats.daily ?? [];
		const max = Math.max(1, ...series.map((s) => s.clicks));
		const w = 300 / Math.max(series.length, 1);
		return series.map((s, i) => {
			const h = s.clicks > 0 ? Math.max(3, (s.clicks / max) * 76) : 1;
			return { x: i * w + 1, y: 80 - h, w: Math.max(w - 2, 1), h, s };
		});
	});

	function split(map: Record<string, number>, labels: Record<string, string>) {
		const total = Object.values(map).reduce((a, b) => a + b, 0) || 1;
		return Object.entries(map)
			.sort((a, b) => b[1] - a[1])
			.map(([k, v]) => ({ key: k, label: labels[k] ? tr(labels[k]) : k, value: v, pct: Math.round((v / total) * 100) }));
	}
	const devices = $derived(data ? split(data.stats.devices, deviceLabel) : []);
	const referrers = $derived(data ? split(data.stats.referrers, referrerLabel) : []);
</script>

<PageTitle title={data?.campaign.name ?? tr('Analisis Kinerja')} />

<section class="ad">
	<a class="ad-back" href={`/klien/${brandUrl}/analytics`}><ArrowLeft size={15} />{tr('Analisis Kinerja')}</a>

	{#if error}
		<div class="app-card ad-error" role="alert">{error}</div>
	{/if}

	{#if loading}
		<div class="app-card ad-loading">{tr('Memuat rincian kampanye…')}</div>
	{:else if data}
		<header>
			<h1 class="page-title">{data.campaign.name}</h1>
			<p class="page-desc">{tr('Klik dari artikel ke situs Anda, 30 hari terakhir.')}</p>
		</header>

		<dl class="ad-tiles">
			<div class="app-card ad-tile"><dt>{tr('Klik')}</dt><dd>{fmt(data.stats.clicks)}</dd></div>
			<div class="app-card ad-tile"><dt>{tr('Pengunjung')}</dt><dd>{fmt(data.stats.visitors)}</dd></div>
			<div class="app-card ad-tile"><dt>{tr('Artikel terbit')}</dt><dd>{data.publications.published}</dd></div>
			<div class="app-card ad-tile">
				<dt>{tr('Tautan bermasalah')}</dt>
				<dd class:ad-bad={data.publications.problem > 0}>{data.publications.problem}</dd>
			</div>
		</dl>

		<section class="app-card ad-chart" aria-labelledby="ad-daily">
			<h2 id="ad-daily" class="ad-h2">{tr('Klik per hari')}</h2>
			{#if data.stats.clicks === 0}
				<p class="ad-muted">{tr('Belum ada klik dalam 30 hari terakhir. Klik hanya tercatat bila artikel memakai tautan pendek kampanye.')}</p>
			{:else}
				<svg class="ad-bars" viewBox="0 0 300 80" preserveAspectRatio="none" role="img" aria-label={tr('Klik per hari')}>
					{#each bars as b (b.s.day)}
						<rect x={b.x} y={b.y} width={b.w} height={b.h} rx="1"><title>{b.s.day}: {b.s.clicks}</title></rect>
					{/each}
				</svg>
				<p class="ad-muted">{data.stats.daily[0]?.day} {tr('sampai')} {data.stats.daily[data.stats.daily.length - 1]?.day}</p>
			{/if}
		</section>

		{#if data.stats.clicks > 0}
			<div class="ad-two">
				<section class="app-card ad-box" aria-labelledby="ad-dev">
					<h2 id="ad-dev" class="ad-h2">{tr('Perangkat')}</h2>
					<ul class="ad-split">
						{#each devices as d (d.key)}
							<li><span>{d.label}</span><span class="ad-track"><i style="width: {d.pct}%"></i></span><b>{d.pct}%</b></li>
						{/each}
					</ul>
				</section>
				<section class="app-card ad-box" aria-labelledby="ad-ref">
					<h2 id="ad-ref" class="ad-h2">{tr('Asal kunjungan')}</h2>
					<ul class="ad-split">
						{#each referrers as d (d.key)}
							<li><span>{d.label}</span><span class="ad-track"><i style="width: {d.pct}%"></i></span><b>{d.pct}%</b></li>
						{/each}
					</ul>
				</section>
			</div>
		{/if}

		<section class="app-card ad-box" aria-labelledby="ad-links">
			<h2 id="ad-links" class="ad-h2">{tr('Kondisi tautan di artikel')}</h2>
			{#if data.publications.published === 0}
				<p class="ad-muted">{tr('Belum ada artikel terbit di kampanye ini.')}</p>
			{:else}
				<p class="ad-muted">
					<strong>{data.publications.live}</strong> {tr('dari')} {data.publications.published} {tr('artikel terbit dan tautannya sehat')}.
					{tr('Terakhir diperiksa')} {fmtDate(data.publications.last_checked_at)}.
				</p>
				<a class="ad-link" href={`/klien/${brandUrl}/link-guardian`}>{tr('Buka Link Guardian')}</a>
			{/if}
		</section>

		<p class="ad-note">
			{data.stats.bots_filtered} {tr('klik dari bot dan pratinjau tautan tidak dihitung.')}
			{tr('Satu pengunjung dihitung sekali per hari. Klik per media dan per negara belum tersedia.')}
			{#if data.stats.truncated}{tr('Data sangat banyak, sebagian klik lama tidak ikut terhitung.')}{/if}
		</p>
	{/if}
</section>

<style>
	.ad { display: flex; flex-direction: column; gap: 20px; }
	.ad-back { display: inline-flex; align-items: center; gap: 8px; min-height: 44px; align-self: flex-start; font-size: 13px; font-weight: 500; color: var(--primary); text-decoration: none; }
	.ad-back:hover { text-decoration: underline; }
	.ad-error { padding: 12px 16px; font-size: 14px; border-color: var(--destructive); color: var(--destructive); }
	.ad-loading { padding: 20px; text-align: center; font-size: 14px; color: var(--muted-foreground); }
	.ad-muted { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.ad-note { font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.ad-h2 { font-size: 16px; line-height: 24px; font-weight: 600; color: var(--foreground); }
	.ad-tiles { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px; }
	.ad-tile { padding: 16px 20px; display: flex; flex-direction: column; gap: 4px; }
	.ad-tile dt { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.ad-tile dd { font-size: 28px; line-height: 36px; font-weight: 600; letter-spacing: -0.02em; color: var(--foreground); font-variant-numeric: tabular-nums; }
	.ad-bad { color: var(--destructive) !important; }
	.ad-chart, .ad-box { padding: 20px; display: flex; flex-direction: column; gap: 12px; }
	.ad-bars { width: 100%; height: 96px; }
	.ad-bars rect { fill: var(--primary); opacity: 0.85; }
	.ad-two { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
	.ad-split { padding: 0; list-style: none; display: flex; flex-direction: column; gap: 12px; }
	.ad-split li { display: grid; grid-template-columns: 112px minmax(0, 1fr) 44px; align-items: center; gap: 12px; font-size: 13px; color: var(--foreground); }
	.ad-split b { font-weight: 600; text-align: right; font-variant-numeric: tabular-nums; }
	.ad-track { height: 6px; border-radius: 999px; background: var(--muted); overflow: hidden; }
	.ad-track i { display: block; height: 100%; border-radius: 999px; background: var(--primary); }
	.ad-link { display: inline-flex; align-items: center; min-height: 44px; font-size: 13px; font-weight: 500; color: var(--primary); text-decoration: none; }
	.ad-link:hover { text-decoration: underline; }
	@media (max-width: 860px) {
		.ad-tiles { grid-template-columns: repeat(2, minmax(0, 1fr)); }
		.ad-two { grid-template-columns: 1fr; }
	}
</style>
