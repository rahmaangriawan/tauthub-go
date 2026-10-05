<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Notifikasi klien — tampilan split dua kolom: kiri "Umum" (notifikasi
	// biasa), kanan "Pengumuman" (broadcast admin). Filter status
	// (Semua/Belum dibaca/Dibaca) dipakai bersama; tiap kolom memuat halaman
	// sendiri lewat "Muat lagi". Polling ringan hanya saat berada di halaman
	// pertama kedua kolom.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { Bell, Check, X } from '@lucide/svelte';
	import { page } from '$app/state';
		import {
		notificationNavigationTarget,
		openNotificationLink,
		type NotificationNavigationTarget
	} from '$lib/navigation/open-notification-link';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import MultilineText from '$lib/components/multiline-text.svelte';

	const brandId = $derived(page.params.brandId as string);

	type Notification = {
		id: string; type: string; title: string; body: string | null; link: string | null;
		publication_id: string | null; read_at: string | null; expired_at: string | null; created_at: string;
	};

	const PAGE_SIZE = 30;
	let gRows = $state<Notification[]>([]);
	let gTotal = $state(0);
	let gUnread = $state(0);
	let gOffset = $state(0);
	let aRows = $state<Notification[]>([]);
	let aTotal = $state(0);
	let aUnread = $state(0);
	let aOffset = $state(0);
	let filter = $state('all');
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	// Popup detail notifikasi (bukan halaman baru) — klik baris cukup menampilkan
	// isi notifikasi itu saja; tautan (bila ada) dibuka dari tombol di popup.
	let active = $state<Notification | null>(null);
	let activeTarget = $state<NotificationNavigationTarget | null>(null);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadAll();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		// Polling ringan: hanya menyegarkan saat pengguna berada di halaman
		// pertama kedua kolom, supaya hasil "Muat lagi" tidak terganti tiba-tiba.
		const timer = window.setInterval(() => {
			if (gOffset === 0 && aOffset === 0 && !busy) void loadAll();
		}, 60_000);
		return () => {
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
			window.clearInterval(timer);
		};
	});

	async function fetchPage(category: 'general' | 'announcement', start: number) {
		const res = await apiRequest(
			`/v1/brands/${brandId}/notifications?category=${category}&status=${filter}&limit=${PAGE_SIZE}&offset=${start}`
		);
		if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat notifikasi.')));
		return (await res.json()) as { notifications: Notification[]; unread: number; total: number };
	}

	async function loadAll() {
		busy = true;
		error = '';
		try {
			const [g, a] = await Promise.all([fetchPage('general', 0), fetchPage('announcement', 0)]);
			gRows = g.notifications;
			gTotal = g.total;
			gUnread = g.unread;
			gOffset = 0;
			aRows = a.notifications;
			aTotal = a.total;
			aUnread = a.unread;
			aOffset = 0;
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal memuat notifikasi.');
		} finally {
			busy = false;
		}
	}

	async function loadMore(which: 'general' | 'announcement') {
		if (busy) return;
		busy = true;
		error = '';
		try {
			if (which === 'general') {
				const d = await fetchPage('general', gOffset + PAGE_SIZE);
				gRows = [...gRows, ...d.notifications];
				gTotal = d.total;
				gUnread = d.unread;
				gOffset += PAGE_SIZE;
			} else {
				const d = await fetchPage('announcement', aOffset + PAGE_SIZE);
				aRows = [...aRows, ...d.notifications];
				aTotal = d.total;
				aUnread = d.unread;
				aOffset += PAGE_SIZE;
			}
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal memuat notifikasi.');
		} finally {
			busy = false;
		}
	}

	function changeFilter(next: string) {
		if (filter === next) return;
		filter = next;
		void loadAll();
	}

	/** Klik baris → tampilkan detail di popup, sekaligus tandai dibaca. */
	async function openRow(notification: Notification) {
		active = notification;
		activeTarget = notificationNavigationTarget(notification.link, window.location.origin);
		if (notification.read_at) return;
		try {
			await apiRequest(`/v1/brands/${brandId}/notifications/${notification.id}/read`, { method: 'POST' });
			const updated = { ...notification, read_at: new Date().toISOString() };
			gRows = gRows.map((n) => (n.id === notification.id ? updated : n));
			aRows = aRows.map((n) => (n.id === notification.id ? updated : n));
			if (gRows.some((n) => n.id === notification.id)) gUnread = Math.max(0, gUnread - 1);
			else aUnread = Math.max(0, aUnread - 1);
		} catch {
			// Penandaan baca tidak boleh menghalangi pembukaan detail.
		}
	}

	function closeDetail() {
		active = null;
		activeTarget = null;
	}

	async function openActiveLink() {
		const n = active;
		if (!n || !activeTarget) return;
		await openNotificationLink({ link: n.link, push: (href) => goto(href), markRead: async () => {} });
		closeDetail();
	}

	function onWindowKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && active) {
			event.preventDefault();
			closeDetail();
		}
	}

	async function readAll() {
		await apiRequest(`/v1/brands/${brandId}/notifications`, { method: 'POST' });
		await loadAll();
	}

	const FILTERS: Array<[string, string]> = [
		['all', tr('Semua')],
		['unread', tr('Belum dibaca')],
		['read', 'Dibaca']
	];

	const fmtDate = (v: string | null) =>
		v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—';
