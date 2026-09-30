<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Sindikasi Konten mitra — port section "Sindikasi Konten" (SindikasiKontenSection):
	// status pengaktifan RSS mitra (diaktifkan Admin) dan status feed per media.
	import { onMount } from 'svelte';
	import { Rss, TriangleAlert } from '@lucide/svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type Feed = {
		id: string; media_name: string | null; name: string; item_status: string;
		install_status: string; install_error: string | null; installed_at: string | null;
		feed_url: string; created_at: string | null;
	};

	let rssEnabled = $state<boolean | null>(null);
	let feeds = $state<Feed[]>([]);
	let loading = $state(true);

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
			const meRes = await apiRequest('/v1/partner/me');
			const feedRes = await apiRequest('/v1/partner/rss-feeds');
			if (feedRes.ok) feeds = (await feedRes.json()).feeds ?? [];
			let enabled: boolean | null = null;
			if (meRes.ok) {
				const me = await meRes.json();
				const raw = me?.partner?.rss_enabled;
				if (typeof raw === 'boolean') enabled = raw;
			}
			// Flag partner-level hanya diisi Admin; bila tidak terekspos, status
			// diturunkan dari ada/tidaknya feed aktif milik mitra.
			rssEnabled = enabled ?? feeds.length > 0;
		} catch {
			rssEnabled = false;
		}
	}

	const INSTALL_LABEL: Record<string, string> = $derived({ verified: 'Terpasang', pending: tr('Belum dipasang'), failed: tr('Gagal verifikasi') });
	const installColor = (s: string) => (s === 'verified' ? 'var(--success)' : s === 'failed' ? 'var(--destructive)' : 'var(--warning)');
	const dt = (v: string | null) => (v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—');
</script>

<PageTitle title={tr('Sindikasi Konten')} />

<section>
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">{tr('Sindikasi Konten')}</h1>
		<p class="mt-1 text-sm text-muted-foreground">{tr("Sindikasi Konten menyalurkan artikel mitra melalui RSS feed ke mitra lain atau publisher eksternal.")}</p>
	</header>

	<div class="app-card mt-3 p-4">
		<div class="rounded-lg border p-3" style="border-color: var(--warning); color: var(--warning);">
			<div class="flex items-start gap-2">
				<TriangleAlert size={16} class="mt-0.5 shrink-0" />
				<p class="text-sm">{tr("Hanya mitra terpilih yang bisa mengaktifkan sindikasi konten.")}</p>
			</div>
		</div>
		<div class="mt-3 flex items-center gap-2">
			<span class="text-sm font-medium">Status:</span>
			{#if loading || rssEnabled === null}
				<span class="text-sm text-muted-foreground">{tr("Memuat…")}</span>
			{:else if rssEnabled}
				<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--success-soft); color: var(--success);">{tr("Aktif")}</span>
			{:else}
				<span class="rounded-full px-2 py-0.5 text-[11px] font-medium text-muted-foreground" style="background-color: var(--secondary);">{tr("Nonaktif")}</span>
			{/if}
		</div>
		<p class="mt-3 text-sm text-muted-foreground">{tr("Fitur ini masih dalam tahap pengembangan. Pengaktifan dilakukan oleh Admin Tauthub untuk mitra yang telah menyetujui ketentuan RSS feed.")}</p>
	</div>

	<div class="app-card mt-4 overflow-x-auto">
		<div class="flex items-center gap-2 border-b px-4 py-3">
			<Rss size={16} />
			<h2 class="text-sm font-semibold">{tr("Status Feed per Media")}</h2>
		</div>
		{#if !loading && feeds.length === 0}
			<div class="flex flex-col items-center gap-2 px-4 py-10 text-center">
				<Rss size={26} style="color: var(--muted-foreground);" />
				<p class="text-sm text-muted-foreground">{tr("Belum ada feed sindikasi untuk partner ini.")}</p>
			</div>
		{:else}
			<table class="w-full text-sm">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">Feed</th>
						<th class="px-4 py-2.5 font-medium">{tr("Media")}</th>
						<th class="px-4 py-2.5 font-medium">{tr("Status item")}</th>
						<th class="px-4 py-2.5 font-medium">{tr('Pemasangan')}</th>
						<th class="px-4 py-2.5 font-medium">{tr('Terverifikasi')}</th>
					</tr>
				</thead>
				<tbody>
					{#each feeds as row (row.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5 font-medium">{row.name}</td>
							<td class="px-4 py-2.5 text-muted-foreground">{row.media_name ?? tr('Semua media')}</td>
							<td class="px-4 py-2.5 text-muted-foreground">{row.item_status}</td>
							<td class="px-4 py-2.5">
								<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--secondary); color: {installColor(row.install_status)};">
									{INSTALL_LABEL[row.install_status] ?? row.install_status}
								</span>
								{#if row.install_error}<p class="mt-1 text-xs" style="color: var(--destructive);">{row.install_error}</p>{/if}
							</td>
							<td class="px-4 py-2.5 text-xs text-muted-foreground">{dt(row.installed_at)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</div>
</section>
