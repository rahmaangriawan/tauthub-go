<script lang="ts">
	// Content Safety — hasil moderasi berlapis. review_required menunggu keputusan admin.
	// Port apps/web/src/app/admin/moderations/page.tsx.
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type Audit = {
		id: string;
		entity_type: string;
		entity_id: string;
		brand_id: string | null;
		brand_name: string | null;
		document_title: string | null;
		status: string;
		risk_score: number;
		category: string;
		reason_code: string;
		signals: unknown;
		moderation_version: string;
		ai_called: boolean;
		reviewed_by: string | null;
		review_decision: string | null;
		created_at: string;
	};

	let rows = $state<Audit[]>([]);
	let filter = $state('review_required');
	let error = $state('');
	let loading = $state(true);
	let stats = $state<{
		total: number;
		safe: number;
		review_required: number;
		blocked: number;
		ai_called: number;
		rule_only: number;
	} | null>(null);

	const filters = ['review_required', 'blocked', 'safe', ''];

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load(filter);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load(status: string) {
		loading = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/moderations?status=${status}&limit=50`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const data = await res.json();
			rows = data.audits ?? [];
			stats = data.stats ?? null;
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	async function review(id: string, decision: 'approved' | 'rejected') {
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/moderations/${id}/review`, {
				method: 'POST',
				body: JSON.stringify({ decision })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			await load(filter);
		} catch {
			error = 'Permintaan gagal.';
		}
	}

	const signalList = (signals: unknown) => (Array.isArray(signals) ? (signals as string[]) : []);
</script>

{#snippet titleCell(r: Audit)}
	<div>
		<p class="font-medium">{r.document_title ?? r.entity_id.slice(0, 8)}</p>
		<p class="text-xs text-muted-foreground">{r.reason_code} · {signalList(r.signals).slice(0, 4).join(', ')}</p>
	</div>
{/snippet}

{#snippet brandCell(r: Audit)}
	<span>{r.brand_name ?? r.brand_id ?? '—'}</span>
{/snippet}

{#snippet statusCell(r: Audit)}
	<div class="flex items-center gap-1">
		<Badge
			tone={r.status === 'blocked' ? 'danger' : r.status === 'review_required' ? 'warning' : 'success'}
			value={r.status}
		/>
		{#if r.ai_called}<Badge tone="accent" label="AI" />{/if}
	</div>
{/snippet}

{#snippet scoreCell(r: Audit)}
	<span class="tabular-nums">{r.risk_score}</span>
{/snippet}

{#snippet moderationActions(r: Audit)}
	{#if r.review_decision}
		<Badge tone={r.review_decision === 'approved' ? 'success' : 'danger'} value={r.review_decision} />
	{:else if r.status === 'review_required'}
		<div class="flex gap-1">
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--success);"
				onclick={() => void review(r.id, 'approved')}>Approve</button
			>
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--destructive);"
				onclick={() => void review(r.id, 'rejected')}>Reject</button
			>
		</div>
	{:else}
		<span class="text-muted-foreground">—</span>
	{/if}
{/snippet}

<section class="mx-auto max-w-7xl">
	<PageHeader title="Content Safety" description="Hasil moderasi berlapis — review_required menunggu keputusan admin.">
		<Badge tone="accent" value={stats ? `${stats.total} total · ${stats.ai_called} AI` : ''} />
	</PageHeader>

	<Feedback {loading} {error} />

	{#if stats}
		<div class="mt-3 flex flex-wrap gap-2">
			<Badge tone="success" label={`SAFE ${stats.safe}`} />
			<Badge tone="warning" label={`REVIEW ${stats.review_required}`} />
			<Badge tone="danger" label={`BLOCKED ${stats.blocked}`} />
			<Badge tone="neutral" label={`AI ${stats.ai_called} · Rule ${stats.rule_only}`} />
		</div>
	{/if}

	<div class="mt-3 flex flex-wrap gap-2">
		{#each filters as f (f)}
			<button
				type="button"
				class="app-btn !min-h-8 !px-3 !text-xs"
				class:app-btn-primary={filter === f}
				class:app-btn-soft={filter !== f}
				onclick={() => {
					filter = f;
					void load(f);
				}}
			>
				{f === '' ? 'Semua' : f}
			</button>
		{/each}
	</div>

	<DataTable
		columns={[
			{ key: 'title', label: 'Judul', render: titleCell },
			{ key: 'brand', label: 'Brand', render: brandCell },
			{ key: 'status', label: 'Status', render: statusCell },
			{ key: 'score', label: 'Skor', render: scoreCell },
			{ key: 'actions', label: 'Aksi', render: moderationActions }
		]}
		rows={rows}
		rowKey={(r: Audit) => r.id}
		{loading}
		empty="Tidak ada data untuk filter ini."
		minWidth="900px"
	/>
</section>
