<script lang="ts">
	// Katalog media klien — port section media (read-only, media aktif saja).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ExternalLink, Search, X } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);

	type Media = {
		id: string; name: string; category: string | null; website_url: string | null;
		media_tier: string | null; media_type: string | null; traffic_estimate: number;
		credit_cost: number; client_price: number; markup_percent: number;
		dewan_pers_verified: boolean; domain_verified: boolean; bahasa_artikel: string[];
		categories: string[]; max_links: number | null; label_sponsor: string | null;
		seo: { da: number | null; pa: number | null; spam_score: number | null; ai_trust_score: number | null };
		description: string | null; topik_terlarang: string[];
	};

	let items = $state<Media[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let q = $state('');
	let category = $state('');
	let tier = $state('');
	let limit = 24;
	let offset = $state(0);
	let categories = $state<Array<{ id: string; name: string }>>([]);
	let detail = $state<Media | null>(null);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			const cats = await apiRequest(`/v1/brands/${brandId}/media/categories`);
			if (cats.ok) categories = await cats.json();
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load(reset = true) {
		if (reset) offset = 0;
		const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
		if (q.trim()) params.set('q', q.trim());
		if (category) params.set('category', category);
		const res = await apiRequest(`/v1/brands/${brandId}/media?${params}`);
		if (res.ok) {
			const data = await res.json();
			items = data.items ?? [];
			total = data.total ?? 0;
		}
	}

	const tierLabel: Record<string, string> = { tier1: 'Tier 1', tier2: 'Tier 2', tier3: 'Tier 3' };
	const fmt = (n: number) => n.toLocaleString('id-ID');
</script>

<section class="mx-auto max-w-6xl">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Katalog Media</h1>
		<p class="mt-1 text-sm text-muted-foreground">{fmt(total)} media aktif siap dipilih untuk publikasi.</p>
	</header>

	<form class="app-card mt-4 flex flex-wrap items-center gap-2 p-3" onsubmit={(e) => { e.preventDefault(); void load(); }}>
		<label class="app-input flex min-w-[220px] flex-1 items-center gap-2 !py-1.5">
			<Search size={15} class="text-muted-foreground" />
			<input class="w-full bg-transparent text-sm outline-none" placeholder="Cari nama media atau domain…" bind:value={q} />
		</label>
		<select class="app-input !w-auto" bind:value={category} onchange={() => load()}>
			<option value="">Semua kategori</option>
			{#each categories as c (c.id)}<option value={c.name}>{c.name}</option>{/each}
		</select>
		<select class="app-input !w-auto" bind:value={tier} onchange={() => load()}>
			<option value="">Semua tier</option>
			<option value="tier1">Tier 1</option>
			<option value="tier2">Tier 2</option>
			<option value="tier3">Tier 3</option>
		</select>
		<button class="app-btn app-btn-primary" type="submit">Cari</button>
	</form>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat katalog…</p>
	{:else if items.length === 0}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">
				{q || category || tier ? 'Tidak ada media yang cocok dengan filter ini.' : 'Belum ada media aktif di katalog.'}
			</p>
			{#if q || category || tier}
				<button class="app-btn app-btn-soft mt-3" onclick={() => { q = ''; category = ''; tier = ''; void load(); }}>Reset filter</button>
			{/if}
		</div>
	{:else}
		<div class="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
			{#each items as m (m.id)}
				<button class="app-card p-4 text-left transition-colors hover:bg-secondary" onclick={() => (detail = m)}>
					<div class="flex items-start justify-between gap-2">
						<div class="min-w-0">
							<p class="truncate font-semibold">{m.name}</p>
							<p class="truncate text-xs text-muted-foreground">{m.website_url ?? m.category ?? '—'}</p>
						</div>
						{#if m.media_tier}
							<span class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold" style="background-color: var(--accent); color: var(--accent-foreground);">{tierLabel[m.media_tier] ?? m.media_tier}</span>
						{/if}
					</div>
					<div class="mt-3 flex flex-wrap items-center gap-2 text-[11px]">
						{#if m.dewan_pers_verified}
							<span class="rounded-full px-2 py-0.5 font-medium" style="background-color: var(--success-soft); color: var(--success);">Dewan Pers</span>
						{/if}
						{#if m.domain_verified}
							<span class="rounded-full px-2 py-0.5 font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">Domain terverifikasi</span>
						{/if}
						{#if m.seo?.da != null}
							<span class="text-muted-foreground">DA {m.seo.da} · PA {m.seo.pa ?? '-'}</span>
						{/if}
					</div>
					<div class="mt-3 flex items-end justify-between border-t pt-2.5">
						<div>
							<p class="text-[10px] font-semibold tracking-wide text-muted-foreground">HARGA KLIEN</p>
							<p class="text-base font-bold tabular-nums">Rp {fmt(m.client_price)}</p>
						</div>
						<p class="text-xs text-muted-foreground">{m.credit_cost} kredit</p>
					</div>
				</button>
			{/each}
		</div>

		{#if total > limit}
			<div class="mt-4 flex items-center justify-between text-sm">
				<p class="text-muted-foreground">Menampilkan {offset + 1}–{Math.min(offset + limit, total)} dari {fmt(total)}</p>
				<div class="flex gap-2">
					<button class="app-btn app-btn-soft" disabled={offset === 0} onclick={() => { offset = Math.max(0, offset - limit); void load(false); }}>Sebelumnya</button>
					<button class="app-btn app-btn-soft" disabled={offset + limit >= total} onclick={() => { offset += limit; void load(false); }}>Berikutnya</button>
				</div>
			</div>
		{/if}
	{/if}
</section>

{#if detail}
	<!-- Modal: <dialog> bawaan browser (fokus terkelola, Escape menutup) -->
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4"
		style="background-color: rgba(0,0,0,.45);"
		role="presentation"
	>
		<dialog
			open
			class="app-card max-h-[85vh] w-full max-w-lg overflow-y-auto p-5"
			aria-label={`Detail media ${detail.name}`}
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => { if (e.key === 'Escape') detail = null; }}
		>
			<div class="flex items-start justify-between gap-3">
				<div>
					<h2 class="text-lg font-semibold">{detail.name}</h2>
					{#if detail.website_url}
						<a class="mt-0.5 inline-flex items-center gap-1 text-xs" style="color: var(--primary);" href={detail.website_url} target="_blank" rel="noreferrer">
							{detail.website_url} <ExternalLink size={12} />
						</a>
					{/if}
				</div>
				<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label="Tutup detail media" onclick={() => (detail = null)}><X size={18} /></button>
			</div>

			{#if detail.description}<p class="mt-3 text-sm text-muted-foreground">{detail.description}</p>{/if}

			<dl class="mt-4 grid grid-cols-2 gap-3 text-sm">
				<div><dt class="text-xs text-muted-foreground">Harga klien</dt><dd class="font-semibold tabular-nums">Rp {fmt(detail.client_price)}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Biaya kredit</dt><dd class="font-semibold tabular-nums">{detail.credit_cost} TC</dd></div>
				<div><dt class="text-xs text-muted-foreground">Tier</dt><dd>{tierLabel[detail.media_tier ?? ''] ?? '—'}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Jangkauan</dt><dd class="tabular-nums">{fmt(detail.traffic_estimate)}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Maks. link</dt><dd>{detail.max_links ?? '—'}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Label sponsor</dt><dd>{detail.label_sponsor ?? '—'}</dd></div>
				{#if detail.seo?.da != null}
					<div><dt class="text-xs text-muted-foreground">DA / PA</dt><dd>{detail.seo.da} / {detail.seo.pa ?? '—'}</dd></div>
					<div><dt class="text-xs text-muted-foreground">Spam score</dt><dd>{detail.seo.spam_score ?? '—'}</dd></div>
				{/if}
			</dl>

			{#if detail.bahasa_artikel.length > 0}
				<p class="mt-4 text-xs text-muted-foreground">Bahasa artikel: {detail.bahasa_artikel.join(', ')}</p>
			{/if}
			{#if detail.topik_terlarang.length > 0}
				<p class="mt-2 text-xs" style="color: var(--destructive);">Topik terlarang: {detail.topik_terlarang.join(', ')}</p>
			{/if}
		</dialog>
	</div>
{/if}
