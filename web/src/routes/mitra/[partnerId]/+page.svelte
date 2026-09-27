<script lang="ts">
	// Ringkasan mitra — port section "Ringkasan" portal mitra: kartu statistik
	// publikasi (menunggu, draf terkirim, terbit otomatis, gagal), media,
	// penghasilan terbaru, dan notifikasi terbaru.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Bell, CircleCheck, Coins, Newspaper, TriangleAlert, Upload } from '@lucide/svelte';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const partnerId = $derived(page.params.partnerId as string);
	const base = $derived(`/mitra/${getPartnerUrlId() || partnerId}`);

	let loading = $state(true);
	let me = $state<any>(null);
	let earnings = $state<any>(null);
	let notifications = $state<any[]>([]);
	let media = $state<any[]>([]);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			const [meRes, earnRes, notifRes, mediaRes] = await Promise.all([
				apiRequest('/v1/partner/me'),
				apiRequest('/v1/partner/earnings?limit=5'),
				apiRequest('/v1/partner/notifications?limit=5'),
				apiRequest('/v1/partner/media?limit=5')
			]);
			if (meRes.ok) me = (await meRes.json()).partner;
			if (earnRes.ok) earnings = await earnRes.json();
			if (notifRes.ok) notifications = (await notifRes.json()).notifications ?? [];
			if (mediaRes.ok) media = (await mediaRes.json()).media ?? [];
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const stats = $derived(me?.publication_stats ?? {});
	const fmt = (n: number) => (n ?? 0).toLocaleString('id-ID');
	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID')}`;
</script>

