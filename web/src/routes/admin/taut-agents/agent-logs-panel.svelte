<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Panel log agent: ringkasan 7 hari, filter agent/status, tabel log eksekusi
	// dan gudang training AI. Port app/admin/taut-agents/agent-logs.tsx.
	import { onMount } from 'svelte';
	import { RefreshCw, TriangleAlert } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';
	import Select from '$lib/components/select.svelte';
	import CopyMarkdownButton from '$lib/components/admin/copy-markdown-button.svelte';
	import { mdReport, mdTable, mdList, mdCode } from '$lib/copy-markdown';

	const AGENT_LABEL: Record<string, string> = {
		profiler: 'Profiler',
		matchmaker: 'Matchmaker',
		writer: 'Writer',
		gatekeeper: 'Gatekeeper'
	};

	let loading = $state(true);
	let error = $state('');
	let data = $state<any>(null);
	let agentFilter = $state('all');
	let statusFilter = $state('all');
	let tab = $state<'runs' | 'training'>('runs');
	let runsPage = $state(1);
	let trainingPage = $state(1);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams();
			if (agentFilter !== 'all') params.set('agent', agentFilter);
			if (statusFilter !== 'all') params.set('status', statusFilter);
			const res = await apiRequest(`/v1/platform/taut-agent-logs?${params.toString()}`);
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal memuat log agent.'));
				return;
			}
			data = await res.json();
		} finally {
			loading = false;
		}
	}

	const okCount = $derived(
		(data?.summary_7d ?? []).filter((s: any) => s.status === 'done').reduce((a: number, b: any) => a + b.count, 0)
	);
	const failCount = $derived(
		(data?.summary_7d ?? []).filter((s: any) => s.status === 'failed').reduce((a: number, b: any) => a + b.count, 0)
	);
	const runs = $derived<any[]>(data?.runs ?? []);
	const training = $derived<any[]>(data?.training ?? []);
	const runsSlice = $derived(runs.slice((runsPage - 1) * 10, runsPage * 10));
	const trainingSlice = $derived(training.slice((trainingPage - 1) * 10, trainingPage * 10));

	const statusText = (s: string) => (s === 'done' ? 'Berhasil' : s === 'failed' ? 'Gagal' : 'Berjalan');
	const isLong = (v: unknown) => typeof v === 'string' && (v.includes('\n') || v.length > 200);

	// Salin tab yang sedang tampil (filter dan halaman ikut) sebagai Markdown.
	function toMarkdown(): string {
		const filters = mdList([
			['Filter agent', agentFilter === 'all' ? 'Semua' : (AGENT_LABEL[agentFilter] ?? agentFilter)],
			['Filter status', statusFilter === 'all' ? 'Semua' : statusText(statusFilter)],
			['7 hari terakhir - Berhasil', okCount],
			['7 hari terakhir - Gagal', failCount]
		]);
		const sections: string[] = [];
		if (tab === 'runs') {
			sections.push(
				'## Log Eksekusi Agent',
				mdList([['Halaman', `${runsPage} dari ${Math.max(1, Math.ceil(runs.length / 10))} (total ${runs.length} run)`]]),
				mdTable(
					['Waktu', 'Agent', 'Status', 'Kampanye / Media', 'Model', 'Token (in/out)', 'Error', 'Run ID'],
					runsSlice.map((r) => [
						fmtDateTime(r.created_at),
						AGENT_LABEL[r.agent] ?? r.agent,
						statusText(r.status),
						`${r.campaign_name ?? '-'}${r.media_name ? ` / ${r.media_name}` : ''}`,
						r.model,
						`${r.token_input ?? '-'} / ${r.token_output ?? '-'}`,
						isLong(r.error) ? '(lihat blok di bawah)' : r.error,
						r.id
					])
				)
			);
			for (const r of runsSlice) {
				if (isLong(r.error)) sections.push(`### Error ${r.id}`, mdCode(r.error));
			}
		} else {
			sections.push(
				'## Gudang Training AI',
				mdList([['Halaman', `${trainingPage} dari ${Math.max(1, Math.ceil(training.length / 10))} (total ${training.length} sample)`]]),
				mdTable(
					['Waktu', 'Agent', 'Status', 'Input (preview)', 'Output (preview)', 'ID'],
					trainingSlice.map((t) => [
						fmtDateTime(t.created_at),
						AGENT_LABEL[t.agent] ?? t.agent,
						statusText(t.status),
						isLong(t.input_preview) ? '(lihat blok di bawah)' : t.input_preview,
						isLong(t.output_preview) ? '(lihat blok di bawah)' : t.output_preview,
						t.id
					])
				)
			);
			for (const t of trainingSlice) {
				if (isLong(t.input_preview)) sections.push(`### Input ${t.id}`, mdCode(t.input_preview));
				if (isLong(t.output_preview)) sections.push(`### Output ${t.id}`, mdCode(t.output_preview));
			}
		}
		return mdReport('Log Agent', filters, ...sections);
	}
