<script lang="ts">
	// Port klien section `distribution` (DistributionOverview di
	// app/klien/[brandId]/[[...section]]/distribution.tsx): ringkasan distribusi
	// 30 hari — metrik laporan, saldo TC (dengan polling 30 detik), publikasi
	// terbaru, dan kampanye terbaru.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Coins } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);

	type ReportSummary = {
		published: number;
		estimated_media_value: number;
		campaign_cost: number;
	};

	let summary = $state<ReportSummary | null>(null);
	let recent = $state<any[]>([]);
	let campaigns = $state<any[]>([]);
	let wallet = $state<{ available_tc: number; reserved_tc: number; rate_idr: number } | null>(null);
	let error = $state('');
	let loading = $state(true);

	onMount(() => {
		let pollTimer: number | null = null;
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([load(), loadCredits()]);
			loading = false;
			// Polling ringan (30 dtk): saldo TC tetap sinkron setelah top-up
			// atau publikasi tanpa reload halaman.
			pollTimer = window.setInterval(() => void loadCredits(), 30_000);
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			if (pollTimer) window.clearInterval(pollTimer);
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
		};
	});

	async function loadCredits() {
		const res = await apiRequest(`/v1/brands/${brandId}/credits?limit=1`);
		if (!res.ok) return;
		const data = await res.json();
		if (data?.wallet) {
			wallet = {
				available_tc: Number(data.wallet.available_tc ?? 0),
				reserved_tc: Number(data.wallet.reserved_tc ?? 0),
				rate_idr: Number(data.tc_config?.rate_idr ?? data.wallet.rate_idr ?? 1000)
			};
		}
	}

	async function load() {
		error = '';
		const now = new Date();
		const from = new Date(now.getTime() - 30 * 86_400_000).toISOString().slice(0, 10);
		const to = now.toISOString().slice(0, 10);
		const [reportsRes, pubsRes, campsRes] = await Promise.all([
			apiRequest(`/v1/brands/${brandId}/reports?from=${from}&to=${to}`),
			apiRequest(`/v1/brands/${brandId}/publications?limit=6`),
			apiRequest(`/v1/brands/${brandId}/campaigns?limit=6`)
		]);
		if (!reportsRes.ok || !pubsRes.ok || !campsRes.ok) {
			error = await readApiError(reportsRes, 'Gagal memuat ringkasan distribusi.');
			return;
		}
		summary = (await reportsRes.json()).summary ?? null;
		recent = (await pubsRes.json()).publications ?? [];
		campaigns = (await campsRes.json()).campaigns ?? [];
	}

	const fmtNum = (n: unknown) => Number(n ?? 0).toLocaleString('id-ID');
	const fmtDate = (v: unknown) =>
		v ? new Date(v as string).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '—';
	// tcLabel: nilai rupiah → "N TC" memakai rate_idr aktif (parity app lama).
	const tcLabel = (amountIdr: number, rate: number) =>
		`${fmtNum(Math.round(Number(amountIdr || 0) / Math.max(1, rate)))} TC`;
</script>

<section class="mx-auto max-w-6xl">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Overview Distribusi</h1>
		<p class="mt-1 text-sm text-muted-foreground">
			Ringkasan publikasi, performa, dan nilai media 30 hari terakhir.
		</p>
	</header>

	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}
	{#if loading}<p class="mt-6 text-sm text-muted-foreground">Memuat ringkasan distribusi…</p>{/if}

	{#if !loading && summary}
		<div class="app-card mt-4 p-4">
			<div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
				<div>
					<p class="text-xs text-muted-foreground">Artikel terbit</p>
					<p class="text-xl font-bold tabular-nums">{fmtNum(summary.published)}</p>
					<p class="text-[11px] text-muted-foreground">30 hari terakhir</p>
				</div>
				<div>
					<p class="text-xs text-muted-foreground">Nilai media</p>
					<p class="text-xl font-bold tabular-nums">{tcLabel(summary.estimated_media_value, wallet?.rate_idr ?? 1000)}</p>
					<p class="text-[11px] text-muted-foreground">Biaya kampanye {tcLabel(summary.campaign_cost, wallet?.rate_idr ?? 1000)}</p>
				</div>
				{#if wallet}
					<div>
						<p class="text-xs text-muted-foreground">Saldo TC tersedia</p>
						<p class="text-xl font-bold tabular-nums">{fmtNum(wallet.available_tc)} TC</p>
						<p class="text-[11px] text-muted-foreground">bisa dipakai untuk publikasi</p>
					</div>
					<div title="TC yang ditahan untuk publikasi yang belum final. Terbit → jadi terpakai; ditolak/batal → kembali ke saldo tersedia.">
						<p class="text-xs text-muted-foreground">Reserved TC ⓘ</p>
						<p class="text-xl font-bold tabular-nums">{fmtNum(wallet.reserved_tc)} TC</p>
						<p class="text-[11px] text-muted-foreground">ditahan publikasi belum final</p>
					</div>
				{/if}
			</div>
			{#if wallet}
				<div class="mt-4">
					<a class="app-btn app-btn-soft" href={`/klien/${brandUrl}/credits`}><Coins size={15} /> Kelola Kredit Saldo</a>
				</div>
			{/if}
		</div>

		<div class="mt-4 grid gap-3 lg:grid-cols-2">
			<div class="app-card p-4">
				<p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Recent</p>
				<h3 class="mt-0.5 text-sm font-semibold">Publikasi terbaru</h3>
				{#if recent.length === 0}
					<p class="mt-3 text-sm text-muted-foreground">Belum ada publikasi. Buat dari halaman Publications.</p>
				{:else}
					<ul class="mt-2 divide-y">
						{#each recent as row (row.id)}
							<li class="flex items-center justify-between gap-3 py-2">
								<div class="min-w-0">
									<p class="truncate text-sm font-medium">
										{row.document_title ?? 'Artikel'} → {row.media_name ?? 'Media'}
									</p>
									<p class="text-[11px] text-muted-foreground">
										{fmtDate(row.created_at)} · {tcLabel(Number(row.client_price) || 0, wallet?.rate_idr ?? 1000)}
									</p>
								</div>
								<span class="shrink-0 rounded-full px-2 py-0.5 text-[11px]" style="background: var(--accent); color: var(--accent-foreground);">
									{row.status}
								</span>
							</li>
						{/each}
					</ul>
				{/if}
			</div>

			<div class="app-card p-4">
				<p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Campaigns</p>
				<h3 class="mt-0.5 text-sm font-semibold">Kampanye terbaru</h3>
				{#if campaigns.length === 0}
					<p class="mt-3 text-sm text-muted-foreground">Belum ada kampanye.</p>
				{:else}
					<ul class="mt-2 divide-y">
						{#each campaigns as row (row.id)}
							<li class="flex items-center justify-between gap-3 py-2">
								<div class="min-w-0">
									<p class="truncate text-sm font-medium">{row.name}</p>
									<p class="text-[11px] text-muted-foreground">
										{Number(row.publication_count ?? 0)} publikasi · {fmtNum(row.tc_used)} TC terpakai
									</p>
								</div>
								<span class="shrink-0 rounded-full px-2 py-0.5 text-[11px]" style="background: var(--accent); color: var(--accent-foreground);">
									{row.status}
								</span>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		</div>
	{/if}
</section>
