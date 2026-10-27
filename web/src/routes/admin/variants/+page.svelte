<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Variant Batches — monitoring multi-artikel Tauthub: satu bahan/artikel → N
	// artikel per media (sudut pandang diacak, diproses worker).
	// Port apps/web/src/app/admin/variants/page.tsx.
	import { onMount } from 'svelte';
	import { ChevronDown, ChevronRight, Download, Search } from '@lucide/svelte';
	import { goto } from '$app/navigation';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const PAGE_SIZE = 20;

	type VariantRow = {
		id: string;
		media_name: string | null;
		angle_label: string;
		status: string;
		document_title: string | null;
		publication_id: string | null;
		publication_status: string | null;
		error_message: string | null;
	};

	type BatchRow = {
		batch_id: string;
		mode: string;
		seed: number;
		status: string;
		created_at: string;
		completed_at: string | null;
		brand_name: string | null;
		brand_public_id: string | null;
		user_email: string | null;
		variant_counts: { total: number; done: number; failed: number };
		variants: VariantRow[];
	};

	const STATUS_META: Record<string, { label: string; tone: 'neutral' | 'accent' | 'success' | 'warning' | 'danger' }> = $derived({
		queued: { label: 'Diantrekan', tone: 'accent' },
		processing: { label: 'Diproses', tone: 'warning' },
		completed: { label: tr('Selesai'), tone: 'success' },
		partial: { label: 'Sebagian gagal', tone: 'warning' },
		failed: { label: tr('Gagal'), tone: 'danger' }
	});

	const VARIANT_STATUS_TONE: Record<string, 'neutral' | 'success' | 'warning' | 'danger'> = {
		queued: 'neutral',
		processing: 'warning',
		completed: 'success',
		failed: 'danger'
	};

	let rows = $state<BatchRow[]>([]);
	let filter = $state('');
	let query = $state('');
	let expanded = $state<Set<string>>(new Set());
	let error = $state('');
	let loading = $state(true);
	let page = $state(1);
	let total = $state(0);
	let exporting = $state(false);
	let retrying = $state('');

	const filters = ['', 'queued', 'processing', 'completed', 'partial', 'failed'];

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
			const res = await apiRequest(`/v1/platform/operations/variant-batches?${params}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const data = await res.json();
			rows = data.batches ?? [];
			total = data.total ?? 0;
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	function toggle(id: string) {
		const next = new Set(expanded);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		expanded = next;
	}

	async function exportCsv() {
		exporting = true;
		error = '';
		try {
			const params = new URLSearchParams({ format: 'csv' });
			if (filter) params.set('status', filter);
			const response = await apiRequest(`/v1/platform/operations/variant-batches?${params}`);
			if (!response.ok) throw new Error(await response.text());
			const blob = await response.blob();
			const a = document.createElement('a');
			a.href = URL.createObjectURL(blob);
			a.download = `tauthub-variant-batches-${new Date().toISOString().slice(0, 10)}.csv`;
			a.click();
			URL.revokeObjectURL(a.href);
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			exporting = false;
		}
	}

	async function retryBatch(batchId: string) {
		retrying = batchId;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/operations/variant-batches/${batchId}/retry`, {
				method: 'POST'
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const data = await res.json();
			if (data.ok) void load();
		} finally {
			retrying = '';
		}
	}

	const expandedBatches = $derived(rows.filter((b) => expanded.has(b.batch_id)));
	const showAccess = $derived(rows.length === 0 && error.includes('Akses platform'));
</script>