</script>

<div class="flex flex-col gap-4">
	<div class="flex flex-wrap gap-3">
		<div class="app-card p-4" style="min-width: 160px;">
			<p class="text-xs text-muted-foreground">{tr("7 hari terakhir — Berhasil")}</p>
			<p class="text-2xl font-bold" style="color: var(--success);">{okCount}</p>
		</div>
		<div class="app-card p-4" style="min-width: 160px;">
			<p class="text-xs text-muted-foreground">{tr("7 hari terakhir — Gagal")}</p>
			<p class="text-2xl font-bold" style="color: {failCount > 0 ? 'var(--destructive)' : 'var(--muted-foreground)'};">{failCount}</p>
		</div>
	</div>

	<div class="flex flex-wrap items-center gap-3">
		<Select
			ariaLabel="Filter agent"
			minWidth={160}
			value={agentFilter}
			onchange={(v) => {
				agentFilter = v;
				void load();
			}}
			options={[
				{ value: 'all', label: tr('Semua Agent') },
				...Object.entries(AGENT_LABEL).map(([v, l]) => ({ value: v, label: l }))
			]}
		/>
		<Select
			ariaLabel="Filter status"
			minWidth={140}
			value={statusFilter}
			onchange={(v) => {
				statusFilter = v;
				void load();
			}}
			options={[
				{ value: 'all', label: tr('Semua Status') },
				{ value: 'done', label: tr('Berhasil') },
				{ value: 'failed', label: tr('Gagal') }
			]}
		/>
		<button class="app-btn app-btn-soft" disabled={loading} onclick={load}><RefreshCw size={14} /> Refresh</button>
		<CopyMarkdownButton build={toMarkdown} disabled={loading || (tab === 'runs' ? runs.length === 0 : training.length === 0)} />
	</div>

	{#if error}<p class="text-sm" style="color: var(--destructive);">{error}</p>{/if}

	{#if loading && !data}
		<p class="text-sm text-muted-foreground">{tr("Memuat log…")}</p>
	{:else}
		<div class="flex gap-2">
			<button class="app-btn !min-h-8 !px-3 !text-xs" class:app-btn-primary={tab === 'runs'} class:app-btn-soft={tab !== 'runs'} onclick={() => (tab = 'runs')}>Log Eksekusi Agent</button>
			<button class="app-btn !min-h-8 !px-3 !text-xs" class:app-btn-primary={tab === 'training'} class:app-btn-soft={tab !== 'training'} onclick={() => (tab = 'training')}>Gudang Training AI</button>
		</div>

		{#if tab === 'runs'}
			<div class="app-card overflow-x-auto p-4">
				<table class="w-full text-sm">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="py-1.5 pr-3 font-medium">Waktu</th>
							<th class="py-1.5 pr-3 font-medium">Agent</th>
							<th class="py-1.5 pr-3 font-medium">Status</th>
							<th class="py-1.5 pr-3 font-medium">{tr("Kampanye / Media")}</th>
							<th class="py-1.5 pr-3 font-medium">Model</th>
							<th class="py-1.5 pr-3 font-medium">Token (in/out)</th>
							<th class="py-1.5 font-medium">Error</th>
						</tr>
					</thead>
					<tbody>
						{#each runsSlice as r (r.id)}
							<tr class="border-t">
								<td class="py-1.5 pr-3 text-xs">{fmtDateTime(r.created_at)}</td>
								<td class="py-1.5 pr-3">{AGENT_LABEL[r.agent] ?? r.agent}</td>
								<td class="py-1.5 pr-3">
									{#if r.status === 'done'}
										<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--success-soft); color: var(--success);">{tr("Berhasil")}</span>
									{:else if r.status === 'failed'}
										<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px]" style="background: var(--destructive-soft, #fdecee); color: var(--destructive);">
											<TriangleAlert size={10} />{tr("Gagal")}</span>
									{:else}
										<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--warning-soft); color: var(--warning);">Berjalan</span>
									{/if}
								</td>
								<td class="py-1.5 pr-3">{r.campaign_name ?? '-'}{r.media_name ? ` / ${r.media_name}` : ''}</td>
								<td class="py-1.5 pr-3 text-xs">{r.model ?? '-'}</td>
								<td class="py-1.5 pr-3 text-xs tabular-nums">{r.token_input ?? '-'} / {r.token_output ?? '-'}</td>
								<td class="max-w-[260px] break-words py-1.5 text-xs">{r.error ?? '-'}</td>
							</tr>
						{/each}
						{#if runs.length === 0}
							<tr><td class="py-6 text-center text-sm text-muted-foreground" colspan="7">{tr("Belum ada log.")}</td></tr>
						{/if}
					</tbody>
				</table>
				{#if runs.length > 10}
					<div class="mt-3 flex items-center justify-between text-sm">
						<p class="text-muted-foreground">{tr("Halaman")} {runsPage} {tr("dari")} {Math.ceil(runs.length / 10)}</p>
						<div class="flex gap-2">
							<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" disabled={runsPage === 1} onclick={() => (runsPage -= 1)}>Sebelumnya</button>
							<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" disabled={runsPage * 10 >= runs.length} onclick={() => (runsPage += 1)}>Berikutnya</button>
						</div>
					</div>
				{/if}
			</div>
		{:else}
			<div class="app-card overflow-x-auto p-4">
				<p class="mb-2 text-xs text-muted-foreground">{tr("Semua input klien & hasil agent tersimpan otomatis untuk improvisasi model platform (maks 10 baris per halaman).")}</p>
				<table class="w-full text-sm">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="py-1.5 pr-3 font-medium">Waktu</th>
							<th class="py-1.5 pr-3 font-medium">Agent</th>
							<th class="py-1.5 pr-3 font-medium">Status</th>
							<th class="py-1.5 pr-3 font-medium">Input (preview)</th>
							<th class="py-1.5 font-medium">Output (preview)</th>
						</tr>
					</thead>
					<tbody>
						{#each trainingSlice as t (t.id)}
							<tr class="border-t">
								<td class="py-1.5 pr-3 text-xs">{fmtDateTime(t.created_at)}</td>
								<td class="py-1.5 pr-3">{AGENT_LABEL[t.agent] ?? t.agent}</td>
								<td class="py-1.5 pr-3">
									{#if t.status === 'done'}
										<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--success-soft); color: var(--success);">{tr("Berhasil")}</span>
									{:else}
										<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--destructive-soft, #fdecee); color: var(--destructive);">{tr("Gagal")}</span>
									{/if}
								</td>
								<td class="max-w-[300px] break-words py-1.5 pr-3 text-xs">{t.input_preview ?? '-'}</td>
								<td class="max-w-[300px] break-words py-1.5 text-xs">{t.output_preview ?? '-'}</td>
							</tr>
						{/each}
						{#if training.length === 0}
							<tr><td class="py-6 text-center text-sm text-muted-foreground" colspan="5">{tr("Belum ada sample training.")}</td></tr>
						{/if}
					</tbody>
				</table>
				{#if training.length > 10}
					<div class="mt-3 flex items-center justify-between text-sm">
						<p class="text-muted-foreground">{tr("Halaman")} {trainingPage} {tr("dari")} {Math.ceil(training.length / 10)}</p>
						<div class="flex gap-2">
							<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" disabled={trainingPage === 1} onclick={() => (trainingPage -= 1)}>Sebelumnya</button>
							<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" disabled={trainingPage * 10 >= training.length} onclick={() => (trainingPage += 1)}>Berikutnya</button>
						</div>
					</div>
				{/if}
			</div>
		{/if}
	{/if}
</div>
