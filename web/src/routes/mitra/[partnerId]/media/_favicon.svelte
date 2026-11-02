<script lang="ts">
	// Favicon domain via Google s2 dengan fallback ikon globe. Versi lokal ini
	// membawa CSS-nya sendiri supaya tidak bergantung pada public.css.
	import { Globe } from '@lucide/svelte';

	let { domain, size = 18 }: { domain: string; size?: number } = $props();

	let failed = $state(false);
	const src = $derived(
		domain ? `https://www.google.com/s2/favicons?domain=${encodeURIComponent(domain)}&sz=64` : ''
	);
</script>

{#if src && !failed}
	<img
		class="fav"
		{src}
		alt=""
		width={size}
		height={size}
		loading="lazy"
		decoding="async"
		onerror={() => (failed = true)}
	/>
{:else}
	<span class="fav fav-fallback" style="width: {size}px; height: {size}px">
		<Globe size={Math.round(size * 0.7)} />
	</span>
{/if}

<style>
	.fav {
		flex-shrink: 0;
		border-radius: 4px;
		object-fit: contain;
	}
	.fav-fallback {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		color: var(--muted-foreground, #64748b);
	}
</style>
