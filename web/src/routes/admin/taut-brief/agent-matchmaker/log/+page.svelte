<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import { onMount } from 'svelte';
	import { apiRequest, readApiError } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';
	import CopyMarkdownButton from '$lib/components/admin/copy-markdown-button.svelte';
	import { mdReport, mdTable, mdList, mdCode } from '$lib/copy-markdown';

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
	let loadingMore = $state(false);
	let nextBefore = $state('');
	let error = $state('');
	let openId = $state('');
	let sentinel = $state<HTMLDivElement | null>(null);

	// Paginasi + lazy load: halaman pertama 50 baris, sisanya menyusul saat
	// sentinel terlihat (atau lewat tombol) — tabel panjang tidak dimuat
	// sekaligus lagi.
	const PAGE_SIZE = 50;

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
		nextBefore = '';
		try {
			const res = await apiRequest(`/v1/platform/taut-brief/logs?agent=${AGENTS}&limit=${PAGE_SIZE}`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat log.')));
			const data = await res.json();
			// Server sudah menyaring agent; saringan ini menjaga daftar tetap
			// matchmaker-only bila API yang dilayani masih versi lama.
			logs = (data?.items ?? []).filter(
				(r: RunLog) => r.agent === 'media_matchmaker' || r.agent === 'media_matchmaker_select'
			);
			nextBefore = data?.next_before ?? '';
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal memuat log.');
		} finally {
			loading = false;
		}
	}

	async function loadMore() {
		if (loading || loadingMore || !nextBefore) return;
		loadingMore = true;
		try {
			const res = await apiRequest(
				`/v1/platform/taut-brief/logs?agent=${AGENTS}&limit=${PAGE_SIZE}&before=${encodeURIComponent(nextBefore)}`
			);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat log.')));
			const data = await res.json();
			const seen = new Set(logs.map((l) => l.id));
			const more = ((data?.items ?? []) as RunLog[]).filter(
				(r) =>
					(r.agent === 'media_matchmaker' || r.agent === 'media_matchmaker_select') && !seen.has(r.id)
			);
			logs = [...logs, ...more];
			nextBefore = data?.next_before ?? '';
		} catch {
			// Halaman berikutnya gagal: biarkan yang sudah termuat, pengguna
			// bisa menekan tombol lagi.
		} finally {
			loadingMore = false;
		}
	}

	function toggle(id: string) {
		openId = openId === id ? '' : id;
	}

	onMount(() => void load());

	// Sentinel di bawah tabel: menyentuh area itu otomatis memuat halaman
	// berikutnya (lazy load); dibongkar saat komponen hilang.
	$effect(() => {
		if (!sentinel) return;
		const io = new IntersectionObserver(
			(entries) => {
				if (entries.some((e) => e.isIntersecting)) void loadMore();
			},
			{ rootMargin: '240px' }
		);
		io.observe(sentinel);
		return () => io.disconnect();
	});

	// Salin semua baris yang sudah termuat; detail hanya untuk baris yang sedang dibuka.
	function toMarkdown(): string {
		const sections: string[] = [
			mdList([
				['Total run', stats.total],
				['Gagal', stats.failed],
				['Token masuk', stats.tokIn.toLocaleString('id-ID')],
				['Token keluar', stats.tokOut.toLocaleString('id-ID')],
				['Durasi rata-rata', `${stats.avgDur} dtk`],
				['Masih ada halaman berikutnya', nextBefore ? 'Ya' : 'Tidak']
			]),
			mdTable(
				['Waktu', 'Agent', 'Brand', 'Status', 'Model', 'Token (in → out)', 'Durasi', 'Run ID'],
				logs.map((l) => [
					l.created_at ? fmtDateTime(l.created_at) : '',
					agentLabel(l.agent),
					l.brand_name,
					l.status === 'failed' ? 'Gagal' : 'Sukses',
					l.model,
					`${(l.token_input ?? 0).toLocaleString('id-ID')} → ${(l.token_output ?? 0).toLocaleString('id-ID')}`,
					l.duration_ms ? `${Math.round(l.duration_ms / 1000)} dtk` : '',
					l.id
				])
			)
		];
		const open = logs.find((l) => l.id === openId);
		if (open) {
			sections.push(
				`## Detail run ${open.id}`,
				'### Bahan yang dikirim ke agent',
				mdCode(open.input_preview || '—'),
				'### Output agent',
				mdCode(open.status === 'failed' ? open.error || 'Tidak ada pesan error.' : open.output_preview || '—')
			);
		}
		return mdReport('Log Agent Matchmaker', ...sections);
	}
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
		>
			<CopyMarkdownButton build={toMarkdown} disabled={loading || logs.length === 0} />
		</PageHeader>

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
				{#if nextBefore}
					<div bind:this={sentinel} class="flex justify-center py-4">
						<button
							class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
							type="button"
							disabled={loadingMore}
							onclick={() => void loadMore()}
						>
							{loadingMore ? tr('Memuat…') : tr('Muat lebih banyak')}
						</button>
					</div>
				{:else if logs.length >= PAGE_SIZE}
					<p class="py-2 text-center text-[11px] text-muted-foreground">{tr('Semua run yang tersedia sudah dimuat.')}</p>
				{/if}
			{/if}
		</section>
		{/if}
	</ExplorerFrame>
</section>
