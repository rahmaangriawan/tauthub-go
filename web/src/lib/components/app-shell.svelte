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
