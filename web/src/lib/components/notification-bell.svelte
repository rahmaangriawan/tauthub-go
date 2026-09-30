<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Lonceng notifikasi (klien / mitra / admin) — port notification-bell.tsx.
	// `apiBase` prefix API ter-scope (`/v1/partner`, `/v1/brands/{id}` atau
	// `/v1/platform/operations`); `fallbackBase` portal bila notifikasi tanpa
	// deep link. `readable=false` (feed agregat admin) menonaktifkan mark-read
	// server dan menyimpan status baca di localStorage.
	import { goto } from '$app/navigation';
	import { Bell, Check, TriangleAlert } from '@lucide/svelte';
	import { apiRequest, readApiError } from '$lib/auth';
	import { openNotificationLink } from '$lib/navigation/open-notification-link';

	type Notification = {
		id: string;
		type: string;
		title: string;
		body: string | null;
		link: string | null;
		read_at: string | null;
		expired_at: string | null;
		created_at: string;
	};

	let {
		apiBase,
		fallbackBase,
		readable = true,
		showAnnouncementTab = false,
		// true: server menyaring kategori (klien/mitra). false: feed tak mendukung
		// parameter kategori (lonceng admin) — penyaringan dilakukan di klien.
		serverCategory = true
	}: {
		apiBase: string;
		fallbackBase: string;
		readable?: boolean;
		showAnnouncementTab?: boolean;
		serverCategory?: boolean;
	} = $props();

	let items = $state<Notification[]>([]);
	let unread = $state(0);
	// Badge lonceng = unread GLOBAL (semua kategori); `unread` di atas mengikuti
	// tab yang sedang aktif.
	let badgeUnread = $state(0);
	let hasMore = $state(false);
	let error = $state('');
	let open = $state(false);
	// Tab kategori (hanya saat showAnnouncementTab): "Umum" = selain pengumuman.
	let category = $state<'general' | 'announcement'>('general');
	let readIds = $state<Set<string>>(new Set());
	let trigger = $state<HTMLButtonElement | null>(null);
	let menu = $state<HTMLDivElement | null>(null);
	let pos = $state({ left: 0, top: 0 });
	let placement = $state<'down' | 'up'>('down');
	let menuMaxHeight = $state(440);

	function timeAgo(value: string) {
		const seconds = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 1000));
		if (seconds < 60) return 'baru saja';
		const minutes = Math.floor(seconds / 60);
		if (minutes < 60) return `${minutes} mnt lalu`;
		const hours = Math.floor(minutes / 60);
		if (hours < 24) return `${hours} jam lalu`;
		const days = Math.floor(hours / 24);
		if (days < 7) return `${days} hari lalu`;
		return new Date(value).toLocaleDateString('id-ID', { dateStyle: 'medium' });
	}

	// Status baca admin disimpan lokal (feed agregat tanpa tracking per-user).
	function readStateKey() {
		return `tauthub:notif-read:${apiBase}`;
	}

	function loadReadState(): Set<string> {
		try {
			const raw = window.localStorage.getItem(readStateKey());
			if (!raw) return new Set();
			const arr = JSON.parse(raw) as unknown;
			if (!Array.isArray(arr)) return new Set();
			return new Set(arr.filter((v): v is string => typeof v === 'string'));
		} catch {
			return new Set();
		}
	}

	function saveReadState(ids: Set<string>) {
		try {
			window.localStorage.setItem(readStateKey(), JSON.stringify(Array.from(ids).slice(-200)));
		} catch {
			// localStorage penuh / tidak tersedia — best-effort
		}
	}

	async function load() {
		try {
			// Dengan tab kategori: badge di-poll dari hitungan global, daftar hanya
			// di-refresh saat menu terbuka (mengikuti tab aktif).
			if (showAnnouncementTab) {
				await loadBadge();
				if (open) await loadList();
				return;
			}
			const res = await apiRequest(`${apiBase}/notifications?limit=20`, { cache: 'no-store' });
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal memuat notifikasi.'));
			const data = (await res.json()) as { notifications: Notification[]; unread: number };
			// Notifikasi kedaluwarsa (media mitra diarsipkan/dilepas) tidak relevan.
			const expired = data.notifications.filter((n) => !n.expired_at);
			const currentRead = readable ? new Set<string>() : loadReadState();
			const visible = expired.filter((n) => !currentRead.has(n.id));
			readIds = currentRead;
			items = visible.slice(0, 3);
			hasMore = visible.length > 3;
			unread = readable ? data.unread : visible.length;
			badgeUnread = unread;
			error = '';
		} catch (reason) {
			error = reason instanceof Error ? reason.message : 'Gagal memuat notifikasi.';
		}
	}

	// Hitungan badge GLOBAL (semua kategori) — satu query murah, dipakai poll
	// 60 dtk dan saat lonceng dibuka.
	async function loadBadge() {
		try {
			const badge = await apiRequest(`${apiBase}/notifications?limit=1`, { cache: 'no-store' });
			if (!badge.ok) return;
			const data = (await badge.json()) as { unread: number };
			badgeUnread = data.unread ?? 0;
		} catch {
			// Badge tidak kritis: biarkan nilai terakhir.
		}
	}

	// Daftar menu ter-filter per tab. Dengan serverCategory penyaringan lewat
	// parameter kategori; tanpanya (feed admin) disaring di klien berdasarkan
	// prefiks type announcement.*.
	async function loadList() {
		try {
			const url = serverCategory
				? `${apiBase}/notifications?limit=20&category=${category}`
				: `${apiBase}/notifications?limit=20`;
			const res = await apiRequest(url, { cache: 'no-store' });
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal memuat notifikasi.'));
			const data = (await res.json()) as { notifications: Notification[]; unread: number };
			const expired = data.notifications.filter((n) => !n.expired_at);
			const currentRead = readable ? new Set<string>() : loadReadState();
			let visible = expired.filter((n) => !currentRead.has(n.id));
			if (!serverCategory) {
				visible = visible.filter((n) => (category === 'announcement') === n.type.startsWith('announcement'));
			}
			readIds = currentRead;
			items = visible.slice(0, 3);
			hasMore = visible.length > 3;
			unread = readable ? data.unread : visible.length;
			error = '';
		} catch (reason) {
			error = reason instanceof Error ? reason.message : 'Gagal memuat notifikasi.';
		}
	}

	function switchCategory(next: 'general' | 'announcement') {
		if (category === next) return;
		category = next;
		void loadList();
	}

	$effect(() => {
		void load();
		const timer = window.setInterval(() => void load(), 60_000);
		return () => window.clearInterval(timer);
	});

	function persist(ids: Set<string>) {
		readIds = new Set(ids);
		if (!readable) saveReadState(ids);
	}

	async function markRead(id: string, link: string | null, type = '') {
		// Pengumuman tidak punya deep link ke halaman lain — tujuannya halaman
		// notifikasi portal masing-masing; feed admin membawa tautannya sendiri
		// (halaman Kotak Pengumuman).
		const target = type.startsWith('announcement')
			? link || `${fallbackBase}/notifications`
			: link;
		await openNotificationLink({
			link: target,
			fallbackLink: `${fallbackBase}/publications`,
			push: (href) => goto(href),
			markRead: async () => {
				if (readable) {
					await apiRequest(`${apiBase}/notifications/${id}/read`, { method: 'POST' });
					items = items.map((n) =>
						n.id === id ? { ...n, read_at: new Date().toISOString() } : n
					);
				} else {
					const next = new Set(readIds);
					next.add(id);
					persist(next);
					items = items.filter((n) => n.id !== id);
				}
				unread = Math.max(0, unread - 1);
				badgeUnread = Math.max(0, badgeUnread - 1);
			}
		});
	}

	async function readAll() {
		if (readable) {
			try {
				await apiRequest(`${apiBase}/notifications`, { method: 'POST' });
			} catch (reason) {
				error = reason instanceof Error ? reason.message : 'Gagal menandai semua dibaca.';
				return;
			}
			items = items.map((n) => ({ ...n, read_at: n.read_at ?? new Date().toISOString() }));
			unread = 0;
			badgeUnread = 0;
			// Tandai-semua menyapu semua kategori; muat ulang agar konsisten.
			if (showAnnouncementTab) void load();
		} else {
			const next = new Set(readIds);
			for (const n of items) next.add(n.id);
			persist(next);
			items = [];
			hasMore = false;
			unread = 0;
		}
	}

	function toggle() {
		if (open) {
			open = false;
			return;
		}
		// Saat dibuka: daftar mengikuti tab aktif, dan badge di-refresh juga —
		// tanpa ini, ping pengumuman baru tidak memunculkan angka merah sampai
		// polling 60 dtk berjalan.
		if (showAnnouncementTab) {
			void loadList();
			void loadBadge();
		} else {
			void load();
		}
		const rect = trigger?.getBoundingClientRect();
		const width = Math.min(400, window.innerWidth - 32);
		// Drop-up bila ruang di bawah tidak cukup untuk daftar (360px) —
		// supaya menu tidak terpotong di bawah viewport.
		const gap = 8;
		const estimated = Math.min(440, window.innerHeight - 24);
		const spaceBelow = window.innerHeight - (rect?.bottom ?? 0) - gap - 8;
		const spaceAbove = (rect?.top ?? 0) - gap - 8;
		const placeUp = spaceBelow < Math.min(estimated, 240) && spaceAbove > spaceBelow;
		placement = placeUp ? 'up' : 'down';
		menuMaxHeight = Math.max(160, Math.min(estimated, placeUp ? spaceAbove : spaceBelow));
		pos = {
			left: rect ? Math.max(8, rect.right - width) : 8,
			top: placeUp && rect ? rect.top - gap - menuMaxHeight : (rect?.bottom ?? 0) + gap
		};
		open = true;
	}

	$effect(() => {
		if (!open) return;
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape') open = false;
		};
		// Tutup hanya bila halaman yang di-scroll; scroll di dalam daftar
		// notifikasi harus tetap bisa (menu punya area scroll sendiri).
		const onScroll = (event: Event) => {
			const target = event.target as Node | null;
			if (target && menu && (target === menu || menu.contains(target))) return;
			open = false;
		};
		const onResize = () => (open = false);
		window.addEventListener('keydown', onKey);
		window.addEventListener('scroll', onScroll, true);
		window.addEventListener('resize', onResize);
		return () => {
			window.removeEventListener('keydown', onKey);
			window.removeEventListener('scroll', onScroll, true);
			window.removeEventListener('resize', onResize);
		};
	});
