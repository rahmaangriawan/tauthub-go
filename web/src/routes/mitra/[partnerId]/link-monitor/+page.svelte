<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

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
	let mediaFilter = $state('');
	let daysFilter = $state('');
	let pageNo = $state(1);
	let error = $state('');
	const PAGE_SIZE = 20;

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
		// Default server-side = "problematic" (gone + unreachable), sama seperti app lama.
		const params = new URLSearchParams({
			view: 'publications',
			limit: String(PAGE_SIZE),
			offset: String((pageNo - 1) * PAGE_SIZE)
		});
		if (q.trim()) params.set('q', q.trim());
		params.set('state', linkState || 'problematic');
		if (mediaFilter) params.set('media', mediaFilter);
		if (daysFilter) params.set('days', daysFilter);
		const res = await apiRequest(`/v1/partner/link-monitor?${params}`);
		if (!res.ok) {
			error = tr('Gagal memuat daftar tautan. Coba lagi.');
			return;
		}
		error = '';
		const data = await res.json();
		publications = data.publications ?? [];
		total = data.total ?? 0;
	}

	async function loadDomains() {
		const params = new URLSearchParams({ view: 'domains', limit: '200' });
		if (health) params.set('health', health);
		const res = await apiRequest(`/v1/partner/link-monitor?${params}`);
		if (res.ok) domains = (await res.json()).domains ?? [];
	}

	function resetFilters() {
		q = '';
		linkState = '';
		mediaFilter = '';
		daysFilter = '';
		pageNo = 1;
		void loadPublications();
	}

	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '—');
	/** "3 hari lalu" — pendamping tanggal absolut (parity app lama). */
	const timeAgo = (v: string | null) => {
		if (!v) return '';
		const diffDays = Math.floor((Date.now() - new Date(v).getTime()) / 86400000);
		if (diffDays <= 0) return tr('hari ini');
		if (diffDays === 1) return tr('1 hari lalu');
		return `${diffDays} ${tr('hari lalu')}`;
	};
	const stateLabel: Record<string, string> = $derived({
		gone: 'Hilang (404)',
		unreachable: tr('Tidak terjangkau'),
		live: tr('Tayang'),
		no_url: tr('URL belum ada'),
		ok: tr('Tayang'),
		unknown: tr('Belum diperiksa')
	});
	const healthy = (s: string | null) => s === 'live' || s === 'ok';
	const healthLabel: Record<string, string> = $derived({ live: tr('Aktif'), unreachable: 'Tidak terjangkau', unknown: tr('Belum diperiksa'), token_missing: 'Token hilang' });
	const mediaNames = $derived([...new Set(domains.map((d) => d.name).filter(Boolean))] as string[]);
</script>

<PageTitle title={tr('Link Guardian')} />

