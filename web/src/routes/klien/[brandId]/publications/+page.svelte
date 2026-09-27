<script lang="ts">
	// Status Publikasi — daftar publikasi brand dengan status & mitra.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ExternalLink, Inbox } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);
	type Publication = {
		id: string; document_id: string; media_id: string; status: string; client_price: number;
		publish_status: string; published_at: string | null; published_url: string | null;
		document_title: string | null; media_name: string | null; created_at: string;
	};

	let items = $state<Publication[]>([]);
	let total = $state(0);
	let loading = $state(true);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			// Endpoint publikasi brand menyusul fase 6; sementara ambil dari overview
			// yang memang membawa 50 publikasi terbaru (baca-only, parity bentuk).
			const res = await apiRequest(`/v1/brands/${brandId}/overview`);
			if (res.ok) {
				const data = await res.json();
				items = data.publications ?? [];
				total = items.length;
			}
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const statusLabel: Record<string, string> = {
		pending: 'Menunggu Mitra', processing: 'Diproses', published: 'Terpublikasi',
		failed: 'Gagal', cancelled: 'Dibatalkan', queued: 'Diantrekan', ready: 'Siap'
	};
	const stateStyle = (s: string) =>
		s === 'published' ? 'background-color: var(--success-soft); color: var(--success);'
		: s === 'failed' ? 'background-color: color-mix(in srgb, var(--destructive) 12%, transparent); color: var(--destructive);'
		: s === 'pending' ? 'background-color: var(--warning-soft); color: var(--warning);'
		: 'background-color: var(--accent); color: var(--accent-foreground);';
	const fmt = (n: number) => n.toLocaleString('id-ID');
	const fmtDate = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID') : '—');
</script>

<section class="mx-auto max-w-6xl">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Status Publikasi</h1>
		<p class="mt-1 text-sm text-muted-foreground">
			Pantau progres penayangan artikel ke media mitra beserta hasil publikasinya.
		</p>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat publikasi…</p>
	{:else if items.length === 0}
		<div class="app-card mt-6 flex flex-col items-center px-6 py-12 text-center">
			<Inbox size={22} style="color: var(--primary);" />
			<p class="mt-2 text-sm font-medium">Belum ada publikasi.</p>
			<p class="mt-1 text-sm text-muted-foreground">
				Publikasi dibuat dari kampanye atau pemilihan media. Antrean muncul di sini setelah dikirim ke mitra.
			</p>
		</div>
	{:else}
		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="border-b text-left text-[11px] tracking-wide text-muted-foreground">
						<th class="px-4 py-2.5 font-semibold">ARTIKEL</th>
						<th class="px-4 py-2.5 font-semibold">MEDIA</th>
						<th class="px-4 py-2.5 font-semibold">STATUS (QC)</th>
						<th class="px-4 py-2.5 text-right font-semibold">KREDIT</th>
						<th class="px-4 py-2.5 font-semibold">TERBIT</th>
						<th class="px-4 py-2.5 font-semibold">URL</th>
					</tr>
				</thead>
				<tbody>
					{#each items as pub (pub.id)}
						<tr class="border-b last:border-0">
							<td class="px-4 py-2.5">
								<a class="max-w-[260px] truncate font-medium hover:underline" href={`/klien/${brandUrl}/editor/${pub.document_id}`}>
									{pub.document_title ?? 'Tanpa judul'}
								</a>
							</td>
							<td class="px-4 py-2.5 text-muted-foreground">{pub.media_name ?? '—'}</td>
							<td class="px-4 py-2.5">
								<span class="rounded-full px-2 py-0.5 text-xs font-semibold" style={stateStyle(pub.status)}>
									{statusLabel[pub.status] ?? pub.status}
								</span>
							</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{fmt(pub.client_price ?? 0)}</td>
							<td class="px-4 py-2.5 text-muted-foreground">{fmtDate(pub.published_at)}</td>
							<td class="px-4 py-2.5">
								{#if pub.published_url}
									<a class="inline-flex items-center gap-1 text-xs" style="color: var(--primary);" href={pub.published_url} target="_blank" rel="noreferrer">
										Buka <ExternalLink size={12} />
									</a>
								{:else}
									<span class="text-xs text-muted-foreground">—</span>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>