</script>

<button
	bind:this={trigger}
	type="button"
	class="notif-trigger"
	aria-label={`Notifikasi${badgeUnread > 0 ? ` (${badgeUnread} belum dibaca)` : ''}`}
	aria-expanded={open}
	onclick={toggle}
>
	<span class="notif-bell" class:notif-bell-unread={badgeUnread > 0}><Bell size={19} /></span>
	{#if badgeUnread > 0}
		<span class="notif-badge">{badgeUnread > 9 ? '9+' : badgeUnread}</span>
	{/if}
</button>

{#if open}
	<button type="button" class="notif-backdrop" aria-label={tr("Tutup notifikasi")} onclick={() => (open = false)}
	></button>
	<div
		bind:this={menu}
		class="notif-menu"
		class:notif-menu-up={placement === 'up'}
		role="menu"
		aria-label="Notifikasi"
		style="left: {pos.left}px; top: {pos.top}px; width: min(400px, calc(100vw - 32px)); max-height: {menuMaxHeight}px;"
	>
		<div class="notif-header">
			<span class="notif-title">Notifikasi</span>
			{#if unread > 0}
				<button class="notif-readall" type="button" onclick={() => void readAll()}>
					<Check size={13} />Telah dibaca semua
				</button>
			{/if}
		</div>
		{#if showAnnouncementTab}
			<div class="notif-tabs" role="tablist" aria-label={tr('Kategori notifikasi')}>
				<button class="notif-tab" class:notif-tab-active={category === 'general'} role="tab" aria-selected={category === 'general'} type="button" onclick={() => switchCategory('general')}>{tr('Umum')}</button>
				<button class="notif-tab" class:notif-tab-active={category === 'announcement'} role="tab" aria-selected={category === 'announcement'} type="button" onclick={() => switchCategory('announcement')}>{tr('Pengumuman')}</button>
			</div>
		{/if}
		<div class="notif-divider"></div>
		{#if error}
			<p class="notif-error"><TriangleAlert size={12} /> {error}</p>
		{/if}
		<div class="notif-list">
			{#if items.length === 0 && !error}
				<p class="notif-empty">{tr("Belum ada notifikasi.")}</p>
			{/if}
			{#each items as notification (notification.id)}
				<button
					class="notif-item"
					type="button"
					onclick={() => {
						open = false;
						void markRead(notification.id, notification.link, notification.type);
					}}
				>
					{#if !notification.read_at}<span class="notif-dot"></span>{/if}
					<span class="notif-item-body">
						<span class="notif-item-title" class:notif-item-title-unread={!notification.read_at}
							>{notification.title}</span
						>
						{#if notification.body}
							<span class="notif-item-text" title={notification.body}>{notification.body}</span>
						{/if}
						<span class="notif-item-time">{timeAgo(notification.created_at)}</span>
					</span>
				</button>
			{/each}
		</div>
		{#if hasMore}
			<div class="notif-footer">
				<a href={`${fallbackBase}/notifications`} onclick={() => (open = false)}>{tr("Lihat semua →")}</a>
			</div>
		{/if}
	</div>
{/if}

<style>
	.notif-trigger {
		position: relative;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 34px;
		height: 34px;
		border: none;
		border-radius: 8px;
		background: var(--secondary, #eef2f8);
		color: var(--foreground);
		cursor: pointer;
		transition: background-color 0.15s ease;
		touch-action: manipulation;
	}
	.notif-trigger:hover {
		background: color-mix(in srgb, var(--secondary, #eef2f8) 80%, var(--border));
	}
	.notif-trigger:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}
	.notif-bell {
		display: inline-flex;
		align-items: center;
		justify-content: center;
	}
	.notif-bell-unread {
		fill: currentColor;
	}
	.notif-badge {
		position: absolute;
		top: -2px;
		right: -2px;
		display: flex;
		align-items: center;
		justify-content: center;
		min-width: 16px;
		height: 16px;
		padding: 0 4px;
		border-radius: 999px;
		background: var(--destructive, #dc2626);
		color: #ffffff;
		font-size: 10px;
		font-weight: 700;
	}

	.notif-backdrop {
		position: fixed;
		inset: 0;
		z-index: 1090;
		border: none;
		padding: 0;
		background: transparent;
		cursor: default;
	}
	.notif-menu {
		position: fixed;
		z-index: 1100;
		display: flex;
		flex-direction: column;
		overflow: hidden;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--popover, #ffffff);
		color: var(--popover-foreground, #111827);
		box-shadow: 0 12px 32px rgba(15, 23, 42, 0.14);
		animation: notifIn 0.12s cubic-bezier(0.16, 1, 0.3, 1);
		transform-origin: top right;
	}
	/* Drop-up: dibuka ke atas saat ruang di bawah tidak cukup. */
	.notif-menu-up {
		transform-origin: bottom right;
		animation-name: notifInUp;
	}
	@keyframes notifInUp {
		from {
			opacity: 0;
			transform: translateY(4px) scale(0.98);
		}
		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}
	@keyframes notifIn {
		from {
			opacity: 0;
			transform: translateY(-4px) scale(0.98);
		}
		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}
	.notif-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		padding: 8px 12px;
	}
	.notif-title {
		font-size: 13px;
		font-weight: 700;
	}
	.notif-readall {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		border: none;
		border-radius: 7px;
		padding: 4px 8px;
		background: var(--secondary, #eef2f8);
		color: var(--foreground);
		font: inherit;
		font-size: 11.5px;
		font-weight: 600;
		cursor: pointer;
	}
	.notif-readall:hover {
		background: color-mix(in srgb, var(--secondary, #eef2f8) 80%, var(--border));
	}
	.notif-divider {
		height: 1px;
		background: var(--border);
	}
	/* Tab Umum / Pengumuman (klien & mitra). */
	.notif-tabs {
		display: flex;
		gap: 4px;
		padding: 0 12px 8px;
	}
	.notif-tab {
		flex: 1;
		border: none;
		border-radius: 7px;
		padding: 5px 8px;
		background: transparent;
		color: var(--muted-foreground);
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
	}
	.notif-tab:hover {
		background: var(--secondary, #eef2f8);
	}
	.notif-tab-active {
		background: var(--secondary, #eef2f8);
		color: var(--foreground);
	}
	.notif-error {
		display: flex;
		align-items: center;
		gap: 5px;
		margin: 0;
		padding: 8px 12px;
		color: var(--destructive, #dc2626);
		font-size: 12px;
	}
	/* Hanya daftar yang scroll; header & footer tetap. */
	/* Daftar mengisi sisa tinggi menu (menu sendiri dibatasi via style
	   max-height sesuai ruang yang tersedia). */
	.notif-list {
		flex: 1 1 auto;
		min-height: 0;
		overflow-y: auto;
		overscroll-behavior: contain;
		padding: 4px 6px 6px;
	}
	.notif-empty {
		margin: 0;
		padding: 12px 16px;
		color: var(--muted-foreground);
		font-size: 13px;
	}
	.notif-item {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		width: 100%;
		padding: 10px 12px;
		border: none;
		border-radius: 8px;
		background: transparent;
		color: inherit;
		font: inherit;
		text-align: left;
		cursor: pointer;
	}
	.notif-item:hover {
		background: var(--secondary, #eef2f8);
	}
	.notif-item + .notif-item {
		border-top: 1px solid var(--border);
	}
	.notif-dot {
		width: 8px;
		height: 8px;
		margin-top: 5px;
		flex-shrink: 0;
		border-radius: 999px;
		background: var(--primary);
	}
	.notif-item-body {
		display: flex;
		flex-direction: column;
		flex: 1;
		min-width: 0;
	}
	.notif-item-title {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		font-size: 13px;
		font-weight: 500;
		line-height: 1.35;
	}
	.notif-item-title-unread {
		font-weight: 700;
	}
	.notif-item-text {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		margin-top: 4px;
		color: var(--muted-foreground);
		font-size: 12px;
		line-height: 1.4;
	}
	.notif-item-time {
		margin-top: 4px;
		color: var(--muted-foreground);
		font-size: 12px;
		line-height: 1.3;
	}
	.notif-footer {
		padding: 8px 12px;
		border-top: 1px solid var(--border);
	}
	.notif-footer a {
		color: var(--primary);
		font-size: 13px;
		font-weight: 600;
		text-decoration: none;
	}
	.notif-footer a:hover {
		text-decoration: underline;
	}
</style>
