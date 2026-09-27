<script lang="ts">
	// Header halaman admin: judul + deskripsi + slot aksi. Dipakai semua halaman
	// /admin agar tipografi & jarak seragam (parity PageHeader app lama).
	import type { Snippet } from 'svelte';

	let {
		title,
		description = '',
		children
	}: { title: string; description?: string; children?: Snippet } = $props();
</script>

<header class="flex flex-wrap items-start justify-between gap-3">
	<div>
		<h1 class="text-2xl font-semibold tracking-tight">{title}</h1>
		{#if description}<p class="mt-1 text-sm text-muted-foreground">{description}</p>{/if}
	</div>
	{#if children}
		<div class="flex flex-wrap gap-2">{@render children()}</div>
	{/if}
</header>
