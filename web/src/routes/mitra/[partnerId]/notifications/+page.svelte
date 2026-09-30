<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Notifikasi mitra — tampilan split dua kolom: kiri "Umum" (notifikasi
	// biasa), kanan "Pengumuman" (broadcast admin). Filter status
	// (Semua/Belum dibaca/Sudah dibaca) dipakai bersama; tiap kolom punya
	// "Muat lagi" sendiri. Klik baris membuka popup detail + menandai dibaca.
	// Section webhook sengaja disembunyikan (parity app lama).
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import {
		notificationNavigationTarget,
		openNotificationLink,
		type NotificationNavigationTarget
	} from '$lib/navigation/open-notification-link';
	import { Bell, Check, X } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type NotificationRow = {
		id: string;
		type: string;
		title: string;
		body: string | null;
		link: string | null;
		publication_id: string | null;
		read_at: string | null;
		expired_at: string | null;
		created_at: string | null;
	};

	const PAGE_SIZE = 20;
	let gRows = $state<NotificationRow[]>([]);
	let gTotal = $state(0);
	let gUnread = $state(0);
	let gOffset = $state(0);
	let aRows = $state<NotificationRow[]>([]);
	let aTotal = $state(0);
	let aUnread = $state(0);
	let aOffset = $state(0);
	let statusFilter = $state('');
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');
	// Popup detail notifikasi (bukan halaman baru).
	let active = $state<NotificationRow | null>(null);
	// Tautan notifikasi yang sudah divalidasi (internal/eksternal). Null bila tidak ada.
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
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function fetchPage(category: 'general' | 'announcement', start: number) {
		const params = new URLSearchParams({ limit: String(PAGE_SIZE), offset: String(start), category });
		if (statusFilter) params.set('status', statusFilter);
		const res = await apiRequest(`/v1/partner/notifications?${params}`);
		if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat notifikasi.')));
		const data = await res.json();
		return {
			rows: (data.notifications ?? []) as NotificationRow[],
			unread: Number(data.unread ?? 0),
			total: Number(data.total ?? 0)
		};
	}

	async function loadAll() {
		busy = true;
		error = '';
		try {
			const [g, a] = await Promise.all([fetchPage('general', 0), fetchPage('announcement', 0)]);
			gRows = g.rows;
			gTotal = g.total;
			gUnread = g.unread;
			gOffset = 0;
			aRows = a.rows;
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
				gRows = [...gRows, ...d.rows];
				gTotal = d.total;
				gUnread = d.unread;
				gOffset += PAGE_SIZE;
			} else {
				const d = await fetchPage('announcement', aOffset + PAGE_SIZE);
				aRows = [...aRows, ...d.rows];
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

	function changeFilter(value: string) {
		if (statusFilter === value) return;
		statusFilter = value;
		void loadAll();
	}

	/** Klik baris → tampilkan detail di popup, sekaligus tandai dibaca. */
	async function openRow(n: NotificationRow) {
		error = '';
		active = n;
		activeTarget = notificationNavigationTarget(n.link, window.location.origin);
		if (n.read_at) return;
		try {
			await apiRequest(`/v1/partner/notifications/${n.id}/read`, { method: 'POST' });
			const updated = { ...n, read_at: new Date().toISOString() };
			gRows = gRows.map((x) => (x.id === n.id ? updated : x));
			aRows = aRows.map((x) => (x.id === n.id ? updated : x));
			if (gRows.some((x) => x.id === n.id)) gUnread = Math.max(0, gUnread - 1);
			else aUnread = Math.max(0, aUnread - 1);
		} catch {
			// Pelacakan baca tidak boleh menghalangi pembukaan detail.
		}
	}

	function closeDetail() {
		active = null;
		activeTarget = null;
	}

	/** Hanya dipanggil saat notifikasi punya tautan valid (lihat `activeTarget`). */
	async function openActiveLink() {
		const n = active;
		if (!n || !activeTarget) return;
		await openNotificationLink({
			link: n.link,
			push: (href) => goto(href),
			markRead: async () => {
				await apiRequest(`/v1/partner/notifications/${n.id}/read`, { method: 'POST' });
			}
		});
		closeDetail();
	}

	function onWindowKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && active) {
			event.preventDefault();
			closeDetail();
		}
	}

	async function markAll() {
		busy = true;
		error = '';
		try {
			const res = await apiRequest('/v1/partner/notifications', { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menandai semua notifikasi.'));
				return;
			}
			notice = tr('Semua notifikasi ditandai sudah dibaca.');
			await loadAll();
		} finally {
			busy = false;
		}
	}

	// timeAgo: port app lama — menit/jam/hari lalu, di atas 7 hari pakai tanggal.
	function timeAgo(value: string | null) {
		if (!value) return '—';
		const seconds = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 1000));
		if (seconds < 60) return tr('baru saja');
		const minutes = Math.floor(seconds / 60);
		if (minutes < 60) return `${minutes} mnt lalu`;
		const hours = Math.floor(minutes / 60);
		if (hours < 24) return `${hours} jam lalu`;
		const days = Math.floor(hours / 24);
		if (days < 7) return `${days} hari lalu`;
		return new Date(value).toLocaleDateString('id-ID', { dateStyle: 'medium' });
	}
