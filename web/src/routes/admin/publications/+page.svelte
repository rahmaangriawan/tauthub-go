<script lang="ts">
	// Publikasi admin — semua publikasi seluruh brand (staff visibility).
	// Port apps/web/src/app/admin/publications/page.tsx.
	import { onMount } from 'svelte';
	import { Download, Megaphone, Search } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import { fmtDateTime } from '$lib/admin-format';
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
		error_code: string | null;
		variant: { angle_label: string; mode: string | null } | null;
	};

	const STATUS_TONE: Record<string, 'neutral' | 'accent' | 'success' | 'warning' | 'danger'> = {
		published: 'success',
		pending: 'warning',
		waiting_partner: 'warning',
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
		'queued',
		'processing',
		'published',
		'failed',
		'retrying',
		'partner_rejected',
		'cancelled'
	];

	let rows = $state<Row[]>([]);
	let filter = $state('');
	let query = $state('');
	let error = $state('');
	let loading = $state(true);
	let exporting = $state(false);
	let page = $state(1);
	let total = $state(0);

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
		new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 }).format(
			Number(n ?? 0)
		);
	const showAccess = $derived(rows.length === 0 && error.includes('Akses platform'));
</script>

{#snippet articleCell(row: Row)}
	<a href={`/admin/publications/${row.id}`} class="block" style="color: var(--primary); text-decoration: none;">
		<div class="flex items-center gap-2">
			<Megaphone size={16} />
			<div class="min-w-0">
				<div class="flex items-center gap-2">
					<p class="truncate font-medium" style="color: inherit;">{row.document_title ?? row.id.slice(0, 8)}</p>
					{#if row.variant}<Badge tone="accent" label="Variant" />{/if}
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
		<Badge tone={STATUS_TONE[row.status] ?? 'neutral'} value={row.status} />
		{#if row.error_code}
			<p class="mt-1 text-xs" style="color: var(--destructive);">{row.error_code}</p>
		{/if}
	</div>
{/snippet}

{#snippet createdAtCell(row: Row)}
	<span class="text-xs text-muted-foreground">{fmtDateTime(row.created_at)}</span>
{/snippet}

<section class="mx-auto max-w-7xl">
	<PageHeader
		title="Publikasi"
		description="Semua publikasi seluruh brand (staff visibility). Klik baris untuk detail lengkap: riwayat attempts, job delivery, dan payload."
	>
		<Badge tone="accent" value={`${total} publikasi`} />
	</PageHeader>

	<Feedback {loading} {error} />

	<div class="mt-4 flex flex-wrap items-center gap-2">
		<label class="app-input flex min-w-[240px] max-w-[360px] flex-1 items-center gap-2 !py-1.5">
			<Search size={16} class="text-muted-foreground" />
			<input
				class="w-full bg-transparent text-sm outline-none"
				placeholder="Cari artikel, brand, media, campaign, partner, atau ID…"
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
				}}>{f === '' ? 'Semua' : f}</button
			>
		{/each}
	</div>

	{#if showAccess}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke data publikasi.</p>
		</div>
	{:else}
		<DataTable
			columns={[
				{ key: 'article', label: 'Artikel', render: articleCell },
				{ key: 'brand', label: 'Brand', render: brandCell },
				{ key: 'media', label: 'Media', render: mediaCell },
				{ key: 'price', label: 'Harga', render: priceCell },
				{ key: 'status', label: 'Status', render: pubStatusCell },
				{ key: 'created', label: 'Waktu', render: createdAtCell }
			]}
			rows={rows}
			rowKey={(row: Row) => row.id}
			{loading}
			empty="Tidak ada publikasi untuk filter ini."
			minWidth="1100px"
		/>

		<div class="mt-3 flex flex-wrap items-center justify-between gap-2">
			<p class="text-xs text-muted-foreground">
				{total} publikasi · halaman {page} dari {Math.max(1, Math.ceil(total / PAGE_SIZE))}
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
</section>
