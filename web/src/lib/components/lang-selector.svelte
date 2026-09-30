<script lang="ts">
	// Selector bahasa di utility bar — parity components/shells/lang-selector.tsx
	// app lama: dropdown ID/EN dengan bendera vektor (bukan emoji), tersimpan di
	// localStorage lewat i18n.svelte.ts. Satu instance dipakai ketiga shell
	// (klien/mitra/admin) supaya perilakunya identik.
	import { ChevronDown, Check } from '@lucide/svelte';
	import { getI18n } from '$lib/i18n/i18n.svelte';
	import type { Locale } from '$lib/i18n/dictionaries';

	let { align = 'end' }: { align?: 'start' | 'end' } = $props();

	// Objek i18n disimpan utuh (bukan didestruktur) — `locale`/`t` dibaca lewat
	// getter agar reaktif saat bahasa diganti; destrukturisasi membekukan nilai.
	const i18n = getI18n();

	const OPTIONS: Array<{ value: Locale; label: string }> = [
		{ value: 'id', label: 'Indonesia' },
		{ value: 'en', label: 'English' }
	];

	let open = $state(false);
	let wrapper = $state<HTMLDivElement | null>(null);

	const currentLabel = $derived(i18n.locale === 'id' ? 'Indonesia' : 'English');

	function pick(value: Locale) {
		i18n.setLocale(value);
		open = false;
	}

	function onClickOutside(event: MouseEvent) {
		if (open && wrapper && !wrapper.contains(event.target as Node)) open = false;
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && open) open = false;
	}
</script>

<svelte:window onclick={onClickOutside} onkeydown={onKeydown} />

