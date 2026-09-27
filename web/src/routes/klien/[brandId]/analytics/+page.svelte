<script lang="ts">
	// Analisis Kinerja — event harian dari analytics_daily (port section analytics).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { BarChart3 } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);
	type Analytics = {
		from: string; to: string;
		totals: Record<string, number>;
		unique_visitors: number;
		daily: Record<string, Record<string, number>>;
	};

	let data = $state<Analytics | null>(null);
	let loading = $state(true);
	let from = $state('');
	let to = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			const today = new Date();
			const past = new Date(today.getTime() - 30 * 86400000);
			from = past.toISOString().slice(0, 10);
			to = today.toISOString().slice(0, 10);
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		const res = await apiRequest(`/v1/brands/${brandId}/analytics?from=${from}&to=${to}`);
		if (res.ok) data = await res.json();
	}

	const eventLabel: Record<string, string> = {
		article_view: 'Dilihat', article_scroll: 'Di-scroll', outbound_click: 'Klik keluar',
		cta_click: 'Klik CTA', tracking_link_click: 'Klik link pelacak'
	};
	const days = $derived(Object.keys(data?.daily ?? {}).sort().reverse());
	const fmt = (n: number) => n.toLocaleString('id-ID');
</script>

<section class="mx-auto max-w-5xl">
	<header class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Analisis Kinerja</h1>
			<p class="mt-1 text-sm text-muted-foreground">Event keterbacaan artikel per hari dari pelacakan publikasi.</p>
		</div>
		<form class="flex items-end gap-2" onsubmit={(e) => { e.preventDefault(); void load(); }}>
			<label class="flex flex-col gap-1 text-xs">Dari<input class="app-input !py-1.5" type="date" bind:value={from} /></label>
			<label class="flex flex-col gap-1 text-xs">Sampai<input class="app-input !py-1.5" type="date" bind:value={to} /></label>
			<button class="app-btn app-btn-primary" type="submit">Terapkan</button>
		</form>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat analitik…</p>
	{:else if !data || Object.keys(data.totals).length === 0}
		<div class="app-card mt-6 flex flex-col items-center px-6 py-12 text-center">
			<BarChart3 size={22} style="color: var(--primary);" />
			<p class="mt-2 text-sm font-medium">Belum ada data event pada rentang ini.</p>
			<p class="mt-1 text-sm text-muted-foreground">Data terisi setelah artikel terbit dan pembaca mengakses halaman.</p>
		</div>
	{:else}
		<div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
			{#each Object.entries(data.totals) as [key, value] (key)}
				<div class="app-card p-4">
					<p class="text-[11px] font-semibold tracking-wide text-muted-foreground">{eventLabel[key] ?? key}</p>
					<p class="mt-1.5 text-2xl font-bold tabular-nums">{fmt(value)}</p>
				</div>
			{/each}
		</div>

		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="border-b text-left text-[11px] tracking-wide text-muted-foreground">
						<th class="px-4 py-2.5 font-semibold">TANGGAL</th>
						{#each Object.keys(data.totals) as key (key)}<th class="px-4 py-2.5 text-right font-semibold">{key}</th>{/each}
					</tr>
				</thead>
				<tbody>
					{#each days as day (day)}
						<tr class="border-b last:border-0">
							<td class="px-4 py-2.5">{new Date(day).toLocaleDateString('id-ID')}</td>
							{#each Object.keys(data.totals) as key (key)}
								<td class="px-4 py-2.5 text-right tabular-nums">{fmt(data.daily[day]?.[key] ?? 0)}</td>
							{/each}
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>