{#snippet brandCell(b: BatchRow)}
	{@const open = expanded.has(b.batch_id)}
	<div class="flex items-center gap-2">
		<button
			type="button"
			class="p-0 text-muted-foreground"
			style="background: none; border: none; cursor: pointer;"
			aria-label={open ? 'Tutup rincian' : 'Perluas rincian'}
			onclick={() => toggle(b.batch_id)}
		>
			{#if open}<ChevronDown size={14} />{:else}<ChevronRight size={14} />{/if}
		</button>
		<a
			href={`/admin/variants/${b.batch_id}`}
			style="color: var(--primary); text-decoration: none;"
			onclick={(e) => {
				e.preventDefault();
				void goto(`/admin/variants/${b.batch_id}`);
			}}
		>
			<div class="min-w-0">
				<p class="font-medium" style="color: inherit;">{b.brand_name ?? '—'}</p>
				{#if b.user_email}<p class="text-xs text-muted-foreground">{b.user_email}</p>{/if}
			</div>
		</a>
	</div>
{/snippet}

{#snippet modeCell(b: BatchRow)}
	<Badge tone={b.mode === 'generate' ? 'accent' : 'neutral'} label={b.mode === 'generate' ? 'Dibuatkan' : 'Parafrase'} />
{/snippet}

{#snippet seedCell(b: BatchRow)}
	<span class="text-xs text-muted-foreground">{b.seed}</span>
{/snippet}

{#snippet variantCountCell(b: BatchRow)}
	<div>
		<p>{b.variant_counts.done}/{b.variant_counts.total} jadi</p>
		{#if b.variant_counts.failed > 0}
			<p class="text-xs" style="color: var(--destructive);">{b.variant_counts.failed} gagal</p>
		{/if}
	</div>
{/snippet}

{#snippet batchStatusCell(b: BatchRow)}
	{@const meta = STATUS_META[b.status] ?? { label: b.status, tone: 'neutral' as const }}
	<Badge tone={meta.tone} label={meta.label} />
{/snippet}

{#snippet batchTimeCell(b: BatchRow)}
	<div class="text-xs text-muted-foreground">
		<p>{fmtDateTime(b.created_at)}</p>
		{#if b.completed_at}<p>selesai {fmtDateTime(b.completed_at)}</p>{/if}
	</div>
{/snippet}

<section>
	<ExplorerFrame parents={[{ label: tr('Publikasi'), href: '/admin/publications' }]} crumb={tr('Varian')} path='/admin/variants'>
	<PageHeader
		title="Variant Batches"
		description={tr("Monitoring multi-artikel Tauthub: satu bahan/artikel → N artikel per media (sudut pandang diacak, diproses worker). Klik baris untuk audit sudut pandang per media.")}
	>
		<Badge tone="accent" value={`${total} batch`} />
	</PageHeader>

	<Feedback {loading} {error} />

	<div class="mt-4 flex flex-wrap items-center justify-between gap-2">
		<label class="app-input flex min-w-[240px] max-w-[380px] flex-1 items-center gap-2 !py-1.5">
			<Search size={16} class="text-muted-foreground" />
			<input
				class="w-full bg-transparent text-sm outline-none"
				placeholder={tr("Cari brand, email, media, sudut pandang, atau ID batch…")}
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
				}}>{f === '' ? tr('Semua') : (STATUS_META[f]?.label ?? f)}</button
			>
		{/each}
	</div>

	{#if showAccess}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke data batch variant.")}</p>
		</div>
	{:else}
		<DataTable
			columns={[
				{ key: 'brand', label: 'Brand', render: brandCell },
				{ key: 'mode', label: 'Mode', render: modeCell },
				{ key: 'seed', label: 'Seed', render: seedCell },
				{ key: 'variants', label: 'Varian', render: variantCountCell },
				{ key: 'status', label: 'Status', render: batchStatusCell },
				{ key: 'time', label: 'Waktu', render: batchTimeCell }
			]}
			rows={rows}
			rowKey={(b: BatchRow) => b.batch_id}
			{loading}
			empty={tr("Tidak ada batch variant untuk filter ini.")}
			minWidth="1000px"
		/>

		{#each expandedBatches as b (b.batch_id)}
			<div class="app-card mt-3 p-4">
				<div class="mb-2 flex items-center justify-between gap-2">
					<p class="font-bold">Batch {b.batch_id.slice(0, 8)} · audit sudut pandang</p>
					{#if ['failed', 'partial'].includes(b.status)}
						<button
							class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
							style="color: var(--warning);"
							disabled={retrying === b.batch_id}
							onclick={() => void retryBatch(b.batch_id)}
							>{retrying === b.batch_id ? 'Mengantrekan…' : `Retry ${b.variant_counts.failed} gagal`}</button
						>
					{/if}
				</div>
				<div class="overflow-x-auto">
					<table class="w-full text-sm" style="min-width: 900px;">
						<thead>
							<tr class="text-xs text-muted-foreground">
								<th class="px-4 py-2.5 text-left font-medium">{tr("Media")}</th>
								<th class="px-4 py-2.5 text-left font-medium">Sudut pandang</th>
								<th class="px-4 py-2.5 text-left font-medium">Artikel</th>
								<th class="px-4 py-2.5 text-left font-medium">{tr("Publikasi")}</th>
								<th class="px-4 py-2.5 text-left font-medium">Status</th>
							</tr>
						</thead>
						<tbody>
							{#if b.variants.length === 0}
								<tr
									><td colspan="5" class="px-4 py-2.5 text-xs text-muted-foreground"
										>{tr("Belum ada varian tercatat.")}</td
									></tr
								>
							{/if}
							{#each b.variants as v (v.id)}
								<tr class="border-t">
									<td class="px-4 py-2.5">{v.media_name ?? '—'}</td>
									<td class="px-4 py-2.5">{v.angle_label}</td>
									<td class="px-4 py-2.5 text-xs text-muted-foreground">{v.document_title ?? '—'}</td>
									<td class="px-4 py-2.5">
										{#if v.publication_id}
											<a
												href={`/admin/publications/${v.publication_id}`}
												class="text-xs"
												style="color: var(--primary);">{v.publication_status ?? 'lihat'}</a
											>
										{:else}
											<span class="text-xs text-muted-foreground">—</span>
										{/if}
									</td>
									<td class="px-4 py-2.5">
										<Badge tone={VARIANT_STATUS_TONE[v.status] ?? 'neutral'} value={v.status} />
										{#if v.error_message}
											<p class="mt-1 text-xs" style="color: var(--destructive);">{v.error_message}</p>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/each}

		<div class="mt-3 flex flex-wrap items-center justify-between gap-2">
			<p class="text-xs text-muted-foreground">
				{total} batch · halaman {page} {tr("dari")} {Math.max(1, Math.ceil(total / PAGE_SIZE))}
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