<div class="lang-wrapper" bind:this={wrapper}>
	<button
		class="app-btn app-btn-soft app-lang-trigger"
		type="button"
		aria-label={i18n.t.nav.pick_language}
		aria-haspopup="menu"
		aria-expanded={open}
		onclick={() => (open = !open)}
	>
		{#if i18n.locale === 'id'}
			<svg width="16" height="16" viewBox="0 0 36 36" aria-hidden="true" class="lang-flag">
				<path fill="#dc1f26" d="M32 5H4a4 4 0 0 0-4 4v9h36V9a4 4 0 0 0-4-4" />
				<path fill="#eee" d="M36 27a4 4 0 0 1-4 4H4a4 4 0 0 1-4-4v-9h36z" />
			</svg>
		{:else}
			<svg width="16" height="16" viewBox="0 0 36 36" aria-hidden="true" class="lang-flag">
				<path fill="#b22334" d="M35.445 7C34.752 5.809 33.477 5 32 5H18v2zM0 25h36v2H0zm18-8h18v2H18zm0-4h18v2H18zM0 21h36v2H0zm4 10h28c1.477 0 2.752-.809 3.445-2H.555c.693 1.191 1.968 2 3.445 2M18 9h18v2H18z" />
				<path fill="#eee" d="M.068 27.679q.025.14.059.277q.04.15.092.296c.089.259.197.509.333.743L.555 29h34.89l.002-.004a4 4 0 0 0 .332-.741a4 4 0 0 0 .152-.576c.041-.22.069-.446.069-.679H0c0 .233.028.458.068.679M0 23h36v2H0zm0-4v2h36v-2H18zm18-4h18v2H18zm0-4h18v2H18zM.555 7l-.003.005zM.128 8.044c.025-.102.06-.199.092-.297a4 4 0 0 0-.092.297M18 9h18c0-.233-.028-.459-.069-.68a3.6 3.6 0 0 0-.153-.576A4 4 0 0 0 35.445 7H18z" />
				<path fill="#3c3b6e" d="M18 5H4a4 4 0 0 0-4 4v10h18z" />
				<path fill="#fff" d="m2.001 7.726l.618.449l-.236.725L3 8.452l.618.448l-.236-.725L4 7.726h-.764L3 7l-.235.726zm2 2l.618.449l-.236.725l.617-.448l.618.448l-.236-.725L6 9.726h-.764L5 9l-.235.726zm4 0l.618.449l-.236.725l.617-.448l.618.448l-.236-.725l.618-.449h-.764L9 9l-.235.726zm4 0l.618.449l-.236.725l.617-.448l.618.448l-.236-.725l.618-.449h-.764L13 9l-.235.726zm-8 4l.618.449l-.236.725l.617-.448l.618.448l-.236-.725l.618-.449h-.764L5 13l-.235.726zm4 0l.618.449l-.236.725l.617-.448l.618.448l-.236-.725l.618-.449h-.764L9 13l-.235.726zm4 0l.618.449l-.236.725l.617-.448l.618.448l-.236-.725l.618-.449h-.764L13 13l-.235.726zm-6-6l.618.449l-.236.725L7 8.452l.618.448l-.236-.725L8 7.726h-.764L7 7l-.235.726zm4 0l.618.449l-.236.725l.617-.448l.618.448l-.236-.725l.618-.449h-.764L11 7l-.235.726zm4 0l.618.449l-.236.725l.617-.448l.618.448l-.236-.725l.618-.449h-.764L15 7l-.235.726zm-12 4l.618.449l-.236.725l.617-.448l.618.448l-.236-.725l.618-.449h-.764L3 11l-.235.726zM6.383 12.9L7 12.452l.618.448l-.236-.725l.618-.449h-.764L7 11l-.235.726h-.764l.618.449zm3.618-1.174l.618.449l-.236.725l.617-.448l.618.448l-.236-.725l.618-.449h-.764L11 11l-.235.726zm4 0l.618.449l-.236.725l.617-.448l.618.448l-.236-.725l.618-.449h-.764L15 11l-.235.726zm-12 4l.618.449l-.236.725l.617-.448l.618.448l-.236-.725l.618-.449h-.764L3 15l-.235.726zM6.383 16.9L7 16.452l.618.448l-.236-.725l.618-.449h-.764L7 15l-.235.726h-.764l.618.449zm3.618-1.174l.618.449l-.236.725l.617-.448l.618.448l-.236-.725l.618-.449h-.764L11 15l-.235.726zm4 0l.618.449l-.236.725l.617-.448l.618.448l-.236-.725l.618-.449h-.764L15 15l-.235.726z" />
			</svg>
		{/if}
		<span class="app-lang-label">{currentLabel}</span>
		<ChevronDown size={14} />
	</button>

	{#if open}
		<div class="lang-menu" class:lang-menu-start={align === 'start'} role="menu" aria-label={i18n.t.nav.pick_language}>
			{#each OPTIONS as option (option.value)}
				<button
					type="button"
					role="menuitemradio"
					aria-checked={i18n.locale === option.value}
					class="lang-item"
					class:lang-item-active={i18n.locale === option.value}
					onclick={() => pick(option.value)}
				>
					<span class="lang-item-label">{option.label}</span>
					{#if i18n.locale === option.value}<Check size={14} />{/if}
				</button>
			{/each}
		</div>
	{/if}
</div>

<style>
	.lang-wrapper {
		position: relative;
		display: inline-flex;
	}
	.app-lang-trigger {
		gap: 6px;
	}
	.lang-flag {
		border-radius: 2px;
		flex-shrink: 0;
	}
	.lang-menu {
		position: absolute;
		top: calc(100% + 6px);
		right: 0;
		z-index: 1100;
		display: flex;
		flex-direction: column;
		min-width: 160px;
		padding: 5px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--card);
		box-shadow: var(--shadow-md);
		animation: langIn 0.12s cubic-bezier(0.16, 1, 0.3, 1);
	}
	.lang-menu-start {
		right: auto;
		left: 0;
	}
	@keyframes langIn {
		from {
			opacity: 0;
			transform: translateY(-4px) scale(0.98);
		}
		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}
	.lang-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		width: 100%;
		min-height: 34px;
		padding: 6px 10px;
		border: 0;
		border-radius: 7px;
		background: transparent;
		color: var(--foreground);
		font: inherit;
		font-size: 13px;
		text-align: left;
		cursor: pointer;
	}
	.lang-item:hover {
		background: var(--secondary);
	}
	.lang-item-active {
		color: var(--primary);
		font-weight: 700;
	}
</style>
