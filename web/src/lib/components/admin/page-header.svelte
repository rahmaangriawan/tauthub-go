<script lang="ts">
	// Header halaman admin: judul + deskripsi + slot aksi. Dipakai semua halaman
	// /admin agar tipografi & jarak seragam (parity PageHeader app lama).
	// Sekaligus memasang <title> halaman (template "%s — Tauthub") supaya semua
	// halaman admin otomatis punya meta title tanpa diubah satu per satu.
	import type { Snippet } from 'svelte';
	import PageTitle from '$lib/components/page-title.svelte';

	let {
		title,
		description = '',
		metaTitle,
		children
	}: { title: string; description?: string; metaTitle?: string; children?: Snippet } = $props();
</script>

<PageTitle title={metaTitle ?? title} />

<header class="flex flex-wrap items-start justify-between gap-3">
	<div>
		<h1 class="page-title">{title}</h1>
		{#if description}<p class="page-desc">{description}</p>{/if}
	</div>
	{#if children}
		<div class="flex flex-wrap gap-2">{@render children()}</div>
	{/if}
</header>