</script>

{#snippet column(kind: 'general' | 'announcement', rows: Notification[], total: number, unreadCount: number)}
	{@const label = kind === 'general' ? tr('Umum') : tr('Pengumuman')}
	{@const emptyText = kind === 'general' ? tr('Tidak ada notifikasi untuk filter ini.') : tr('Belum ada pengumuman untuk filter ini.')}
	<div class="app-card flex flex-col p-4">
		<div class="flex items-center justify-between gap-2">
			<h2 class="text-sm font-semibold">{label}</h2>
			<span class="text-xs text-muted-foreground">{total}</span>
		</div>
		<div class="mt-1 min-h-[120px] divide-y">
			{#if loading || (busy && rows.length === 0)}
				<div class="space-y-2 py-3">
					<div class="h-10 animate-pulse rounded" style="background-color: var(--muted);"></div>
					<div class="h-10 animate-pulse rounded" style="background-color: var(--muted);"></div>
					<div class="h-10 animate-pulse rounded" style="background-color: var(--muted);"></div>
				</div>
			{:else if rows.length === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<Bell size={26} class="text-muted-foreground" />
					<p class="mt-3 text-sm text-muted-foreground">{emptyText}</p>
				</div>
			{:else}
				{#each rows as notification (notification.id)}
					<button
						type="button"
						class="flex w-full items-start gap-3 rounded-lg px-3 py-3 text-left transition-colors hover:bg-secondary"
						onclick={() => void openRow(notification)}
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
		{#if rows.length < total}
			<button
				class="mt-3 self-center text-xs font-medium underline-offset-2 hover:underline"
				style="color: var(--primary);"
				type="button"
				disabled={busy}
				onclick={() => void loadMore(kind)}
			>{busy ? tr('Memuat…') : tr('Muat lagi')}</button>
		{/if}
		{#if unreadCount > 0}
			<p class="mt-2 text-center text-[11px] text-muted-foreground">{unreadCount} {tr('belum dibaca')}</p>
		{/if}
	</div>
{/snippet}

<PageTitle title="Notifikasi" />

<svelte:window onkeydown={onWindowKeydown} />

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<p class="text-[11px] font-semibold tracking-wide text-muted-foreground">NOTIFICATIONS</p>
			<h1 class="mt-1 text-2xl font-semibold tracking-tight">Notifikasi</h1>
			<p class="mt-1 text-sm text-muted-foreground">{tr("Riwayat pemberitahuan persetujuan mitra dan penerbitan artikel Anda.")}</p>
		</div>
		{#if gUnread + aUnread > 0}
			<button class="app-btn app-btn-soft" type="button" onclick={() => void readAll()}>
				<Check size={15} /> Tandai semua dibaca
			</button>
		{/if}
	</header>

	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	<div class="app-card mt-4 flex flex-wrap items-center gap-2 p-4">
		{#each FILTERS as [value, label] (value)}
			<button
				class="app-btn !min-h-8 !px-3 !text-xs"
				class:app-btn-primary={filter === value}
				class:app-btn-soft={filter !== value}
				type="button"
				onclick={() => changeFilter(value)}
			>{label}</button>
		{/each}
		<span class="ml-auto text-xs text-muted-foreground">{gTotal + aTotal} {tr('notifikasi')} · {gUnread + aUnread} {tr('belum dibaca')}</span>
	</div>

	<div class="mt-4 grid items-start gap-4 lg:grid-cols-2">
		{@render column('general', gRows, gTotal, gUnread)}
		{@render column('announcement', aRows, aTotal, aUnread)}
	</div>

	{#if active}
		<div class="fixed inset-0 z-[1200] flex items-center justify-center p-4">
			<button type="button" class="absolute inset-0" style="background: rgba(15, 23, 42, 0.35);" aria-label={tr("Tutup")} onclick={closeDetail}></button>
			<div class="relative z-10 max-h-[85vh] w-full max-w-lg overflow-y-auto rounded-xl border p-5 shadow-lg" style="border-color: var(--border); background: var(--card, #fff);" role="dialog" aria-modal="true" aria-labelledby="notif-detail-title">
				<div class="flex items-start justify-between gap-3">
					<div class="min-w-0">
						<h2 id="notif-detail-title" class="text-base font-semibold leading-snug">{active.title}</h2>
						<p class="mt-1 text-[11px] text-muted-foreground">{fmtDate(active.created_at)}</p>
					</div>
					<button type="button" class="app-btn app-btn-soft !min-h-8 !w-8 !p-0" aria-label={tr("Tutup")} onclick={closeDetail}><X size={15} /></button>
				</div>

				{#if active.body}
					<MultilineText class="mt-3 break-words text-sm leading-relaxed text-muted-foreground" text={active.body} />
				{/if}

				<div class="mt-4 flex justify-end gap-2">
					{#if activeTarget}
						<button type="button" class="app-btn app-btn-primary" onclick={() => void openActiveLink()}>{tr("Buka tautan")}</button>
					{/if}
					<button type="button" class="app-btn app-btn-soft" onclick={closeDetail}>{tr("Tutup")}</button>
				</div>
			</div>
		</div>
	{/if}
</section>
