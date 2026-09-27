<script lang="ts">
	// Monitoring taut campaign — port app/admin/taut-pintar/page.tsx.
	// Pipeline estafet 4 agent per kampanye + retry per-step, statistik agent run,
	// dan bersihkan run lama (> 30 hari).
	import { onMount } from 'svelte';
	import { TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type CampaignRun = {
		campaign_id: string;
		name: string;
		brand_name: string | null;
		taut_status: string | null;
		budget_strategy: string | null;
		runs: number;
		failed_runs: number;
		updated_at: string | null;
	};
	type AgentRun = {
		id: string;
		agent: string;
		status: string;
		model: string | null;
		token_input: number | null;
		token_output: number | null;
		duration_ms: number | null;
		error: string | null;
		created_at: string | null;
		media_name: string | null;
	};
	type AgentStat = {
		agent: string;
		total: number;
		done: number;
		failed: number;
		active: number;
		token_input: number;
		token_output: number;
	};
	type Stats = {
		per_agent: AgentStat[];
		totals: { total: number; done: number; failed: number; active: number; token_input: number; token_output: number };
	};

	const AGENT_LABEL: Record<string, string> = {
		profiler: 'Profiler',
		matchmaker: 'Matchmaker',
		writer: 'Writer',
		gatekeeper: 'Gatekeeper'
	};
	const PAGE_SIZE = 10;

	let loading = $state(true);
	let error = $state('');
	let campaigns = $state<CampaignRun[]>([]);
	let stats = $state<Stats | null>(null);
	let detail = $state<AgentRun[] | null>(null);
	let selected = $state<string | null>(null);
	let page = $state(1);
	let detailLoading = $state(false);

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
		try {
			const res = await apiRequest('/v1/platform/taut/runs');
			if (!res.ok) {
				const msg = await readApiError(res, 'Permintaan platform gagal.');
				error = msg === 'Resource not found' ? 'Akses platform administrator tidak tersedia.' : msg;
				return;
			}
			const data = await res.json();
			campaigns = data.campaigns ?? [];
			stats = data.stats ?? null;
		} catch {
			error = 'Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.';
		}
	}

	async function openDetail(campaignId: string) {
		selected = campaignId;
		detail = null;
		detailLoading = true;
		try {
			const res = await apiRequest(`/v1/platform/taut/runs/${campaignId}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			detail = (await res.json()).runs ?? [];
		} finally {
			detailLoading = false;
		}
	}

	async function retryRun(runId: string) {
		error = '';
		const res = await apiRequest(`/v1/platform/taut/runs/retry/${runId}`, { method: 'POST' });
		if (!res.ok) {
			error = await readApiError(res, 'Permintaan platform gagal.');
			return;
		}
		const data = await res.json();
		if (!data.ok) {
			error = 'Retry gagal.';
			return;
		}
		if (selected) await openDetail(selected);
	}

	async function cleanupRuns() {
		if (!confirm('Bersihkan run agent yang lebih tua dari 30 hari?')) return;
		error = '';
		const res = await apiRequest('/v1/platform/taut/runs/cleanup', {
			method: 'POST',
			body: JSON.stringify({ older_than_days: 30 })
		});
		if (!res.ok) {
			error = await readApiError(res, 'Permintaan platform gagal.');
			return;
		}
		const data = await res.json();
		if (!data.ok) {
			error = 'Bersihkan gagal.';
			return;
		}
		await load();
		window.alert(`Bersihkan selesai — ${data.deleted ?? 0} run lama dihapus.`);
	}

	const visibleCampaigns = $derived(campaigns.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE));
	const selectedCampaign = $derived(campaigns.find((c) => c.campaign_id === selected) ?? null);
</script>

<section>
	<PageHeader title="Taut Pintar — Monitoring" description="Pipeline estafet 4 agent per kampanye + retry per-step." />

	<Feedback {loading} {error} loadingText="Memuat monitoring Taut Pintar…" />

	{#if stats}
		<div class="app-card mt-4 p-4">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<h2 class="text-sm font-semibold">Statistik Agent Run</h2>
				<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void cleanupRuns()}>Bersihkan run &gt; 30 hari</button>
			</div>
			<div class="mt-2 flex flex-wrap items-center gap-3">
				<Badge tone="accent" label="Total {stats.totals.total} run" />
				<Badge tone="success" label="{stats.totals.done} sukses" />
				<Badge tone="danger" label="{stats.totals.failed} gagal" />
				{#if stats.totals.active > 0}<Badge tone="warning" label="{stats.totals.active} aktif" />{/if}
				<Badge tone="neutral" label="{stats.totals.token_input + stats.totals.token_output} token" />
			</div>
			{#if stats.per_agent.length > 0}
				<div class="mt-2 space-y-1">
					{#each stats.per_agent as a (a.agent)}
						<div class="flex flex-wrap items-center justify-between gap-2 border-b py-1.5">
							<p class="font-medium">{AGENT_LABEL[a.agent] ?? a.agent}</p>
							<div class="flex flex-wrap items-center gap-2">
								<Badge tone="neutral" label="{a.total} run" />
								<Badge tone={a.failed > 0 ? 'danger' : 'success'} label="{a.done} sukses{a.failed > 0 ? ` · ${a.failed} gagal` : ''}" />
								<Badge tone="neutral" label="{a.token_input}/{a.token_output} tok" />
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}

	<div class="app-card mt-4 p-4">
		{#if campaigns.length === 0}
			<p class="py-3 text-sm text-muted-foreground">Belum ada kampanye Taut Pintar.</p>
		{:else}
			<div class="space-y-1">
				{#each visibleCampaigns as c (c.campaign_id)}
					<div class="flex flex-wrap items-center justify-between gap-2 border-b py-2">
						<div>
							<p class="font-medium">{c.name}</p>
							<p class="text-xs text-muted-foreground">{c.brand_name ?? '—'} · {c.budget_strategy ?? '—'}</p>
						</div>
						<div class="flex flex-wrap items-center gap-2">
							<Badge
								tone={c.taut_status === 'submitted' ? 'success' : c.taut_status === 'failed' ? 'danger' : 'accent'}
								label={c.taut_status ?? '—'}
							/>
							{#if c.failed_runs > 0}<Badge tone="danger" label="{c.failed_runs} gagal" />{/if}
							<Badge tone="neutral" label="{c.runs} run" />
							<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void openDetail(c.campaign_id)}>Detail</button>
						</div>
					</div>
				{/each}
			</div>
			<Pagination total={campaigns.length} limit={PAGE_SIZE} offset={(page - 1) * PAGE_SIZE} onChange={(next) => (page = next / PAGE_SIZE + 1)} />
		{/if}
	</div>

	{#if selected}
		<div class="app-card mt-4 p-4">
			<div class="flex flex-wrap items-center gap-2">
				<h2 class="text-sm font-semibold">Pipeline Agent{selectedCampaign ? ` · ${selectedCampaign.name}` : ''}</h2>
				<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => { selected = null; detail = null; }}>Tutup</button>
			</div>
			{#if detailLoading || !detail}
				<p class="mt-2 text-sm text-muted-foreground">Memuat…</p>
			{:else if detail.length === 0}
				<p class="mt-2 text-sm text-muted-foreground">Belum ada agent run.</p>
			{:else}
				<div class="mt-2 space-y-1">
					{#each detail as r (r.id)}
						<div class="flex flex-wrap items-center justify-between gap-2 border-b py-2">
							<div class="min-w-0">
								<p class="font-medium">{AGENT_LABEL[r.agent] ?? r.agent}{r.media_name && r.media_name !== '-' ? ` · ${r.media_name}` : ''}</p>
								<p class="text-xs text-muted-foreground">{r.model ?? '—'} · {r.duration_ms ?? 0}ms · {r.token_input ?? 0}/{r.token_output ?? 0} tok</p>
								{#if r.error}<p class="text-xs" style="color: var(--destructive);">{r.error.slice(0, 160)}</p>{/if}
							</div>
							<div class="flex flex-wrap items-center gap-2">
								<Badge tone={r.status === 'done' ? 'success' : r.status === 'failed' ? 'danger' : 'warning'} label={r.status} />
								{#if r.status === 'failed'}
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void retryRun(r.id)}>Retry</button>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</section>
