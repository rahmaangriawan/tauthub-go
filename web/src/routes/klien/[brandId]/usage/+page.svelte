<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Port UsageSection (brand-workspace.tsx) — pemakaian token AI brand ini
	// dari /usage. Endpoint & payload sama dengan app lama.
	import { onMount, untrack } from 'svelte';
	import { page } from '$app/state';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);

	type UsagePayload = {
		days: number;
		summary: { calls: number; successful_calls: number; failed_calls: number; input_tokens: number; output_tokens: number; total_tokens: number; avg_latency_ms: number; cache_hits: number };
		quota: { monthly_token_limit: number; warning_percent: number; plan_code: string; daily_call_limit: number } | null;
		daily: Array<{ date: string; calls: number; input_tokens: number; output_tokens: number; total_tokens: number }>;
		by_model: Array<{ provider: string; model: string; calls: number; total_tokens: number; avg_latency_ms: number }>;
		by_task: Array<{ task_type: string; calls: number; total_tokens: number; failed_calls: number }>;
		events: Array<{ task_type: string; provider: string; model: string; input_tokens: number; output_tokens: number; total_tokens: number; latency_ms: number; success: number | boolean; error_category: string | null; cache_hit: number | boolean; created_at: string }>;
		events_total: number;
		limit: number;
		offset: number;
	};

	let days = $state(30);
	let data = $state<UsagePayload | null>(null);
	let busy = $state(true);
	let err = $state('');
	let dailyPage = $state(1);
	let eventPage = $state(1);
	const PAGE_SIZE = 20;

	// Samarkan nama model internal jadi label Tauthub yang ramah klien (parity).
	const modelLabel = (model: string) => {
		const n = model.toLowerCase();
		if (n.includes('economy') || n.includes('lite') || n.includes('mini') || n.includes('1.2')) return 'Tauthub Lite';
		if (n.includes('premium') || n.includes('pro') || n.includes('max') || n.includes('turbo')) return 'Tauthub High';
		return 'Tauthub Standard';
	};
	const PROVIDER_LABEL = 'Tauthub AI';

	const fmt = (n: number) => new Intl.NumberFormat('id-ID').format(n);
	const fmtDateTime = (v: string) => new Date(v).toLocaleString('id-ID');

	async function load() {
		busy = true;
		err = '';
		try {
			const res = await apiRequest(
				`/v1/brands/${brandId}/usage?days=${days}&limit=${PAGE_SIZE}&offset=${(eventPage - 1) * PAGE_SIZE}`
			);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat usage.')));
			data = (await res.json()) as UsagePayload;
		} catch (e) {
			err = e instanceof Error ? e.message : tr('Gagal memuat usage.');
		} finally {
			busy = false;
		}
	}

	$effect(() => {
		void days;
		void eventPage;
		untrack(() => void load());
	});

	onMount(() => {
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const summary = $derived(data?.summary);
	const quota = $derived(data?.quota);
	const used = $derived(summary?.total_tokens ?? 0);
	const limit = $derived(quota?.monthly_token_limit ?? 0);
	const pct = $derived(limit > 0 ? Math.min(100, Math.round((used / limit) * 100)) : 0);
	const dailyTotalPages = $derived(Math.max(1, Math.ceil((data?.daily?.length ?? 0) / PAGE_SIZE)));
	const visibleDaily = $derived((data?.daily ?? []).slice((dailyPage - 1) * PAGE_SIZE, dailyPage * PAGE_SIZE));
	const metrics = $derived(summary ? [
		['Total tokens', fmt(summary.total_tokens), ''],
		['Input', fmt(summary.input_tokens), ''],
		['Output', fmt(summary.output_tokens), ''],
		['Calls', fmt(summary.calls), `${summary.successful_calls} ok · ${summary.failed_calls} fail`],
		['Avg latency', `${fmt(summary.avg_latency_ms)} ms`, ''],
		['Cache hit', fmt(summary.cache_hits), '']
	] : []);
</script>

<PageTitle title="Usage" />

<section>
	<header class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Usage</h1>
			<p class="mt-1 text-sm text-muted-foreground">Pemakaian token AI brand ini dari model_usage_events.</p>
		</div>
		<div class="flex gap-1 rounded-lg border p-1" style="border-color: var(--border);">
			{#each [7, 30, 90] as d (d)}
				<button
					type="button"
					onclick={() => { days = d; eventPage = 1; }}
					class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
					style={days === d
						? 'background: var(--primary); color: var(--primary-foreground);'
						: 'color: var(--muted-foreground);'}
				>{d} hari</button>
			{/each}
		</div>
	</header>

	{#if err}
		<p class="mt-4 rounded-md px-3 py-2 text-sm text-destructive" style="background: color-mix(in srgb, var(--destructive) 8%, transparent);">{err}</p>
	{/if}

	{#if busy && !data}
		<p class="mt-6 text-sm text-muted-foreground">{tr('Memuat usage…')}</p>
	{:else if summary}
		<div class="app-card mt-4 grid grid-cols-2 gap-4 p-5 sm:grid-cols-3 lg:grid-cols-6">
			{#each metrics as [label, value, meta] (label)}
				<div>
					<span class="text-xs text-muted-foreground">{label}</span>
					<strong class="block text-lg font-semibold tabular-nums">{value}</strong>
					{#if meta}<span class="text-xs text-muted-foreground">{meta}</span>{/if}
				</div>
			{/each}
		</div>
		{#if quota}
			<div class="app-card mt-4 p-5">
				<div class="flex flex-wrap items-center justify-between gap-2 text-sm">
					<span class="font-medium">{tr('Kuota bulanan')} · {quota.plan_code}</span>
					<span class="tabular-nums text-muted-foreground">{fmt(used)} / {fmt(limit)} ({pct}%)</span>
				</div>
				<div class="mt-2 h-2 w-full overflow-hidden rounded-full" style="background: var(--muted);">
					<div
						class="h-full rounded-full"
						style="width: {pct}%; background: {pct >= (quota.warning_percent || 80) ? 'var(--warning)' : 'var(--primary)'};"
					></div>
				</div>
				<span class="mt-1.5 block text-xs text-muted-foreground">Daily call limit: {fmt(quota.daily_call_limit)} · warning {quota.warning_percent}%</span>
			</div>
		{/if}
	{/if}

	<div class="mt-4 grid gap-4 lg:grid-cols-2">
		<div class="app-card p-5">
			<h3 class="font-semibold">Per model</h3>
			<div class="mt-3 flex flex-col gap-2">
				{#if (data?.by_model ?? []).length === 0}
					<p class="text-sm text-muted-foreground">Belum ada pemakaian.</p>
				{:else}
					{#each data?.by_model ?? [] as row (`${row.provider}:${row.model}`)}
						<div class="flex items-center justify-between gap-3 rounded-lg border px-3 py-2" style="border-color: var(--border);">
							<div>
								<span class="block text-sm font-medium">{modelLabel(row.model)}</span>
								<span class="text-xs text-muted-foreground">{PROVIDER_LABEL} · {fmt(Number(row.calls))} calls · avg {fmt(Number(row.avg_latency_ms))} ms</span>
							</div>
							<span class="rounded-full border px-2 py-0.5 text-xs font-semibold" style="border-color: var(--border);">{fmt(Number(row.total_tokens))} tok</span>
						</div>
					{/each}
				{/if}
			</div>
		</div>

		<div class="app-card p-5">
			<h3 class="font-semibold">Per task</h3>
			<div class="mt-3 flex flex-col gap-2">
				{#if (data?.by_task ?? []).length === 0}
					<p class="text-sm text-muted-foreground">Belum ada task.</p>
				{:else}
					{#each data?.by_task ?? [] as row (row.task_type)}
						<div class="flex items-center justify-between gap-3 rounded-lg border px-3 py-2" style="border-color: var(--border);">
							<div>
								<span class="block text-sm font-medium">{row.task_type}</span>
								<span class="text-xs text-muted-foreground">{fmt(Number(row.calls))} calls · {fmt(Number(row.failed_calls))} fail</span>
							</div>
							<span class="rounded-full border px-2 py-0.5 text-xs font-semibold" style="border-color: var(--border);">{fmt(Number(row.total_tokens))} tok</span>
						</div>
					{/each}
				{/if}
			</div>
		</div>
	</div>

	<div class="app-card mt-4 p-5">
		<h3 class="font-semibold">Harian</h3>
		<div class="mt-3 flex flex-col gap-2">
			{#if (data?.daily ?? []).length === 0}
				<p class="text-sm text-muted-foreground">Belum ada data harian.</p>
			{:else}
				{#each visibleDaily as row (row.date)}
					<div class="flex items-center justify-between gap-3 rounded-lg border px-3 py-2" style="border-color: var(--border);">
						<div>
							<span class="block text-sm font-medium">{row.date}</span>
							<span class="text-xs text-muted-foreground">{fmt(Number(row.calls))} calls · in {fmt(Number(row.input_tokens))} · out {fmt(Number(row.output_tokens))}</span>
						</div>
						<span class="rounded-full border px-2 py-0.5 text-xs font-semibold" style="border-color: var(--border);">{fmt(Number(row.total_tokens))} tok</span>
					</div>
				{/each}
			{/if}
		</div>
		{#if dailyTotalPages > 1}
			<div class="mt-3 flex items-center justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" disabled={dailyPage <= 1 || busy} onclick={() => (dailyPage = Math.max(1, dailyPage - 1))}>{tr('Sebelumnya')}</button>
				<button class="app-btn app-btn-soft" type="button" disabled={dailyPage >= dailyTotalPages || busy} onclick={() => (dailyPage = Math.min(dailyTotalPages, dailyPage + 1))}>{tr('Berikutnya')}</button>
			</div>
		{/if}
	</div>

	<div class="app-card mt-4 p-5">
		<h3 class="font-semibold">Event terbaru</h3>
		<div class="mt-3 flex flex-col gap-2">
			{#if (data?.events ?? []).length === 0}
				<p class="text-sm text-muted-foreground">Belum ada event.</p>
			{:else}
				{#each data?.events ?? [] as row, i (`${row.created_at}-${i}`)}
					<div class="flex items-center justify-between gap-3 rounded-lg border px-3 py-2" style="border-color: var(--border);">
						<div>
							<span class="block text-sm font-medium">{row.task_type} · {modelLabel(row.model)}</span>
							<span class="text-xs text-muted-foreground">{PROVIDER_LABEL} · in {fmt(Number(row.input_tokens))} · out {fmt(Number(row.output_tokens))} · {fmt(Number(row.latency_ms))} ms · {fmtDateTime(row.created_at)}{row.error_category ? ` · ${row.error_category}` : ''}{row.cache_hit ? ' · cache' : ''}</span>
						</div>
						<span class="text-xs font-semibold" style="color: {row.success ? 'var(--success)' : 'var(--destructive)'};">{row.success ? 'ok' : 'fail'} · {fmt(Number(row.total_tokens))}</span>
					</div>
				{/each}
			{/if}
		</div>
		<div class="mt-3 flex items-center justify-end gap-2">
			<button class="app-btn app-btn-soft" type="button" disabled={eventPage <= 1 || busy} onclick={() => (eventPage = Math.max(1, eventPage - 1))}>{tr('Sebelumnya')}</button>
			<span class="text-xs text-muted-foreground">{eventPage}</span>
			<button class="app-btn app-btn-soft" type="button" disabled={busy || (data?.events?.length ?? 0) < PAGE_SIZE} onclick={() => (eventPage = eventPage + 1)}>{tr('Berikutnya')}</button>
		</div>
	</div>
</section>
