<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import { onMount } from 'svelte';
	import { apiRequest, readApiError } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';

	// Log pemakaian khusus dua agent AI Media Matchmaker (peringkas brief +
	// pemilih media): token, durasi, error, dan preview input/output.
	const AGENTS = 'media_matchmaker,media_matchmaker_select';

	type RunLog = {
		id: string;
		agent: string;
		brand_name: string;
		status: string;
		model: string | null;
		token_input: number | null;
		token_output: number | null;
		duration_ms: number | null;
		error: string | null;
		input_preview: string | null;
		output_preview: string | null;
		created_at: string | null;
	};

	let logs = $state<RunLog[]>([]);
	let loading = $state(true);
	let error = $state('');
	let openId = $state('');

	const agentLabel = (agent: string) =>
		agent === 'media_matchmaker_select' ? tr('Agent Pemilih Media') : tr('Agent Ringkasan');

	const stats = $derived.by(() => {
		const total = logs.length;
		const failed = logs.filter((l) => l.status === 'failed').length;
		const tokIn = logs.reduce((s, l) => s + (l.token_input ?? 0), 0);
		const tokOut = logs.reduce((s, l) => s + (l.token_output ?? 0), 0);
		const durated = logs.filter((l) => (l.duration_ms ?? 0) > 0);
		const avgDur = durated.length
			? Math.round(durated.reduce((s, l) => s + (l.duration_ms ?? 0), 0) / durated.length / 1000)
			: 0;
		return { total, failed, tokIn, tokOut, avgDur };
	});

	async function load() {
		loading = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/taut-brief/logs?agent=${AGENTS}&limit=200`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat log.')));
			const data = await res.json();
			// Server sudah menyaring agent; saringan ini menjaga daftar tetap
			// matchmaker-only bila API yang dilayani masih versi lama.
			logs = (data?.items ?? []).filter(
				(r: RunLog) => r.agent === 'media_matchmaker' || r.agent === 'media_matchmaker_select'
			);
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal memuat log.');
		} finally {
			loading = false;
		}
	}

	function toggle(id: string) {
		openId = openId === id ? '' : id;
	}

	onMount(() => void load());
</script>

<section>
	<ExplorerFrame
		parents={[{ label: tr('Taut Pintar'), href: '/admin/taut-pintar' }, { label: tr('Agent Matchmaker'), href: '/admin/taut-brief/agent-matchmaker' }]}
		crumb={tr('Log')}
		path="/admin/taut-brief/agent-matchmaker/log"
	>
		<PageHeader
			title={tr('Log Agent Matchmaker')}
			description={tr('Pemakaian token, durasi, dan error dua agent AI Media Matchmaker (ringkasan brief & pemilih media).')}
		/>

		<Feedback {loading} {error} loadingText={tr('Memuat log…')} />

		{#if !loading && !error}
			<div class="statRow">
				<span class="stat">{tr('Total run')} <strong>{stats.total}</strong></span>
				<span class="stat statBad">{tr('Gagal')} <strong>{stats.failed}</strong></span>
				<span class="stat">{tr('Token masuk')} <strong>{stats.tokIn.toLocaleString('id-ID')}</strong></span>
				<span class="stat">{tr('Token keluar')} <strong>{stats.tokOut.toLocaleString('id-ID')}</strong></span>
				<span class="stat">{tr('Durasi rata-rata')} <strong>{stats.avgDur} dtk</strong></span>
				<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={() => void load()}>
					{tr('Muat ulang')}
				</button>
			</div>

			<section class="app-card mt-4 p-5">
				{#if logs.length === 0}
					<p class="py-3 text-sm text-muted-foreground">{tr('Belum ada run. Coba AI Matchmaker dari katalog media dulu.')}</p>
				{:else}
					<table class="w-full text-left text-xs">
						<thead class="text-muted-foreground">
							<tr>
								<th class="py-1 pr-3">Waktu</th>
								<th class="py-1 pr-3">Agent</th>
								<th class="py-1 pr-3">Brand</th>
								<th class="py-1 pr-3">Status</th>
								<th class="py-1 pr-3">Model</th>
								<th class="py-1 pr-3">Token (in → out)</th>
								<th class="py-1 pr-3">Durasi</th>
							</tr>
						</thead>
						<tbody>
							{#each logs as l (l.id)}
								{@const failed = l.status === 'failed'}
								<tr
									class="cursor-pointer border-t hover:bg-secondary/50"
									onclick={() => toggle(l.id)}
								>
									<td class="py-1.5 pr-3 whitespace-nowrap">{l.created_at ? fmtDateTime(l.created_at) : '—'}</td>
									<td class="py-1.5 pr-3">{agentLabel(l.agent)}</td>
									<td class="py-1.5 pr-3">{l.brand_name || '—'}</td>
									<td class="py-1.5 pr-3">
										<span class={failed ? 'text-red-600 font-semibold' : 'text-emerald-600'}>{failed ? 'Gagal' : 'Sukses'}</span>
									</td>
									<td class="py-1.5 pr-3">{l.model || '—'}</td>
									<td class="py-1.5 pr-3">{(l.token_input ?? 0).toLocaleString('id-ID')} → {(l.token_output ?? 0).toLocaleString('id-ID')}</td>
									<td class="py-1.5 pr-3 whitespace-nowrap">{l.duration_ms ? `${Math.round(l.duration_ms / 1000)} dtk` : '—'}</td>
								</tr>
								{#if openId === l.id}
									<tr class="border-t bg-secondary/30">
										<td colspan="7" class="py-3 pr-3">
											<div class="grid gap-3 md:grid-cols-2">
												<div>
													<p class="mb-1 font-semibold">{tr('Bahan yang dikirim ke agent')}</p>
													<pre class="max-h-56 overflow-auto whitespace-pre-wrap break-words rounded-md bg-background p-2 text-[11px] text-muted-foreground">{l.input_preview || '—'}</pre>
												</div>
												<div>
													<p class="mb-1 font-semibold">{tr('Output agent')}</p>
													{#if failed}
														<p class="rounded-md bg-red-50 p-2 text-[11px] text-red-700">{l.error || tr('Tidak ada pesan error.')}</p>
													{:else}
														<pre class="max-h-56 overflow-auto whitespace-pre-wrap break-words rounded-md bg-background p-2 text-[11px] text-muted-foreground">{l.output_preview || '—'}</pre>
													{/if}
												</div>
											</div>
										</td>
									</tr>
								{/if}
							{/each}
						</tbody>
					</table>
				{/if}
			</section>
		{/if}
	</ExplorerFrame>
</section>
