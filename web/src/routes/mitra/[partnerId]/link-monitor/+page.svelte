<script lang="ts">
	// Pelacak 404 mitra — port section "Pelacak 404": ringkasan tautan diawasi,
	// daftar publikasi bermasalah (hilang/tidak terjangkau), dan kesehatan domain
	// media (verifikasi kepemilikan domain).
	import { onMount } from 'svelte';
	import { ExternalLink, Search, TriangleAlert } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type View = 'publications' | 'domains';
	let view = $state<View>('publications');
	let loading = $state(true);
	let summary = $state<any>(null);
	let publications = $state<any[]>([]);
	let domains = $state<any[]>([]);
	let total = $state(0);
	let q = $state('');
	let linkState = $state('');
	let health = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([loadSummary(), loadPublications(), loadDomains()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadSummary() {
		const res = await apiRequest('/v1/partner/link-monitor?view=summary');
		if (res.ok) summary = await res.json();
	}

	async function loadPublications() {
		const params = new URLSearchParams({ view: 'publications', limit: '200' });
		if (q.trim()) params.set('q', q.trim());
		if (linkState) params.set('state', linkState);
		const res = await apiRequest(`/v1/partner/link-monitor?${params}`);
		if (res.ok) {
			const data = await res.json();
			publications = data.publications ?? [];
			total = data.total ?? 0;
		}
	}

	async function loadDomains() {
		const params = new URLSearchParams({ view: 'domains', limit: '200' });
		if (health) params.set('health', health);
		const res = await apiRequest(`/v1/partner/link-monitor?${params}`);
		if (res.ok) domains = (await res.json()).domains ?? [];
	}

	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '—');
	const stateLabel: Record<string, string> = { gone: 'Hilang (404)', unreachable: 'Tidak terjangkau', ok: 'Normal', unknown: 'Belum diperiksa' };
	const healthLabel: Record<string, string> = { live: 'Aktif', unreachable: 'Tidak terjangkau', unknown: 'Belum diperiksa', token_missing: 'Token hilang' };
</script>

<PageTitle title="Pelacak 404" />

<section>
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Pelacak 404</h1>
		<p class="mt-1 text-sm text-muted-foreground">Pantau tautan artikel yang hilang dan kesehatan domain media Anda.</p>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat data pelacak…</p>
	{:else}
		<div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
			<div class="app-card p-4">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground">DIAWASI</p>
				<p class="mt-2 text-2xl font-bold tabular-nums">{summary?.watched ?? 0}</p>
			</div>
			<div class="app-card p-4">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground">HILANG (404)</p>
				<p class="mt-2 text-2xl font-bold tabular-nums" style="color: var(--destructive);">{summary?.gone ?? 0}</p>
			</div>
			<div class="app-card p-4">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground">TIDAK TERJANGKAU</p>
				<p class="mt-2 text-2xl font-bold tabular-nums" style="color: var(--warning, #a15c00);">{summary?.unreachable ?? 0}</p>
			</div>
			<div class="app-card p-4">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground">DOMAIN DOWN</p>
				<p class="mt-2 text-2xl font-bold tabular-nums" style="color: var(--destructive);">{summary?.domains_down ?? 0}</p>
			</div>
			<div class="app-card p-4">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground">PERLU PERHATIAN</p>
				<p class="mt-2 text-2xl font-bold tabular-nums" style="color: var(--warning, #a15c00);">{summary?.domains_attention ?? 0}</p>
			</div>
		</div>

		<div class="mt-4 flex gap-2">
			<button class="app-btn !min-h-8 !px-3 !text-xs" class:app-btn-primary={view === 'publications'} class:app-btn-soft={view !== 'publications'} onclick={() => (view = 'publications')}>Tautan Artikel</button>
			<button class="app-btn !min-h-8 !px-3 !text-xs" class:app-btn-primary={view === 'domains'} class:app-btn-soft={view !== 'domains'} onclick={() => (view = 'domains')}>Kesehatan Domain</button>
		</div>

		{#if view === 'publications'}
			<form class="app-card mt-3 flex flex-wrap items-center gap-2 p-3" onsubmit={(e) => { e.preventDefault(); void loadPublications(); }}>
				<label class="app-input flex min-w-[200px] flex-1 items-center gap-2 !py-1.5">
					<Search size={15} class="text-muted-foreground" />
					<input class="w-full bg-transparent text-sm outline-none" placeholder="Cari judul, media, atau URL…" bind:value={q} />
				</label>
				<Select
					bind:value={linkState}
					ariaLabel="Status tautan"
					placeholder="Bermasalah (404 / tidak terjangkau)"
					align="end"
					onchange={loadPublications}
					options={[
						{ value: '', label: 'Bermasalah (404 / tidak terjangkau)' },
						{ value: 'all', label: 'Semua status' },
						{ value: 'gone', label: 'Hilang (404)' },
						{ value: 'unreachable', label: 'Tidak terjangkau' },
						{ value: 'ok', label: 'Normal' }
					]}
				/>
			</form>

			<div class="app-card mt-3 overflow-x-auto">
				{#if publications.length === 0}
					<div class="px-4 py-10 text-center">
						<TriangleAlert size={26} class="mx-auto text-muted-foreground" />
						<p class="mt-3 text-sm text-muted-foreground">Tidak ada tautan bermasalah. Semua artikel sehat.</p>
					</div>
				{:else}
					<table class="w-full text-sm">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="px-4 py-2.5 font-medium">Artikel</th>
								<th class="px-4 py-2.5 font-medium">Media</th>
								<th class="px-4 py-2.5 font-medium">URL</th>
								<th class="px-4 py-2.5 font-medium">Status</th>
								<th class="px-4 py-2.5 text-right font-medium">Gagal</th>
								<th class="px-4 py-2.5 font-medium">Diperiksa</th>
							</tr>
						</thead>
						<tbody>
							{#each publications as p (p.id)}
								<tr class="border-t">
									<td class="max-w-[240px] truncate px-4 py-2.5">{p.document_title ?? '—'}</td>
									<td class="px-4 py-2.5 text-muted-foreground">{p.media_name ?? '—'}</td>
									<td class="max-w-[240px] px-4 py-2.5">
										{#if p.published_url}
											<a class="inline-flex items-center gap-1 truncate text-xs" style="color: var(--primary);" href={p.published_url} target="_blank" rel="noreferrer">
												{p.published_url.replace(/^https?:\/\//, '').slice(0, 42)} <ExternalLink size={11} />
											</a>
										{:else}<span class="text-xs text-muted-foreground">—</span>{/if}
									</td>
									<td class="px-4 py-2.5">
										<span
											class="rounded-full px-2 py-0.5 text-[11px] font-medium"
											style="background-color: {p.state === 'ok' ? 'var(--accent)' : 'var(--destructive-soft, #fdecee)'}; color: {p.state === 'ok' ? 'var(--accent-foreground)' : 'var(--destructive)'};"
										>{stateLabel[p.state ?? 'unknown'] ?? p.state}</span>
									</td>
									<td class="px-4 py-2.5 text-right tabular-nums">{p.consecutive_failures ?? 0}</td>
									<td class="px-4 py-2.5 text-xs text-muted-foreground">{dt(p.last_checked_at)}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>
		{:else}
			<div class="app-card mt-3 flex flex-wrap items-center gap-2 p-3">
				<Select
					bind:value={health}
					ariaLabel="Kesehatan domain"
					placeholder="Perlu perhatian (belum diperiksa / token hilang / down)"
					onchange={loadDomains}
					options={[
						{ value: '', label: 'Perlu perhatian (belum diperiksa / token hilang / down)' },
						{ value: 'live', label: 'Aktif' },
						{ value: 'unreachable', label: 'Tidak terjangkau' },
						{ value: 'unknown', label: 'Belum diperiksa' },
						{ value: 'token_missing', label: 'Token hilang' }
					]}
				/>
			</div>

			<div class="app-card mt-3 overflow-x-auto">
				{#if domains.length === 0}
					<p class="px-4 py-10 text-center text-sm text-muted-foreground">Tidak ada domain yang perlu perhatian.</p>
				{:else}
					<table class="w-full text-sm">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="px-4 py-2.5 font-medium">Media</th>
								<th class="px-4 py-2.5 font-medium">Domain</th>
								<th class="px-4 py-2.5 font-medium">Kesehatan</th>
								<th class="px-4 py-2.5 font-medium">Verifikasi</th>
								<th class="px-4 py-2.5 text-right font-medium">Gagal</th>
								<th class="px-4 py-2.5 font-medium">Diperiksa</th>
							</tr>
						</thead>
						<tbody>
							{#each domains as d (d.media_id)}
								<tr class="border-t">
									<td class="px-4 py-2.5">{d.name}</td>
									<td class="px-4 py-2.5 text-muted-foreground">{d.website_url ?? '—'}</td>
									<td class="px-4 py-2.5">{healthLabel[d.domain_health ?? 'unknown'] ?? d.domain_health}</td>
									<td class="px-4 py-2.5">
										{#if d.domain_verified}
											<span style="color: var(--success);">Terverifikasi</span>
										{:else}
											<span style="color: var(--warning, #a15c00);">Belum</span>
										{/if}
									</td>
									<td class="px-4 py-2.5 text-right tabular-nums">{d.domain_check_fail_count ?? 0}</td>
									<td class="px-4 py-2.5 text-xs text-muted-foreground">{dt(d.domain_last_checked_at)}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>
		{/if}
	{/if}
</section>
