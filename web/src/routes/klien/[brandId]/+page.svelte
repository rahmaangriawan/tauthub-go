<script lang="ts">
	// Fase 2: dashboard per brand (port section dashboard; section lain menyusul).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	let data = $state<any>(null);
	let loading = $state(true);

	const brandId = $derived(page.params.brandId as string);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			const res = await apiRequest(`/v1/brands/${brandId}/dashboard`);
			if (res.ok) data = await res.json();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});
</script>

<main class="mx-auto max-w-5xl px-4 py-8">
	<a class="text-sm text-muted-foreground underline" href="/klien">← Semua brand</a>
	<h1 class="mt-2 text-2xl font-semibold tracking-tight">Dashboard</h1>
	<p class="text-sm text-muted-foreground">
		{#if data}role: {data.role}{/if}
		<a class="ml-2 underline" href={`/klien/${brandId}/profile`}>Profil brand</a>
	</p>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat…</p>
	{:else if data}
		<div class="mt-6 grid gap-3 sm:grid-cols-4">
			<div class="rounded-lg border border-border bg-card p-4">
				<p class="text-sm text-muted-foreground">Sedang dikerjakan</p>
				<p class="text-2xl font-semibold">{data.content_in_progress}</p>
			</div>
			<div class="rounded-lg border border-border bg-card p-4">
				<p class="text-sm text-muted-foreground">Menunggu review</p>
				<p class="text-2xl font-semibold">{data.waiting_review}</p>
			</div>
			<div class="rounded-lg border border-border bg-card p-4">
				<p class="text-sm text-muted-foreground">Job gagal</p>
				<p class="text-2xl font-semibold">{data.failed_jobs}</p>
			</div>
			<div class="rounded-lg border border-border bg-card p-4">
				<p class="text-sm text-muted-foreground">Artikel bulan ini</p>
				<p class="text-2xl font-semibold">{data.monthly_articles.used}<span class="text-sm font-normal text-muted-foreground">/{data.monthly_articles.limit}</span></p>
				{#if data.monthly_articles.warning}
					<p class="text-xs text-destructive">Melewati ambang {data.monthly_articles.percent}%</p>
				{/if}
			</div>
		</div>

		<section class="mt-6 rounded-lg border border-border bg-card p-4">
			<h2 class="font-medium">Aktivitas terakhir</h2>
			<ul class="mt-2 text-sm">
				{#each data.recent_activity as act (act.id)}
					<li class="flex justify-between border-b border-border/50 py-1.5 last:border-0">
						<span>{act.action}</span>
						<span class="text-muted-foreground">{new Date(act.created_at).toLocaleString('id-ID')}</span>
					</li>
				{/each}
			</ul>
		</section>
	{/if}
</main>
