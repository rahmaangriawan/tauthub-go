<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Operations center — port app/admin/operations/page.tsx.
	// Investigasi alert, batch aktif, dan item yang keluar dari retry pipeline.
	import { onMount } from 'svelte';
	import { TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';

	type Summary = { alerts: string[]; running_batches: number; dead_letter_items: number };
	type Batch = {
		id: string;
		brand_owner_email: string;
		brand_name: string;
		name: string;
		status: string;
		total_items: number;
		updated_at: string;
	};
	type QueueAlert = {
		kind: string;
		failures: number;
		last_at: string;
		last_error: string;
	};
	type Dead = {
		id: string;
		batch_name: string;
		brand_owner_email: string;
		brand_name: string;
		row_number: number;
		retry_count: number;
		error_category?: string;
		error_message?: string;
		updated_at: string;
	};

	const PAGE_SIZE = 20;

	let loading = $state(true);
	let error = $state('');
	let summary = $state<Summary | null>(null);
	let batches = $state<Batch[]>([]);
	let dead = $state<Dead[]>([]);
	let queue = $state<QueueAlert[]>([]);
	let busyId = $state('');
	let tab = $state<'alerts' | 'batches' | 'dead' | 'queue'>('alerts');
	let batchOffset = $state(0);
	let deadOffset = $state(0);

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

	async function get<T>(path: string): Promise<{ ok: true; data: T } | { ok: false; error: string }> {
		try {
			const res = await apiRequest(`/v1/platform${path}`);
			if (!res.ok) {
				const detail = await readApiError(res, 'Permintaan platform gagal.');
				return {
					ok: false,
					error:
						res.status === 404 && detail === 'Resource not found'
							? 'Akses platform administrator tidak tersedia.'
							: detail
				};
			}
			return { ok: true, data: (await res.json()) as T };
		} catch {
			return { ok: false, error: tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.') };
		}
	}

	async function load() {
		error = '';
		const [summaryRes, batchRes, deadRes, queueRes] = await Promise.all([
			get<Summary>('/operations/summary'),
			get<Batch[]>('/operations/running-batches'),
			get<Dead[]>('/operations/dead-letters'),
			get<{ items: QueueAlert[] }>('/operations/queue-alerts')
		]);
		if (!summaryRes.ok) {
			error = summaryRes.error;
			return;
		}
		summary = summaryRes.data;
		batches = batchRes.ok ? batchRes.data : [];
		dead = deadRes.ok ? deadRes.data : [];
		queue = queueRes.ok ? queueRes.data.items : [];
	}

	async function retry(item: Dead) {
		busyId = item.id;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/operations/dead-letters/${item.id}/retry`, { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			await load();
		} finally {
			busyId = '';
		}
	}

	const visibleBatches = $derived(batches.slice(batchOffset, batchOffset + PAGE_SIZE));
	const visibleDead = $derived(dead.slice(deadOffset, deadOffset + PAGE_SIZE));
</script>

<section>
	<PageHeader
		title="Operations"
		description={tr("Investigasi alert, batch aktif, dan item yang keluar dari retry pipeline.")}
	>
		<Badge tone={(summary?.alerts.length ?? 0) > 0 ? 'warning' : 'success'} label="{summary?.alerts.length ?? 0} alerts" />
	</PageHeader>

	<Feedback {loading} {error} loadingText="Memuat platform operations…" />

	{#if !summary && error.includes('Akses platform')}
		<div class="app-card mt-4 px-4 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke platform operations.")}</p>
		</div>
	{:else if summary}
		<div class="mt-4 flex flex-wrap gap-2">
			<button
				class="app-btn {tab === 'alerts' ? 'app-btn-primary' : 'app-btn-soft'} !min-h-8 !px-3 !text-xs"
				type="button"
				onclick={() => (tab = 'alerts')}
			>Alerts ({summary.alerts.length})</button>
			<button
				class="app-btn {tab === 'batches' ? 'app-btn-primary' : 'app-btn-soft'} !min-h-8 !px-3 !text-xs"
				type="button"
				onclick={() => (tab = 'batches')}
			>Running batches ({batches.length})</button>
			<button
				class="app-btn {tab === 'dead' ? 'app-btn-primary' : 'app-btn-soft'} !min-h-8 !px-3 !text-xs"
				type="button"
				onclick={() => (tab = 'dead')}
			>Dead letters ({dead.length})</button>
			<button
				class="app-btn {tab === 'queue' ? 'app-btn-primary' : 'app-btn-soft'} !min-h-8 !px-3 !text-xs"
				type="button"
				onclick={() => (tab = 'queue')}
			>Queue gagal ({queue.reduce((n, q) => n + q.failures, 0)})</button>
		</div>

		{#if tab === 'alerts'}
			<div class="mt-3 space-y-3">
				{#if summary.alerts.length}
					{#each summary.alerts as alert (alert)}
						<div class="app-card flex items-start gap-2 px-4 py-3" style="border-color: var(--warning);">
							<TriangleAlert size={16} class="mt-0.5 shrink-0" style="color: var(--warning);" />
							<p class="text-sm">{alert}</p>
						</div>
					{/each}
				{:else}
					<div class="app-card px-4 py-12 text-center">
						<h2 class="text-base font-semibold">{tr("Tidak ada alert aktif")}</h2>
						<p class="mt-1 text-sm text-muted-foreground">{tr("Control plane tidak mendeteksi kondisi yang memerlukan tindakan.")}</p>
					</div>
				{/if}
			</div>
		{/if}

		{#if tab === 'batches'}
			<div class="app-card mt-3 overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 880px;">
					<thead>
						<tr class="text-xs text-muted-foreground">
							<th class="px-4 py-2.5 text-left font-medium">Batch</th>
							<th class="px-4 py-2.5 text-left font-medium">Owner</th>
							<th class="px-4 py-2.5 text-left font-medium">Brand</th>
							<th class="px-4 py-2.5 text-left font-medium">Items</th>
							<th class="px-4 py-2.5 text-left font-medium">Updated</th>
						</tr>
					</thead>
					<tbody>
						{#each visibleBatches as item (item.id)}
							<tr class="border-t">
								<td class="px-4 py-2.5 font-medium">{item.name}</td>
								<td class="px-4 py-2.5">{item.brand_owner_email}</td>
								<td class="px-4 py-2.5">{item.brand_name}</td>
								<td class="px-4 py-2.5">{item.total_items}</td>
								<td class="px-4 py-2.5">{fmtDateTime(item.updated_at)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
				{#if batches.length === 0}
					<p class="px-4 py-10 text-center text-sm text-muted-foreground">{tr("Tidak ada batch yang sedang berjalan.")}</p>
				{/if}
			</div>
			<Pagination total={batches.length} limit={PAGE_SIZE} offset={batchOffset} onChange={(v) => (batchOffset = v)} />
		{/if}

		{#if tab === 'dead'}
			<div class="app-card mt-3 overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 980px;">
					<thead>
						<tr class="text-xs text-muted-foreground">
							<th class="px-4 py-2.5 text-left font-medium">Item</th>
							<th class="px-4 py-2.5 text-left font-medium">Owner</th>
							<th class="px-4 py-2.5 text-left font-medium">Error</th>
							<th class="px-4 py-2.5 text-left font-medium">Retry</th>
							<th class="px-4 py-2.5 text-right font-medium"></th>
						</tr>
					</thead>
					<tbody>
						{#each visibleDead as item (item.id)}
							<tr class="border-t">
								<td class="px-4 py-2.5">
									<span class="font-medium">{item.batch_name} · row {item.row_number}</span>
									<span class="block text-xs text-muted-foreground">{item.brand_name}</span>
								</td>
								<td class="px-4 py-2.5">{item.brand_owner_email}</td>
								<td class="px-4 py-2.5">
									<Badge tone="danger" label={item.error_category || 'unknown'} />
									{#if item.error_message}
										<p class="mt-1 text-xs text-muted-foreground">{item.error_message}</p>
									{/if}
								</td>
								<td class="px-4 py-2.5">{item.retry_count}/3</td>
								<td class="px-4 py-2.5 text-right">
									<button
										class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
										type="button"
										disabled={busyId === item.id || item.retry_count >= 3}
										onclick={() => void retry(item)}
									>{busyId === item.id ? 'Queueing...' : 'Retry item'}</button>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
				{#if dead.length === 0}
					<p class="px-4 py-10 text-center text-sm text-muted-foreground">Dead-letter queue kosong.</p>
				{/if}
			</div>
			<Pagination total={dead.length} limit={PAGE_SIZE} offset={deadOffset} onChange={(v) => (deadOffset = v)} />
		{/if}

		{#if tab === 'queue'}
			<div class="app-card mt-3 overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 760px;">
					<thead>
						<tr class="text-xs text-muted-foreground">
							<th class="px-4 py-2.5 text-left font-medium">{tr('Jenis Job')}</th>
							<th class="px-4 py-2.5 text-left font-medium">{tr('Kegagalan (24 jam)')}</th>
							<th class="px-4 py-2.5 text-left font-medium">{tr('Terakhir')}</th>
							<th class="px-4 py-2.5 text-left font-medium">{tr('Error terakhir')}</th>
						</tr>
					</thead>
					<tbody>
						{#each queue as item (item.kind)}
							<tr class="border-t">
								<td class="px-4 py-2.5 font-medium">{item.kind}</td>
								<td class="px-4 py-2.5 tabular-nums">{item.failures}</td>
								<td class="px-4 py-2.5 whitespace-nowrap">{fmtDateTime(item.last_at)}</td>
								<td class="max-w-[520px] px-4 py-2.5"><span class="block truncate" title={item.last_error}>{item.last_error || '—'}</span></td>
							</tr>
						{/each}
					</tbody>
				</table>
				{#if queue.length === 0}
					<p class="px-4 py-10 text-center text-sm text-muted-foreground">{tr('Tidak ada kegagalan queue dalam 24 jam terakhir.')}</p>
				{/if}
			</div>
		{/if}
	{/if}
</section>
