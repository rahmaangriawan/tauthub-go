<script lang="ts">
	import '$lib/docs/docs.css';
	import { page } from '$app/state';
	import { afterNavigate } from '$app/navigation';
	import { Menu, Search, X } from '@lucide/svelte';
	import DocsNav from '$lib/components/docs/docs-nav.svelte';
	import DocsSearch from '$lib/components/docs/docs-search.svelte';
	import { searchUi } from '$lib/docs/search-state.svelte';
	import type { LayoutData } from './$types';

	let { data, children }: { data: LayoutData; children: import('svelte').Snippet } = $props();

	let menuOpen = $state(false);
	afterNavigate(() => (menuOpen = false));

	const pathname = $derived(page.url.pathname.replace(/\/+$/, ''));
	const sectionKey = $derived(pathname.split('/')[2] ?? '');
	const section = $derived(data.nav.find((s) => s.key === sectionKey));
	const year = new Date().getFullYear();

	function onkeydown(ev: KeyboardEvent) {
		const el = ev.target as HTMLElement | null;
		const typing = el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable);
		if ((ev.key === 'k' || ev.key === 'K') && (ev.metaKey || ev.ctrlKey)) {
			ev.preventDefault();
			searchUi.open = true;
		} else if (ev.key === '/' && !typing && !ev.metaKey && !ev.ctrlKey) {
			ev.preventDefault();
			searchUi.open = true;
		}
	}
</script>

<svelte:window {onkeydown} />

<div class="dk-page">
	<a class="dk-skip" href="#isi">Lewati ke isi</a>
	<header class="dk-header">
		<div class="dk-header-inner">
			{#if section}
				<button
					class="dk-icon-btn dk-menu-btn"
					type="button"
					aria-label="Buka daftar panduan"
					aria-expanded={menuOpen}
					onclick={() => (menuOpen = !menuOpen)}
				>
					<Menu size={18} strokeWidth={1.5} />
				</button>
			{/if}
			<a class="dk-brand" href="/dokumentasi" aria-label="Dokumentasi Tauthub, beranda">
				<img src="/tauthub-logo.webp" alt="Tauthub" width="134" height="22" />
				<span class="dk-brand-tag">Dokumentasi</span>
			</a>
			<div class="dk-header-actions">
				<button class="dk-search-btn" type="button" onclick={() => (searchUi.open = true)}>
					<Search size={16} strokeWidth={1.5} aria-hidden="true" />
					<span class="dk-search-btn-label">Cari panduan</span>
					<kbd>/</kbd>
				</button>
				<a class="dk-btn dk-btn-primary dk-btn-sm" href="/login">Masuk</a>
			</div>
		</div>
	</header>

	<div class="dk-body" class:dk-has-side={!!section}>
		{#if section}
			{#if menuOpen}
				<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
				<div class="dk-scrim" onclick={() => (menuOpen = false)}></div>
			{/if}
			<aside class="dk-sidebar" class:open={menuOpen} aria-label="Daftar panduan">
				<button class="dk-icon-btn dk-close-btn" type="button" aria-label="Tutup" onclick={() => (menuOpen = false)}>
					<X size={18} strokeWidth={1.5} />
				</button>
				<div class="dk-seg" role="tablist" aria-label="Audiens">
					{#each data.nav as s (s.key)}
						<a href={s.url} class:active={s.key === section.key} aria-current={s.key === section.key ? 'true' : undefined}>
							{s.label}
						</a>
					{/each}
				</div>
				<nav>
					<a class="dk-nav-link dk-nav-root" class:active={pathname === section.url} href={section.url}>
						Peta panduan {section.label}
					</a>
					<DocsNav nodes={section.children} current={pathname} />
				</nav>
			</aside>
		{/if}
		<div class="dk-content" id="isi">
			{@render children()}
		</div>
	</div>

	<footer class="dk-footer">
		<div class="dk-footer-inner">
			<p>© {year} Tauthub</p>
			<nav aria-label="Legal">
				<a href="https://tauthub.com/kebijakan-privasi/">Kebijakan Privasi</a>
				<a href="https://tauthub.com/ketentuan-layanan/">Syarat &amp; Ketentuan</a>
			</nav>
		</div>
	</footer>
</div>

<DocsSearch />
