<script lang="ts">
	// Pelacak 404 klien: artikel hilang dalam masa garansi 12 bulan.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { CircleCheck, CircleX, HelpCircle, Loader } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);
	type Row = {
		id: string; document_title: string | null; media_name: string | null; published_url: string;
		published_at: string | null; state: string; http_status: number | null;
		consecutive_failures: number | null; error_message: string | null;
		last_checked_at: string | null; final_url: string | null;
	};

	let items = $state<Row[]>([]);
	let summary = $state<{ watched: number; live: number; gone: number; unreachable: number; no_url: number } | null>(null);
	let total = $state(0);
	let loading = $state(true);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			const [s, l] = await Promise.all([
				apiRequest(`/v1/brands/${brandId}/link-monitor?view=summary`),
				apiRequest(`/v1/brands/${brandId}/link-monitor?limit=100`)
			]);
			if (s.ok) summary = await s.json();
			if (l.ok) {
				const data = await l.json();
				items = data.items ?? [];
				total = data.total ?? 0;
			}
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const stateLabel: Record<string, string> = {
		live: 'Aman', gone: 'Hilang (404)', unreachable: 'Tidak terjangkau', no_url: 'Belum ada URL'
	};
	const stateStyle = (s: string) =>
		s === 'live' ? 'background-color: var(--success-soft); color: var(--success);'
		: s === 'gone' ? 'background-color: color-mix(in srgb, var(--destructive) 12%, transparent); color: var(--destructive);'
		: s === 'unreachable' ? 'background-color: var(--warning-soft); color: var(--warning);'
		: 'background-color: var(--muted); color: var(--muted-foreground);';
	const fmtDate = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID') : '—');
</script>

<section class="mx-auto max-w-6xl">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Pelacak 404</h1>
		<p class="mt-1 text-sm text-muted-foreground">
			Artikel terbit yang dipantau selama 12 bulan garansi. Pemeriksaan otomatis oleh worker.
		</p>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat status link…</p>
	{:else}
		{#if summary}
			<div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
				<div class="app-card flex items-center gap-3 p-4">
					<CircleCheck size={18} style="color: var(--success);" />
					<div><p class="text-xs text-muted-foreground">Aman</p><p class="text-xl font-bold tabular-nums">{summary.live}</p></div>
				</div>
				<div class="app-card flex items-center gap-3 p-4">
					<CircleX size={18} style="color: var(--destructive);" />
					<div><p class="text-xs text-muted-foreground">Hilang (404)</p><p class="text-xl font-bold tabular-nums">{summary.gone}</p></div>
				</div>
				<div class="app-card flex items-center gap-3 p-4">
					<Loader size={18} style="color: var(--warning);" />
					<div><p class="text-xs text-muted-foreground">Tidak terjangkau</p><p class="text-xl font-bold tabular-nums">{summary.unreachable}</p></div>
				</div>
				<div class="app-card flex items-center gap-3 p-4">
					<HelpCircle size={18} class="text-muted-foreground" />
					<div><p class="text-xs text-muted-foreground">Belum ada URL</p><p class="text-xl font-bold tabular-nums">{summary.no_url}</p></div>
				</div>
			</div>
		{/if}

		{#if items.length === 0}
			<div class="app-card mt-4 px-6 py-10 text-center text-sm text-muted-foreground">
				Belum ada artikel terbit dengan URL yang bisa dipantau. Pelacakan mulai berjalan setelah publikasi terbit.
			</div>
		{:else}
			<div class="app-card mt-4 overflow-x-auto">
				<table class="w-full text-sm">
					<thead>
						<tr class="border-b text-left text-[11px] tracking-wide text-muted-foreground">
							<th class="px-4 py-2.5 font-semibold">ARTIKEL</th>
							<th class="px-4 py-2.5 font-semibold">MEDIA</th>
							<th class="px-4 py-2.5 font-semibold">STATUS</th>
							<th class="px-4 py-2.5 font-semibold">TERAKHIR DIPERIKSA</th>
							<th class="px-4 py-2.5 font-semibold">CATATAN</th>
						</tr>
					</thead>
					<tbody>
						{#each items as row (row.id)}
							<tr class="border-b last:border-0">
								<td class="px-4 py-2.5">
									<p class="max-w-[260px] truncate font-medium">{row.document_title ?? '—'}</p>
									<a class="text-xs" style="color: var(--primary);" href={row.published_url} target="_blank" rel="noreferrer">Buka URL</a>
								</td>
								<td class="px-4 py-2.5 text-muted-foreground">{row.media_name ?? '—'}</td>
								<td class="px-4 py-2.5">
									<span class="rounded-full px-2 py-0.5 text-xs font-semibold" style={stateStyle(row.state)}>
										{stateLabel[row.state] ?? row.state}
									</span>
								</td>
								<td class="px-4 py-2.5 text-muted-foreground">{fmtDate(row.last_checked_at)}</td>
								<td class="px-4 py-2.5 text-xs text-muted-foreground">
									{row.http_status ? `HTTP ${row.http_status}` : ''}
									{#if row.error_message}<span class="ml-1">{row.error_message}</span>{/if}
									{#if (row.consecutive_failures ?? 0) > 0}<span class="ml-1">({row.consecutive_failures}× gagal)</span>{/if}
									{#if !row.http_status && !row.error_message}—{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{/if}
</section>
