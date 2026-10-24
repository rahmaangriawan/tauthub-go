<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import { apiRequest, readApiError } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';
	import { onMount } from 'svelte';

	// Tabel log run satu agent (atau beberapa agent sekaligus) dengan paginasi
	// keyset + lazy load. Dipakai bersama oleh halaman agent Taut Brief supaya
	// setiap agent punya tab Log sendiri — admin tak perlu mencari ke journal
	// server saat agent gagal.
	let { agents, emptyText = '' }: { agents: string[]; emptyText?: string } = $props();

	type RunLog = {
		id: string;
		agent: string;
		brand_name: string;
		profile_name: string;
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

	const BASE = '/v1/platform/taut-brief';
	const PAGE_SIZE = 50;

	let logs = $state<RunLog[]>([]);
	let loading = $state(true);
	let loadingMore = $state(false);
	let nextBefore = $state('');
	let error = $state('');
	let openId = $state('');
	let sentinel = $state<HTMLDivElement | null>(null);

	const agentQuery = $derived(agents.join(','));
	const allow = $derived(new Set(agents));

	async function fetchPage(before: string): Promise<{ items: RunLog[]; next: string }> {
		const qs = `agent=${encodeURIComponent(agentQuery)}&limit=${PAGE_SIZE}` +
			(before ? `&before=${encodeURIComponent(before)}` : '');
		const res = await apiRequest(`${BASE}/logs?${qs}`);
		if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat log.')));
		const data = await res.json();
		// Server sudah menyaring via ?agent=; saringan ini menjaga tampilan tetap
		// benar bila API yang dilayani masih versi lama.
		const items = ((data?.items ?? []) as RunLog[]).filter((r) => allow.has(r.agent));
		return { items, next: data?.next_before ?? '' };
	}

	async function load() {
		loading = true;
		error = '';
		nextBefore = '';
		try {
			const page = await fetchPage('');
			logs = page.items;
			nextBefore = page.next;
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
			const page = await fetchPage(nextBefore);
			const seen = new Set(logs.map((l) => l.id));
			logs = [...logs, ...page.items.filter((l) => !seen.has(l.id))];
			nextBefore = page.next;
		} catch {
			// biarkan yang sudah termuat; tombol bisa ditekan lagi
		} finally {
			loadingMore = false;
		}
	}

	onMount(() => void load());

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
</script>

<section class="app-card mt-4 p-5">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<div>
			<h2 class="text-sm font-semibold">{tr('Log pemakaian')}</h2>
			<p class="mt-0.5 text-[11px] text-muted-foreground">
				{tr('Token, durasi, error, dan preview bahan/output tiap run agent ini.')}
			</p>
		</div>
		<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={loading} onclick={() => void load()}>
			{tr('Muat ulang')}
		</button>
	</div>

	{#if error}
		<p class="mt-3 text-sm" style="color: var(--destructive);">{error}</p>
	{:else if loading}
		<p class="mt-3 text-sm text-muted-foreground">{tr('Memuat log…')}</p>
	{:else if logs.length === 0}
		<p class="mt-3 text-sm text-muted-foreground">
			{emptyText || tr('Belum ada run untuk agent ini.')}
		</p>
	{:else}
		<div class="mt-3 overflow-x-auto">
			<table class="w-full text-left text-xs">
				<thead class="text-muted-foreground">
					<tr>
						<th class="py-1 pr-3">Waktu</th>
						<th class="py-1 pr-3">Brand</th>
						<th class="py-1 pr-3">Brief</th>
						<th class="py-1 pr-3">Status</th>
						<th class="py-1 pr-3">Model</th>
						<th class="py-1 pr-3">Token</th>
						<th class="py-1 pr-3">Durasi</th>
					</tr>
				</thead>
				<tbody>
					{#each logs as r (r.id)}
						{@const failed = r.status === 'failed'}
						<tr class="cursor-pointer border-t hover:bg-secondary/50" style="border-color: var(--border);" onclick={() => (openId = openId === r.id ? '' : r.id)}>
							<td class="py-1.5 pr-3 whitespace-nowrap">{r.created_at ? fmtDateTime(r.created_at) : '—'}</td>
							<td class="py-1.5 pr-3">{r.brand_name || '—'}</td>
							<td class="py-1.5 pr-3">{r.profile_name || '—'}</td>
							<td class="py-1.5 pr-3">
								<span style="color: {failed ? 'var(--destructive)' : r.status === 'done' ? 'var(--success)' : 'var(--muted-foreground)'};">{r.status}</span>
							</td>
							<td class="py-1.5 pr-3">{r.model || '—'}</td>
							<td class="py-1.5 pr-3">{r.token_input ?? 0} / {r.token_output ?? 0}</td>
							<td class="py-1.5 pr-3 whitespace-nowrap">{r.duration_ms ? `${Math.round(r.duration_ms / 1000)} dtk` : '—'}</td>
						</tr>
						{#if openId === r.id}
							<tr class="border-t bg-secondary/30" style="border-color: var(--border);">
								<td colspan="7" class="py-3 pr-3">
									<div class="grid gap-3 md:grid-cols-2">
										<div>
											<p class="mb-1 font-semibold">{tr('Bahan yang dikirim ke agent')}</p>
											<pre class="max-h-56 overflow-auto whitespace-pre-wrap break-words rounded-md bg-background p-2 text-[11px] text-muted-foreground">{r.input_preview || '—'}</pre>
										</div>
										<div>
											<p class="mb-1 font-semibold">{failed ? tr('Pesan error') : tr('Output agent')}</p>
											{#if failed}
												<p class="max-h-56 overflow-auto whitespace-pre-wrap break-words rounded-md p-2 text-[11px]" style="background: var(--warning-soft); color: var(--warning);">{r.error || tr('Tidak ada pesan error.')}</p>
											{:else}
												<pre class="max-h-56 overflow-auto whitespace-pre-wrap break-words rounded-md bg-background p-2 text-[11px] text-muted-foreground">{r.output_preview || '—'}</pre>
											{/if}
										</div>
									</div>
								</td>
							</tr>
						{/if}
					{/each}
				</tbody>
			</table>
		</div>
		{#if nextBefore}
			<div bind:this={sentinel} class="flex justify-center py-3">
				<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={loadingMore} onclick={() => void loadMore()}>
					{loadingMore ? tr('Memuat…') : tr('Muat lebih banyak')}
				</button>
			</div>
		{/if}
	{/if}
</section>
