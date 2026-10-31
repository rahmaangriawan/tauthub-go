<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	/**
	 * Kerangka halaman ber-sidebar (klien, mitra, admin). Parity dengan
	 * product-shell.module.css app lama: sidebar 264px tetap di desktop, dan di
	 * bawah 768px sidebar berubah jadi drawer (tombol hamburger di topbar)
	 * supaya konten tidak lagi terjepit / memicu scroll horizontal.
	 */
	import type { Component, Snippet } from 'svelte';
	import { fly, fade } from 'svelte/transition';
	import { cubicIn, cubicOut } from 'svelte/easing';
	import { Search } from '@lucide/svelte';
	import { page } from '$app/state';

	export type ShellNavItem = {
		label: string;
		href: string;
		icon: Component<{ size?: number; strokeWidth?: number }>;
		active: boolean;
		// Sub-halaman bertingkat (parity navSublinks app lama): tampil menjorok
			// di bawah induknya, selalu terbuka.
			children?: Array<{ label: string; href: string; active: boolean; badge?: number | null }>;
		/** Badge angka (merah) di kanan label — mis. jumlah pesan belum dibaca. */
		badge?: number | null;
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
	// Default sidebar desktop selalu terbuka (collapsible="icon" ala shadcn).
	let sidebarCollapsed = $state(false);
	// Animasi drawer (buka & tutup) dimatikan bila pengguna memilih reduce motion.
	const motionOK = typeof window !== 'undefined' && !window.matchMedia('(prefers-reduced-motion: reduce)').matches;
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

	// Ctrl/Cmd+B untuk toggle expand/collapse sidebar desktop (konvensi shadcn),
	// dan Ctrl/Cmd+K memfokuskan pencarian menu (bila searchable).
	$effect(() => {
		const onKey = (event: KeyboardEvent) => {
			const target = event.target as HTMLElement | null;
			const inInput = target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable);
			if ((event.metaKey || event.ctrlKey) && (event.key === 'b' || event.key === 'B') && !inInput) {
				event.preventDefault();
				sidebarCollapsed = !sidebarCollapsed;
				return;
			}
			if (searchable && (event.metaKey || event.ctrlKey) && (event.key === 'k' || event.key === 'K')) {
				event.preventDefault();
				if (sidebarCollapsed) sidebarCollapsed = false;
				queueMicrotask(() => {
					searchEl?.focus();
					searchEl?.select();
				});
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

{#snippet sidebar(inDrawer = false)}
	{@const isCollapsed = !inDrawer && sidebarCollapsed}
	<div class="app-sidebar-inner">
		<div class="app-sidebar-brand">
			<a
				href={brandHref}
				class="app-brand-link"
				aria-label="Tauthub"
				tabindex={isCollapsed ? -1 : undefined}
				aria-hidden={isCollapsed ? 'true' : undefined}
			>
				<img src="/tauthub-logo.webp" alt="Tauthub" width="140" height="35" class="app-brand-logo" />
			</a>
			{#if !inDrawer}
				<button
					type="button"
					class="app-sidebar-collapse-btn"
					aria-label={isCollapsed ? tr('Perluas sidebar') : tr('Ciutkan sidebar')}
					aria-expanded={!isCollapsed}
					title={isCollapsed ? `${tr('Perluas sidebar')} (⌘B)` : `${tr('Ciutkan sidebar')} (⌘B)`}
					onclick={() => (sidebarCollapsed = !sidebarCollapsed)}
				>
					<!-- Saat sidebar tertutup: favicon jadi tampilan default, ikon panel
					     muncul saat hover/fokus supaya aksi perluas tetap terbaca. -->
					{#if isCollapsed}
						<img
							src="/tauthub-icon.png"
							alt=""
							width="22"
							height="22"
							class="app-collapse-favicon"
							aria-hidden="true"
						/>
					{/if}
					<span class="app-collapse-icon-wrap" data-collapsed={isCollapsed ? 'true' : 'false'}>
						{#if isCollapsed}
							<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
								<path d="M0 0h24v24H0z" fill="none" />
								<g fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5">
									<path d="M3 11c0-3.771 0-5.657 1.172-6.828S7.229 3 11 3h2c3.771 0 5.657 0 6.828 1.172S21 7.229 21 11v2c0 3.771 0 5.657-1.172 6.828S16.771 21 13 21h-2c-3.771 0-5.657 0-6.828-1.172S3 16.771 3 13zm12-8v18" />
									<path d="m8 9l1.108.879C10.37 10.879 11 11.379 11 12s-.63 1.121-1.892 2.121L8 15" />
								</g>
							</svg>
						{:else}
							<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
								<path d="M0 0h24v24H0z" fill="none" />
								<g fill="none" stroke="currentColor" stroke-width="1.5">
									<path d="M2 12c0-3.69 0-5.534.814-6.841a4.8 4.8 0 0 1 1.105-1.243C5.08 3 6.72 3 10 3h4c3.28 0 4.919 0 6.081.916c.43.338.804.759 1.105 1.243C22 6.466 22 8.31 22 12s0 5.534-.814 6.841a4.8 4.8 0 0 1-1.105 1.243C18.92 21 17.28 21 14 21h-4c-3.28 0-4.919 0-6.081-.916a4.8 4.8 0 0 1-1.105-1.243C2 17.534 2 15.69 2 12Z" />
									<path stroke-linejoin="round" d="M9.5 3v18" />
									<path stroke-linecap="round" stroke-linejoin="round" d="M5 7h1m-1 3h1" />
								</g>
							</svg>
						{/if}
					</span>
				</button>
			{/if}
		</div>

		{#if searchable}
			<div class="app-sidebar-search">
				<Search size={14} aria-hidden="true" />
				<input
					bind:this={searchEl}
					bind:value={navQuery}
					type="search"
					name="menu-filter"
					autocomplete="off"
					spellcheck="false"
					readonly
					tabindex={isCollapsed ? -1 : undefined}
					onfocus={(e) => e.currentTarget.removeAttribute('readonly')}
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
						data-tooltip={isCollapsed ? item.label : undefined}
						title={isCollapsed ? item.label : undefined}
						aria-label={isCollapsed ? item.label : undefined}
						aria-current={item.active ? 'page' : undefined}
					>
						<item.icon size={18} strokeWidth={item.active ? 2.4 : 2} />
						<span class="app-sidebar-link-text">{item.label}</span>
						{#if (item.badge ?? 0) > 0}
							<span class="app-sidebar-badge ml-auto inline-flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[10px] font-bold leading-none text-white" style="background: var(--destructive, #e11d48);">{item.badge}</span>
						{/if}
					</a>
					{#if item.children?.length}
						<span class="app-sidebar-sublinks">
							{#each item.children as child (child.href + child.label)}
								<a href={child.href} class="app-sidebar-sublink" data-active={child.active ? 'true' : 'false'} aria-current={child.active ? 'page' : undefined}>
									<span>{child.label}</span>
									{#if (child.badge ?? 0) > 0}
										<span class="ml-auto inline-flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[10px] font-bold leading-none text-white" style="background: var(--destructive, #e11d48);">{child.badge}</span>
									{/if}
								</a>
							{/each}
						</span>
					{/if}
				{/each}
			{/each}
			{#if filteredGroups.length === 0 && !isCollapsed}
				<p class="px-2.5 py-2 text-xs" style="color: var(--sidebar-soft);">{tr('Tidak ada menu yang cocok.')}</p>
			{/if}
		</nav>

		{@render account?.()}
	</div>
{/snippet}

<div class="app-shell">
	<aside class="app-sidebar" data-collapsed={sidebarCollapsed ? 'true' : 'false'}>{@render sidebar(false)}</aside>

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
					<!-- Ikon "buka sidebar": panel dengan chevron menunjuk kanan. -->
					<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
						<g fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5">
							<path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2" />
							<path d="M7.25 10L5.5 12l1.75 2m2.25 7V3" />
						</g>
					</svg>
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
			transition:fade={{ duration: motionOK ? 180 : 0 }}
		></button>
		<div
			class="app-drawer-panel"
			role="dialog"
			aria-modal="true"
			aria-label={navLabel}
			tabindex="-1"
			bind:this={panel}
			in:fly={{ x: -300, duration: motionOK ? 280 : 0, easing: cubicOut }}
			out:fly={{ x: -300, duration: motionOK ? 200 : 0, easing: cubicIn }}
		>
			<button
				class="app-drawer-close"
				type="button"
				aria-label={tr("Tutup navigasi")}
				onclick={() => (drawerOpen = false)}
			>
				<!-- Ikon "tutup sidebar": panel dengan chevron menunjuk kiri. -->
				<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" aria-hidden="true">
					<g fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5">
						<path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2m-9.5 0V3" />
						<path d="m5.5 10l1.75 2l-1.75 2" />
					</g>
				</svg>
			</button>
				{@render sidebar(true)}
		</div>
	</div>
{/if}
