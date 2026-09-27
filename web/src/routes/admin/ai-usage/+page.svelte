<script lang="ts">
	// Usage AI — port app/admin/ai-usage/page.tsx.
	// Log pemakaian token AI seluruh step Taut Pintar (taut_agent_runs):
	// filter rentang/step/status + pencarian kampanye, ringkasan, breakdown, dan log per panggilan.
	import { onMount } from 'svelte';
	import { RefreshCw, Search } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type AgentUsage = {
		agent: string;
		calls: number;
		failed_calls: number;
		token_input: number;
		token_output: number;
		total_tokens: number;
		avg_duration_ms: number;
	};
	type UsageRun = {
		id: string;
		agent: string;
		status: string;
		model: string | null;
		token_input: number | null;
		token_output: number | null;
		duration_ms: number | null;
		error: string | null;
		created_at: string;
		campaign_name: string | null;
		media_name: string | null;
	};
	type AiUsageResponse = {
		summary: {
			calls: number;
			done_calls: number;
			failed_calls: number;
			token_input: number;
			token_output: number;
			total_tokens: number;
			avg_duration_ms: number;
		};
		by_agent: AgentUsage[];
		total: number;
		runs: UsageRun[];
	};

	const AGENT_META: Record<string, { label: string; tone: 'accent' | 'success' | 'warning' }> = {
		profiler: { label: 'Profiler', tone: 'accent' },
		matchmaker: { label: 'Matchmaker', tone: 'accent' },
		writer: { label: 'Writer', tone: 'success' },
		gatekeeper: { label: 'Gatekeeper', tone: 'warning' }
	};

	const PAGE_SIZE = 20;

	let loading = $state(true);
	let error = $state('');
	let data = $state<AiUsageResponse | null>(null);
	let days = $state('30');
	let agent = $state('all');
	let status = $state('all');
	let searchInput = $state('');
	let search = $state('');
	let page = $state(1);

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
		const params = new URLSearchParams({ days, page: String(page), pageSize: String(PAGE_SIZE) });
		if (agent !== 'all') params.set('agent', agent);
		if (status !== 'all') params.set('status', status);
		if (search.trim()) params.set('search', search.trim());
		try {
			const res = await apiRequest(`/v1/platform/taut/ai-usage?${params.toString()}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			data = await res.json();
		} catch {
			error = 'Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.';
		} finally {
			loading = false;
		}
	}

	function submitSearch(e: SubmitEvent) {
		e.preventDefault();
		page = 1;
		search = searchInput;
		void load();
	}

	const fmtInt = (n: number | null | undefined) => (n ?? 0).toLocaleString('id-ID');
	const fmtDur = (ms: number | null | undefined) => {
		const v = Math.round(ms ?? 0);
		if (!v) return '-';
		if (v < 60_000) return `${(v / 1000).toFixed(1)} dtk`;
		return `${(v / 60_000).toFixed(1)} mnt`;
	};
	const fmtTime = (iso: string | null) =>
		iso ? new Date(iso).toLocaleString('id-ID', { dateStyle: 'short', timeStyle: 'short' }) : '-';

	const totalTokens = $derived(data?.summary.total_tokens ?? 0);
</script>

<section class="mx-auto max-w-7xl">
	<PageHeader
		title="AI Token Usage"
		description="Pemakaian token AI seluruh step Taut Pintar — Profiler, Matchmaker, Writer, Gatekeeper — dari log eksekusi taut_agent_runs."
	/>

	<Feedback {loading} {error} loadingText="Memuat data usage…" />

	{#if loading && !data}
		<p class="mt-6 text-sm text-muted-foreground">Memuat…</p>
	{:else}
		<div class="mt-4 flex flex-wrap items-center gap-3">
			<select class="app-input w-auto min-w-[130px]" aria-label="Rentang waktu" bind:value={days} onchange={() => { page = 1; void load(); }}>
				<option value="7">7 hari</option>
				<option value="30">30 hari</option>
				<option value="90">90 hari</option>
				<option value="365">1 tahun</option>
			</select>
			<select class="app-input w-auto min-w-[150px]" aria-label="Filter step" bind:value={agent} onchange={() => { page = 1; void load(); }}>
				<option value="all">Semua Step</option>
				{#each Object.entries(AGENT_META) as [value, meta] (value)}
					<option value={value}>{meta.label}</option>
				{/each}
			</select>
			<select class="app-input w-auto min-w-[140px]" aria-label="Filter status" bind:value={status} onchange={() => { page = 1; void load(); }}>
				<option value="all">Semua Status</option>
				<option value="done">Berhasil</option>
				<option value="failed">Gagal</option>
				<option value="running">Berjalan</option>
			</select>
			<form class="flex items-center gap-2" onsubmit={submitSearch}>
				<input class="app-input w-[240px]" placeholder="Cari nama kampanye…" bind:value={searchInput} />
				<button class="app-btn app-btn-soft" type="submit"><Search size={14} /> Cari</button>
			</form>
			<button class="app-btn app-btn-soft" type="button" disabled={loading} onclick={() => void load()}>
				<RefreshCw size={14} /> Refresh
			</button>
		</div>

		<div class="mt-4 space-y-4">
			<div class="flex flex-wrap gap-3">
				<div class="app-card min-w-[170px] flex-1 p-3">
					<p class="text-xs text-muted-foreground">Total Panggilan AI</p>
					<p class="text-xl font-bold">{fmtInt(data?.summary.calls)}</p>
					<p class="text-xs text-muted-foreground">{fmtInt(data?.summary.done_calls)} berhasil · {fmtInt(data?.summary.failed_calls)} gagal</p>
				</div>
				<div class="app-card min-w-[170px] flex-1 p-3">
					<p class="text-xs text-muted-foreground">Token Masuk</p>
					<p class="text-xl font-bold" style="color: var(--accent-foreground);">{fmtInt(data?.summary.token_input)}</p>
				</div>
				<div class="app-card min-w-[170px] flex-1 p-3">
					<p class="text-xs text-muted-foreground">Token Keluar</p>
					<p class="text-xl font-bold" style="color: var(--success);">{fmtInt(data?.summary.token_output)}</p>
				</div>
				<div class="app-card min-w-[170px] flex-1 p-3">
					<p class="text-xs text-muted-foreground">Total Token</p>
					<p class="text-xl font-bold" style="color: var(--warning);">{fmtInt(totalTokens)}</p>
				</div>
				<div class="app-card min-w-[170px] flex-1 p-3">
					<p class="text-xs text-muted-foreground">Rata-rata Durasi</p>
					<p class="text-xl font-bold">{fmtDur(data?.summary.avg_duration_ms)}</p>
					<p class="text-xs text-muted-foreground">per panggilan berhasil</p>
				</div>
			</div>

			<div class="app-card p-4">
				<h2 class="text-sm font-semibold">Breakdown per Step AI</h2>
				<div class="mt-2 overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="text-xs text-muted-foreground">
								<th class="px-3 py-2 text-left font-medium">Step</th>
								<th class="px-3 py-2 text-left font-medium">Panggilan</th>
								<th class="px-3 py-2 text-left font-medium">Token Masuk</th>
								<th class="px-3 py-2 text-left font-medium">Token Keluar</th>
								<th class="px-3 py-2 text-left font-medium">Total Token</th>
								<th class="px-3 py-2 text-left font-medium">Porsi Token</th>
								<th class="px-3 py-2 text-left font-medium">Rata-rata Durasi</th>
								<th class="px-3 py-2 text-left font-medium">Gagal</th>
							</tr>
						</thead>
						<tbody>
							{#each data?.by_agent ?? [] as a (a.agent)}
								{@const share = totalTokens > 0 ? Math.round((a.total_tokens / totalTokens) * 100) : 0}
								<tr class="border-t">
									<td class="px-3 py-2"><Badge tone={AGENT_META[a.agent]?.tone ?? 'neutral'} label={AGENT_META[a.agent]?.label ?? a.agent} /></td>
									<td class="px-3 py-2">{fmtInt(a.calls)}</td>
									<td class="px-3 py-2">{fmtInt(a.token_input)}</td>
									<td class="px-3 py-2">{fmtInt(a.token_output)}</td>
									<td class="px-3 py-2">{fmtInt(a.total_tokens)}</td>
									<td class="px-3 py-2">
										<div class="flex min-w-[140px] items-center gap-2">
											<div class="h-1 flex-1 rounded" style="background-color: var(--secondary);">
												<div class="h-1 rounded" style="width: {share}%; background-color: var(--primary);"></div>
											</div>
											<span class="text-xs text-muted-foreground">{share}%</span>
										</div>
									</td>
									<td class="px-3 py-2">{fmtDur(a.avg_duration_ms)}</td>
									<td class="px-3 py-2">
										{#if a.failed_calls > 0}
											<span class="font-bold" style="color: var(--destructive);">{fmtInt(a.failed_calls)}</span>
										{:else}
											0
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
					{#if (data?.by_agent ?? []).length === 0}
						<p class="px-4 py-8 text-center text-sm text-muted-foreground">Belum ada pemakaian AI pada rentang ini.</p>
					{/if}
				</div>
			</div>

			<div class="app-card p-4">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<h2 class="text-sm font-semibold">Log Panggilan AI</h2>
					{#if data && data.total > 0}
						<p class="text-xs text-muted-foreground">
							{fmtInt((page - 1) * PAGE_SIZE + 1)}–{fmtInt(Math.min(page * PAGE_SIZE, data.total))} dari {fmtInt(data.total)} run
						</p>
					{/if}
				</div>
				<div class="mt-2 overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="text-xs text-muted-foreground">
								<th class="px-3 py-2 text-left font-medium">Waktu</th>
								<th class="px-3 py-2 text-left font-medium">Step</th>
								<th class="px-3 py-2 text-left font-medium">Status</th>
								<th class="px-3 py-2 text-left font-medium">Kampanye / Media</th>
								<th class="px-3 py-2 text-left font-medium">Model</th>
								<th class="px-3 py-2 text-left font-medium">Token Masuk</th>
								<th class="px-3 py-2 text-left font-medium">Token Keluar</th>
								<th class="px-3 py-2 text-left font-medium">Total</th>
								<th class="px-3 py-2 text-left font-medium">Durasi</th>
								<th class="px-3 py-2 text-left font-medium">Error</th>
							</tr>
						</thead>
						<tbody>
							{#each data?.runs ?? [] as r (r.id)}
								<tr class="border-t">
									<td class="px-3 py-2">{fmtTime(r.created_at)}</td>
									<td class="px-3 py-2"><Badge tone={AGENT_META[r.agent]?.tone ?? 'neutral'} label={AGENT_META[r.agent]?.label ?? r.agent} /></td>
									<td class="px-3 py-2">
										<Badge
											tone={r.status === 'done' ? 'success' : r.status === 'failed' ? 'danger' : 'warning'}
											label={r.status === 'done' ? 'Berhasil' : r.status === 'failed' ? 'Gagal' : 'Berjalan'}
										/>
									</td>
									<td class="px-3 py-2">{r.campaign_name ?? '-'}{r.media_name ? ` / ${r.media_name}` : ''}</td>
									<td class="px-3 py-2 text-xs text-muted-foreground">{r.model ?? '-'}</td>
									<td class="px-3 py-2">{r.token_input == null ? '-' : fmtInt(r.token_input)}</td>
									<td class="px-3 py-2">{r.token_output == null ? '-' : fmtInt(r.token_output)}</td>
									<td class="px-3 py-2">{r.token_input == null && r.token_output == null ? '-' : fmtInt((r.token_input ?? 0) + (r.token_output ?? 0))}</td>
									<td class="px-3 py-2">{fmtDur(r.duration_ms)}</td>
									<td class="max-w-[220px] break-words px-3 py-2 text-xs" style="color: var(--destructive);" title={r.error ?? undefined}>{r.error ?? '-'}</td>
								</tr>
							{/each}
						</tbody>
					</table>
					{#if (data?.runs ?? []).length === 0}
						<p class="px-4 py-8 text-center text-sm text-muted-foreground">Belum ada run pada filter ini.</p>
					{/if}
				</div>
				{#if data && data.total > PAGE_SIZE}
					<Pagination
						total={data.total}
						limit={PAGE_SIZE}
						offset={(page - 1) * PAGE_SIZE}
						onChange={(next) => {
							page = next / PAGE_SIZE + 1;
							void load();
						}}
					/>
				{/if}
			</div>
		</div>
	{/if}
</section>
