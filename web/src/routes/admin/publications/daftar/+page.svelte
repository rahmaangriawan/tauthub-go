<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Publikasi admin — semua publikasi seluruh brand (staff visibility).
	// Port apps/web/src/app/admin/publications/page.tsx.
	import { onMount } from 'svelte';
	import { Ban, Download, ImagePlus, Megaphone, Search } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Tooltip from '$lib/components/tooltip.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import { sameOriginAssetUrl } from '$lib/media/cover';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const PAGE_SIZE = 20;

	type Row = {
		id: string;
		status: string;
		client_price: number;
		document_title: string | null;
		media_name: string | null;
		brand_name: string | null;
		campaign_name: string | null;
		created_at: string;
		/** Jadwal terbit yang dipesan klien (null = tanpa jadwal). */
		scheduled_at: string | null;
		published_at: string | null;
		/** 'ai' = pernah dibuatkan The Writer; 'manual' = Tulis Sendiri. */
		article_source: 'ai' | 'manual';
		error_code: string | null;
		variant: { angle_label: string; mode: string | null } | null;
		media_integration_type: string | null;
		published_url: string | null;
		remote_post_id: string | null;
		/** Cover artikel yang sama dengan sisi klien (taut_campaign_media). */
		featured_image_url: string | null;
	};

	const STATUS_TONE: Record<string, 'neutral' | 'accent' | 'success' | 'warning' | 'danger'> = {
		published: 'success',
		pending: 'warning',
		waiting_partner: 'warning',
		revision: 'warning',
		partner_approved: 'accent',
		partner_rejected: 'danger',
		queued: 'accent',
		processing: 'accent',
		failed: 'danger',
		retrying: 'warning',
		cancelled: 'neutral'
	};

	const filters = [
		'',
		'pending',
		'waiting_partner',
		'revision',
		'queued',
		'processing',
		'published',
		'failed',
		'retrying',
		'partner_rejected',
		'cancelled'
	];

	// Label filter memakai bahasa manusia, bukan nilai mentah status
	// (waiting_partner, partner_rejected, dst. hanya untuk mesin).
	const FILTER_LABEL: Record<string, string> = $derived({
		pending: tr('Menunggu diproses'),
		waiting_partner: tr('Menunggu persetujuan media'),
		revision: tr('Perlu revisi'),
		queued: tr('Antrean'),
		processing: tr('Diproses'),
		published: tr('Terbit'),
		failed: tr('Gagal'),
		retrying: tr('Mencoba ulang'),
		partner_rejected: tr('Ditolak media'),
		cancelled: tr('Dibatalkan')
	});

	let rows = $state<Row[]>([]);
	let filter = $state('');
	let query = $state('');
	let error = $state('');
	let loading = $state(true);
	// Thumbnail yang gagal dimuat kembali memakai ikon (jangan mencoba terus).
	let thumbFailed = $state<Record<string, boolean>>({});
	let exporting = $state(false);
	let page = $state(1);
	let total = $state(0);
	let syncBusyId = $state('');
	let syncNotice = $state('');
	let syncError = $state('');
	let cancelBusyId = $state('');
	let cancelTarget = $state<Row | null>(null);
	let cancelReason = $state('');
	let cancelError = $state('');

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

	let queryTimer: ReturnType<typeof setTimeout> | null = null;
	$effect(() => {
		const value = query;
		if (queryTimer) clearTimeout(queryTimer);
		queryTimer = setTimeout(() => {
			page = 1;
			void load();
		}, 300);
		return () => {
			if (queryTimer) clearTimeout(queryTimer);
		};
	});
	async function load() {
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams();
			if (filter) params.set('status', filter);
			if (query.trim()) params.set('q', query.trim());
			params.set('limit', String(PAGE_SIZE));
			params.set('offset', String((page - 1) * PAGE_SIZE));
			const res = await apiRequest(`/v1/platform/operations/publications?${params}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const data = await res.json();
			rows = data.publications ?? [];
			total = data.total ?? 0;
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	async function exportCsv() {
		exporting = true;
		error = '';
		try {
			const params = new URLSearchParams({ format: 'csv' });
			if (filter) params.set('status', filter);
			if (query.trim()) params.set('q', query.trim());
			const response = await apiRequest(`/v1/platform/operations/publications?${params}`);
			if (!response.ok) throw new Error(await response.text());
			const blob = await response.blob();
			const a = document.createElement('a');
			a.href = URL.createObjectURL(blob);
			a.download = `tauthub-publikasi-${new Date().toISOString().slice(0, 10)}.csv`;
			a.click();
			URL.revokeObjectURL(a.href);
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			exporting = false;
		}
	}

	const idr = (n: unknown) =>
		new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0, roundingMode: 'trunc' }).format(
			Number(n ?? 0)
		);

	// Tombol sinkron hanya masuk akal untuk post WordPress yang sudah terbit dan
	// masih bisa ditelusuri post-nya (ID tersimpan, atau tautannya berbentuk ?p=).
	const canResync = (row: Row) =>
		row.status === 'published' &&
		row.media_integration_type === 'wordpress' &&
		(!!row.remote_post_id || /\?p=\d+/.test(row.published_url ?? ''));

	async function resyncRow(row: Row) {
		if (
			!confirm(
				`Sinkronkan ulang gambar utama & kategori untuk “${row.document_title ?? row.id.slice(0, 8)}”?\n\n` +
					'Post di WordPress akan DIPERBARUI (judul dan isi ikut dikirim ulang), bukan dibuat baru.'
			)
		)
			return;
		syncBusyId = row.id;
		syncNotice = '';
		syncError = '';
		try {
			const res = await apiRequest(
				`/v1/platform/operations/publications/${row.id}/resync-wordpress`,
				{ method: 'POST' }
			);
			if (!res.ok) {
				syncError = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			syncNotice = `Sinkronisasi diantrekan untuk “${row.document_title ?? row.id.slice(0, 8)}”. Gambar utama dan kategori akan dikirim ulang ke post WordPress yang sama.`;
			void load();
		} finally {
			syncBusyId = '';
		}
	}
	const showAccess = $derived(rows.length === 0 && error.includes('Akses platform'));

	/** Pembatalan hanya untuk pesanan yang belum terbit dan belum batal. */
	const canCancel = (row: Row) => row.status !== 'published' && row.status !== 'cancelled';

	async function confirmCancel() {
		if (!cancelTarget) return;
		cancelBusyId = cancelTarget.id;
		cancelError = '';
		try {
			const res = await apiRequest(
				`/v1/platform/operations/publications/${cancelTarget.id}/cancel`,
				{ method: 'POST', body: JSON.stringify({ reason: cancelReason.trim() }) }
			);
			if (!res.ok) {
				cancelError = await readApiError(res, 'Gagal membatalkan publikasi.');
				return;
			}
			syncNotice = `Publikasi “${cancelTarget.document_title ?? cancelTarget.id.slice(0, 8)}” dibatalkan. Kredit reservasi dikembalikan ke Saldo Akun klien.`;
			cancelTarget = null;
			cancelReason = '';
			void load();
		} finally {
			cancelBusyId = '';
		}
	}
</script>

{#snippet articleCell(row: Row)}
	<!-- Batas 350px dipasang di ISI sel, bukan di <td>: browser mengabaikan
	     max-width pada sel tabel (auto layout), dan tanpa ini judul nowrap
	     meluber menimpa kolom sebelah. -->
	<a href={`/admin/publications/${row.id}`} class="block max-w-[350px]" style="color: var(--primary); text-decoration: none;">
		<div class="flex items-center gap-2">
			<!-- Thumbnail = cover yang sama dengan yang dipakai sisi klien; ikon
			     toa hanya muncul bila artikel tidak punya cover atau gagal dimuat. -->
			{#if row.featured_image_url && !thumbFailed[row.id]}
				<img
					src={sameOriginAssetUrl(row.featured_image_url) ?? row.featured_image_url}
					alt=""
					class="h-10 w-14 shrink-0 rounded-[5px] border border-slate-200 object-cover"
					loading="lazy"
					decoding="async"
					onerror={() => (thumbFailed = { ...thumbFailed, [row.id]: true })}
				/>
			{:else}
				<span class="flex h-10 w-14 shrink-0 items-center justify-center rounded-[5px] bg-slate-100 text-slate-400">
					<Megaphone size={14} />
				</span>
			{/if}
				<div class="min-w-0 flex-1">
					<!-- Baris judul + badge sengaja tanpa flex-wrap: judul selalu satu
					     baris (truncate) dan badge menempel di kanannya. -->
					<div class="flex items-center gap-2">
						<div class="min-w-0 flex-1">
							{#if row.document_title}
								<!-- Tooltip styled (shadcn-style, delay 0) kembali dipakai.
								     Wajib `block`: tanpa itu anchor inline-flex shrink-wrap ke
								     lebar teks nowrap sehingga truncate gagal dan judul meluber
								     menimpa kolom sebelah. -->
								<Tooltip content={row.document_title} maxWidth={380} delay={0} block>
									<span class="block truncate font-medium" style="color: inherit;">{row.document_title}</span>
								</Tooltip>
							{:else}
								<p class="truncate font-medium" style="color: inherit;">{row.id.slice(0, 8)}</p>
							{/if}
						</div>
					{#if row.variant}<Badge tone="accent" label="Variant" />{/if}
					<!-- Sumber konten: pernah dibuatkan AI atau ditulis sendiri klien. -->
					<span
						class="shrink-0 rounded-[5px] border px-2 py-0.5 text-[10px] font-semibold {row.article_source === 'ai'
							? 'border-violet-200 bg-violet-50 text-violet-700'
							: 'border-slate-200 bg-slate-100/80 text-slate-600'}"
						title={row.article_source === 'ai' ? tr('Artikel dihasilkan The Writer (Generate AI)') : tr('Artikel diunggah/ditulis sendiri oleh klien (Tulis Sendiri)')}
					>
						{row.article_source === 'ai' ? tr('Generate AI') : tr('Tulis Sendiri')}
					</span>
				</div>
				{#if row.variant}
					<p class="text-xs" style="color: var(--primary);">
						{row.variant.mode === 'generate' ? 'Dibuatkan' : 'Parafrase'} · {row.variant.angle_label}
					</p>
				{:else}
					<p class="text-xs text-muted-foreground">{row.id.slice(0, 8)}</p>
				{/if}
			</div>
		</div>
	</a>
{/snippet}

{#snippet brandCell(row: Row)}
	<span>{row.brand_name ?? '—'}</span>
{/snippet}

{#snippet mediaCell(row: Row)}
	<div>
		<p>{row.media_name ?? '—'}</p>
		{#if row.campaign_name}
			<p class="text-xs text-muted-foreground">Campaign: {row.campaign_name}</p>
		{/if}
	</div>
{/snippet}

{#snippet priceCell(row: Row)}
	<span class="tabular-nums">{idr(row.client_price)}</span>
{/snippet}

{#snippet pubStatusCell(row: Row)}
	<div>
		<Badge tone={STATUS_TONE[row.status] ?? 'neutral'} value={FILTER_LABEL[row.status] ?? row.status} />
		{#if row.error_code}
			<p class="mt-1 text-xs" style="color: var(--destructive);">{row.error_code}</p>
		{/if}
	</div>
{/snippet}

{#snippet scheduleCell(row: Row)}
	{#if row.scheduled_at}
		<span class="text-xs">{fmtDateTime(row.scheduled_at)}</span>
		{#if row.published_at}
			<p class="mt-1 text-xs" style="color: var(--success);">{tr('Terbit')} {fmtDateTime(row.published_at)}</p>
		{/if}
	{:else if row.published_at}
		<p class="text-xs" style="color: var(--success);">{tr('Terbit')} {fmtDateTime(row.published_at)}</p>
	{:else}
		<span class="text-xs text-muted-foreground">{tr('Tanpa jadwal')}</span>
	{/if}
{/snippet}

{#snippet actionCell(row: Row)}
	<div class="flex flex-wrap items-center gap-1.5">
		{#if canResync(row)}
			<button
				class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
				disabled={syncBusyId === row.id}
				onclick={() => resyncRow(row)}
			>
				<ImagePlus size={14} />{syncBusyId === row.id ? 'Mengantrekan…' : tr('Sinkron ulang')}
			</button>
		{/if}
		{#if canCancel(row)}
			<button
				class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
				style="color: var(--destructive);"
				disabled={cancelBusyId === row.id}
				onclick={() => {
					cancelTarget = row;
					cancelReason = '';
					cancelError = '';
				}}
			>
				<Ban size={14} />{tr('Batalkan')}
			</button>
		{/if}
		{#if !canResync(row) && !canCancel(row)}
			<span class="text-xs text-muted-foreground">—</span>
		{/if}
	</div>
{/snippet}

<section>
	<ExplorerFrame parents={[{ label: tr('Publikasi'), href: '/admin/publications' }]} crumb={tr('Daftar Publikasi')} path='/admin/publications/daftar'>
	<PageHeader
		title={tr("Publikasi")}
		description={tr("Semua publikasi seluruh brand (staff visibility). Klik baris untuk detail lengkap: riwayat attempts, job delivery, dan payload.")}
	>
		<Badge tone="accent" value={`${total} publikasi`} />
	</PageHeader>

	<Feedback {loading} {error} />

	{#if syncNotice}
		<p class="mt-3 text-sm" style="color: var(--success);">{syncNotice}</p>
	{/if}
	{#if syncError}
		<p class="mt-3 text-sm" style="color: var(--destructive);">{syncError}</p>
	{/if}

	<div class="mt-4 flex flex-wrap items-center gap-2">
		<label class="app-input flex min-w-[240px] max-w-[360px] flex-1 items-center gap-2 !py-1.5">
			<Search size={16} class="text-muted-foreground" />
			<input
				class="w-full bg-transparent text-sm outline-none"
				placeholder={tr("Cari artikel, klien, media, campaign, partner, atau ID…")}
				bind:value={query}
			/>
		</label>
		<button class="app-btn app-btn-soft" onclick={exportCsv} disabled={exporting}>
			<Download size={16} />{exporting ? 'Menyiapkan…' : 'Export CSV'}
		</button>
	</div>

	<div class="mt-3 flex flex-wrap gap-2">
		{#each filters as f (f)}
			<button
				type="button"
				class="app-btn !min-h-8 !px-3 !text-xs"
				class:app-btn-primary={filter === f}
				class:app-btn-soft={filter !== f}
				onclick={() => {
					filter = f;
					page = 1;
					void load();
				}}>{f === '' ? tr('Semua') : FILTER_LABEL[f] ?? f}</button
			>
		{/each}
	</div>

	{#if showAccess}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke data publikasi.")}</p>
		</div>
	{:else}
		<DataTable
			columns={[
				{ key: 'article', label: 'Artikel', width: '350px', render: articleCell },
				{ key: 'brand', label: tr('Klien'), render: brandCell },
				{ key: 'media', label: tr('Media'), render: mediaCell },
				{ key: 'price', label: tr('Harga'), render: priceCell },
				{ key: 'status', label: 'Status', render: pubStatusCell },
				{ key: 'schedule', label: tr('Jadwal Terbit'), width: '170px', render: scheduleCell },
				{ key: 'action', label: tr('Aksi'), render: actionCell }
			]}
			rows={rows}
			rowKey={(row: Row) => row.id}
			{loading}
			empty={tr("Tidak ada publikasi untuk filter ini.")}
			minWidth="1240px"
		/>

		<div class="mt-3 flex flex-wrap items-center justify-between gap-2">
			<p class="text-xs text-muted-foreground">
				{total} publikasi · halaman {page} {tr("dari")} {Math.max(1, Math.ceil(total / PAGE_SIZE))}
			</p>
			<div class="flex gap-2">
				<button
					class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
					disabled={page <= 1 || loading}
					onclick={() => {
						page = Math.max(1, page - 1);
						void load();
					}}>‹ Sebelumnya</button
				>
				<button
					class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
					disabled={page >= Math.max(1, Math.ceil(total / PAGE_SIZE)) || loading}
					onclick={() => {
						page = page + 1;
						void load();
					}}>Berikutnya ›</button
				>
			</div>
		</div>
	{/if}
	</ExplorerFrame>
</section>

{#if cancelTarget}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);">
		<button
			type="button"
			aria-label={tr('Tutup')}
			tabindex="-1"
			style="position:absolute; inset:0; border:none; background:transparent; cursor:default;"
			onclick={() => {
				if (!cancelBusyId) cancelTarget = null;
			}}
		></button>
		<dialog
			open
			class="app-card relative z-10 w-full max-w-md p-5"
			aria-label={tr('Batalkan publikasi')}
			onkeydown={(e) => {
				if (e.key === 'Escape' && !cancelBusyId) cancelTarget = null;
			}}
		>
			<h2 class="text-base font-bold">{tr('Batalkan publikasi')}</h2>
			<p class="mt-2 text-sm text-muted-foreground">
				{tr('Pesanan')} <strong style="color: var(--foreground);">{cancelTarget.document_title ?? cancelTarget.id.slice(0, 8)}</strong>
				{tr('akan dibatalkan. Kredit reservasi dikembalikan ke Saldo Akun klien, dan klien serta mitra menerima notifikasi.')}
			</p>
			<label class="mt-3 flex flex-col gap-1 text-sm">
				{tr('Alasan (opsional)')}
				<textarea
					class="app-input"
					rows="3"
					bind:value={cancelReason}
					maxlength={500}
					placeholder={tr('Permintaan klien, duplikat pesanan, atau kendala operasional.')}
				></textarea>
			</label>
			{#if cancelError}
				<p class="mt-2 text-sm" style="color: var(--destructive);">{cancelError}</p>
			{/if}
			<div class="mt-4 flex justify-end gap-2">
				<button
					class="app-btn app-btn-soft"
					type="button"
					disabled={!!cancelBusyId}
					onclick={() => (cancelTarget = null)}
				>
					{tr('Batal')}
				</button>
				<button
					class="app-btn"
					style="background-color: var(--destructive); color: #fff;"
					type="button"
					disabled={!!cancelBusyId}
					onclick={() => void confirmCancel()}
				>
					{cancelBusyId ? tr('Membatalkan…') : tr('Batalkan publikasi')}
				</button>
			</div>
		</dialog>
	</div>
{/if}
