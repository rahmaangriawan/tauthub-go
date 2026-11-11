<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import { Search, X } from '@lucide/svelte';

	// Kotak cari untuk daftar di dalam modal. `onchange` dipanggil setiap isi
	// berubah supaya induk bisa kembali ke halaman pertama atau memuat ulang.
	let {
		value = $bindable(''),
		placeholder = '',
		onchange
	}: { value?: string; placeholder?: string; onchange?: () => void } = $props();
</script>

<label class="ls">
	<span class="lsIcon"><Search size={15} /></span>
	<input
		type="search"
		bind:value
		{placeholder}
		aria-label={placeholder}
		autocomplete="off"
		oninput={() => onchange?.()}
	/>
	{#if value}
		<button
			type="button"
			class="lsClear"
			aria-label={tr('Hapus pencarian')}
			onclick={() => {
				value = '';
				onchange?.();
			}}
		>
			<X size={14} />
		</button>
	{/if}
</label>

<style>
	.ls {
		position: relative;
		display: block;
		width: min(100%, 360px);
	}
	.lsIcon {
		position: absolute;
		top: 50%;
		left: 12px;
		display: flex;
		color: var(--muted-foreground);
		transform: translateY(-50%);
		pointer-events: none;
	}
	.ls input {
		width: 100%;
		height: 40px;
		padding: 0 36px 0 36px;
		border: 1px solid var(--border);
		border-radius: 9px;
		background: var(--card);
		color: var(--foreground);
		font: inherit;
		font-size: 13px;
		transition: border-color 0.15s ease, box-shadow 0.15s ease;
	}
	.ls input::placeholder {
		color: var(--muted-foreground);
	}
	.ls input::-webkit-search-cancel-button {
		display: none;
	}
	.ls input:focus {
		outline: none;
		border-color: var(--primary);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 18%, transparent);
	}
	.lsClear {
		position: absolute;
		top: 50%;
		right: 6px;
		display: grid;
		place-items: center;
		width: 28px;
		height: 28px;
		border: 0;
		border-radius: 6px;
		background: transparent;
		color: var(--muted-foreground);
		cursor: pointer;
		transform: translateY(-50%);
		transition: background-color 0.15s ease, color 0.15s ease;
	}
	.lsClear:hover {
		background: var(--secondary);
		color: var(--foreground);
	}
	.lsClear:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: -2px;
	}
	@media (prefers-reduced-motion: reduce) {
		.ls input,
		.lsClear {
			transition: none;
		}
	}
</style>
