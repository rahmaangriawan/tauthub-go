<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	/**
	 * Kerangka halaman ber-sidebar (klien, mitra, admin). Parity dengan
	 * product-shell.module.css app lama: sidebar 264px tetap di desktop, dan di
	 * bawah 768px sidebar berubah jadi drawer (tombol hamburger di topbar)
	 * supaya konten tidak lagi terjepit / memicu scroll horizontal.
	 */
	import type { Component, Snippet } from 'svelte';
	import { List, Search, X } from '@lucide/svelte';
	import { page } from '$app/state';

	export type ShellNavItem = {
		label: string;
		href: string;
		icon: Component<{ size?: number; strokeWidth?: number }>;
		active: boolean;
		// Sub-halaman bertingkat (parity navSublinks app lama): tampil menjorok
		// di bawah induknya, selalu terbuka.
		children?: Array<{ label: string; href: string; active: boolean }>;
	};
	export type ShellNavGroup = { label: string; items: ShellNavItem[] };

	let {
		brandHref,
		navLabel,
		groups,
		searchable = false,
		account,
		toolbarLeft,
		toolbar,
		children
	}: {
		brandHref: string;
		navLabel: string;
		groups: ShellNavGroup[];
		// Tampilkan kotak "Cari halaman…" yang memfilter menu secara live
		// (Ctrl/Cmd+K memfokuskannya). Dipakai sidebar admin.
		searchable?: boolean;
		account?: Snippet;
		// Sisi kiri utility bar — parity slot `utilityBar > div:first-child` app
		// lama tempat pill langkah Taut Pintar di-portal (sebaris hamburger).
		toolbarLeft?: Snippet;
		toolbar?: Snippet;
		children: Snippet;
	} = $props();

	let drawerOpen = $state(false);
	let panel = $state<HTMLDivElement | null>(null);
	let navQuery = $state('');
	let searchEl = $state<HTMLInputElement | null>(null);
	const pathname = $derived(page.url.pathname);

	// Tutup drawer setiap kali pindah halaman (parity onNavigate app lama).
	$effect(() => {
		pathname;
		drawerOpen = false;
	});

	// Ganti halaman = reset filter menu supaya sidebar kembali utuh.
	$effect(() => {
		pathname;
		navQuery = '';
	});

	// Ctrl/Cmd+K memfokuskan pencarian menu (parity palette app lama).
	$effect(() => {
		if (!searchable) return;
		const onKey = (event: KeyboardEvent) => {
			if ((event.metaKey || event.ctrlKey) && (event.key === 'k' || event.key === 'K')) {
				event.preventDefault();
				searchEl?.focus();
				searchEl?.select();
			}
		};
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});

	// Filter menu: item cocok bila labelnya (atau salah satu sub-halamannya)
	// mengandung kata kunci. Grup tanpa item yang cocok disembunyikan.
	const filteredGroups = $derived.by(() => {
		const q = navQuery.trim().toLowerCase();
		if (!q) return groups;
		const out: ShellNavGroup[] = [];
		for (const group of groups) {
			const items: ShellNavItem[] = [];
			for (const item of group.items) {
				if (item.label.toLowerCase().includes(q)) {
					items.push(item);
					continue;
				}
				const children = (item.children ?? []).filter((child) => child.label.toLowerCase().includes(q));
				if (children.length > 0) items.push({ ...item, children });
			}
			if (items.length > 0) out.push({ ...group, items });
		}
		return out;
	});

	// Kunci scroll latar, Escape untuk menutup, fokus dipindah ke panel lalu
	// dikembalikan ke tombol pemicu, dan Tab dijebak di dalam panel selama
	// drawer terbuka (parity <Dialog.Content> Radix di app lama).
	$effect(() => {
		if (!drawerOpen) return;
		const previous = document.body.style.overflow;
		const opener = document.activeElement as HTMLElement | null;
		document.body.style.overflow = 'hidden';
		panel?.focus();
		const focusables = () =>
			Array.from(
				panel?.querySelectorAll<HTMLElement>('a[href], button:not([disabled])') ?? []
			).filter((el) => el.offsetParent !== null);
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape') {
				drawerOpen = false;
				return;
			}
			if (event.key !== 'Tab') return;
			const items = focusables();
			if (items.length === 0) return;
			const first = items[0];
			const last = items[items.length - 1];
			const active = document.activeElement as HTMLElement | null;
			if (event.shiftKey && (active === first || active === panel)) {
				event.preventDefault();
				last.focus();
			} else if (!event.shiftKey && active === last) {
				event.preventDefault();
				first.focus();
			}
		};
		window.addEventListener('keydown', onKey);
		return () => {
			document.body.style.overflow = previous;
			window.removeEventListener('keydown', onKey);
			opener?.focus();
		};
	});
