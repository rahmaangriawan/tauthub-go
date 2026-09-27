<script lang="ts">
	/**
	 * Kerangka halaman ber-sidebar (klien, mitra, admin). Parity dengan
	 * product-shell.module.css app lama: sidebar 264px tetap di desktop, dan di
	 * bawah 768px sidebar berubah jadi drawer (tombol hamburger di topbar)
	 * supaya konten tidak lagi terjepit / memicu scroll horizontal.
	 */
	import type { Component, Snippet } from 'svelte';
	import { List, X } from '@lucide/svelte';
	import { page } from '$app/state';

	export type ShellNavItem = {
		label: string;
		href: string;
		icon: Component<{ size?: number; strokeWidth?: number }>;
		active: boolean;
	};
	export type ShellNavGroup = { label: string; items: ShellNavItem[] };

	let {
		brandHref,
		navLabel,
		groups,
		account,
		toolbar,
		children
	}: {
		brandHref: string;
		navLabel: string;
		groups: ShellNavGroup[];
		account?: Snippet;
		toolbar?: Snippet;
		children: Snippet;
	} = $props();

	let drawerOpen = $state(false);
	let panel = $state<HTMLDivElement | null>(null);
	const pathname = $derived(page.url.pathname);

	// Tutup drawer setiap kali pindah halaman (parity onNavigate app lama).
	$effect(() => {
		pathname;
		drawerOpen = false;
	});

	// Kunci scroll latar, Escape untuk menutup, dan fokus dipindah ke panel
	// (lalu dikembalikan ke tombol pemicu) selama drawer terbuka.
	$effect(() => {
		if (!drawerOpen) return;
		const previous = document.body.style.overflow;
		const opener = document.activeElement as HTMLElement | null;
		document.body.style.overflow = 'hidden';
		panel?.focus();
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape') drawerOpen = false;
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
			<a href={brandHref} class="text-xl font-extrabold tracking-tight" style="color: var(--primary);">
				taut<span style="color: var(--foreground);">hub</span>
			</a>
		</div>

		<nav class="app-sidebar-nav" aria-label={navLabel}>
			{#each groups as group, index (group.label)}
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
				{/each}
			{/each}
		</nav>

		{@render account?.()}
	</div>
{/snippet}

<div class="app-shell">
	<aside class="app-sidebar">{@render sidebar()}</aside>

	<div class="flex min-w-0 flex-1 flex-col">
		<header class="app-utility-bar">
			<button
				class="app-shell-trigger app-btn app-btn-soft !min-h-8 !px-2"
				type="button"
				aria-label="Buka navigasi"
				aria-expanded={drawerOpen}
				onclick={() => (drawerOpen = true)}
			>
				<List size={20} />
			</button>
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
			aria-label="Tutup navigasi"
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
				aria-label="Tutup navigasi"
				onclick={() => (drawerOpen = false)}
			>
				<X size={20} />
			</button>
			{@render sidebar()}
		</div>
	</div>
{/if}
