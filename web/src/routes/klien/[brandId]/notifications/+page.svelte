<script lang="ts">
	// Notifikasi klien — port section notifications: bell + daftar notifikasi
	// in-app dengan filter semua/belum dibaca/dibaca, paginasi, tandai dibaca.
	// Polling ringan 60 dtk (parity NotificationBell di shell).
	import { onMount } from 'svelte';
	import { Bell, Check } from '@lucide/svelte';
	import { page } from '$app/state';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);

	type Notification = {
		id: string; type: string; title: string; body: string | null; link: string | null;
		publication_id: string | null; read_at: string | null; expired_at: string | null; created_at: string;
	};

	const PAGE_SIZE = 30;
	let rows = $state<Notification[]>([]);
	let total = $state(0);
	let unread = $state(0);
	let filter = $state('all');
	let offset = $state(0);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load(filter, offset);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		// Polling ringan: status notifikasi tetap segar tanpa reload manual.
		const timer = window.setInterval(() => void load(filter, offset), 60_000);
		return () => {
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
			window.clearInterval(timer);
		};
	});

	async function load(status = 'all', start = 0) {
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/notifications?status=${status}&limit=${PAGE_SIZE}&offset=${start}`);
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal memuat notifikasi.'));
			const data = (await res.json()) as { notifications: Notification[]; unread: number; total: number };
			rows = data.notifications;
			unread = data.unread;
			total = data.total;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Gagal memuat notifikasi.';
		} finally {
			busy = false;
		}
	}

	/** Buka notifikasi: tandai dibaca lalu navigasi ke deep link bila ada. */
	async function open(notification: Notification) {
		if (!notification.read_at) {
			await apiRequest(`/v1/brands/${brandId}/notifications/${notification.id}/read`, { method: 'POST' });
			rows = rows.map((n) => (n.id === notification.id ? { ...n, read_at: new Date().toISOString() } : n));
			unread = Math.max(0, unread - 1);
		}
		const link = notification.link;
		if (!link) return;
		if (/^https?:\/\//i.test(link)) window.open(link, '_blank', 'noopener');
		else window.location.href = link.startsWith('/klien/') ? link : `/klien/${brandUrl}/publications`;
	}

	async function readAll() {
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/notifications`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal menandai semua dibaca.'));
			await load(filter, offset);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Gagal menandai semua dibaca.';
		}
	}

	function changeFilter(next: string) {
		filter = next;
		offset = 0;
		void load(next, 0);
	}

	const FILTERS: Array<[string, string]> = [
		['all', 'Semua'],
		['unread', 'Belum dibaca'],
		['read', 'Dibaca']
	];

	const fmtDate = (v: string | null) =>
		v ? new Date(v).toLocaleString('id-ID', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' }) : '—';
</script>

<section class="mx-auto max-w-4xl">
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<p class="text-[11px] font-semibold tracking-wide text-muted-foreground">NOTIFICATIONS</p>
			<h1 class="mt-1 text-2xl font-semibold tracking-tight">Notifikasi</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				Riwayat pemberitahuan persetujuan mitra dan penerbitan artikel Anda.
			</p>
		</div>
		{#if unread > 0}
			<button class="app-btn app-btn-soft" type="button" onclick={() => void readAll()}>
				<Check size={15} /> Tandai semua dibaca
			</button>
		{/if}
	</header>

	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	<div class="app-card mt-4 p-4">
		<div class="flex flex-wrap items-center gap-2">
			{#each FILTERS as [value, label] (value)}
				<button
					class="app-btn !min-h-8 !px-3 !text-xs"
					class:app-btn-primary={filter === value}
					class:app-btn-soft={filter !== value}
					type="button"
					onclick={() => changeFilter(value)}
				>{label}</button>
			{/each}
			<span class="ml-auto text-xs text-muted-foreground">{total} notifikasi · {unread} belum dibaca</span>
		</div>

		<div class="mt-3 divide-y">
			{#if loading || (busy && rows.length === 0)}
				<div class="space-y-2 py-3">
					<div class="h-10 animate-pulse rounded" style="background-color: var(--muted);"></div>
					<div class="h-10 animate-pulse rounded" style="background-color: var(--muted);"></div>
					<div class="h-10 animate-pulse rounded" style="background-color: var(--muted);"></div>
				</div>
			{:else if rows.length === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<Bell size={26} class="text-muted-foreground" />
					<p class="mt-3 text-sm text-muted-foreground">Tidak ada notifikasi untuk filter ini.</p>
				</div>
			{:else}
				{#each rows as notification (notification.id)}
					<button
						type="button"
						class="flex w-full items-start gap-3 py-3 text-left transition-colors hover:bg-secondary"
						onclick={() => void open(notification)}
					>
						<span class="mt-0.5 flex h-2 w-2 shrink-0 items-center justify-center rounded-full" style="background-color: {notification.read_at ? 'transparent' : 'var(--primary)'};"></span>
						<span class="min-w-0 flex-1">
							<span class="block text-sm" style="font-weight: {notification.read_at ? 500 : 700};">{notification.title}</span>
							{#if notification.body}<span class="mt-0.5 block text-xs text-muted-foreground">{notification.body}</span>{/if}
						</span>
						<span class="shrink-0 text-[11px] text-muted-foreground">{fmtDate(notification.created_at)}</span>
					</button>
				{/each}
			{/if}
		</div>

		{#if total > PAGE_SIZE}
			<div class="mt-3 flex flex-wrap items-center justify-between gap-2 border-t pt-3">
				<span class="text-xs text-muted-foreground">
					Menampilkan {offset + 1}–{Math.min(offset + PAGE_SIZE, total)} dari {total}
				</span>
				<div class="flex gap-2">
					<button class="app-btn app-btn-soft" type="button" disabled={offset === 0 || busy} onclick={() => { offset = Math.max(0, offset - PAGE_SIZE); void load(filter, offset); }}>Sebelumnya</button>
					<button class="app-btn app-btn-soft" type="button" disabled={offset + PAGE_SIZE >= total || busy} onclick={() => { offset += PAGE_SIZE; void load(filter, offset); }}>Berikutnya</button>
				</div>
			</div>
		{/if}
	</div>
</section>
