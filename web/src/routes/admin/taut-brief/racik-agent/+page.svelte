<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import { apiRequest, readApiError } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';
	import { RefreshCw } from '@lucide/svelte';
	import AgentPanel from '../_agent-panel.svelte';

	const BASE = '/v1/platform/taut-brief';

	type RunLog = {
		id: string;
		agent: string | null;
		brand_name: string;
		status: string;
		model: string | null;
		token_input: number | null;
		token_output: number | null;
		duration_ms: number | null;
		error: string | null;
		input_preview: string | null;
		output_preview: string | null;
		created_at: string;
	};

	let tab = $state<'agent' | 'log'>('agent');
	let logs = $state<RunLog[]>([]);
	let logsLoading = $state(false);
	let logsError = $state('');

	async function loadLogs() {
		logsLoading = true;
		logsError = '';
		try {
			const res = await apiRequest(`${BASE}/logs?agent=racik_instruksi&limit=100`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat log.')));
			const data = await res.json();
			// Filter server (?agent=) sudah aktif; saringan ini menjaga tab tetap
			// racik-only bila API yang dilayani masih versi lama.
			logs = (data?.items ?? []).filter((r: RunLog) => r.agent === 'racik_instruksi');
		} catch (e) {
			logsError = e instanceof Error ? e.message : tr('Gagal memuat log.');
		} finally {
			logsLoading = false;
		}
	}

	function openTab(next: 'agent' | 'log') {
		tab = next;
		if (next === 'log' && logs.length === 0 && !logsLoading) void loadLogs();
	}
</script>

<section>
	<PageHeader
		title={tr('Racik Agent')}
		description={tr('Menyusun instruksi topik dari anchor di Pengelola Naskah. Dipakai tombol Racik Instruksi dari Anchor.')}
	/>

	<div class="mt-4 flex flex-wrap gap-1">
		{#each [['agent', 'Konfigurasi'], ['log', 'Log pemakaian']] as [value, label] (value)}
			<button
				type="button"
				class="app-btn !min-h-8 !px-3 !text-xs"
				class:app-btn-primary={tab === value}
				class:app-btn-soft={tab !== value}
				onclick={() => openTab(value as 'agent' | 'log')}
			>{tr(label)}</button>
		{/each}
	</div>

	{#if tab === 'agent'}
		<AgentPanel
			agent="racik_instruksi"
			title={tr('Koneksi & Prompt Agent')}
			description={tr('Atur transport, model, dan master prompt yang dipakai agent racik instruksi.')}
			disabledHint={tr('Selama nonaktif, tombol Racik Instruksi dari Anchor akan menolak permintaan.')}
		/>
	{:else}
		<section class="app-card mt-4 p-5">
			<div class="flex items-center justify-between">
				<h2 class="text-sm font-semibold">{tr('Log pemakaian')}</h2>
				<button
					class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
					type="button"
					disabled={logsLoading}
					onclick={() => void loadLogs()}
				>
					<RefreshCw size={13} />{tr('Muat ulang')}
				</button>
			</div>

			{#if logsError}
				<p class="mt-3 text-sm" style="color: var(--destructive);">{logsError}</p>
			{:else if logs.length === 0}
				<p class="mt-3 text-sm text-muted-foreground">
					{logsLoading ? tr('Memuat…') : tr('Belum ada pemakaian.')}
				</p>
			{:else}
				<div class="mt-3 overflow-x-auto">
					<table class="w-full text-left text-xs">
						<thead class="text-muted-foreground">
							<tr>
								<th class="py-1 pr-3">Waktu</th>
								<th class="py-1 pr-3">Brand</th>
								<th class="py-1 pr-3">Status</th>
								<th class="py-1 pr-3">Model</th>
								<th class="py-1 pr-3">Token</th>
								<th class="py-1 pr-3">Durasi</th>
								<th class="py-1 pr-3">Anchor</th>
								<th class="py-1 pr-3">Hasil</th>
							</tr>
						</thead>
						<tbody>
							{#each logs as r (r.id)}
								<tr class="border-t" style="border-color: var(--border);">
									<td class="py-1.5 pr-3 whitespace-nowrap">{fmtDateTime(r.created_at)}</td>
									<td class="py-1.5 pr-3">{r.brand_name || '—'}</td>
									<td class="py-1.5 pr-3">
										<span style="color: {r.status === 'done' ? 'var(--success)' : r.status === 'failed' ? 'var(--destructive)' : 'var(--muted-foreground)'};">{r.status}</span>
										{#if r.error}<span class="ml-1 text-muted-foreground" title={r.error}>·</span>{/if}
									</td>
									<td class="py-1.5 pr-3">{r.model ?? '—'}</td>
									<td class="py-1.5 pr-3">{r.token_input ?? 0} / {r.token_output ?? 0}</td>
									<td class="py-1.5 pr-3 whitespace-nowrap">{r.duration_ms ?? 0} ms</td>
									<td class="max-w-[16rem] truncate py-1.5 pr-3" title={r.input_preview ?? ''}>{r.input_preview || '—'}</td>
									<td class="max-w-[20rem] truncate py-1.5 pr-3" title={r.output_preview ?? ''}>{r.output_preview || '—'}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</section>
	{/if}
</section>
