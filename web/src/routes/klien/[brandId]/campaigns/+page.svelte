<script lang="ts">
	// Daftar kampanye klien (read-only; wizard flow menyusul fase 7).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Megaphone } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);
	type Campaign = {
		id: string; name: string; description: string | null; status: string; order_code: string | null;
		article_quota: number; budget: number; publication_count: number; articles_created: number;
		published_count: number; media_published_count: number; tc_used: number; created_at: string;
	};

	let campaigns = $state<Campaign[]>([]);
	let total = $state(0);
	let status = $state('');
	let loading = $state(true);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		const params = new URLSearchParams({ limit: '100' });
		if (status) params.set('status', status);
		const res = await apiRequest(`/v1/brands/${brandId}/campaigns?${params}`);
		if (res.ok) {
			const data = await res.json();
			campaigns = data.campaigns ?? [];
			total = data.total ?? 0;
		}
	}

	const statusLabel: Record<string, string> = {
		draft: 'Draf', active: 'Aktif', paused: 'Dijeda', completed: 'Selesai', cancelled: 'Dibatalkan'
	};
	const tone: Record<string, string> = {
		active: 'success', completed: 'success', paused: 'warning', cancelled: 'muted', draft: 'muted'
	};
	const toneStyle = (t: string) =>
		t === 'success' ? 'background-color: var(--success-soft); color: var(--success);'
		: t === 'warning' ? 'background-color: var(--warning-soft); color: var(--warning);'
		: 'background-color: var(--muted); color: var(--muted-foreground);';
	const fmt = (n: number) => n.toLocaleString('id-ID');
</script>

<section class="mx-auto max-w-6xl">
	<header class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Kampanye</h1>
			<p class="mt-1 text-sm text-muted-foreground">{fmt(total)} kampanye reguler terdaftar.</p>
		</div>
		<select class="app-input !w-auto" bind:value={status} onchange={() => load()}>
			<option value="">Semua status</option>
			<option value="draft">Draf</option>
			<option value="active">Aktif</option>
			<option value="paused">Dijeda</option>
			<option value="completed">Selesai</option>
			<option value="cancelled">Dibatalkan</option>
		</select>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat kampanye…</p>
	{:else if campaigns.length === 0}
		<div class="app-card mt-6 flex flex-col items-center px-6 py-12 text-center">
			<Megaphone size={22} style="color: var(--primary);" />
			<p class="mt-2 text-sm font-medium">{status ? 'Tidak ada kampanye dengan status ini.' : 'Belum ada kampanye.'}</p>
			<p class="mt-1 text-sm text-muted-foreground">
				Kampanye menggabungkan beberapa artikel ke satu rencana distribusi. Sebelum kampanye pertama dibuat,
				artikel bisa digenerate satu per satu lewat halaman Generate.
			</p>
			<a class="app-btn app-btn-primary mt-3" href={`/klien/${brandUrl}/generate`}>Buka halaman Generate</a>
			{#if status}
				<button class="app-btn app-btn-soft mt-3" onclick={() => { status = ''; void load(); }}>Reset filter</button>
			{/if}
		</div>
	{:else}
		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="border-b text-left text-[11px] tracking-wide text-muted-foreground">
						<th class="px-4 py-2.5 font-semibold">KAMPANYE</th>
						<th class="px-4 py-2.5 font-semibold">STATUS</th>
						<th class="px-4 py-2.5 text-right font-semibold">ARTIKEL</th>
						<th class="px-4 py-2.5 text-right font-semibold">PUBLIKASI</th>
						<th class="px-4 py-2.5 text-right font-semibold">MEDIA</th>
						<th class="px-4 py-2.5 text-right font-semibold">KREDIT</th>
					</tr>
				</thead>
				<tbody>
					{#each campaigns as c (c.id)}
						<tr class="border-b last:border-0">
							<td class="px-4 py-2.5">
								<p class="font-medium">{c.name}</p>
								{#if c.order_code}<p class="text-xs text-muted-foreground">{c.order_code}</p>{/if}
							</td>
							<td class="px-4 py-2.5">
								<span class="rounded-full px-2 py-0.5 text-xs font-semibold" style={toneStyle(tone[c.status] ?? 'muted')}>
									{statusLabel[c.status] ?? c.status}
								</span>
							</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{c.articles_created}/{c.article_quota}</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{c.published_count}/{c.publication_count}</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{c.media_published_count}</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{fmt(c.tc_used)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>
