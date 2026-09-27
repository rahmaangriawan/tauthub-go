<script lang="ts">
	// Pelacak 404 — menjaga garansi tayang 12 bulan: memeriksa apakah artikel yang
	// sudah terbit di situs media mitra masih bisa dibuka.
	// Port apps/web/src/app/admin/link-monitor/page.tsx.
	import { onMount } from 'svelte';
	import { RefreshCw, Search, Unlink } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const PAGE_SIZE = 20;

	interface Summary {
		watched: number;
		live: number;
		gone: number;
		unreachable: number;
		no_url: number;
		domains_down: number;
		domains_unknown: number;
	}
	interface PublicationItem {
		id: string;
		document_title: string | null;
		brand_name: string | null;
		media_name: string | null;
		partner_name: string | null;
		published_url: string | null;
		published_at: string | null;
		state: string | null;
		http_status: number | null;
		consecutive_failures: number | null;
		first_detected_gone_at: string | null;
		last_checked_at: string | null;
		error_message: string | null;
	}
	interface DomainItem {
		media_id: string;
		name: string | null;
		website_url: string | null;
		partner_name: string | null;
		domain_health: string;
		domain_last_checked_at: string | null;
		domain_check_fail_count: number | null;
		domain_verified: boolean | number;
	}
	interface Page<T> {
		items: T[];
		total: number;
		limit: number;
		offset: number;
	}
	interface CheckResult {
		ok: boolean;
		queued_publications: number;
		queued_domains: number;
		total: number;
		capped: boolean;
	}

	/** Badge status link artikel — merah hanya untuk hilang nyata (404/410). */
	function linkMeta(state: string | null): {
		label: string;
		tone: 'neutral' | 'success' | 'warning' | 'danger';
		title: string;
	} {
		switch (state) {
			case 'gone':
				return {
					label: 'Hilang (404)',
					tone: 'danger',
					title:
						'Halaman artikel sudah tidak ada di situs media (404/410). Tekan Ganti Artikel untuk membuat pengganti di media lain.'
				};
			case 'unreachable':
				return {
					label: 'Tak Terjangkau',
					tone: 'warning',
					title:
						'Situs media menolak akses atau sedang error. Belum tentu hilang, jadi tidak dihitung sebagai artikel hilang.'
				};
			case 'live':
				return { label: 'Tayang', tone: 'success', title: 'Halaman artikel masih bisa dibuka di situs media.' };
			case 'no_url':
				return {
					label: 'Tanpa URL',
					tone: 'neutral',
					title: 'Publikasi ini belum punya URL, jadi halamannya belum bisa diperiksa.'
				};
			default:
				return { label: 'Belum Dicek', tone: 'neutral', title: 'Halaman artikel belum pernah diperiksa.' };
		}
	}

	/** Badge kesehatan domain mitra (media.domain_health). */
	function domainMeta(health: string): { label: string; tone: 'neutral' | 'success' | 'warning' | 'danger'; title: string } {
		switch (health) {
			case 'unreachable':
				return {
					label: 'Tak Terjangkau',
					tone: 'danger',
					title: 'Situs mitra tidak bisa dihubungi, sehingga artikel yang terbit di sana ikut tidak bisa dibuka.'
				};
			case 'token_missing':
				return {
					label: 'Token Hilang',
					tone: 'warning',
					title: 'Situs mitra hidup, tetapi penanda verifikasi domain tidak ditemukan lagi di halamannya.'
				};
			case 'live':
				return { label: 'Hidup', tone: 'success', title: 'Situs mitra bisa dihubungi dan penanda verifikasi domain ditemukan.' };
			default:
				return { label: 'Belum Dicek', tone: 'neutral', title: 'Situs mitra ini belum pernah diperiksa.' };
		}
	}

	let summary = $state<Summary | null>(null);
	let pubs = $state<Page<PublicationItem>>({ items: [], total: 0, limit: 500, offset: 0 });
	let domains = $state<Page<DomainItem>>({ items: [], total: 0, limit: 500, offset: 0 });
	let loading = $state(true);
	let checking = $state(false);
	let busyId = $state<string | null>(null);
	let error = $state('');
	let notice = $state('');
	let stateFilter = $state('');
	let healthFilter = $state('');
	let search = $state('');
	let tab = $state('articles');
	let pubsPage = $state(1);
	let domainsPage = $state(1);

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
		loading = true;
		error = '';
		const pubQuery = new URLSearchParams({ view: 'publications', limit: '500' });
		if (stateFilter) pubQuery.set('state', stateFilter);
		if (search.trim()) pubQuery.set('q', search.trim());
		const domQuery = new URLSearchParams({ view: 'domains', limit: '500' });
		if (healthFilter) domQuery.set('health', healthFilter);
		try {
			const [sumRes, pubsRes, domsRes] = await Promise.all([
				apiRequest('/v1/platform/link-monitor?view=summary'),
				apiRequest(`/v1/platform/link-monitor?${pubQuery}`),
				apiRequest(`/v1/platform/link-monitor?${domQuery}`)
			]);
			if (!sumRes.ok) {
				error = await readApiError(sumRes, 'Permintaan gagal.');
				return;
			}
			summary = await sumRes.json();
			pubs = pubsRes.ok ? await pubsRes.json() : { items: [], total: 0, limit: 500, offset: 0 };
			domains = domsRes.ok ? await domsRes.json() : { items: [], total: 0, limit: 500, offset: 0 };
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	async function checkNow() {
		checking = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest('/v1/platform/link-monitor/check', { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const result = (await res.json()) as CheckResult;
			if (result.total === 0) {
				notice =
					'Tidak ada pengecekan baru yang diantrekan — semua artikel dan domain masih dalam masa berlaku cek 7 hari.';
			} else {
				notice = `Antrean pengecekan dibuat: ${result.queued_publications} artikel dan ${result.queued_domains} domain.${result.capped ? ' Dibuat maksimal 200 per kategori; sisanya menyusul pada siklus berikutnya.' : ''} Hasil muncul setelah worker selesai.`;
			}
			await load();
		} finally {
			checking = false;
		}
	}

	async function replaceArticle(publicationId: string) {
		busyId = publicationId;
		error = '';
		notice = '';
		try {
			const res = await apiRequest('/v1/platform/link-monitor/replace', {
				method: 'POST',
				body: JSON.stringify({ publicationId })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			notice = 'Artikel digantikan ke media alternatif. Publikasi pengganti sudah dibuat dan masuk antrean proses.';
			await load();
		} finally {
			busyId = null;
		}
	}

	const metrics = $derived([
		{ label: 'Artikel Dipantau', value: summary?.watched ?? 0, color: undefined },
		{ label: 'Tayang', value: summary?.live ?? 0, color: 'var(--success)' },
		{
			label: 'Hilang (404)',
			value: summary?.gone ?? 0,
			color: (summary?.gone ?? 0) > 0 ? 'var(--destructive)' : 'var(--muted-foreground)'
		},
		{
			label: 'Tak Terjangkau',
			value: summary?.unreachable ?? 0,
			color: (summary?.unreachable ?? 0) > 0 ? 'var(--warning)' : 'var(--muted-foreground)'
		},
		{
			label: 'Domain Mati',
			value: summary?.domains_down ?? 0,
			color: (summary?.domains_down ?? 0) > 0 ? 'var(--destructive)' : 'var(--muted-foreground)'
		},
		{ label: 'Domain Belum Dicek', value: summary?.domains_unknown ?? 0, color: undefined }
	]);

	const visiblePubs = $derived(pubs.items.slice((pubsPage - 1) * PAGE_SIZE, pubsPage * PAGE_SIZE));
	const visibleDomains = $derived(domains.items.slice((domainsPage - 1) * PAGE_SIZE, domainsPage * PAGE_SIZE));
	const websiteHref = (url: string | null) =>
		url ? (/^[a-z][a-z0-9+.-]*:\/\//i.test(url) ? url : `https://${url}`) : '';
</script>

{#snippet articleCell(item: PublicationItem)}
	<div>
		<p class="font-medium">{item.document_title ?? 'Tanpa judul'}</p>
		<p class="text-xs text-muted-foreground">{item.brand_name ?? '—'}</p>
	</div>
{/snippet}

{#snippet articleMediaCell(item: PublicationItem)}
	<div>
		<p>{item.media_name ?? '—'}</p>
		<p class="text-xs text-muted-foreground">{item.partner_name ?? 'Tanpa mitra'}</p>
	</div>
{/snippet}

{#snippet urlCell(item: PublicationItem)}
	<div>
		{#if item.published_url}
			<a
				class="truncate text-xs"
				style="color: var(--primary); max-width: 260px; display: block;"
				href={item.published_url}
				target="_blank"
				rel="noreferrer noopener"
				title={item.published_url}>{item.published_url}</a
			>
		{:else}
			<span class="text-xs text-muted-foreground">—</span>
		{/if}
		{#if item.http_status != null}
			<p class="text-xs text-muted-foreground">HTTP {item.http_status}</p>
		{/if}
	</div>
{/snippet}

{#snippet linkStatusCell(item: PublicationItem)}
	{@const meta = linkMeta(item.state)}
	<div class="max-w-[280px]">
		<span title={meta.title}><Badge tone={meta.tone} label={meta.label} /></span>
		{#if item.error_message}
			<p class="mt-1 truncate text-xs text-muted-foreground" title={item.error_message}>{item.error_message}</p>
		{/if}
	</div>
{/snippet}

{#snippet failuresCell(item: PublicationItem)}
	<span class="tabular-nums">{item.consecutive_failures ?? 0}x</span>
{/snippet}

{#snippet lastCheckedCell(item: PublicationItem)}
	<div>
		<p class="text-xs">{fmtDateTime(item.last_checked_at)}</p>
		{#if item.first_detected_gone_at}
			<p class="text-xs" style="color: var(--destructive);">Sejak {fmtDateTime(item.first_detected_gone_at)}</p>
		{/if}
	</div>
{/snippet}

{#snippet replaceCell(item: PublicationItem)}
	<div class="text-right">
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			style="color: var(--destructive);"
			disabled={busyId !== null || item.state !== 'gone'}
			title={item.state === 'gone'
				? 'Buat publikasi pengganti ke media alternatif'
				: 'Hanya artikel berstatus hilang (404) yang bisa diganti'}
			onclick={() => void replaceArticle(item.id)}
			>{busyId === item.id ? 'Memproses…' : 'Ganti Artikel'}</button
		>
	</div>
{/snippet}

{#snippet domainNameCell(item: DomainItem)}
	<div>
		<p class="font-medium">{item.name ?? 'Tanpa nama'}</p>
		<p class="text-xs text-muted-foreground">
			{item.domain_verified === true || item.domain_verified === 1
				? 'Domain terverifikasi'
				: 'Domain belum terverifikasi'}
		</p>
	</div>
{/snippet}

{#snippet websiteCell(item: DomainItem)}
	{#if item.website_url}
		<a
			class="text-xs"
			style="color: var(--primary);"
			href={websiteHref(item.website_url)}
			target="_blank"
			rel="noreferrer noopener"
			title={item.website_url}>{item.website_url}</a
		>
	{:else}
		<span class="text-xs text-muted-foreground">Website belum diisi</span>
	{/if}
{/snippet}

{#snippet domainHealthCell(item: DomainItem)}
	{@const meta = domainMeta(item.domain_health)}
	<span title={meta.title}><Badge tone={meta.tone} label={meta.label} /></span>
{/snippet}

{#snippet domainCheckedCell(item: DomainItem)}
	<span class="text-xs">{fmtDateTime(item.domain_last_checked_at)}</span>
{/snippet}

{#snippet domainFailCell(item: DomainItem)}
	<span class="tabular-nums">{item.domain_check_fail_count ?? 0}x</span>
{/snippet}

{#snippet domainPartnerCell(item: DomainItem)}
	<span>{item.partner_name ?? '—'}</span>
{/snippet}

<section class="mx-auto max-w-7xl">
	<PageHeader
		title="Pelacak 404"
		description="Halaman ini menjaga garansi tayang 12 bulan: sistem memeriksa apakah artikel yang sudah terbit di situs media mitra masih bisa dibuka. Kalau ada halaman yang hilang, artikel itu ditandai di baris daftar supaya bisa segera diganti di media lain. Pengecekan ulang berjalan otomatis tiap 7 hari dan bisa diantrekan manual lewat tombol Cek Sekarang."
	>
		<Badge
			tone={(summary?.gone ?? 0) > 0 ? 'danger' : 'success'}
			label={`${summary?.gone ?? 0} artikel hilang`}
		/>
		<button class="app-btn app-btn-primary" onclick={checkNow} disabled={checking}>
			<RefreshCw size={15} /> {checking ? 'Mengantrekan…' : 'Cek Sekarang'}
		</button>
	</PageHeader>

	<Feedback {loading} {error} {notice} />

	<div class="mt-4 flex flex-wrap gap-3">
		{#each metrics as metric (metric.label)}
			<div class="app-card min-w-[140px] flex-1 p-3">
				<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{metric.label}</p>
				<p class="text-2xl font-bold" style={metric.color ? `color: ${metric.color};` : undefined}>{metric.value}</p>
			</div>
		{/each}
	</div>

	<div class="mt-4 flex flex-wrap gap-1 border-b" role="tablist">
		<button
			type="button"
			class="app-btn !min-h-8 !rounded-b-none !px-3.5 !text-sm"
			class:app-btn-primary={tab === 'articles'}
			class:app-btn-soft={tab !== 'articles'}
			onclick={() => (tab = 'articles')}>Artikel Hilang ({pubs.items.length})</button
		>
		<button
			type="button"
			class="app-btn !min-h-8 !rounded-b-none !px-3.5 !text-sm"
			class:app-btn-primary={tab === 'domains'}
			class:app-btn-soft={tab !== 'domains'}
			onclick={() => (tab = 'domains')}>Domain Mitra ({domains.items.length})</button
		>
	</div>

	<div class="pt-4">
		{#if tab === 'articles'}
			<div class="app-card mb-3 px-4 py-2.5 text-sm text-muted-foreground">
				Sistem membuka halaman artikel di situs media mitra untuk memastikan halamannya masih bisa diakses. Kalau halaman
				tidak ditemukan (Hilang 404), tekan tombol Ganti Artikel pada baris tersebut untuk membuat artikel pengganti di
				media lain. Status Tak Terjangkau tidak dihitung sebagai artikel hilang, karena situsnya mungkin hanya sedang
				memblokir akses atau mati sementara.
			</div>

			<form class="mb-3 flex flex-wrap items-center gap-2" onsubmit={(e) => e.preventDefault()}>
				<label class="app-input flex w-[280px] items-center gap-2 !py-1.5">
					<Search size={14} class="text-muted-foreground" />
					<input
						class="w-full bg-transparent text-sm outline-none"
						placeholder="Cari judul, media, mitra, atau URL…"
						bind:value={search}
					/>
				</label>
				<Select
					ariaLabel="Filter status artikel"
					minWidth={300}
					value={stateFilter}
					onchange={(v) => {
						stateFilter = v;
						notice = '';
						pubsPage = 1;
						void load();
					}}
					options={[
						{ value: '', label: 'Bermasalah (Hilang + Tak Terjangkau)' },
						{ value: 'gone', label: 'Hilang (404) — halaman sudah tidak ada' },
						{ value: 'unreachable', label: 'Tak Terjangkau — situs menolak akses atau sedang error' },
						{ value: 'live', label: 'Tayang — artikel masih bisa diakses' },
						{ value: 'no_url', label: 'Tanpa URL' }
					]}
				/>
				<button
					type="submit"
					class="app-btn app-btn-soft"
					onclick={() => {
						pubsPage = 1;
						void load();
					}}>Terapkan</button
				>
				<button
					type="button"
					class="app-btn app-btn-soft"
					onclick={() => {
						search = '';
						stateFilter = '';
						notice = '';
						pubsPage = 1;
						void load();
					}}>Reset</button
				>
			</form>

			{#if loading}
				<p class="text-sm text-muted-foreground">Memuat data…</p>
			{:else if pubs.items.length === 0}
				<div class="app-card px-6 py-10 text-center">
					<Unlink size={28} class="mx-auto" />
					<p class="mt-2 text-sm text-muted-foreground">
						{stateFilter || search.trim()
							? 'Tidak ada artikel yang cocok dengan filter ini. Kosongkan filter atau jalankan Cek Sekarang untuk memperbarui data.'
							: `Tidak ada artikel hilang atau tak terjangkau. ${summary?.watched ?? 0} artikel sedang dipantau — pengecekan ulang otomatis berjalan tiap 7 hari.`}
					</p>
				</div>
			{:else}
				<div class="hidden lg:block">
				<DataTable
					columns={[
						{ key: 'article', label: 'Artikel', render: articleCell },
						{ key: 'media', label: 'Media', render: articleMediaCell },
						{ key: 'url', label: 'URL', render: urlCell },
						{ key: 'status', label: 'Status', render: linkStatusCell },
						{ key: 'failures', label: 'Gagal Berturut', render: failuresCell },
						{ key: 'checked', label: 'Terakhir Dicek', render: lastCheckedCell },
						{ key: 'actions', label: '', align: 'right' as const, render: replaceCell }
					]}
					rows={visiblePubs}
					rowKey={(item: PublicationItem) => item.id}
					{loading}
					empty="Tidak ada artikel yang cocok dengan filter ini."
					minWidth="1200px"
				/>
				</div>
				<!-- Kartu ringkas untuk layar kecil (parity varian mobile halaman lama). -->
				<div class="mt-3 grid gap-3 lg:hidden">
					{#each visiblePubs as item (item.id)}
						{@const m = linkMeta(item.state)}
						<div class="app-card p-4">
							<p class="font-medium">{item.document_title ?? 'Tanpa judul'}</p>
							<p class="text-xs text-muted-foreground">{item.brand_name ?? '—'}</p>
							<div class="mt-3 space-y-1.5 text-sm">
								<p class="text-xs"><span class="text-muted-foreground">Media:</span> {item.media_name ?? '—'}{item.partner_name ? ` · ${item.partner_name}` : ''}</p>
								<p class="break-all text-xs"><span class="text-muted-foreground">URL:</span> {item.published_url ?? '—'}</p>
								<p class="text-xs"><span class="text-muted-foreground">HTTP:</span> {item.http_status != null ? String(item.http_status) : '—'}</p>
								<p><span title={m.title}><Badge tone={m.tone} label={m.label} /></span></p>
								{#if item.error_message}<p class="text-xs"><span class="text-muted-foreground">Keterangan:</span> {item.error_message}</p>{/if}
								<p class="text-xs"><span class="text-muted-foreground">Gagal berturut:</span> {item.consecutive_failures ?? 0}x</p>
								<p class="text-xs"><span class="text-muted-foreground">Terakhir dicek:</span> {fmtDateTime(item.last_checked_at)}</p>
								{#if item.first_detected_gone_at}<p class="text-xs"><span class="text-muted-foreground">Sejak hilang:</span> {fmtDateTime(item.first_detected_gone_at)}</p>{/if}
							</div>
							<button
								class="app-btn app-btn-soft mt-3 !min-h-7 !px-2.5 !text-xs"
								style="color: var(--destructive);"
								disabled={busyId !== null || item.state !== 'gone'}
								onclick={() => void replaceArticle(item.id)}
								>{busyId === item.id ? 'Memproses…' : 'Ganti Artikel'}</button
							>
						</div>
					{/each}
				</div>
				<Pagination
					total={pubs.items.length}
					limit={PAGE_SIZE}
					offset={(pubsPage - 1) * PAGE_SIZE}
					onChange={(next) => (pubsPage = next / PAGE_SIZE + 1)}
				/>
				{#if pubs.total > pubs.items.length}
					<p class="mt-2 text-xs text-muted-foreground">
						Menampilkan {pubs.items.length} dari total {pubs.total} artikel bermasalah. Persempit dengan filter untuk melihat
						sisanya.
					</p>
				{/if}
			{/if}
		{:else}
			<div class="app-card mb-3 px-4 py-2.5 text-sm text-muted-foreground">
				Bagian ini memeriksa apakah situs media mitra masih bisa dihubungi sama sekali. Domain yang mati penting karena
				semua artikel yang terbit di sana jadi tidak bisa dibuka, jadi perlu segera ditangani.
			</div>

			<form class="mb-3 flex flex-wrap items-center gap-2" onsubmit={(e) => e.preventDefault()}>
				<Select
					ariaLabel="Filter kesehatan domain"
					minWidth={300}
					value={healthFilter}
					onchange={(v) => {
						healthFilter = v;
						notice = '';
						domainsPage = 1;
						void load();
					}}
					options={[
						{ value: '', label: 'Perlu perhatian (Mati + Belum Dicek)' },
						{ value: 'unreachable', label: 'Domain mati — situs tidak bisa dihubungi' },
						{ value: 'unknown', label: 'Belum dicek' },
						{ value: 'token_missing', label: 'Token verifikasi hilang — situs hidup, tapi penanda verifikasi tidak ditemukan' },
						{ value: 'live', label: 'Hidup' }
					]}
				/>
				<button
					type="button"
					class="app-btn app-btn-soft"
					onclick={() => {
						healthFilter = '';
						notice = '';
						domainsPage = 1;
						void load();
					}}>Reset</button
				>
			</form>

			{#if loading}
				<p class="text-sm text-muted-foreground">Memuat data…</p>
			{:else if domains.items.length === 0}
				<div class="app-card px-6 py-10 text-center">
					<Unlink size={28} class="mx-auto" />
					<p class="mt-2 text-sm text-muted-foreground">
						{healthFilter
							? 'Tidak ada domain mitra dengan status kesehatan yang dipilih. Kosongkan filter atau jalankan Cek Sekarang untuk memperbarui data.'
							: 'Tidak ada domain mitra yang mati atau belum dicek. Pengecekan ulang otomatis berjalan tiap 7 hari untuk semua media mitra yang punya website.'}
					</p>
				</div>
			{:else}
				<div class="hidden lg:block">
				<DataTable
					columns={[
						{ key: 'media', label: 'Media', render: domainNameCell },
						{ key: 'partner', label: 'Mitra', render: domainPartnerCell },
						{ key: 'website', label: 'Website', render: websiteCell },
						{ key: 'status', label: 'Status', render: domainHealthCell },
						{ key: 'checked', label: 'Terakhir Dicek', render: domainCheckedCell },
						{ key: 'fail', label: 'Gagal', render: domainFailCell }
					]}
					rows={visibleDomains}
					rowKey={(item: DomainItem) => item.media_id}
					{loading}
					empty="Tidak ada domain mitra dengan status kesehatan yang dipilih."
					minWidth="1000px"
				/>
				</div>
				<!-- Kartu ringkas untuk layar kecil (parity varian mobile halaman lama). -->
				<div class="mt-3 grid gap-3 lg:hidden">
					{#each visibleDomains as item (item.media_id)}
						{@const m = domainMeta(item.domain_health)}
						<div class="app-card p-4">
							<p class="font-medium">{item.name ?? 'Tanpa nama'}</p>
							<p class="text-xs text-muted-foreground">{item.domain_verified === true || item.domain_verified === 1 ? 'Domain terverifikasi' : 'Domain belum terverifikasi'}</p>
							<div class="mt-3 space-y-1.5 text-sm">
								<p class="text-xs"><span class="text-muted-foreground">Mitra:</span> {item.partner_name ?? '—'}</p>
								<p class="break-all text-xs"><span class="text-muted-foreground">Website:</span> {item.website_url ?? 'Website belum diisi'}</p>
								<p><span title={m.title}><Badge tone={m.tone} label={m.label} /></span></p>
								<p class="text-xs"><span class="text-muted-foreground">Terakhir dicek:</span> {fmtDateTime(item.domain_last_checked_at)}</p>
								<p class="text-xs"><span class="text-muted-foreground">Gagal:</span> {item.domain_check_fail_count ?? 0}x</p>
							</div>
						</div>
					{/each}
				</div>
				<Pagination
					total={domains.items.length}
					limit={PAGE_SIZE}
					offset={(domainsPage - 1) * PAGE_SIZE}
					onChange={(next) => (domainsPage = next / PAGE_SIZE + 1)}
				/>
				{#if domains.total > domains.items.length}
					<p class="mt-2 text-xs text-muted-foreground">
						Menampilkan {domains.items.length} dari total {domains.total} domain. Persempit dengan filter untuk melihat
						sisanya.
					</p>
				{/if}
			{/if}
		{/if}
	</div>

	<p class="mt-4 text-xs text-muted-foreground">
		Artikel dipantau selama 12 bulan sejak terbit. Status Hilang (404) berarti halaman sudah tidak ada dan bisa diganti ke
		media lain; Tak Terjangkau berarti situs menolak akses atau sedang error dan tidak dianggap hilang.
	</p>
</section>
