<script lang="ts">
	import { safeHref } from '$lib/safe-url';
	import { tr } from '$lib/i18n/phrases';

	// Pelacak 404 mitra — port section "Pelacak 404": ringkasan tautan diawasi,
	// daftar publikasi bermasalah (hilang/tidak terjangkau), dan kesehatan domain
	// media (verifikasi kepemilikan domain).
	import { onMount } from 'svelte';
	import { CircleCheck, ExternalLink, Search } from '@lucide/svelte';
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
		gone: tr('Hilang (404)'),
		unreachable: tr('Tidak terjangkau'),
		live: tr('Tayang'),
		no_url: tr('URL belum ada'),
		ok: tr('Tayang'),
		unknown: tr('Belum diperiksa')
	});
	const healthy = (s: string | null) => s === 'live' || s === 'ok';
	/** Nada pill kesehatan domain untuk kartu mobile (warna semantik design system). */
	const healthTone = (s: string | null) =>
		s === 'live'
			? { bg: 'var(--success-soft)', fg: 'var(--success)' }
			: s === 'unreachable' || s === 'token_missing'
				? { bg: 'var(--warning-soft, #fdf3e3)', fg: 'var(--warning, #a15c00)' }
				: { bg: 'var(--muted)', fg: 'var(--muted-foreground)' };
	const healthLabel: Record<string, string> = $derived({ live: tr('Aktif'), unreachable: tr('Tidak terjangkau'), unknown: tr('Belum diperiksa'), token_missing: tr('Token hilang') });
	const mediaNames = $derived([...new Set(domains.map((d) => d.name).filter(Boolean))] as string[]);
	// Angka ringkasan untuk panel: "tidak bermasalah" dihitung dari sisa diawasi
	// (diawasi − hilang − tidak terjangkau), bukan klaim tayang.
	const watched = $derived(summary?.watched ?? 0);
	const goneN = $derived(summary?.gone ?? 0);
	const unreachableN = $derived(summary?.unreachable ?? 0);
	const okCount = $derived(Math.max(0, watched - goneN - unreachableN));
	const domainsDown = $derived(summary?.domains_down ?? 0);
	const domainsAttention = $derived(summary?.domains_attention ?? 0);
	/** Nada status tautan untuk badge (hijau aman · kuning tak terjangkau · merah hilang). */
	const stateKind = (s: string | null) => (healthy(s) ? 'ok' : s === 'gone' ? 'bad' : s === 'unreachable' ? 'warn' : 'muted');
	const healthKind = (s: string | null) => (s === 'live' ? 'ok' : s === 'unreachable' ? 'bad' : s === 'token_missing' ? 'warn' : 'muted');
</script>

<PageTitle title={tr('Link Guardian')} />