</script>

{#snippet sidebar()}
	<div class="app-sidebar-inner">
		<div class="app-sidebar-brand">
			<a href={brandHref} class="app-brand-link" aria-label="Tauthub">
				<img src="/tauthub-logo.webp" alt="Tauthub" width="140" height="35" class="app-brand-logo" />
			</a>
		</div>

		{#if searchable}
			<div class="app-sidebar-search">
				<Search size={14} aria-hidden="true" />
				<input
					bind:this={searchEl}
					bind:value={navQuery}
					type="search"
					placeholder={tr('Cari halaman…')}
					aria-label={tr('Cari halaman admin')}
				/>
			</div>
		{/if}

		<nav class="app-sidebar-nav" aria-label={navLabel}>
			{#each filteredGroups as group, index (group.label)}
				{#if index > 0}<span class="app-sidebar-separator" aria-hidden="true"></span>{/if}
				<span class="app-sidebar-label">{group.label}</span>
				{#each group.items as item (item.href + item.label)}
					<a
						href={item.href}
						class="app-sidebar-link"
						data-active={item.active ? 'true' : 'false'}
						aria-current={item.active ? 'page' : undefined}
					>
						<item.icon size={18} strokeWidth={item.active ? 2.4 : 2} />
						<span>{item.label}</span>
					</a>
					{#if item.children?.length}
						<span class="app-sidebar-sublinks">
							{#each item.children as child (child.href + child.label)}
								<a href={child.href} class="app-sidebar-sublink" data-active={child.active ? 'true' : 'false'} aria-current={child.active ? 'page' : undefined}>
									{child.label}
								</a>
							{/each}
						</span>
					{/if}
				{/each}
			{/each}
			{#if filteredGroups.length === 0}
				<p class="px-2.5 py-2 text-xs" style="color: var(--sidebar-soft);">{tr('Tidak ada menu yang cocok.')}</p>
			{/if}
		</nav>

		{@render account?.()}
	</div>
{/snippet}

<div class="app-shell">
	<aside class="app-sidebar">{@render sidebar()}</aside>

	<div class="flex min-w-0 flex-1 flex-col">
		<header class="app-utility-bar">
			<div class="app-utility-left">
				<button
					class="app-shell-trigger app-btn app-btn-soft !min-h-8 !px-2"
					type="button"
					aria-label={tr("Buka navigasi")}
					aria-expanded={drawerOpen}
					onclick={() => (drawerOpen = true)}
				>
					<List size={20} />
				</button>
				{@render toolbarLeft?.()}
			</div>
			<div class="app-utility-actions">{@render toolbar?.()}</div>
		</header>
		<main class="app-content">
			{@render children()}
		</main>
	</div>
</div>

{#if drawerOpen}
	<div class="app-drawer">
		<button
			class="app-drawer-backdrop"
			type="button"
			aria-label={tr("Tutup navigasi")}
			onclick={() => (drawerOpen = false)}
		></button>
		<div
			class="app-drawer-panel"
			role="dialog"
			aria-modal="true"
			aria-label={navLabel}
			tabindex="-1"
			bind:this={panel}
		>
			<button
				class="app-drawer-close"
				type="button"
				aria-label={tr("Tutup navigasi")}
				onclick={() => (drawerOpen = false)}
			>
				<X size={20} />
			</button>
			{@render sidebar()}
		</div>
	</div>
{/if}
