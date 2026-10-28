<script lang="ts">
	import { Globe } from '@lucide/svelte';

	/**
	 * Favicon domain media lewat Google s2 — mekanisme yang sudah dipakai
	 * dashboard (chat mitra, katalog klien, ringkasan artikel). Gagal muat →
	 * ikon globe generik; tidak pernah meminjam logo brand lain.
	 */
	let { domain, size = 32 }: { domain: string; size?: number } = $props();

	let failed = $state(false);
	const src = $derived(
		domain ? `https://www.google.com/s2/favicons?domain=${encodeURIComponent(domain)}&sz=64` : ''
	);
</script>

{#if src && !failed}
	<img
		class="pub-favicon"
		{src}
		alt=""
		width={size}
		height={size}
		loading="lazy"
		decoding="async"
		onerror={() => (failed = true)}
	/>
{:else}
	<span class="pub-favicon pub-favicon-fallback" aria-hidden="true">
		<Globe size={Math.round(size * 0.55)} />
	</span>
{/if}
