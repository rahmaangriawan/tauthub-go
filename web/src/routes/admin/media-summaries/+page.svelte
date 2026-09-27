<script lang="ts">
	// Monitoring Kesimpulan Website — daftar event tracking (queued/success/failed)
	// beserta metrik ringkas. Port apps/web/src/app/admin/media-summaries/page.tsx.
	import { onMount } from 'svelte';
	import { Search } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const LIMIT = 50;

	interface SummaryEvent {
		id: string;
		media_id: string;
		media_name: string | null;
		website_url: string | null;
		status: string;
		pages_tried: string | Array<{ url: string; ok: boolean; chars: number; note?: string }>;
		error_message: string | null;
		attempt: number;
		created_at: string;
	}
	interface Metrics {
		queued_events: number;
		success: number;
		failed: number;
		jobs_pending: number;
		jobs_running: number;
		jobs_failed: number;
	}
	type Page = { total: number; items: SummaryEvent[]; metrics?: Metrics };

	function parsePages(raw: SummaryEvent['pages_tried']): Array<{ url: string; ok: boolean; chars: number; note?: string }> {
		let parsed: unknown = raw;
		if (typeof raw === 'string') {
			try {
				parsed = JSON.parse(raw);
			} catch {
				return [];
			}
		}
		if (!Array.isArray(parsed)) return [];
		return parsed.filter(
			(p) => p && typeof p === 'object' && typeof (p as { url?: unknown }).url === 'string'
		) as Array<{ url: string; ok: boolean; chars: number; note?: string }>;
	}

	function statusLabel(status: string, pages: Array<{ ok: boolean }>) {
		if (status === 'success') {
			// Semua halaman gagal ditarik → kesimpulan dibuat dari data mitra (fallback).
			if (pages.length > 0 && pages.every((p) => !p.ok)) return 'Sukses · Data Mitra';
			return 'Sukses';
		}
		if (status === 'failed') return 'Gagal';
		return 'Menunggu Antrean';
	}

	function statusBadgeTone(status: string, pages: Array<{ ok: boolean }>): 'neutral' | 'success' | 'warning' | 'danger' {
		if (status === 'success') return pages.length > 0 && pages.every((p) => !p.ok) ? 'warning' : 'success';
		if (status === 'failed') return 'danger';
		return 'neutral';
	}

	let data = $state<Page>({ total: 0, items: [] });
	let offset = $state(0);
	let loading = $state(true);
	let error = $state('');
	let search = $state('');
	let status = $state('');
	let busyId = $state<string | null>(null);
	let notice = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load(0, '', '');
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load(nextOffset: number, nextStatus: string, nextSearch: string) {
		loading = true;
		error = '';
		const query = new URLSearchParams({ offset: String(nextOffset), limit: String(LIMIT) });
		if (nextStatus) query.set('status', nextStatus);
		if (nextSearch) query.set('search', nextSearch);
		try {
			const res = await apiRequest(`/v1/platform/media-summaries?${query}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const page = await res.json();
			data = { total: page.total ?? 0, items: page.items ?? [], metrics: page.metrics };
			offset = nextOffset;
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	function submitFilters(event: SubmitEvent) {
		event.preventDefault();
		notice = '';
		void load(0, status, search.trim());
	}

	async function regenerate(mediaId: string) {
		busyId = mediaId;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/platform/media-summaries/${mediaId}/regenerate`, { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			notice = 'Regenerasi dijadwalkan. Kesimpulan akan diperbarui lewat antrean global AI.';
			await load(offset, status, search.trim());
		} finally {
			busyId = null;
		}
	}

	const metrics = $derived([
		{ label: 'Sukses', value: data.metrics?.success ?? 0, color: 'var(--success)' },
		{
			label: 'Gagal',
			value: data.metrics?.failed ?? 0,
			color: (data.metrics?.failed ?? 0) > 0 ? 'var(--destructive)' : 'var(--muted-foreground)'
		},
		{ label: 'Menunggu Antrean', value: data.metrics?.queued_events ?? 0, color: undefined },
		{
			label: 'Job Aktif',
			value: (data.metrics?.jobs_pending ?? 0) + (data.metrics?.jobs_running ?? 0),
			color: undefined
		},
		{
			label: 'Job Gagal Permanen',
			value: data.metrics?.jobs_failed ?? 0,
			color: (data.metrics?.jobs_failed ?? 0) > 0 ? 'var(--destructive)' : 'var(--muted-foreground)'
		}
	]);
</script>

{#snippet statusCell(item: SummaryEvent)}
	{@const pages = parsePages(item.pages_tried)}
	<Badge tone={statusBadgeTone(item.status, pages)} label={statusLabel(item.status, pages)} />
{/snippet}

{#snippet mediaCell(item: SummaryEvent)}
	<div class="max-w-[320px]">
		<p class="font-medium">{item.media_name ?? item.media_id.slice(0, 8)}</p>
		{#if item.website_url}
			<p class="text-xs text-muted-foreground">
				{item.website_url.replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '')}
			</p>
		{/if}
		<p class="text-xs text-muted-foreground">{fmtDateTime(item.created_at)}</p>
	</div>
{/snippet}

{#snippet pagesCell(item: SummaryEvent)}
	{@const pages = parsePages(item.pages_tried)}
	{@const okPages = pages.filter((p) => p.ok)}
	<div class="max-w-[300px]">
		<p class="text-xs" style={pages.length > 0 && okPages.length === 0 ? 'color: var(--destructive);' : undefined}>
			{pages.length > 0 ? `${okPages.length}/${pages.length} halaman` : '—'}
		</p>
		{#if okPages.length > 0}
			<p
				class="max-w-[280px] truncate text-xs text-muted-foreground"
				title={okPages.map((p) => p.url.replace(/^https?:\/\/(www\.)?/, '')).join(', ')}
			>
				{okPages.map((p) => p.url.replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '')).join(', ')}
			</p>
		{/if}
		{#if pages.length > 0 && okPages.length === 0}
			<p
				class="max-w-[280px] truncate text-xs"
				style="color: var(--destructive);"
				title={pages[0]?.note ?? 'Web tidak terjangkau'}
			>
				{pages[0]?.note ?? 'Web tidak terjangkau'}
			</p>
		{/if}
	</div>
{/snippet}

{#snippet errorCell(item: SummaryEvent)}
	<div class="max-w-[240px]">
		{#if item.error_message}
			<p class="truncate text-xs" style="color: var(--destructive);" title={item.error_message}>{item.error_message}</p>
		{:else}
			<p class="text-xs text-muted-foreground">—</p>
		{/if}
		<p class="text-xs text-muted-foreground">{item.attempt} percobaan</p>
	</div>
{/snippet}

{#snippet regenCell(item: SummaryEvent)}
	<div class="text-right">
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			disabled={busyId !== null}
			title="Hapus kesimpulan lama dan antrekan penerbitan ulang"
			onclick={() => void regenerate(item.media_id)}
			>{busyId === item.media_id ? 'Mengantrekan…' : 'Generate ulang'}</button
		>
	</div>
{/snippet}

<section>
	<PageHeader
		title="Monitoring Kesimpulan Website"
		description="Tracking AI Tauthub saat menarik data web untuk profil Kesimpulan. Sukses dan gagal semuanya tercatat di sini."
	/>

	<Feedback {loading} {error} {notice} />

	<div class="mt-4 flex flex-wrap gap-3">
		{#each metrics as m (m.label)}
			<div class="app-card min-w-[140px] flex-1 p-3">
				<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{m.label}</p>
				<p class="text-2xl font-bold" style={m.color ? `color: ${m.color};` : undefined}>{m.value}</p>
			</div>
		{/each}
	</div>

	<form class="mt-4 flex flex-wrap items-center gap-2" onsubmit={submitFilters}>
		<label class="app-input flex w-[280px] items-center gap-2 !py-1.5">
			<Search size={14} class="text-muted-foreground" />
			<input class="w-full bg-transparent text-sm outline-none" placeholder="Cari nama media atau website…" bind:value={search} />
		</label>
		<input class="app-input" style="width: 220px;" placeholder="Sukses / gagal / queued" bind:value={status} />
		<button type="submit" class="app-btn app-btn-soft">Terapkan</button>
		<button
			type="button"
			class="app-btn app-btn-soft"
			onclick={() => {
				search = '';
				status = '';
				notice = '';
				void load(0, '', '');
			}}>Reset</button
		>
	</form>

	<DataTable
		columns={[
			{ key: 'media', label: 'Media', render: mediaCell },
			{ key: 'status', label: 'Status', render: statusCell },
			{ key: 'pages', label: 'Halaman Ditarik', render: pagesCell },
			{ key: 'error', label: 'Error', render: errorCell },
			{ key: 'actions', label: 'Aksi', align: 'right' as const, render: regenCell }
		]}
		rows={data.items}
		rowKey={(item: SummaryEvent) => item.id}
		{loading}
		empty="Belum ada event tracking yang cocok."
		minWidth="1000px"
	/>

	<Pagination total={data.total} limit={LIMIT} {offset} onChange={(next) => void load(next, status, search.trim())} />

	<p class="mt-4 text-xs text-muted-foreground">
		Kesimpulan dibuat otomatis oleh AI lewat antrean global. Gagal diproses otomatis dicoba ulang oleh sistem; job yang gagal
		permanen terlihat pada kartu Job Gagal Permanen.
	</p>
</section>