</script>

{#snippet column(kind: 'general' | 'announcement', rows: NotificationRow[], total: number, unreadCount: number)}
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
				{#each rows as n (n.id)}
					<button class="flex w-full items-start gap-3 rounded-lg px-3 py-3 text-left transition-colors hover:bg-secondary" onclick={() => openRow(n)} aria-haspopup="dialog">
						<span class="mt-1.5 h-2 w-2 shrink-0 rounded-full" style="background-color: {!n.read_at && !n.expired_at ? 'var(--primary)' : 'transparent'};"></span>
						<span class="min-w-0 flex-1">
							<span class="flex flex-wrap items-center gap-2">
								<strong class="text-sm" style:font-weight={n.read_at ? 500 : 700} style:opacity={n.expired_at ? 0.65 : 1}>{n.title}</strong>
								{#if n.expired_at}<span class="rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--secondary); color: var(--muted-foreground);">{tr("Kedaluwarsa")}</span>{/if}
							</span>
							{#if n.body}<span class="mt-0.5 block text-xs text-muted-foreground">{n.body}</span>{/if}
							<span class="mt-0.5 block text-[11px] text-muted-foreground">{timeAgo(n.created_at)}</span>
						</span>
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

<PageTitle title={tr('Notifikasi')} />

<svelte:window onkeydown={onWindowKeydown} />

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{tr('Notifikasi')}</h1>
			<p class="mt-1 text-sm text-muted-foreground">{gUnread + aUnread} {tr("notifikasi belum dibaca")}.</p>
		</div>
		<button class="app-btn app-btn-soft" disabled={busy} onclick={markAll}><Check size={15} /> {tr("Tandai semua dibaca")}</button>
	</header>

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	<div class="mt-4 flex flex-wrap items-center gap-2">
		{#each [['', tr('Semua')], ['unread', tr('Belum dibaca')], ['read', tr('Sudah dibaca')]] as [value, label] (value)}
			<button
				class="app-btn !min-h-8 !px-3 !text-xs"
				class:app-btn-primary={statusFilter === value}
				class:app-btn-soft={statusFilter !== value}
				onclick={() => changeFilter(value)}
			>{label}</button>
		{/each}
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
						<div class="mt-1 flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
							<span>{timeAgo(active.created_at)}</span>
							{#if active.expired_at}
								<span class="rounded-full px-2 py-0.5" style="background-color: var(--secondary); color: var(--muted-foreground);">{tr("Kedaluwarsa")}</span>
							{/if}
						</div>
					</div>
					<button type="button" class="app-btn app-btn-soft !min-h-8 !w-8 !p-0" aria-label={tr("Tutup")} onclick={closeDetail}><X size={15} /></button>
				</div>

				{#if active.body}
					<p class="mt-3 whitespace-pre-line break-words text-sm leading-relaxed text-muted-foreground">{active.body}</p>
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
