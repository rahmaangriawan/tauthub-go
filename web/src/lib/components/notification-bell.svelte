<script lang="ts">
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
		readable = true
	}: { apiBase: string; fallbackBase: string; readable?: boolean } = $props();

	let items = $state<Notification[]>([]);
	let unread = $state(0);
	let hasMore = $state(false);
	let error = $state('');
	let open = $state(false);
	let readIds = $state<Set<string>>(new Set());
	let trigger = $state<HTMLButtonElement | null>(null);
	let pos = $state({ left: 0, top: 0 });

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
			error = '';
		} catch (reason) {
			error = reason instanceof Error ? reason.message : 'Gagal memuat notifikasi.';
		}
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

	async function markRead(id: string, link: string | null) {
		await openNotificationLink({
			link,
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
		const rect = trigger?.getBoundingClientRect();
		const width = Math.min(400, window.innerWidth - 32);
		pos = {
			left: rect ? Math.max(8, rect.right - width) : 8,
			top: (rect?.bottom ?? 0) + 8
		};
		open = true;
	}

	$effect(() => {
		if (!open) return;
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape') open = false;
		};
		const onScroll = () => (open = false);
		window.addEventListener('keydown', onKey);
		window.addEventListener('scroll', onScroll, true);
		window.addEventListener('resize', onScroll);
		return () => {
			window.removeEventListener('keydown', onKey);
			window.removeEventListener('scroll', onScroll, true);
			window.removeEventListener('resize', onScroll);
		};
	});
</script>

<button
	bind:this={trigger}
	type="button"
	class="notif-trigger"
	aria-label={`Notifikasi${unread > 0 ? ` (${unread} belum dibaca)` : ''}`}
	aria-expanded={open}
	onclick={toggle}
>
	<span class="notif-bell" class:notif-bell-unread={unread > 0}><Bell size={19} /></span>
	{#if unread > 0}
		<span class="notif-badge">{unread > 9 ? '9+' : unread}</span>
	{/if}
</button>

{#if open}
	<button type="button" class="notif-backdrop" aria-label="Tutup notifikasi" onclick={() => (open = false)}
	></button>
	<div
		class="notif-menu"
		role="menu"
		aria-label="Notifikasi"
		style="left: {pos.left}px; top: {pos.top}px; width: min(400px, calc(100vw - 32px));"
	>
		<div class="notif-header">
			<span class="notif-title">Notifikasi</span>
			{#if unread > 0}
				<button class="notif-readall" type="button" onclick={() => void readAll()}>
					<Check size={13} />Telah dibaca semua
				</button>
			{/if}
		</div>
		<div class="notif-divider"></div>
		{#if error}
			<p class="notif-error"><TriangleAlert size={12} /> {error}</p>
		{/if}
		<div class="notif-list">
			{#if items.length === 0 && !error}
				<p class="notif-empty">Belum ada notifikasi.</p>
			{/if}
			{#each items as notification (notification.id)}
				<button class="notif-item" type="button" onclick={() => void markRead(notification.id, notification.link)}>
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
				<a href={`${fallbackBase}/notifications`} onclick={() => (open = false)}>Lihat semua →</a>
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
		max-height: min(440px, calc(100vh - 80px));
		overflow: hidden;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--popover, #ffffff);
		color: var(--popover-foreground, #111827);
		box-shadow: 0 12px 32px rgba(15, 23, 42, 0.14);
		animation: notifIn 0.12s cubic-bezier(0.16, 1, 0.3, 1);
		transform-origin: top right;
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
	.notif-list {
		max-height: min(360px, calc(100vh - 180px));
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
