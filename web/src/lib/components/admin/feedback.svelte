<script lang="ts">
	// Blok umpan balik admin: loading / error / notice. Dipakai semua halaman
	// supaya pesan status tampil dengan gaya yang sama.
	let {
		loading = false,
		error = '',
		notice = '',
		loadingText = 'Memuat data…'
	}: { loading?: boolean; error?: string; notice?: string; loadingText?: string } = $props();
</script>

{#if notice}
	<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>
{/if}
{#if error}
	<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>
{/if}
{#if loading}
	<p class="mt-6 text-sm text-muted-foreground">{loadingText}</p>
{/if}