<section>
	<header>
		<h1 class="page-title">{tr('Link Guardian')}</h1>
		<p class="page-desc">{tr("Pantau tautan artikel yang hilang dan kesehatan domain media Anda.")}</p>
	</header>

	{#if loading}
		<p class="lm-sub mt-6">{tr("Memuat data pelacak…")}</p>
	{:else}
		<!-- Ringkasan: satu panel. Kiri = tautan artikel (porsi aman / tidak terjangkau / hilang
		     dalam satu bilah), kanan = kesehatan domain. Warna hanya muncul bila angkanya > 0. -->
		<div class="lm-panel">
			<div class="lm-watch">
				<p class="lm-label">{tr('Diawasi')}</p>
				<p class="lm-big">{watched}</p>
				<div class="lm-bar" role="img" aria-label="{tr('Tidak bermasalah')} {okCount}, {tr('Tidak terjangkau')} {unreachableN}, {tr('Hilang (404)')} {goneN}">
					{#if watched > 0}
						<span class="lm-bar-ok" style="width: {(okCount / watched) * 100}%"></span>
						<span class="lm-bar-warn" style="width: {(unreachableN / watched) * 100}%"></span>
						<span class="lm-bar-bad" style="width: {(goneN / watched) * 100}%"></span>
					{/if}
				</div>
				<ul class="lm-legend">
					<li><i class="lm-dot lm-dot-ok"></i>{tr('Tidak bermasalah')} <b>{okCount}</b></li>
					<li><i class="lm-dot lm-dot-warn"></i>{tr('Tidak terjangkau')} <b>{unreachableN}</b></li>
					<li><i class="lm-dot lm-dot-bad"></i>{tr('Hilang (404)')} <b>{goneN}</b></li>
				</ul>
			</div>
			<div class="lm-doms">
				<div class="lm-dom" class:lm-dom-bad={domainsDown > 0}>
					<p class="lm-label">{tr('Domain down')}</p>
					<p class="lm-mid">{domainsDown}</p>
				</div>
				<div class="lm-dom" class:lm-dom-warn={domainsAttention > 0}>
					<p class="lm-label">{tr('Perlu perhatian')}</p>
					<p class="lm-mid">{domainsAttention}</p>
				</div>
			</div>
		</div>

		<div class="lm-tabs" role="tablist" aria-label={tr('Link Guardian')}>
			<button class="lm-tab" type="button" role="tab" aria-selected={view === 'publications'} onclick={() => (view = 'publications')}>{tr('Tautan Artikel')}</button>
			<button class="lm-tab" type="button" role="tab" aria-selected={view === 'domains'} onclick={() => (view = 'domains')}>{tr('Kesehatan Domain')}</button>
		</div>

		{#if view === 'publications'}
			<form class="app-card lm-filter" onsubmit={(e) => { e.preventDefault(); pageNo = 1; void loadPublications(); }}>
				<label class="app-input flex min-w-[200px] flex-1 items-center gap-2 !py-1.5">
					<Search size={16} strokeWidth={1.5} class="text-muted-foreground" />
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
				<div class="app-card lm-error" role="alert">
					{error}
					<button class="app-btn app-btn-soft" type="button" onclick={() => void loadPublications()}>{tr('Coba lagi')}</button>
				</div>
			{/if}

			{#if publications.length === 0}
				<div class="app-card lm-empty">
					<span class="lm-empty-icon"><CircleCheck size={22} strokeWidth={1.5} /></span>
					<p>{tr("Tidak ada tautan bermasalah. Semua artikel sehat.")}</p>
				</div>
			{:else}
				<!-- Desktop (≥768px) -->
				<div class="app-card lm-tablecard hidden overflow-x-auto md:block">
					<table class="lm-table">
						<thead>
							<tr>
								<th class="lm-th">{tr('Artikel')}</th>
								<th class="lm-th">{tr("Media")}</th>
								<th class="lm-th">URL</th>
								<th class="lm-th">Status</th>
								<th class="lm-th">{tr('Diperiksa')}</th>
							</tr>
						</thead>
						<tbody>
							{#each publications as p (p.id)}
								<tr class="lm-tr">
									<td class="lm-td">
										<div class="lm-article">
											{@render pubThumb(p)}
											<span class="lm-title" title={p.document_title ?? '—'}>{p.document_title ?? '—'}</span>
										</div>
									</td>
									<td class="lm-td">
										{@render pubMediaCell(p)}
									</td>
									<td class="lm-td lm-td-url">
										{@render pubUrlCell(p)}
									</td>
									<td class="lm-td">
										{@render pubStatusBadge(p)}
									</td>
									<td class="lm-td">
										{@render pubCheckedCell(p)}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				<!-- Mobile (<768px): setiap tautan menjadi kartu; data sama dengan tabel. -->
				<div class="lm-cards md:hidden">
					{#each publications as p (p.id)}
						<article class="app-card lm-card">
							<div class="lm-card-top">
								{@render pubThumb(p)}
								<div class="min-w-0 flex-1">
									<h3 class="lm-card-title" title={p.document_title ?? '—'}>{p.document_title ?? '—'}</h3>
									<div class="mt-2 flex flex-wrap">
										{@render pubStatusBadge(p)}
									</div>
								</div>
							</div>
							<dl class="lm-dl">
								<div class="min-w-0">
									<dt>{tr('Media')}</dt>
									<dd>{@render pubMediaCell(p)}</dd>
								</div>
								<div class="min-w-0">
									<dt>URL</dt>
									<dd>{@render pubUrlCell(p)}</dd>
								</div>
								<div class="min-w-0">
									<dt>{tr('Diperiksa')}</dt>
									<dd>{@render pubCheckedCell(p)}</dd>
								</div>
							</dl>
						</article>
					{/each}
				</div>
			{/if}
			<div class="lm-pager">
				<span class="lm-meta">
					{tr('Menampilkan')} {publications.length} {tr('dari')} {total} {tr('tautan')}
				</span>
				<div class="flex gap-2">
					<button class="app-btn app-btn-soft lm-pager-btn" type="button" disabled={pageNo <= 1} onclick={() => { pageNo = Math.max(1, pageNo - 1); void loadPublications(); }}>{tr('Sebelumnya')}</button>
					<button class="app-btn app-btn-soft lm-pager-btn" type="button" disabled={pageNo * PAGE_SIZE >= total} onclick={() => { pageNo = pageNo + 1; void loadPublications(); }}>{tr('Berikutnya')}</button>
				</div>
			</div>
		{:else}
			<div class="app-card lm-filter">
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

			{#if domains.length === 0}
				<div class="app-card lm-empty">
					<span class="lm-empty-icon"><CircleCheck size={22} strokeWidth={1.5} /></span>
					<p>{tr("Tidak ada domain yang perlu perhatian.")}</p>
				</div>
			{:else}
				<!-- Desktop (≥768px) -->
				<div class="app-card lm-tablecard hidden overflow-x-auto md:block">
					<table class="lm-table">
						<thead>
							<tr>
								<th class="lm-th">{tr("Media")}</th>
								<th class="lm-th">Domain</th>
								<th class="lm-th">{tr('Kesehatan')}</th>
								<th class="lm-th">{tr('Verifikasi')}</th>
								<th class="lm-th lm-th-end">{tr("Gagal")}</th>
								<th class="lm-th">{tr('Diperiksa')}</th>
							</tr>
						</thead>
						<tbody>
							{#each domains as d (d.media_id)}
								<tr class="lm-tr">
									<td class="lm-td"><span class="lm-title">{d.name}</span></td>
									<td class="lm-td"><span class="lm-host">{d.website_url ?? '—'}</span></td>
									<td class="lm-td"><span class="lm-pill lm-pill-{healthKind(d.domain_health)}">{healthLabel[d.domain_health ?? 'unknown'] ?? d.domain_health}</span></td>
									<td class="lm-td">
										{#if d.domain_verified}
											<span class="lm-pill lm-pill-ok">{tr('Terverifikasi')}</span>
										{:else}
											<span class="lm-pill lm-pill-warn">{tr("Belum")}</span>
										{/if}
									</td>
									<td class="lm-td lm-td-end"><span class="lm-fail" class:lm-fail-bad={(d.domain_check_fail_count ?? 0) > 0}>{d.domain_check_fail_count ?? 0}</span></td>
									<td class="lm-td"><span class="lm-date">{dt(d.domain_last_checked_at)}</span></td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				<!-- Mobile (<768px): kartu domain dengan data yang sama. -->
				<div class="lm-cards md:hidden">
					{#each domains as d (d.media_id)}
						<article class="app-card lm-card">
							<div class="flex flex-wrap items-center gap-x-2 gap-y-1">
								<h3 class="lm-card-title">{d.name}</h3>
								<span class="lm-pill lm-pill-{healthKind(d.domain_health)}">{healthLabel[d.domain_health ?? 'unknown'] ?? d.domain_health}</span>
							</div>
							<p class="lm-host mt-1 truncate">{d.website_url ?? '—'}</p>
							<div class="lm-card-foot">
								<span>{tr('Verifikasi')}: {#if d.domain_verified}<span class="lm-ok">{tr('Terverifikasi')}</span>{:else}<span class="lm-warn">{tr("Belum")}</span>{/if}</span>
								<span>{tr("Gagal")}: <span class="tabular-nums">{d.domain_check_fail_count ?? 0}</span></span>
								<span>{tr('Diperiksa')}: {dt(d.domain_last_checked_at)}</span>
							</div>
						</article>
					{/each}
				</div>
			{/if}
		{/if}
	{/if}
</section>

{#snippet pubThumb(p: any)}
	{#if p.featured_image_url || p.draft_featured_image_url}
		<img
			src={p.featured_image_url ?? p.draft_featured_image_url}
			alt=""
			class="lm-thumb"
			loading="lazy"
		/>
	{/if}
{/snippet}

{#snippet pubMediaCell(p: any)}
	<p class="lm-media">{p.media_name ?? '—'}</p>
	<p class="lm-media-sub">{p.media_type ?? ''}{p.media_category ? ` · ${p.media_category}` : ''}</p>
{/snippet}

{#snippet pubUrlCell(p: any)}
	{#if p.published_url}
		<a class="lm-url" href={safeHref(p.published_url)} target="_blank" rel="noreferrer">
			<span class="truncate">{p.published_url.replace(/^https?:\/\//, '').slice(0, 42)}</span> <ExternalLink size={12} strokeWidth={1.5} />
		</a>
	{:else}<span class="lm-date">{tr('URL belum ada')}</span>{/if}
{/snippet}

{#snippet pubStatusBadge(p: any)}
	<span
		class="lm-badge lm-badge-{stateKind(p.state)}"
		title={p.http_status ? `HTTP ${p.http_status} · ${p.consecutive_failures ?? 0} ${tr('gagal berturut-turut')}` : `${p.consecutive_failures ?? 0} ${tr('gagal berturut-turut')}`}
	><i class="lm-dot"></i>{stateLabel[p.state ?? 'unknown'] ?? p.state}</span>
{/snippet}

{#snippet pubCheckedCell(p: any)}
	<span class="lm-date">{dt(p.last_checked_at)}</span>
	{#if p.last_checked_at}<span class="lm-rel">{timeAgo(p.last_checked_at)}</span>{/if}
{/snippet}

<style>
	/* Link Guardian mitra: panel ringkasan + tabel, ritme 4px, bobot 400–500.
	   Warna hanya untuk arti: hijau aman · kuning tak terjangkau · merah hilang.
	   UI lama: +page.legacy.svelte.txt (salin ke +page.svelte untuk rollback). */
	.lm-sub { margin-top: 4px; font-size: 14px; line-height: 20px; color: var(--muted-foreground); }
	.lm-label { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.lm-meta { font-size: 12px; line-height: 16px; color: var(--muted-foreground); }

	.lm-panel { display: grid; grid-template-columns: 1fr; margin-top: 24px; border: 1px solid var(--border); border-radius: 16px; background-color: var(--card); }
	@media (min-width: 768px) { .lm-panel { grid-template-columns: minmax(0, 3fr) minmax(0, 2fr); } }
	.lm-watch { padding: 24px; }
	.lm-big { margin-top: 4px; font-size: 40px; line-height: 48px; font-weight: 500; font-variant-numeric: tabular-nums; }
	.lm-bar { display: flex; height: 8px; margin-top: 16px; overflow: hidden; border-radius: 9999px; background-color: var(--muted); }
	.lm-bar span { display: block; height: 100%; }
	.lm-bar-ok { background-color: var(--success); }
	.lm-bar-warn { background-color: #e0a526; }
	.lm-bar-bad { background-color: var(--destructive); }
	.lm-legend { display: flex; flex-wrap: wrap; gap: 4px 16px; margin-top: 12px; font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.lm-legend li { display: inline-flex; align-items: center; gap: 6px; }
	.lm-legend b { font-weight: 500; color: var(--foreground); font-variant-numeric: tabular-nums; }
	.lm-dot { display: inline-block; width: 8px; height: 8px; flex-shrink: 0; border-radius: 9999px; background-color: currentColor; }
	.lm-dot-ok { background-color: var(--success); }
	.lm-dot-warn { background-color: #e0a526; }
	.lm-dot-bad { background-color: var(--destructive); }

	.lm-doms { display: grid; grid-template-columns: 1fr 1fr; border-top: 1px solid var(--border); }
	@media (min-width: 768px) { .lm-doms { grid-template-columns: 1fr; grid-template-rows: 1fr 1fr; border-top: 0; border-left: 1px solid var(--border); } }
	.lm-dom { display: flex; flex-direction: column; justify-content: center; padding: 16px 24px; }
	.lm-dom + .lm-dom { border-left: 1px solid var(--border); }
	@media (min-width: 768px) { .lm-dom + .lm-dom { border-left: 0; border-top: 1px solid var(--border); } }
	.lm-mid { font-size: 28px; line-height: 36px; font-weight: 500; font-variant-numeric: tabular-nums; }
	.lm-dom-bad .lm-mid { color: var(--destructive); }
	.lm-dom-warn .lm-mid { color: var(--warning); }

	.lm-tabs { display: flex; gap: 24px; margin-top: 24px; border-bottom: 1px solid var(--border); }
	.lm-tab {
		position: relative; margin-bottom: -1px; padding: 12px 2px; border: 0; background: transparent; font-size: 14px; font-weight: 500;
		color: var(--muted-foreground); transition: color 120ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.lm-tab::after { content: ''; position: absolute; left: 0; right: 0; bottom: 0; height: 2px; border-radius: 2px 2px 0 0; background: var(--primary); transform: scaleX(0); transition: transform 160ms cubic-bezier(0.22, 1, 0.36, 1); }
	.lm-tab:hover { color: var(--foreground); }
	.lm-tab[aria-selected='true'] { color: var(--accent-foreground); }
	.lm-tab[aria-selected='true']::after { transform: scaleX(1); }
	.lm-tab:focus-visible { outline: 2px solid var(--primary); outline-offset: 2px; border-radius: 4px; }

	.lm-filter { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; margin-top: 16px; padding: 12px 16px; }
	.lm-error { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; margin-top: 12px; padding: 12px 16px; font-size: 14px; color: var(--destructive); }
	.lm-empty { display: flex; flex-direction: column; align-items: center; gap: 12px; margin-top: 16px; padding: 40px 24px; text-align: center; font-size: 14px; color: var(--muted-foreground); }
	.lm-empty-icon { display: inline-flex; width: 44px; height: 44px; align-items: center; justify-content: center; border-radius: 12px; background-color: var(--success-soft); color: var(--success); }

	.lm-tablecard { margin-top: 16px; }
	.lm-table { width: 100%; border-collapse: collapse; font-size: 14px; }
	.lm-th { padding: 12px 16px; font-size: 12px; line-height: 16px; font-weight: 400; text-align: left; white-space: nowrap; color: var(--muted-foreground); background-color: color-mix(in srgb, var(--foreground) 2%, transparent); border-bottom: 1px solid var(--border); }
	.lm-th-end { text-align: right; }
	.lm-tr { border-top: 1px solid var(--border); transition: background-color 120ms cubic-bezier(0.22, 1, 0.36, 1); }
	.lm-tr:hover { background-color: color-mix(in srgb, var(--foreground) 3%, transparent); }
	.lm-td { padding: 12px 16px; vertical-align: middle; }
	.lm-td-end { text-align: right; font-variant-numeric: tabular-nums; }
	.lm-td-url { max-width: 240px; }

	.lm-article { display: flex; align-items: center; gap: 12px; min-width: 0; }
	.lm-thumb { width: 40px; height: 40px; flex: none; border: 1px solid var(--border); border-radius: 8px; object-fit: cover; background-color: var(--muted); }
	.lm-title { display: block; max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 14px; line-height: 20px; font-weight: 500; }
	.lm-media { font-size: 14px; line-height: 20px; }
	.lm-media-sub { font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.lm-host { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.lm-url { display: inline-flex; max-width: 100%; align-items: center; gap: 6px; font-size: 13px; line-height: 20px; color: var(--accent-foreground); }
	.lm-url:hover { text-decoration: underline; }
	.lm-date { display: block; font-size: 13px; line-height: 20px; }
	.lm-rel { display: block; font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.lm-fail { font-size: 14px; color: var(--muted-foreground); }
	.lm-fail-bad { color: var(--destructive); font-weight: 500; }
	.lm-ok { color: var(--success); }
	.lm-warn { color: var(--warning); }

	.lm-badge, .lm-pill {
		display: inline-flex; width: fit-content; align-items: center; gap: 6px; padding: 2px 10px; border-radius: 9999px; white-space: nowrap;
		font-size: 12px; line-height: 16px; font-weight: 500; background-color: var(--muted); color: var(--muted-foreground);
	}
	.lm-badge-ok, .lm-pill-ok { background-color: var(--success-soft); color: var(--success); }
	.lm-badge-warn, .lm-pill-warn { background-color: var(--warning-soft); color: var(--warning); }
	.lm-badge-bad, .lm-pill-bad { background-color: color-mix(in srgb, var(--destructive) 12%, transparent); color: var(--destructive); }
	.lm-pill { padding: 2px 10px; }

	.lm-pager { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; margin-top: 16px; }
	.lm-pager-btn { min-height: 32px; padding: 0 12px; font-size: 13px; font-weight: 500; }

	.lm-cards { display: flex; flex-direction: column; gap: 12px; margin-top: 16px; }
	.lm-card { padding: 16px; }
	.lm-card-top { display: flex; align-items: flex-start; gap: 12px; }
	.lm-card-title { font-size: 15px; line-height: 20px; font-weight: 500; display: -webkit-box; -webkit-line-clamp: 3; line-clamp: 3; -webkit-box-orient: vertical; overflow: hidden; }
	.lm-dl { display: flex; flex-direction: column; gap: 12px; margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--border); }
	.lm-dl dt { font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.lm-dl dd { margin-top: 2px; font-size: 13px; }
	.lm-card-foot { display: flex; flex-wrap: wrap; gap: 4px 16px; margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--border); font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
</style>