<section class="mx-auto max-w-6xl">
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{me?.name ?? 'Ringkasan Mitra'}</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				{me?.media_count ?? 0} media terdaftar{#if me?.public_id} · ID mitra {me.public_id}{/if}
			</p>
		</div>
		<div class="flex gap-2">
			<a class="app-btn app-btn-soft" href={`${base}/media`}>Kelola Media</a>
			<a class="app-btn app-btn-primary" href={`${base}/publications`}>Lihat Publikasi</a>
		</div>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat ringkasan…</p>
	{:else}
		<div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
			<div class="app-card p-4">
				<div class="flex items-center gap-2 text-xs font-semibold tracking-wide text-muted-foreground">
					<Bell size={14} /> MENUNGGU PERSETUJUAN
				</div>
				<p class="mt-2 text-2xl font-bold tabular-nums">{fmt(stats.waiting)}</p>
				<p class="mt-1 text-xs text-muted-foreground">Artikel butuh tindakan Anda</p>
			</div>
			<div class="app-card p-4">
				<div class="flex items-center gap-2 text-xs font-semibold tracking-wide text-muted-foreground">
					<Upload size={14} /> DRAF TERKIRIM
				</div>
				<p class="mt-2 text-2xl font-bold tabular-nums">{fmt(stats.draft_delivered)}</p>
				<p class="mt-1 text-xs text-muted-foreground">Menunggu Anda terbitkan</p>
			</div>
			<div class="app-card p-4">
				<div class="flex items-center gap-2 text-xs font-semibold tracking-wide text-muted-foreground">
					<CircleCheck size={14} /> TERBIT OTOMATIS
				</div>
				<p class="mt-2 text-2xl font-bold tabular-nums">{fmt(stats.auto_published)}</p>
				<p class="mt-1 text-xs text-muted-foreground">Dipublikasikan oleh integrasi</p>
			</div>
			<div class="app-card p-4">
				<div class="flex items-center gap-2 text-xs font-semibold tracking-wide text-muted-foreground">
					<TriangleAlert size={14} /> GAGAL
				</div>
				<p class="mt-2 text-2xl font-bold tabular-nums">{fmt(stats.failed)}</p>
				<p class="mt-1 text-xs text-muted-foreground">Perlu pengiriman ulang</p>
			</div>
		</div>

		<div class="mt-4 grid gap-3 lg:grid-cols-3">
			<div class="app-card p-4 lg:col-span-1">
				<div class="flex items-center gap-2 text-xs font-semibold tracking-wide text-muted-foreground">
					<Coins size={14} /> SALDO & PENGHASILAN
				</div>
				<p class="mt-2 text-2xl font-bold tabular-nums">{rp(earnings?.summary?.settled ?? 0)}</p>
				<dl class="mt-3 space-y-1.5 text-sm">
					<div class="flex justify-between"><dt class="text-muted-foreground">Menunggu cair</dt><dd class="tabular-nums">{rp(earnings?.summary?.pending ?? 0)}</dd></div>
					<div class="flex justify-between"><dt class="text-muted-foreground">Sudah dibayar</dt><dd class="tabular-nums">{rp(earnings?.summary?.paid_settlements ?? 0)}</dd></div>
					<div class="flex justify-between"><dt class="text-muted-foreground">Min. penarikan</dt><dd class="tabular-nums">{rp(earnings?.minimum_withdraw_idr ?? 0)}</dd></div>
				</dl>
				<a class="app-btn app-btn-soft mt-3 w-full justify-center" href={`${base}/earnings`}>Kelola Penghasilan</a>
			</div>

			<div class="app-card p-4 lg:col-span-1">
				<div class="flex items-center gap-2 text-xs font-semibold tracking-wide text-muted-foreground">
					<Newspaper size={14} /> MEDIA TERBARU
				</div>
				{#if media.length === 0}
					<p class="mt-3 text-sm text-muted-foreground">Belum ada media. <a class="font-medium" style="color: var(--primary);" href={`${base}/media`}>Tambah media</a></p>
				{:else}
					<ul class="mt-3 space-y-2 text-sm">
						{#each media as m (m.id)}
							<li class="flex items-center justify-between gap-2">
								<span class="truncate">{m.name}</span>
								<span class="shrink-0 text-xs text-muted-foreground">{m.status}</span>
							</li>
						{/each}
					</ul>
				{/if}
			</div>

			<div class="app-card p-4 lg:col-span-1">
				<div class="flex items-center gap-2 text-xs font-semibold tracking-wide text-muted-foreground">
					<Bell size={14} /> NOTIFIKASI
				</div>
				{#if notifications.length === 0}
					<p class="mt-3 text-sm text-muted-foreground">Belum ada notifikasi.</p>
				{:else}
					<ul class="mt-3 space-y-2.5 text-sm">
						{#each notifications as n (n.id)}
							<li>
								<p class="font-medium">{n.title}</p>
								{#if n.body}<p class="text-xs text-muted-foreground">{n.body.slice(0, 110)}</p>{/if}
							</li>
						{/each}
					</ul>
				{/if}
				<a class="app-btn app-btn-soft mt-3 w-full justify-center" href={`${base}/notifications`}>Semua Notifikasi</a>
			</div>
		</div>

		{#if Array.isArray(earnings?.earnings) && earnings.earnings.length > 0}
			<div class="app-card mt-4 overflow-hidden">
				<div class="border-b px-4 py-3">
					<h2 class="text-sm font-semibold">Penghasilan Terbaru</h2>
				</div>
				<table class="w-full text-sm">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="px-4 py-2 font-medium">Artikel</th>
							<th class="px-4 py-2 font-medium">Media</th>
							<th class="px-4 py-2 text-right font-medium">Nilai</th>
							<th class="px-4 py-2 font-medium">Status</th>
						</tr>
					</thead>
					<tbody>
						{#each earnings.earnings as e (e.id)}
							<tr class="border-t">
								<td class="max-w-[320px] truncate px-4 py-2">{e.document_title ?? '—'}</td>
								<td class="px-4 py-2 text-muted-foreground">{e.media_name ?? '—'}</td>
								<td class="px-4 py-2 text-right tabular-nums">{rp(e.amount)}</td>
								<td class="px-4 py-2"><span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{e.status}</span></td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{/if}
</section>
