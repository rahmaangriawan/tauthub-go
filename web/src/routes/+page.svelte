<script lang="ts">
	// Fase 0 — placeholder landing. Port landing asli ada di fase 1
	// (REWRITE/05_UI_PAGES.md §1).
	let dbStatus = $state<'mengecek' | 'ok' | 'gagal'>('mengecek');

	$effect(() => {
		fetch('/healthz')
			.then((r) => (r.ok ? (dbStatus = 'ok') : (dbStatus = 'gagal')))
			.catch(() => (dbStatus = 'gagal'));
	});
</script>

<main class="flex min-h-screen flex-col items-center justify-center gap-4 bg-background text-foreground">
	<h1 class="text-4xl font-semibold tracking-tight">Tulisio</h1>
	<p class="text-muted-foreground">Rewrite Go + SvelteKit — fase 0 (bootstrap)</p>
	<p class="text-sm">
		API <code class="rounded bg-muted px-1.5 py-0.5">/healthz</code>:
		{#if dbStatus === 'ok'}
			<span class="font-medium text-emerald-600">terhubung</span>
		{:else if dbStatus === 'gagal'}
			<span class="font-medium text-red-600">gagal</span>
		{:else}
			mengecek…
		{/if}
	</p>
	<a
		href="/login"
		class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:opacity-90"
	>
		Masuk
	</a>
</main>