<section>
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">{tr('Link Guardian')}</h1>
		<p class="mt-1 text-sm text-muted-foreground">{tr("Pantau tautan artikel yang hilang dan kesehatan domain media Anda.")}</p>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat data pelacak…")}</p>
	{:else}
		<div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
			<div class="app-card p-4">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground">{tr('DIAWASI')}</p>
				<p class="mt-2 text-2xl font-bold tabular-nums">{summary?.watched ?? 0}</p>
			</div>
			<div class="app-card p-4">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground">{tr('HILANG (404)')}</p>
				<p class="mt-2 text-2xl font-bold tabular-nums" style="color: var(--destructive);">{summary?.gone ?? 0}</p>
			</div>
			<div class="app-card p-4">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground">{tr('TIDAK TERJANGKAU')}</p>
				<p class="mt-2 text-2xl font-bold tabular-nums" style="color: var(--warning, #a15c00);">{summary?.unreachable ?? 0}</p>
			</div>
			<div class="app-card p-4">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground">DOMAIN DOWN</p>
				<p class="mt-2 text-2xl font-bold tabular-nums" style="color: var(--destructive);">{summary?.domains_down ?? 0}</p>
			</div>
			<div class="app-card p-4">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground">{tr('PERLU PERHATIAN')}</p>
				<p class="mt-2 text-2xl font-bold tabular-nums" style="color: var(--warning, #a15c00);">{summary?.domains_attention ?? 0}</p>
			</div>
		</div>

		<div class="mt-4 flex gap-2">
			<button class="app-btn !min-h-8 !px-3 !text-xs" class:app-btn-primary={view === 'publications'} class:app-btn-soft={view !== 'publications'} onclick={() => (view = 'publications')}>{tr('Tautan Artikel')}</button>
			<button class="app-btn !min-h-8 !px-3 !text-xs" class:app-btn-primary={view === 'domains'} class:app-btn-soft={view !== 'domains'} onclick={() => (view = 'domains')}>{tr('Kesehatan Domain')}</button>
		</div>

		{#if view === 'publications'}
			<form class="app-card mt-3 flex flex-wrap items-center gap-2 p-3" onsubmit={(e) => { e.preventDefault(); pageNo = 1; void loadPublications(); }}>
				<label class="app-input flex min-w-[200px] flex-1 items-center gap-2 !py-1.5">
					<Search size={15} class="text-muted-foreground" />
					<input class="w-full bg-transparent text-sm outline-none" placeholder={tr("Cari judul, media, atau URL…")} bind:value={q} />
				</label>
				<Select
					bind:value={linkState}
					ariaLabel={tr('Status tautan')}
					placeholder={tr('Bermasalah (404 / tidak terjangkau)')}
					align="end"
					onchange={() => { pageNo = 1; void loadPublications(); }}
					options={[
						{ value: '', label: tr('Bermasalah (404 / tidak terjangkau)') },
						{ value: 'all', label: tr('Semua status') },
						{ value: 'gone', label: tr('Hilang (404)') },
						{ value: 'unreachable', label: tr('Tidak terjangkau') },
						{ value: 'live', label: tr('Tayang') },
						{ value: 'no_url', label: tr('URL belum ada') }
					]}
				/>
				<Select
					bind:value={mediaFilter}
					ariaLabel={tr("Filter media")}
					placeholder={tr("Semua media")}
					align="end"
					onchange={() => { pageNo = 1; void loadPublications(); }}
					options={[{ value: '', label: tr('Semua media') }, ...mediaNames.map((n) => ({ value: n, label: n }))]}
				/>
				<Select
					bind:value={daysFilter}
					ariaLabel={tr("Rentang waktu")}
					placeholder={tr("Semua waktu")}
					align="end"
					onchange={() => { pageNo = 1; void loadPublications(); }}
					options={[
						{ value: '', label: tr('Semua waktu') },
						{ value: '7', label: tr('7 hari terakhir') },
						{ value: '30', label: tr('30 hari terakhir') },
						{ value: '90', label: tr('90 hari terakhir') }
					]}
				/>
				<button class="app-btn app-btn-soft" type="button" onclick={resetFilters}>{tr('Reset filter')}</button>
			</form>

			{#if error}
				<div class="app-card mt-3 flex flex-wrap items-center justify-between gap-2 px-4 py-3 text-sm" style="color: var(--destructive);">
					{error}
					<button class="app-btn app-btn-soft" type="button" onclick={() => void loadPublications()}>{tr('Coba lagi')}</button>
				</div>
			{/if}

			<div class="app-card mt-3 overflow-x-auto">
				{#if publications.length === 0}
					<div class="px-4 py-10 text-center">
						<TriangleAlert size={26} class="mx-auto text-muted-foreground" />
						<p class="mt-3 text-sm text-muted-foreground">{tr("Tidak ada tautan bermasalah. Semua artikel sehat.")}</p>
					</div>
				{:else}
					<table class="w-full text-sm">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="px-4 py-2.5 font-medium">{tr('Artikel')}</th>
								<th class="px-4 py-2.5 font-medium">{tr("Media")}</th>
								<th class="px-4 py-2.5 font-medium">URL</th>
								<th class="px-4 py-2.5 font-medium">Status</th>
								<th class="px-4 py-2.5 font-medium">{tr('Diperiksa')}</th>
							</tr>
						</thead>
						<tbody>
							{#each publications as p (p.id)}
								<tr class="border-t">
									<td class="px-4 py-2.5">
										<div class="flex items-center gap-2.5">
											{#if p.featured_image_url || p.draft_featured_image_url}
												<img
													src={p.featured_image_url ?? p.draft_featured_image_url}
													alt=""
													class="h-10 w-10 flex-none rounded-lg border object-cover"
													style="border-color: var(--border); background-color: var(--muted);"
													loading="lazy"
												/>
											{/if}
											<span class="max-w-[220px] truncate">{p.document_title ?? '—'}</span>
										</div>
									</td>
									<td class="px-4 py-2.5 text-muted-foreground">
										<p>{p.media_name ?? '—'}</p>
										<p class="text-xs">{p.media_type ?? ''}{p.media_category ? ` · ${p.media_category}` : ''}</p>
									</td>
									<td class="max-w-[220px] px-4 py-2.5">
										{#if p.published_url}
											<a class="inline-flex items-center gap-1 truncate text-xs" style="color: var(--primary);" href={p.published_url} target="_blank" rel="noreferrer">
												{p.published_url.replace(/^https?:\/\//, '').slice(0, 42)} <ExternalLink size={11} />
											</a>
										{:else}<span class="text-xs text-muted-foreground">{tr('URL belum ada')}</span>{/if}
									</td>
									<td class="px-4 py-2.5">
										<span
											class="rounded-full px-2 py-0.5 text-[11px] font-medium"
											style="background-color: {healthy(p.state) ? 'var(--accent)' : 'var(--destructive-soft, #fdecee)'}; color: {healthy(p.state) ? 'var(--accent-foreground)' : 'var(--destructive)'};"
											title={p.http_status ? `HTTP ${p.http_status} · ${p.consecutive_failures ?? 0} ${tr('gagal berturut-turut')}` : `${p.consecutive_failures ?? 0} ${tr('gagal berturut-turut')}`}
										>{stateLabel[p.state ?? 'unknown'] ?? p.state}</span>
									</td>
									<td class="px-4 py-2.5 text-xs text-muted-foreground">
										{dt(p.last_checked_at)}
										{#if p.last_checked_at}<span class="block">{timeAgo(p.last_checked_at)}</span>{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>
			<div class="mt-3 flex flex-wrap items-center justify-between gap-2">
				<span class="text-xs text-muted-foreground">
					{tr('Menampilkan')} {publications.length} {tr('dari')} {total} {tr('tautan')}
				</span>
				<div class="flex gap-2">
					<button class="app-btn app-btn-soft" type="button" disabled={pageNo <= 1} onclick={() => { pageNo = Math.max(1, pageNo - 1); }}>{tr('Sebelumnya')}</button>
					<button class="app-btn app-btn-soft" type="button" disabled={pageNo * PAGE_SIZE >= total} onclick={() => { pageNo = pageNo + 1; }}>{tr('Berikutnya')}</button>
				</div>
			</div>
		{:else}
			<div class="app-card mt-3 flex flex-wrap items-center gap-2 p-3">
				<Select
					bind:value={health}
					ariaLabel={tr('Kesehatan domain')}
					placeholder={tr('Perlu perhatian (belum diperiksa / token hilang / down)')}
					onchange={loadDomains}
					options={[
						{ value: '', label: tr('Perlu perhatian (belum diperiksa / token hilang / down)') },
						{ value: 'live', label: tr('Aktif') },
						{ value: 'unreachable', label: tr('Tidak terjangkau') },
						{ value: 'unknown', label: tr('Belum diperiksa') },
						{ value: 'token_missing', label: tr('Token hilang') }
					]}
				/>
			</div>

			<div class="app-card mt-3 overflow-x-auto">
				{#if domains.length === 0}
					<p class="px-4 py-10 text-center text-sm text-muted-foreground">{tr("Tidak ada domain yang perlu perhatian.")}</p>
				{:else}
					<table class="w-full text-sm">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="px-4 py-2.5 font-medium">{tr("Media")}</th>
								<th class="px-4 py-2.5 font-medium">Domain</th>
								<th class="px-4 py-2.5 font-medium">{tr('Kesehatan')}</th>
								<th class="px-4 py-2.5 font-medium">{tr('Verifikasi')}</th>
								<th class="px-4 py-2.5 text-right font-medium">{tr("Gagal")}</th>
								<th class="px-4 py-2.5 font-medium">{tr('Diperiksa')}</th>
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
											<span style="color: var(--success);">{tr('Terverifikasi')}</span>
										{:else}
											<span style="color: var(--warning, #a15c00);">{tr("Belum")}</span>
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
