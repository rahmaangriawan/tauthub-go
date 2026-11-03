<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Riwayat notifikasi admin — dua kolom: "Umum" (notifikasi operasional &
	// publikasi) dan "Pengumuman" (broadcast admin). Sumbernya tabel
	// notifications per user admin yang login, bukan feed masalah "saat ini" di
	// lonceng, jadi di sini riwayatnya benar-benar tersimpan.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { Bell, Check } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type Item = {
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

	const PAGE_SIZE = 30;
	let gRows = $state<Item[]>([]);
	let gTotal = $state(0);
	let gUnread = $state(0);
	let gOffset = $state(0);
	let aRows = $state<Item[]>([]);
	let aTotal = $state(0);
	let aUnread = $state(0);
	let aOffset = $state(0);
	let status = $state('all');
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');

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
		const params = new URLSearchParams({
			category,
			status,
			limit: String(PAGE_SIZE),
			offset: String(start)
		});
		const res = await apiRequest(`/v1/platform/notifications?${params}`);
		if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat notifikasi.')));
		const data = await res.json();
		return {
			rows: (data.notifications ?? []) as Item[],
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

	function changeStatus(next: string) {
		if (status === next) return;
		status = next;
		void loadAll();
	}

	async function openRow(n: Item) {
		if (!n.read_at) {
			await apiRequest(`/v1/platform/notifications/${n.id}/read`, { method: 'POST' });
			const updated = { ...n, read_at: new Date().toISOString() };
			gRows = gRows.map((x) => (x.id === n.id ? updated : x));
			aRows = aRows.map((x) => (x.id === n.id ? updated : x));
			if (gRows.some((x) => x.id === n.id)) gUnread = Math.max(0, gUnread - 1);
			else aUnread = Math.max(0, aUnread - 1);
		}
		const link = n.link;
		if (!link) return;
		if (/^https?:\/\//i.test(link)) window.open(link, '_blank', 'noopener');
		else void goto(link.startsWith('/') ? link : `/${link}`);
	}

	async function markAll() {
		busy = true;
		error = '';
		try {
			const res = await apiRequest('/v1/platform/notifications', { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menandai semua dibaca.'));
				return;
			}
			notice = tr('Semua notifikasi ditandai sudah dibaca.');
			await loadAll();
		} finally {
			busy = false;
		}
	}

	const fmtDate = (v: string | null) =>
		v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—';
</script>

{#snippet column(kind: 'general' | 'announcement', rows: Item[], total: number, unreadCount: number)}
	{@const label = kind === 'general' ? tr('Umum') : tr('Pengumuman')}
	{@const emptyText =
		kind === 'general'
			? tr('Tidak ada notifikasi untuk filter ini.')
			: tr('Belum ada pengumuman untuk filter ini.')}
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
				</div>
			{:else if rows.length === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<Bell size={26} class="text-muted-foreground" />
					<p class="mt-3 text-sm text-muted-foreground">{emptyText}</p>
				</div>
			{:else}
				{#each rows as n (n.id)}
					<button
						type="button"
						class="flex w-full items-start gap-3 rounded-lg px-3 py-3 text-left transition-colors hover:bg-secondary"
						onclick={() => void openRow(n)}
					>
						<span class="mt-1.5 h-2 w-2 shrink-0 rounded-full" style="background-color: {n.read_at ? 'transparent' : 'var(--primary)'};"></span>
						<span class="min-w-0 flex-1">
							<span class="block text-sm" style="font-weight: {n.read_at ? 500 : 700};">{n.title}</span>
							{#if n.body}<span class="mt-0.5 block text-xs text-muted-foreground">{n.body}</span>{/if}
						</span>
						<span class="shrink-0 text-[11px] text-muted-foreground">{fmtDate(n.created_at)}</span>
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

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="page-title">{tr('Notifikasi')}</h1>
			<p class="page-desc">
				{tr('Riwayat pemberitahuan operasional, publikasi, dan pengumuman untuk akun admin ini.')}
			</p>
		</div>
		{#if gUnread + aUnread > 0}
			<button class="app-btn app-btn-soft" disabled={busy} onclick={() => void markAll()}>
				<Check size={15} /> {tr('Tandai semua dibaca')}
			</button>
		{/if}
	</header>

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	<div class="mt-4 flex flex-wrap items-center gap-2">
		{#each [['all', tr('Semua')], ['unread', tr('Belum dibaca')], ['read', tr('Sudah dibaca')]] as [value, label] (value)}
			<button
				class="app-btn !min-h-8 !px-3 !text-xs"
				class:app-btn-primary={status === value}
				class:app-btn-soft={status !== value}
				type="button"
				onclick={() => changeStatus(value)}
			>{label}</button>
		{/each}
		<span class="ml-auto text-xs text-muted-foreground">{gTotal + aTotal} {tr('notifikasi')} · {gUnread + aUnread} {tr('belum dibaca')}</span>
	</div>

	<div class="mt-4 grid items-start gap-4 lg:grid-cols-2">
		{@render column('general', gRows, gTotal, gUnread)}
		{@render column('announcement', aRows, aTotal, aUnread)}
	</div>
</section>
